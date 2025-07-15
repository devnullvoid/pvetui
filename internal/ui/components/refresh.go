package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// manualRefresh refreshes all data manually
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

		// Update UI with new data
		a.QueueUpdateDraw(func() {
			// Get current search states
			nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

			// Update component data
			a.clusterStatus.Update(cluster)

			// Rebuild VM list from fresh cluster data
			var vms []*api.VM
			for _, node := range cluster.Nodes {
				if node != nil {
					for _, vm := range node.VMs {
						if vm != nil {
							vms = append(vms, vm)
						}
					}
				}
			}

			// Update global state with fresh data
			models.GlobalState.OriginalNodes = make([]*api.Node, len(cluster.Nodes))
			models.GlobalState.FilteredNodes = make([]*api.Node, len(cluster.Nodes))
			models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
			models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))

			copy(models.GlobalState.OriginalNodes, cluster.Nodes)
			copy(models.GlobalState.FilteredNodes, cluster.Nodes)
			copy(models.GlobalState.OriginalVMs, vms)
			copy(models.GlobalState.FilteredVMs, vms)

			// Apply filters if active, otherwise use all data
			if nodeSearchState != nil && nodeSearchState.Filter != "" {
				// Re-filter with the current search term
				models.FilterNodes(nodeSearchState.Filter)
				a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			} else {
				// No filter active, use all nodes
				a.nodeList.SetNodes(models.GlobalState.OriginalNodes)
			}

			// Same approach for VMs
			if vmSearchState != nil && vmSearchState.Filter != "" {
				// Re-filter with the current search term
				models.FilterVMs(vmSearchState.Filter)
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			} else {
				// No filter active, use all VMs
				a.vmList.SetVMs(models.GlobalState.OriginalVMs)
			}

			a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
				hasSelectedNode, selectedNodeName, nodeSearchState)

			// Update details if items are selected
			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, cluster.Nodes)
			}

			if vm := a.vmList.GetSelectedVM(); vm != nil {
				a.vmDetails.Update(vm)
			}

			// Refresh tasks as well
			go func() {
				tasks, err := a.client.GetClusterTasks()
				if err == nil {
					a.QueueUpdateDraw(func() {
						// Update tasks in global state
						models.GlobalState.OriginalTasks = make([]*api.ClusterTask, len(tasks))
						models.GlobalState.FilteredTasks = make([]*api.ClusterTask, len(tasks))
						copy(models.GlobalState.OriginalTasks, tasks)
						copy(models.GlobalState.FilteredTasks, tasks)

						// Apply task search filter if active
						taskSearchState := models.GlobalState.GetSearchState(api.PageTasks)
						if taskSearchState != nil && taskSearchState.Filter != "" {
							models.FilterTasks(taskSearchState.Filter)
							a.tasksList.SetFilteredTasks(models.GlobalState.FilteredTasks)
						} else {
							a.tasksList.SetTasks(tasks)
						}
					})
				}
			}()

			// Restore search input UI state if it was active before refresh
			a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)

			// Show success message
			a.header.ShowSuccess("Data refreshed successfully")
			a.footer.SetLoading(false)
		})
	}()
}

// refreshVMData refreshes data for the selected VM
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

// refreshNodeData refreshes data for a specific node and updates the UI
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
