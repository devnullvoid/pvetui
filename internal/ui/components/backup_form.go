package components

import (
	"fmt"
	"strings"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BackupForm manages the create backup form.
type BackupForm struct {
	app               *App
	vm                *api.VM
	onSuccessCallback func()
	onTaskStarted     func(upid string)
}

// NewBackupForm creates a new backup form handler.
func NewBackupForm(app *App, vm *api.VM) *BackupForm {
	return &BackupForm{
		app: app,
		vm:  vm,
	}
}

// SetTaskStartedCallback sets the callback for when a backup task starts.
func (bf *BackupForm) SetTaskStartedCallback(callback func(upid string)) {
	bf.onTaskStarted = callback
}

// ShowCreateForm displays the create backup form.
func (bf *BackupForm) ShowCreateForm(onSuccess func()) {
	bf.onSuccessCallback = onSuccess
	bf.app.header.ShowLoading("Loading storages...")

	go func() {
		client, err := bf.app.getClientForVM(bf.vm)
		if err != nil {
			bf.app.Application.QueueUpdateDraw(func() {
				bf.app.header.ShowActiveProfile(bf.app.header.GetCurrentProfile())
				bf.app.header.ShowError(fmt.Sprintf("Failed to get client: %v", err))
			})
			return
		}

		storages, err := client.GetNodeStorages(bf.vm.Node)

		bf.app.Application.QueueUpdateDraw(func() {
			bf.app.header.ShowActiveProfile(bf.app.header.GetCurrentProfile())

			if err != nil {
				bf.app.header.ShowError(fmt.Sprintf("Failed to load storages: %v", err))
				return
			}

			var storageOptions []string

			// Filter storages
			for _, s := range storages {
				if strings.Contains(s.Content, "backup") {
					storageOptions = append(storageOptions, s.Name)
				}
			}

			if len(storageOptions) == 0 {
				bf.app.showMessageSafe("No storage supports backups on this node.")
				return
			}

			bf.displayCreateForm(storageOptions)
		})
	}()
}

func (bf *BackupForm) displayCreateForm(storageOptions []string) {
	storageDropdown := tview.NewDropDown().
		SetLabel("Target Storage").
		SetOptions(storageOptions, nil).
		SetCurrentOption(0).
		SetFieldWidth(30)

	modeOptions := []string{"snapshot", "suspend", "stop"}
	modeDropdown := tview.NewDropDown().
		SetLabel("Backup Mode").
		SetOptions(modeOptions, nil).
		SetCurrentOption(0). // Snapshot default
		SetFieldWidth(20)

	compressOptions := []string{"zstd", "gzip", "lzo", "none"}
	compressDropdown := tview.NewDropDown().
		SetLabel("Compression").
		SetOptions(compressOptions, nil).
		SetCurrentOption(0). // zstd default
		SetFieldWidth(20)

	notesField := tview.NewInputField().
		SetLabel("Notes").
		SetFieldWidth(40)

	removeCheck := tview.NewCheckbox().
		SetLabel("Prune (Remove old)").
		SetChecked(false)

	form := tview.NewForm().
		AddFormItem(storageDropdown).
		AddFormItem(modeDropdown).
		AddFormItem(compressDropdown).
		AddFormItem(notesField).
		AddFormItem(removeCheck)
	form.SetLabelColor(theme.Colors.HeaderText)

	form.AddButton("Backup", func() {
		storageIdx, _ := storageDropdown.GetCurrentOption()
		storage := storageOptions[storageIdx]

		modeIdx, _ := modeDropdown.GetCurrentOption()
		mode := modeOptions[modeIdx]

		compressIdx, _ := compressDropdown.GetCurrentOption()
		compress := compressOptions[compressIdx]
		if compress == "none" {
			compress = "0"
		}

		notes := notesField.GetText()
		remove := removeCheck.IsChecked()

		bf.app.pages.RemovePage("createBackup")
		bf.onSuccessCallback() // Restore focus

		bf.app.header.ShowLoading("Starting backup...")

		go func() {
			ops := NewBackupOperations(bf.app, bf.vm)
			opts := api.BackupOptions{
				Storage:     storage,
				Mode:        mode,
				Compression: compress,
				Notes:       notes,
				Remove:      remove,
			}

			upid, err := ops.CreateBackup(opts)
			bf.app.Application.QueueUpdateDraw(func() {
				if err != nil {
					bf.app.header.ShowError(fmt.Sprintf("Backup failed to start: %v", err))
				} else {
					bf.app.header.ShowSuccess(fmt.Sprintf("Backup started (UPID: %s)", upid))
					if bf.onTaskStarted != nil {
						bf.onTaskStarted(upid)
					}
				}
			})
		}()
	}).
		AddButton("Cancel", func() {
			bf.app.pages.RemovePage("createBackup")
			bf.onSuccessCallback()
		})

	form.SetBorder(true).SetTitle(" Create Backup ").SetTitleAlign(tview.AlignCenter)

	// Esc key
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			bf.app.pages.RemovePage("createBackup")
			bf.onSuccessCallback()
			return nil
		}
		return event
	})

	bf.app.pages.AddPage("createBackup", form, true, true)
	bf.app.SetFocus(form)
}
