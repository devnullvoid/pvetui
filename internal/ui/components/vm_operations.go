package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// performVMOperation performs an asynchronous VM operation and shows status message.
func (a *App) performVMOperation(vm *api.VM, operation func(*api.VM) error, operationName string) {
	models.GlobalState.SetVMPending(vm, operationName)

	go func() {
		time.Sleep(50 * time.Millisecond)
		a.QueueUpdateDraw(func() {
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		})
	}()
	a.header.ShowLoading(fmt.Sprintf("%s %s", operationName, vm.Name))

	var originalUptime int64 = -1

	if strings.ToLower(operationName) == "restarting" {
		freshVM, err := a.client.RefreshVMData(vm, nil)
		if err == nil {
			originalUptime = freshVM.Uptime
		}
	}

	go func() {
		defer func() {
			models.GlobalState.ClearVMPending(vm)
			a.QueueUpdateDraw(func() {
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			})
		}()

		if err := operation(vm); err != nil {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error %s %s: %v", strings.ToLower(operationName), vm.Name, err))
			})

			return
		}

		a.QueueUpdateDraw(func() {
			a.header.ShowLoading(fmt.Sprintf("Waiting for %s %s to complete...", strings.ToLower(operationName), vm.Name))
		})

		if strings.ToLower(operationName) == "restarting" {
			a.waitForVMRestartCompletionWithRefresh(vm, originalUptime)
		} else {
			a.waitForVMOperationCompletionWithRefresh(vm, operationName)
		}

		a.QueueUpdateDraw(func() {
			a.header.ShowSuccess(fmt.Sprintf("%s %s completed successfully", operationName, vm.Name))
		})
		time.Sleep(2 * time.Second)
		a.QueueUpdateDraw(func() {
			a.refreshVMData(vm)
		})
	}()
}

// performVMDeleteOperation performs an asynchronous VM delete operation and refreshes the VM list.
func (a *App) performVMDeleteOperation(vm *api.VM, forced bool) {
	models.GlobalState.SetVMPending(vm, "Deleting")

	go func() {
		time.Sleep(50 * time.Millisecond)
		a.QueueUpdateDraw(func() {
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		})
	}()

	a.header.ShowLoading(fmt.Sprintf("Deleting %s", vm.Name))

	go func() {
		defer func() {
			models.GlobalState.ClearVMPending(vm)
			a.QueueUpdateDraw(func() {
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			})
		}()

		var err error

		if forced {
			options := &api.DeleteVMOptions{
				Force:                    true,
				DestroyUnreferencedDisks: true,
				Purge:                    true,
			}
			err = a.client.DeleteVMWithOptions(vm, options)
		} else {
			err = a.client.DeleteVM(vm)
		}

		if err != nil {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error deleting %s: %v", vm.Name, err))
			})
		} else {
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Successfully deleted %s", vm.Name))
			})
			a.client.ClearAPICache()

			go func() {
				time.Sleep(5 * time.Second)
				a.QueueUpdateDraw(func() {
					a.manualRefresh()
				})
			}()
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

	start := time.Now()
	for time.Since(start) < maxWait {
		freshVM, err := a.client.RefreshVMData(vm, nil)
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

	start := time.Now()
	for time.Since(start) < maxWait {
		freshVM, err := a.client.RefreshVMData(vm, nil)
		if err == nil && freshVM != nil {
			if strings.ToLower(operationName) == "stopping" && freshVM.Status != api.VMStatusRunning {
				break
			}

			if strings.ToLower(operationName) == "starting" && freshVM.Status == api.VMStatusRunning {
				break
			}
		}

		time.Sleep(pollInterval)
	}
}
