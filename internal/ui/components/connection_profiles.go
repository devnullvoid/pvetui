package components

import (
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
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

	// Add the "Add New Profile" option
	menuItems := make([]string, len(profileNames))
	for i, name := range profileNames {
		if name == addNewProfileText {
			menuItems[i] = "➕ " + name
		} else {
			// Show star for default profile, and indicate if connected
			displayName := name
			isDefault := name == a.config.DefaultProfile
			isConnected := name == a.header.GetCurrentProfile()

			if isConnected {
				// Connected profile gets priority - show as connected
				displayName = "⚡ " + name
			}
			if isDefault {
				// Default profile (when not connected) shows as default
				displayName = displayName + " ⭐"
			}
			menuItems[i] = displayName
		}
	}

	// Add the "Add New Profile" option
	menuItems = append(menuItems, addNewProfileText)
	profileNames = append(profileNames, addNewProfileText)

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

	// Remove the menu's own border since we'll create our own
	menuList.SetBorder(false)

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
			case 'd', 'D':
				// Delete the currently selected profile
				index := menuList.GetCurrentItem()
				if index >= 0 && index < len(profileNames) {
					profileName := profileNames[index]
					if profileName != addNewProfileText {
						// Don't allow deletion if it's the only profile
						if len(profiles) <= 1 {
							a.showMessageSafe("Cannot delete the only profile. At least one profile must remain.")
							return nil
						}
						a.CloseConnectionProfilesMenu()
						a.showDeleteProfileDialog(profileName)
						return nil
					}
				}
			case 's', 'S':
				// Set the currently selected profile as default
				index := menuList.GetCurrentItem()
				if index >= 0 && index < len(profileNames) {
					profileName := profileNames[index]
					if profileName != addNewProfileText {
						a.CloseConnectionProfilesMenu()
						a.setDefaultProfile(profileName)
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

	// Create help text
	helpText := tview.NewTextView()
	helpText.SetText("e:edit d:delete s:default")
	helpText.SetTextAlign(tview.AlignCenter)
	helpText.SetDynamicColors(true)
	helpText.SetTextColor(theme.Colors.Secondary)

	// Create the layout with menu and help text
	layout := tview.NewFlex().SetDirection(tview.FlexRow)
	layout.AddItem(menuList, len(menuItems)+1, 1, true) // +1 for border
	layout.AddItem(helpText, 1, 0, false)               // Help text at bottom

	// Create a frame that acts as our bordered container
	frame := tview.NewFrame(layout)
	frame.SetBorder(true)
	frame.SetTitle(" Connection Profiles ")
	// frame.SetTitleColor(theme.Colors.Title)
	frame.SetBorderColor(theme.Colors.Border)

	// Use the same layout as the global context menu
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, len(menuItems)+6, 1, true). // +6 for border + help text + extra space for full text
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
		VMSSHUser:   "",
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
		VMSSHUser:   newProfile.VMSSHUser,
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
			// Show immediate feedback using header notification
			a.Application.QueueUpdateDraw(func() {
				a.header.ShowSuccess("Profile '" + result.ProfileName + "' saved successfully!")
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

	// Create a temporary config for the wizard with proper profile structure
	tempConfig := &config.Config{
		Profiles:       make(map[string]config.ProfileConfig),
		DefaultProfile: profileName,
		Debug:          a.config.Debug,
		CacheDir:       a.config.CacheDir,
		Theme:          a.config.Theme,
	}

	// Copy the profile data into the temporary config's profiles map
	tempConfig.Profiles[profileName] = profile

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
			// Show immediate feedback using header notification
			a.Application.QueueUpdateDraw(func() {
				a.header.ShowSuccess("Profile '" + result.ProfileName + "' saved successfully!")
			})
		} else if result.Canceled {
			// Restore focus to the last focused element when canceled
			if a.lastFocus != nil {
				a.SetFocus(a.lastFocus)
			}
		}
	}()
}
