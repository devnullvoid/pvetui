package components

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
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
	form := tview.NewForm().SetHorizontal(false)
	page := &VMConfigPage{
		Form:   form,
		app:    app,
		vm:     vm,
		config: config,
		saveFn: saveFn,
	}

	// Add Resize Storage Volume button as a FormButton at the top (left-aligned)
	resizeBtn := NewFormButton("Resize Storage Volume", func() {
		showResizeStorageModal(app, vm)
	}).SetAlignment(AlignLeft)
	form.AddFormItem(resizeBtn)
	// Restore to simple vertical layout for Cores, Sockets, Memory (MB)
	form.SetHorizontal(false)
	form.AddInputField("Cores", strconv.Itoa(config.Cores), 4, func(textToCheck string, lastChar rune) bool {
		return lastChar >= '0' && lastChar <= '9'
	}, func(text string) {
		if v, err := strconv.Atoi(text); err == nil {
			page.config.Cores = v
		}
	})

	if vm.Type == api.VMTypeQemu {
		form.AddInputField("Sockets", strconv.Itoa(config.Sockets), 4, func(textToCheck string, lastChar rune) bool {
			return lastChar >= '0' && lastChar <= '9'
		}, func(text string) {
			if v, err := strconv.Atoi(text); err == nil {
				page.config.Sockets = v
			}
		})
	}

	form.AddInputField("Memory (MB)", strconv.FormatInt(config.Memory/1024/1024, 10), 8, func(textToCheck string, lastChar rune) bool {
		return lastChar >= '0' && lastChar <= '9'
	}, func(text string) {
		if v, err := strconv.ParseInt(text, 10, 64); err == nil {
			page.config.Memory = v * 1024 * 1024
		}
	})

	// Description
	initialDesc := utils.TrimTrailingWhitespace(config.Description)
	form.AddTextArea("Description", initialDesc, 0, 3, 0, func(text string) {
		page.config.Description = utils.TrimTrailingWhitespace(text)
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
		// Show loading indicator
		app.header.ShowLoading(fmt.Sprintf("Saving configuration for %s...", vm.Name))

		// Run save operation in goroutine to avoid blocking UI
		go func() {
			err := page.saveFn(page.config)

			app.QueueUpdateDraw(func() {
				if err != nil {
					app.header.ShowError(fmt.Sprintf("Failed to save config: %v", err))
				} else {
					app.header.ShowSuccess("Configuration updated successfully.")
					// Remove the config page first
					app.pages.RemovePage("vmConfig")
					// Then refresh data
					app.manualRefresh()
				}
			})
		}()
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
	form.SetBorder(true).SetTitle(title).SetTitleColor(theme.Colors.Primary)
	// Set ESC key to cancel
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if err := app.pages.RemovePage("vmConfig"); err != nil {
				models.GetUILogger().Error("Failed to remove vmConfig page: %v", err)
			}

			return nil
		}

		return event
	})
	// // Set initial focus to the first field (Resize Storage Volume)
	// form.SetFocus(0)
	return page
}

// showResizeStorageModal displays a modal for resizing a storage volume.
func showResizeStorageModal(app *App, vm *api.VM) {
	modal := tview.NewForm().SetHorizontal(false)

	// Build list of storage devices (filter to only resizable volumes)
	var deviceNames []string

	deviceMap := make(map[string]*api.StorageDevice)

	for _, dev := range vm.StorageDevices {
		if dev.Size == "" {
			continue // must have a size
		}

		if dev.Media == "cdrom" {
			continue // skip CD-ROM/ISO
		}

		if strings.HasPrefix(dev.Device, "efidisk") || strings.HasPrefix(dev.Device, "scsihw") {
			continue // skip EFI/controller
		}

		label := fmt.Sprintf("%s (%s, %s)", dev.Device, dev.Storage, dev.Size)
		deviceNames = append(deviceNames, label)
		deviceMap[label] = &dev
	}

	selectedDevice := ""
	if len(deviceNames) > 0 {
		selectedDevice = deviceNames[0]
	}

	modal.AddDropDown("Volume", deviceNames, 0, func(option string, idx int) {
		selectedDevice = option
	})
	modal.AddInputField("Expand by (GB)", "", 8, func(textToCheck string, lastChar rune) bool {
		if lastChar < '0' || lastChar > '9' {
			return false
		}

		return true
	}, nil)

	modal.AddButton("Resize", func() {
		amountField, ok := modal.GetFormItemByLabel("Expand by (GB)").(*tview.InputField)
		if !ok {
			app.showMessageSafe("Failed to get amount field.")

			return
		}

		amountStr := amountField.GetText()

		amount, err := strconv.Atoi(amountStr)
		if err != nil || amount <= 0 {
			app.showMessageSafe("Please enter a positive number of GB.")

			return
		}

		if selectedDevice == "" {
			app.showMessageSafe("Please select a storage volume.")

			return
		}

		dev := deviceMap[selectedDevice]
		if dev == nil {
			app.showMessageSafe("Invalid storage device selected.")

			return
		}
		// Format size string for Proxmox (e.g., '+10G')
		sizeStr := fmt.Sprintf("+%dG", amount)
		go func() {
			err := app.client.ResizeVMStorage(vm, dev.Device, sizeStr)
			app.QueueUpdateDraw(func() {
				if err != nil {
					app.header.ShowError(fmt.Sprintf("Resize failed: %v", err))
				} else {
					app.header.ShowSuccess("Resize operation started successfully.")
					// Remove the modal first
					if err := app.pages.RemovePage("resizeStorage"); err != nil {
						models.GetUILogger().Error("Failed to remove resizeStorage page: %v", err)
					}
					// Add a delay to allow Proxmox API to update the config data
					// This matches the pattern used in other VM operations
					go func() {
						time.Sleep(2 * time.Second)

						// Refresh the specific VM data and tasks to show updated volume size and resize task
						app.refreshVMDataAndTasks(vm)
					}()
				}
			})
		}()
	})
	modal.AddButton("Cancel", func() {
		if err := app.pages.RemovePage("resizeStorage"); err != nil {
			models.GetUILogger().Error("Failed to remove resizeStorage page: %v", err)
		}
	})
	modal.SetBorder(true).SetTitle("Resize Storage Volume").SetTitleColor(theme.Colors.Primary)
	// Set ESC key to cancel for resize modal
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if err := app.pages.RemovePage("resizeStorage"); err != nil {
				models.GetUILogger().Error("Failed to remove resizeStorage page: %v", err)
			}

			return nil
		}

		return event
	})
	app.pages.AddPage("resizeStorage", modal, true, true)
	app.SetFocus(modal)
}
