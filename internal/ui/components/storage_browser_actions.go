package components

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/devnullvoid/pvetui/internal/taskmanager"
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) showStorageBrowser(node *api.Node) {
	a.syncStorageBrowserNodes()
	if a.storageBrowser != nil && node != nil {
		a.storageBrowser.SelectNode(node)
		return
	}

	a.pages.SwitchToPage(api.PageStorage)
	if browser, ok := a.storageBrowser.(*StorageBrowser); ok {
		a.SetFocus(browser.tree)
	}
}

func (a *App) syncStorageBrowserNodes() {
	if a.storageBrowser == nil {
		return
	}
	a.storageBrowser.SetNodes(append([]*api.Node(nil), models.GlobalState.OriginalNodes...))
}

type storageMenuEntry struct {
	label    string
	shortcut rune
	handler  func()
}

func (a *App) ShowStorageContextMenu() {
	browser, ok := a.storageBrowser.(*StorageBrowser)
	if !ok || browser == nil {
		return
	}

	if browser.selection.Storage == nil {
		a.showMessageSafe("Select a storage or content item first")
		return
	}

	a.lastFocus = a.GetFocus()

	if item := browser.selectedContentItem(); item != nil {
		a.showStorageContentContextMenu(browser, item)
		return
	}

	a.showStorageSelectionContextMenu(browser)
}

func (a *App) showStorageSelectionContextMenu(browser *StorageBrowser) {
	entries := []storageMenuEntry{
		{label: "Refresh Content", shortcut: 'r', handler: func() {
			browser.showContentMessage("Refreshing storage content...")
			browser.loadStorageContent(browser.selection)
		}},
		{label: "Show All Content", shortcut: 'a', handler: func() {
			browser.contentFilter = storageFilterAll
			browser.showStorageContent()
		}},
		{label: "Show Guest Volumes", shortcut: 'd', handler: func() {
			browser.contentFilter = storageFilterGuestVolumes
			browser.showStorageContent()
		}},
		{label: "Show ISO Images", shortcut: 'i', handler: func() {
			browser.contentFilter = storageFilterISO
			browser.showStorageContent()
		}},
		{label: "Show Templates", shortcut: 't', handler: func() {
			browser.contentFilter = storageFilterTemplates
			browser.showStorageContent()
		}},
		{label: "Show Snippets", shortcut: 's', handler: func() {
			browser.contentFilter = storageFilterSnippets
			browser.showStorageContent()
		}},
		{label: "Show Backups", shortcut: 'b', handler: func() {
			browser.contentFilter = storageFilterBackups
			browser.showStorageContent()
		}},
	}

	a.showStorageMenu(" Storage Actions ", entries, browser.tree)
}

func (a *App) showStorageContentContextMenu(browser *StorageBrowser, item *api.StorageContentItem) {
	entries := make([]storageMenuEntry, 0, 3)
	switch item.Content {
	case storageFilterBackups:
		entries = append(entries,
			storageMenuEntry{label: "Restore Backup", shortcut: 'r', handler: func() {
				a.showStorageBackupRestoreForm(browser.selection, *item)
			}},
			storageMenuEntry{label: "Delete Backup", shortcut: 'd', handler: func() {
				a.confirmDeleteStorageContent(browser.selection, *item)
			}},
		)
	case storageFilterISO:
		entries = append(entries, storageMenuEntry{label: "Delete ISO", shortcut: 'd', handler: func() {
			a.confirmDeleteStorageContent(browser.selection, *item)
		}})
	case storageFilterTemplates:
		entries = append(entries, storageMenuEntry{label: "Delete Template", shortcut: 'd', handler: func() {
			a.confirmDeleteStorageContent(browser.selection, *item)
		}})
	case storageFilterSnippets:
		entries = append(entries, storageMenuEntry{label: "Delete Snippet", shortcut: 'd', handler: func() {
			a.confirmDeleteStorageContent(browser.selection, *item)
		}})
	default:
		entries = append(entries, storageMenuEntry{label: "Inspect Details", shortcut: 'i', handler: func() {
			browser.showSelectedContentDetails(item)
			a.SetFocus(browser.details)
		}})
	}

	if len(entries) == 0 {
		a.showMessageSafe("No actions available for the selected storage content")
		return
	}

	a.showStorageMenu(" Content Actions ", entries, browser.contentTable)
}

