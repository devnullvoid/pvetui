package components

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// manualRefresh refreshes all data manually.
func (a *App) manualRefresh() {
	// * Check if there are any pending operations
	if models.GlobalState.HasPendingOperations() {
		a.showMessageSafe("Cannot refresh data while there are pending operations in progress")
		return
	}

	// Show loading indicator
	a.header.ShowLoading("Refreshing data...")
	a.footer.SetLoading(true)

	// Store current selections for restoration
	var hasSelectedNode, hasSelectedVM bool

	var selectedNodeName, selectedVMNode string

	var selectedVMID int

	if node := a.nodeList.GetSelectedNode(); node != nil {
		hasSelectedNode = true
		selectedNodeName = node.Name
	}

	if vm := a.vmList.GetSelectedVM(); vm != nil {
		hasSelectedVM = true
		selectedVMID = vm.ID
		selectedVMNode = vm.Node
	}

	// Check if search is currently active
	searchWasActive := a.mainLayout.GetItemCount() > 4

	// Run data refresh in goroutine to avoid blocking UI
	go func() {
		// Wait a moment for API changes to propagate to cluster resources endpoint
		// This ensures we get fresh data after configuration updates
		time.Sleep(500 * time.Millisecond)

		if a.isGroupMode {
			// Group mode logic
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			nodes, vms, err := a.groupManager.GetGroupClusterResources(ctx, true)
			if err != nil {
				a.QueueUpdateDraw(func() {
					a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
					a.footer.SetLoading(false)
				})

				return
			}

			// Debug logging retained at Debug level for troubleshooting.

			a.QueueUpdateDraw(func() {
				// Update GlobalState nodes/VMs; UI lists will be updated after enrichment to reduce flicker.
				models.GlobalState.OriginalNodes = nodes
				models.GlobalState.OriginalVMs = vms

				// Apply node filter if active
				nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
				if nodeSearchState != nil && nodeSearchState.Filter != "" {
					models.FilterNodes(nodeSearchState.Filter)
				} else {
					models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
					copy(models.GlobalState.FilteredNodes, nodes)
				}

				// Apply VM filter if active
				vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
				if vmSearchState != nil && vmSearchState.Filter != "" {
					models.FilterVMs(vmSearchState.Filter)
				} else {
					models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
					copy(models.GlobalState.FilteredVMs, vms)
				}

				// Start background enrichment for detailed node stats
				// Pass false for isInitialLoad since this is a manual refresh
				a.enrichGroupNodesSequentially(nodes, hasSelectedNode, selectedNodeName, hasSelectedVM, selectedVMID, selectedVMNode, searchWasActive, false)
			})
		} else {
			// Single profile logic
			// Fetch fresh data bypassing cache
			cluster, err := a.client.GetFreshClusterStatus()
			if err != nil {
				a.QueueUpdateDraw(func() {
					a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
					a.footer.SetLoading(false)
				})

				return
			}

			// Initial UI update and enrichment
			a.applyInitialClusterUpdate(cluster)
			a.enrichNodesSequentially(cluster, hasSelectedNode, selectedNodeName, hasSelectedVM, selectedVMID, selectedVMNode, searchWasActive)
		}
	}()
}

// applyInitialClusterUpdate updates global state and UI with basic cluster data and rebuilt VM list
func (a *App) applyInitialClusterUpdate(cluster *api.Cluster) {
	a.QueueUpdateDraw(func() {
		// Update global state nodes from cluster resources
		models.GlobalState.OriginalNodes = make([]*api.Node, len(cluster.Nodes))
		copy(models.GlobalState.OriginalNodes, cluster.Nodes)

		// Apply node filter if active
		if nodeState := models.GlobalState.GetSearchState(api.PageNodes); nodeState != nil && nodeState.Filter != "" {
			models.FilterNodes(nodeState.Filter)
		} else {
			models.GlobalState.FilteredNodes = make([]*api.Node, len(cluster.Nodes))
			copy(models.GlobalState.FilteredNodes, cluster.Nodes)
		}
		a.nodeList.SetNodes(models.GlobalState.FilteredNodes)

		// Rebuild VM list from fresh cluster resources so new guests appear immediately
		var vms []*api.VM
		for _, n := range cluster.Nodes {
			if n != nil {
				for _, vm := range n.VMs {
					if vm != nil {
						vms = append(vms, vm)
					}
				}
			}
		}
		models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
		copy(models.GlobalState.OriginalVMs, vms)

		// Apply VM filter if active
		if vmState := models.GlobalState.GetSearchState(api.PageGuests); vmState != nil && vmState.Filter != "" {
			models.FilterVMs(vmState.Filter)
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		} else {
			models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
			copy(models.GlobalState.FilteredVMs, vms)
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		}

		// Update cluster summary/status
		a.clusterStatus.Update(cluster)
	})
}

