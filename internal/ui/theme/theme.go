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
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/config"
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

	// Additional tview theme colors
	Title        tcell.Color // For tview TitleColor
	Contrast     tcell.Color // For tview ContrastBackgroundColor
	MoreContrast tcell.Color // For tview MoreContrastBackgroundColor
	Inverse      tcell.Color // For tview InverseTextColor

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
	Header:     tcell.ColorDefault,
	HeaderText: tcell.ColorYellow,
	Footer:     tcell.ColorDefault,
	FooterText: tcell.ColorWhite,

	Title:        tcell.ColorWhite,
	Contrast:     tcell.ColorBlue,
	MoreContrast: tcell.ColorFuchsia,
	Inverse:      tcell.ColorBlack,

	StatusRunning: tcell.ColorGreen,
	StatusStopped: tcell.ColorMaroon,
	StatusPending: tcell.ColorYellow,
	StatusError:   tcell.ColorRed,

	UsageLow:      tcell.ColorGreen,
	UsageMedium:   tcell.ColorYellow,
	UsageHigh:     tcell.ColorRed,
	UsageCritical: tcell.ColorFuchsia,
}

// Only expose semantic tags that map directly to user-themeable colors
var semanticTagMap = map[string]func() tcell.Color{
	"primary":   func() tcell.Color { return Colors.Primary },
	"secondary": func() tcell.Color { return Colors.Secondary },
	"tertiary":  func() tcell.Color { return Colors.Tertiary },
	"success":   func() tcell.Color { return Colors.Success },
	"warning":   func() tcell.Color { return Colors.Warning },
	"error":     func() tcell.Color { return Colors.Error },
	"info":      func() tcell.Color { return Colors.Info },
	"selection": func() tcell.Color { return Colors.Selection },
	"header":    func() tcell.Color { return Colors.HeaderText },
	"footer":    func() tcell.Color { return Colors.FooterText },
	"title":     func() tcell.Color { return Colors.Title },
}

// ReplaceSemanticTags replaces semantic tags like [primary] with the current theme color tag.
func ReplaceSemanticTags(s string) string {
	for tag, colorFunc := range semanticTagMap {
		color := colorFunc()
		colorTag := ColorToTag(color)
		s = strings.ReplaceAll(s, "["+tag+"]", "["+colorTag+"]")
	}
	return s
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
	case tcell.ColorDefault:
		return "default"
	case tcell.ColorBlack:
		return "black"
	case tcell.ColorMaroon:
		return "maroon"
	case tcell.ColorGreen:
		return "green"
	case tcell.ColorOlive:
		return "olive"
	case tcell.ColorNavy:
		return "navy"
	case tcell.ColorPurple:
		return "purple"
	case tcell.ColorTeal:
		return "teal"
	case tcell.ColorSilver:
		return "silver"
	case tcell.ColorGray:
		return "gray"
	case tcell.ColorRed:
		return "red"
	case tcell.ColorLime:
		return "lime"
	case tcell.ColorYellow:
		return "yellow"
	case tcell.ColorBlue:
		return "blue"
	case tcell.ColorFuchsia:
		return "fuchsia"
	case tcell.ColorAqua:
		return "aqua"
	case tcell.ColorWhite:
		return "white"
	default:
		return fmt.Sprintf("#%06x", c.Hex())
	}
}

// ApplyToTview sets the global tview.Styles to match the semantic theme colors.
func ApplyToTview() {
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    Colors.Background,
		ContrastBackgroundColor:     Colors.Contrast,
		MoreContrastBackgroundColor: Colors.Selection,
		BorderColor:                 Colors.Border,
		TitleColor:                  Colors.Title,
		GraphicsColor:               Colors.Info,
		PrimaryTextColor:            Colors.Primary,
		SecondaryTextColor:          Colors.Secondary,
		TertiaryTextColor:           Colors.Tertiary,
		InverseTextColor:            Colors.Inverse,
		ContrastSecondaryTextColor:  Colors.Selection,
	}
}

