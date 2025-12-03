package components

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// createEmbeddedConfigWizard creates a config wizard that doesn't call app.Stop() when canceling
func (a *App) createEmbeddedConfigWizard(cfg *config.Config, resultChan chan<- WizardResult) tview.Primitive {
	form := tview.NewForm().SetHorizontal(false)
	pages := tview.NewPages()
	pages.AddPage("form", form, true, true)

	// Use the app's actual config path instead of hardcoding the default path
	configPath := a.configPath
	if configPath == "" {
		// Fallback to default path if no path is stored
		var found bool
		configPath, found = config.FindDefaultConfigPath()
		if !found {
			configPath = config.GetDefaultConfigPath()
		}
	}
	wasSOPS := false
	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			wasSOPS = isSOPSEncrypted(configPath, data)
		}
	}
	// Check for .sops.yaml in config dir or parents
	sopsRuleExists := false
	if configPath != "" {
		sopsRuleExists = findSOPSRule(filepath.Dir(configPath))
	}

	// Determine if this is a new profile or editing existing
	isNewProfile := cfg.DefaultProfile == "new_profile"

	// For new profiles, create the profile entry in the map
	if isNewProfile {
		if cfg.Profiles == nil {
			cfg.Profiles = make(map[string]config.ProfileConfig)
		}
		// Create a new profile entry with default values
		cfg.Profiles["new_profile"] = config.ProfileConfig{
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
	}

	// Add profile name field at the top
	var profileName string

	if isNewProfile {
		form.AddInputField("Profile Name", "", 20, nil, func(text string) {
			profileName = strings.TrimSpace(text)
		})
	} else {
		// For editing, allow renaming the profile
		profileName = cfg.DefaultProfile // Start with current name
		form.AddInputField("Profile Name", cfg.DefaultProfile, 20, nil, func(text string) {
			profileName = strings.TrimSpace(text)
		})
	}

	// Determine which data to use for form fields
	var addr, user, password, tokenID, tokenSecret, realm, apiPath, sshUser, vmSSHUser, aggregate string
	var insecure bool

	// If we have profiles and a default profile, use profile data
	//nolint:dupl // Shared with config wizard but kept inline for clarity
	if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
		if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
			addr = profile.Addr
			user = profile.User
			// Decrypt password and tokenSecret for display in the form
			password = profile.Password
			if password != "" {
				if decrypted, err := config.DecryptField(password); err == nil {
					password = decrypted
				}
				// If decryption failed, keep the encrypted value (user can see it's encrypted)
			}
			tokenID = profile.TokenID
			tokenSecret = profile.TokenSecret
			if tokenSecret != "" {
				if decrypted, err := config.DecryptField(tokenSecret); err == nil {
					tokenSecret = decrypted
				}
				// If decryption failed, keep the encrypted value (user can see it's encrypted)
			}
			realm = profile.Realm
			apiPath = profile.ApiPath
			insecure = profile.Insecure
			sshUser = profile.SSHUser
			vmSSHUser = profile.VMSSHUser
			aggregate = profile.Aggregate
		}
	} else {
		// Use legacy fields
		addr = cfg.Addr
		user = cfg.User
		// Decrypt password and tokenSecret for display in the form
		password = cfg.Password
		if password != "" {
			if decrypted, err := config.DecryptField(password); err == nil {
				password = decrypted
			}
		}
		tokenID = cfg.TokenID
		tokenSecret = cfg.TokenSecret
		if tokenSecret != "" {
			if decrypted, err := config.DecryptField(tokenSecret); err == nil {
				tokenSecret = decrypted
			}
		}
		realm = cfg.Realm
		apiPath = cfg.ApiPath
		insecure = cfg.Insecure
		sshUser = cfg.SSHUser
		vmSSHUser = cfg.VMSSHUser
		// Legacy config doesn't have aggregate field
	}

	form.AddInputField("Proxmox API URL", addr, 40, nil, func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			// Update profile
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.Addr = strings.TrimSpace(text)
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			// Update legacy field
			cfg.Addr = strings.TrimSpace(text)
		}
	})
	form.AddInputField("Username", user, 20, nil, func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.User = strings.TrimSpace(text)
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.User = strings.TrimSpace(text)
		}
	})
	form.AddPasswordField("Password", password, 20, '*', func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.Password = text
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.Password = text
		}
	})
	form.AddInputField("API Token ID", tokenID, 20, nil, func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.TokenID = strings.TrimSpace(text)
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.TokenID = strings.TrimSpace(text)
		}
	})
	form.AddPasswordField("API Token Secret", tokenSecret, 20, '*', func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.TokenSecret = text
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.TokenSecret = text
		}
	})
	form.AddInputField("Realm", realm, 10, nil, func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.Realm = strings.TrimSpace(text)
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.Realm = strings.TrimSpace(text)
		}
	})
	form.AddInputField("API Path", apiPath, 20, nil, func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.ApiPath = strings.TrimSpace(text)
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.ApiPath = strings.TrimSpace(text)
		}
	})
	form.AddCheckbox("Skip TLS Verification", insecure, func(checked bool) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.Insecure = checked
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.Insecure = checked
		}
	})
	form.AddInputField("SSH Username", sshUser, 20, nil, func(text string) {
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.SSHUser = strings.TrimSpace(text)
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.SSHUser = strings.TrimSpace(text)
		}
	})
	form.AddInputField("VM SSH Username", vmSSHUser, 20, nil, func(text string) {
		value := strings.TrimSpace(text)
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.VMSSHUser = value
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			cfg.VMSSHUser = value
		}
	})
	form.AddInputField("Aggregate Group", aggregate, 20, nil, func(text string) {
		value := strings.TrimSpace(text)
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				profile.Aggregate = value
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		}
		// Legacy config doesn't support aggregate
	})

	form.AddButton("Save", func() {
		// Validate profile name for all profiles
		if profileName == "" {
			showWizardModal(pages, form, a.Application, "error", "Profile name cannot be empty.", nil)
			return
		}

		// Check if profile already exists (for new profiles or renamed profiles)
		if isNewProfile || profileName != cfg.DefaultProfile {
			if a.config.Profiles != nil && a.config.Profiles[profileName] != (config.ProfileConfig{}) {
				showWizardModal(pages, form, a.Application, "error", "Profile '"+profileName+"' already exists.", nil)
				return
			}
		}

		// Determine which data to validate based on whether we're using profiles
		var hasPassword, hasToken bool

		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			// Validate profile data
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				hasPassword = profile.Password != ""
				hasToken = profile.TokenID != "" && profile.TokenSecret != ""
			}
		} else {
			// Validate legacy data
			hasPassword = cfg.Password != ""
			hasToken = cfg.TokenID != "" && cfg.TokenSecret != ""
		}

		if hasPassword && hasToken {
			showWizardModal(pages, form, a.Application, "error", "Please choose either password authentication or token authentication, not both.", nil)
			return
		}

		if !hasPassword && !hasToken {
			showWizardModal(pages, form, a.Application, "error", "You must provide either a password or a token for authentication.", nil)
			return
		}

		// Clear conflicting auth method
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				if hasPassword {
					profile.TokenID = ""
					profile.TokenSecret = ""
				} else if hasToken {
					profile.Password = ""
				}
				cfg.Profiles[cfg.DefaultProfile] = profile
			}
		} else {
			if hasPassword {
				cfg.TokenID = ""
				cfg.TokenSecret = ""
			} else if hasToken {
				cfg.Password = ""
			}
		}

		if err := cfg.Validate(); err != nil {
			showWizardModal(pages, form, a.Application, "error", "Validation error: "+err.Error(), nil)
			return
		}

		// Update main config with the edited profile and save that
		if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
			// Ensure main config has profiles map
			if a.config.Profiles == nil {
				a.config.Profiles = make(map[string]config.ProfileConfig)
			}

			// Handle profile renaming or new profile creation
			if isNewProfile {
				// For new profiles, remove the temporary "new_profile" entry and add with the actual name
				delete(a.config.Profiles, "new_profile")
				a.config.Profiles[profileName] = profile
			} else if profileName != cfg.DefaultProfile {
				// For existing profiles being renamed
				oldProfileName := cfg.DefaultProfile
				delete(a.config.Profiles, oldProfileName)

				// Update default profile if we're renaming the current default
				if a.config.DefaultProfile == oldProfileName {
					a.config.DefaultProfile = profileName
				}

				// Add with new name
				a.config.Profiles[profileName] = profile
			} else {
				// For existing profiles with same name, just update
				a.config.Profiles[profileName] = profile
			}
		}

		// Save the main config (which has all profiles)
		if err := SaveConfigToFile(&a.config, configPath); err != nil {
			showWizardModal(pages, form, a.Application, "error", "Failed to save profile: "+err.Error(), nil)
			return
		}

		// If SOPS re-encryption is possible, prompt user
		if wasSOPS && sopsRuleExists {
			onYes := func() {
				cmd := exec.Command("sops", "-e", "-i", configPath)
				err := cmd.Run()
				if err != nil {
					showWizardModal(pages, form, a.Application, "error", "SOPS re-encryption failed: "+err.Error(), nil)
					return
				}

				// showWizardModal(pages, form, a.Application, "info", "Profile saved and re-encrypted with SOPS!", func() {
				if isNewProfile {
					resultChan <- WizardResult{Saved: true, SopsEncrypted: true, ProfileName: profileName}
				} else {
					resultChan <- WizardResult{Saved: true, SopsEncrypted: true, ProfileName: cfg.DefaultProfile}
				}
				// })
			}
			onNo := func() {
				// showWizardModal(pages, form, a.Application, "info", "Profile saved (unencrypted).", func() {
				if isNewProfile {
					resultChan <- WizardResult{Saved: true, ProfileName: profileName}
				} else {
					resultChan <- WizardResult{Saved: true, ProfileName: cfg.DefaultProfile}
				}
				// })
			}
			confirm := CreateConfirmDialog("SOPS Re-encryption", "The original config was SOPS-encrypted. Re-encrypt the new config with SOPS?", onYes, onNo)
			pages.AddPage("modal", confirm, false, true)
			pages.SwitchToPage("modal")
			return
		}

		// Send saved result
		if isNewProfile {
			resultChan <- WizardResult{Saved: true, ProfileName: profileName}
		} else {
			resultChan <- WizardResult{Saved: true, ProfileName: cfg.DefaultProfile}
		}
	})

	form.AddButton("Cancel", func() {
		resultChan <- WizardResult{Canceled: true}
	})

	form.SetBorder(true).SetTitle("pvetui - Profile Configuration").SetTitleColor(theme.Colors.Primary)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			resultChan <- WizardResult{Canceled: true}
			return nil
		}
		return event
	})

	return pages
}
