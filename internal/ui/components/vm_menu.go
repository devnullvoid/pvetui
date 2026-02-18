package components

import (
	"fmt"
	"strings"

	"github.com/devnullvoid/pvetui/internal/taskmanager"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
)

// VM menu action constants
const (
	vmActionOpenShell  = "Open Shell"
	vmActionOpenVNC    = "Open VNC Console"
	vmActionEditConfig = "Edit Configuration"
	vmActionSnapshots  = "Manage Snapshots"
	vmActionBackups    = "Manage Backups"
	vmActionRefresh    = "Refresh"
	vmActionStart      = "Start"
	vmActionShutdown   = "Shutdown"
	vmActionStop       = "Stop (force)"
	vmActionRestart    = "Restart"
	vmActionReset      = "Reset (hard)"
	vmActionMigrate    = "Migrate"
	vmActionDelete     = "Delete"
	vmActionClearBatch = "Clear Selection"
)

// ShowVMContextMenu displays the context menu for VM actions.
func (a *App) ShowVMContextMenu() {
	if a.guestSelectionCount() > 1 {
		a.showBatchVMContextMenu()
		return
	}

	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		return
	}

	// Get the node for this VM
	node := a.vmList.GetNodeForVM(vm)

	// Check if this VM has a pending operation
	var isPending bool
	var pendingOperation string
	if a.taskManager != nil {
		if task := a.taskManager.GetActiveTaskForVM(vm.Node, vm.ID); task != nil {
			isPending = true
			pendingOperation = task.Type
			if task.Status == taskmanager.StatusQueued {
				pendingOperation += " (Queued)"
			}
		}
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on VM state
	menuItems := []string{
		vmActionOpenShell,
		vmActionEditConfig,
		vmActionSnapshots,
		vmActionBackups,
		vmActionRefresh,
	}

	if (vm.Type == api.VMTypeQemu || vm.Type == api.VMTypeLXC) && vm.Status == api.VMStatusRunning {
		menuItems = append(menuItems[:1], append([]string{vmActionOpenVNC}, menuItems[1:]...)...)
	}

	// Add plugin-contributed guest actions
	var pluginActions []GuestAction
	if a.pluginRegistry != nil && node != nil {
		pluginActions = a.pluginRegistry.GuestActionsForGuest(node, vm)
		for _, action := range pluginActions {
			menuItems = append(menuItems, action.Label)
		}
	}

	// Always show lifecycle actions, allowing queuing
	if vm.Status == api.VMStatusRunning {
		// When running, offer graceful Shutdown, force Stop, and Restart
		menuItems = append(menuItems, vmActionShutdown, vmActionStop, vmActionRestart)
		// Hard Reset is QEMU-only
		if vm.Type == api.VMTypeQemu {
			menuItems = append(menuItems, vmActionReset)
		}
	} else if vm.Status == api.VMStatusStopped {
		menuItems = append(menuItems, vmActionStart)
	}

	menuItems = append(menuItems, vmActionMigrate)
	menuItems = append(menuItems, vmActionDelete)

	if isPending {
		// * Show pending operation info in menu
		menuItems = append(menuItems, fmt.Sprintf("⚠️  %s in progress...", pendingOperation))
	}

	// Generate letter shortcuts based on menu items
	shortcuts := generateVMShortcuts(menuItems, pluginActions)

	menu := NewContextMenuWithShortcuts(" Guest Actions ", menuItems, shortcuts, func(index int, action string) {
		a.CloseContextMenu()

		// * Prevent actions if VM has pending operations
		// if isPending {
		// 	a.showMessageSafe(fmt.Sprintf("Cannot perform actions while '%s' is in progress", pendingOperation))
		// 	return
		// }

		// Check if this is a plugin action
		for _, pluginAction := range pluginActions {
			if action == pluginAction.Label {
				// Execute plugin action handler
				go func() {
					if err := pluginAction.Handler(a.ctx, a, node, vm); err != nil {
						a.QueueUpdateDraw(func() {
							a.showMessageSafe(fmt.Sprintf("Plugin action failed: %v", err))
						})
					}
				}()
				return
			}
		}

		switch action {
		case vmActionOpenShell:
			a.openVMShell()
		case vmActionOpenVNC:
			a.openVMVNC()
		case vmActionEditConfig:
			go func() {
				client, clientErr := a.getClientForVM(vm)
				if clientErr != nil {
					a.QueueUpdateDraw(func() {
						a.showMessageSafe(fmt.Sprintf("Failed to get client: %v", clientErr))
					})
					return
				}

				cfg, err := client.GetVMConfig(vm)
				a.QueueUpdateDraw(func() {
					if err != nil {
						a.showMessageSafe(fmt.Sprintf("Failed to load config: %v", err))

						return
					}

					page := NewVMConfigPage(a, vm, cfg, func(newCfg *api.VMConfig) error {
						// Re-fetch client in case it changed (unlikely but safe) or just use closure
						// Using closure 'client' is fine here
						return client.UpdateVMConfig(vm, newCfg)
					})
					a.pages.AddPage("vmConfig", page, true, true)
					a.SetFocus(page)
				})
			}()
		case vmActionSnapshots:
			go func() {
				a.QueueUpdateDraw(func() {
					snapshotManager := NewSnapshotManager(a, vm)
					a.pages.AddPage("snapshots", snapshotManager, true, true)
					a.SetFocus(snapshotManager)
				})
			}()
		case vmActionBackups:
			go func() {
				a.QueueUpdateDraw(func() {
					backupManager := NewBackupManager(a, vm)
					a.pages.AddPage("backups", backupManager, true, true)
					a.SetFocus(backupManager)
				})
			}()
		case vmActionRefresh:
			a.refreshVMData(vm)
		case vmActionStart:
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to start VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					client, err := a.getClientForVM(vm)
					if err != nil {
						a.showMessageSafe(fmt.Sprintf("Error: %v", err))
						return
					}
					a.performVMOperation(vm, client.StartVM, "Starting")
				},
			)
		case vmActionShutdown:
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to gracefully shut down '%s' (ID: %d)?\n\nThis requests an OS shutdown and may take time.", vm.Name, vm.ID),
				func() {
					client, err := a.getClientForVM(vm)
					if err != nil {
						a.showMessageSafe(fmt.Sprintf("Error: %v", err))
						return
					}
					a.performVMOperation(vm, client.ShutdownVM, "Shutting down")
				},
			)
		case vmActionStop:
			a.showConfirmationDialog(
				fmt.Sprintf("⚠️  Force stop '%s' (ID: %d)?\n\nThis is equivalent to power off and may cause data loss.", vm.Name, vm.ID),
				func() {
					client, err := a.getClientForVM(vm)
					if err != nil {
						a.showMessageSafe(fmt.Sprintf("Error: %v", err))
						return
					}
					a.performVMOperation(vm, client.StopVM, "Stopping")
				},
			)
		case vmActionRestart:
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to restart VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					client, err := a.getClientForVM(vm)
					if err != nil {
						a.showMessageSafe(fmt.Sprintf("Error: %v", err))
						return
					}
					a.performVMOperation(vm, client.RestartVM, "Restarting")
				},
			)
		case vmActionReset:
			if vm.Type == api.VMTypeQemu {
				a.showConfirmationDialog(
					fmt.Sprintf("⚠️  Hard reset '%s' (ID: %d)?\n\nThis is an immediate reset (like pressing reset) and may cause data loss.", vm.Name, vm.ID),
					func() {
						client, err := a.getClientForVM(vm)
						if err != nil {
							a.showMessageSafe(fmt.Sprintf("Error: %v", err))
							return
						}
						a.performVMOperation(vm, client.ResetVM, "Resetting")
					},
				)
			}
		case vmActionMigrate:
			a.showMigrationDialog(vm)
		case vmActionDelete:
			if vm.Status == api.VMStatusRunning {
				a.showDeleteRunningVMDialog(vm)
			} else {
				a.showConfirmationDialog(
					fmt.Sprintf("⚠️  DANGER: Are you sure you want to permanently DELETE VM '%s' (ID: %d)?\n\nThis action is IRREVERSIBLE and will destroy all VM data including disks!", vm.Name, vm.ID),
					func() {
						a.performVMDeleteOperation(vm, false)
					},
				)
			}
		}
	})
	menu.SetApp(a)

	menuList := menu.Show()

	// Add input capture to close menu on Escape or 'h'
	oldCapture := menuList.GetInputCapture()
	menuList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'h') {
			a.CloseContextMenu()

			return nil
		}

		if oldCapture != nil {
			return oldCapture(event)
		}

		return event
	})

	// * Update menu title to show pending status if applicable
	menuTitle := " Guest Actions "
	if isPending {
		menuTitle = fmt.Sprintf(" Guest Actions (%s) ", pendingOperation)
	}

	// Update the menu title to reflect pending status
	menuList.SetTitle(menuTitle)
	a.showContextMenuPage(menuList, menuItems, 30, true)
}

