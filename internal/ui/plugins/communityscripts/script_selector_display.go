package communityscripts

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// showScriptInfo displays the script information in a page (not modal).
func (s *ScriptSelector) showScriptInfo(script Script) {
	// Create a text view for the script information
	textView := tview.NewTextView()
	textView.SetDynamicColors(true)
	textView.SetScrollable(true)
	textView.SetWrap(true)
	textView.SetBorder(true)
	textView.SetTitle(fmt.Sprintf(" %s - Script Details ", script.Name))
	textView.SetTitleColor(theme.Colors.Title)
	textView.SetBorderColor(theme.Colors.Border)
	textView.SetText(s.formatScriptInfo(script))

	// Create buttons
	installButton := tview.NewButton("Install").
		SetSelectedFunc(func() {
			s.app.Pages().RemovePage("scriptInfo")
			s.installScript(script)
		})

	cancelButton := tview.NewButton("Cancel").
		SetSelectedFunc(func() {
			s.app.Pages().RemovePage("scriptInfo")
			s.app.SetFocus(s.scriptList)
		})

	// Create spacers with proper background for centering
	leftSpacer := tview.NewBox().SetBackgroundColor(theme.Colors.Background)
	middleSpacer := tview.NewBox().SetBackgroundColor(theme.Colors.Background)
	rightSpacer := tview.NewBox().SetBackgroundColor(theme.Colors.Background)

	// Create button container with centered buttons
	buttonContainer := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(leftSpacer, 0, 1, false).
		AddItem(installButton, 12, 0, true).
		AddItem(middleSpacer, 2, 0, false).
		AddItem(cancelButton, 12, 0, false).
		AddItem(rightSpacer, 0, 1, false)

	// Create vertical spacers for button padding
	topSpacer := tview.NewBox().SetBackgroundColor(theme.Colors.Background)
	bottomSpacer := tview.NewBox().SetBackgroundColor(theme.Colors.Background)

	// Create the main layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true).
		AddItem(topSpacer, 1, 0, false).
		AddItem(buttonContainer, 3, 0, false).
		AddItem(bottomSpacer, 1, 0, false)

	// layout.SetBorder(true)
	// layout.SetBorderColor(theme.Colors.Border)
	// layout.SetTitle(" Script Details ")
	// layout.SetTitleColor(theme.Colors.Primary)

	// Set up input capture for navigation
	layout.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			s.app.Pages().RemovePage("scriptInfo")
			s.app.SetFocus(s.scriptList)

			return nil
		} else if event.Key() == tcell.KeyTab {
			// Tab between textview and buttons in the script info dialog
			currentFocus := s.app.GetFocus()
			if currentFocus == textView {
				s.app.SetFocus(installButton)
			} else if currentFocus == installButton {
				s.app.SetFocus(cancelButton)
			} else if currentFocus == cancelButton {
				s.app.SetFocus(textView)
			}

			return nil
		} else if event.Key() == tcell.KeyEnter {
			// Enter on textview focuses install button
			if s.app.GetFocus() == textView {
				s.app.SetFocus(installButton)

				return nil
			}
		}

		return event
	})

	// Show the page
	s.app.Pages().AddPage("scriptInfo", layout, true, true)
	s.app.SetFocus(textView)
}

// Hide hides the script selector.
func (s *ScriptSelector) Hide() {
	// Stop loading animation and indicator if running
	s.stopLoadingAnimation()

	if s.isLoading {
		s.isLoading = false
		s.app.Header().StopLoading()
	}

	// Remove the script selector page
	s.app.Pages().RemovePage("scriptSelector")

	// Restore focus to the appropriate list based on current page
	pageName, _ := s.app.Pages().GetFrontPage()
	if pageName == api.PageNodes {
		s.app.SetFocus(s.app.NodeList())
	} else if pageName == api.PageGuests {
		s.app.SetFocus(s.app.VMList())
	}
}

// Show displays the script selector.
func (s *ScriptSelector) Show() {
	// Ensure we have a valid node IP
	if s.nodeIP == "" {
		s.app.ShowMessage("Node IP address not available. Cannot connect to install scripts.")

		return
	}

	// Add the script selector page to the main app
	s.app.Pages().AddPage("scriptSelector", s.Pages, true, true)
	s.app.SetFocus(s.categoryList)

	// Set up input capture for category list
	s.categoryList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Remove any script info page first
			if s.app.Pages().HasPage("scriptInfo") {
				s.app.Pages().RemovePage("scriptInfo")
				s.app.SetFocus(s.scriptList)

				return nil
			}

			// Hide the script selector page
			s.Hide()

			return nil
		} else if event.Key() == tcell.KeyEnter {
			// Manually trigger the selection
			idx := s.categoryList.GetCurrentItem()
			if idx >= 0 && idx < len(s.categories) {
				category := s.categories[idx]
				s.fetchScriptsForCategory(category)
			}

			return nil
		} else if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			// Backspace on category list closes the page (handle both backspace variants)
			s.Hide()

			return nil
		} else if event.Key() == tcell.KeyRune {
			// Handle VI-like navigation (hjkl)
			switch event.Rune() {
			case 'j': // VI-like down navigation
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k': // VI-like up navigation
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'h': // VI-like left navigation - close page
				s.Hide()

				return nil
			case 'l': // VI-like right navigation - select category (same as Enter)
				idx := s.categoryList.GetCurrentItem()
				if idx >= 0 && idx < len(s.categories) {
					category := s.categories[idx]
					s.fetchScriptsForCategory(category)
				}

				return nil
			}
		}
		// Let arrow keys pass through for navigation
		return event
	})
}
