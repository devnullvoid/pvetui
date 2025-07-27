package components

import (
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowGlobalContextMenu displays the global context menu for app-wide actions
func (a *App) ShowGlobalContextMenu() {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items for global actions
	menuItems := []string{
		"Connection Profiles",
		"Refresh All Data",
		"Toggle Auto-Refresh",
		"Help",
		"About",
	}

	menu := NewContextMenu(" Global Actions ", menuItems, func(index int, action string) {
		a.CloseContextMenu()
		switch action {
		case "Connection Profiles":
			a.showConnectionProfilesDialog()
		case "Refresh All Data":
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

	a.contextMenu = menuList
	a.isMenuOpen = true

	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// showConnectionProfilesDialog displays a dialog for managing connection profiles
func (a *App) showConnectionProfilesDialog() {
	// Get current profiles
	profiles := a.config.Profiles
	if profiles == nil {
		profiles = make(map[string]config.ProfileConfig)
	}

	// Create profile list
	profileNames := make([]string, 0, len(profiles))
	for name := range profiles {
		profileNames = append(profileNames, name)
	}

	// Add "Add New Profile" option
	profileNames = append(profileNames, "Add New Profile")

	// Create dialog
	modal := tview.NewModal()
	modal.SetText("Select a connection profile:")
	modal.SetBackgroundColor(theme.Colors.Background)
	modal.SetTextColor(theme.Colors.Primary)
	modal.SetBorderColor(theme.Colors.Border)
	modal.SetTitle("Connection Profiles")
	modal.SetTitleColor(theme.Colors.Title)

	// Add buttons for each profile
	buttons := make([]string, len(profileNames))
	for i, name := range profileNames {
		if name == a.config.DefaultProfile {
			buttons[i] = name + " (Default)"
		} else {
			buttons[i] = name
		}
	}
	modal.AddButtons(buttons)

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		a.pages.RemovePage("connectionProfiles")
		if buttonIndex >= 0 && buttonIndex < len(profileNames) {
			profileName := profileNames[buttonIndex]
			if profileName == "Add New Profile" {
				a.showAddProfileDialog()
			} else {
				a.applyConnectionProfile(profileName)
			}
		}
	})

	a.pages.AddPage("connectionProfiles", modal, false, true)
}

// showAddProfileDialog displays a dialog for adding a new connection profile
func (a *App) showAddProfileDialog() {
	// This would be a more complex form dialog
	// For now, just show a message
	a.showMessage("Add Profile functionality coming soon!")
}

// applyConnectionProfile applies the selected connection profile
func (a *App) applyConnectionProfile(profileName string) {
	err := a.config.ApplyProfile(profileName)
	if err != nil {
		a.showMessage("Failed to apply profile: " + err.Error())
		return
	}

	// Recreate the API client with the new profile
	client, err := api.NewClient(&a.config, api.WithLogger(models.GetUILogger()))
	if err != nil {
		a.showMessage("Failed to create API client: " + err.Error())
		return
	}

	// Update the app's client
	a.client = client

	// Refresh the data
	a.manualRefresh()

	a.showMessage("Profile '" + profileName + "' applied successfully!")
}

// showAboutDialog displays information about the application
func (a *App) showAboutDialog() {
	aboutText := `Proxmox TUI

A terminal user interface for Proxmox VE

Features:
• Node and VM management
• VNC console access
• SSH shell access
• Community scripts
• Connection profiles

Version: 1.24.5`

	modal := tview.NewModal()
	modal.SetText(aboutText)
	modal.SetBackgroundColor(theme.Colors.Background)
	modal.SetTextColor(theme.Colors.Primary)
	modal.SetBorderColor(theme.Colors.Border)
	modal.SetTitle("About")
	modal.SetTitleColor(theme.Colors.Title)
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		a.pages.RemovePage("about")
	})

	a.pages.AddPage("about", modal, false, true)
}
