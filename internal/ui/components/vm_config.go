package components

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api"
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
	form.SetLabelColor(theme.Colors.HeaderText)
	page := &VMConfigPage{
		Form:   form,
		app:    app,
		vm:     vm,
		config: config,
		saveFn: saveFn,
	}

	// Add Resize Storage Volume button as a FormButton at the top (left-aligned)
	resizeBtn := NewFormButton("Resize Storage Volume", func() {
		// * Check if VM has pending operations
		if isPending, pendingOperation := models.GlobalState.IsVMPending(vm); isPending {
			app.showMessageSafe(fmt.Sprintf("Cannot resize storage while '%s' is in progress", pendingOperation))
			return
		}
		showResizeStorageModal(app, vm)
	}).SetAlignment(AlignLeft)
	form.AddFormItem(resizeBtn)

	// Add Name/Hostname field
	if vm.Type == api.VMTypeQemu {
		// For QEMU VMs, use the "name" field
		initialName := vm.Name
		if config.Name != "" {
			initialName = config.Name
		}
		form.AddInputField("Name", initialName, 20, func(textToCheck string, lastChar rune) bool {
			// Validate hostname characters: letters, digits, hyphens only
			// Hostnames cannot start or end with hyphens, and cannot contain underscores
			return isValidHostnameChar(lastChar)
		}, func(text string) {
			page.config.Name = text
			// Update title in real-time
			title := fmt.Sprintf("Edit Configuration: VM %d - %s", vm.ID, text)
			form.SetTitle(title)
		})
	} else if vm.Type == api.VMTypeLXC {
		// For LXC containers, use the "hostname" field
		initialHostname := vm.Name
		if config.Hostname != "" {
			initialHostname = config.Hostname
		}
		form.AddInputField("Hostname", initialHostname, 20, func(textToCheck string, lastChar rune) bool {
			// Validate hostname characters: letters, digits, hyphens only
			// Hostnames cannot start or end with hyphens, and cannot contain underscores
			return isValidHostnameChar(lastChar)
		}, func(text string) {
			page.config.Hostname = text
			// Update title in real-time
			title := fmt.Sprintf("Edit Configuration: CT %d - %s", vm.ID, text)
			form.SetTitle(title)
		})
	}

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

	// Tags
	initialTags := normalizeTags(config.Tags)
	form.AddInputField("Tags (semicolon-separated)", initialTags, 40, nil, func(text string) {
		page.config.Tags = normalizeTags(text)
		page.config.TagsExplicit = true
	})

	// OnBoot
	onboot := false
	if config.OnBoot != nil {
		onboot = *config.OnBoot
	}

	form.AddCheckbox("Start at boot", onboot, func(checked bool) {
		page.config.OnBoot = &checked
	})

	if vm.Type == api.VMTypeQemu {
		agentEnabled := vm.AgentEnabled
		if config.Agent != nil {
			agentEnabled = *config.Agent
		}

		form.AddCheckbox("Enable QEMU guest agent", agentEnabled, func(checked bool) {
			page.config.Agent = &checked
		})
	}
	// Save/Cancel buttons
	form.AddButton("Save", func() {
		// * Check if VM has pending operations
		if isPending, pendingOperation := models.GlobalState.IsVMPending(vm); isPending {
			app.showMessageSafe(fmt.Sprintf("Cannot save configuration while '%s' is in progress", pendingOperation))
			return
		}

		// Validate hostname format before saving
		var validationError string
		if vm.Type == api.VMTypeQemu && page.config.Name != "" {
			if !isValidHostname(page.config.Name) {
				validationError = fmt.Sprintf("Invalid VM name: %s", page.config.Name)
			}
		} else if vm.Type == api.VMTypeLXC && page.config.Hostname != "" {
			if !isValidHostname(page.config.Hostname) {
				validationError = fmt.Sprintf("Invalid hostname: %s", page.config.Hostname)
			}
		}

		if validationError != "" {
			app.header.ShowError(validationError)
			return
		}

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

					// Update the VM name in the current VM object for title update
					if vm.Type == api.VMTypeQemu && page.config.Name != "" {
						vm.Name = page.config.Name
					} else if vm.Type == api.VMTypeLXC && page.config.Hostname != "" {
						vm.Name = page.config.Hostname
					}

					// Remove the config page first
					app.removePageIfPresent("vmConfig")

					// Show loading indicator while waiting for API changes to propagate
					app.header.ShowLoading("Waiting for configuration changes to propagate...")

					// Poll Proxmox API to verify the name change has propagated
					// This is more professional than arbitrary delays
					go func() {
						// Store the expected new name
						expectedName := ""
						if vm.Type == api.VMTypeQemu && page.config.Name != "" {
							expectedName = page.config.Name
						} else if vm.Type == api.VMTypeLXC && page.config.Hostname != "" {
							expectedName = page.config.Hostname
						}

						nameChanged := expectedName != "" && expectedName != vm.Name
						tagsChanged := page.config.TagsExplicit && page.config.Tags != vm.Tags

						// Use the dedicated polling function
						app.pollForConfigChange(vm, expectedName, nameChanged, page.config.Tags, tagsChanged)
					}()
				}
			})
		}()
	})
	form.AddButton("Cancel", func() {
		app.removePageIfPresent("vmConfig")
	})
	// Set dynamic title with guest info
	guestType := "VM"
	if vm.Type == api.VMTypeLXC {
		guestType = "CT"
	}

	// Use the current name from config if available, otherwise use VM name
	displayName := vm.Name
	if vm.Type == api.VMTypeQemu && config.Name != "" {
		displayName = config.Name
	} else if vm.Type == api.VMTypeLXC && config.Hostname != "" {
		displayName = config.Hostname
	}

	title := fmt.Sprintf("Edit Configuration: %s %d - %s", guestType, vm.ID, displayName)
	form.SetBorder(true).SetTitle(title).SetTitleColor(theme.Colors.Primary)
	// Set ESC key to cancel
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			app.removePageIfPresent("vmConfig")
			return nil
		}

		return event
	})
	// // Set initial focus to the first field (Resize Storage Volume)
	// form.SetFocus(0)
	return page
}

