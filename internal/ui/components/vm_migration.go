package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// showMigrationDialog displays a dialog for configuring VM migration.
func (a *App) showMigrationDialog(vm *api.VM) {
	if vm == nil {
		a.showMessage("No VM selected")

		return
	}

	// * Check if VM has pending operations
	if isPending, pendingOperation := models.GlobalState.IsVMPending(vm); isPending {
		a.showMessageSafe(fmt.Sprintf("Cannot migrate VM while '%s' is in progress", pendingOperation))
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
	form.SetTitleColor(theme.Colors.Primary)
	form.SetBorderColor(theme.Colors.Border)

	// Target node dropdown
	nodeOptions := make([]string, len(availableNodes))
	for i, node := range availableNodes {
		nodeOptions[i] = node.Name
	}

	selectedNodeIndex := 0
	form.AddDropDown("Target Node", nodeOptions, selectedNodeIndex, nil)

	// Show migration mode info (read-only)
	var modeInfo string
	switch vm.Type {
	case api.VMTypeLXC:
		modeInfo = "Mode: restart"
	case api.VMTypeQemu:
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
		// GetCurrentOption() doesn't return an error, so we can ignore the errcheck warning
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
			switch vm.Type {
			case api.VMTypeLXC:
				// LXC migration is always "restart" style by default - no parameters needed
				// LXC containers don't support live migration
			case api.VMTypeQemu:
				// QEMU: online for running VMs, offline for stopped VMs
				online := vm.Status == api.VMStatusRunning
				options.Online = &online
			}

			// Close dialog and perform migration
			a.removePageIfPresent("migration")

			a.performMigrationOperation(vm, options)
		})
	})

	form.AddButton("Cancel", func() {
		a.removePageIfPresent("migration")
	})

	// Set up input capture for navigation
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.removePageIfPresent("migration")
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

// performMigrationOperation performs an asynchronous VM migration operation.
func (a *App) performMigrationOperation(vm *api.VM, options *api.MigrationOptions) {
	// Set pending state immediately for visual feedback
	const (
		migrationTypeOffline = "offline"
		migrationTypeRestart = "restart"
		migrationTypeOnline  = "online"
	)

	migrationTypeStr := migrationTypeOffline
	if vm.Type == api.VMTypeLXC {
		migrationTypeStr = migrationTypeRestart
	} else if options.Online != nil && *options.Online {
		migrationTypeStr = migrationTypeOnline
	}

	models.GlobalState.SetVMPending(vm, "Migrating - will move to new node")

	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("Migrating %s to %s (%s)", vm.Name, options.Target, migrationTypeStr))

	// Show visual feedback with small delay to avoid UI deadlock
	go func() {
		time.Sleep(50 * time.Millisecond)
		a.QueueUpdateDraw(func() {
			a.updateVMListWithSelectionPreservation()
		})
	}()

	// originalUptime no longer needed

	// Run operation in goroutine to avoid blocking UI
	go func() {
		defer func() {
			// * Clear pending state after final refresh to ensure clean state
			// Note: For migration, we clear pending state after the final refresh
			// because the VM still exists (just on a different node)
		}()

		if err := a.client.MigrateVM(vm, options); err != nil {
			// Update message with detailed error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Migration failed: %v", err))
				// Also show a modal with more details
				a.showMessage(fmt.Sprintf("Migration of %s '%s' (ID: %d) to %s failed:\n\n%v\n\nCheck the logs for more details.",
					strings.ToUpper(vm.Type), vm.Name, vm.ID, options.Target, err))
			})

			return
		}

		// Migration started successfully
		// Now poll for migration completion
		maxWaitTime := 5 * time.Minute
		checkInterval := 3 * time.Second
		startTime := time.Now()
		migrationComplete := false

		for time.Since(startTime) < maxWaitTime {
			migratedVM := &api.VM{ID: vm.ID, Node: options.Target, Type: vm.Type}
			freshVM, err := a.client.RefreshVMData(migratedVM, nil)

			if err == nil && freshVM != nil {
				migratedVM = freshVM
			}

			if migratedVM != nil {
				if vm.Type == api.VMTypeLXC || (vm.Type == api.VMTypeQemu && (options.Online == nil || !*options.Online)) {
					// LXC or offline QEMU: consider migration complete as soon as uptime is > 0
					if migratedVM.Uptime > 0 {
						migrationComplete = true

						break
					}
				} else if vm.Type == api.VMTypeQemu && options.Online != nil && *options.Online {
					// Online QEMU: wait for status to be running
					if migratedVM.Status == api.VMStatusRunning {
						migrationComplete = true

						break
					}
				}
			}

			time.Sleep(checkInterval)
		}

		if migrationComplete {
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Migration of %s to %s completed successfully", vm.Name, options.Target))
			})
		} else {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Migration of %s to %s timed out", vm.Name, options.Target))
			})
		}

		// Clear API cache to ensure fresh data is loaded
		a.client.ClearAPICache()

		// Final refresh after migration
		a.QueueUpdateDraw(func() {
			a.manualRefresh() // Refresh all data to show updated VM location and tasks
		})

		// * Clear pending state after the final refresh completes
		// This ensures the VM remains in pending state until the refresh shows the updated location
		go func() {
			time.Sleep(6 * time.Second) // Wait for manualRefresh to complete
			a.QueueUpdateDraw(func() {
				models.GlobalState.ClearVMPending(vm)
				a.updateVMListWithSelectionPreservation()
			})
		}()
	}()
}
