package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Constants for snapshot management
const (
	currentSnapshotName = "current"
)

// SnapshotManager manages the snapshot interface for VMs and containers.
type SnapshotManager struct {
	*tview.Flex
	vm           *api.VM
	app          *App
	snapshotList *tview.Table
	infoText     *tview.TextView
	loading      bool
	createBtn    *tview.Button
	deleteBtn    *tview.Button
	rollbackBtn  *tview.Button
	backBtn      *tview.Button
	snapshots    []api.Snapshot // Store loaded snapshots
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
			// Go back to VM list when Escape is pressed
			if key == tcell.KeyEsc {
				sm.goBack()
			}
		})

	// Create info text
	sm.infoText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true)

	// Create footer/help bar
	helpText := sm.getHelpText()
	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(helpText)

	// Create layout
	sm.Flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(sm.createHeader(), 5, 0, false). // Increased height from 3 to 5
		AddItem(sm.snapshotList, 0, 1, true).
		AddItem(sm.infoText, 3, 0, false).
		AddItem(helpBar, 1, 0, false)

	// Set up table headers
	sm.setupTableHeaders()

	// Load snapshots
	sm.loadSnapshots()

	// Set up keyboard navigation
	sm.setupKeyboardNavigation()

	return sm
}

// setupKeyboardNavigation sets up keyboard shortcuts for the snapshot manager.
func (sm *SnapshotManager) setupKeyboardNavigation() {
	sm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			sm.goBack()
			return nil
		case tcell.KeyTab:
			// Handle Tab navigation between table and buttons
			currentFocus := sm.app.GetFocus()
			if currentFocus == sm.snapshotList {
				// From table to first button
				sm.app.SetFocus(sm.createBtn)
				return nil
			} else if currentFocus == sm.createBtn {
				// From create button to delete button
				sm.app.SetFocus(sm.deleteBtn)
				return nil
			} else if currentFocus == sm.deleteBtn {
				// From delete button to rollback button
				sm.app.SetFocus(sm.rollbackBtn)
				return nil
			} else if currentFocus == sm.rollbackBtn {
				// From rollback button to back button
				sm.app.SetFocus(sm.backBtn)
				return nil
			} else if currentFocus == sm.backBtn {
				// From back button back to table
				sm.app.SetFocus(sm.snapshotList)
				return nil
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'c', 'C':
				sm.createSnapshot()
				return nil
			case 'd', 'D':
				sm.deleteSnapshot()
				return nil
			case 'r', 'R':
				sm.rollbackSnapshot()
				return nil
			case 'b', 'B':
				sm.goBack()
				return nil
			}
		}
		return event
	})
}

// goBack returns to the VM list.
func (sm *SnapshotManager) goBack() {
	sm.app.pages.RemovePage("snapshots")
	sm.app.SetFocus(sm.app.vmList)
}

