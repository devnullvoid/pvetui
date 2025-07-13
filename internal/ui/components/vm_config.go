package components

import (
	"fmt"
	"strconv"
	"strings"

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
	form := tview.NewForm().SetHorizontal(false)
	page := &VMConfigPage{
		Form:   form,
		app:    app,
		vm:     vm,
		config: config,
		saveFn: saveFn,
	}

	// Restore to simple vertical layout for Cores, Sockets, Memory (MB)
	form.SetHorizontal(false)
	form.AddInputField("Cores", strconv.Itoa(config.Cores), 4, nil, func(text string) {
		if v, err := strconv.Atoi(text); err == nil {
			page.config.Cores = v
		}
	})
	if vm.Type == api.VMTypeQemu {
		form.AddInputField("Sockets", strconv.Itoa(config.Sockets), 4, nil, func(text string) {
			if v, err := strconv.Atoi(text); err == nil {
				page.config.Sockets = v
			}
		})
	}
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
	// Add Resize Storage Volume button
	form.AddButton("Resize Storage Volume", func() {
		showResizeStorageModal(app, vm)
	})
	// Save/Cancel buttons
	form.AddButton("Save", func() {
		err := page.saveFn(page.config)
		if err != nil {
			app.showMessage(fmt.Sprintf("Failed to save config: %v", err))
		} else {
			app.showMessage("Configuration updated successfully.")
			app.manualRefresh()
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
	// Set ESC key to cancel
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		formItemIdx, _ := form.GetFocusedItemIndex()
		// Description field is at index 3 (after Cores, Sockets (if QEMU), Memory)
		isDescriptionField := false
		if vm.Type == api.VMTypeQemu {
			if formItemIdx == 3 {
				isDescriptionField = true
			}
		} else {
			if formItemIdx == 2 {
				isDescriptionField = true
			}
		}
		if (event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2) && isDescriptionField {
			// Let Backspace work for editing Description
			return event
		}
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			app.pages.RemovePage("vmConfig")
			return nil
		}
		return event
	})
	return page
}

// showResizeStorageModal displays a modal for resizing a storage volume.
func showResizeStorageModal(app *App, vm *api.VM) {
	modal := tview.NewForm().SetHorizontal(false)

	// Build list of storage devices (filter to only resizable volumes)
	var deviceNames []string
	var deviceMap = make(map[string]*api.StorageDevice)
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
		amountField := modal.GetFormItemByLabel("Expand by (GB)").(*tview.InputField)
		amountStr := amountField.GetText()
		amount, err := strconv.Atoi(amountStr)
		if err != nil || amount <= 0 {
			app.showMessage("Please enter a positive number of GB.")
			return
		}
		if selectedDevice == "" {
			app.showMessage("Please select a storage volume.")
			return
		}
		dev := deviceMap[selectedDevice]
		if dev == nil {
			app.showMessage("Invalid storage device selected.")
			return
		}
		// Format size string for Proxmox (e.g., '+10G')
		sizeStr := fmt.Sprintf("+%dG", amount)
		go func() {
			err := app.client.ResizeVMStorage(vm, dev.Device, sizeStr)
			app.QueueUpdateDraw(func() {
				if err != nil {
					app.showMessage(fmt.Sprintf("Resize failed: %v", err))
				} else {
					app.showMessage("Resize operation started successfully.")
					app.manualRefresh()
					app.pages.RemovePage("resizeStorage")
				}
			})
		}()
	})
	modal.AddButton("Cancel", func() {
		app.pages.RemovePage("resizeStorage")
	})
	modal.SetBorder(true).SetTitle("Resize Storage Volume").SetTitleColor(tcell.ColorYellow)
	// Set ESC key to cancel for resize modal
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			app.pages.RemovePage("resizeStorage")
			return nil
		}
		return event
	})
	app.pages.AddPage("resizeStorage", modal, true, true)
	app.SetFocus(modal)
}