// isValidHostnameChar validates if a character is allowed in a hostname.
// Hostnames can only contain letters (a-z, A-Z), digits (0-9), and hyphens (-).
// They cannot start or end with hyphens, and cannot contain underscores or other special characters.
func isValidHostnameChar(char rune) bool {
	// Allow letters (a-z, A-Z)
	if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
		return true
	}
	// Allow digits (0-9)
	if char >= '0' && char <= '9' {
		return true
	}
	// Allow hyphens (-)
	if char == '-' {
		return true
	}
	// Reject all other characters including underscores, spaces, etc.
	return false
}

// isValidHostname validates if a complete hostname string is valid.
// This checks the overall format, not just individual characters.
func isValidHostname(hostname string) bool {
	if hostname == "" {
		return false
	}

	// Check length (RFC 1035: max 63 characters)
	if len(hostname) > 63 {
		return false
	}

	// Check minimum length (at least 1 character)
	if len(hostname) < 1 {
		return false
	}

	// Check that it doesn't start or end with a hyphen
	if hostname[0] == '-' || hostname[len(hostname)-1] == '-' {
		return false
	}

	// Check that it contains at least one letter or digit
	hasValidChar := false
	for _, char := range hostname {
		if isValidHostnameChar(char) {
			hasValidChar = true
		} else {
			return false // Invalid character found
		}
	}

	return hasValidChar
}

func normalizeTags(raw string) string {
	tags := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, ";")
}

// showResizeStorageModal displays a modal for resizing a storage volume.
func showResizeStorageModal(app *App, vm *api.VM) {
	modal := tview.NewForm().SetHorizontal(false)
	modal.SetLabelColor(theme.Colors.HeaderText)

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
		// * Check if VM has pending operations
		if isPending, pendingOperation := models.GlobalState.IsVMPending(vm); isPending {
			app.showMessageSafe(fmt.Sprintf("Cannot resize storage while '%s' is in progress", pendingOperation))
			return
		}

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
		app.removePageIfPresent("resizeStorage")
	})
	modal.SetBorder(true).SetTitle("Resize Storage Volume").SetTitleColor(theme.Colors.Primary)
	// Set ESC key to cancel for resize modal
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			app.removePageIfPresent("resizeStorage")
			return nil
		}

		return event
	})
	app.pages.AddPage("resizeStorage", modal, true, true)
	app.SetFocus(modal)
}

// pollForConfigChange polls the Proxmox API to verify that a configuration change has propagated
// to both the config endpoint and the cluster resources endpoint before refreshing the UI.
// This prevents race conditions where config is updated but cluster resources still show old names.
func (app *App) pollForConfigChange(vm *api.VM, expectedName string, nameChanged bool, expectedTags string, tagsChanged bool) {
	client, err := app.getClientForVM(vm)
	if err != nil {
		client = app.client
	}

	// Poll every 500ms for up to 15 seconds (increased timeout for cluster resources propagation)
	maxAttempts := 30
	if !nameChanged {
		maxAttempts = 10
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		time.Sleep(500 * time.Millisecond)

		// First check if the config endpoint has the new name using the existing API function
		config, err := client.GetVMConfig(vm)
		configUpdated := true

		if err == nil && config != nil {
			if nameChanged {
				if vm.Type == api.VMTypeQemu && config.Name != expectedName {
					configUpdated = false
				} else if vm.Type == api.VMTypeLXC && config.Hostname != expectedName {
					configUpdated = false
				}
			}

			if tagsChanged && config.Tags != expectedTags {
				configUpdated = false
			}
		} else {
			configUpdated = false
		}

		// If config is updated, also check if cluster resources reflect the change when needed
		if configUpdated && nameChanged {
			// Use the existing GetVmList function to check cluster resources
			vmList, err := client.GetVmList(context.Background())
			if err == nil {
				for _, vmData := range vmList {
					if resType, exists := vmData["type"].(string); exists && resType == vm.Type {
						if nodeName, exists := vmData["node"].(string); exists && nodeName == vm.Node {
							if vmID, exists := vmData["vmid"].(float64); exists && int(vmID) == vm.ID {
								if name, exists := vmData["name"].(string); exists && name == expectedName {
									// Both config and cluster resources show the new name, we can proceed
									app.QueueUpdateDraw(func() {
										app.manualRefresh()
									})
									return
								}
								break
							}
						}
					}
				}
			}
		} else if configUpdated {
			app.QueueUpdateDraw(func() {
				app.manualRefresh()
			})
			return
		}
	}

	// If we timeout, refresh anyway and show a warning
	app.QueueUpdateDraw(func() {
		app.header.ShowWarning("Configuration change propagation timeout, refreshing anyway...")
		app.manualRefresh()
	})
}
