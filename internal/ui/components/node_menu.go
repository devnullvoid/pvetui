package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Node menu action constants
const (
	nodeActionOpenShell = "Open Shell"
	nodeActionOpenVNC   = "Open VNC Console"
	nodeActionInstall   = "Install Community Script"
	nodeActionRefresh   = "Refresh"
)

// ShowNodeContextMenu displays the context menu for node actions.
func (a *App) ShowNodeContextMenu() {
	node := a.nodeList.GetSelectedNode()
	if node == nil {
		return
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on node state
	menuItems := []string{
		nodeActionOpenShell,
		nodeActionOpenVNC,
		// "View Logs",
		nodeActionInstall,
		nodeActionRefresh,
	}

	// Define letter shortcuts for node actions
	shortcuts := []rune{'s', 'v', 'i', 'r'}

	menu := NewContextMenuWithShortcuts(" Node Actions ", menuItems, shortcuts, func(index int, action string) {
		a.CloseContextMenu()

		switch action {
		case nodeActionOpenShell:
			a.openNodeShell()
		case nodeActionOpenVNC:
			a.openNodeVNC()
		// case "View Logs":
		// 	a.showMessage("Viewing logs for node: " + node.Name)
		case nodeActionInstall:
			a.openScriptSelector(node, nil)
		case nodeActionRefresh:
			a.refreshNodeData(node)
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
