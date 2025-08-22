package components

import (
	"github.com/devnullvoid/peevetui/internal/ui/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ContextMenu represents a popup menu with actions for a selected item.
type ContextMenu struct {
	list      *tview.List
	app       *App
	onAction  func(index int, action string)
	menuItems []string
	shortcuts []rune
	title     string
}

// NewContextMenu creates a new context menu component.
func NewContextMenu(title string, actions []string, onAction func(index int, action string)) *ContextMenu {
	return &ContextMenu{
		menuItems: actions,
		title:     title,
		onAction:  onAction,
	}
}

// NewContextMenuWithShortcuts creates a new context menu component with custom shortcuts.
func NewContextMenuWithShortcuts(title string, actions []string, shortcuts []rune, onAction func(index int, action string)) *ContextMenu {
	return &ContextMenu{
		menuItems: actions,
		shortcuts: shortcuts,
		title:     title,
		onAction:  onAction,
	}
}

// SetApp sets the parent app reference.
func (cm *ContextMenu) SetApp(app *App) {
	cm.app = app
}

// Show displays the context menu as a modal.
func (cm *ContextMenu) Show() *tview.List {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(cm.title)
	list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary))

	for i, action := range cm.menuItems {
		var shortcut rune
		if i < len(cm.shortcuts) {
			shortcut = cm.shortcuts[i]
		} else {
			shortcut = rune('1' + i)
		}
		list.AddItem(action, "", shortcut, nil)
	}

	list.SetHighlightFullLine(true)

	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		// The parent App should handle closing the context menu
		if cm.onAction != nil {
			cm.onAction(index, mainText)
		}
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// The parent App should handle closing the context menu
			return nil
		} else if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'h':
				// The parent App should handle closing the context menu
				return nil
			case 'l':
				index := list.GetCurrentItem()
				if index >= 0 && index < len(cm.menuItems) {
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

// CloseContextMenu closes the context menu and restores the previous focus.
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
