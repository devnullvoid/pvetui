// Package display provides presentation helpers shared by console and TUI flows.
package display

// IconText prefixes text with icon when icon display is enabled.
func IconText(icon, text string, showIcons bool) string {
	if !showIcons || icon == "" {
		return text
	}
	return icon + " " + text
}
