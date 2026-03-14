package components

import (
	"github.com/devnullvoid/pvetui/internal/version"
	"github.com/gdamore/tcell/v2"

	"github.com/devnullvoid/pvetui/internal/ui/models"
)

// ShowGlobalContextMenu displays the global context menu for app-wide actions.
func (a *App) ShowGlobalContextMenu() {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	ansibleEnabled := false
	if plugin, ok := a.plugins["ansible"]; ok && plugin != nil {
		_, ansibleEnabled = plugin.(GlobalActionPlugin)
	}

	// Create menu items for global actions
	menuItems := []string{
		"Connection Profiles",
		"Manage Plugins",
		"Create VM",
		"Create LXC",
	}
	shortcuts := []rune{'p', 'm', 'c', 'l'}

	if ansibleEnabled {
		menuItems = append(menuItems, "Ansible Toolkit")
		shortcuts = append(shortcuts, 'A')
	}

	menuItems = append(menuItems,
		"Refresh All Data",
		"Toggle Auto-Refresh",
		"Help",
		"About",
		"Quit",
	)

	// Define custom shortcuts for global menu
	shortcuts = append(shortcuts, 'r', 'a', '?', 'i', 'q')

	menu := NewContextMenuWithShortcuts(" Global Actions ", menuItems, shortcuts, func(index int, action string) {
		a.CloseContextMenu()

		switch action {
		case "Connection Profiles":
			a.showConnectionProfilesDialog()
		case "Manage Plugins":
			a.showManagePluginsDialog()
		case "Create VM":
			a.showVMCreateForm(a.nodeList.GetSelectedNode())
		case "Create LXC":
			a.showLXCCreateForm(a.nodeList.GetSelectedNode())
		case "Ansible Toolkit":
			plugin, ok := a.plugins["ansible"]
			if !ok || plugin == nil {
				a.showMessageSafe("Ansible plugin is not enabled.")
				return
			}
			globalAction, ok := plugin.(GlobalActionPlugin)
			if !ok {
				a.showMessageSafe("Ansible plugin does not support global actions.")
				return
			}
			if err := globalAction.OpenGlobal(a.ctx, a); err != nil {
				a.showMessageSafe("Ansible Toolkit failed: " + err.Error())
			}
		case "Refresh All Data":
			// * Check if there are any pending operations
			if models.GlobalState.HasPendingOperations() {
				a.showMessageSafe("Cannot refresh data while there are pending operations in progress")
				return
			}
			a.manualRefresh()
		case "Toggle Auto-Refresh":
			a.toggleAutoRefresh()
		case "Help":
			if a.pages.HasPage("help") {
				a.helpModal.Hide()
			} else {
				a.helpModal.Show()
			}
		case "About":
			a.showAboutDialog()
		case "Quit":
			a.showQuitConfirmation()
		}
	})
	menu.SetApp(a)

	menuList := menu.Show()

	// Add input capture to close menu on Escape or 'h'
	oldCapture := menuList.GetInputCapture()
	menuList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'h') {
			a.CloseContextMenu()
			return nil
		}

		if oldCapture != nil {
			return oldCapture(event)
		}

		return event
	})

	a.showContextMenuPage(menuList, menuItems, 30, false, nil)
}

// showAboutDialog displays information about the application.
func (a *App) showAboutDialog() {
	// Get version information
	versionInfo := version.GetBuildInfo()

	// Create about dialog using the reusable function
	modal := CreateAboutDialog(versionInfo, func() {
		a.pages.RemovePage("about")
	})

	a.pages.AddPage("about", modal, false, true)
}
