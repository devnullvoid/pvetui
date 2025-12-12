package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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
)

// ShowVMContextMenu displays the context menu for VM actions.
func (a *App) ShowVMContextMenu() {
	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		return
	}

	// Get the node for this VM
	node := a.vmList.GetNodeForVM(vm)

	// Check if this VM has a pending operation
	isPending, pendingOperation := models.GlobalState.IsVMPending(vm)

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

	// * Only show lifecycle actions if no operation is pending
	if !isPending {
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
	} else {
		// * Show pending operation info in menu title
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

	a.contextMenu = menuList
	a.isMenuOpen = true

	// * Update menu title to show pending status if applicable
	menuTitle := " Guest Actions "
	if isPending {
		menuTitle = fmt.Sprintf(" Guest Actions (%s) ", pendingOperation)
	}

	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true).
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)

	// Update the menu title to reflect pending status
	menuList.SetTitle(menuTitle)
	a.SetFocus(menuList)
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
