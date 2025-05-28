package components

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Header encapsulates the application header
type Header struct {
	*tview.TextView
	isLoading   bool
	loadingText string
	stopLoading chan bool
	app         *tview.Application
}

// NewHeader creates a new application header
func NewHeader() *Header {
	header := tview.NewTextView()
	header.SetTextAlign(tview.AlignCenter)
	header.SetText("Proxmox TUI")
	header.SetDynamicColors(true)
	header.SetBackgroundColor(tcell.ColorBlue)
	header.SetTextColor(tcell.ColorGray)

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

// ShowSuccess displays a success message temporarily
func (h *Header) ShowSuccess(message string) {
	h.StopLoading()
	h.SetText(fmt.Sprintf("[green]✓ %s[-]", message))

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
	h.SetText(fmt.Sprintf("[red]✗ %s[-]", message))

	// Clear the message after 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		if h.app != nil {
			h.app.QueueUpdateDraw(func() {
				h.SetText("Proxmox TUI")
			})
		}
	}()
}

// animateLoading creates a spinning animation for loading
func (h *Header) animateLoading() {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopLoading:
			return
		case <-ticker.C:
			if h.app != nil {
				spinner := spinners[i%len(spinners)]
				h.app.QueueUpdateDraw(func() {
					h.SetText(fmt.Sprintf("[yellow]%s [-]%s...", spinner, h.loadingText))
				})
				i++
			}
		}
	}
}