// enrichNodesSequentially enriches node data in parallel and finalizes the refresh
func (a *App) enrichNodesSequentially(cluster *api.Cluster, hasSelectedNode bool, selectedNodeName string, hasSelectedVM bool, selectedVMID int, selectedVMNode string, searchWasActive bool) {
	go func() {
		var wg sync.WaitGroup

		// Enrich nodes in parallel
		for i, node := range cluster.Nodes {
			if node == nil {
				continue
			}

			wg.Add(1)
			go func(idx int, n *api.Node) {
				defer wg.Done()

				freshNode, err := a.client.RefreshNodeData(n.Name)
				if err == nil && freshNode != nil {
					// Preserve VMs from the FRESH cluster data (not the original stale data)
					// This ensures we keep the updated VM names we just fetched
					if cluster.Nodes[idx] != nil {
						freshNode.VMs = cluster.Nodes[idx].VMs
					}

					// Update only the specific node index in global state
					// Safe to access specific index concurrently
					if idx < len(models.GlobalState.OriginalNodes) {
						models.GlobalState.OriginalNodes[idx] = freshNode
					}
				}
			}(i, node)
		}

		wg.Wait()

		// Final update: rebuild VMs, cluster version, status, and complete refresh
		a.QueueUpdateDraw(func() {
			// Re-apply node filters with enriched data
			nodeState := models.GlobalState.GetSearchState(api.PageNodes)
			if nodeState != nil && nodeState.Filter != "" {
				models.FilterNodes(nodeState.Filter)
			} else {
				// Update filtered nodes from original (copy)
				// We need to re-copy because OriginalNodes was updated in place
				models.GlobalState.FilteredNodes = make([]*api.Node, len(models.GlobalState.OriginalNodes))
				copy(models.GlobalState.FilteredNodes, models.GlobalState.OriginalNodes)
			}
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)

			// Rebuild VM list from enriched nodes
			var vms []*api.VM
			for _, n := range models.GlobalState.OriginalNodes {
				if n != nil {
					for _, vm := range n.VMs {
						if vm != nil {
							vms = append(vms, vm)
						}
					}
				}
			}

			// Update global VM state with enriched data
			models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
			copy(models.GlobalState.OriginalVMs, vms)

			// Apply VM filter if active
			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
			if vmSearchState != nil && vmSearchState.Filter != "" {
				models.FilterVMs(vmSearchState.Filter)
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			} else {
				models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
				copy(models.GlobalState.FilteredVMs, vms)
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			}

			// Update cluster version from enriched nodes
			for _, n := range models.GlobalState.OriginalNodes {
				if n != nil && n.Version != "" {
					cluster.Version = fmt.Sprintf("Proxmox VE %s", n.Version)
					break
				}
			}
			a.clusterStatus.Update(cluster)

			// Final selection restore and search UI restoration
			nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)

			a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
				hasSelectedNode, selectedNodeName, nodeSearchState)

			// Update details if items are selected
			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
			}

			if vm := a.vmList.GetSelectedVM(); vm != nil {
				a.vmDetails.Update(vm)
			}

			a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)
			a.header.ShowSuccess("Data refreshed successfully")
			a.footer.SetLoading(false)
			a.loadTasksData()
		})
	}()
}

// enrichGroupNodesSequentially enriches group node data in parallel and finalizes the refresh
func (a *App) enrichGroupNodesSequentially(nodes []*api.Node, hasSelectedNode bool, selectedNodeName string, hasSelectedVM bool, selectedVMID int, selectedVMNode string, searchWasActive bool, isInitialLoad bool) {
	go func() {
		var wg sync.WaitGroup

		// Create a context for the enrichment process
		ctx := context.Background()

		// Enrich nodes in parallel
		for i, node := range nodes {
			if node == nil || node.SourceProfile == "" {
				continue
			}

			wg.Add(1)
			go func(idx int, n *api.Node) {
				defer wg.Done()

				// We need to fetch the node status from the specific profile
				freshNode, err := a.groupManager.GetNodeFromGroup(ctx, n.SourceProfile, n.Name)

				if err == nil && freshNode != nil {
					// Ensure Online status is set to true if we got a response
					freshNode.Online = true

					// Preserve VMs from the existing node list
					if nodes[idx] != nil {
						freshNode.VMs = nodes[idx].VMs
						// Preserve IP if missing in freshNode
						if freshNode.IP == "" {
							freshNode.IP = nodes[idx].IP
						}
						// Preserve ID if missing in freshNode
						if freshNode.ID == "" {
							freshNode.ID = nodes[idx].ID
						}
						// Preserve Storage if missing in freshNode
						if freshNode.Storage == nil && nodes[idx].Storage != nil {
							freshNode.Storage = nodes[idx].Storage
						}
					}

					// Ensure SourceProfile is preserved
					freshNode.SourceProfile = n.SourceProfile

					// Update GlobalState
					if idx < len(models.GlobalState.OriginalNodes) {
						models.GlobalState.OriginalNodes[idx] = freshNode
					}
				}
			}(i, node)
		}

		wg.Wait()

		// Final update
		a.QueueUpdateDraw(func() {
			// Re-apply node filters
			nodeState := models.GlobalState.GetSearchState(api.PageNodes)
			if nodeState != nil && nodeState.Filter != "" {
				models.FilterNodes(nodeState.Filter)
			} else {
				models.GlobalState.FilteredNodes = make([]*api.Node, len(models.GlobalState.OriginalNodes))
				copy(models.GlobalState.FilteredNodes, models.GlobalState.OriginalNodes)
			}
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)

			// Update cluster status with the enriched nodes
			syntheticCluster := a.createSyntheticGroup(models.GlobalState.OriginalNodes)
			a.clusterStatus.Update(syntheticCluster)

			// Final selection restore and search UI restoration
			nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

			a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
				hasSelectedNode, selectedNodeName, nodeSearchState)

			// Update details if items are selected
			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
			}

			if vm := a.vmList.GetSelectedVM(); vm != nil {
				a.vmDetails.Update(vm)
			}

			a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)
			// Update lists and cluster status after enrichment to minimize flicker
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			a.clusterStatus.Update(a.getDisplayCluster())

			// Show appropriate success message based on context
			if isInitialLoad {
				a.header.ShowSuccess("Guest agent data loaded")
			} else {
				a.header.ShowSuccess("Data refreshed successfully")
			}
			a.footer.SetLoading(false)
			a.loadTasksData()
		})
	}()
}