func (a *App) showStorageMenu(title string, entries []storageMenuEntry, anchor tview.Primitive) {
	menuItems := make([]string, len(entries))
	shortcuts := make([]rune, len(entries))
	for i, entry := range entries {
		menuItems[i] = entry.label
		shortcuts[i] = entry.shortcut
	}

	menu := NewContextMenuWithShortcuts(title, menuItems, shortcuts, func(index int, action string) {
		a.CloseContextMenu()
		if index >= 0 && index < len(entries) && entries[index].handler != nil {
			entries[index].handler()
		}
	})
	menu.SetApp(a)
	menuList := menu.Show()

	oldCapture := menuList.GetInputCapture()
	menuList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'h') {
			a.CloseContextMenu()
			return nil
		}

		if oldCapture != nil {
			return oldCapture(event)
		}

		return event
	})

	a.showContextMenuPage(menuList, menuItems, 28, true, anchor)
}

func (a *App) confirmDeleteStorageContent(selection storageSelection, item api.StorageContentItem) {
	message := fmt.Sprintf(
		"Delete %s?\n\n%s\n\nThis action cannot be undone.",
		storageContentLabel(item),
		item.VolID,
	)

	confirm := CreateConfirmDialog("Delete Storage Content", message, func() {
		a.removePageIfPresent("confirmation")
		a.enqueueStorageContentDelete(selection, item)
	}, func() {
		a.removePageIfPresent("confirmation")
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	})
	a.pages.AddPage("confirmation", confirm, false, true)
	a.SetFocus(confirm)
}

func (a *App) enqueueStorageContentDelete(selection storageSelection, item api.StorageContentItem) {
	if selection.Node == nil || selection.Storage == nil {
		a.showMessageSafe("Select a storage content item first")
		return
	}

	task := &taskmanager.Task{
		Type:        "Delete",
		Description: fmt.Sprintf("Delete %s", item.VolID),
		TargetVMID:  storageTaskTargetID(item),
		TargetNode:  selection.Node.Name,
		TargetName:  item.VolID,
		Operation: func() (string, error) {
			client, err := a.getClientForNode(selection.Node)
			if err != nil {
				return "", err
			}
			return client.DeleteStorageContent(selection.Node.Name, selection.Storage.Name, item.VolID)
		},
		OnComplete: func(err error) {
			if err != nil {
				a.QueueUpdateDraw(func() {
					a.header.ShowError(fmt.Sprintf("Failed to delete %s: %v", item.VolID, err))
				})
				return
			}

			a.ClearAPICache()
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Deleted %s", item.VolID))
			})
			go a.manualRefresh()
		},
	}

	a.taskManager.Enqueue(task)
	a.header.ShowSuccess(fmt.Sprintf("Queued delete for %s", item.VolID))
}

