package components

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CleanConfig represents a clean config structure without legacy fields when profiles are used
type CleanConfig struct {
	Profiles       map[string]config.ProfileConfig `yaml:"profiles,omitempty"`
	DefaultProfile string                          `yaml:"default_profile,omitempty"`
	Debug          bool                            `yaml:"debug,omitempty"`
	CacheDir       string                          `yaml:"cache_dir,omitempty"`
	KeyBindings    config.KeyBindings              `yaml:"key_bindings,omitempty"`
	Theme          config.ThemeConfig              `yaml:"theme,omitempty"`
	// Legacy fields only included when no profiles are defined
	Addr        string `yaml:"addr,omitempty"`
	User        string `yaml:"user,omitempty"`
	Password    string `yaml:"password,omitempty"`
	TokenID     string `yaml:"token_id,omitempty"`
	TokenSecret string `yaml:"token_secret,omitempty"`
	Realm       string `yaml:"realm,omitempty"`
	ApiPath     string `yaml:"api_path,omitempty"`
	Insecure    bool   `yaml:"insecure,omitempty"`
	SSHUser     string `yaml:"ssh_user,omitempty"`
}

func configToYAML(cfg *config.Config) ([]byte, error) {
	// Create a clean config structure
	cleanConfig := CleanConfig{
		Profiles:       cfg.Profiles,
		DefaultProfile: cfg.DefaultProfile,
		Debug:          cfg.Debug,
		CacheDir:       cfg.CacheDir,
		KeyBindings:    cfg.KeyBindings,
		Theme:          cfg.Theme,
	}

	// Only include legacy fields if no profiles are defined (for backward compatibility)
	// When profiles are used, legacy fields should be omitted entirely
	if len(cfg.Profiles) == 0 {
		// Legacy mode - include legacy fields
		cleanConfig.Addr = cfg.Addr
		cleanConfig.User = cfg.User
		cleanConfig.Password = cfg.Password
		cleanConfig.TokenID = cfg.TokenID
		cleanConfig.TokenSecret = cfg.TokenSecret
		cleanConfig.Realm = cfg.Realm
		cleanConfig.ApiPath = cfg.ApiPath
		cleanConfig.Insecure = cfg.Insecure
		cleanConfig.SSHUser = cfg.SSHUser
	}
	// Note: When profiles are used, legacy fields are completely omitted

	return yaml.Marshal(cleanConfig)
}

// Update showWizardModal to accept an onClose callback.
func showWizardModal(pages *tview.Pages, form *tview.Form, app *tview.Application, kind, message string, onClose func()) {
	cb := func() {
		pages.RemovePage("modal")
		pages.SwitchToPage("form")
		app.SetFocus(form)

		if onClose != nil {
			onClose()
		}
	}

	var modal *tview.Modal

	if kind == "error" {
		modal = CreateErrorDialog("Error", message, cb)
	} else {
		modal = CreateInfoDialog("Info", message, cb)
	}

	pages.AddPage("modal", modal, false, true)
	pages.SwitchToPage("modal")
}

// (removed) FilepathBase is removed in favor of filepath.Dir which is OS-agnostic

// isSOPSEncrypted checks if a config file appears to be SOPS encrypted.
func isSOPSEncrypted(path string, data []byte) bool {
	// Use the config package's SOPS detection logic
	return config.IsSOPSEncrypted(path, data)
}

// findSOPSRule walks up parent directories to find a .sops.yaml, returns true if found.
func findSOPSRule(startDir string) bool {
	// Use the config package's SOPS rule detection logic
	return config.FindSOPSRule(startDir)
}

// WizardResult represents the result of a configuration wizard operation.
type WizardResult struct {
	Saved         bool
	SopsEncrypted bool
	Canceled      bool
	ProfileName   string
}

