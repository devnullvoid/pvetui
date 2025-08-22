package components

import (
	"fmt"

	"github.com/devnullvoid/peevetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// VM menu action constants
const (
	vmActionOpenShell  = "Open Shell"
	vmActionOpenVNC    = "Open VNC Console"
	vmActionEditConfig = "Edit Configuration"
	vmActionSnapshots  = "Manage Snapshots"
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

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on VM state
	menuItems := []string{
		vmActionOpenShell,
		vmActionEditConfig,
		vmActionSnapshots,
		vmActionRefresh,
	}

	if (vm.Type == api.VMTypeQemu || vm.Type == api.VMTypeLXC) && vm.Status == api.VMStatusRunning {
		menuItems = append(menuItems[:1], append([]string{vmActionOpenVNC}, menuItems[1:]...)...)
	}

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

	// Generate letter shortcuts based on menu items
	shortcuts := generateVMShortcuts(menuItems)

	menu := NewContextMenuWithShortcuts(" Guest Actions ", menuItems, shortcuts, func(index int, action string) {
		a.CloseContextMenu()

		switch action {
		case vmActionOpenShell:
			a.openVMShell()
		case vmActionOpenVNC:
			a.openVMVNC()
		case vmActionEditConfig:
			go func() {
				cfg, err := a.client.GetVMConfig(vm)
				a.QueueUpdateDraw(func() {
					if err != nil {
						a.showMessageSafe(fmt.Sprintf("Failed to load config: %v", err))

						return
					}

					page := NewVMConfigPage(a, vm, cfg, func(newCfg *api.VMConfig) error {
						return a.client.UpdateVMConfig(vm, newCfg)
					})
					a.pages.AddPage("vmConfig", page, true, true)
					a.SetFocus(page)
				})
			}()
		case vmActionSnapshots:
			snapshotManager := NewSnapshotManager(a, vm)
			a.pages.AddPage("snapshots", snapshotManager, true, true)
			a.SetFocus(snapshotManager)
		case vmActionRefresh:
			a.refreshVMData(vm)
		case vmActionStart:
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to start VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.StartVM, "Starting")
				},
			)
		case vmActionShutdown:
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to gracefully shut down '%s' (ID: %d)?\n\nThis requests an OS shutdown and may take time.", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.ShutdownVM, "Shutting down")
				},
			)
		case vmActionStop:
			a.showConfirmationDialog(
				fmt.Sprintf("⚠️  Force stop '%s' (ID: %d)?\n\nThis is equivalent to power off and may cause data loss.", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.StopVM, "Stopping")
				},
			)
		case vmActionRestart:
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to restart VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.RestartVM, "Restarting")
				},
			)
		case vmActionReset:
			if vm.Type == api.VMTypeQemu {
				a.showConfirmationDialog(
					fmt.Sprintf("⚠️  Hard reset '%s' (ID: %d)?\n\nThis is an immediate reset (like pressing reset) and may cause data loss.", vm.Name, vm.ID),
					func() {
						a.performVMOperation(vm, a.client.ResetVM, "Resetting")
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

	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true).
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// generateVMShortcuts generates letter shortcuts for VM menu items.
func generateVMShortcuts(menuItems []string) []rune {
	shortcuts := make([]rune, len(menuItems))

	// Define shortcuts based on menu item names
	for i, item := range menuItems {
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
		default:
			// Fallback to number if no specific shortcut defined
			shortcuts[i] = rune('1' + i)
		}
	}

	return shortcuts
}
