package components

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/devnullvoid/peevetui/internal/config"
	"github.com/devnullvoid/peevetui/internal/ui/models"
	"github.com/devnullvoid/peevetui/pkg/api"
)

// applyConnectionProfile applies the selected connection profile.
func (a *App) applyConnectionProfile(profileName string) {
	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Switching to profile '%s'...", profileName))

	// Run profile switching in goroutine to avoid blocking UI
	go func() {
		uiLogger := models.GetUILogger()
		uiLogger.Debug("Starting profile switch to: %s", profileName)

		err := a.config.ApplyProfile(profileName)
		if err != nil {
			uiLogger.Error("Failed to apply profile %s: %v", profileName, err)
			a.QueueUpdateDraw(func() {
				a.header.ShowError("Failed to apply profile: " + err.Error())
			})
			return
		}

		uiLogger.Debug("Profile %s applied successfully to config", profileName)

		// Note: We don't save the config file when switching profiles in the UI
		// The default_profile should only be changed via the config wizard
		// This allows temporary profile switching without affecting the saved config

		// Recreate the API client with the new profile
		uiLogger.Debug("Creating new API client with updated config")
		client, err := api.NewClient(&a.config, api.WithLogger(models.GetUILogger()))
		if err != nil {
			uiLogger.Error("Failed to create API client for profile %s: %v", profileName, err)
			a.QueueUpdateDraw(func() {
				a.header.ShowError("Failed to create API client: " + err.Error())
			})
			return
		}

		uiLogger.Debug("New API client created successfully for profile %s", profileName)

		a.QueueUpdateDraw(func() {
			uiLogger.Debug("Updating app client and VNC service")
			a.client = client

			// Update VNC service with new connection details
			if a.vncService != nil {
				uiLogger.Debug("Updating VNC service client")
				a.vncService.UpdateClient(client)
			}

			// Update the header to show the new active profile
			uiLogger.Debug("Updating header with new active profile: %s", profileName)
			a.header.ShowActiveProfile(profileName)
		})

		// Show success message
		a.QueueUpdateDraw(func() {
			a.header.ShowSuccess("Switched to profile '" + profileName + "' successfully!")
		})

		// Then refresh data with new connection (this will update the UI)
		uiLogger.Debug("Starting manual refresh with new client")
		a.manualRefresh()
	}()
}

// showDeleteProfileDialog displays a confirmation dialog for deleting a profile.
func (a *App) showDeleteProfileDialog(profileName string) {
	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create confirmation dialog
	message := fmt.Sprintf("Are you sure you want to delete profile '%s'?\n\nThis action cannot be undone.", profileName)

	onConfirm := func() {
		// Remove the modal first
		a.pages.RemovePage("deleteProfile")

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

			// Check if the original config was SOPS encrypted BEFORE saving
			wasSOPS := false
			if data, err := os.ReadFile(configPath); err == nil {
				wasSOPS = config.IsSOPSEncrypted(configPath, data)
			}

			if err := SaveConfigToFile(&a.config, configPath); err != nil {
				a.header.ShowError("Failed to save config after deletion: " + err.Error())
				return
			}

			// Re-encrypt if the original was SOPS encrypted
			if wasSOPS {
				if err := a.reEncryptConfigIfNeeded(configPath); err != nil {
					a.header.ShowError("Failed to re-encrypt config after deletion: " + err.Error())
					return
				}
			}

			// Show success message
			a.header.ShowSuccess("Profile '" + profileName + "' deleted successfully!")
		}

		// Restore focus
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	onCancel := func() {
		// Remove the modal
		a.pages.RemovePage("deleteProfile")

		// Restore focus
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	confirm := CreateConfirmDialog("Delete Profile", message, onConfirm, onCancel)
	a.pages.AddPage("deleteProfile", confirm, false, true)
	a.SetFocus(confirm)
}

// setDefaultProfile sets the specified profile as the default profile.
func (a *App) setDefaultProfile(profileName string) {
	// Check if the profile exists
	if a.config.Profiles == nil {
		a.header.ShowError("No profiles available.")
		return
	}

	if _, exists := a.config.Profiles[profileName]; !exists {
		a.header.ShowError(fmt.Sprintf("Profile '%s' not found.", profileName))
		return
	}

	// Check if it's already the default
	if a.config.DefaultProfile == profileName {
		a.header.ShowError(fmt.Sprintf("Profile '%s' is already the default profile.", profileName))
		return
	}

	// Store the old default profile name for the message
	oldDefault := a.config.DefaultProfile

	// Set the new default profile
	a.config.DefaultProfile = profileName

	// Save the config
	configPath, found := config.FindDefaultConfigPath()
	if !found {
		configPath = config.GetDefaultConfigPath()
	}

	// Check if the original config was SOPS encrypted BEFORE saving
	wasSOPS := false
	if data, err := os.ReadFile(configPath); err == nil {
		wasSOPS = config.IsSOPSEncrypted(configPath, data)
	}

	if err := SaveConfigToFile(&a.config, configPath); err != nil {
		a.header.ShowError(fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	// Re-encrypt if the original was SOPS encrypted
	if wasSOPS {
		if err := a.reEncryptConfigIfNeeded(configPath); err != nil {
			a.header.ShowError(fmt.Sprintf("Failed to re-encrypt config: %v", err))
			return
		}
	}

	// Show success message
	a.header.ShowSuccess(fmt.Sprintf("Default profile changed from '%s' to '%s'.", oldDefault, profileName))
}

// reEncryptConfigIfNeeded re-encrypts the config file with SOPS.
func (a *App) reEncryptConfigIfNeeded(configPath string) error {
	// Check if SOPS rule exists
	sopsRuleExists := config.FindSOPSRule(filepath.Dir(configPath))
	if !sopsRuleExists {
		return nil // No SOPS rule, can't re-encrypt
	}

	// Re-encrypt with SOPS
	cmd := exec.Command("sops", "-e", "-i", configPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SOPS re-encryption failed: %w", err)
	}

	return nil
}
