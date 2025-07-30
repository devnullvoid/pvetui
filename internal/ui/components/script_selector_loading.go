package components

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
)

// =============================================================================
// LOADING AND ANIMATION
// =============================================================================

// startLoadingAnimation starts the loading animation.
func (s *ScriptSelector) startLoadingAnimation() {
	if s.animationTicker != nil {
		return // Already running
	}

	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIndex := 0

	s.animationTicker = time.NewTicker(100 * time.Millisecond)

	go func() {
		defer func() {
			if s.animationTicker != nil {
				s.animationTicker.Stop()
			}
		}()

		for range s.animationTicker.C {
			if !s.isLoading {
				return
			}

			spinner := spinners[spinnerIndex%len(spinners)]
			spinnerIndex++

			// Use a non-blocking update to prevent deadlocks
			go s.app.QueueUpdateDraw(func() {
				if s.loadingText != nil && s.isLoading {
					spinnerColor := theme.ColorToTag(theme.Colors.Warning)
					whiteColor := theme.ColorToTag(theme.Colors.Primary)
					grayColor := theme.ColorToTag(theme.Colors.Secondary)
					s.loadingText.SetText(fmt.Sprintf("[%s]Loading Scripts...[%s]\n\n%s Fetching scripts from GitHub\n\nThis may take a moment\n\n[%s]Press Backspace or Escape to cancel[%s]", spinnerColor, whiteColor, spinner, grayColor, whiteColor))
				}
			})
		}
	}()
}

// stopLoadingAnimation stops the loading animation.
func (s *ScriptSelector) stopLoadingAnimation() {
	if s.animationTicker != nil {
		s.animationTicker.Stop()
		s.animationTicker = nil
	}
}

// createLoadingPage creates a loading indicator page.
func (s *ScriptSelector) createLoadingPage() *tview.Flex {
	// Create animated loading text
	s.loadingText = tview.NewTextView()
	s.loadingText.SetDynamicColors(true)
	s.loadingText.SetTextAlign(tview.AlignCenter)

	spinnerColor := theme.ColorToTag(theme.Colors.Warning)
	whiteColor := theme.ColorToTag(theme.Colors.Primary)
	grayColor := theme.ColorToTag(theme.Colors.Secondary)
	// In createLoadingPage and loading animation, use spinner only if defined
	loadingMsg := fmt.Sprintf("[%s]Loading Scripts...[%s]\n\n⏳ Fetching scripts from GitHub\n\nThis may take a moment\n\n[%s]Press Backspace or Escape to cancel[%s]", spinnerColor, whiteColor, grayColor, whiteColor)
	s.loadingText.SetText(loadingMsg)

	// Set up input capture to allow canceling the loading
	s.loadingText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 || event.Key() == tcell.KeyEscape {
			// Cancel loading and go back to categories
			s.stopLoadingAnimation()
			s.isLoading = false
			s.app.header.StopLoading()
			s.pages.SwitchToPage("categories")
			s.app.SetFocus(s.categoryList)

			return nil
		} else if event.Key() == tcell.KeyRune {
			// Handle VI-like navigation
			switch event.Rune() {
			case 'h': // VI-like left navigation - go back to categories
				s.stopLoadingAnimation()
				s.isLoading = false
				s.app.header.StopLoading()
				s.pages.SwitchToPage("categories")
				s.app.SetFocus(s.categoryList)

				return nil
			}
		}

		return event
	})

	// Create the loading page layout
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).           // Top padding
		AddItem(s.loadingText, 8, 0, false). // Loading message (increased height)
		AddItem(nil, 0, 1, false)            // Bottom padding
}

// createResponsiveLayout creates a layout that adapts to terminal size.
func (s *ScriptSelector) createResponsiveLayout() *tview.Flex {
	// Create a responsive layout using proportional sizing with better ratios
	return tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false). // Left padding (flexible)
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).    // Top padding (flexible, smaller)
			AddItem(s.pages, 0, 8, true). // Main content (takes most space)
			AddItem(nil, 0, 1, false),    // Bottom padding (flexible, smaller)
						0, 6, true). // Main column (wider than before)
		AddItem(nil, 0, 1, false) // Right padding (flexible)
}
