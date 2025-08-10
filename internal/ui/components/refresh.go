package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// manualRefresh refreshes all data manually.
func (a *App) manualRefresh() {
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
		// Fetch fresh data bypassing cache
		cluster, err := a.client.GetFreshClusterStatus()
		if err != nil {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
			})

			return
		}

		// Immediately update UI with basic cluster data
		models.GlobalState.OriginalNodes = make([]*api.Node, len(cluster.Nodes))
		models.GlobalState.FilteredNodes = make([]*api.Node, len(cluster.Nodes))
		copy(models.GlobalState.OriginalNodes, cluster.Nodes)
		copy(models.GlobalState.FilteredNodes, cluster.Nodes)
		a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
		// Rebuild VM list from fresh cluster data so new guests appear immediately
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
		models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
		copy(models.GlobalState.OriginalVMs, vms)
		copy(models.GlobalState.FilteredVMs, vms)
		a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		a.clusterStatus.Update(cluster)

		// Start sequential enrichment in a goroutine (one node at a time, UI updated after each)
		go func() {
			enrichedNodes := make([]*api.Node, len(cluster.Nodes))
			copy(enrichedNodes, cluster.Nodes)

			for i, node := range cluster.Nodes {
				if node == nil {
					enrichedNodes[i] = nil

					continue
				}

				freshNode, err := a.client.RefreshNodeData(node.Name)
				if err == nil && freshNode != nil {
					enrichedNodes[i] = freshNode
				} else {
					enrichedNodes[i] = node
				}

				a.QueueUpdateDraw(func() {
					models.GlobalState.OriginalNodes = make([]*api.Node, len(enrichedNodes))
					models.GlobalState.FilteredNodes = make([]*api.Node, len(enrichedNodes))
					copy(models.GlobalState.OriginalNodes, enrichedNodes)
					copy(models.GlobalState.FilteredNodes, enrichedNodes)
					a.nodeList.SetNodes(models.GlobalState.FilteredNodes)

					for _, n := range enrichedNodes {
						if n != nil && n.Version != "" {
							cluster.Version = fmt.Sprintf("Proxmox VE %s", n.Version)

							break
						}
					}

					a.clusterStatus.Update(cluster)
					// Keep selection stable during refresh
					nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
					vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
					a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
						hasSelectedNode, selectedNodeName, nodeSearchState)
					// Only update node details if the enriched node is the selected node
					selected := a.nodeList.GetSelectedNode()
					if selected != nil && enrichedNodes[i] != nil && selected.Name == enrichedNodes[i].Name {
						a.nodeDetails.Update(enrichedNodes[i], enrichedNodes)
					}
				})
			}
			// Final UI update and rest of refresh logic
			a.QueueUpdateDraw(func() {
				// Apply filters if active, otherwise use all data
				nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
				vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

				if nodeSearchState != nil && nodeSearchState.Filter != "" {
					models.FilterNodes(nodeSearchState.Filter)
					a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
				} else {
					a.nodeList.SetNodes(models.GlobalState.OriginalNodes)
				}

				if vmSearchState != nil && vmSearchState.Filter != "" {
					models.FilterVMs(vmSearchState.Filter)
					a.vmList.SetVMs(models.GlobalState.FilteredVMs)
				} else {
					a.vmList.SetVMs(models.GlobalState.OriginalVMs)
				}

				a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
					hasSelectedNode, selectedNodeName, nodeSearchState)

				if node := a.nodeList.GetSelectedNode(); node != nil {
					a.nodeDetails.Update(node, enrichedNodes)
				}

				if vm := a.vmList.GetSelectedVM(); vm != nil {
					a.vmDetails.Update(vm)
				}

				a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)
				a.header.ShowSuccess("Data refreshed successfully")
				a.footer.SetLoading(false)
				// Refresh tasks as well
				a.loadTasksData()
			})
		}()
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
