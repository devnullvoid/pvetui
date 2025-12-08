package components

import (
	"fmt"
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BackupManager manages the backup interface for VMs and containers.
type BackupManager struct {
	*tview.Flex
	vm           *api.VM
	app          *App
	backupTable  *BackupTable
	infoText     *tview.TextView
	loading      bool
	createBtn    *tview.Button
	restoreBtn   *tview.Button
	deleteBtn    *tview.Button
	backBtn      *tview.Button
	operations   *BackupOperations
	form         *BackupForm
}

// NewBackupManager creates a new backup manager for the given VM.
func NewBackupManager(app *App, vm *api.VM) *BackupManager {
	bm := &BackupManager{
		vm:  vm,
		app: app,
	}

	// Create components
	bm.backupTable = NewBackupTable(app, vm)
	bm.operations = NewBackupOperations(app, vm)
	bm.form = NewBackupForm(app, vm)

	// Create info text
	bm.infoText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true)

	// Create header
	header := bm.createHeader()

	// Create layout
	bm.Flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(header, 3, 0, false).
		AddItem(bm.backupTable, 0, 1, true).
		AddItem(bm.infoText, 1, 0, false)

	// Add border
	bm.SetBorder(true)
	bm.SetBorderColor(theme.Colors.Border)
	bm.SetTitle(" Backup Manager ")
	bm.SetTitleColor(theme.Colors.Title)

	// Load backups
	bm.loadBackups()

	// Set up keyboard navigation
	bm.setupKeyboardNavigation()

	return bm
}

// setupKeyboardNavigation sets up keyboard shortcuts.
func (bm *BackupManager) setupKeyboardNavigation() {
	bm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			bm.goBack()
			return nil
		case tcell.KeyTab:
			currentFocus := bm.app.GetFocus()
			if currentFocus == bm.backupTable {
				bm.app.SetFocus(bm.createBtn)
				return nil
			} else if currentFocus == bm.createBtn {
				bm.app.SetFocus(bm.restoreBtn)
				return nil
			} else if currentFocus == bm.restoreBtn {
				bm.app.SetFocus(bm.deleteBtn)
				return nil
			} else if currentFocus == bm.deleteBtn {
				bm.app.SetFocus(bm.backBtn)
				return nil
			} else if currentFocus == bm.backBtn {
				bm.app.SetFocus(bm.backupTable)
				return nil
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'c', 'C':
				bm.createBackup()
				return nil
			case 'r', 'R':
				bm.restoreBackup()
				return nil
			case 'd', 'D':
				bm.deleteBackup()
				return nil
			case 'b', 'B':
				bm.goBack()
				return nil
			}
		}
		return event
	})
}

// goBack returns to the previous screen.
func (bm *BackupManager) goBack() {
	bm.app.pages.RemovePage("backups")
	bm.app.SetFocus(bm.app.vmList)
}

