package components

import (
	"fmt"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SnapshotForm manages the create snapshot form.
type SnapshotForm struct {
	app               *App
	vm                *api.VM
	onSuccessCallback func()
}

// NewSnapshotForm creates a new snapshot form handler.
func NewSnapshotForm(app *App, vm *api.VM) *SnapshotForm {
	return &SnapshotForm{
		app: app,
		vm:  vm,
	}
}

// ShowCreateForm displays the create snapshot form.
func (sf *SnapshotForm) ShowCreateForm(onSuccess func()) {
	// Store the success callback for later use
	sf.onSuccessCallback = onSuccess
	// Create form items first
	nameField := tview.NewInputField().SetLabel("Snapshot Name").SetFieldWidth(20)
	descField := tview.NewInputField().SetLabel("Description").SetFieldWidth(40)

	form := tview.NewForm().
		AddFormItem(nameField).
		AddFormItem(descField)
	form.SetLabelColor(theme.Colors.HeaderText)

	// Only show VM State for QEMU guests
	var vmStateCheck *tview.Checkbox
	if sf.vm.Type == api.VMTypeQemu {
		vmStateCheck = tview.NewCheckbox().SetLabel("Include VM State (RAM)").SetChecked(true)
		form.AddFormItem(vmStateCheck)
	}

	form.AddButton("Create", func() {
		name := nameField.GetText()
		description := descField.GetText()
		vmState := false
		if sf.vm.Type == api.VMTypeQemu && vmStateCheck != nil {
			vmState = vmStateCheck.IsChecked()
		}

		if name == "" {
			sf.app.showMessageSafe("❌ Snapshot name is required")
			return
		}

		// Remove form and restore focus immediately
		sf.app.pages.RemovePage("createSnapshot")
		onSuccess() // Restore focus to snapshot manager

		// Show loading indicator
		sf.app.header.ShowLoading(fmt.Sprintf("Creating snapshot '%s'", name))

		// Perform async operation
		go func() {
			operations := NewSnapshotOperations(sf.app, sf.vm)
			err := operations.CreateSnapshot(name, description, vmState)
			if err != nil {
				sf.app.Application.QueueUpdateDraw(func() {
					sf.app.header.ShowError(fmt.Sprintf("Failed to create snapshot: %v", err))
				})
			} else {
				// Use the same polling method as delete/rollback
				sf.pollForSnapshotUpdates("Snapshot created successfully")
			}
		}()
	}).
		AddButton("Cancel", func() {
			sf.app.pages.RemovePage("createSnapshot")
			onSuccess()
		})

	form.SetBorder(true).SetTitle(" Create Snapshot ").SetTitleAlign(tview.AlignCenter)

	// Add keyboard input capture for Escape key
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			sf.app.pages.RemovePage("createSnapshot")
			onSuccess()
			return nil
		}
		return event
	})

	sf.app.pages.AddPage("createSnapshot", form, true, true)
	sf.app.SetFocus(form)
}

// pollForSnapshotUpdates handles snapshot list updates after operations.
func (sf *SnapshotForm) pollForSnapshotUpdates(successMessage string) {
	// Since the API already polls for task completion, we can show success immediately
	sf.app.Application.QueueUpdateDraw(func() {
		sf.app.header.ShowSuccess(successMessage)
		// Call the success callback to reload snapshots
		if sf.onSuccessCallback != nil {
			sf.onSuccessCallback()
		}
	})
}
