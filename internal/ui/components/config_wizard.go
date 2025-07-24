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

// showModal displays a simple modal dialog with a message and an OK button, then calls onClose and always calls app.Stop().
func ShowModal(app *tview.Application, message string, onClose func()) {
	modal := tview.NewModal().SetText(message).AddButtons([]string{"OK"}).SetDoneFunc(func(_ int, _ string) {
		if onClose != nil {
			onClose()
		}
		app.Stop()
	})
	app.SetRoot(modal, true)
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

// ConfigWizardPage is a TUI form for creating or editing the main config file.
type ConfigWizardPage struct {
	*tview.Form
	app        *tview.Application
	config     *config.Config
	configPath string
	saveFn     func(*config.Config) error
	cancelFn   func()
}

// NewConfigWizardPage creates a new config wizard/editor form.
func NewConfigWizardPage(app *tview.Application, cfg *config.Config, configPath string, saveFn func(*config.Config) error, cancelFn func()) *ConfigWizardPage {
	form := tview.NewForm().SetHorizontal(false)
	page := &ConfigWizardPage{
		Form:       form,
		app:        app,
		config:     cfg,
		configPath: configPath,
		saveFn:     saveFn,
		cancelFn:   cancelFn,
	}

	// Detect if original config was SOPS-encrypted
	wasSOPS := false
	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			wasSOPS = isSOPSEncrypted(configPath, data)
		}
	}
	// Check for .sops.yaml in config dir
	sopsRuleExists := false
	if configPath != "" {
		sopsRuleExists = findSOPSRule(filepath.Dir(configPath))
	}

	// Add fields for all config options
	form.AddInputField("Proxmox API URL", cfg.Addr, 40, nil, func(text string) { page.config.Addr = strings.TrimSpace(text) })
	form.AddInputField("Username", cfg.User, 20, nil, func(text string) { page.config.User = strings.TrimSpace(text) })
	form.AddPasswordField("Password", cfg.Password, 20, '*', func(text string) { page.config.Password = text })
	form.AddInputField("API Token ID", cfg.TokenID, 20, nil, func(text string) { page.config.TokenID = strings.TrimSpace(text) })
	form.AddPasswordField("API Token Secret", cfg.TokenSecret, 20, '*', func(text string) { page.config.TokenSecret = text })
	form.AddInputField("Realm", cfg.Realm, 10, nil, func(text string) { page.config.Realm = strings.TrimSpace(text) })
	form.AddInputField("API Path", cfg.ApiPath, 20, nil, func(text string) { page.config.ApiPath = strings.TrimSpace(text) })
	form.AddCheckbox("Skip TLS Verification", cfg.Insecure, func(checked bool) { page.config.Insecure = checked })
	form.AddInputField("SSH Username", cfg.SSHUser, 20, nil, func(text string) { page.config.SSHUser = strings.TrimSpace(text) })
	form.AddCheckbox("Enable Debug Logging", cfg.Debug, func(checked bool) { page.config.Debug = checked })
	form.AddInputField("Cache Directory", cfg.CacheDir, 40, nil, func(text string) { page.config.CacheDir = strings.TrimSpace(text) })
	form.AddInputField("Theme Name", cfg.Theme.Name, 20, nil, func(text string) { page.config.Theme.Name = strings.TrimSpace(text) })
	// TODO: Optionally add color overrides and keybindings as advanced sections

	form.AddButton("Save", func() {
		if err := page.config.Validate(); err != nil {
			ShowModal(app, "Validation error: "+err.Error(), func() { app.SetRoot(page, true) })
			return
		}
		// Save config first
		saveErr := page.saveFn(page.config)
		if saveErr != nil {
			ShowModal(app, "Failed to save config: "+saveErr.Error(), func() { app.SetRoot(page, true) })
			return
		}
		// If SOPS re-encryption is possible, prompt user
		if wasSOPS && sopsRuleExists {
			modal := tview.NewModal().SetText("The original config was SOPS-encrypted. Re-encrypt the new config with SOPS?").AddButtons([]string{"Yes", "No"}).SetDoneFunc(func(i int, label string) {
				if label == "Yes" {
					// Run sops -e -i <configPath>
					cmd := exec.Command("sops", "-e", "-i", configPath)
					err := cmd.Run()
					if err != nil {
						ShowModal(app, "SOPS re-encryption failed: "+err.Error(), func() {
							if page.cancelFn != nil {
								page.cancelFn()
							}
						})
						return
					}
					ShowModal(app, "Configuration saved and re-encrypted with SOPS!", func() {
						if page.cancelFn != nil {
							page.cancelFn()
						}
					})
				} else {
					ShowModal(app, "Configuration saved (unencrypted).", func() {
						if page.cancelFn != nil {
							page.cancelFn()
						}
					})
				}
				app.Stop()
			})
			app.SetRoot(modal, true)
			return
		}
		ShowModal(app, "Configuration saved successfully!", func() {
			if page.cancelFn != nil {
				page.cancelFn()
			}
		})
		app.Stop()
	})
	form.AddButton("Cancel", func() {
		if page.cancelFn != nil {
			page.cancelFn()
		}
		app.Stop()
	})
	form.SetBorder(true).SetTitle("Proxmox TUI - Config Wizard").SetTitleColor(theme.Colors.Primary)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if page.cancelFn != nil {
				page.cancelFn()
			}
			app.Stop()
			return nil
		}
		return event
	})
	return page
}

// SaveConfigToFile writes the config to the given path in YAML format.
func SaveConfigToFile(cfg *config.Config, path string) error {
	// Use the config package's YAML marshalling
	data, err := configToYAML(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(strings.TrimSuffix(path, "/"+FilepathBase(path)), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