// createHeader creates the header with buttons.
func (bm *BackupManager) createHeader() *tview.Flex {
	bm.createBtn = tview.NewButton("Backup Now (C)").
		SetSelectedFunc(bm.createBackup)

	bm.restoreBtn = tview.NewButton("Restore (R)").
		SetSelectedFunc(bm.restoreBackup)

	bm.deleteBtn = tview.NewButton("Delete (D)").
		SetSelectedFunc(bm.deleteBackup)

	bm.backBtn = tview.NewButton("Back (B)").
		SetSelectedFunc(bm.goBack)

	buttons := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(bm.createBtn, 18, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(bm.restoreBtn, 15, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(bm.deleteBtn, 15, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(bm.backBtn, 12, 0, false).
		AddItem(tview.NewBox(), 0, 1, false)

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(buttons, 1, 0, false).
		AddItem(tview.NewBox(), 1, 0, false)
}

// loadBackups loads and displays backups.
func (bm *BackupManager) loadBackups() {
	bm.loading = true
	bm.updateInfoText("Loading backups...")

	go func() {
		backups, err := bm.operations.GetBackups()
		bm.app.Application.QueueUpdateDraw(func() {
			bm.loading = false
			if err != nil {
				bm.updateInfoText(fmt.Sprintf("❌ Error loading backups: %v", err))
				return
			}
			bm.displayBackups(backups)
		})
	}()
}

func (bm *BackupManager) displayBackups(backups []api.Backup) {
	bm.backupTable.DisplayBackups(backups)

	if len(backups) == 0 {
		bm.updateInfoText("No backups found for this VM.")
		return
	}
	bm.updateInfoText(fmt.Sprintf("✅ Loaded %d backups", len(backups)))
}

func (bm *BackupManager) updateInfoText(text string) {
	bm.infoText.SetText(text)
}

func (bm *BackupManager) createBackup() {
	if isPending, pendingOperation := models.GlobalState.IsVMPending(bm.vm); isPending {
		bm.app.showMessageSafe(fmt.Sprintf("Cannot create backup while '%s' is in progress", pendingOperation))
		return
	}

	bm.form.ShowCreateForm(func() {
		bm.app.SetFocus(bm)
		// We don't auto-reload because backup is async and takes time.
		// Maybe reload to see if there is a lock or temp file?
		// But usually not necessary immediately.
	})
}

func (bm *BackupManager) restoreBackup() {
	if isPending, pendingOperation := models.GlobalState.IsVMPending(bm.vm); isPending {
		bm.app.showMessageSafe(fmt.Sprintf("Cannot restore backup while '%s' is in progress", pendingOperation))
		return
	}

	backup := bm.backupTable.GetSelectedBackup()
	if backup == nil {
		bm.updateInfoText("❌ No backup selected.")
		return
	}

	bm.app.lastFocus = bm.app.GetFocus()

	message := fmt.Sprintf("Are you sure you want to restore backup '%s'?\n\nThis will overwrite the current VM state!\nThis action cannot be undone.", backup.VolID)

	onConfirm := func() {
		bm.app.pages.RemovePage("confirmation")
		if bm.app.lastFocus != nil {
			bm.app.SetFocus(bm.app.lastFocus)
		}

		bm.app.header.ShowLoading(fmt.Sprintf("Restoring backup '%s'", backup.VolID))

		go func() {
			upid, err := bm.operations.RestoreBackup(backup.VolID)
			bm.app.Application.QueueUpdateDraw(func() {
				if err != nil {
					bm.app.header.ShowError(fmt.Sprintf("Restore failed to start: %v", err))
				} else {
					bm.app.header.ShowSuccess(fmt.Sprintf("Restore started (UPID: %s)", upid))
				}
			})
		}()
	}

	onCancel := func() {
		bm.app.pages.RemovePage("confirmation")
		if bm.app.lastFocus != nil {
			bm.app.SetFocus(bm.app.lastFocus)
		}
	}

	confirm := CreateConfirmDialog("Restore Backup", message, onConfirm, onCancel)
	bm.app.pages.AddPage("confirmation", confirm, false, true)
	bm.app.SetFocus(confirm)
}

func (bm *BackupManager) deleteBackup() {
	if isPending, pendingOperation := models.GlobalState.IsVMPending(bm.vm); isPending {
		bm.app.showMessageSafe(fmt.Sprintf("Cannot delete backup while '%s' is in progress", pendingOperation))
		return
	}

	backup := bm.backupTable.GetSelectedBackup()
	if backup == nil {
		bm.updateInfoText("❌ No backup selected.")
		return
	}

	bm.app.lastFocus = bm.app.GetFocus()

	message := fmt.Sprintf("Are you sure you want to delete backup '%s'?\n\nThis action cannot be undone.", backup.VolID)

	onConfirm := func() {
		bm.app.pages.RemovePage("confirmation")
		if bm.app.lastFocus != nil {
			bm.app.SetFocus(bm.app.lastFocus)
		}

		bm.app.header.ShowLoading(fmt.Sprintf("Deleting backup '%s'", backup.VolID))

		go func() {
			err := bm.operations.DeleteBackup(backup.VolID)
			bm.app.Application.QueueUpdateDraw(func() {
				if err != nil {
					bm.app.header.ShowError(fmt.Sprintf("Failed to delete backup: %v", err))
				} else {
					bm.app.header.ShowSuccess(fmt.Sprintf("Successfully deleted backup %s", backup.VolID))

					// Reload backups after delay
					go func() {
						time.Sleep(1 * time.Second)
						bm.loadBackups()
					}()
				}
			})
		}()
	}

	onCancel := func() {
		bm.app.pages.RemovePage("confirmation")
		if bm.app.lastFocus != nil {
			bm.app.SetFocus(bm.app.lastFocus)
		}
	}

	confirm := CreateConfirmDialog("Delete Backup", message, onConfirm, onCancel)
	bm.app.pages.AddPage("confirmation", confirm, false, true)
	bm.app.SetFocus(confirm)
}
