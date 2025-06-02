package components

import (
	"github.com/rivo/tview"
)

// Footer encapsulates the application footer
type Footer struct {
	*tview.TextView
}

// NewFooter creates a new application footer with key bindings
func NewFooter() *Footer {
	footer := tview.NewTextView()
	footer.SetTextAlign(tview.AlignCenter)
	footer.SetDynamicColors(true)
	footer.SetText("[yellow]F1:[white]Nodes  [yellow]F2:[white]Guests  [yellow]/:[white]Search  [yellow]S:[white]Shell  [yellow]V:[white]VNC  [yellow]C:[white]Scripts  [yellow]M:[white]Menu  [yellow]Tab:[white]Next Tab  [yellow]Q:[white]Quit")

	return &Footer{
		TextView: footer,
	}
}

// UpdateKeybindings updates the footer text with custom key bindings
func (f *Footer) UpdateKeybindings(text string) {
	f.SetText(text)
}
