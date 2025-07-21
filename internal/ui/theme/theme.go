// Package theme provides color theming support for the Proxmox TUI application.
//
// This package defines semantic color constants that map to standard ANSI colors,
// allowing users to customize the application appearance through their terminal
// emulator's color scheme while maintaining consistent semantic meaning.
//
// Color Semantics:
//   - Primary: Main text and UI elements
//   - Secondary: Supporting text and labels
//   - Accent: Highlighted elements and important information
//   - Success: Positive states (running, online, etc.)
//   - Warning: Caution states (partial failures, etc.)
//   - Error: Error states (stopped, offline, etc.)
//   - Info: Informational elements (descriptions, metadata)
//
// Usage:
//
//	import "github.com/devnullvoid/proxmox-tui/internal/ui/theme"
//
//	// Use semantic colors instead of hardcoded tcell.Color values
//	cell.SetTextColor(theme.Colors.Primary)
//	cell.SetTextColor(theme.Colors.Success)
//
// Terminal Emulator Theming:
//
// Users can customize the application appearance by configuring their terminal
// emulator's color scheme. Popular themes include:
//   - Dracula: Dark theme with purple accents
//   - Nord: Arctic-inspired dark theme
//   - Solarized: Carefully designed color palette
//   - Gruvbox: Retro groove color scheme
//   - Catppuccin: Soothing pastel theme
//
// The application will automatically adapt to the terminal's color scheme
// while maintaining semantic color relationships.
package theme

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Colors defines the semantic color palette for the application.
// These colors map to standard ANSI colors that can be customized
// through terminal emulator themes.
var Colors = struct {
	// Primary colors
	Primary   tcell.Color // Main text and UI elements
	Secondary tcell.Color // Supporting text and labels
	Tertiary  tcell.Color // Supporting text and labels

	// Semantic colors
	Success tcell.Color // Positive states (running, online, etc.)
	Warning tcell.Color // Caution states (partial failures, etc.)
	Error   tcell.Color // Error states (stopped, offline, etc.)
	Info    tcell.Color // Informational elements (descriptions, metadata)

	// UI element colors
	Background tcell.Color // Main background
	Border     tcell.Color // Border and separator lines
	Selection  tcell.Color // Selected item background
	Header     tcell.Color // Header background
	HeaderText tcell.Color // Header text color
	Footer     tcell.Color // Footer background
	FooterText tcell.Color // Footer text color

	// Status-specific colors
	StatusRunning tcell.Color // Running VM/container status
	StatusStopped tcell.Color // Stopped VM/container status
	StatusPending tcell.Color // Pending operation status
	StatusError   tcell.Color // Error status

	// Resource usage colors
	UsageLow      tcell.Color // Low resource usage (< 50%)
	UsageMedium   tcell.Color // Medium resource usage (50-75%)
	UsageHigh     tcell.Color // High resource usage (75-90%)
	UsageCritical tcell.Color // Critical resource usage (> 90%)
}{
	// Map to standard ANSI colors that work well with terminal themes
	Primary:   tcell.ColorWhite,
	Secondary: tcell.ColorGray,
	Tertiary:  tcell.ColorAqua,

	Success: tcell.ColorGreen,
	Warning: tcell.ColorYellow,
	Error:   tcell.ColorRed,
	Info:    tcell.ColorBlue,

	Background: tcell.ColorDefault,
	Border:     tcell.ColorGray,
	Selection:  tcell.ColorBlue,
	Header:     tcell.ColorBlue,
	HeaderText: tcell.ColorYellow,
	Footer:     tcell.ColorDefault,
	FooterText: tcell.ColorWhite,

	StatusRunning: tcell.ColorGreen,
	StatusStopped: tcell.ColorRed,
	StatusPending: tcell.ColorYellow,
	StatusError:   tcell.ColorRed,

	UsageLow:      tcell.ColorGreen,
	UsageMedium:   tcell.ColorYellow,
	UsageHigh:     tcell.ColorRed,
	UsageCritical: tcell.ColorRed,
}

// GetStatusColor returns the appropriate color for a given status.
func GetStatusColor(status string) tcell.Color {
	switch status {
	case "running", "OK":
		return Colors.StatusRunning
	case "stopped", "error":
		return Colors.StatusStopped
	case "pending", "migrating":
		return Colors.StatusPending
	default:
		return Colors.Secondary
	}
}

// GetUsageColor returns the appropriate color for resource usage percentage.
func GetUsageColor(percentage float64) tcell.Color {
	switch {
	case percentage < 50:
		return Colors.UsageLow
	case percentage < 75:
		return Colors.UsageMedium
	case percentage < 90:
		return Colors.UsageHigh
	default:
		return Colors.UsageCritical
	}
}

// IsDarkTheme returns true if the current terminal appears to be using a dark theme.
// This is a simple heuristic based on the background color.
func IsDarkTheme() bool {
	// Default to assuming dark theme for better compatibility
	// Users can override this through terminal emulator settings
	return true
}

// ColorToTag returns a tview color tag string for a tcell.Color
func ColorToTag(c tcell.Color) string {
	switch c {
	case tcell.ColorBlack:
		return "black"
	case tcell.ColorRed:
		return "red"
	case tcell.ColorGreen:
		return "green"
	case tcell.ColorYellow:
		return "yellow"
	case tcell.ColorBlue:
		return "blue"
	case tcell.ColorPurple:
		return "purple"
	case tcell.ColorTeal:
		return "teal"
	case tcell.ColorWhite:
		return "white"
	case tcell.ColorGray:
		return "gray"
	case tcell.ColorLightBlue:
		return "aqua"
	default:
		return fmt.Sprintf("#%06x", c.Hex())
	}
}

// ApplyToTview sets the global tview.Styles to match the semantic theme colors.
func ApplyToTview() {
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    Colors.Background,
		ContrastBackgroundColor:     Colors.Tertiary,
		MoreContrastBackgroundColor: Colors.Selection,
		BorderColor:                 Colors.Border,
		TitleColor:                  Colors.HeaderText,
		GraphicsColor:               Colors.Info,
		PrimaryTextColor:            Colors.Primary,
		SecondaryTextColor:          Colors.Secondary,
		TertiaryTextColor:           Colors.Tertiary,
		InverseTextColor:            Colors.HeaderText,
		ContrastSecondaryTextColor:  Colors.FooterText,
	}
}
