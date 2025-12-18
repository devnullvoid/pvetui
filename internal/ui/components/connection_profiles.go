package components

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const addNewProfileText = "Add New Profile"

const addNewGroupText = "Add New Group"

// Selection type constants for profile picker

const (
	selectionTypeHeader = "header"

	selectionTypeSeparator = "separator"

	selectionTypeProfile = "profile"

	selectionTypeGroup = "group"

	selectionTypeAction = "action"
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

	selectionMap := make([]string, 0) // Maps menu index to profile/group name

	selectionTypes := make([]string, 0) // Maps menu index to "profile" or "group"

	// Add groups first (if any)

	if len(groups) > 0 {

		// Add section header

		menuItems = append(menuItems, "[::u]Groups[::-]")

		selectionMap = append(selectionMap, "") // Placeholder for header

		selectionTypes = append(selectionTypes, selectionTypeHeader)

		// Add each group (sorted)
		groupNames := make([]string, 0, len(groups))
		for groupName := range groups {
			groupNames = append(groupNames, groupName)
		}
		sort.Strings(groupNames)

		for _, groupName := range groupNames {
			profileNames := groups[groupName]

			displayName := fmt.Sprintf("▸ %s (%d profiles)", groupName, len(profileNames))

			// Check if currently connected to this group

			if a.isGroupMode && a.groupName == groupName {

				displayName = "⚡ " + displayName

			}

			if groupName == a.config.DefaultProfile {
				displayName = displayName + " ⭐"
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

	// Add separator and "Add New Profile" / "Add New Group" options

	menuItems = append(menuItems, "")

	selectionMap = append(selectionMap, "")

	selectionTypes = append(selectionTypes, selectionTypeSeparator)

	menuItems = append(menuItems, addNewProfileText)

	selectionMap = append(selectionMap, addNewProfileText)

	selectionTypes = append(selectionTypes, selectionTypeAction)

	menuItems = append(menuItems, addNewGroupText)

	selectionMap = append(selectionMap, addNewGroupText)

	selectionTypes = append(selectionTypes, selectionTypeAction)

	// Generate shortcuts

	shortcuts := make([]rune, len(menuItems))

	for i := range menuItems {

		if selectionMap[i] == addNewProfileText {

			shortcuts[i] = 'a' // 'a' for Add Profile

		} else if selectionMap[i] == addNewGroupText {

			shortcuts[i] = 'g' // 'g' for Add Group

		} else {

			shortcuts[i] = 0

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

		} else if selectionValue == addNewGroupText {

			a.showAddGroupDialog()

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

			// Handle navigation keys first

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

				// Edit - works for profiles AND groups

				if selectionType == selectionTypeProfile {

					a.CloseConnectionProfilesMenu()

					a.showEditProfileDialog(selectionValue)

					return nil

				} else if selectionType == selectionTypeGroup {

					a.CloseConnectionProfilesMenu()

					a.showEditGroupDialog(selectionValue)

					return nil

				}

			case 'd', 'D':

				// Delete - works for profiles and groups

				if selectionType == selectionTypeProfile {

					// Don't allow deletion if it's the last profile

					if len(profiles) <= 1 {

						a.showMessageSafe("Cannot delete the only profile. At least one profile must remain.")

						return nil

					}

					a.CloseConnectionProfilesMenu()

					a.showDeleteProfileDialog(selectionValue)

					return nil

				} else if selectionType == selectionTypeGroup {

					a.CloseConnectionProfilesMenu()

					a.showDeleteGroupDialog(selectionValue)

					return nil

				}

			case 's', 'S':

				// Set as default - works for profiles and groups
				if selectionType == selectionTypeProfile || selectionType == selectionTypeGroup {

					a.CloseConnectionProfilesMenu()

					a.setDefaultProfile(selectionValue)

					return nil

				}

			case 'a', 'A':

				a.CloseConnectionProfilesMenu()

				a.showAddProfileDialog()

				return nil

			case 'g', 'G':

				a.CloseConnectionProfilesMenu()

				a.showAddGroupDialog()

				return nil

			case 'j':

				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)

			case 'k':

				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)

			}

		}

		// If not handled by special actions, let the underlying List handle it

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

	layout.AddItem(menuList, 0, 1, true) // Flexible height for list

	layout.AddItem(helpText, 1, 0, false) // Help text at bottom

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
			AddItem(frame, 0, 1, true),

			60, 0, true).
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

// showDeleteGroupDialog displays a confirmation dialog for deleting a group.

func (a *App) showDeleteGroupDialog(groupName string) {

	// Store last focused primitive

	a.lastFocus = a.GetFocus()

	// Create confirmation dialog

	message := fmt.Sprintf("Are you sure you want to delete group '%s'?\n\nThis will remove the group tag from all profiles.\nThe profiles themselves will NOT be deleted.", groupName)

	onConfirm := func() {

		// Remove the modal first

		a.pages.RemovePage("deleteGroup")

		oldDefault := a.config.DefaultProfile

		// Remove group from all profiles

		hasChanges := false

		if a.config.Profiles != nil {

			for name, profile := range a.config.Profiles {

				newGroups := make([]string, 0)

				changed := false

				for _, g := range profile.Groups {

					if g == groupName {

						changed = true

					} else {

						newGroups = append(newGroups, g)

					}

				}

				if changed {

					profile.Groups = newGroups

					a.config.Profiles[name] = profile

					hasChanges = true

				}

			}

		}

		// If the deleted group was the default startup selection, fall back to a
		// remaining profile so startup remains valid.
		if oldDefault == groupName {
			remaining := make([]string, 0, len(a.config.Profiles))
			for name := range a.config.Profiles {
				remaining = append(remaining, name)
			}
			sort.Strings(remaining)
			if len(remaining) > 0 {
				a.config.DefaultProfile = remaining[0]
			} else {
				a.config.DefaultProfile = ""
			}
			hasChanges = true
		}

		if hasChanges {

			// Save the config

			configPath := a.configPath

			// Check if the original config was SOPS encrypted BEFORE saving

			wasSOPS := false

			if data, err := os.ReadFile(configPath); err == nil {

				wasSOPS = config.IsSOPSEncrypted(configPath, data)

			}

			if err := SaveConfigToFile(&a.config, configPath); err != nil {

				a.header.ShowError("Failed to save config after group deletion: " + err.Error())

				return

			}

			// Re-encrypt if needed

			if wasSOPS {

				if err := a.reEncryptConfigIfNeeded(configPath); err != nil {

					models.GetUILogger().Error("Failed to re-encrypt config: %v", err)

				}

			}

			a.header.ShowSuccess(fmt.Sprintf("Group '%s' deleted successfully!", groupName))

		}

		// Re-open profile selector

		a.showConnectionProfilesDialog()

	}

	onCancel := func() {

		// Remove the modal

		a.pages.RemovePage("deleteGroup")

		// Re-open profile selector

		a.showConnectionProfilesDialog()

	}

	confirm := CreateConfirmDialog("Delete Group", message, onConfirm, onCancel)

	a.pages.AddPage("deleteGroup", confirm, false, true)

	a.SetFocus(confirm)

}

// showAddProfileDialog displays a dialog for adding a new connection profile.

func (a *App) showAddProfileDialog() {

	// Store last focused primitive

	a.lastFocus = a.GetFocus()

	// Create a new profile config

	newProfile := config.ProfileConfig{

		Addr: "",

		User: "",

		Password: "",

		TokenID: "",

		TokenSecret: "",

		Realm: "pam",

		ApiPath: "/api2/json",

		Insecure: false,

		SSHUser: "",

		VMSSHUser: "",
	}

	// Create a temporary config for the wizard

	tempConfig := &config.Config{

		Profiles: make(map[string]config.ProfileConfig),

		DefaultProfile: "new_profile", // This will be replaced with the actual name

		// Use the legacy fields for the wizard

		Addr: newProfile.Addr,

		User: newProfile.User,

		Password: newProfile.Password,

		TokenID: newProfile.TokenID,

		TokenSecret: newProfile.TokenSecret,

		Realm: newProfile.Realm,

		ApiPath: newProfile.ApiPath,

		Insecure: newProfile.Insecure,

		SSHUser: newProfile.SSHUser,

		VMSSHUser: newProfile.VMSSHUser,

		Debug: a.config.Debug,

		CacheDir: a.config.CacheDir,

		Theme: a.config.Theme,
	}

	// Pre-populate the new profile in the temp config so wizard callbacks work

	tempConfig.Profiles["new_profile"] = newProfile

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

		a.Application.QueueUpdateDraw(func() {

			a.pages.RemovePage("profileWizard")

			if result.Saved {

				a.header.ShowSuccess("Profile '" + result.ProfileName + "' saved successfully!")

			}

			// Always return to profile manager to maintain context

			a.showConnectionProfilesDialog()

		})

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

		Profiles: make(map[string]config.ProfileConfig),

		DefaultProfile: profileName,

		Debug: a.config.Debug,

		CacheDir: a.config.CacheDir,

		Theme: a.config.Theme,
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

		a.Application.QueueUpdateDraw(func() {

			a.pages.RemovePage("profileWizard")

			if result.Saved {

				a.header.ShowSuccess("Profile '" + result.ProfileName + "' saved successfully!")

			}

			// Always return to profile manager to maintain context

			a.showConnectionProfilesDialog()

		})

	}()

}

