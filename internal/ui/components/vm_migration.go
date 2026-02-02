package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/taskmanager"
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// showMigrationDialog displays a dialog for configuring VM migration.
func (a *App) showMigrationDialog(vm *api.VM) {
	if vm == nil {
		a.showMessageSafe("No VM selected")

		return
	}

	// * Check if VM has pending operations
	if isPending, pendingOperation := models.GlobalState.IsVMPending(vm); isPending {
		a.showMessageSafe(fmt.Sprintf("Cannot migrate VM while '%s' is in progress", pendingOperation))
		return
	}

	// Get client for VM to fetch nodes from the same cluster
	client, err := a.getClientForVM(vm)
	if err != nil {
		a.showMessageSafe(fmt.Sprintf("Error determining VM cluster: %v", err))
		return
	}

	// Get available nodes (excluding current node) from the same cluster as the VM
	// In group mode, only nodes from the VM's source profile/cluster are considered.
	var availableNodes []*api.Node

	if client.Cluster != nil {
		for _, node := range client.Cluster.Nodes {
			if node != nil && node.Name != vm.Node && node.Online {
				availableNodes = append(availableNodes, node)
			}
		}
	}

	if len(availableNodes) == 0 {
		a.showMessageSafe("No other online nodes available for migration")

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

// performMigrationOperation performs an asynchronous VM migration operation via the TaskManager.
func (a *App) performMigrationOperation(vm *api.VM, options *api.MigrationOptions) {
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

	task := &taskmanager.Task{
		Type:        "Migrate",
		Description: fmt.Sprintf("Migrate %s (%d) to %s (%s)", vm.Name, vm.ID, options.Target, migrationTypeStr),
		TargetVMID:  vm.ID,
		TargetNode:  vm.Node,
		TargetName:  vm.Name,
		Operation: func() (string, error) {
			client, err := a.getClientForVM(vm)
			if err != nil {
				return "", err
			}
			return client.MigrateVM(vm, options)
		},
		OnComplete: func(err error) {
			a.QueueUpdateDraw(func() {
				if err != nil {
					a.header.ShowError(fmt.Sprintf("Migration error: %v", err))
					a.showMessage(fmt.Sprintf("Migration of %s '%s' (ID: %d) to %s failed:\n\n%v",
						strings.ToUpper(vm.Type), vm.Name, vm.ID, options.Target, err))
				} else {
					a.header.ShowSuccess(fmt.Sprintf("Migration of %s to %s completed successfully", vm.Name, options.Target))

					// Clear API cache for the specific client
					client, _ := a.getClientForVM(vm)
					if client != nil {
						client.ClearAPICache()
					}

					// Full refresh needed as node changed
					a.manualRefresh()
				}
			})
		},
	}

	a.taskManager.Enqueue(task)
	a.header.ShowSuccess(fmt.Sprintf("Queued migration for %s", vm.Name))
}
