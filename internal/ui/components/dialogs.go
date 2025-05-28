package components

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showMessage displays a message to the user
func (a *App) showMessage(message string) {
	modal := tview.NewModal().
		SetText(message).
		SetBackgroundColor(tcell.ColorGray).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("message")
		})

	a.pages.AddPage("message", modal, false, true)
}

// showConfirmationDialog displays a confirmation dialog with Yes/No options
func (a *App) showConfirmationDialog(message string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(message).
		SetBackgroundColor(tcell.ColorGray).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("confirmation")
			if buttonIndex == 0 {
				// Yes was selected
				onConfirm()
			}
		})

	a.pages.AddPage("confirmation", modal, false, true)
}

// openScriptSelector opens the script selector dialog
func (a *App) openScriptSelector(node *api.Node, vm *api.VM) {
	if a.config.SSHUser == "" {
		a.showMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")
		return
	}

	selector := NewScriptSelector(a, node, vm, a.config.SSHUser)
	selector.Show()
}
