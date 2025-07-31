package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SnapshotManager manages the snapshot interface for VMs and containers.
type SnapshotManager struct {
	*tview.Flex
	vm           *api.VM
	app          *App
	snapshotList *tview.Table
	infoText     *tview.TextView
	loading      bool
}

// NewSnapshotManager creates a new snapshot manager for the given VM.
func NewSnapshotManager(app *App, vm *api.VM) *SnapshotManager {
	sm := &SnapshotManager{
		vm:  vm,
		app: app,
	}

	// Create snapshot list table
	sm.snapshotList = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetDoneFunc(func(key tcell.Key) {
			app.SetFocus(app.vmList)
		})

	// Create info text
	sm.infoText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true)

	// Create layout
	sm.Flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(sm.createHeader(), 3, 0, false).
		AddItem(sm.snapshotList, 0, 1, true).
		AddItem(sm.infoText, 5, 0, false)

	// Set up table headers
	sm.setupTableHeaders()

	// Load snapshots
	sm.loadSnapshots()

	return sm
}

// createHeader creates the header for the snapshot manager.
func (sm *SnapshotManager) createHeader() *tview.Flex {
	title := tview.NewTextView().
		SetText(fmt.Sprintf("Snapshots for %s %s (ID: %d)", sm.vm.Type, sm.vm.Name, sm.vm.ID)).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	buttons := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(sm.createButton("Create", sm.createSnapshot), 12, 0, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(sm.createButton("Delete", sm.deleteSnapshot), 12, 0, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(sm.createButton("Rollback", sm.rollbackSnapshot), 12, 0, false).
		AddItem(tview.NewBox(), 0, 1, false)

	header := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(title, 1, 0, false).
		AddItem(buttons, 1, 0, false)

	return header
}

// createButton creates a button for the header.
func (sm *SnapshotManager) createButton(text string, action func()) *tview.Box {
	button := tview.NewButton(text).
		SetSelectedFunc(action).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	return button
}

// setupTableHeaders sets up the table headers.
func (sm *SnapshotManager) setupTableHeaders() {
	headers := []string{"Name", "Description", "Created", "Size", "Type", "Children"}
	colors := []tcell.Color{tcell.ColorYellow, tcell.ColorYellow, tcell.ColorYellow, tcell.ColorYellow, tcell.ColorYellow, tcell.ColorYellow}

	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(colors[i]).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1)
		sm.snapshotList.SetCell(0, i, cell)
	}
}

// loadSnapshots loads and displays snapshots for the VM.
func (sm *SnapshotManager) loadSnapshots() {
	sm.loading = true
	sm.updateInfoText("Loading snapshots...")

	go func() {
		snapshots, err := sm.app.client.GetSnapshots(sm.vm)
		sm.app.QueueUpdateDraw(func() {
			sm.loading = false
			if err != nil {
				sm.updateInfoText(fmt.Sprintf("❌ Error loading snapshots: %v", err))
				return
			}

			sm.displaySnapshots(snapshots)
		})
	}()
}

// displaySnapshots displays the snapshots in the table.
func (sm *SnapshotManager) displaySnapshots(snapshots []api.Snapshot) {
	// Clear existing rows (keep headers)
	for row := 1; row < sm.snapshotList.GetRowCount(); row++ {
		for col := 0; col < sm.snapshotList.GetColumnCount(); col++ {
			sm.snapshotList.SetCell(row, col, nil)
		}
	}

	if len(snapshots) == 0 {
		sm.updateInfoText("No snapshots found for this VM.")
		return
	}

	// Add snapshot rows
	for i, snapshot := range snapshots {
		row := i + 1
		sm.snapshotList.SetCell(row, 0, tview.NewTableCell(snapshot.Name).SetTextColor(tcell.ColorWhite))
		sm.snapshotList.SetCell(row, 1, tview.NewTableCell(snapshot.Description).SetTextColor(tcell.ColorWhite))
		sm.snapshotList.SetCell(row, 2, tview.NewTableCell(snapshot.Timestamp.Format("2006-01-02 15:04:05")).SetTextColor(tcell.ColorWhite))
		sm.snapshotList.SetCell(row, 3, tview.NewTableCell(api.FormatBytes(snapshot.Size)).SetTextColor(tcell.ColorWhite))

		// Show snapshot type
		typeText := ""
		if snapshot.VMState {
			typeText += "VM"
		}
		if snapshot.Config {
			if typeText != "" {
				typeText += "+"
			}
			typeText += "Config"
		}
		if snapshot.Disk {
			if typeText != "" {
				typeText += "+"
			}
			typeText += "Disk"
		}
		sm.snapshotList.SetCell(row, 4, tview.NewTableCell(typeText).SetTextColor(tcell.ColorWhite))

		// Show children count
		childrenText := fmt.Sprintf("%d", len(snapshot.Children))
		sm.snapshotList.SetCell(row, 5, tview.NewTableCell(childrenText).SetTextColor(tcell.ColorWhite))
	}

	sm.updateInfoText(fmt.Sprintf("✅ Loaded %d snapshots", len(snapshots)))
}

