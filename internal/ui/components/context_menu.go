package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// ContextMenu represents a popup menu with actions for a selected item
type ContextMenu struct {
	list      *tview.List
	app       *App
	onAction  func(index int, action string)
	menuItems []string
	title     string
}

// NewContextMenu creates a new context menu component
func NewContextMenu(title string, actions []string, onAction func(index int, action string)) *ContextMenu {
	return &ContextMenu{
		menuItems: actions,
		title:     title,
		onAction:  onAction,
	}
}

// SetApp sets the parent app reference
func (cm *ContextMenu) SetApp(app *App) {
	cm.app = app
}

// Show displays the context menu as a modal
func (cm *ContextMenu) Show() *tview.List {
	// Create the list with proper type
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(cm.title)

	// Add actions to the list
	for i, action := range cm.menuItems {
		list.AddItem(action, "", rune('a'+i), nil)
	}

	// Set list highlight color
	list.SetHighlightFullLine(true)
	// list.SetSelectedBackgroundColor(tcell.ColorBlue)
	// list.SetSelectedTextColor(tcell.ColorGray)

	// Set up action handler
	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if cm.app != nil {
			cm.app.CloseContextMenu()
		}
		if cm.onAction != nil {
			cm.onAction(index, mainText)
		}
	})

	// Setup input capture to close on escape and handle VI-like navigation
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape && cm.app != nil {
			cm.app.CloseContextMenu()
			return nil
		} else if event.Key() == tcell.KeyRune {
			// Handle VI-like navigation (hjkl)
			switch event.Rune() {
			case 'j': // VI-like down navigation
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k': // VI-like up navigation
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'h': // VI-like left navigation - close menu
				if cm.app != nil {
					cm.app.CloseContextMenu()
				}
				return nil
			case 'l': // VI-like right navigation - select item (same as Enter)
				index := list.GetCurrentItem()
				if index >= 0 && index < len(cm.menuItems) {
					if cm.app != nil {
						cm.app.CloseContextMenu()
					}
					if cm.onAction != nil {
						cm.onAction(index, cm.menuItems[index])
					}
				}
				return nil
			}
		}
		return event
	})

	cm.list = list
	return list
}

// ShowNodeContextMenu displays the context menu for node actions
func (a *App) ShowNodeContextMenu() {
	node := a.nodeList.GetSelectedNode()
	if node == nil {
		return
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on node state
	menuItems := []string{
		"Open Shell",
		"Open VNC Shell",
		// "View Logs",
		"Install Community Script",
		"Refresh",
	}

	// Create and show context menu
	menu := NewContextMenu(" Node Actions ", menuItems, func(index int, action string) {
		switch action {
		case "Open Shell":
			a.openNodeShell()
		case "Open VNC Shell":
			a.openNodeVNC()
		// case "View Logs":
		// 	a.showMessage("Viewing logs for node: " + node.Name)
		case "Install Community Script":
			a.openScriptSelector(node, nil)
		case "Refresh":
			a.refreshNodeData(node)
		}
	})
	menu.SetApp(a)

	// Display the menu
	menuList := menu.Show()
	a.contextMenu = menuList
	a.isMenuOpen = true

	// Create a centered modal layout
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// ShowVMContextMenu displays the context menu for VM actions
func (a *App) ShowVMContextMenu() {
	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		return
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on VM state
	menuItems := []string{
		"Open Shell",
		"Refresh",
	}

	// Add VNC option for QEMU VMs and LXC containers that are running
	if (vm.Type == api.VMTypeQemu || vm.Type == api.VMTypeLXC) && vm.Status == api.VMStatusRunning {
		menuItems = append([]string{"Open VNC Console"}, menuItems...)
	}

	// Add state-dependent actions
	if vm.Status == api.VMStatusRunning {
		menuItems = append(menuItems, "Shutdown", "Restart")
	} else if vm.Status == api.VMStatusStopped {
		menuItems = append(menuItems, "Start")
	}

	// Add migrate option (always available)
	menuItems = append(menuItems, "Migrate")

	// Add delete option (always available)
	menuItems = append(menuItems, "Delete")

	// Note: Removed "Install Community Script" as it's only applicable to nodes

	// Create and show context menu
	menu := NewContextMenu(" Guest Actions ", menuItems, func(index int, action string) {
		switch action {
		case "Open VNC Console":
			a.openVMVNC()
		case "Open Shell":
			a.openVMShell()
		case "Refresh":
			a.refreshVMData(vm)
		case "Start":
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to start VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.StartVM, "Starting")
				},
			)
		case "Shutdown":
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to shutdown VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.StopVM, "Shutting down")
				},
			)
		case "Restart":
			a.showConfirmationDialog(
				fmt.Sprintf("Are you sure you want to restart VM '%s' (ID: %d)?", vm.Name, vm.ID),
				func() {
					a.performVMOperation(vm, a.client.RestartVM, "Restarting")
				},
			)
		case "Migrate":
			a.showMigrationDialog(vm)
		case "Delete":
			// Check if VM is running and provide appropriate options
			if vm.Status == api.VMStatusRunning {
				a.showDeleteRunningVMDialog(vm)
			} else {
				a.showConfirmationDialog(
					fmt.Sprintf("⚠️  DANGER: Are you sure you want to permanently DELETE VM '%s' (ID: %d)?\n\nThis action is IRREVERSIBLE and will destroy all VM data including disks!", vm.Name, vm.ID),
					func() {
						a.performVMDeleteOperation(vm, false) // false = not forced
					},
				)
			}
		}
	})
	menu.SetApp(a)

	// Display the menu
	menuList := menu.Show()
	a.contextMenu = menuList
	a.isMenuOpen = true

	// Create a centered modal layout
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// CloseContextMenu closes the context menu and restores the previous focus
func (a *App) CloseContextMenu() {
	if a.isMenuOpen {
		a.pages.RemovePage("contextMenu")
		a.isMenuOpen = false
		a.contextMenu = nil
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}
}

