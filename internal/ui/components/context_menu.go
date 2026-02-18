package components

import (
	"github.com/devnullvoid/pvetui/internal/ui/theme"
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
	list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary).Attributes(tcell.AttrReverse))

	for i, action := range cm.menuItems {
		var shortcut rune

		// If explicit shortcuts are provided, use them (even if 0)
		if len(cm.shortcuts) > 0 {
			if i < len(cm.shortcuts) {
				shortcut = cm.shortcuts[i]
			}
		}

		// If shortcut is 0, tview will not display it.
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

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

func contextMenuWidth(title string, menuItems []string, preferred int) int {
	maxContentWidth := tview.TaggedStringWidth(title)
	for _, item := range menuItems {
		if w := tview.TaggedStringWidth(item); w > maxContentWidth {
			maxContentWidth = w
		}
	}

	// Border/padding slack.
	minWidth := maxContentWidth + 4
	if preferred > minWidth {
		return preferred
	}
	if minWidth < 20 {
		return 20
	}
	return minWidth
}

func screenSizeForMenu(app *App) (int, int) {
	if app != nil {
		if app.pages != nil {
			_, _, w, h := app.pages.GetRect()
			if w > 0 && h > 0 {
				return w, h
			}
		}
		if app.mainLayout != nil {
			_, _, w, h := app.mainLayout.GetRect()
			if w > 0 && h > 0 {
				return w, h
			}
		}
		if app.footer != nil {
			_, _, w, h := app.footer.GetRect()
			if w > 0 && h > 0 {
				return w, h
			}
		}
		if app.header != nil {
			_, _, w, h := app.header.GetRect()
			if w > 0 && h > 0 {
				return w, h
			}
		}
		if app.nodeList != nil {
			_, _, w, h := app.nodeList.GetRect()
			if w > 0 && h > 0 {
				return w, h
			}
		}
		if app.vmList != nil {
			_, _, w, h := app.vmList.GetRect()
			if w > 0 && h > 0 {
				return w, h
			}
		}
		if app.tasksList != nil {
			_, _, w, h := app.tasksList.GetRect()
			return w, h
		}
	}

	return 120, 40
}

func (a *App) anchoredContextMenuPosition(menuWidth, menuHeight, screenW, screenH int) (int, int) {
	x := (screenW - menuWidth) / 2
	y := (screenH - menuHeight) / 2

	focus := a.lastFocus
	if focus == nil {
		focus = a.GetFocus()
	}
	if focus == nil {
		return clamp(x, 0, max(0, screenW-menuWidth)), clamp(y, 0, max(0, screenH-menuHeight))
	}

	fx, _, fw, fh := focus.GetRect()
	if fw <= 0 || fh <= 0 {
		return clamp(x, 0, max(0, screenW-menuWidth)), clamp(y, 0, max(0, screenH-menuHeight))
	}

	dividerX := fx + fw
	// Center menu around the divider between list/details panes.
	x = dividerX - (menuWidth / 2)
	// Keep menu vertically centered to avoid row-based clipping.
	y = (screenH - menuHeight) / 2

	x = clamp(x, 0, max(0, screenW-menuWidth))
	y = clamp(y, 0, max(0, screenH-menuHeight))

	return x, y
}

func (a *App) showContextMenuPage(menuList *tview.List, menuItems []string, preferredWidth int, anchorToFocus bool) {
	a.contextMenu = menuList
	a.isMenuOpen = true

	screenW, screenH := screenSizeForMenu(a)
	menuH := len(menuItems) + 2
	if menuH < 3 {
		menuH = 3
	}
	menuW := contextMenuWidth(menuList.GetTitle(), menuItems, preferredWidth)
	menuW = clamp(menuW, 20, max(20, screenW-2))
	menuH = clamp(menuH, 3, max(3, screenH-2))

	var x, y int
	if anchorToFocus {
		x, y = a.anchoredContextMenuPosition(menuW, menuH, screenW, screenH)
	} else {
		x = (screenW - menuW) / 2
		y = (screenH - menuH) / 2
		x = clamp(x, 0, max(0, screenW-menuW))
		y = clamp(y, 0, max(0, screenH-menuH))
	}

	container := tview.NewFlex().
		AddItem(nil, x, 0, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, y, 0, false).
			AddItem(menuList, menuH, 0, true).
			AddItem(nil, 0, 1, false), menuW, 0, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("contextMenu", container, true, true)
	a.SetFocus(menuList)
}
