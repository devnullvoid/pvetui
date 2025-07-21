package components

import (
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// TasksList encapsulates the tasks list panel
type TasksList struct {
	*tview.Table
	app   *App
	tasks []*api.ClusterTask
}

var _ TasksListComponent = (*TasksList)(nil)

// NewTasksList creates a new tasks list panel
func NewTasksList() *TasksList {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetTitle(" Tasks ")
	table.SetBorder(true)
	table.Clear()
	table.SetCell(0, 0, tview.NewTableCell("No tasks available").
		SetTextColor(theme.Colors.Primary).
		SetTextColor(theme.Colors.Warning).
		SetAlign(tview.AlignCenter))

	// Set selection style
	table.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary))

	return &TasksList{
		Table: table,
		tasks: make([]*api.ClusterTask, 0),
	}
}

// SetApp sets the application reference
func (tl *TasksList) SetApp(app *App) {
	tl.app = app
}

// SetTasks updates the tasks list with new data
func (tl *TasksList) SetTasks(tasks []*api.ClusterTask) {
	tl.tasks = tasks
	tl.updateTable()
}

// SetFilteredTasks updates the tasks list with filtered data
func (tl *TasksList) SetFilteredTasks(tasks []*api.ClusterTask) {
	tl.tasks = tasks
	tl.updateTable()
}

// GetSelectedTask returns the currently selected task
func (tl *TasksList) GetSelectedTask() *api.ClusterTask {
	row, _ := tl.GetSelection()
	if row <= 0 || row > len(tl.tasks) {
		return nil
	}
	return tl.tasks[row-1] // -1 because row 0 is the header
}

// Select wraps the table Select method to match the interface
func (tl *TasksList) Select(row, column int) *tview.Table {
	return tl.Table.Select(row, column)
}

// Clear clears the tasks list
func (tl *TasksList) Clear() *tview.Table {
	tl.Table.Clear()
	tl.SetCell(0, 0, tview.NewTableCell("No tasks available").
		SetTextColor(theme.Colors.Primary).
		SetTextColor(theme.Colors.Warning).
		SetAlign(tview.AlignCenter))
	return tl.Table
}

// updateTable refreshes the table content
func (tl *TasksList) updateTable() {
	tl.Clear()

	if len(tl.tasks) == 0 {
		tl.SetCell(0, 0, tview.NewTableCell("No tasks available").
			SetTextColor(theme.Colors.Primary).
			SetTextColor(theme.Colors.Warning).
			SetAlign(tview.AlignCenter))
		return
	}

	// Sort tasks by start time (newest first)
	sortedTasks := make([]*api.ClusterTask, len(tl.tasks))
	copy(sortedTasks, tl.tasks)
	sort.Slice(sortedTasks, func(i, j int) bool {
		return sortedTasks[i].StartTime > sortedTasks[j].StartTime
	})

	// Set headers
	tl.SetCell(0, 0, tview.NewTableCell("Task").SetTextColor(theme.Colors.Primary).SetSelectable(false))
	tl.SetCell(0, 1, tview.NewTableCell("Status").SetTextColor(theme.Colors.Primary).SetSelectable(false))
	tl.SetCell(0, 2, tview.NewTableCell("Start Time").SetTextColor(theme.Colors.Primary).SetSelectable(false))
	tl.SetCell(0, 3, tview.NewTableCell("Duration").SetTextColor(theme.Colors.Primary).SetSelectable(false))

	// Add task data
	for i, task := range sortedTasks {
		row := i + 1

		// Task name (truncate if too long)
		taskName := task.ID
		if len(taskName) > 30 {
			taskName = taskName[:27] + "..."
		}
		tl.SetCell(row, 0, tview.NewTableCell(taskName).SetTextColor(theme.Colors.Primary))

		// Status with color coding
		var color tcell.Color
		switch strings.ToLower(task.Status) {
		case "running":
			color = theme.Colors.StatusRunning
		case "stopped", "failed":
			color = theme.Colors.StatusStopped
		case "pending":
			color = theme.Colors.StatusPending
		default:
			color = theme.Colors.StatusError
		}

		// Handle unknown status
		if task.Status == "" {
			color = theme.Colors.Primary
		}

		tl.SetCell(row, 1, tview.NewTableCell(task.Status).SetTextColor(color))

		// Start time
		startTime := "N/A"
		if task.StartTime > 0 {
			startTime = time.Unix(task.StartTime, 0).Format("15:04:05")
		}
		tl.SetCell(row, 2, tview.NewTableCell(startTime).SetTextColor(theme.Colors.Secondary))

		// Duration
		duration := "N/A"
		if task.StartTime > 0 {
			var endTime int64
			if task.EndTime > 0 {
				endTime = task.EndTime
			} else {
				endTime = time.Now().Unix()
			}
			durationTime := time.Duration(endTime-task.StartTime) * time.Second
			durationStr := durationTime.String()
			if len(durationStr) > 8 {
				durationStr = durationStr[:8]
			}
			duration = durationStr
		}
		tl.SetCell(row, 3, tview.NewTableCell(duration).SetTextColor(theme.Colors.Secondary))
	}

	// Scroll to the top
	tl.ScrollToBeginning()
}
