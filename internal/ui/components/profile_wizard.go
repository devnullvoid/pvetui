package components

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// createEmbeddedConfigWizard creates a config wizard that doesn't call app.Stop() when canceling
func (a *App) createEmbeddedConfigWizard(cfg *config.Config, resultChan chan<- WizardResult) tview.Primitive {
	form := tview.NewForm().SetHorizontal(false)
	pages := tview.NewPages()
	pages.AddPage("form", form, true, true)

	// Detect if original config was SOPS-encrypted
	configPath, found := config.FindDefaultConfigPath()
	if !found {
		configPath = config.GetDefaultConfigPath()
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

	// Add profile name field at the top (only for new profiles)
	var profileName string
	isNewProfile := cfg.DefaultProfile == "new_profile"

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

	form.AddInputField("Proxmox API URL", cfg.Addr, 40, nil, func(text string) { cfg.Addr = strings.TrimSpace(text) })
	form.AddInputField("Username", cfg.User, 20, nil, func(text string) { cfg.User = strings.TrimSpace(text) })
	form.AddPasswordField("Password", cfg.Password, 20, '*', func(text string) { cfg.Password = text })
	form.AddInputField("API Token ID", cfg.TokenID, 20, nil, func(text string) { cfg.TokenID = strings.TrimSpace(text) })
	form.AddPasswordField("API Token Secret", cfg.TokenSecret, 20, '*', func(text string) { cfg.TokenSecret = text })
	form.AddInputField("Realm", cfg.Realm, 10, nil, func(text string) { cfg.Realm = strings.TrimSpace(text) })
	form.AddInputField("API Path", cfg.ApiPath, 20, nil, func(text string) { cfg.ApiPath = strings.TrimSpace(text) })
	form.AddCheckbox("Skip TLS Verification", cfg.Insecure, func(checked bool) { cfg.Insecure = checked })
	form.AddInputField("SSH Username", cfg.SSHUser, 20, nil, func(text string) { cfg.SSHUser = strings.TrimSpace(text) })

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

		hasPassword := cfg.Password != ""
		hasToken := cfg.TokenID != "" && cfg.TokenSecret != ""

		if hasPassword && hasToken {
			showWizardModal(pages, form, a.Application, "error", "Please choose either password authentication or token authentication, not both.", nil)
			return
		}

		if !hasPassword && !hasToken {
			showWizardModal(pages, form, a.Application, "error", "You must provide either a password or a token for authentication.", nil)
			return
		}

		if hasPassword {
			cfg.TokenID = ""
			cfg.TokenSecret = ""
		} else if hasToken {
			cfg.Password = ""
		}

		if err := cfg.Validate(); err != nil {
			showWizardModal(pages, form, a.Application, "error", "Validation error: "+err.Error(), nil)
			return
		}

		// Create the profile config
		profileConfig := config.ProfileConfig{
			Addr:        cfg.Addr,
			User:        cfg.User,
			Password:    cfg.Password,
			TokenID:     cfg.TokenID,
			TokenSecret: cfg.TokenSecret,
			Realm:       cfg.Realm,
			ApiPath:     cfg.ApiPath,
			Insecure:    cfg.Insecure,
			SSHUser:     cfg.SSHUser,
		}

		if isNewProfile {
			// Add the new profile to the config
			if a.config.Profiles == nil {
				a.config.Profiles = make(map[string]config.ProfileConfig)
			}
			a.config.Profiles[profileName] = profileConfig
		} else {
			// Handle profile renaming
			existingProfileName := cfg.DefaultProfile
			if profileName != existingProfileName {
				// Profile is being renamed - remove old name and add new name
				delete(a.config.Profiles, existingProfileName)
				a.config.Profiles[profileName] = profileConfig
			} else {
				// Profile name unchanged - just update the config
				a.config.Profiles[existingProfileName] = profileConfig
			}
		}

		// Save the config first
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

	form.SetBorder(true).SetTitle("Proxmox TUI - Profile Configuration").SetTitleColor(theme.Colors.Primary)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			resultChan <- WizardResult{Canceled: true}
			return nil
		}
		return event
	})

	return pages
}