func (a *App) showBatchVMContextMenu() {
	selected := a.selectedGuestsFromCurrentList()
	if len(selected) <= 1 {
		// Selection may have changed while opening the menu; fallback to single item flow.
		return
	}

	// Keep selection order deterministic for summaries/confirm copy.
	sortVMsByIdentity(selected)

	menuItems := []string{
		vmActionStart,
		vmActionShutdown,
		vmActionStop,
		vmActionRestart,
		vmActionReset,
		vmActionClearBatch,
	}
	shortcuts := []rune{'t', 'd', 'D', 'a', 'R', 'c'}

	a.lastFocus = a.GetFocus()

	menu := NewContextMenuWithShortcuts(
		fmt.Sprintf(" Batch Actions (%d selected) ", len(selected)),
		menuItems,
		shortcuts,
		func(_ int, action string) {
			a.CloseContextMenu()
			if action == vmActionClearBatch {
				a.clearGuestSelections()
				a.header.ShowSuccess("Cleared guest selection")
				return
			}

			a.confirmAndQueueBatchOperation(action, selected)
		},
	)
	menu.SetApp(a)

	menuList := menu.Show()
	oldCapture := menuList.GetInputCapture()
	menuList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'h') {
			a.CloseContextMenu()
			return nil
		}
		if oldCapture != nil {
			return oldCapture(event)
		}
		return event
	})

	a.showContextMenuPage(menuList, menuItems, 34, true)
}

