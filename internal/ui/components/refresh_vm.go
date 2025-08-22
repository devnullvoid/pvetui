package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

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

// refreshVMDataAndTasks refreshes both VM data and tasks list.
// This is useful for operations that affect both VM state and create tasks (like volume resize and snapshot rollback).
func (a *App) refreshVMDataAndTasks(vm *api.VM) {
	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Refreshing VM %s and tasks", vm.Name))

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

			// Also refresh tasks to show any new tasks created by the operation
			a.loadTasksData()

			// Show success message
			a.header.ShowSuccess(fmt.Sprintf("VM %s and tasks refreshed successfully", vm.Name))
		})
	}()
}
