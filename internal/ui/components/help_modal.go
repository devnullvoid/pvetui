package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/peevetui/internal/config"
	"github.com/devnullvoid/peevetui/internal/ui/theme"
)

// HelpModal represents a modal dialog showing keybindings and usage information.
type HelpModal struct {
	*tview.Pages

	app      *App
	textView *tview.TextView
}

// NewHelpModal creates a new help modal.
func NewHelpModal(keys config.KeyBindings) *HelpModal {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)

	textView.SetBorder(true).
		SetTitle(" Proxmox TUI - Help & Keybindings ").
		SetTitleColor(theme.Colors.Primary).
		SetBorderColor(theme.Colors.Border)

	helpText := buildHelpText(keys)
	textView.SetText(helpText)

	// Create a flex container to center the text view with better proportions
	flex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false). // Left padding
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).      // Top padding
			AddItem(textView, 0, 10, true). // Main content
			AddItem(nil, 0, 1, false),      // Bottom padding
						0, 8, true). // Main column
		AddItem(nil, 0, 1, false) // Right padding

	pages := tview.NewPages()
	pages.AddPage("help-content", flex, true, true)

	return &HelpModal{
		Pages:    pages,
		textView: textView,
	}
}

// buildHelpText constructs the formatted and aligned help text.
func buildHelpText(keys config.KeyBindings) string {
	// Define all help items in sections for clarity
	items := []struct {
		Cat, Key, Desc string
	}{
		{Cat: "[warning]Navigation[-]"},
		{Key: "Arrow Keys / hjkl", Desc: "Navigate lists and panels"},
		{Key: fmt.Sprintf("%s / %s", keys.SwitchView, keys.SwitchViewReverse), Desc: "Switch between views (forward/reverse)"},
		{Key: keys.NodesPage, Desc: "Switch to Nodes tab"},
		{Key: keys.GuestsPage, Desc: "Switch to Guests tab"},
		{Key: keys.TasksPage, Desc: "Switch to Tasks tab"},
		{Cat: ""}, // Spacer
		{Cat: "[warning]Actions[-]"},
		{Key: keys.Search, Desc: "Search/Filter current list"},
		{Key: keys.Shell, Desc: "Open SSH shell (node/guest)"},
		{Key: keys.VNC, Desc: "Open VNC console (node/guest)"},
		{Key: keys.Menu, Desc: "Open context menu"},
		{Key: keys.GlobalMenu, Desc: "Open global menu"},
		{Key: keys.Refresh, Desc: "Manual refresh"},
		{Key: keys.AutoRefresh, Desc: "Toggle auto-refresh (10s interval)"},
		{Key: keys.Quit, Desc: "Quit application"},
		{Cat: ""},
		{Cat: "[warning]Tips & Usage[-]"},
		{Desc: fmt.Sprintf("• Use search ([primary]%s[-]) to quickly find nodes or guests.", keys.Search)},
		{Desc: fmt.Sprintf("• The context menu ([primary]%s[-]) provides quick access to actions.", keys.Menu)},
		{Desc: "• Press [primary]Esc[-] to open the global menu for app-wide actions."},
		{Desc: "• The 'g' key is still available for global menu if configured in key_bindings."},
		{Desc: "• VNC opens in your default web browser."},
		{Desc: "• SSH sessions suspend the UI until the session is closed."},
	}

	// Calculate the maximum width of the key column to align descriptions
	maxKeyWidth := 0

	for _, item := range items {
		if item.Key != "" {
			width := tview.TaggedStringWidth(item.Key)
			if width > maxKeyWidth {
				maxKeyWidth = width
			}
		}
	}

	var builder strings.Builder

	for _, item := range items {
		if item.Cat != "" {
			builder.WriteString(fmt.Sprintf("%s\n", item.Cat))
		} else if item.Key != "" {
			padding := maxKeyWidth - tview.TaggedStringWidth(item.Key)
			builder.WriteString(fmt.Sprintf("  [primary]%-s%s[-]  %s\n", item.Key, strings.Repeat(" ", padding), item.Desc))
		} else if item.Desc != "" {
			builder.WriteString(fmt.Sprintf("  %s\n", item.Desc))
		} else {
			builder.WriteString("\n")
		}
	}

	// Add the final footer text
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("[info]Press [primary]%s[-][info] again, [primary]Escape[-][info], or [primary]%s[-][info] to exit this help[-]", strings.ToLower(keys.Help), strings.ToLower(keys.Quit)))

	return theme.ReplaceSemanticTags(builder.String())
}

// SetApp sets the parent app reference.
func (hm *HelpModal) SetApp(app *App) {
	hm.app = app
}

// Show displays the help modal.
func (hm *HelpModal) Show() {
	if hm.app != nil {
		// Set up input capture to handle closing and scrolling
		hm.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch {
			case event.Key() == tcell.KeyEscape ||
				(event.Key() == tcell.KeyRune && (event.Rune() == '?' || event.Rune() == 'q')):
				hm.Hide()

				return nil
			case event.Key() == tcell.KeyRune && event.Rune() == 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone) // Map 'j' to Down
			case event.Key() == tcell.KeyRune && event.Rune() == 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone) // Map 'k' to Up
			}

			return event
		})

		hm.app.pages.AddPage("help", hm.Pages, true, true)
		hm.app.SetFocus(hm.textView)
	}
}

// Hide hides the help modal.
func (hm *HelpModal) Hide() {
	if hm.app != nil && hm.app.pages != nil {
		hm.app.pages.RemovePage("help")
	}
}
