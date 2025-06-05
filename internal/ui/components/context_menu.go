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

	// Setup input capture to close on escape
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape && cm.app != nil {
			cm.app.CloseContextMenu()
			return nil
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

			// Wait a moment before refreshing to allow the operation to complete on the server
			time.Sleep(1 * time.Second)

			// Manually refresh data to show updated state
			a.manualRefresh()
		}
	}()
}
