package components

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// Footer encapsulates the application footer
type Footer struct {
	*tview.TextView
	vncSessionCount   int
	autoRefreshActive bool
	baseText          string
}

// NewFooter creates a new application footer with key bindings
func NewFooter() *Footer {
	footer := tview.NewTextView()
	footer.SetTextAlign(tview.AlignLeft)
	footer.SetDynamicColors(true)

	baseText := "[yellow]Tab:[white]Switch  [yellow]F1:[white]Nodes  [yellow]F2:[white]Guests  [yellow]F3:[white]Tasks  [yellow]/:[white]Search  [yellow]M:[white]Menu  [yellow]A:[white]Auto-Refresh  [yellow]?:[white]Help  [yellow]Q:[white]Quit"

	f := &Footer{
		TextView:        footer,
		vncSessionCount: 0,
		baseText:        baseText,
	}

	// Set initial display with status indicators
	f.updateDisplay()

	return f
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

// UpdateAutoRefreshStatus updates the auto-refresh status display
func (f *Footer) UpdateAutoRefreshStatus(active bool) {
	f.autoRefreshActive = active
	f.updateDisplay()
}

// updateDisplay refreshes the footer text with current information
func (f *Footer) updateDisplay() {
	// Get the terminal width to calculate spacing
	_, _, width, _ := f.GetRect()

	// Use reasonable fallback if width not available yet
	if width == 0 {
		width = 120
	}

	f.updateDisplayWithWidth(width)
}

// updateDisplayWithWidth refreshes the footer text with a specific width
func (f *Footer) updateDisplayWithWidth(width int) {
	// Build status indicators for the right side
	var statusParts []string
	if f.vncSessionCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("[green]VNC:[white]%d", f.vncSessionCount))
	}
	if f.autoRefreshActive {
		statusParts = append(statusParts, "[cyan]Auto-Refresh:[white]ON")
	}

	statusText := strings.Join(statusParts, "  ")

	// If we have status text, create a right-aligned layout
	if statusText != "" {
		// Calculate available space for the base text
		// We need to account for the length of the status text (without color codes)
		statusLength := tview.TaggedStringWidth(statusText)

		// Ensure we have enough space
		if width > statusLength+10 { // +10 for some padding
			// Create padding to push status to the right
			baseLength := tview.TaggedStringWidth(f.baseText)
			padding := width - baseLength - statusLength - 2 // -2 for some spacing
			if padding > 0 {
				paddingStr := strings.Repeat(" ", padding)
				f.SetText(fmt.Sprintf("%s%s%s", f.baseText, paddingStr, statusText))
			} else {
				// Not enough space, just append normally
				f.SetText(fmt.Sprintf("%s  %s", f.baseText, statusText))
			}
		} else {
			// Terminal too narrow, just append normally
			f.SetText(fmt.Sprintf("%s  %s", f.baseText, statusText))
		}
	} else {
		// No status text, just show base text
		f.SetText(f.baseText)
	}
}
