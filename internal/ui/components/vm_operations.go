package components

import (
	"fmt"
	"strings"

	"github.com/devnullvoid/pvetui/internal/taskmanager"
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

// performVMOperation performs an asynchronous VM operation via the TaskManager.
func (a *App) performVMOperation(vm *api.VM, operation func(*api.VM) (string, error), operationName string) {
	// Create task
	task := &taskmanager.Task{
		Type:        operationName,
		Description: fmt.Sprintf("%s %s (%d)", operationName, vm.Name, vm.ID),
		TargetVMID:  vm.ID,
		TargetNode:  vm.Node,
		TargetName:  vm.Name,
		Operation: func() (string, error) {
			return operation(vm)
		},
		OnComplete: func(err error) {
			a.QueueUpdateDraw(func() {
				if err != nil {
					a.header.ShowError(fmt.Sprintf("Error %s %s: %v", strings.ToLower(operationName), vm.Name, err))
				} else {
					a.header.ShowSuccess(fmt.Sprintf("%s %s completed successfully", operationName, vm.Name))
					// Trigger refresh of VM data to update status
					a.refreshVMData(vm)
					// Refresh tasks history
					a.loadTasksData()
				}
			})
		},
	}

	a.taskManager.Enqueue(task)
	a.header.ShowSuccess(fmt.Sprintf("Queued %s for %s", operationName, vm.Name))
}

// performVMDeleteOperation performs an asynchronous VM delete operation via the TaskManager.
func (a *App) performVMDeleteOperation(vm *api.VM, forced bool) {
	task := &taskmanager.Task{
		Type:        "Delete",
		Description: fmt.Sprintf("Delete %s (%d)", vm.Name, vm.ID),
		TargetVMID:  vm.ID,
		TargetNode:  vm.Node,
		TargetName:  vm.Name,
		Operation: func() (string, error) {
			client, err := a.getClientForVM(vm)
			if err != nil {
				return "", err
			}

			if forced {
				options := &api.DeleteVMOptions{
					Force:                    true,
					DestroyUnreferencedDisks: true,
					Purge:                    true,
				}
				return client.DeleteVMWithOptions(vm, options)
			}
			return client.DeleteVM(vm)
		},
		OnComplete: func(err error) {
			a.QueueUpdateDraw(func() {
				if err != nil {
					a.header.ShowError(fmt.Sprintf("Error deleting %s: %v", vm.Name, err))
				} else {
					a.header.ShowSuccess(fmt.Sprintf("Successfully deleted %s", vm.Name))

					// Clear API cache for the specific client
					client, _ := a.getClientForVM(vm)
					if client != nil {
						client.ClearAPICache()
					}

					// Refresh to update the UI (VM should be gone)
					a.manualRefresh()
				}
			})
		},
	}

	a.taskManager.Enqueue(task)
	a.header.ShowSuccess(fmt.Sprintf("Queued deletion for %s", vm.Name))
}

// showDeleteRunningVMDialog shows a dialog with options for deleting a running VM.
func (a *App) showDeleteRunningVMDialog(vm *api.VM) {
	message := fmt.Sprintf("⚠️  VM '%s' (ID: %d) is currently RUNNING\n\nProxmox can force delete running VMs.\n\nAre you sure you want to FORCE DELETE this running VM?\n\nThis will IMMEDIATELY DESTROY the VM and ALL its data!", vm.Name, vm.ID)
	a.showConfirmationDialog(message, func() {
		a.performVMDeleteOperation(vm, true)
	})
}
