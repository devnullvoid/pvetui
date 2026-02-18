package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// Node menu action constants
const (
	nodeActionOpenShell = "Open Shell"
	nodeActionOpenVNC   = "Open VNC Console"
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

	type menuEntry struct {
		label    string
		shortcut rune
		handler  func()
	}

	entries := []menuEntry{
		{label: nodeActionOpenShell, shortcut: 's', handler: func() { a.openNodeShell() }},
		{label: nodeActionOpenVNC, shortcut: 'v', handler: func() { a.openNodeVNC() }},
	}

	if a.pluginRegistry != nil {
		for _, pluginAction := range a.pluginRegistry.NodeActionsForNode(node) {
			pa := pluginAction
			entries = append(entries, menuEntry{
				label:    pa.Label,
				shortcut: pa.Shortcut,
				handler: func() {
					if pa.Handler == nil {
						return
					}

					if err := pa.Handler(a.ctx, a, node); err != nil {
						a.showMessageSafe(fmt.Sprintf("%s failed: %v", pa.Label, err))
					}
				},
			})
		}
	}

	entries = append(entries, menuEntry{label: nodeActionRefresh, shortcut: 'r', handler: func() {
		a.refreshNodeData(node)
	}})

	menuItems := make([]string, len(entries))
	shortcuts := make([]rune, len(entries))
	for i, entry := range entries {
		menuItems[i] = entry.label
		shortcuts[i] = entry.shortcut
	}

	menu := NewContextMenuWithShortcuts(" Node Actions ", menuItems, shortcuts, func(index int, action string) {
		a.CloseContextMenu()

		if index >= 0 && index < len(entries) {
			handler := entries[index].handler
			if handler != nil {
				handler()
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

	a.showContextMenuPage(menuList, menuItems, 30, true)
}
