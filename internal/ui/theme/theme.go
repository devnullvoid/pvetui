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
	// Gruvbox (https://github.com/morhetz/gruvbox)
	"gruvbox": {
		"primary":       "#ebdbb2",
		"secondary":     "#bdae93",
		"tertiary":      "#a89984",
		"success":       "#b8bb26",
		"warning":       "#fabd2f",
		"error":         "#fb4934",
		"info":          "#83a598",
		"background":    "#282828",
		"border":        "#504945",
		"selection":     "#665c54",
		"header":        "#3c3836",
		"headertext":    "#fe8019",
		"footer":        "#3c3836",
		"footertext":    "#ebdbb2",
		"title":         "#d3869b",
		"contrast":      "#3c3836",
		"morecontrast":  "#1d2021",
		"inverse":       "#282828",
		"statusrunning": "#b8bb26",
		"statusstopped": "#fb4934",
		"statuspending": "#fabd2f",
		"statuserror":   "#fb4934",
		"usagelow":      "#b8bb26",
		"usagemedium":   "#fabd2f",
		"usagehigh":     "#fe8019",
		"usagecritical": "#fb4934",
	},
	// Nord (https://www.nordtheme.com/docs/colors-and-palettes)
	"nord": {
		"primary":       "#d8dee9",
		"secondary":     "#e5e9f0",
		"tertiary":      "#eceff4",
		"success":       "#a3be8c",
		"warning":       "#ebcb8b",
		"error":         "#bf616a",
		"info":          "#5e81ac",
		"background":    "#2e3440",
		"border":        "#4c566a",
		"selection":     "#434c5e",
		"header":        "#3b4252",
		"headertext":    "#88c0d0",
		"footer":        "#3b4252",
		"footertext":    "#d8dee9",
		"title":         "#b48ead",
		"contrast":      "#3b4252",
		"morecontrast":  "#242933",
		"inverse":       "#2e3440",
		"statusrunning": "#a3be8c",
		"statusstopped": "#bf616a",
		"statuspending": "#ebcb8b",
		"statuserror":   "#bf616a",
		"usagelow":      "#a3be8c",
		"usagemedium":   "#ebcb8b",
		"usagehigh":     "#d08770",
		"usagecritical": "#bf616a",
	},
	// Rose Pine (https://rosepinetheme.com/palette)
	"rose-pine": {
		"primary":       "#e0def4",
		"secondary":     "#908caa",
		"tertiary":      "#6e6a86",
		"success":       "#9ccfd8",
		"warning":       "#f6c177",
		"error":         "#eb6f92",
		"info":          "#31748f",
		"background":    "#191724",
		"border":        "#26233a",
		"selection":     "#403d52",
		"header":        "#232136",
		"headertext":    "#e0def4",
		"footer":        "#232136",
		"footertext":    "#e0def4",
		"title":         "#c4a7e7",
		"contrast":      "#232136",
		"morecontrast":  "#1f1d2e",
		"inverse":       "#191724",
		"statusrunning": "#9ccfd8",
		"statusstopped": "#eb6f92",
		"statuspending": "#f6c177",
		"statuserror":   "#eb6f92",
		"usagelow":      "#9ccfd8",
		"usagemedium":   "#f6c177",
		"usagehigh":     "#ea9a97",
		"usagecritical": "#eb6f92",
	},
	// Tokyo Night (https://github.com/folke/tokyonight.nvim#palette)
	"tokyonight": {
		"primary":       "#c0caf5",
		"secondary":     "#a9b1d6",
		"tertiary":      "#9aa5ce",
		"success":       "#9ece6a",
		"warning":       "#e0af68",
		"error":         "#f7768e",
		"info":          "#7aa2f7",
		"background":    "#1a1b26",
		"border":        "#414868",
		"selection":     "#33467c",
		"header":        "#24283b",
		"headertext":    "#bb9af7",
		"footer":        "#24283b",
		"footertext":    "#c0caf5",
		"title":         "#bb9af7",
		"contrast":      "#24283b",
		"morecontrast":  "#16161e",
		"inverse":       "#1a1b26",
		"statusrunning": "#9ece6a",
		"statusstopped": "#f7768e",
		"statuspending": "#e0af68",
		"statuserror":   "#f7768e",
		"usagelow":      "#9ece6a",
		"usagemedium":   "#e0af68",
		"usagehigh":     "#ff9e64",
		"usagecritical": "#f7768e",
	},
	// Solarized Dark (https://ethanschoonover.com/solarized/)
	"solarized": {
		"primary":       "#839496",
		"secondary":     "#93a1a1",
		"tertiary":      "#586e75",
		"success":       "#859900",
		"warning":       "#b58900",
		"error":         "#dc322f",
		"info":          "#268bd2",
		"background":    "#002b36",
		"border":        "#073642",
		"selection":     "#073642",
		"header":        "#073642",
		"headertext":    "#b58900",
		"footer":        "#073642",
		"footertext":    "#839496",
		"title":         "#6c71c4",
		"contrast":      "#073642",
		"morecontrast":  "#001f27",
		"inverse":       "#002b36",
		"statusrunning": "#859900",
		"statusstopped": "#dc322f",
		"statuspending": "#b58900",
		"statuserror":   "#dc322f",
		"usagelow":      "#859900",
		"usagemedium":   "#b58900",
		"usagehigh":     "#cb4b16",
		"usagecritical": "#dc322f",
	},
	// Dracula (https://draculatheme.com/contribute)
	"dracula": {
		"primary":       "#f8f8f2",
		"secondary":     "#bd93f9",
		"tertiary":      "#6272a4",
		"success":       "#50fa7b",
		"warning":       "#f1fa8c",
		"error":         "#ff5555",
		"info":          "#8be9fd",
		"background":    "#282a36",
		"border":        "#44475a",
		"selection":     "#44475a",
		"header":        "#44475a",
		"headertext":    "#bd93f9",
		"footer":        "#44475a",
		"footertext":    "#f8f8f2",
		"title":         "#ff79c6",
		"contrast":      "#44475a",
		"morecontrast":  "#1e1f29",
		"inverse":       "#282a36",
		"statusrunning": "#50fa7b",
		"statusstopped": "#ff5555",
		"statuspending": "#f1fa8c",
		"statuserror":   "#ff5555",
		"usagelow":      "#50fa7b",
		"usagemedium":   "#f1fa8c",
		"usagehigh":     "#ffb86c",
		"usagecritical": "#ff5555",
	},
	// Kanagawa (https://github.com/rebelot/kanagawa.nvim#palette)
	"kanagawa": {
		"primary":       "#dcd7ba",
		"secondary":     "#a6a69c",
		"tertiary":      "#7e9cd8",
		"success":       "#98bb6c",
		"warning":       "#ffa066",
		"error":         "#e46876",
		"info":          "#7fb4ca",
		"background":    "#1f1f28",
		"border":        "#2a2a37",
		"selection":     "#223249",
		"header":        "#2a2a37",
		"headertext":    "#dcd7ba",
		"footer":        "#2a2a37",
		"footertext":    "#dcd7ba",
		"title":         "#957fb8",
		"contrast":      "#2a2a37",
		"morecontrast":  "#16161d",
		"inverse":       "#1f1f28",
		"statusrunning": "#98bb6c",
		"statusstopped": "#e46876",
		"statuspending": "#ffa066",
		"statuserror":   "#e46876",
		"usagelow":      "#98bb6c",
		"usagemedium":   "#ffa066",
		"usagehigh":     "#e6c384",
		"usagecritical": "#e46876",
	},
	// Everforest (https://github.com/sainnhe/everforest#palette)
	"everforest": {
		"primary":       "#d3c6aa",
		"secondary":     "#a7c080",
		"tertiary":      "#e67e80",
		"success":       "#a7c080",
		"warning":       "#dbbc7f",
		"error":         "#e67e80",
		"info":          "#7fbbb3",
		"background":    "#2b3339",
		"border":        "#4f5b58",
		"selection":     "#4f5b58",
		"header":        "#4f5b58",
		"headertext":    "#d3c6aa",
		"footer":        "#4f5b58",
		"footertext":    "#d3c6aa",
		"title":         "#e69875",
		"contrast":      "#4f5b58",
		"morecontrast":  "#232a2e",
		"inverse":       "#2b3339",
		"statusrunning": "#a7c080",
		"statusstopped": "#e67e80",
		"statuspending": "#dbbc7f",
		"statuserror":   "#e67e80",
		"usagelow":      "#a7c080",
		"usagemedium":   "#dbbc7f",
		"usagehigh":     "#e69875",
		"usagecritical": "#e67e80",
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
