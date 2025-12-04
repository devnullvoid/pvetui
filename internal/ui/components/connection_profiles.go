package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const addNewProfileText = "Add New Profile"

// Selection type constants for profile picker
const (
	selectionTypeHeader    = "header"
	selectionTypeSeparator = "separator"
	selectionTypeProfile   = "profile"
	selectionTypeGroup     = "group"
	selectionTypeAction    = "action"
)

// showConnectionProfilesDialog displays a dialog for managing connection profiles and groups.
func (a *App) showConnectionProfilesDialog() {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Get current profiles
	profiles := a.config.Profiles
	if profiles == nil {
		profiles = make(map[string]config.ProfileConfig)
	}

	// Get groups
	groups := a.config.GetGroups()

	// Create lists for menu items and selection mapping
	menuItems := make([]string, 0)
	selectionMap := make([]string, 0)   // Maps menu index to profile/group name
	selectionTypes := make([]string, 0) // Maps menu index to "profile" or "group"

	// Add groups first (if any)
	if len(groups) > 0 {
		// Add section header
		menuItems = append(menuItems, "[::u]Groups[::-]")
		selectionMap = append(selectionMap, "") // Placeholder for header
		selectionTypes = append(selectionTypes, selectionTypeHeader)

		// Add each group
		for groupName, profileNames := range groups {
			displayName := fmt.Sprintf("▸ %s (%d profiles)", groupName, len(profileNames))

			// Check if currently connected to this group
			if a.isGroupMode && a.groupName == groupName {
				displayName = "⚡ " + displayName
			}

			menuItems = append(menuItems, displayName)
			selectionMap = append(selectionMap, groupName)
			selectionTypes = append(selectionTypes, selectionTypeGroup)
		}

		// Add separator
		menuItems = append(menuItems, "")
		selectionMap = append(selectionMap, "")
		selectionTypes = append(selectionTypes, selectionTypeSeparator)
	}

	// Add individual profiles section
	menuItems = append(menuItems, "[::u]Individual Profiles[::-]")
	selectionMap = append(selectionMap, "")
	selectionTypes = append(selectionTypes, selectionTypeHeader)

	// Collect all profile names and sort them alphabetically
	allProfileNames := make([]string, 0, len(profiles))
	for name := range profiles {
		allProfileNames = append(allProfileNames, name)
	}
	sort.Strings(allProfileNames)

	// Add sorted profiles
	for _, name := range allProfileNames {
		profile := profiles[name]
		displayName := name

		isDefault := name == a.config.DefaultProfile
		isConnected := name == a.header.GetCurrentProfile() && !a.isGroupMode

		if isConnected {
			displayName = "⚡ " + displayName
		}
		if isDefault {
			displayName = displayName + " ⭐"
		}

		// Append groups if present
		if len(profile.Groups) > 0 {
			groupStr := strings.Join(profile.Groups, ", ")
			displayName += fmt.Sprintf(" [secondary](%s)[-]", groupStr)
		}

		menuItems = append(menuItems, displayName)
		selectionMap = append(selectionMap, name)
		selectionTypes = append(selectionTypes, selectionTypeProfile)
	}

	// Add separator and "Add New Profile" option
	menuItems = append(menuItems, "")
	selectionMap = append(selectionMap, "")
	selectionTypes = append(selectionTypes, selectionTypeSeparator)

	menuItems = append(menuItems, addNewProfileText)
	selectionMap = append(selectionMap, addNewProfileText)
	selectionTypes = append(selectionTypes, selectionTypeAction)

	// Generate shortcuts: 0 for everything except specific actions
	shortcuts := make([]rune, len(menuItems))
	for i := range menuItems {
		if selectionMap[i] == addNewProfileText {
			shortcuts[i] = 'a' // 'a' for Add
		} else {
			shortcuts[i] = 0 // No shortcut for others
		}
	}

	// Create the context menu
	menu := NewContextMenuWithShortcuts(" Connection Profiles ", menuItems, shortcuts, func(index int, action string) {
		if index < 0 || index >= len(selectionMap) {
			return
		}

		selectionType := selectionTypes[index]
		selectionValue := selectionMap[index]

		// Skip headers and separators
		if selectionType == selectionTypeHeader || selectionType == selectionTypeSeparator {
			return
		}

		a.CloseConnectionProfilesMenu()

		if selectionValue == addNewProfileText {
			a.showAddProfileDialog()
		} else if selectionType == selectionTypeGroup {
			// Switch to group
			a.switchToGroup(selectionValue)
		} else if selectionType == selectionTypeProfile {
			// Switch to individual profile
			a.applyConnectionProfile(selectionValue)
		}
	})
	menu.SetApp(a)

	menuList := menu.Show()

	// Remove the menu's own border since we'll create our own
	menuList.SetBorder(false)

	// Add input capture for additional actions
	menuList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'h') {
			a.CloseConnectionProfilesMenu()
			return nil
		}

		if event.Key() == tcell.KeyRune {
			// Handle navigation keys first, regardless of selection
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}

			index := menuList.GetCurrentItem()
			if index < 0 || index >= len(selectionMap) {
				return event
			}

			selectionType := selectionTypes[index]
			selectionValue := selectionMap[index]

			// Skip actions on headers, separators, and special items
			if selectionType == selectionTypeHeader || selectionType == selectionTypeSeparator || selectionValue == "" {
				return event
			}

			switch event.Rune() {
			case 'e', 'E':
				// Edit - only works for profiles
				if selectionType == selectionTypeProfile {
					a.CloseConnectionProfilesMenu()
					a.showEditProfileDialog(selectionValue)
					return nil
				}
			case 'd', 'D':
				// Delete - only works for profiles
				if selectionType == selectionTypeProfile {
					// Don't allow deletion if it's the last profile
					if len(profiles) <= 1 {
						a.showMessageSafe("Cannot delete the only profile. At least one profile must remain.")
						return nil
					}
					a.CloseConnectionProfilesMenu()
					a.showDeleteProfileDialog(selectionValue)
					return nil
				}
			case 's', 'S':
				// Set as default - only works for standalone profiles
				if selectionType == selectionTypeProfile {
					a.CloseConnectionProfilesMenu()
					a.setDefaultProfile(selectionValue)
					return nil
				}
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}
		}

		// If not handled by special actions, let the underlying List handle it (e.g., arrow keys)
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
			AddItem(nil, 0, 1, false), 60, 1, true).
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
