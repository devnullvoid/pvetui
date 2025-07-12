package components

import (
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// VMConfigPage is a modal/page for editing VM or LXC configuration.
type VMConfigPage struct {
	*tview.Form
	app    *App
	vm     *api.VM
	config *api.VMConfig
	saveFn func(*api.VMConfig) error
}

// NewVMConfigPage creates a new config editor for the given VM.
func NewVMConfigPage(app *App, vm *api.VM, config *api.VMConfig, saveFn func(*api.VMConfig) error) *VMConfigPage {
	form := tview.NewForm().SetHorizontal(true)
	page := &VMConfigPage{
		Form:   form,
		app:    app,
		vm:     vm,
		config: config,
		saveFn: saveFn,
	}

	// CPU cores
	form.AddInputField("Cores", strconv.Itoa(config.Cores), 4, nil, func(text string) {
		if v, err := strconv.Atoi(text); err == nil {
			page.config.Cores = v
		}
	})
	// Sockets (QEMU only)
	if vm.Type == api.VMTypeQemu {
		form.AddInputField("Sockets", strconv.Itoa(config.Sockets), 4, nil, func(text string) {
			if v, err := strconv.Atoi(text); err == nil {
				page.config.Sockets = v
			}
		})
	}
	// Memory (MB)
	form.AddInputField("Memory (MB)", strconv.FormatInt(config.Memory/1024/1024, 10), 8, nil, func(text string) {
		if v, err := strconv.ParseInt(text, 10, 64); err == nil {
			page.config.Memory = v * 1024 * 1024
		}
	})
	// Description
	form.AddInputField("Description", config.Description, 32, nil, func(text string) {
		page.config.Description = text
	})
	// OnBoot
	onboot := false
	if config.OnBoot != nil {
		onboot = *config.OnBoot
	}
	form.AddCheckbox("Start at boot", onboot, func(checked bool) {
		page.config.OnBoot = &checked
	})
	// Save/Cancel buttons
	form.AddButton("Save", func() {
		err := page.saveFn(page.config)
		if err != nil {
			app.showMessage(fmt.Sprintf("Failed to save config: %v", err))
		} else {
			app.showMessage("Configuration updated successfully.")
			app.pages.RemovePage("vmConfig")
		}
	})
	form.AddButton("Cancel", func() {
		app.pages.RemovePage("vmConfig")
	})
	// Set dynamic title with guest info
	guestType := "VM"
	if vm.Type == api.VMTypeLXC {
		guestType = "CT"
	}
	title := fmt.Sprintf("Edit Configuration: %s %d - %s", guestType, vm.ID, vm.Name)
	form.SetBorder(true).SetTitle(title).SetTitleColor(tcell.ColorYellow)
	return page
}
