package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Header encapsulates the application header
type Header struct {
	*tview.TextView
}

// NewHeader creates a new application header
func NewHeader() *Header {
	header := tview.NewTextView()
	header.SetTextAlign(tview.AlignCenter)
	header.SetText("Proxmox CLI UI")
	header.SetDynamicColors(true)
	header.SetBackgroundColor(tcell.ColorBlue)
	header.SetTextColor(tcell.ColorWhite)
	
	return &Header{
		TextView: header,
	}
}

// SetTitle updates the header text
func (h *Header) SetTitle(title string) {
	h.SetText(title)
} 