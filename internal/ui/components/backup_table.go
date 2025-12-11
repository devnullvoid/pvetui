package components

import (
	"sort"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BackupTable manages the backup list display and selection.
type BackupTable struct {
	*tview.Table
	vm      *api.VM
	app     *App
	backups []api.Backup
}

// NewBackupTable creates a new backup table.
//
//nolint:dupl // Similar to snapshot table
func NewBackupTable(app *App, vm *api.VM) *BackupTable {
	bt := &BackupTable{
		vm:  vm,
		app: app,
	}

	bt.Table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary).Attributes(tcell.AttrReverse)).
		SetDoneFunc(func(key tcell.Key) {
			// Go back when Escape is pressed
			if key == tcell.KeyEsc {
				bt.goBack()
			}
		})

	bt.SetBorder(true)
	bt.SetBorderColor(theme.Colors.Border)
	bt.SetTitle(" Backups for " + vm.Name + " (" + vm.Type + ") ")
	bt.SetTitleColor(theme.Colors.Title)

	bt.setupTableHeaders()
	return bt
}

// setupTableHeaders sets up the table headers.
func (bt *BackupTable) setupTableHeaders() {
	headers := []string{"Date", "VolID / Name", "Size", "Storage", "Format", "Notes"}
	colors := []tcell.Color{
		theme.Colors.HeaderText,
		theme.Colors.HeaderText,
		theme.Colors.HeaderText,
		theme.Colors.HeaderText,
		theme.Colors.HeaderText,
		theme.Colors.HeaderText,
	}

	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(colors[i]).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1)
		bt.SetCell(0, i, cell)
	}
}

// DisplayBackups displays the backups in the table.
func (bt *BackupTable) DisplayBackups(backups []api.Backup) {
	bt.backups = backups

	// Clear existing rows (keep headers)
	for row := 1; row < bt.GetRowCount(); row++ {
		for col := 0; col < bt.GetColumnCount(); col++ {
			bt.SetCell(row, col, nil)
		}
	}
	// We might need to reduce row count if it doesn't auto shrink?
	// tview Table doesn't automatically shrink row count when setting cells to nil?
	// Actually we should create a new table or just reset it properly.
	// But `SetCell` to nil doesn't remove the row.
	// The `Clear` method removes all content.
	// Let's use RemoveRow loop.
	for i := bt.GetRowCount() - 1; i >= 1; i-- {
		bt.RemoveRow(i)
	}

	if len(backups) == 0 {
		return
	}

	// Sort backups: newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Date.After(backups[j].Date)
	})

	for i, backup := range backups {
		row := i + 1

		dateStr := backup.Date.Format("2006-01-02 15:04:05")
		bt.SetCell(row, 0, tview.NewTableCell(dateStr).SetTextColor(theme.Colors.Primary))

		// Show VolID or Name. Name is usually better if derived from VolID
		name := backup.Name
		if name == "" {
			name = backup.VolID
		}
		bt.SetCell(row, 1, tview.NewTableCell(name).SetTextColor(theme.Colors.Primary))

		sizeStr := utils.FormatBytes(backup.Size)
		bt.SetCell(row, 2, tview.NewTableCell(sizeStr).SetTextColor(theme.Colors.Primary))

		bt.SetCell(row, 3, tview.NewTableCell(backup.Storage).SetTextColor(theme.Colors.Primary))

		bt.SetCell(row, 4, tview.NewTableCell(backup.Format).SetTextColor(theme.Colors.Primary))

		bt.SetCell(row, 5, tview.NewTableCell(backup.Notes).SetTextColor(theme.Colors.Primary))
	}

	bt.Select(1, 0)
}

// GetSelectedBackup gets the currently selected backup.
func (bt *BackupTable) GetSelectedBackup() *api.Backup {
	row, _ := bt.GetSelection()
	if row <= 0 || row >= bt.GetRowCount() {
		return nil
	}

	// Index matches displayed backups (since we sorted input array in DisplayBackups,
	// but wait, we sorted the slice 'backups' which is a copy? No, slice is reference?
	// Wait, sort.Slice modifies the slice in place.
	// But `bt.backups = backups` happened before sort? No, `backups` argument.
	// We should sort `bt.backups` after assignment.
	// Let's fix DisplayBackups.

	// Actually, `backups` slice passed to DisplayBackups is usually a fresh slice from API.
	// So we can sort it.
	// But `bt.backups` stores it.

	// To be safe, let's look at the implementation of GetSelectedBackup in SnapshotTable.
	// It relies on getting the name from the cell and finding it.
	// Backups don't have a unique name column necessarily displayed fully (if we truncate or show filename).
	// But VolID is unique.
	// But we displayed Name/VolID in column 1.

	// We can trust the row index if we store the sorted slice in `bt.backups`.
	// Since we sorted `backups` in DisplayBackups, and assigned `bt.backups = backups` (pointer copy),
	// if we sort `backups` BEFORE assignment or modify it in place, it works.
	// In DisplayBackups, I did `bt.backups = backups` then sort.
	// `backups` slice header is copied. `sort.Slice` swaps elements in underlying array.
	// So `bt.backups` will see the sorted order? Yes, if they share underlying array.

	// Use index logic.
	index := row - 1
	if index >= 0 && index < len(bt.backups) {
		return &bt.backups[index]
	}

	return nil
}

// goBack returns to the previous screen.
func (bt *BackupTable) goBack() {
	bt.app.pages.RemovePage("backups")
	bt.app.SetFocus(bt.app.vmList)
}
