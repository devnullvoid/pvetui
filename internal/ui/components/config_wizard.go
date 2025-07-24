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

// Add a helper ShowModalOnPages(pages *tview.Pages, form *tview.Form, message string)
func ShowModalOnPages(app *tview.Application, pages *tview.Pages, form *tview.Form, message string) {
	modal := tview.NewModal().SetText(message).AddButtons([]string{"OK"}).SetDoneFunc(func(_ int, _ string) {
		pages.RemovePage("modal")
		pages.SwitchToPage("form")
		app.SetFocus(form)
	})
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

// NewConfigWizardPage creates a new config wizard/editor form.
func NewConfigWizardPage(app *tview.Application, cfg *config.Config, configPath string, saveFn func(*config.Config) error, cancelFn func()) tview.Primitive {
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
			ShowModalOnPages(app, pages, form, "Please choose either password authentication or token authentication, not both.")
			return
		}
		if !hasPassword && !hasToken {
			ShowModalOnPages(app, pages, form, "You must provide either a password or a token for authentication.")
			return
		}
		if hasPassword {
			cfg.TokenID = ""
			cfg.TokenSecret = ""
		} else if hasToken {
			cfg.Password = ""
		}
		if err := cfg.Validate(); err != nil {
			ShowModalOnPages(app, pages, form, "Validation error: "+err.Error())
			return
		}
		// Save config first
		saveErr := saveFn(cfg)
		if saveErr != nil {
			ShowModalOnPages(app, pages, form, "Failed to save config: "+saveErr.Error())
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
						ShowModalOnPages(app, pages, form, "SOPS re-encryption failed: "+err.Error())
						return
					}
					ShowModalOnPages(app, pages, form, "Configuration saved and re-encrypted with SOPS!")
				} else {
					ShowModalOnPages(app, pages, form, "Configuration saved (unencrypted).")
				}
				app.Stop()
			})
			pages.AddPage("modal", modal, false, true)
			pages.SwitchToPage("modal")
			return
		}
		ShowModalOnPages(app, pages, form, "Configuration saved successfully!")
		app.Stop()
	})
	form.AddButton("Cancel", func() {
		if cancelFn != nil {
			cancelFn()
		}
		app.Stop()
	})
	form.SetBorder(true).SetTitle("Proxmox TUI - Config Wizard").SetTitleColor(theme.Colors.Primary)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if cancelFn != nil {
				cancelFn()
			}
			app.Stop()
			return nil
		}
		return event
	})
	return pages
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
