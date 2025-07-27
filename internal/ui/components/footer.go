package components

import (
	"fmt"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
)

// Footer encapsulates the application footer.
type Footer struct {
	*tview.TextView

	vncSessionCount   int
	autoRefreshActive bool
	baseText          string
	refreshCountdown  int // seconds until next auto-refresh
	isLoading         bool
	spinnerIndex      int
}

var _ FooterComponent = (*Footer)(nil)

// NewFooter creates a new application footer with key bindings.
func NewFooter() *Footer {
	footer := tview.NewTextView()
	footer.SetTextAlign(tview.AlignLeft)
	footer.SetDynamicColors(true)
	footer.SetBackgroundColor(theme.Colors.Footer)

	// Remove the hardcoded baseText and always use FormatFooterText for the footer text
	// In the Footer constructor and any other relevant place, set the footer text using FormatFooterText()

	f := &Footer{
		TextView:        footer,
		vncSessionCount: 0,
		baseText:        "", // Initialize baseText to empty
	}

	// Set initial display with status indicators
	f.updateDisplay()

	return f
}

// FormatFooterText builds the footer key binding text from config.
func FormatFooterText(keys config.KeyBindings) string {
	return fmt.Sprintf(
		"[%s]%s:[%s]Nodes  [%s]%s:[%s]Guests  [%s]%s:[%s]Tasks  [%s]%s:[%s]Search  [%s]%s:[%s]Menu  [%s]%s:[%s]Global  [%s]%s:[%s]Help  [%s]%s:[%s]Quit",
		theme.Colors.HeaderText, keys.NodesPage, theme.Colors.Primary,
		theme.Colors.HeaderText, keys.GuestsPage, theme.Colors.Primary,
		theme.Colors.HeaderText, keys.TasksPage, theme.Colors.Primary,
		theme.Colors.HeaderText, keys.Search, theme.Colors.Primary,
		theme.Colors.HeaderText, keys.Menu, theme.Colors.Primary,
		theme.Colors.HeaderText, keys.GlobalMenu, theme.Colors.Primary,
		theme.Colors.HeaderText, keys.Help, theme.Colors.Primary,
		theme.Colors.HeaderText, keys.Quit, theme.Colors.Primary,
	)
}

// UpdateKeybindings updates the footer text with custom key bindings.
func (f *Footer) UpdateKeybindings(text string) {
	f.baseText = text
	f.updateDisplay()
}

// UpdateVNCSessionCount updates the VNC session count display.
func (f *Footer) UpdateVNCSessionCount(count int) {
	f.vncSessionCount = count
	f.updateDisplay()
}

// UpdateAutoRefreshStatus updates the auto-refresh status display.
func (f *Footer) UpdateAutoRefreshStatus(active bool) {
	f.autoRefreshActive = active
	f.updateDisplay()
}

// UpdateAutoRefreshCountdown updates the countdown for the next auto-refresh.
func (f *Footer) UpdateAutoRefreshCountdown(seconds int) {
	f.refreshCountdown = seconds
	f.updateDisplay()
}

// SetLoading sets the loading state and resets the spinner.
func (f *Footer) SetLoading(loading bool) {
	f.isLoading = loading
	if !loading {
		f.spinnerIndex = 0
	}

	f.updateDisplay()
}

// IsLoading returns true if the footer is currently showing a loading spinner.
func (f *Footer) IsLoading() bool {
	return f.isLoading
}

// TickSpinner advances the loading spinner animation once.
func (f *Footer) TickSpinner() {
	f.spinnerIndex++
	f.updateDisplay()
}

// updateDisplay refreshes the footer text with current information.
func (f *Footer) updateDisplay() {
	// Get the terminal width to calculate spacing
	_, _, width, _ := f.GetRect()

	// Use reasonable fallback if width not available yet
	if width == 0 {
		width = 120
	}

	f.updateDisplayWithWidth(width)
}

// updateDisplayWithWidth refreshes the footer text with a specific width.
func (f *Footer) updateDisplayWithWidth(width int) {
	// Build status indicators for the right side
	var statusParts []string

	// Add VNC session count if any
	if f.vncSessionCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("[info]VNC:[secondary]%d", f.vncSessionCount))
	}

	// Add auto-refresh status if active
	if f.autoRefreshActive {
		if f.isLoading {
			spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			spinner := spinners[f.spinnerIndex%len(spinners)]
			statusParts = append(statusParts, fmt.Sprintf("[warning]%s Refreshing...[secondary]", spinner))
		} else if f.refreshCountdown > 0 {
			statusParts = append(statusParts, fmt.Sprintf("[info]Auto-Refresh:[secondary]ON ([warning]%ds[secondary])", f.refreshCountdown))
		} else {
			statusParts = append(statusParts, "[info]Auto-Refresh:[secondary]ON")
		}
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