// BuiltInThemes defines the available built-in themes.
var BuiltInThemes = map[string]map[string]string{
	"default": {
		"primary":       "white",
		"secondary":     "gray",
		"tertiary":      "aqua",
		"success":       "green",
		"warning":       "yellow",
		"error":         "red",
		"info":          "blue",
		"background":    "default",
		"border":        "gray",
		"selection":     "blue",
		"header":        "navy",
		"headertext":    "yellow",
		"footer":        "default",
		"footertext":    "white",
		"title":         "white",
		"contrast":      "blue",
		"morecontrast":  "fuchsia",
		"inverse":       "black",
		"statusrunning": "green",
		"statusstopped": "maroon",
		"statuspending": "yellow",
		"statuserror":   "red",
		"usagelow":      "green",
		"usagemedium":   "yellow",
		"usagehigh":     "red",
		"usagecritical": "fuchsia",
	},
	"catppuccin-mocha": {
		"primary":       "#cdd6f4",
		"secondary":     "#bac2de",
		"tertiary":      "#a6adc8",
		"success":       "#a6e3a1",
		"warning":       "#f9e2af",
		"error":         "#f38ba8",
		"info":          "#89b4fa",
		"background":    "#1e1e2e",
		"border":        "#45475a",
		"selection":     "#585b70",
		"header":        "#313244",
		"headertext":    "#f5e0dc",
		"footer":        "#313244",
		"footertext":    "#cdd6f4",
		"title":         "#b4befe",
		"contrast":      "#313244",
		"morecontrast":  "#181825",
		"inverse":       "#1e1e2e",
		"statusrunning": "#a6e3a1",
		"statusstopped": "#f38ba8",
		"statuspending": "#f9e2af",
		"statuserror":   "#f38ba8",
		"usagelow":      "#a6e3a1",
		"usagemedium":   "#f9e2af",
		"usagehigh":     "#fab387",
		"usagecritical": "#f38ba8",
	},
}

// ResolveTheme merges the selected built-in theme with user overrides.
func ResolveTheme(cfg *config.ThemeConfig) map[string]string {
	base := BuiltInThemes["default"]
	if cfg != nil && cfg.Name != "" {
		if t, ok := BuiltInThemes[cfg.Name]; ok {
			base = t
		}
	}
	// Copy base to avoid mutation
	resolved := make(map[string]string)
	for k, v := range base {
		resolved[k] = v
	}
	if cfg != nil {
		for k, v := range cfg.Colors {
			resolved[k] = v
		}
	}
	return resolved
}

// ApplyCustomTheme applies the resolved theme to the Colors struct.
// Users can select a built-in theme by name and override any color.
func ApplyCustomTheme(cfg *config.ThemeConfig) {
	resolved := ResolveTheme(cfg)
	for key, val := range resolved {
		c := parseColor(val)
		switch key {
		case "primary":
			Colors.Primary = c
		case "secondary":
			Colors.Secondary = c
		case "tertiary":
			Colors.Tertiary = c
		case "success":
			Colors.Success = c
		case "warning":
			Colors.Warning = c
		case "error":
			Colors.Error = c
		case "info":
			Colors.Info = c
		case "background":
			Colors.Background = c
		case "border":
			Colors.Border = c
		case "selection":
			Colors.Selection = c
		case "header":
			Colors.Header = c
		case "headertext":
			Colors.HeaderText = c
		case "footer":
			Colors.Footer = c
		case "footertext":
			Colors.FooterText = c
		case "title":
			Colors.Title = c
		case "contrast":
			Colors.Contrast = c
		case "morecontrast":
			Colors.MoreContrast = c
		case "inverse":
			Colors.Inverse = c
		case "statusrunning":
			Colors.StatusRunning = c
		case "statusstopped":
			Colors.StatusStopped = c
		case "statuspending":
			Colors.StatusPending = c
		case "statuserror":
			Colors.StatusError = c
		case "usagelow":
			Colors.UsageLow = c
		case "usagemedium":
			Colors.UsageMedium = c
		case "usagehigh":
			Colors.UsageHigh = c
		case "usagecritical":
			Colors.UsageCritical = c
		}
	}
}

// parseColor parses a color string (ANSI name, W3C name, or hex code) to tcell.Color.
func parseColor(s string) tcell.Color {
	if strings.EqualFold(s, "default") {
		return tcell.ColorDefault
	}
	return tcell.GetColor(s)
}
