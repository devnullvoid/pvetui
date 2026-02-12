package components

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
)

const appName = "pvetui"

// Header encapsulates the application header.
type Header struct {
	*tview.TextView

	mu             sync.Mutex
	isLoading      bool
	loadingText    string
	loadingCancel  context.CancelFunc
	app            *tview.Application
	currentProfile string // Track the current active profile
}

var _ HeaderComponent = (*Header)(nil)

// NewHeader creates a new application header.
func NewHeader() *Header {
	header := tview.NewTextView()
	header.SetTextAlign(tview.AlignCenter)
	header.SetText("pvetui")
	header.SetDynamicColors(true)
	header.SetBackgroundColor(theme.Colors.Header)
	header.SetTextColor(theme.Colors.HeaderText)

	return &Header{
		TextView: header,
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
	h.mu.Lock()
	if h.isLoading && h.loadingText == message {
		h.mu.Unlock()
		return
	}

	if h.isLoading && h.loadingCancel != nil {
		h.loadingCancel()
	}

	h.isLoading = true
	h.loadingText = message
	ctx, cancel := context.WithCancel(context.Background())
	h.loadingCancel = cancel
	h.mu.Unlock()

	go h.animateLoading(ctx)
}

// StopLoading stops the loading animation.
func (h *Header) StopLoading() {
	h.mu.Lock()
	if !h.isLoading {
		h.mu.Unlock()
		return
	}
	h.isLoading = false
	if h.loadingCancel != nil {
		h.loadingCancel()
		h.loadingCancel = nil
	}
	h.mu.Unlock()
}

// IsLoading reports whether the header is currently showing a loading state.
func (h *Header) IsLoading() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.isLoading
}

// ShowSuccess displays a success message temporarily.
func (h *Header) ShowSuccess(message string) {
	// Mark not loading before changing text to prevent race with animateLoading
	h.mu.Lock()
	h.isLoading = false
	if h.loadingCancel != nil {
		h.loadingCancel()
		h.loadingCancel = nil
	}
	h.mu.Unlock()
	h.StopLoading()
	h.SetText(theme.ReplaceSemanticTags("[success]✓ " + message + "[-]"))

	// Clear the message after 2 seconds (shorter than error messages)
	h.clearMessageAfterDelay(2 * time.Second)
}

// ShowError displays an error message temporarily.
func (h *Header) ShowError(message string) {
	h.mu.Lock()
	h.isLoading = false
	if h.loadingCancel != nil {
		h.loadingCancel()
		h.loadingCancel = nil
	}
	h.mu.Unlock()
	h.StopLoading()
	h.SetText(theme.ReplaceSemanticTags("[error]✗ " + message + "[-]"))

	// Clear the message after 3 seconds
	h.clearMessageAfterDelay(3 * time.Second)
}

// ShowWarning displays a warning message temporarily.
func (h *Header) ShowWarning(message string) {
	h.mu.Lock()
	h.isLoading = false
	if h.loadingCancel != nil {
		h.loadingCancel()
		h.loadingCancel = nil
	}
	h.mu.Unlock()
	h.StopLoading()
	h.SetText(theme.ReplaceSemanticTags("[warning]⚠ " + message + "[-]"))

	// Clear the message after 3 seconds
	h.clearMessageAfterDelay(3 * time.Second)
}

// formatProfileText creates the formatted header text for a profile.
func (h *Header) formatProfileText(profileName string) string {
	if profileName == "" {
		return appName
	}

	return theme.ReplaceSemanticTags(fmt.Sprintf("%s [info][%s[][-]", appName, profileName))
}

// ShowActiveProfile displays the active profile in the header.
func (h *Header) ShowActiveProfile(profileName string) {
	h.mu.Lock()
	h.isLoading = false
	if h.loadingCancel != nil {
		h.loadingCancel()
		h.loadingCancel = nil
	}
	h.mu.Unlock()
	h.StopLoading()
	h.currentProfile = profileName // Store the profile name
	h.SetText(h.formatProfileText(profileName))
}

// GetCurrentProfile returns the currently connected profile name.
func (h *Header) GetCurrentProfile() string {
	return h.currentProfile
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
				// Avoid overriding an active loading indicator that may have started after ShowSuccess/ShowError
				h.mu.Lock()
				loading := h.isLoading
				h.mu.Unlock()
				if loading {
					return
				}
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
func (h *Header) animateLoading(ctx context.Context) {
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	index := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.mu.Lock()
			loading := h.isLoading
			h.mu.Unlock()

			if !loading {
				return
			}

			if h.app != nil {
				spinnerChar := spinner[index]
				h.app.QueueUpdateDraw(func() {
					h.mu.Lock()
					if !h.isLoading {
						h.mu.Unlock()
						return
					}
					currentMessage := h.loadingText
					h.mu.Unlock()
					h.SetText(theme.ReplaceSemanticTags(fmt.Sprintf("[info]%s %s[-]", spinnerChar, currentMessage)))
				})
			}

			index = (index + 1) % len(spinner)
		}
	}
}
