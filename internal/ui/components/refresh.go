package components

import (
	"fmt"
	"strings"
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

		// Fetch fresh data bypassing cache
		cluster, err := a.client.GetFreshClusterStatus()
		if err != nil {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
			})

			return
		}

		// Initial UI update and enrichment
		a.applyInitialClusterUpdate(cluster)
		a.enrichNodesSequentially(cluster, hasSelectedNode, selectedNodeName, hasSelectedVM, selectedVMID, selectedVMNode, searchWasActive)
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

// enrichNodesSequentially enriches node data one-by-one and finalizes the refresh
func (a *App) enrichNodesSequentially(cluster *api.Cluster, hasSelectedNode bool, selectedNodeName string, hasSelectedVM bool, selectedVMID int, selectedVMNode string, searchWasActive bool) {
	go func() {
		// Collect current node filter to avoid repeated lookups
		nodeState := models.GlobalState.GetSearchState(api.PageNodes)
		activeFilter := ""
		if nodeState != nil {
			activeFilter = nodeState.Filter
		}

		// Enrich nodes incrementally with minimal UI updates
		for i, node := range cluster.Nodes {
			if node == nil {
				continue
			}

			freshNode, err := a.client.RefreshNodeData(node.Name)
			if err == nil && freshNode != nil {
				// Preserve VMs from the FRESH cluster data (not the original stale data)
				// This ensures we keep the updated VM names we just fetched
				if cluster.Nodes[i] != nil {
					freshNode.VMs = cluster.Nodes[i].VMs
				}

				// Update only the specific node index in global state
				models.GlobalState.OriginalNodes[i] = freshNode

				// Update filtered list only if this node matches current filter
				shouldUpdateFiltered := false
				if activeFilter == "" {
					// No filter active, always update
					models.GlobalState.FilteredNodes[i] = freshNode
					shouldUpdateFiltered = true
				} else {
					// Check if node matches filter before updating filtered list
					if a.nodeMatchesFilter(freshNode, activeFilter) {
						models.FilterNodes(activeFilter) // Re-apply filter efficiently
						shouldUpdateFiltered = true
					}
				}

				// Only update UI if filtered list changed or this is the selected node
				selected := a.nodeList.GetSelectedNode()
				if shouldUpdateFiltered || (selected != nil && selected.Name == freshNode.Name) {
					a.QueueUpdateDraw(func() {
						if shouldUpdateFiltered {
							a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
						}
						// Update details if this is the currently selected node
						if selected != nil && selected.Name == freshNode.Name {
							a.nodeDetails.Update(freshNode, models.GlobalState.OriginalNodes)
						}
					})
				}
			}
		}

		// Final update: rebuild VMs, cluster version, status, and complete refresh
		a.QueueUpdateDraw(func() {
			// Rebuild VM list from enriched nodes (which now preserve VMs from FRESH cluster data)
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

			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
			}

			a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)
			a.header.ShowSuccess("Data refreshed successfully")
			a.footer.SetLoading(false)
			a.loadTasksData()
		})
	}()
}

// nodeMatchesFilter checks if a node matches the given filter string
func (a *App) nodeMatchesFilter(node *api.Node, filter string) bool {
	if filter == "" || node == nil {
		return true
	}

	filter = strings.ToLower(filter)

	// Check node name
	if strings.Contains(strings.ToLower(node.Name), filter) {
		return true
	}

	// Check node IP
	if strings.Contains(strings.ToLower(node.IP), filter) {
		return true
	}

	// Check node status
	statusText := "offline"
	if node.Online {
		statusText = "online"
	}
	if strings.Contains(statusText, filter) {
		return true
	}

	return false
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
		freshNode, err := a.client.RefreshNodeData(node.Name)
		a.QueueUpdateDraw(func() {
			if err != nil {
				a.header.ShowError(fmt.Sprintf("Error refreshing node %s: %v", node.Name, err))

				return
			}
			// Update node in global state
			for i, n := range models.GlobalState.OriginalNodes {
				if n != nil && n.Name == node.Name {
					models.GlobalState.OriginalNodes[i] = freshNode

					break
				}
			}

			for i, n := range models.GlobalState.FilteredNodes {
				if n != nil && n.Name == node.Name {
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
		tasks, err := a.client.GetClusterTasks()
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
