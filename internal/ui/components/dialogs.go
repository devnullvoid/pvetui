package components

import (
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
)

// showMessage displays a message to the user.
// This simplified version sets focus directly without QueueUpdateDraw,
// which eliminates potential deadlocks when called from menu handlers
// or other UI event contexts. SetFocus will trigger a redraw automatically.
func (a *App) showMessage(message string) {
	// Save current focus before showing modal
	a.lastFocus = a.GetFocus()

	modal := tview.NewModal().
		SetText(message).
		// SetBackgroundColor(theme.Colors.Background).
		SetTextColor(theme.Colors.Primary).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.removePageIfPresent("message")
			// Restore focus to the last focused element
			if a.lastFocus != nil {
				a.SetFocus(a.lastFocus)
			}
		})

	a.pages.AddPage("message", modal, false, true)
	// Set focus directly - this triggers a redraw without queuing
	a.SetFocus(modal)
}

// showMessageSafe is now an alias to showMessage for backwards compatibility.
// Both use the same safe implementation that avoids QueueUpdateDraw deadlocks.
func (a *App) showMessageSafe(message string) {
	a.showMessage(message)
}

// showConfirmationDialog displays a confirmation dialog with Yes/No options.
// DEPRECATED: Use CreateConfirmDialog instead for consistency.
func (a *App) showConfirmationDialog(message string, onConfirm func()) {
	confirm := CreateConfirmDialog("Confirm", message, func() {
		a.removePageIfPresent("confirmation")
		onConfirm()
	}, func() {
		a.removePageIfPresent("confirmation")
	})
	a.pages.AddPage("confirmation", confirm, false, true)
	a.SetFocus(confirm)
}
