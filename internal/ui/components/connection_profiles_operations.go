package components

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
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

		// Update app client and VNC service immediately to ensure subsequent calls use the new client
		// This must happen before manualRefresh() is called
		a.client = client
		if a.vncService != nil {
			a.vncService.UpdateClient(client)
		}

		// Clear group mode state
		if a.isGroupMode {
			uiLogger.Debug("Disabling group mode")
			if a.groupManager != nil {
				a.groupManager.Close()
			}
			a.groupManager = nil
			a.isGroupMode = false
			a.groupName = ""
			// Clear tasks list to remove group tasks
			a.tasksList.Clear()
		}

		a.QueueUpdateDraw(func() {
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

// switchToGroup switches to a group cluster view.
func (a *App) switchToGroup(groupName string) {
	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Connecting to group '%s'...", groupName))

	// Run group initialization in goroutine to avoid blocking UI
	go func() {
		uiLogger := models.GetUILogger()
		uiLogger.Debug("Starting group switch to: %s", groupName)

		// Get profile names for this group
		profileNames := a.config.GetProfileNamesInGroup(groupName)
		if len(profileNames) == 0 {
			uiLogger.Error("No profiles found for group %s", groupName)
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("No profiles found for group '%s'", groupName))
			})
			return
		}

		uiLogger.Debug("Found %d profiles in group %s: %v", len(profileNames), groupName, profileNames)

		// Create group manager
		manager := api.NewGroupClientManager(
			groupName,
			models.GetUILogger(),
			a.client.GetCache(), // Use existing cache
		)

		// Build profile entries
		var profiles []api.ProfileEntry
		for _, name := range profileNames {
			profile, exists := a.config.Profiles[name]
			if !exists {
				uiLogger.Debug("Profile %s not found in config, skipping", name)
				continue
			}

			// Create a config object from the profile for the adapter
			profileConfig := &config.Config{
				Addr:        profile.Addr,
				User:        profile.User,
				Password:    profile.Password,
				TokenID:     profile.TokenID,
				TokenSecret: profile.TokenSecret,
				Realm:       profile.Realm,
				ApiPath:     profile.ApiPath,
				Insecure:    profile.Insecure,
				SSHUser:     profile.SSHUser,
				VMSSHUser:   profile.VMSSHUser,
				CacheDir:    a.config.CacheDir,
				Debug:       a.config.Debug,
			}

			profiles = append(profiles, api.ProfileEntry{
				Name:   name,
				Config: adapters.NewConfigAdapter(profileConfig),
			})
		}

		if len(profiles) == 0 {
			uiLogger.Error("No valid profiles to initialize for group %s", groupName)
			a.QueueUpdateDraw(func() {
				a.header.ShowError("No valid profiles found")
			})
			return
		}

		// Initialize group manager (concurrent connection to all profiles)
		ctx := context.Background()
		uiLogger.Debug("Initializing group manager with %d profiles", len(profiles))

		if err := manager.Initialize(ctx, profiles); err != nil {
			// All profiles failed to connect
			uiLogger.Error("Failed to initialize group %s: %v", groupName, err)
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Failed to connect to any profiles: %v", err))
			})
			return
		}

		// Get connection summary
		summary := manager.GetConnectionSummary()
		uiLogger.Debug("Group initialized: %d/%d profiles connected", summary.ConnectedCount, summary.TotalProfiles)

		// Update app state
		a.QueueUpdateDraw(func() {
			// Set group mode
			// Note: We keep a.client around even in group mode to avoid breaking callbacks
			// that were set up during initialization. In group mode, we use a.groupManager
			// for operations instead of a.client.
			a.groupManager = manager
			a.isGroupMode = true
			a.groupName = groupName

			// Update header
			a.updateHeaderWithActiveProfile()

			// Show warning if some profiles failed
			if summary.ErrorCount > 0 {
				a.header.ShowWarning(fmt.Sprintf("Connected to %d/%d profiles", summary.ConnectedCount, summary.TotalProfiles))
			} else {
				a.header.ShowSuccess(fmt.Sprintf("Connected to group '%s' (%d profiles)", groupName, summary.ConnectedCount))
			}
		})

		// Load group data
		uiLogger.Debug("Loading group cluster resources")
		nodes, vms, err := manager.GetGroupClusterResources(ctx)
		if err != nil {
			uiLogger.Error("Failed to load group resources: %v", err)
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Failed to load resources: %v", err))
			})
			return
		}

		uiLogger.Debug("Loaded %d nodes and %d VMs from group", len(nodes), len(vms))

		// Update UI with group data
		a.QueueUpdateDraw(func() {
			// Store in global state
			models.GlobalState.OriginalNodes = nodes
			models.GlobalState.OriginalVMs = vms
			models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
			models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
			copy(models.GlobalState.FilteredNodes, nodes)
			copy(models.GlobalState.FilteredVMs, vms)

			// Update lists
			a.nodeList.SetNodes(nodes)
			a.vmList.SetVMs(vms)

			// Update cluster status (create a summary cluster object)
			if len(nodes) > 0 {
				// We need to construct a synthetic cluster to calculate totals correctly
				// The App's createSyntheticCluster method handles this calculation
				// but we need to update the cluster status component with it
				syntheticCluster := a.createSyntheticGroup(nodes)
				a.clusterStatus.Update(syntheticCluster)

				// Start background enrichment for detailed node stats
				// This ensures nodes get Version, Kernel, LoadAvg etc. populated
				a.enrichGroupNodesSequentially(nodes, false, "", false, 0, "", false)
			}

			// Update selection and details
			if len(nodes) > 0 {
				a.nodeList.SetCurrentItem(0)
				if selected := a.nodeList.GetSelectedNode(); selected != nil {
					a.nodeDetails.Update(selected, nodes)
				}
			} else {
				a.nodeDetails.Clear()
			}

			if len(vms) > 0 {
				a.vmList.SetCurrentItem(0)
				if selected := a.vmList.GetSelectedVM(); selected != nil {
					a.vmDetails.Update(selected)
				}
			} else {
				a.vmDetails.Clear()
			}

			uiLogger.Debug("Group data loaded successfully")

			// Refresh tasks from all profiles
			// Clear existing tasks first to avoid showing stale single-profile data
			a.tasksList.Clear()
			uiLogger.Debug("Loading group tasks")
			a.loadTasksData()
		})
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
