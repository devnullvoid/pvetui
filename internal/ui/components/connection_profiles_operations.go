package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

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

	a.client = client

	// Update VNC service with new connection details
	if a.vncService != nil {
		a.vncService.UpdateClient(client)
	}

	// Refresh data with new connection
	a.manualRefresh()

	// Show success message
	a.showMessage("Switched to profile '" + profileName + "' successfully!")
}

// showDeleteProfileDialog displays a confirmation dialog for deleting a profile.
func (a *App) showDeleteProfileDialog(profileName string) {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create confirmation dialog
	message := fmt.Sprintf("Are you sure you want to delete profile '%s'?\n\nThis action cannot be undone.", profileName)

	onConfirm := func() {
		// Delete the profile
		if a.config.Profiles != nil {
			delete(a.config.Profiles, profileName)

			// If this was the default profile, set the first remaining profile as default
			if a.config.DefaultProfile == profileName {
				for name := range a.config.Profiles {
					a.config.DefaultProfile = name
					break
				}
			}

			// Save the config
			configPath, found := config.FindDefaultConfigPath()
			if !found {
				configPath = config.GetDefaultConfigPath()
			}

			if err := SaveConfigToFile(&a.config, configPath); err != nil {
				a.Application.QueueUpdateDraw(func() {
					a.showMessage("Failed to save config after deletion: " + err.Error())
				})
				return
			}

			// Show success message with proper focus
			a.Application.QueueUpdateDraw(func() {
				a.showMessage("Profile '" + profileName + "' deleted successfully!")
			})
		}

		// Restore focus
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	onCancel := func() {
		// Restore focus
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	confirm := CreateConfirmDialog("Delete Profile", message, onConfirm, onCancel)
	a.pages.AddPage("deleteProfile", confirm, false, true)
	a.SetFocus(confirm)
}
