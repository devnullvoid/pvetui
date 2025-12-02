package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// updateVMListWithSelectionPreservation updates the VM list while preserving the currently selected VM.
func (a *App) updateVMListWithSelectionPreservation() {
	// Store current selection
	var selectedVMID int
	var selectedVMNode string
	var hasSelectedVM bool

	if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
		selectedVMID = selectedVM.ID
		selectedVMNode = selectedVM.Node
		hasSelectedVM = true
	}

	// Update the VM list
	a.vmList.SetVMs(models.GlobalState.FilteredVMs)

	// Restore selection if we had one
	if hasSelectedVM {
		vmList := a.vmList.GetVMs()
		for i, vm := range vmList {
			if vm != nil && vm.ID == selectedVMID && vm.Node == selectedVMNode {
				a.vmList.SetCurrentItem(i)
				// Manually trigger the VM changed callback to update details
				if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
					a.vmDetails.Update(selectedVM)
				}
				break
			}
		}
	}
}

// performVMOperation performs an asynchronous VM operation and shows status message.
func (a *App) performVMOperation(vm *api.VM, operation func(*api.VM) error, operationName string) {
	models.GlobalState.SetVMPending(vm, operationName)

	go func() {
		time.Sleep(50 * time.Millisecond)
		a.QueueUpdateDraw(func() {
			a.updateVMListWithSelectionPreservation()
		})
	}()
	a.header.ShowLoading(fmt.Sprintf("%s %s", operationName, vm.Name))

	var originalUptime int64 = -1

	if op := strings.ToLower(operationName); op == "restarting" {
		client, err := a.getClientForVM(vm)
		if err == nil {
			freshVM, err := client.RefreshVMData(vm, nil)
			if err == nil {
				originalUptime = freshVM.Uptime
			}
		}
	}

	go func() {
		if err := operation(vm); err != nil {
			models.GlobalState.ClearVMPending(vm)
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error %s %s: %v", strings.ToLower(operationName), vm.Name, err))
				a.updateVMListWithSelectionPreservation()
			})

			return
		}

		op := strings.ToLower(operationName)
		if op == "resetting" {
			// Reset is instantaneous and may not change reported uptime; do a short refresh delay instead of waiting
			time.Sleep(3 * time.Second)
		} else {
			a.QueueUpdateDraw(func() {
				a.header.ShowLoading(fmt.Sprintf("Waiting for %s %s to complete...", op, vm.Name))
			})
			if op == "restarting" {
				a.waitForVMRestartCompletionWithRefresh(vm, originalUptime)
			} else {
				a.waitForVMOperationCompletionWithRefresh(vm, operationName)
			}
		}

		a.QueueUpdateDraw(func() {
			a.header.ShowSuccess(fmt.Sprintf("%s %s completed successfully", operationName, vm.Name))
		})

		// Clear pending state immediately after operation completes
		models.GlobalState.ClearVMPending(vm)

		// Small delay before refresh to let success message show
		time.Sleep(500 * time.Millisecond)

		a.QueueUpdateDraw(func() {
			a.updateVMListWithSelectionPreservation()
			a.refreshVMData(vm)
			// Also refresh tasks to show any new tasks created by the operation
			a.loadTasksData()
		})
	}()
}

// performVMDeleteOperation performs an asynchronous VM delete operation and refreshes the VM list.
func (a *App) performVMDeleteOperation(vm *api.VM, forced bool) {
	models.GlobalState.SetVMPending(vm, "Deleting")

	go func() {
		time.Sleep(50 * time.Millisecond)
		a.QueueUpdateDraw(func() {
			a.updateVMListWithSelectionPreservation()
		})
	}()

	a.header.ShowLoading(fmt.Sprintf("Deleting %s", vm.Name))

	go func() {
		var err error
		var client *api.Client

		client, err = a.getClientForVM(vm)
		if err == nil {
			if forced {
				options := &api.DeleteVMOptions{
					Force:                    true,
					DestroyUnreferencedDisks: true,
					Purge:                    true,
				}
				err = client.DeleteVMWithOptions(vm, options)
			} else {
				err = client.DeleteVM(vm)
			}
		}

		if err != nil {
			// * Clear pending state on error
			models.GlobalState.ClearVMPending(vm)
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error deleting %s: %v", vm.Name, err))
				a.updateVMListWithSelectionPreservation()
			})
		} else {
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Successfully deleted %s", vm.Name))
			})

			// Clear pending state immediately after successful deletion
			models.GlobalState.ClearVMPending(vm)

			// Clear API cache for the specific client
			if client != nil {
				client.ClearAPICache()
			}

			// Small delay before refresh to let success message show
			time.Sleep(500 * time.Millisecond)

			// Refresh to update the UI (VM should be gone)
			a.QueueUpdateDraw(func() {
				a.manualRefresh()
			})
		}
	}()
}

// showDeleteRunningVMDialog shows a dialog with options for deleting a running VM.
func (a *App) showDeleteRunningVMDialog(vm *api.VM) {
	message := fmt.Sprintf("⚠️  VM '%s' (ID: %d) is currently RUNNING\n\nProxmox can force delete running VMs.\n\nAre you sure you want to FORCE DELETE this running VM?\n\nThis will IMMEDIATELY DESTROY the VM and ALL its data!", vm.Name, vm.ID)
	a.showConfirmationDialog(message, func() {
		a.performVMDeleteOperation(vm, true)
	})
}

// waitForVMRestartCompletionWithRefresh waits for a VM to complete a restart by polling with RefreshVMData.
func (a *App) waitForVMRestartCompletionWithRefresh(vm *api.VM, originalUptime int64) {
	const maxWait = 2 * time.Minute

	const pollInterval = 2 * time.Second

	client, err := a.getClientForVM(vm)
	if err != nil {
		return
	}

	start := time.Now()
	for time.Since(start) < maxWait {
		freshVM, err := client.RefreshVMData(vm, nil)
		if err == nil && freshVM != nil && freshVM.Uptime > 0 && freshVM.Uptime < originalUptime-10 {
			break
		}

		time.Sleep(pollInterval)
	}
}

// waitForVMOperationCompletionWithRefresh waits for a VM operation (start, stop, etc.) to complete by polling with RefreshVMData.
func (a *App) waitForVMOperationCompletionWithRefresh(vm *api.VM, operationName string) {
	const maxWait = 2 * time.Minute

	const pollInterval = 2 * time.Second

	client, err := a.getClientForVM(vm)
	if err != nil {
		return
	}

	start := time.Now()
	for time.Since(start) < maxWait {
		freshVM, err := client.RefreshVMData(vm, nil)
		if err == nil && freshVM != nil {
			if strings.ToLower(operationName) == "stopping" && freshVM.Status != api.VMStatusRunning {
				break
			} else if strings.ToLower(operationName) == "shutting down" && freshVM.Status != api.VMStatusRunning {
				break
			} else if strings.ToLower(operationName) == "starting" && freshVM.Status == api.VMStatusRunning {
				break
			}
		}

		time.Sleep(pollInterval)
	}
}
