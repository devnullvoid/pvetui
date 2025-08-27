package components

import (
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// showMessage displays a message to the user.
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

	// Ensure modal gets focus in the next update cycle
	a.QueueUpdateDraw(func() {
		a.SetFocus(modal)
	})
}

// showMessageSafe displays a message to the user without using QueueUpdateDraw to avoid deadlocks.
// Use this when calling from contexts that might already be in a UI update cycle.
func (a *App) showMessageSafe(message string) {
	// Save current focus before showing modal
	a.lastFocus = a.GetFocus()

	modal := tview.NewModal().
		SetText(message).
		// SetBackgroundColor(theme.Colors.Background).
		SetTextColor(theme.Colors.Primary).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.removePageIfPresent("message_safe")
			// Restore focus to the last focused element
			if a.lastFocus != nil {
				a.SetFocus(a.lastFocus)
			}
		})

	a.pages.AddPage("message_safe", modal, false, true)
	a.SetFocus(modal)
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

// openScriptSelector opens the script selector dialog.
func (a *App) openScriptSelector(node *api.Node, vm *api.VM) {
	if a.config.SSHUser == "" {
		a.showMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")

		return
	}

	selector := NewScriptSelector(a, node, vm, a.config.SSHUser)
	selector.Show()
}
