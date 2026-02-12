package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// refreshVMData refreshes data for the selected VM.
func (a *App) refreshVMData(vm *api.VM) {
	// * Check if VM has pending operations
	if isPending, pendingOperation := models.GlobalState.IsVMPending(vm); isPending {
		a.showMessageSafe(fmt.Sprintf("Cannot refresh VM while '%s' is in progress", pendingOperation))
		return
	}

	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Refreshing VM %s", vm.Name))

	// Store VM identity for selection restoration
	vmID := vm.ID
	vmNode := vm.Node

	// Run refresh in goroutine to avoid blocking UI
	go func() {
		// Get the correct client for this VM (important for group mode)
		client, err := a.getClientForVM(vm)
		if err != nil {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error refreshing VM: %v", err))
			})
			return
		}

		// Fetch fresh VM data with callback for when enrichment completes
		freshVM, err := client.RefreshVMData(vm, func(enrichedVM *api.VM) {
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
					// Preserve SourceProfile from original VM (important for group mode)
					if originalVM.SourceProfile != "" {
						freshVM.SourceProfile = originalVM.SourceProfile
					}
					models.GlobalState.OriginalVMs[i] = freshVM

					break
				}
			}

			// Update filtered VMs if they exist
			for i, filteredVM := range models.GlobalState.FilteredVMs {
				if filteredVM != nil && filteredVM.ID == vmID && filteredVM.Node == vmNode {
					// Preserve SourceProfile from filtered VM (important for group mode)
					if filteredVM.SourceProfile != "" {
						freshVM.SourceProfile = filteredVM.SourceProfile
					}
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

			// Reapply filter if one is active
			if vmSearchState != nil && vmSearchState.Filter != "" {
				models.FilterVMs(vmSearchState.Filter)
			}

			// Update the VM list display
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)

			// Preserve whichever VM the user is currently focused on (selection is restored by SetVMs).
			if vmSearchState != nil {
				vmSearchState.SelectedIndex = a.vmList.GetCurrentItem()
			}

			// Update VM details for the currently selected VM.
			if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
				a.vmDetails.Update(selectedVM)
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
		// Get the correct client for this VM (important for group mode)
		client, err := a.getClientForVM(vm)
		if err != nil {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error refreshing VM: %v", err))
			})
			return
		}

		// Fetch fresh VM data with callback for when enrichment completes
		freshVM, err := client.RefreshVMData(vm, func(enrichedVM *api.VM) {
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
					// Preserve SourceProfile from original VM (important for group mode)
					if originalVM.SourceProfile != "" {
						freshVM.SourceProfile = originalVM.SourceProfile
					}
					models.GlobalState.OriginalVMs[i] = freshVM

					break
				}
			}

			// Update filtered VMs if they exist
			for i, filteredVM := range models.GlobalState.FilteredVMs {
				if filteredVM != nil && filteredVM.ID == vmID && filteredVM.Node == vmNode {
					// Preserve SourceProfile from filtered VM (important for group mode)
					if filteredVM.SourceProfile != "" {
						freshVM.SourceProfile = filteredVM.SourceProfile
					}
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

			// Reapply filter if one is active
			if vmSearchState != nil && vmSearchState.Filter != "" {
				models.FilterVMs(vmSearchState.Filter)
			}

			// Update the VM list display
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)

			// Preserve whichever VM the user is currently focused on (selection is restored by SetVMs).
			if vmSearchState != nil {
				vmSearchState.SelectedIndex = a.vmList.GetCurrentItem()
			}

			// Update VM details for the currently selected VM.
			if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
				a.vmDetails.Update(selectedVM)
			}

			// Also refresh tasks to show any new tasks created by the operation
			a.loadTasksData()

			// Show success message
			a.header.ShowSuccess(fmt.Sprintf("VM %s and tasks refreshed successfully", vm.Name))
		})
	}()
}
