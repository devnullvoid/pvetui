package components

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
)

// Header encapsulates the application header
type Header struct {
	*tview.TextView
	isLoading   bool
	loadingText string
	stopLoading chan bool
	app         *tview.Application
}

var _ HeaderComponent = (*Header)(nil)

// NewHeader creates a new application header
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

// SetApp sets the application reference for UI updates
func (h *Header) SetApp(app *tview.Application) {
	h.app = app
}

// SetTitle updates the header text
func (h *Header) SetTitle(title string) {
	h.SetText(title)
}

// ShowLoading displays an animated loading indicator
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

// StopLoading stops the loading animation
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

// ShowSuccess displays a success message temporarily
func (h *Header) ShowSuccess(message string) {
	h.StopLoading()
	h.SetText(fmt.Sprintf("[%s]✓ %s[-]", theme.Colors.Success, message))

	// Clear the message after 3 seconds
	go func() {
		time.Sleep(3 * time.Second)
		if h.app != nil {
			h.app.QueueUpdateDraw(func() {
				h.SetText("Proxmox TUI")
			})
		}
	}()
}

// ShowError displays an error message temporarily
func (h *Header) ShowError(message string) {
	h.StopLoading()
	h.SetText(fmt.Sprintf("[%s]✗ %s[-]", theme.Colors.Error, message))

	// Clear the message after 3 seconds
	go func() {
		time.Sleep(3 * time.Second)
		if h.app != nil {
			h.app.QueueUpdateDraw(func() {
				h.SetText("Proxmox TUI")
			})
		}
	}()
}

// animateLoading displays an animated loading indicator
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
					h.SetText(fmt.Sprintf("[%s]%s %s[-]", theme.Colors.Warning, spinnerChar, h.loadingText))
				})
			}
			index = (index + 1) % len(spinner)
			time.Sleep(100 * time.Millisecond)
		}
	}
}