// createHeader creates the header for the snapshot manager.
func (sm *SnapshotManager) createHeader() *tview.Flex {
	title := tview.NewTextView().
		SetText(fmt.Sprintf("Snapshots for %s %s (ID: %d)", sm.vm.Type, sm.vm.Name, sm.vm.ID)).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Create buttons with proper styling
	sm.createBtn = tview.NewButton("Take Snapshot (C)").
		SetSelectedFunc(sm.createSnapshot)

	sm.deleteBtn = tview.NewButton("Remove (D)").
		SetSelectedFunc(sm.deleteSnapshot)

	sm.rollbackBtn = tview.NewButton("Rollback (R)").
		SetSelectedFunc(sm.rollbackSnapshot)

	sm.backBtn = tview.NewButton("Back (B)").
		SetSelectedFunc(sm.goBack)

	buttons := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(sm.createBtn, 18, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(sm.deleteBtn, 12, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(sm.rollbackBtn, 12, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(sm.backBtn, 10, 0, false).
		AddItem(tview.NewBox(), 0, 1, false)

	header := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(title, 2, 0, false).
		AddItem(buttons, 2, 0, false)

	return header
}

// setupTableHeaders sets up the table headers.
func (sm *SnapshotManager) setupTableHeaders() {
	var headers []string
	var colors []tcell.Color

	if sm.vm.Type == api.VMTypeQemu {
		headers = []string{"Name", "RAM", "Date/Status", "Description"}
		colors = []tcell.Color{theme.Colors.HeaderText, theme.Colors.HeaderText, theme.Colors.HeaderText, theme.Colors.HeaderText}
	} else {
		headers = []string{"Name", "Date/Status", "Description"}
		colors = []tcell.Color{theme.Colors.HeaderText, theme.Colors.HeaderText, theme.Colors.HeaderText}
	}

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
	// Store snapshots for later access
	sm.snapshots = snapshots

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

		// Handle "current" as "NOW" like the web UI
		displayName := snapshot.Name
		if snapshot.Name == currentSnapshotName {
			displayName = "NOW"
		}

		// Use different color for "NOW" to indicate it's not a real snapshot
		nameColor := theme.Colors.Primary
		if snapshot.Name == currentSnapshotName {
			nameColor = theme.Colors.Secondary // Dimmed color for current state
		}

		sm.snapshotList.SetCell(row, 0, tview.NewTableCell(displayName).SetTextColor(nameColor))

		// Handle different column layouts for QEMU vs LXC
		if sm.vm.Type == api.VMTypeQemu {
			// QEMU: Name, RAM, Date/Status, Description
			ramText := ""
			if snapshot.VMState {
				ramText = "Yes"
			}
			sm.snapshotList.SetCell(row, 1, tview.NewTableCell(ramText).SetTextColor(theme.Colors.Primary))

			dateText := ""
			if !snapshot.SnapTime.IsZero() {
				dateText = snapshot.SnapTime.Format("2006-01-02 15:04:05")
			}
			sm.snapshotList.SetCell(row, 2, tview.NewTableCell(dateText).SetTextColor(theme.Colors.Primary))

			sm.snapshotList.SetCell(row, 3, tview.NewTableCell(snapshot.Description).SetTextColor(theme.Colors.Primary))
		} else {
			// LXC: Name, Date/Status, Description
			dateText := ""
			if !snapshot.SnapTime.IsZero() {
				dateText = snapshot.SnapTime.Format("2006-01-02 15:04:05")
			}
			sm.snapshotList.SetCell(row, 1, tview.NewTableCell(dateText).SetTextColor(theme.Colors.Primary))

			sm.snapshotList.SetCell(row, 2, tview.NewTableCell(snapshot.Description).SetTextColor(theme.Colors.Primary))
		}
	}

	// Count real snapshots (excluding "current")
	realSnapshotCount := 0
	for _, snapshot := range snapshots {
		if snapshot.Name != currentSnapshotName {
			realSnapshotCount++
		}
	}

	sm.updateInfoText(fmt.Sprintf("✅ Loaded %d snapshots", realSnapshotCount))
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

	// Get the snapshot name from the first column
	nameCell := sm.snapshotList.GetCell(row, 0)
	if nameCell == nil {
		return nil
	}

	// Convert "NOW" back to "current" for API calls
	snapshotName := nameCell.Text
	if snapshotName == "NOW" {
		snapshotName = currentSnapshotName
	}

	// Find the snapshot in our list
	for _, snapshot := range sm.snapshots {
		if snapshot.Name == snapshotName {
			return &snapshot
		}
	}

	return nil
}

// createSnapshot shows the create snapshot dialog.
func (sm *SnapshotManager) createSnapshot() {
	// Create form items first
	nameField := tview.NewInputField().SetLabel("Snapshot Name").SetFieldWidth(20)
	descField := tview.NewInputField().SetLabel("Description").SetFieldWidth(40)

	form := tview.NewForm().
		AddFormItem(nameField).
		AddFormItem(descField)

	// Only show VM State for QEMU guests
	var vmStateCheck *tview.Checkbox
	if sm.vm.Type == api.VMTypeQemu {
		vmStateCheck = tview.NewCheckbox().SetLabel("Include VM State (RAM)").SetChecked(true)
		form.AddFormItem(vmStateCheck)
	}

	form.AddButton("Create", func() {
		name := nameField.GetText()
		description := descField.GetText()
		vmState := false
		if sm.vm.Type == api.VMTypeQemu && vmStateCheck != nil {
			vmState = vmStateCheck.IsChecked()
		}

		if name == "" {
			sm.app.showMessage("❌ Snapshot name is required")
			return
		}

		options := &api.SnapshotOptions{
			Description: description,
			VMState:     vmState,
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

	// Add keyboard input capture for Escape key
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			sm.app.pages.RemovePage("createSnapshot")
			sm.app.SetFocus(sm)
			return nil
		}
		return event
	})

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
	snapshot := sm.getSelectedSnapshot()
	if snapshot == nil {
		sm.updateInfoText("❌ No snapshot selected.")
		return
	}

	// Prevent deleting "current" (NOW) as it's not a real snapshot
	if snapshot.Name == currentSnapshotName {
		sm.updateInfoText("❌ Cannot delete current state (NOW).")
		return
	}

	sm.performSnapshotOperation(
		"Delete",
		fmt.Sprintf("❌ Failed to delete snapshot %s", snapshot.Name),
		fmt.Sprintf("✅ Successfully deleted snapshot %s", snapshot.Name),
		func() error {
			return sm.app.client.DeleteSnapshot(sm.vm, snapshot.Name)
		},
	)
}

// rollbackSnapshot rolls back to the selected snapshot.
func (sm *SnapshotManager) rollbackSnapshot() {
	snapshot := sm.getSelectedSnapshot()
	if snapshot == nil {
		sm.updateInfoText("❌ No snapshot selected.")
		return
	}

	// Prevent rolling back to "current" (NOW) as it's not a real snapshot
	if snapshot.Name == currentSnapshotName {
		sm.updateInfoText("❌ Cannot rollback to current state (NOW).")
		return
	}

	sm.performSnapshotOperation(
		"Rollback",
		fmt.Sprintf("❌ Failed to rollback to snapshot %s", snapshot.Name),
		fmt.Sprintf("✅ Successfully rolled back to snapshot %s", snapshot.Name),
		func() error {
			return sm.app.client.RollbackToSnapshot(sm.vm, snapshot.Name)
		},
	)
}

// getHelpText returns the help/footer text for the snapshot manager.
func (sm *SnapshotManager) getHelpText() string {
	if sm.vm.Type == api.VMTypeQemu {
		return theme.ReplaceSemanticTags("[info]C[-]reate  [info]D[-]elete  [info]R[-]ollback  [info]B[-]ack  [info]↑/↓[-] Navigate  [info]Enter[-] Select")
	}
	return theme.ReplaceSemanticTags("[info]C[-]reate  [info]D[-]elete  [info]R[-]ollback  [info]B[-]ack  [info]↑/↓[-] Navigate  [info]Enter[-] Select")
}
