package components

import (
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

		// Send saved result without stopping the app
		resultChan <- WizardResult{Saved: true}
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