// updateInfoText updates the info text at the bottom.
func (sm *SnapshotManager) updateInfoText(text string) {
	sm.infoText.SetText(text)
}

// getSelectedSnapshot gets the currently selected snapshot.
func (sm *SnapshotManager) getSelectedSnapshot() *api.Snapshot {
	row, _ := sm.snapshotList.GetSelection()
	if row <= 0 || row >= sm.snapshotList.GetRowCount() {
		return nil
	}

	// Get snapshot name from first column
	cell := sm.snapshotList.GetCell(row, 0)
	if cell == nil {
		return nil
	}

	snapshotName := cell.Text
	// Find the snapshot in the loaded data
	// Note: This is a simplified approach. In a real implementation,
	// you'd want to store the snapshots in the struct for better access.
	return &api.Snapshot{Name: snapshotName}
}

// createSnapshot shows the create snapshot dialog.
func (sm *SnapshotManager) createSnapshot() {
	// Create form items first
	nameField := tview.NewInputField().SetLabel("Snapshot Name").SetFieldWidth(20)
	descField := tview.NewInputField().SetLabel("Description").SetFieldWidth(40)
	vmStateCheck := tview.NewCheckbox().SetLabel("Include VM State").SetChecked(true)
	configCheck := tview.NewCheckbox().SetLabel("Include Configuration").SetChecked(true)
	diskCheck := tview.NewCheckbox().SetLabel("Include Disk State").SetChecked(true)

	// Create the form
	form := tview.NewForm().
		AddFormItem(nameField).
		AddFormItem(descField).
		AddFormItem(vmStateCheck).
		AddFormItem(configCheck).
		AddFormItem(diskCheck).
		AddButton("Create", func() {
			name := nameField.GetText()
			description := descField.GetText()
			vmState := vmStateCheck.IsChecked()
			config := configCheck.IsChecked()
			disk := diskCheck.IsChecked()

			if name == "" {
				sm.app.showMessage("❌ Snapshot name is required")
				return
			}

			options := &api.SnapshotOptions{
				Description: description,
				VMState:     vmState,
				Config:      config,
				Disk:        disk,
			}

			sm.app.pages.RemovePage("createSnapshot")
			sm.app.SetFocus(sm)

			go func() {
				err := sm.app.client.CreateSnapshot(sm.vm, name, options)
				sm.app.QueueUpdateDraw(func() {
					if err != nil {
						sm.app.showMessage(fmt.Sprintf("❌ Failed to create snapshot: %v", err))
					} else {
						sm.app.showMessage("✅ Snapshot created successfully")
						sm.loadSnapshots() // Reload snapshots
					}
				})
			}()
		}).
		AddButton("Cancel", func() {
			sm.app.pages.RemovePage("createSnapshot")
			sm.app.SetFocus(sm)
		})

	form.SetBorder(true).SetTitle(" Create Snapshot ").SetTitleAlign(tview.AlignCenter)
	sm.app.pages.AddPage("createSnapshot", form, true, true)
	sm.app.SetFocus(form)
}

// performSnapshotOperation performs a snapshot operation with confirmation and error handling.
func (sm *SnapshotManager) performSnapshotOperation(
	operationName string,
	errorMessage string,
	successMessage string,
	operation func() error,
) {
	snapshot := sm.getSelectedSnapshot()
	if snapshot == nil {
		sm.app.showMessage(fmt.Sprintf("❌ Please select a snapshot to %s", operationName))
		return
	}

	sm.app.showConfirmationDialog(
		fmt.Sprintf("Are you sure you want to %s snapshot '%s'?\n\nThis action cannot be undone.", operationName, snapshot.Name),
		func() {
			go func() {
				err := operation()
				sm.app.QueueUpdateDraw(func() {
					if err != nil {
						sm.app.showMessage(fmt.Sprintf("❌ %s: %v", errorMessage, err))
					} else {
						sm.app.showMessage(fmt.Sprintf("✅ %s", successMessage))
						sm.loadSnapshots() // Reload snapshots
					}
				})
			}()
		},
	)
}

// deleteSnapshot deletes the selected snapshot.
func (sm *SnapshotManager) deleteSnapshot() {
	sm.performSnapshotOperation(
		"delete",
		"Failed to delete snapshot",
		"Snapshot deleted successfully",
		func() error {
			snapshot := sm.getSelectedSnapshot()
			return sm.app.client.DeleteSnapshot(sm.vm, snapshot.Name)
		},
	)
}

// rollbackSnapshot rolls back to the selected snapshot.
func (sm *SnapshotManager) rollbackSnapshot() {
	sm.performSnapshotOperation(
		"rollback to",
		"Failed to rollback to snapshot",
		"Rollback completed successfully",
		func() error {
			snapshot := sm.getSelectedSnapshot()
			return sm.app.client.RollbackToSnapshot(sm.vm, snapshot.Name)
		},
	)
}
