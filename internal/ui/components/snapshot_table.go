package components

import (
	"sort"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SnapshotTable manages the snapshot list display and selection.
type SnapshotTable struct {
	*tview.Table
	vm        *api.VM
	app       *App
	snapshots []api.Snapshot
}

// NewSnapshotTable creates a new snapshot table.
func NewSnapshotTable(app *App, vm *api.VM) *SnapshotTable {
	st := &SnapshotTable{
		vm:  vm,
		app: app,
	}

	st.Table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary)).
		SetDoneFunc(func(key tcell.Key) {
			// Go back to VM list when Escape is pressed
			if key == tcell.KeyEsc {
				st.goBack()
			}
		})

	st.SetBorder(true)
	st.SetBorderColor(theme.Colors.Border)
	st.SetTitle(" Snapshots for " + vm.Name + " (" + vm.Type + ") ")
	st.SetTitleColor(theme.Colors.Title)

	st.setupTableHeaders()
	return st
}

// setupTableHeaders sets up the table headers.
func (st *SnapshotTable) setupTableHeaders() {
	var headers []string
	var colors []tcell.Color

	if st.vm.Type == api.VMTypeQemu {
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
		st.SetCell(0, i, cell)
	}
}

// DisplaySnapshots displays the snapshots in the table.
func (st *SnapshotTable) DisplaySnapshots(snapshots []api.Snapshot) {
	// Store snapshots for later access
	st.snapshots = snapshots

	// Clear existing rows (keep headers)
	for row := 1; row < st.GetRowCount(); row++ {
		for col := 0; col < st.GetColumnCount(); col++ {
			st.SetCell(row, col, nil)
		}
	}

	if len(snapshots) == 0 {
		// Reset selection to header row when no snapshots
		st.Select(0, 0)
		return
	}

	// Build sorted view: oldest at top, newest at bottom; place "current" (NOW) last
	display := make([]api.Snapshot, 0, len(snapshots))
	var current *api.Snapshot
	for _, s := range snapshots {
		if s.Name == CurrentSnapshotName {
			ss := s
			current = &ss
		} else {
			display = append(display, s)
		}
	}
	sort.Slice(display, func(i, j int) bool {
		// If SnapTime is zero, treat as oldest
		if display[i].SnapTime.IsZero() && display[j].SnapTime.IsZero() {
			return display[i].Name < display[j].Name
		}
		if display[i].SnapTime.IsZero() {
			return true
		}
		if display[j].SnapTime.IsZero() {
			return false
		}
		return display[i].SnapTime.Before(display[j].SnapTime)
	})
	if current != nil {
		display = append(display, *current)
	}

	// Add snapshot rows
	for i, snapshot := range display {
		row := i + 1

		// Handle "current" as "NOW" like the web UI
		displayName := snapshot.Name
		if snapshot.Name == CurrentSnapshotName {
			displayName = "NOW"
		}

		// Use different color for "NOW" to indicate it's not a real snapshot
		nameColor := theme.Colors.Primary
		if snapshot.Name == CurrentSnapshotName {
			nameColor = theme.Colors.Secondary // Dimmed color for current state
		}

		st.SetCell(row, 0, tview.NewTableCell(displayName).SetTextColor(nameColor))

		// Handle different column layouts for QEMU vs LXC
		if st.vm.Type == api.VMTypeQemu {
			// QEMU: Name, RAM, Date/Status, Description
			ramText := ""
			if snapshot.VMState {
				ramText = "Yes"
			}
			st.SetCell(row, 1, tview.NewTableCell(ramText).SetTextColor(theme.Colors.Primary))

			dateText := ""
			if !snapshot.SnapTime.IsZero() {
				dateText = snapshot.SnapTime.Format("2006-01-02 15:04:05")
			}
			st.SetCell(row, 2, tview.NewTableCell(dateText).SetTextColor(theme.Colors.Primary))

			st.SetCell(row, 3, tview.NewTableCell(snapshot.Description).SetTextColor(theme.Colors.Primary))
		} else {
			// LXC: Name, Date/Status, Description
			dateText := ""
			if !snapshot.SnapTime.IsZero() {
				dateText = snapshot.SnapTime.Format("2006-01-02 15:04:05")
			}
			st.SetCell(row, 1, tview.NewTableCell(dateText).SetTextColor(theme.Colors.Primary))

			st.SetCell(row, 2, tview.NewTableCell(snapshot.Description).SetTextColor(theme.Colors.Primary))
		}
	}

	// Set selection to first snapshot row after refresh
	st.Select(1, 0)
}

// GetSelectedSnapshot gets the currently selected snapshot.
func (st *SnapshotTable) GetSelectedSnapshot() *api.Snapshot {
	row, _ := st.GetSelection()
	if row <= 0 || row >= st.GetRowCount() {
		return nil
	}

	// Get the snapshot name from the first column
	nameCell := st.GetCell(row, 0)
	if nameCell == nil {
		return nil
	}

	// Convert "NOW" back to "current" for API calls
	snapshotName := nameCell.Text
	if snapshotName == "NOW" {
		snapshotName = CurrentSnapshotName
	}

	// Find the snapshot in our list
	for _, snapshot := range st.snapshots {
		if snapshot.Name == snapshotName {
			return &snapshot
		}
	}

	return nil
}

// GetSnapshotCount returns the count of real snapshots (excluding "current").
func (st *SnapshotTable) GetSnapshotCount() int {
	realSnapshotCount := 0
	for _, snapshot := range st.snapshots {
		if snapshot.Name != CurrentSnapshotName {
			realSnapshotCount++
		}
	}
	return realSnapshotCount
}

// goBack returns to the previous screen.
func (st *SnapshotTable) goBack() {
	st.app.pages.RemovePage("snapshots")
	st.app.SetFocus(st.app.vmList)
}
