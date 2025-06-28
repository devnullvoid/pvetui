package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showMessage displays a message to the user
func (a *App) showMessage(message string) {
	modal := tview.NewModal().
		SetText(message).
		// SetBackgroundColor(tcell.ColorGray).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("message")
		})

	a.pages.AddPage("message", modal, false, true)
}

// showConfirmationDialog displays a confirmation dialog with Yes/No options
func (a *App) showConfirmationDialog(message string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(message).
		// SetBackgroundColor(tcell.ColorGray).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("confirmation")
			if buttonIndex == 0 {
				// Yes was selected
				onConfirm()
			}
		})

	a.pages.AddPage("confirmation", modal, false, true)
}

// openScriptSelector opens the script selector dialog
func (a *App) openScriptSelector(node *api.Node, vm *api.VM) {
	if a.config.SSHUser == "" {
		a.showMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")
		return
	}

	selector := NewScriptSelector(a, node, vm, a.config.SSHUser)
	selector.Show()
}

// showMigrationDialog displays a dialog for configuring VM migration
func (a *App) showMigrationDialog(vm *api.VM) {
	if vm == nil {
		a.showMessage("No VM selected")
		return
	}

	// Get available nodes (excluding current node)
	var availableNodes []*api.Node
	if a.client.Cluster != nil {
		for _, node := range a.client.Cluster.Nodes {
			if node != nil && node.Name != vm.Node && node.Online {
				availableNodes = append(availableNodes, node)
			}
		}
	}

	if len(availableNodes) == 0 {
		a.showMessage("No other online nodes available for migration")
		return
	}

	// Create form
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" Migrate %s '%s' (ID: %d) ", strings.ToUpper(vm.Type), vm.Name, vm.ID))
	form.SetTitleColor(tcell.ColorYellow)
	form.SetBorderColor(tcell.ColorYellow)

	// Target node dropdown
	nodeOptions := make([]string, len(availableNodes))
	for i, node := range availableNodes {
		nodeOptions[i] = node.Name
	}
	selectedNodeIndex := 0
	form.AddDropDown("Target Node", nodeOptions, selectedNodeIndex, nil)

	// Show migration mode info (read-only)
	var modeInfo string
	if vm.Type == api.VMTypeLXC {
		modeInfo = "Mode: restart"
	} else if vm.Type == api.VMTypeQemu {
		if vm.Status == api.VMStatusRunning {
			modeInfo = "Mode: online"
		} else {
			modeInfo = "Mode: offline"
		}
	}

	// Add info text (using a disabled input field for display)
	infoField := tview.NewInputField()
	infoField.SetLabel("Migration Mode")
	infoField.SetText(modeInfo)
	infoField.SetDisabled(true)
	form.AddFormItem(infoField)

	// Add buttons
	form.AddButton("Migrate", func() {
		// Get form values
		_, targetNode := form.GetFormItemByLabel("Target Node").(*tview.DropDown).GetCurrentOption()

		// Show confirmation dialog
		confirmText := fmt.Sprintf("Migrate %s '%s' (ID: %d) from %s to %s?\n\n%s",
			strings.ToUpper(vm.Type), vm.Name, vm.ID, vm.Node, targetNode, modeInfo)

		a.showConfirmationDialog(confirmText, func() {
			// Build migration options with smart defaults
			options := &api.MigrationOptions{
				Target: targetNode,
			}

			// Set mode based on VM type and status
			if vm.Type == api.VMTypeLXC {
				// LXC migration is always "restart" style by default - no parameters needed
				// LXC containers don't support live migration
			} else if vm.Type == api.VMTypeQemu {
				// QEMU: online for running VMs, offline for stopped VMs
				online := vm.Status == api.VMStatusRunning
				options.Online = &online
			}

			// Close dialog and perform migration
			a.pages.RemovePage("migration")
			a.performMigrationOperation(vm, options)
		})
	})

	form.AddButton("Cancel", func() {
		a.pages.RemovePage("migration")
	})

	// Set up input capture for navigation
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage("migration")
			return nil
		}
		return event
	})

	// Create centered modal layout with minimum height
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 12, 0, true). // Set minimum height of 12 lines for the form
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("migration", modal, true, true)
	a.SetFocus(form)
}

// performMigrationOperation performs an asynchronous VM migration operation
func (a *App) performMigrationOperation(vm *api.VM, options *api.MigrationOptions) {
	// Show loading indicator
	migrationTypeStr := "offline"
	if vm.Type == api.VMTypeLXC {
		migrationTypeStr = "restart"
	} else if options.Online != nil && *options.Online {
		migrationTypeStr = "online"
	}

	a.header.ShowLoading(fmt.Sprintf("Migrating %s to %s (%s)", vm.Name, options.Target, migrationTypeStr))

	// Run operation in goroutine to avoid blocking UI
	go func() {
		if err := a.client.MigrateVM(vm, options); err != nil {
			// Update message with detailed error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Migration failed: %v", err))
				// Also show a modal with more details
				a.showMessage(fmt.Sprintf("Migration of %s '%s' (ID: %d) to %s failed:\n\n%v\n\nCheck the logs for more details.",
					strings.ToUpper(vm.Type), vm.Name, vm.ID, options.Target, err))
			})
		} else {
			// Update message with success on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Migration of %s to %s started successfully", vm.Name, options.Target))
			})

			// Clear API cache to ensure fresh data is loaded
			a.client.ClearAPICache()

			// Migration is an async operation, so we refresh after a delay to show task progress
			// The actual migration status can be monitored in the Tasks tab
			go func() {
				// Wait longer for migration to be reflected in the task list and VM location
				// Migrations can take time to complete, especially for larger VMs
				time.Sleep(5 * time.Second)
				a.QueueUpdateDraw(func() {
					a.manualRefresh() // Refresh all data to show updated VM location and tasks
				})
			}()
		}
	}()
}