// showAddGroupDialog prompts for a new group name and then opens the editor.

func (a *App) showAddGroupDialog() {

	// Store last focused primitive if we haven't already (e.g. if coming from menu directly)

	if a.lastFocus == nil {

		a.lastFocus = a.GetFocus()

	}

	form := tview.NewForm()

	form.SetBorder(true)

	form.SetTitle(" Create New Group ")

	form.SetBorderColor(theme.Colors.Border)

	form.SetLabelColor(theme.Colors.HeaderText)

	// Group Name Input

	nameInput := tview.NewInputField().
		SetLabel("Group Name").
		SetFieldWidth(30)

	form.AddFormItem(nameInput)

	// Create Button
	form.AddButton("Create", func() {
		name := strings.TrimSpace(nameInput.GetText())
		if name == "" {
			a.showMessageSafe("Group name cannot be empty")
			return
		}
		go func() {
			a.Application.QueueUpdateDraw(func() {
				a.pages.RemovePage("addGroupInput")
				a.showEditGroupDialog(name)
			})
		}()
	})

	// Cancel Button

	form.AddButton("Cancel", func() {

		a.pages.RemovePage("addGroupInput")

		a.showConnectionProfilesDialog()

	})

	// Handle Escape key to cancel

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

		if event.Key() == tcell.KeyEscape {

			a.pages.RemovePage("addGroupInput")

			a.showConnectionProfilesDialog()

			return nil

		}

		return event

	})

	// Handle Enter in input field to trigger Create
	nameInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			name := strings.TrimSpace(nameInput.GetText())
			if name != "" {
				go func() {
					a.Application.QueueUpdateDraw(func() {
						a.pages.RemovePage("addGroupInput")
						a.showEditGroupDialog(name)
					})
				}()
			} else {
				a.showMessageSafe("Group name cannot be empty")
			}
		}
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 13, 0, true).
			AddItem(nil, 0, 1, false), 60, 0, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("addGroupInput", modal, true, true)

	a.SetFocus(form)

}