// performVMOperation performs an asynchronous VM operation and shows status message
func (a *App) performVMOperation(vm *api.VM, operation func(*api.VM) error, operationName string) {
	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("%s %s", operationName, vm.Name))

	// Run operation in goroutine to avoid blocking UI
	go func() {
		if err := operation(vm); err != nil {
			// Update message with error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error %s %s: %v", strings.ToLower(operationName), vm.Name, err))
			})
		} else {
			// Update message with success on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("%s %s completed successfully", operationName, vm.Name))
			})

			// Clear API cache to ensure fresh VM state is loaded
			a.client.ClearAPICache()

			// Wait a moment for the Proxmox server to fully process the operation
			// before refreshing the VM data
			go func() {
				time.Sleep(5 * time.Second) // Shorter delay for non-delete operations
				a.QueueUpdateDraw(func() {
					a.refreshVMData(vm) // Use targeted refresh for state changes
				})
			}()
		}
	}()
}

// performVMDeleteOperation performs an asynchronous VM delete operation and refreshes the VM list
func (a *App) performVMDeleteOperation(vm *api.VM, forced bool) {
	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Deleting %s", vm.Name))

	// Run operation in goroutine to avoid blocking UI
	go func() {
		var err error
		if forced {
			// Use force delete for running VMs
			options := &api.DeleteVMOptions{
				Force:                    true,
				DestroyUnreferencedDisks: true,
				Purge:                    true,
			}
			err = a.client.DeleteVMWithOptions(vm, options)
		} else {
			// Regular delete
			err = a.client.DeleteVM(vm)
		}

		if err != nil {
			// Update message with error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error deleting %s: %v", vm.Name, err))
			})
		} else {
			// Update message with success on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Successfully deleted %s", vm.Name))
			})

			// Clear API cache to ensure deleted VM is removed from the list
			a.client.ClearAPICache()

			// Wait a few seconds for the Proxmox server to fully process the deletion
			// before refreshing the VM list
			go func() {
				time.Sleep(5 * time.Second)
				a.QueueUpdateDraw(func() {
					a.manualRefresh()
				})
			}()
		}
	}()
}

// showDeleteRunningVMDialog shows a dialog with options for deleting a running VM
func (a *App) showDeleteRunningVMDialog(vm *api.VM) {
	message := fmt.Sprintf("⚠️  VM '%s' (ID: %d) is currently RUNNING\n\nProxmox can force delete running VMs.\n\nAre you sure you want to FORCE DELETE this running VM?\n\nThis will IMMEDIATELY DESTROY the VM and ALL its data!", vm.Name, vm.ID)

	a.showConfirmationDialog(message, func() {
		// User chose to force delete the running VM
		a.performVMDeleteOperation(vm, true) // true = forced
	})
}
