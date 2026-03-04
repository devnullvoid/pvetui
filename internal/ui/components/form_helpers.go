package components

import (
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
)

// newStandardForm creates a tview form with the project's standard label color.
func newStandardForm() *tview.Form {
	return tview.NewForm().SetLabelColor(theme.Colors.HeaderText)
}

// NewStandardForm creates a tview form with the project's standard label color.
// This exported variant is intended for plugin UIs outside the components package.
func NewStandardForm() *tview.Form {
	return newStandardForm()
}