func (a *App) showStorageBackupRestoreForm(selection storageSelection, item api.StorageContentItem) {
	if selection.Node == nil {
		a.showMessageSafe("Select a backup first")
		return
	}

	defaultType, defaultVMID := inferBackupRestoreDefaults(item.VolID)
	if defaultType == "" {
		defaultType = api.VMTypeQemu
	}

	typeOptions := []string{api.VMTypeQemu, api.VMTypeLXC}
	defaultTypeIndex := 0
	if defaultType == api.VMTypeLXC {
		defaultTypeIndex = 1
	}

	form := newStandardForm()
	form.SetBorder(true)
	form.SetTitle(" Restore Backup ")

	typeDropdown := tview.NewDropDown().
		SetLabel("Guest Type").
		SetOptions(typeOptions, nil).
		SetCurrentOption(defaultTypeIndex).
		SetFieldWidth(18)

	vmidField := tview.NewInputField().
		SetLabel("Target VMID").
		SetFieldWidth(12)
	if defaultVMID > 0 {
		vmidField.SetText(strconv.Itoa(defaultVMID))
	}

	forceCheckbox := tview.NewCheckbox().
		SetLabel("Overwrite Existing").
		SetChecked(false)

	form.AddFormItem(typeDropdown)
	form.AddFormItem(vmidField)
	form.AddFormItem(forceCheckbox)

	restorePage := "modal:storageRestore"

	closeRestore := func() {
		a.removePageIfPresent(restorePage)
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}

	form.AddButton("Restore", func() {
		rawVMID := strings.TrimSpace(vmidField.GetText())
		vmid, err := strconv.Atoi(rawVMID)
		if err != nil || vmid <= 0 {
			a.showMessageSafe("Enter a valid positive VMID")
			return
		}

		typeIndex, _ := typeDropdown.GetCurrentOption()
		guestType := typeOptions[typeIndex]
		force := forceCheckbox.IsChecked()

		confirmMessage := fmt.Sprintf(
			"Restore backup %s\n\nto %s %d on node %s?",
			item.VolID,
			strings.ToUpper(guestType),
			vmid,
			selection.Node.Name,
		)
		if force {
			confirmMessage += "\n\nExisting guest data at that VMID may be overwritten."
		}

		confirmPage := "modal:storageRestoreConfirm"
		confirm := CreateConfirmDialog("Confirm Restore", confirmMessage, func() {
			a.removePageIfPresent(confirmPage)
			closeRestore()
			a.enqueueStorageBackupRestore(selection, item, guestType, vmid, force)
		}, func() {
			a.removePageIfPresent(confirmPage)
			a.SetFocus(form)
		})
		a.pages.AddPage(confirmPage, confirm, false, true)
		a.SetFocus(confirm)
	})
	form.AddButton("Cancel", closeRestore)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeRestore()
			return nil
		}
		return event
	})

	a.pages.AddPage(restorePage, form, true, true)
	a.SetFocus(form)
}

func (a *App) enqueueStorageBackupRestore(selection storageSelection, item api.StorageContentItem, guestType string, vmid int, force bool) {
	if selection.Node == nil {
		a.showMessageSafe("Select a backup first")
		return
	}

	task := &taskmanager.Task{
		Type:        "Restore",
		Description: fmt.Sprintf("Restore %s to %s %d", item.VolID, strings.ToUpper(guestType), vmid),
		TargetVMID:  vmid,
		TargetNode:  selection.Node.Name,
		TargetName:  item.VolID,
		Operation: func() (string, error) {
			client, err := a.getClientForNode(selection.Node)
			if err != nil {
				return "", err
			}
			return client.RestoreGuestFromBackup(selection.Node.Name, guestType, vmid, item.VolID, force)
		},
		OnComplete: func(err error) {
			if err != nil {
				a.QueueUpdateDraw(func() {
					a.header.ShowError(fmt.Sprintf("Failed to restore %s: %v", item.VolID, err))
				})
				return
			}

			a.ClearAPICache()
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("Restored %s to %s %d", item.VolID, strings.ToUpper(guestType), vmid))
			})
			go a.manualRefresh()
		},
	}

	a.taskManager.Enqueue(task)
	a.header.ShowSuccess(fmt.Sprintf("Queued restore of %s", item.VolID))
}

func storageContentLabel(item api.StorageContentItem) string {
	switch item.Content {
	case storageFilterBackups:
		return "backup"
	case storageFilterISO:
		return "ISO image"
	case storageFilterTemplates:
		return "template"
	case storageFilterSnippets:
		return "snippet"
	case "images":
		return "disk image"
	case "rootdir":
		return "container volume"
	default:
		return "storage content"
	}
}

func storageTaskTargetID(item api.StorageContentItem) int {
	if item.VMID > 0 {
		return item.VMID
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(item.VolID))
	return int(hasher.Sum32() & 0x7fffffff)
}

func inferBackupRestoreDefaults(volID string) (string, int) {
	lower := strings.ToLower(volID)
	switch {
	case strings.Contains(lower, "vzdump-qemu-"):
		return api.VMTypeQemu, extractBackupVMID(lower, "vzdump-qemu-")
	case strings.Contains(lower, "vzdump-lxc-"):
		return api.VMTypeLXC, extractBackupVMID(lower, "vzdump-lxc-")
	default:
		return "", 0
	}
}

func extractBackupVMID(volID, marker string) int {
	idx := strings.Index(volID, marker)
	if idx < 0 {
		return 0
	}

	rest := volID[idx+len(marker):]
	end := strings.Index(rest, "-")
	if end <= 0 {
		return 0
	}

	vmid, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0
	}
	return vmid
}