// refreshNodeData refreshes data for a specific node and updates the UI.
func (a *App) refreshNodeData(node *api.Node) {
	a.header.ShowLoading(fmt.Sprintf("Refreshing node %s", node.Name))
	// Record the currently selected node's name
	selectedNodeName := ""
	if selected := a.nodeList.GetSelectedNode(); selected != nil {
		selectedNodeName = selected.Name
	}

	go func() {
		var freshNode *api.Node
		var err error

		client, clientErr := a.getClientForNode(node)
		if clientErr != nil {
			err = clientErr
		} else {
			freshNode, err = client.RefreshNodeData(node.Name)
		}

		a.QueueUpdateDraw(func() {
			if err != nil {
				a.header.ShowError(fmt.Sprintf("Error refreshing node %s: %v", node.Name, err))

				return
			}
			// Update node in global state
			for i, n := range models.GlobalState.OriginalNodes {
				if n != nil && n.Name == node.Name {
					// In group mode, ensure SourceProfile is preserved/set
					if a.isGroupMode {
						freshNode.SourceProfile = node.SourceProfile
					}
					models.GlobalState.OriginalNodes[i] = freshNode
					break
				}
			}

			for i, n := range models.GlobalState.FilteredNodes {
				if n != nil && n.Name == node.Name {
					// In group mode, ensure SourceProfile is preserved/set
					if a.isGroupMode {
						freshNode.SourceProfile = node.SourceProfile
					}
					models.GlobalState.FilteredNodes[i] = freshNode
					break
				}
			}

			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			a.nodeDetails.Update(freshNode, models.GlobalState.OriginalNodes)
			// Restore selection by previously selected node name using the tview list data
			restored := false

			nodeList := a.nodeList.GetNodes()
			for i, n := range nodeList {
				if n != nil && n.Name == selectedNodeName {
					a.nodeList.SetCurrentItem(i)
					// Manually trigger the node changed callback to update details
					if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
						a.nodeDetails.Update(selectedNode, models.GlobalState.OriginalNodes)
					}

					restored = true

					break
				}
			}

			if !restored && len(nodeList) > 0 {
				a.nodeList.SetCurrentItem(0)
				// Manually trigger the node changed callback to update details
				if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
					a.nodeDetails.Update(selectedNode, models.GlobalState.OriginalNodes)
				}
			}

			a.header.ShowSuccess(fmt.Sprintf("Node %s refreshed successfully", node.Name))
		})
	}()
}

// loadTasksData loads and updates task data with proper filtering.
func (a *App) loadTasksData() {
	go func() {
		var tasks []*api.ClusterTask
		var err error

		if a.isGroupMode {
			// Create context with timeout for group operations
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tasks, err = a.groupManager.GetGroupTasks(ctx)
		} else {
			tasks, err = a.client.GetClusterTasks()
		}

		if err == nil {
			a.QueueUpdateDraw(func() {
				// Update global state with tasks
				models.GlobalState.OriginalTasks = make([]*api.ClusterTask, len(tasks))
				models.GlobalState.FilteredTasks = make([]*api.ClusterTask, len(tasks))
				copy(models.GlobalState.OriginalTasks, tasks)
				copy(models.GlobalState.FilteredTasks, tasks)

				// Check for existing search filters
				taskSearchState := models.GlobalState.GetSearchState(api.PageTasks)
				if taskSearchState != nil && taskSearchState.Filter != "" {
					// Apply existing filter
					models.FilterTasks(taskSearchState.Filter)
					a.tasksList.SetFilteredTasks(models.GlobalState.FilteredTasks)
				} else {
					// No filter, use original data
					a.tasksList.SetTasks(tasks)
				}
			})
		}
	}()
}
