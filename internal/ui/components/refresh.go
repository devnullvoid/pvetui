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

// refreshVMData refreshes data for the selected VM.
func (a *App) refreshVMData(vm *api.VM) {
	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Refreshing VM %s", vm.Name))

	// Store VM identity for selection restoration
	vmID := vm.ID
	vmNode := vm.Node

	// Run refresh in goroutine to avoid blocking UI
	go func() {
		// Fetch fresh VM data with callback for when enrichment completes
		freshVM, err := a.client.RefreshVMData(vm, func(enrichedVM *api.VM) {
			// This callback is called after guest agent data has been loaded
			a.QueueUpdateDraw(func() {
				// Update VM details if this VM is currently selected
				if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil && selectedVM.ID == enrichedVM.ID && selectedVM.Node == enrichedVM.Node {
					a.vmDetails.Update(enrichedVM)
				}
			})
		})
		if err != nil {
			// If VM refresh fails (e.g., VM was migrated to a different node),
			// fall back to a full refresh to find the VM on its new node
			a.QueueUpdateDraw(func() {
				a.header.ShowLoading("VM may have been migrated, performing full refresh")
			})
			a.QueueUpdateDraw(func() {
				a.manualRefresh() // This will find the VM on its new node
			})

			return
		}

		// Update UI with fresh data on main thread
		a.QueueUpdateDraw(func() {
			// Get current search state
			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

			// Find the VM in the global state and update it
			for i, originalVM := range models.GlobalState.OriginalVMs {
				if originalVM != nil && originalVM.ID == vmID && originalVM.Node == vmNode {
					models.GlobalState.OriginalVMs[i] = freshVM

					break
				}
			}

			// Update filtered VMs if they exist
			for i, filteredVM := range models.GlobalState.FilteredVMs {
				if filteredVM != nil && filteredVM.ID == vmID && filteredVM.Node == vmNode {
					models.GlobalState.FilteredVMs[i] = freshVM

					break
				}
			}

			// Also update the VM in the node's VM list
			for _, node := range models.GlobalState.OriginalNodes {
				if node != nil && node.Name == vmNode {
					for i, nodeVM := range node.VMs {
						if nodeVM != nil && nodeVM.ID == vmID {
							node.VMs[i] = freshVM

							break
						}
					}

					break
				}
			}

			// Update the VM list display
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)

			// Find and select the refreshed VM by ID and node in the widget's list
			vmList := a.vmList.GetVMs()
			for i, refreshedVM := range vmList {
				if refreshedVM != nil && refreshedVM.ID == vmID && refreshedVM.Node == vmNode {
					a.vmList.SetCurrentItem(i)

					if vmSearchState != nil {
						vmSearchState.SelectedIndex = i
					}

					break
				}
			}

			// Update VM details if this VM is currently selected
			selectedVM := a.vmList.GetSelectedVM()
			if selectedVM != nil && selectedVM.ID == vmID && selectedVM.Node == vmNode {
				a.vmDetails.Update(freshVM)
			}

			// Show success message
			a.header.ShowSuccess(fmt.Sprintf("VM %s refreshed successfully", vm.Name))
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

					restored = true

					break
				}
			}

			if !restored && len(nodeList) > 0 {
				a.nodeList.SetCurrentItem(0)
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
