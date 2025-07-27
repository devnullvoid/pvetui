package components

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
)

const appName = "Proxmox TUI"

// Header encapsulates the application header.
type Header struct {
	*tview.TextView

	isLoading      bool
	loadingText    string
	stopLoading    chan bool
	app            *tview.Application
	currentProfile string // Track the current active profile
}

var _ HeaderComponent = (*Header)(nil)

// NewHeader creates a new application header.
func NewHeader() *Header {
	header := tview.NewTextView()
	header.SetTextAlign(tview.AlignCenter)
	header.SetText("Proxmox TUI")
	header.SetDynamicColors(true)
	header.SetBackgroundColor(theme.Colors.Header)
	header.SetTextColor(theme.Colors.HeaderText)

	return &Header{
		TextView:    header,
		stopLoading: make(chan bool, 1),
	}
}

// SetApp sets the application reference for UI updates.
func (h *Header) SetApp(app *tview.Application) {
	h.app = app
}

// SetTitle updates the header text.
func (h *Header) SetTitle(title string) {
	h.SetText(title)
}

// SetText updates the header text directly.
func (h *Header) SetText(text string) {
	h.TextView.SetText(text)
}

// ShowLoading displays an animated loading indicator.
func (h *Header) ShowLoading(message string) {
	if h.isLoading {
		h.StopLoading() // Stop any existing loading animation
	}

	h.isLoading = true
	h.loadingText = message
	h.stopLoading = make(chan bool, 1)

	// Start the loading animation
	go h.animateLoading()
}

// StopLoading stops the loading animation.
func (h *Header) StopLoading() {
	if h.isLoading {
		h.isLoading = false
		select {
		case h.stopLoading <- true:
		default:
		}
	}
}

// IsLoading reports whether the header is currently showing a loading state.
func (h *Header) IsLoading() bool {
	return h.isLoading
}

// ShowSuccess displays a success message temporarily.
func (h *Header) ShowSuccess(message string) {
	h.StopLoading()
	h.SetText(theme.ReplaceSemanticTags("[success]✓ " + message + "[-]"))

	// Clear the message after 2 seconds (shorter than error messages)
	h.clearMessageAfterDelay(2 * time.Second)
}

// ShowError displays an error message temporarily.
func (h *Header) ShowError(message string) {
	h.StopLoading()
	h.SetText(theme.ReplaceSemanticTags("[error]✗ " + message + "[-]"))

	// Clear the message after 3 seconds
	h.clearMessageAfterDelay(3 * time.Second)
}

// formatProfileText creates the formatted header text for a profile.
func (h *Header) formatProfileText(profileName string) string {
	if profileName == "" {
		return appName
	}

	return theme.ReplaceSemanticTags(fmt.Sprintf("%s - [info]%s[-]", appName, profileName))
}

// ShowActiveProfile displays the active profile in the header.
func (h *Header) ShowActiveProfile(profileName string) {
	h.StopLoading()
	h.currentProfile = profileName // Store the profile name
	h.SetText(h.formatProfileText(profileName))
}

// restoreProfile simply restores the profile display without stopping loading.
func (h *Header) restoreProfile() {
	h.SetText(h.formatProfileText(h.currentProfile))
}

// Add a helper to clear the header message after a delay.
func (h *Header) clearMessageAfterDelay(delay time.Duration) {
	go func() {
		time.Sleep(delay)

		if h.app != nil {
			h.app.QueueUpdateDraw(func() {
				// Restore the current profile if it exists, otherwise reset to default
				if h.currentProfile != "" {
					h.restoreProfile()
				} else {
					h.SetText(appName)
				}
			})
		}
	}()
}

// animateLoading displays an animated loading indicator.
func (h *Header) animateLoading() {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	index := 0

	for h.isLoading {
		select {
		case <-h.stopLoading:
			return
		default:
			if h.app != nil {
				h.app.QueueUpdateDraw(func() {
					spinnerChar := spinner[index]
					h.SetText(theme.ReplaceSemanticTags(fmt.Sprintf("[info]%s %s[-]", spinnerChar, h.loadingText)))
				})
			}

			index = (index + 1) % len(spinner)

			time.Sleep(100 * time.Millisecond)
		}
	}
}
