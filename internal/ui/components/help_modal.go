package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HelpModal represents a modal dialog showing keybindings and usage information
type HelpModal struct {
	*tview.Modal
	app *App
}

// NewHelpModal creates a new help modal
func NewHelpModal() *HelpModal {
	modal := tview.NewModal()

	helpText := `[yellow]Proxmox TUI - Help & Keybindings[-]

[yellow]Navigation:[-]
  [white]Arrow Keys / hjkl[-]    Navigate lists and panels
  [white]Tab[-]                  Switch between Nodes and Guests tabs
  [white]F1[-]                   Switch to Nodes tab
  [white]F2[-]                   Switch to Guests tab

[yellow]Actions:[-]
  [white]/[-]                    Search/Filter current list
  [white]S[-]                    Open SSH shell (node/guest)
  [white]V[-]                    Open VNC console (node/guest)
  [white]M[-]                    Open context menu
  [white]C[-]                    Install community scripts (nodes only)
  [white]R[-]                    Manual refresh
  [white]Q[-]                    Quit application

[yellow]VI-like Navigation:[-]
  [white]h[-]                    Move left / Go back
  [white]j[-]                    Move down
  [white]k[-]                    Move up
  [white]l[-]                    Move right / Select/Enter

[yellow]In Lists:[-]
  [white]Enter[-]                Select item
  [white]Escape[-]               Close modal/search

[yellow]In Modals:[-]
  [white]Escape[-]               Close modal
  [white]Tab[-]                  Navigate between buttons
  [white]Enter[-]                Activate button

[yellow]Tips:[-]
  • Use search (/) to quickly find nodes or guests
  • Context menu (M) provides quick access to common actions
  • VNC opens in your default browser
  • SSH sessions open in new terminal windows
  • Community scripts are installed interactively

[yellow]Press ? again or Escape to close this help[-]`

	modal.SetText(helpText).
		SetBackgroundColor(tcell.ColorBlack).
		SetTextColor(tcell.ColorWhite).
		AddButtons([]string{"Close"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			// Close the help modal when any button is pressed
		})

	return &HelpModal{
		Modal: modal,
	}
}

// SetApp sets the parent app reference
func (hm *HelpModal) SetApp(app *App) {
	hm.app = app
}

// Show displays the help modal
func (hm *HelpModal) Show() {
	if hm.app != nil {
		// Set up input capture to handle ? and Escape keys
		hm.Modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == '?') {
				hm.Hide()
				return nil
			}
			return event
		})

		// Set done function to close modal when Close button is pressed
		hm.Modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			hm.Hide()
		})

		hm.app.pages.AddPage("help", hm.Modal, true, true)
		hm.app.SetFocus(hm.Modal)
	}
}

// Hide closes the help modal
func (hm *HelpModal) Hide() {
	if hm.app != nil {
		hm.app.pages.RemovePage("help")
		// Restore focus to the appropriate component based on current page
		pageName, _ := hm.app.pages.GetFrontPage()
		if pageName == "Nodes" {
			hm.app.SetFocus(hm.app.nodeList)
		} else if pageName == "Guests" {
			hm.app.SetFocus(hm.app.vmList)
		}
	}
}
