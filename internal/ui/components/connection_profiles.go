package components

import (
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const addNewProfileText = "Add New Profile"

// showConnectionProfilesDialog displays a dialog for managing connection profiles.
func (a *App) showConnectionProfilesDialog() {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

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
	profileNames = append(profileNames, addNewProfileText)

	// Create menu items with proper display names
	menuItems := make([]string, len(profileNames))
	for i, name := range profileNames {
		if name == addNewProfileText {
			menuItems[i] = "➕ " + name
		} else {
			// Show if it's the default profile
			displayName := name
			if name == a.config.DefaultProfile {
				displayName = "⭐ " + name + " (Default)"
			}
			menuItems[i] = displayName
		}
	}

	// Create the context menu
	menu := NewContextMenu(" Connection Profiles ", menuItems, func(index int, action string) {
		a.CloseConnectionProfilesMenu()

		profileName := profileNames[index]
		if profileName == addNewProfileText {
			a.showAddProfileDialog()
		} else {
			// Default action is to switch to the profile
			a.applyConnectionProfile(profileName)
		}
	})
	menu.SetApp(a)

	menuList := menu.Show()

	// Add input capture for additional actions
	oldCapture := menuList.GetInputCapture()
	menuList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'h') {
			a.CloseConnectionProfilesMenu()
			return nil
		}

		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'e', 'E':
				// Edit the currently selected profile
				index := menuList.GetCurrentItem()
				if index >= 0 && index < len(profileNames) {
					profileName := profileNames[index]
					if profileName != addNewProfileText {
						a.CloseConnectionProfilesMenu()
						a.showEditProfileDialog(profileName)
						return nil
					}
				}
			case 'a', 'A':
				// Add new profile
				a.CloseConnectionProfilesMenu()
				a.showAddProfileDialog()
				return nil
			case 's', 'S':
				// Switch to the currently selected profile
				index := menuList.GetCurrentItem()
				if index >= 0 && index < len(profileNames) {
					profileName := profileNames[index]
					if profileName != addNewProfileText {
						a.CloseConnectionProfilesMenu()
						a.applyConnectionProfile(profileName)
						return nil
					}
				}
			}
		}

		if oldCapture != nil {
			return oldCapture(event)
		}

		return event
	})

	a.contextMenu = menuList
	a.isMenuOpen = true

	// Create a custom layout with help text at the bottom
	helpText := tview.NewTextView()
	helpText.SetText("e:edit a:add s:switch")
	helpText.SetTextAlign(tview.AlignCenter)
	helpText.SetDynamicColors(true)
	helpText.SetTextColor(theme.Colors.Secondary)

	// Create the layout with menu and help text
	layout := tview.NewFlex().SetDirection(tview.FlexRow)
	layout.AddItem(menuList, len(menuItems)+2, 1, true) // +2 for border
	layout.AddItem(helpText, 1, 0, false)               // Help text at bottom

	// Use the same layout as the global context menu
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(layout, len(menuItems)+3, 1, true). // +3 for border + help text
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// CloseConnectionProfilesMenu closes the connection profiles menu and restores the previous focus.
func (a *App) CloseConnectionProfilesMenu() {
	if a.isMenuOpen {
		a.pages.RemovePage("contextMenu")
		a.isMenuOpen = false
		a.contextMenu = nil

		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}
}

