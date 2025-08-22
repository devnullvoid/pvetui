package components

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/peevetui/internal/ui/theme"
	"github.com/devnullvoid/peevetui/internal/version"
)

// CreateLoginForm creates a login form dialog.
func CreateLoginForm() *tview.Form {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Login ")
	form.SetTitleColor(theme.Colors.Primary)
	form.SetBorderColor(theme.Colors.Border)

	// Add form fields
	form.AddInputField("Server URL", "", 30, nil, nil)
	form.AddInputField("Username", "", 20, nil, nil)
	form.AddPasswordField("Password", "", 20, '*', nil)
	form.AddInputField("Realm", "pam", 10, nil, nil)

	// Add buttons
	form.AddButton("Login", nil)
	form.AddButton("Cancel", nil)

	return form
}

// CreateConfirmDialog creates a confirmation dialog.
func CreateConfirmDialog(title, message string, onConfirm, onCancel func()) *tview.Modal {
	modal := tview.NewModal()
	modal.SetText(message)
	// modal.SetBackgroundColor(theme.Colors.Background)
	modal.SetTextColor(theme.Colors.Primary)
	modal.SetBorderColor(theme.Colors.Border)
	modal.SetTitle(title)
	modal.SetTitleColor(theme.Colors.Title)

	// Add buttons
	modal.AddButtons([]string{"Yes", "No"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonIndex == 0 && onConfirm != nil {
			onConfirm()
		} else if onCancel != nil {
			onCancel()
		}
	})

	// Add keyboard shortcuts for Y/N keys
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'y', 'Y':
			if onConfirm != nil {
				onConfirm()
			}
			return nil
		case 'n', 'N':
			if onCancel != nil {
				onCancel()
			}
			return nil
		}
		return event
	})

	return modal
}

// createBaseModal creates a base modal with common configuration.
func createBaseModal(title, message string, textColor tcell.Color, onClose func()) *tview.Modal {
	modal := tview.NewModal()
	modal.SetText(message)
	// modal.SetBackgroundColor(theme.Colors.Background)
	modal.SetTextColor(textColor)
	modal.SetBorderColor(theme.Colors.Border)
	modal.SetTitle(title)
	modal.SetTitleColor(theme.Colors.Title)

	// Add close button
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if onClose != nil {
			onClose()
		}
	})

	return modal
}

// CreateInfoDialog creates an information dialog.
func CreateInfoDialog(title, message string, onClose func()) *tview.Modal {
	return createBaseModal(title, message, theme.Colors.Primary, onClose)
}

// CreateErrorDialog creates an error dialog.
func CreateErrorDialog(title, message string, onClose func()) *tview.Modal {
	return createBaseModal(title, message, theme.Colors.Error, onClose)
}

// CreateErrorDialogWithScrollableText creates an error dialog with scrollable text for long URLs.
func CreateErrorDialogWithScrollableText(title, message string, onClose func()) *tview.Modal {
	// Create a modal with the message
	modal := tview.NewModal()
	modal.SetText(message)
	modal.SetTextColor(theme.Colors.Error)
	modal.SetBorderColor(theme.Colors.Border)
	modal.SetTitle(title)
	modal.SetTitleColor(theme.Colors.Title)

	// Add close button
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if onClose != nil {
			onClose()
		}
	})

	// Add keyboard shortcuts for dismissal
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			if onClose != nil {
				onClose()
			}
			return nil
		}
		return event
	})

	return modal
}

// CreateSuccessDialogWithURL creates a success dialog with URL information for VNC connections.
func CreateSuccessDialogWithURL(title, message string, onClose func()) *tview.Modal {
	// Create a modal with the message
	modal := tview.NewModal()
	modal.SetText(message)
	modal.SetTextColor(theme.Colors.Success)
	modal.SetBorderColor(theme.Colors.Border)
	modal.SetTitle(title)
	modal.SetTitleColor(theme.Colors.Title)

	// Add close button
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if onClose != nil {
			onClose()
		}
	})

	// Add keyboard shortcuts for dismissal
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			if onClose != nil {
				onClose()
			}
			return nil
		}
		return event
	})

	return modal
}

// CreateFormDialog creates a form dialog with custom fields.
func CreateFormDialog(title string, fields []FormField, onSubmit, onCancel func(map[string]string)) *tview.Form {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" %s ", title))
	form.SetTitleColor(theme.Colors.Title)
	form.SetBorderColor(theme.Colors.Border)

	// Add form fields
	fieldValues := make(map[string]string)

	for _, field := range fields {
		fieldName := field.Name
		form.AddInputField(field.Label, field.DefaultValue, field.MaxLength, nil, func(text string) {
			fieldValues[fieldName] = text
		})
	}

	// Add buttons
	form.AddButton("Submit", func() {
		if onSubmit != nil {
			onSubmit(fieldValues)
		}
	})
	form.AddButton("Cancel", func() {
		if onCancel != nil {
			onCancel(fieldValues)
		}
	})

	return form
}

// FormField represents a form field.
type FormField struct {
	Name         string
	Label        string
	DefaultValue string
	MaxLength    int
}

// CreateAboutDialog creates an about dialog with version information and links.
func CreateAboutDialog(versionInfo *version.BuildInfo, onClose func()) *tview.Modal {
	// Format build date for display
	buildDateDisplay := versionInfo.BuildDate
	if buildDateDisplay != "unknown" {
		if parsed, err := time.Parse(time.RFC3339, buildDateDisplay); err == nil {
			buildDateDisplay = parsed.Format("2006-01-02 15:04:05 UTC")
		}
	}

	// Create about text with dynamic information
	aboutText := fmt.Sprintf(`Proxmox TUI

A terminal user interface for Proxmox VE

Version: %s
Build Date: %s
Commit: %s
Go Version: %s
OS/Arch: %s/%s

Copyright © %s %s
Licensed under the %s

GitHub: %s
Releases: %s`,
		versionInfo.Version,
		buildDateDisplay,
		versionInfo.Commit,
		versionInfo.GoVersion,
		versionInfo.OS,
		versionInfo.Arch,
		version.GetCopyrightYearRange(),
		version.Author,
		version.License,
		version.GetGitHubURL(),
		version.GetGitHubReleaseURL())

	return createBaseModal("About", aboutText, theme.Colors.Primary, onClose)
}