// showEditGroupDialog displays a form to manage group members.

func (a *App) showEditGroupDialog(groupName string) {

	// Store last focused primitive

	if a.lastFocus == nil {

		a.lastFocus = a.GetFocus()

	}

	form := tview.NewForm()

	form.SetBorder(true)

	form.SetTitle(fmt.Sprintf(" Edit Group: %s ", groupName))

	form.SetTitleColor(theme.Colors.Title)

	form.SetBorderColor(theme.Colors.Border)

	form.SetLabelColor(theme.Colors.HeaderText)

	// Collect and sort profile names

	profileNames := make([]string, 0)

	for name := range a.config.Profiles {

		profileNames = append(profileNames, name)

	}

	sort.Strings(profileNames)

	// Map to track selections

	selections := make(map[string]bool)

	// Add checkboxes

	for _, name := range profileNames {

		profile := a.config.Profiles[name]

		inGroup := false

		for _, g := range profile.Groups {

			if g == groupName {

				inGroup = true

				break

			}

		}

		selections[name] = inGroup

		// Capture name variable for closure

		pName := name

		form.AddCheckbox(pName, inGroup, func(checked bool) {

			selections[pName] = checked

		})

	}

	// Save button

	form.AddButton("Save", func() {

		a.pages.RemovePage("editGroup")

		// Update profiles

		hasChanges := false

		for name, checked := range selections {

			profile := a.config.Profiles[name]

			currentlyInGroup := false

			for _, g := range profile.Groups {

				if g == groupName {

					currentlyInGroup = true

					break

				}

			}

			if checked && !currentlyInGroup {

				// Add to group

				profile.Groups = append(profile.Groups, groupName)

				a.config.Profiles[name] = profile

				hasChanges = true

			} else if !checked && currentlyInGroup {

				// Remove from group

				newGroups := make([]string, 0)

				for _, g := range profile.Groups {

					if g != groupName {

						newGroups = append(newGroups, g)

					}

				}

				profile.Groups = newGroups

				a.config.Profiles[name] = profile

				hasChanges = true

			}

		}

		if hasChanges {

			// Save config

			configPath := a.configPath // Use app's config path

			// Check if the original config was SOPS encrypted BEFORE saving

			wasSOPS := false

			if data, err := os.ReadFile(configPath); err == nil {

				wasSOPS = config.IsSOPSEncrypted(configPath, data)

			}

			// Try to handle SOPS if needed (similar to other save operations)

			if err := SaveConfigToFile(&a.config, configPath); err != nil {

				a.header.ShowError("Failed to save config: " + err.Error())

			} else {

				// Try re-encrypting if needed

				if wasSOPS {

					if err := a.reEncryptConfigIfNeeded(configPath); err != nil {

						// Log warning but don't fail user operation

						models.GetUILogger().Error("Failed to re-encrypt config: %v", err)

					}

				}

				a.header.ShowSuccess(fmt.Sprintf("Group '%s' updated successfully", groupName))

			}

		}

		// Re-open profile selector to show changes

		a.showConnectionProfilesDialog()

	})

	// Cancel button

	form.AddButton("Cancel", func() {

		a.pages.RemovePage("editGroup")

		// Re-open profile selector

		a.showConnectionProfilesDialog()

	})

	// Handle Escape key

	form.SetCancelFunc(func() {

		a.pages.RemovePage("editGroup")

		a.showConnectionProfilesDialog()

	})

	// Layout

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 2, 0, false).
			AddItem(form, 0, 1, true).
			AddItem(nil, 2, 0, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("editGroup", modal, true, true)

	a.SetFocus(form)

}
