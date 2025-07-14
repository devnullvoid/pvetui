package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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
		"Edit Configuration",
		"Refresh",
	}

	if (vm.Type == api.VMTypeQemu || vm.Type == api.VMTypeLXC) && vm.Status == api.VMStatusRunning {
		menuItems = append(menuItems[:1], append([]string{"Open VNC Console"}, menuItems[1:]...)...)
	}

	if vm.Status == api.VMStatusRunning {
		menuItems = append(menuItems, "Shutdown", "Restart")
	} else if vm.Status == api.VMStatusStopped {
		menuItems = append(menuItems, "Start")
	}

	menuItems = append(menuItems, "Migrate")
	menuItems = append(menuItems, "Delete")

	menu := NewContextMenu(" Guest Actions ", menuItems, func(index int, action string) {
		a.CloseContextMenu()
		switch action {
		case "Open Shell":
			a.openVMShell()
		case "Open VNC Console":
			a.openVMVNC()
		case "Edit Configuration":
			go func() {
				cfg, err := a.client.GetVMConfig(vm)
				a.QueueUpdateDraw(func() {
					if err != nil {
						a.showMessage(fmt.Sprintf("Failed to load config: %v", err))
						return
					}
					page := NewVMConfigPage(a, vm, cfg, func(newCfg *api.VMConfig) error {
						return a.client.UpdateVMConfig(vm, newCfg)
					})
					a.pages.AddPage("vmConfig", page, true, true)
					a.SetFocus(page)
				})
			}()
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