// showAddProfileDialog displays a dialog for adding a new connection profile.
func (a *App) showAddProfileDialog() {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create a new profile config
	newProfile := config.ProfileConfig{
		Addr:        "",
		User:        "",
		Password:    "",
		TokenID:     "",
		TokenSecret: "",
		Realm:       "pam",
		ApiPath:     "/api2/json",
		Insecure:    false,
		SSHUser:     "",
	}

	// Create a temporary config for the wizard
	tempConfig := &config.Config{
		Profiles:       a.config.Profiles,
		DefaultProfile: "new_profile", // This will be replaced with the actual name
		// Use the legacy fields for the wizard
		Addr:        newProfile.Addr,
		User:        newProfile.User,
		Password:    newProfile.Password,
		TokenID:     newProfile.TokenID,
		TokenSecret: newProfile.TokenSecret,
		Realm:       newProfile.Realm,
		ApiPath:     newProfile.ApiPath,
		Insecure:    newProfile.Insecure,
		SSHUser:     newProfile.SSHUser,
		Debug:       a.config.Debug,
		CacheDir:    a.config.CacheDir,
		Theme:       a.config.Theme,
	}

	// Create a channel for the wizard result
	resultChan := make(chan WizardResult, 1)

	// Create the wizard page using our embedded version
	wizardPage := a.createEmbeddedConfigWizard(tempConfig, resultChan)

	// Add the wizard page
	a.pages.AddPage("profileWizard", wizardPage, true, true)
	a.SetFocus(wizardPage)

	// Handle the result
	go func() {
		result := <-resultChan
		a.pages.RemovePage("profileWizard")

		if result.Saved {
			// Show immediate feedback using QueueUpdateDraw
			a.Application.QueueUpdateDraw(func() {
				a.showMessage("Profile '" + result.ProfileName + "' saved successfully!")
			})
		} else if result.Canceled {
			// Restore focus to the last focused element when canceled
			if a.lastFocus != nil {
				a.SetFocus(a.lastFocus)
			}
		}
	}()
}

// showEditProfileDialog displays a dialog for editing an existing connection profile.
func (a *App) showEditProfileDialog(profileName string) {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Get the existing profile
	profiles := a.config.Profiles
	if profiles == nil {
		profiles = make(map[string]config.ProfileConfig)
	}

	profile, exists := profiles[profileName]
	if !exists {
		a.showMessage("Profile '" + profileName + "' not found!")
		return
	}

	// Create a temporary config for the wizard
	tempConfig := &config.Config{
		Profiles:       a.config.Profiles,
		DefaultProfile: profileName,
		// Use the legacy fields for the wizard
		Addr:        profile.Addr,
		User:        profile.User,
		Password:    profile.Password,
		TokenID:     profile.TokenID,
		TokenSecret: profile.TokenSecret,
		Realm:       profile.Realm,
		ApiPath:     profile.ApiPath,
		Insecure:    profile.Insecure,
		SSHUser:     profile.SSHUser,
		Debug:       a.config.Debug,
		CacheDir:    a.config.CacheDir,
		Theme:       a.config.Theme,
	}

	// Create a channel for the wizard result
	resultChan := make(chan WizardResult, 1)

	// Create the wizard page using our embedded version
	wizardPage := a.createEmbeddedConfigWizard(tempConfig, resultChan)

	// Add the wizard page
	a.pages.AddPage("profileWizard", wizardPage, true, true)
	a.SetFocus(wizardPage)

	// Handle the result
	go func() {
		result := <-resultChan
		a.pages.RemovePage("profileWizard")

		if result.Saved {
			// Show immediate feedback using QueueUpdateDraw
			a.Application.QueueUpdateDraw(func() {
				a.showMessage("Profile '" + result.ProfileName + "' saved successfully!")
			})
		} else if result.Canceled {
			// Restore focus to the last focused element when canceled
			if a.lastFocus != nil {
				a.SetFocus(a.lastFocus)
			}
		}
	}()
}

// applyConnectionProfile applies the selected connection profile.
func (a *App) applyConnectionProfile(profileName string) {
	err := a.config.ApplyProfile(profileName)
	if err != nil {
		a.showMessage("Failed to apply profile: " + err.Error())

		return
	}

	// Note: We don't save the config file when switching profiles in the UI
	// The default_profile should only be changed via the config wizard
	// This allows temporary profile switching without affecting the saved config

	// Recreate the API client with the new profile
	client, err := api.NewClient(&a.config, api.WithLogger(models.GetUILogger()))
	if err != nil {
		a.showMessage("Failed to create API client: " + err.Error())

		return
	}

	// Update the app's client
	a.client = client

	// Update the VNC service's client to use the new profile
	a.vncService.UpdateClient(client)

	// Update the header to show the active profile
	a.updateHeaderWithActiveProfile()

	// Refresh the data
	a.manualRefresh()

	a.showMessage("Profile '" + profileName + "' applied successfully!")
}
