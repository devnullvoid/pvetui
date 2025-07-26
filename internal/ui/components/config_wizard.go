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

// configToYAML marshals the config to YAML.
func configToYAML(cfg *config.Config) ([]byte, error) {
	return yaml.Marshal(cfg)
}

// Update showWizardModal to accept an onClose callback
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

// FilepathBase returns the last element of the path.
func FilepathBase(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// isSOPSEncrypted checks if a config file appears to be SOPS encrypted (local copy of config.isSOPSEncrypted).
func isSOPSEncrypted(path string, data []byte) bool {
	if strings.HasSuffix(path, ".enc.yaml") || strings.HasSuffix(path, ".enc.yml") ||
		strings.HasSuffix(path, ".sops.yaml") || strings.HasSuffix(path, ".sops.yml") {
		return true
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err == nil {
		if _, ok := m["sops"]; ok {
			return true
		}
	}
	return false
}

// findSOPSRule walks up parent directories to find a .sops.yaml, returns true if found.
func findSOPSRule(startDir string) bool {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".sops.yaml")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

// Add WizardResult struct
type WizardResult struct {
	Saved         bool
	SopsEncrypted bool
	Canceled      bool
}

// Update NewConfigWizardPage to accept a resultChan chan<- WizardResult
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
	form.AddInputField("Proxmox API URL", cfg.Addr, 40, nil, func(text string) { cfg.Addr = strings.TrimSpace(text) })
	form.AddInputField("Username", cfg.User, 20, nil, func(text string) { cfg.User = strings.TrimSpace(text) })
	form.AddPasswordField("Password", cfg.Password, 20, '*', func(text string) { cfg.Password = text })
	form.AddInputField("API Token ID", cfg.TokenID, 20, nil, func(text string) { cfg.TokenID = strings.TrimSpace(text) })
	form.AddPasswordField("API Token Secret", cfg.TokenSecret, 20, '*', func(text string) { cfg.TokenSecret = text })
	form.AddInputField("Realm", cfg.Realm, 10, nil, func(text string) { cfg.Realm = strings.TrimSpace(text) })
	form.AddInputField("API Path", cfg.ApiPath, 20, nil, func(text string) { cfg.ApiPath = strings.TrimSpace(text) })
	form.AddCheckbox("Skip TLS Verification", cfg.Insecure, func(checked bool) { cfg.Insecure = checked })
	form.AddInputField("SSH Username", cfg.SSHUser, 20, nil, func(text string) { cfg.SSHUser = strings.TrimSpace(text) })
	form.AddCheckbox("Enable Debug Logging", cfg.Debug, func(checked bool) { cfg.Debug = checked })
	form.AddInputField("Cache Directory", cfg.CacheDir, 40, nil, func(text string) { cfg.CacheDir = strings.TrimSpace(text) })
	form.AddInputField("Theme Name", cfg.Theme.Name, 20, nil, func(text string) { cfg.Theme.Name = strings.TrimSpace(text) })
	form.AddButton("Save", func() {
		hasPassword := cfg.Password != ""
		hasToken := cfg.TokenID != "" && cfg.TokenSecret != ""
		if hasPassword && hasToken {
			showWizardModal(pages, form, app, "error", "Please choose either password authentication or token authentication, not both.", nil)
			return
		}
		if !hasPassword && !hasToken {
			showWizardModal(pages, form, app, "error", "You must provide either a password or a token for authentication.", nil)
			return
		}
		if hasPassword {
			cfg.TokenID = ""
			cfg.TokenSecret = ""
		} else if hasToken {
			cfg.Password = ""
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
					resultChan <- WizardResult{Saved: true, SopsEncrypted: true}
					app.Stop()
				})
			}
			onNo := func() {
				showWizardModal(pages, form, app, "info", "Configuration saved (unencrypted).", func() {
					resultChan <- WizardResult{Saved: true}
					app.Stop()
				})
			}
			confirm := CreateConfirmDialog("SOPS Re-encryption", "The original config was SOPS-encrypted. Re-encrypt the new config with SOPS?", onYes, onNo)
			pages.AddPage("modal", confirm, false, true)
			pages.SwitchToPage("modal")
			return
		}
		showWizardModal(pages, form, app, "info", "Configuration saved successfully!", func() {
			resultChan <- WizardResult{Saved: true}
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
	form.SetBorder(true).SetTitle("Proxmox TUI - Config Wizard").SetTitleColor(theme.Colors.Primary)
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
	if err := os.MkdirAll(strings.TrimSuffix(path, "/"+FilepathBase(path)), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
