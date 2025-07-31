package components

import (
	"fmt"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	CurrentSnapshotName = "current"
)

// SnapshotManager manages the snapshot interface for VMs and containers.
type SnapshotManager struct {
	*tview.Flex
	vm            *api.VM
	app           *App
	snapshotTable *SnapshotTable
	infoText      *tview.TextView
	loading       bool
	createBtn     *tview.Button
	deleteBtn     *tview.Button
	rollbackBtn   *tview.Button
	backBtn       *tview.Button
	operations    *SnapshotOperations
	form          *SnapshotForm
}

// NewSnapshotManager creates a new snapshot manager for the given VM.
func NewSnapshotManager(app *App, vm *api.VM) *SnapshotManager {
	sm := &SnapshotManager{
		vm:  vm,
		app: app,
	}

	// Create components
	sm.snapshotTable = NewSnapshotTable(app, vm)
	sm.operations = NewSnapshotOperations(app, vm)
	sm.form = NewSnapshotForm(app, vm)

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
		AddItem(sm.createHeader(), 3, 0, false).
		AddItem(sm.snapshotTable, 0, 1, true).
		AddItem(sm.infoText, 3, 0, false).
		AddItem(helpBar, 1, 0, false)

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
			if currentFocus == sm.snapshotTable {
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
				sm.app.SetFocus(sm.snapshotTable)
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

// goBack returns to the previous screen.
func (sm *SnapshotManager) goBack() {
	sm.app.pages.RemovePage("snapshots")
	sm.app.SetFocus(sm.app.vmList)
}

// createHeader creates the header with title and buttons.
func (sm *SnapshotManager) createHeader() *tview.Flex {
	title := tview.NewTextView().
		SetText(fmt.Sprintf("Snapshots for %s %s (ID: %d)", sm.vm.Type, sm.vm.Name, sm.vm.ID)).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Create buttons with proper styling
	sm.createBtn = tview.NewButton("Take Snapshot (C)").
		SetSelectedFunc(sm.createSnapshot)

	sm.deleteBtn = tview.NewButton("Delete (D)").
		SetSelectedFunc(sm.deleteSnapshot)

	sm.rollbackBtn = tview.NewButton("Rollback (R)").
		SetSelectedFunc(sm.rollbackSnapshot)

	sm.backBtn = tview.NewButton("Back (B)").
		SetSelectedFunc(sm.goBack)

	buttons := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(sm.createBtn, 20, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(sm.deleteBtn, 15, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(sm.rollbackBtn, 15, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(sm.backBtn, 12, 0, false).
		AddItem(tview.NewBox(), 0, 1, false)

	header := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(title, 2, 0, false).
		AddItem(buttons, 1, 0, false)

	return header
}

// loadSnapshots loads and displays snapshots for the VM.
func (sm *SnapshotManager) loadSnapshots() {
	sm.loading = true
	sm.updateInfoText("Loading snapshots...")

	go func() {
		snapshots, err := sm.operations.GetSnapshots()
		sm.app.Application.QueueUpdateDraw(func() {
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
	sm.snapshotTable.DisplaySnapshots(snapshots)

	if len(snapshots) == 0 {
		sm.updateInfoText("No snapshots found for this VM.")
		return
	}

	// Count real snapshots (excluding "current")
	realSnapshotCount := sm.snapshotTable.GetSnapshotCount()
	sm.updateInfoText(fmt.Sprintf("✅ Loaded %d snapshots", realSnapshotCount))
}

// updateInfoText updates the info text at the bottom.
func (sm *SnapshotManager) updateInfoText(text string) {
	sm.infoText.SetText(text)
}

// createSnapshot shows the create snapshot dialog.
func (sm *SnapshotManager) createSnapshot() {
	sm.form.ShowCreateForm(func() {
		sm.app.SetFocus(sm)
		sm.loadSnapshots() // Reload snapshots
	})
}

// deleteSnapshot deletes the selected snapshot.
func (sm *SnapshotManager) deleteSnapshot() {
	snapshot := sm.snapshotTable.GetSelectedSnapshot()
	if snapshot == nil {
		sm.updateInfoText("❌ No snapshot selected.")
		return
	}

	sm.performSnapshotOperation(
		"Delete",
		fmt.Sprintf("Failed to delete snapshot %s", snapshot.Name),
		fmt.Sprintf("Successfully deleted snapshot %s", snapshot.Name),
		func() error {
			return sm.operations.DeleteSnapshot(snapshot.Name)
		},
	)
}

// rollbackSnapshot rolls back to the selected snapshot.
func (sm *SnapshotManager) rollbackSnapshot() {
	snapshot := sm.snapshotTable.GetSelectedSnapshot()
	if snapshot == nil {
		sm.updateInfoText("❌ No snapshot selected.")
		return
	}

	sm.performSnapshotOperation(
		"Rollback",
		fmt.Sprintf("Failed to rollback to snapshot %s", snapshot.Name),
		fmt.Sprintf("Successfully rolled back to snapshot %s", snapshot.Name),
		func() error {
			return sm.operations.RollbackToSnapshot(snapshot.Name)
		},
	)
}

// performSnapshotOperation performs a snapshot operation with confirmation and error handling.
func (sm *SnapshotManager) performSnapshotOperation(
	operationName string,
	errorMessage string,
	successMessage string,
	operation func() error,
) {
	snapshot := sm.snapshotTable.GetSelectedSnapshot()
	if snapshot == nil {
		sm.app.showMessage(fmt.Sprintf("❌ Please select a snapshot to %s", operationName))
		return
	}

	// Store current focus
	sm.app.lastFocus = sm.app.GetFocus()

	// Create confirmation dialog
	message := fmt.Sprintf("Are you sure you want to %s snapshot '%s'?\n\nThis action cannot be undone.", operationName, snapshot.Name)

	onConfirm := func() {
		// Remove the confirmation dialog
		sm.app.pages.RemovePage("confirmation")

		// Restore focus
		if sm.app.lastFocus != nil {
			sm.app.SetFocus(sm.app.lastFocus)
		}

		// Perform operation in goroutine
		go func() {
			err := operation()
			if err != nil {
				sm.app.Application.QueueUpdateDraw(func() {
					sm.app.header.ShowError(fmt.Sprintf("%s: %v", errorMessage, err))
				})
			} else {
				// Add a small delay to allow backend data to update
				time.Sleep(500 * time.Millisecond)
				sm.app.Application.QueueUpdateDraw(func() {
					sm.loadSnapshots()
				})
				sm.app.Application.QueueUpdateDraw(func() {
					sm.app.header.ShowSuccess(successMessage)
				})
			}
		}()
	}

	onCancel := func() {
		// Remove the confirmation dialog
		sm.app.pages.RemovePage("confirmation")

		// Restore focus
		if sm.app.lastFocus != nil {
			sm.app.SetFocus(sm.app.lastFocus)
		}
	}

	confirm := CreateConfirmDialog(operationName, message, onConfirm, onCancel)
	sm.app.pages.AddPage("confirmation", confirm, false, true)
	sm.app.SetFocus(confirm)
}

// getHelpText returns the help/footer text for the snapshot manager.
func (sm *SnapshotManager) getHelpText() string {
	if sm.vm.Type == api.VMTypeQemu {
		return theme.ReplaceSemanticTags("[info]C[-]reate  [info]D[-]elete  [info]R[-]ollback  [info]B[-]ack  [info]↑/↓[-] Navigate  [info]Enter[-] Select")
	}
	return theme.ReplaceSemanticTags("[info]C[-]reate  [info]D[-]elete  [info]R[-]ollback  [info]B[-]ack  [info]↑/↓[-] Navigate  [info]Enter[-] Select")
}
