package components

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/internal/keys"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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
func NewConfigWizardPage(app *tview.Application, cfg *config.Config, configPath string, saveFn func(*config.Config) error, cancelFn func(), resultChan chan<- WizardResult, targetProfile string) tview.Primitive {
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
	originalProfileName := ""

	// Determine if we are editing an existing profile
	var isEditing bool
	resolveProfileForWizard := func(candidate string) (string, bool) {
		if candidate == "" {
			return "", false
		}

		if cfg.Profiles != nil {
			if _, exists := cfg.Profiles[candidate]; exists {
				return candidate, true
			}
		}

		// If a group name is supplied, edit the first member profile (stable order).
		if cfg.IsGroup(candidate) {
			members := cfg.GetProfileNamesInGroup(candidate)
			if len(members) > 0 {
				if _, exists := cfg.Profiles[members[0]]; exists {
					return members[0], true
				}
			}
		}

		// Unknown name: treat as creating a new profile.
		return candidate, false
	}

	if targetProfile != "" {
		// User requested specific profile/group
		profileName, isEditing = resolveProfileForWizard(targetProfile)
	} else {
		// Fallback to default behavior
		// If we have profiles, default to editing the default profile
		if len(cfg.Profiles) > 0 && cfg.DefaultProfile != "" {
			profileName, isEditing = resolveProfileForWizard(cfg.DefaultProfile)
		}
		// If no profiles, profileName remains empty, isEditing false
	}
	if isEditing {
		originalProfileName = profileName
	}

	isNewProfile := !isEditing

	form.AddInputField("Profile Name", profileName, 20, nil, func(text string) {
		profileName = strings.TrimSpace(text)
	})

	// Add checkbox for default profile
	defaultChecked := isNewProfile || (originalProfileName != "" && cfg.DefaultProfile == originalProfileName)
	defaultCheckbox := tview.NewCheckbox().SetLabel("Set as Default Profile").SetChecked(defaultChecked)
	form.AddFormItem(defaultCheckbox)

	// Determine which data to use for form fields
	var addr, user, password, tokenID, tokenSecret, realm, apiPath, sshUser, vmSSHUser string
	var insecure bool

	// If we are editing a profile, use its data
	//nolint:dupl // Shared with profile wizard to keep legacy/profile editing consistent
	if isEditing {
		if profile, exists := cfg.Profiles[originalProfileName]; exists {
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
		}
	} else {
		// Use legacy fields or defaults
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
	}

	form.AddInputField("Proxmox API URL", addr, 40, nil, func(text string) {
		addr = strings.TrimSpace(text)
		if !isEditing {
			cfg.Addr = addr
		}
	})
	form.AddInputField("Username", user, 20, nil, func(text string) {
		user = strings.TrimSpace(text)
		if !isEditing {
			cfg.User = user
		}
	})
	form.AddPasswordField("Password", password, 20, '*', func(text string) {
		password = text
		if !isEditing {
			cfg.Password = password
		}
	})
	form.AddInputField("API Token ID", tokenID, 20, nil, func(text string) {
		tokenID = strings.TrimSpace(text)
		if !isEditing {
			cfg.TokenID = tokenID
		}
	})
	form.AddPasswordField("API Token Secret", tokenSecret, 20, '*', func(text string) {
		tokenSecret = text
		if !isEditing {
			cfg.TokenSecret = tokenSecret
		}
	})
	form.AddInputField("Realm", realm, 10, nil, func(text string) {
		realm = strings.TrimSpace(text)
		if !isEditing {
			cfg.Realm = realm
		}
	})
	form.AddInputField("API Path", apiPath, 20, nil, func(text string) {
		apiPath = strings.TrimSpace(text)
		if !isEditing {
			cfg.ApiPath = apiPath
		}
	})
	form.AddCheckbox("Skip TLS Verification", insecure, func(checked bool) {
		insecure = checked
		if !isEditing {
			cfg.Insecure = insecure
		}
	})
	form.AddInputField("SSH Username", sshUser, 20, nil, func(text string) {
		sshUser = strings.TrimSpace(text)
		if !isEditing {
			cfg.SSHUser = sshUser
		}
	})
	form.AddInputField("VM SSH Username", vmSSHUser, 20, nil, func(text string) {
		vmSSHUser = strings.TrimSpace(text)
		if !isEditing {
			cfg.VMSSHUser = vmSSHUser
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
		if cfg.Profiles != nil {
			if isNewProfile || (isEditing && profileName != originalProfileName) {
				if _, exists := cfg.Profiles[profileName]; exists {
					showWizardModal(pages, form, app, "error", "Profile '"+profileName+"' already exists.", nil)
					return
				}
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
		setAsDefault := defaultCheckbox.IsChecked()
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
			if setAsDefault {
				cfg.DefaultProfile = profileName
			}
		} else {
			// Update existing profile
			profile, exists := cfg.Profiles[originalProfileName]
			if !exists {
				showWizardModal(pages, form, app, "error", "Profile '"+originalProfileName+"' not found.", nil)
				return
			}

			// Preserve group memberships (not editable in this wizard).
			updated := config.ProfileConfig{
				Addr:        strings.TrimSpace(addr),
				User:        strings.TrimSpace(user),
				Password:    password,
				TokenID:     strings.TrimSpace(tokenID),
				TokenSecret: tokenSecret,
				Realm:       strings.TrimSpace(realm),
				ApiPath:     strings.TrimSpace(apiPath),
				Insecure:    insecure,
				SSHUser:     strings.TrimSpace(sshUser),
				VMSSHUser:   strings.TrimSpace(vmSSHUser),
				Groups:      append([]string{}, profile.Groups...),
			}

			if hasPassword {
				updated.TokenID = ""
				updated.TokenSecret = ""
			} else if hasToken {
				updated.Password = ""
			}

			// Rename if needed.
			if profileName != originalProfileName {
				delete(cfg.Profiles, originalProfileName)
			}
			cfg.Profiles[profileName] = updated

			// If we renamed the current default profile, keep the default pointing at the renamed profile.
			if cfg.DefaultProfile == originalProfileName {
				cfg.DefaultProfile = profileName
			}

			// Or set as default if explicitly requested.
			if setAsDefault {
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
	form.SetBorder(true).SetTitle("pvetui - Config Wizard").SetTitleColor(theme.Colors.Primary).SetBorderColor(theme.Colors.Border)
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
	// Check if SOPS is being used (read file to check)
	data, err := os.ReadFile(path)
	isSOPS := false
	if err == nil {
		isSOPS = config.IsSOPSEncrypted(path, data)
	}

	// If not using SOPS, encrypt sensitive fields before saving
	if !isSOPS {
		// Create a copy of the config to encrypt (don't modify the original)
		cfgCopy := *cfg
		cfgCopy.Profiles = make(map[string]config.ProfileConfig)
		for k, v := range cfg.Profiles {
			cfgCopy.Profiles[k] = v
		}

		// Encrypt sensitive fields
		if err := config.EncryptConfigSensitiveFields(&cfgCopy); err != nil {
			// Log warning but continue - allow saving even if encryption fails
			// This allows users to save cleartext if they prefer
			if config.DebugEnabled {
				fmt.Printf("⚠️  Warning: Failed to encrypt some fields: %v\n", err)
			}
		} else {
			cfg = &cfgCopy
		}
	}

	// Use the config package's YAML marshaling
	data, err = yaml.Marshal(cfg)
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
	tviewApp.SetInputCapture(keys.NormalizeNavigationEvent)

	// Apply theme configuration first, then apply to tview (same as main application)
	theme.ApplyCustomTheme(&cfg.Theme)
	theme.ApplyToTview()

	resultChan := make(chan WizardResult, 1)
	wizard := NewConfigWizardPage(tviewApp, cfg, configPath, func(c *config.Config) error {
		// The form now handles profile updates directly
		return SaveConfigToFile(c, configPath)
	}, func() {
		tviewApp.Stop()
	}, resultChan, activeProfile)
	tviewApp.SetRoot(wizard, true)
	_ = tviewApp.Run()

	return <-resultChan
}