// NewConfigWizardPage creates a new configuration wizard page.
func NewConfigWizardPage(app *tview.Application, cfg *config.Config, configPath string, saveFn func(*config.Config) error, cancelFn func(), resultChan chan<- WizardResult) tview.Primitive {
	// Detect if original config was SOPS-encrypted
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

	form := tview.NewForm().SetHorizontal(false)
	pages := tview.NewPages()
	pages.AddPage("form", form, true, true)

	// Add profile name field at the top
	var profileName string
	var isDefaultProfile bool

	// Determine if this is a new profile or editing existing
	isNewProfile := len(cfg.Profiles) == 0 || cfg.DefaultProfile == ""

	if isNewProfile {
		form.AddInputField("Profile Name", "", 20, nil, func(text string) {
			profileName = strings.TrimSpace(text)
		})
	} else {
		// For editing existing profile, start with current name
		profileName = cfg.DefaultProfile
		form.AddInputField("Profile Name", cfg.DefaultProfile, 20, nil, func(text string) {
			profileName = strings.TrimSpace(text)
		})
	}

	// Add checkbox for default profile
	form.AddCheckbox("Set as Default Profile", isNewProfile || cfg.DefaultProfile == profileName, func(checked bool) {
		isDefaultProfile = checked
	})

	// Determine which data to use for form fields
	var addr, user, password, tokenID, tokenSecret, realm, apiPath, sshUser string
	var insecure bool

	// If we have profiles and a default profile, use profile data
	if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
		if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
			addr = profile.Addr
			user = profile.User
			password = profile.Password
			tokenID = profile.TokenID
			tokenSecret = profile.TokenSecret
			realm = profile.Realm
			apiPath = profile.ApiPath
			insecure = profile.Insecure
			sshUser = profile.SSHUser
		}
	} else {
		// Use legacy fields
		addr = cfg.Addr
		user = cfg.User
		password = cfg.Password
		tokenID = cfg.TokenID
		tokenSecret = cfg.TokenSecret
		realm = cfg.Realm
		apiPath = cfg.ApiPath
		insecure = cfg.Insecure
		sshUser = cfg.SSHUser
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
	form.AddCheckbox("Enable Debug Logging", cfg.Debug, func(checked bool) { cfg.Debug = checked })
	form.AddInputField("Cache Directory", cfg.CacheDir, 40, nil, func(text string) { cfg.CacheDir = strings.TrimSpace(text) })
	form.AddInputField("Theme Name", cfg.Theme.Name, 20, nil, func(text string) { cfg.Theme.Name = strings.TrimSpace(text) })
	form.AddButton("Save", func() {
		// Validate profile name
		if profileName == "" {
			showWizardModal(pages, form, app, "error", "Profile name cannot be empty.", nil)
			return
		}

		// Check if profile already exists (for new profiles or renamed profiles)
		if isNewProfile || profileName != cfg.DefaultProfile {
			if cfg.Profiles != nil && cfg.Profiles[profileName] != (config.ProfileConfig{}) {
				showWizardModal(pages, form, app, "error", "Profile '"+profileName+"' already exists.", nil)
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
			showWizardModal(pages, form, app, "error", "Please choose either password authentication or token authentication, not both.", nil)
			return
		}

		if !hasPassword && !hasToken {
			showWizardModal(pages, form, app, "error", "You must provide either a password or a token for authentication.", nil)
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

		// Handle profile creation/updating
		if isNewProfile {
			// Create new profile
			if cfg.Profiles == nil {
				cfg.Profiles = make(map[string]config.ProfileConfig)
			}

			// Create profile from current form data
			newProfile := config.ProfileConfig{
				Addr:        strings.TrimSpace(addr),
				User:        strings.TrimSpace(user),
				Password:    password,
				TokenID:     strings.TrimSpace(tokenID),
				TokenSecret: tokenSecret,
				Realm:       strings.TrimSpace(realm),
				ApiPath:     strings.TrimSpace(apiPath),
				Insecure:    insecure,
				SSHUser:     strings.TrimSpace(sshUser),
			}

			// Clear conflicting auth method in new profile
			if hasPassword {
				newProfile.TokenID = ""
				newProfile.TokenSecret = ""
			} else if hasToken {
				newProfile.Password = ""
			}

			cfg.Profiles[profileName] = newProfile

			// Set as default if requested
			if isDefaultProfile {
				cfg.DefaultProfile = profileName
			}
		} else {
			// Update existing profile
			if profile, exists := cfg.Profiles[cfg.DefaultProfile]; exists {
				// Handle profile renaming
				if profileName != cfg.DefaultProfile {
					// Store the old profile name before deleting
					oldProfileName := cfg.DefaultProfile

					// Remove old profile name
					delete(cfg.Profiles, cfg.DefaultProfile)

					// Update default profile if we're renaming the current default
					if cfg.DefaultProfile == oldProfileName {
						cfg.DefaultProfile = profileName
					}
				}

				// The profile data has already been updated by the form field handlers
				// Just clear conflicting auth method if needed
				if hasPassword {
					profile.TokenID = ""
					profile.TokenSecret = ""
				} else if hasToken {
					profile.Password = ""
				}

				cfg.Profiles[profileName] = profile
			}

			// Update default profile setting
			if isDefaultProfile {
				cfg.DefaultProfile = profileName
			}
		}

		if err := cfg.Validate(); err != nil {
			showWizardModal(pages, form, app, "error", "Validation error: "+err.Error(), nil)
			return
		}
		// Save config first
		saveErr := saveFn(cfg)
		if saveErr != nil {
			showWizardModal(pages, form, app, "error", "Failed to save config: "+saveErr.Error(), nil)
			return
		}
		// If SOPS re-encryption is possible, prompt user
		if wasSOPS && sopsRuleExists {
			onYes := func() {
				cmd := exec.Command("sops", "-e", "-i", configPath)

				err := cmd.Run()
				if err != nil {
					showWizardModal(pages, form, app, "error", "SOPS re-encryption failed: "+err.Error(), nil)
					return
				}

				showWizardModal(pages, form, app, "info", "Configuration saved and re-encrypted with SOPS!", func() {
					resultChan <- WizardResult{Saved: true, SopsEncrypted: true, ProfileName: profileName}
					app.Stop()
				})
			}
			onNo := func() {
				showWizardModal(pages, form, app, "info", "Configuration saved (unencrypted).", func() {
					resultChan <- WizardResult{Saved: true, ProfileName: profileName}
					app.Stop()
				})
			}
			confirm := CreateConfirmDialog("SOPS Re-encryption", "The original config was SOPS-encrypted. Re-encrypt the new config with SOPS?", onYes, onNo)
			pages.AddPage("modal", confirm, false, true)
			pages.SwitchToPage("modal")
			return
		}

		showWizardModal(pages, form, app, "info", "Configuration saved successfully!", func() {
			resultChan <- WizardResult{Saved: true, ProfileName: profileName}
			app.Stop()
		})
	})
	form.AddButton("Cancel", func() {
		if cancelFn != nil {
			cancelFn()
		}
		resultChan <- WizardResult{Canceled: true}
		app.Stop()
	})
	form.SetBorder(true).SetTitle("Proxmox TUI - Config Wizard").SetTitleColor(theme.Colors.Primary).SetBorderColor(theme.Colors.Border)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if cancelFn != nil {
				cancelFn()
			}
			resultChan <- WizardResult{Canceled: true}
			app.Stop()
			return nil
		}
		return event
	})

	return pages
}

// SaveConfigToFile writes the config to the given path in YAML format.
func SaveConfigToFile(cfg *config.Config, path string) error {
	// Use the config package's YAML marshaling
	data, err := configToYAML(cfg)
	if err != nil {
		return err
	}

	// Ensure the target directory exists using OS-agnostic path operations
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// LaunchConfigWizard launches the configuration wizard and returns the result.
func LaunchConfigWizard(cfg *config.Config, configPath string, activeProfile string) WizardResult {
	tviewApp := tview.NewApplication()

	// Apply theme configuration first, then apply to tview (same as main application)
	theme.ApplyCustomTheme(&cfg.Theme)
	theme.ApplyToTview()

	resultChan := make(chan WizardResult, 1)
	wizard := NewConfigWizardPage(tviewApp, cfg, configPath, func(c *config.Config) error {
		// The form now handles profile updates directly
		return SaveConfigToFile(c, configPath)
	}, func() {
		tviewApp.Stop()
	}, resultChan)
	tviewApp.SetRoot(wizard, true)
	_ = tviewApp.Run()

	return <-resultChan
}
