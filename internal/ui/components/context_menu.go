package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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
	list.SetSelectedBackgroundColor(tcell.ColorBlue)
	list.SetSelectedTextColor(tcell.ColorGray)

	// Set up action handler
	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if cm.app != nil {
			cm.app.closeContextMenu()
		}
		if cm.onAction != nil {
			cm.onAction(index, mainText)
		}
	})

	// Setup input capture to close on escape
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape && cm.app != nil {
			cm.app.closeContextMenu()
			return nil
		}
		return event
	})

	cm.list = list
	return list
}