func (a *App) confirmAndQueueBatchOperation(action string, selected []*api.VM) {
	if len(selected) == 0 {
		return
	}

	var opLabel string
	switch action {
	case vmActionStart:
		opLabel = "start"
	case vmActionShutdown:
		opLabel = "shutdown"
	case vmActionStop:
		opLabel = "stop"
	case vmActionRestart:
		opLabel = "restart"
	case vmActionReset:
		opLabel = "reset"
	default:
		return
	}

	msg := fmt.Sprintf("Queue %s for %d selected guests?", strings.ToUpper(opLabel), len(selected))
	a.showConfirmationDialog(msg, func() {
		submitted, skipped, failed := a.queueBatchOperation(action, selected)
		a.header.ShowSuccess(fmt.Sprintf("Batch %s queued: %d submitted, %d skipped, %d failed", opLabel, submitted, skipped, failed))
	})
}

func (a *App) queueBatchOperation(action string, selected []*api.VM) (submitted, skipped, failed int) {
	for _, vm := range selected {
		if vm == nil {
			skipped++
			continue
		}

		if !isEligibleForBatchAction(vm, action) {
			skipped++
			continue
		}

		client, err := a.getClientForVM(vm)
		if err != nil {
			failed++
			continue
		}

		switch action {
		case vmActionStart:
			a.enqueueVMOperation(vm, client.StartVM, "Starting", false)
		case vmActionShutdown:
			a.enqueueVMOperation(vm, client.ShutdownVM, "Shutting down", false)
		case vmActionStop:
			a.enqueueVMOperation(vm, client.StopVM, "Stopping", false)
		case vmActionRestart:
			a.enqueueVMOperation(vm, client.RestartVM, "Restarting", false)
		case vmActionReset:
			a.enqueueVMOperation(vm, client.ResetVM, "Resetting", false)
		default:
			skipped++
			continue
		}

		submitted++
	}

	return submitted, skipped, failed
}

func isEligibleForBatchAction(vm *api.VM, action string) bool {
	if vm == nil {
		return false
	}

	switch action {
	case vmActionStart:
		return vm.Status == api.VMStatusStopped
	case vmActionShutdown, vmActionStop, vmActionRestart:
		return vm.Status == api.VMStatusRunning
	case vmActionReset:
		return vm.Status == api.VMStatusRunning && vm.Type == api.VMTypeQemu
	default:
		return false
	}
}

// generateVMShortcuts generates letter shortcuts for VM menu items.
func generateVMShortcuts(menuItems []string, pluginActions []GuestAction) []rune {
	shortcuts := make([]rune, len(menuItems))

	// Define shortcuts based on menu item names
	for i, item := range menuItems {
		// Check if this is a plugin action first
		var isPluginAction bool
		for _, action := range pluginActions {
			if item == action.Label {
				shortcuts[i] = action.Shortcut
				isPluginAction = true
				break
			}
		}

		if isPluginAction {
			continue
		}

		switch item {
		case vmActionOpenShell:
			shortcuts[i] = 's'
		case vmActionOpenVNC:
			shortcuts[i] = 'v'
		case vmActionEditConfig:
			shortcuts[i] = 'e'
		case vmActionRefresh:
			shortcuts[i] = 'r'
		case vmActionStart:
			shortcuts[i] = 't'
		case vmActionShutdown:
			shortcuts[i] = 'd'
		case vmActionRestart:
			shortcuts[i] = 'a'
		case vmActionStop:
			// Use 'D' to avoid conflict with Open Shell ('s') and to pair with Shutdown ('d')
			shortcuts[i] = 'D'
		case vmActionReset:
			shortcuts[i] = 'R'
		case vmActionMigrate:
			shortcuts[i] = 'm'
		case vmActionDelete:
			shortcuts[i] = 'x'
		case vmActionSnapshots:
			shortcuts[i] = 'n'
		case vmActionBackups:
			shortcuts[i] = 'b'
		default:
			// Fallback to number if no specific shortcut defined
			shortcuts[i] = rune('1' + i)
		}
	}

	return shortcuts
}
