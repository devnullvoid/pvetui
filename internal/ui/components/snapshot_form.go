package components

import (
	"fmt"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SnapshotForm manages the create snapshot form.
type SnapshotForm struct {
	app *App
	vm  *api.VM
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
	// Create form items first
	nameField := tview.NewInputField().SetLabel("Snapshot Name").SetFieldWidth(20)
	descField := tview.NewInputField().SetLabel("Description").SetFieldWidth(40)

	form := tview.NewForm().
		AddFormItem(nameField).
		AddFormItem(descField)

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
			sf.app.showMessage("❌ Snapshot name is required")
			return
		}

		sf.app.pages.RemovePage("createSnapshot")
		onSuccess()

		go func() {
			operations := NewSnapshotOperations(sf.app, sf.vm)
			err := operations.CreateSnapshot(name, description, vmState)
			sf.app.QueueUpdateDraw(func() {
				if err != nil {
					sf.app.showMessage(fmt.Sprintf("❌ Failed to create snapshot: %v", err))
				} else {
					sf.app.showMessage("✅ Snapshot created successfully")
					onSuccess() // Reload snapshots
				}
			})
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
