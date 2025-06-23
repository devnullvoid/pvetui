package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// HelpModal represents a modal dialog showing keybindings and usage information
type HelpModal struct {
	*tview.Pages
	app      *App
	textView *tview.TextView
}

// NewHelpModal creates a new help modal
func NewHelpModal() *HelpModal {
	// Create a scrollable text view for the help content
	textView := tview.NewTextView()
	textView.SetDynamicColors(true)
	textView.SetScrollable(true)
	textView.SetWrap(false) // Disable wrapping to prevent awkward line breaks
	textView.SetBorder(true)
	textView.SetTitle(" Proxmox TUI - Help & Keybindings ")
	textView.SetTitleColor(tcell.ColorYellow)
	textView.SetBorderColor(tcell.ColorYellow)

	helpText := `[yellow]Navigation:[-]
  [white]Arrow Keys / hjkl[-]         Navigate lists and panels
  [white]Tab[-]                       Switch between Nodes and Guests
  [white]F1[-]                        Switch to Nodes tab
  [white]F2[-]                        Switch to Guests tab
  [white]F3[-]                        Switch to Tasks tab

[yellow]Actions:[-]
  [white]/[-]                         Search/Filter current list
  [white]S[-]                         Open SSH shell (node/guest)
  [white]V[-]                         Open VNC console (node/guest)
  [white]M[-]                         Open context menu
  [white]C[-]                         Install community scripts (nodes)
  [white]R[-]                         Manual refresh
  [white]A[-]                         Toggle auto-refresh (10s interval)
  [white]Q[-]                         Quit application (confirms if VNC sessions active)

[yellow]VI-like Navigation:[-]
  [white]h[-]                         Move left / Go back
  [white]j[-]                         Move down
  [white]k[-]                         Move up
  [white]l[-]                         Move right / Select/Enter

[yellow]In Lists:[-]
  [white]Enter[-]                     Select item
  [white]Escape[-]                    Close modal/search

[yellow]In Modals:[-]
  [white]Escape[-]                    Close modal
  [white]Tab[-]                       Navigate between buttons
  [white]Enter[-]                     Activate button

[yellow]Search Functionality:[-]
  [white]Type to filter[-]            Filter by name, ID, or node
  [white]Enter/Escape[-]              Exit search mode
  [white]Arrow keys/jk[-]             Navigate filtered results

[yellow]Context Menu Actions:[-]
  [white]Nodes:[-]                    Shell, VNC, Scripts, Refresh
  [white]Guests:[-]                   Shell, VNC, Start/Stop/Restart

[yellow]Tips & Usage:[-]
  • Use search ([white]/[-]) to quickly find nodes or guests
  • Context menu ([white]M[-]) provides quick access to actions
  • VNC opens in your browser using embedded noVNC client
  • SSH sessions open in new terminal windows
  • Community scripts are installed interactively via SSH
  • Use [white]hjkl[-] keys for VI-like navigation throughout
  • All arrow key functionality is preserved alongside hjkl
  • Quitting with active VNC sessions will prompt for confirmation

[yellow]Troubleshooting:[-]
  • If VNC doesn't open, check your default browser settings
  • SSH requires proper key-based authentication or password
  • Community scripts require internet access on the target node
  • Use [white]R[-] to manually refresh if data seems stale

[yellow]Scrolling in Help:[-]
  [white]Arrow Keys / jk[-]           Scroll up/down through help content
  [white]Page Up/Down[-]              Scroll by page
  [white]Home/End[-]                  Go to top/bottom

[gray]Press [white]?[-][gray] again, [white]Escape[-][gray], or [white]q[-][gray] to exit this help[-]`

	textView.SetText(helpText)

	// Create a flex container to center the text view with better proportions
	flex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false). // Left padding (smaller)
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).      // Top padding
			AddItem(textView, 0, 10, true). // Main content (takes most space)
			AddItem(nil, 0, 1, false),      // Bottom padding
						0, 8, true). // Main column (much wider)
		AddItem(nil, 0, 1, false) // Right padding (smaller)

	// Create pages container
	pages := tview.NewPages()
	pages.AddPage("help-content", flex, true, true)

	return &HelpModal{
		Pages:    pages,
		textView: textView,
	}
}

// SetApp sets the parent app reference
func (hm *HelpModal) SetApp(app *App) {
	hm.app = app
}

// Show displays the help modal
func (hm *HelpModal) Show() {
	if hm.app != nil {
		// Set up input capture to handle ?, Escape, and q keys, plus scrolling
		hm.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch {
			case event.Key() == tcell.KeyEscape ||
				(event.Key() == tcell.KeyRune && (event.Rune() == '?' || event.Rune() == 'q')):
				hm.Hide()
				return nil
			case event.Key() == tcell.KeyRune && event.Rune() == 'j':
				// VI-like down scrolling
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case event.Key() == tcell.KeyRune && event.Rune() == 'k':
				// VI-like up scrolling
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}
			return event
		})

		hm.app.pages.AddPage("help", hm.Pages, true, true)
		hm.app.SetFocus(hm.textView)
	}
}

// Hide closes the help modal
func (hm *HelpModal) Hide() {
	if hm.app != nil {
		hm.app.pages.RemovePage("help")
		// Restore focus to the appropriate component based on current page
		pageName, _ := hm.app.pages.GetFrontPage()
		if pageName == api.PageNodes {
			hm.app.SetFocus(hm.app.nodeList)
		} else if pageName == api.PageGuests {
			hm.app.SetFocus(hm.app.vmList)
		}
	}
}
