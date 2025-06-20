package components

import (
	"fmt"

	"github.com/rivo/tview"
)

// Footer encapsulates the application footer
type Footer struct {
	*tview.TextView
	vncSessionCount int
	baseText        string
}

// NewFooter creates a new application footer with key bindings
func NewFooter() *Footer {
	footer := tview.NewTextView()
	footer.SetTextAlign(tview.AlignCenter)
	footer.SetDynamicColors(true)

	baseText := "[yellow]Tab:[white]Switch  [yellow]/:[white]Search  [yellow]M:[white]Menu  [yellow]?:[white]Help  [yellow]Q:[white]Quit"
	footer.SetText(baseText)

	return &Footer{
		TextView:        footer,
		vncSessionCount: 0,
		baseText:        baseText,
	}
}

// UpdateKeybindings updates the footer text with custom key bindings
func (f *Footer) UpdateKeybindings(text string) {
	f.baseText = text
	f.updateDisplay()
}

// UpdateVNCSessionCount updates the VNC session count display
func (f *Footer) UpdateVNCSessionCount(count int) {
	f.vncSessionCount = count
	f.updateDisplay()
}

// updateDisplay refreshes the footer text with current information
func (f *Footer) updateDisplay() {
	text := f.baseText
	if f.vncSessionCount > 0 {
		text = fmt.Sprintf("%s  [green]VNC:[white]%d", text, f.vncSessionCount)
	}
	f.SetText(text)
}
