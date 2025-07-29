package components

import (
	"github.com/devnullvoid/proxmox-tui/internal/version"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowGlobalContextMenu displays the global context menu for app-wide actions.
func (a *App) ShowGlobalContextMenu() {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items for global actions
	menuItems := []string{
		"Connection Profiles",
		"Refresh All Data",
		"Toggle Auto-Refresh",
		"Help",
		"About",
		"Quit",
	}

	// Define custom shortcuts for global menu
	shortcuts := []rune{'p', 'r', 'a', '?', 'i', 'q'}

	menu := NewContextMenuWithShortcuts(" Global Actions ", menuItems, shortcuts, func(index int, action string) {
		a.CloseContextMenu()

		switch action {
		case "Connection Profiles":
			a.showConnectionProfilesDialog()
		case "Refresh All Data":
			a.manualRefresh()
		case "Toggle Auto-Refresh":
			a.toggleAutoRefresh()
		case "Help":
			if a.pages.HasPage("help") {
				a.helpModal.Hide()
			} else {
				a.helpModal.Show()
			}
		case "About":
			a.showAboutDialog()
		case "Quit":
			a.Application.Stop()
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
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// showAboutDialog displays information about the application.
func (a *App) showAboutDialog() {
	// Get version information
	versionInfo := version.GetBuildInfo()

	// Create about dialog using the reusable function
	modal := CreateAboutDialog(versionInfo, func() {
		a.pages.RemovePage("about")
	})

	a.pages.AddPage("about", modal, false, true)
}
