package components

import (
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/peevetui/internal/ui/theme"
	"github.com/devnullvoid/peevetui/pkg/api"
)

// TasksList encapsulates the tasks list panel.
type TasksList struct {
	*tview.Table

	tasks []*api.ClusterTask
	app   *App
}

var _ TasksListComponent = (*TasksList)(nil)

// NewTasksList creates a new tasks list panel.
func NewTasksList() *TasksList {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetTitle(" Tasks ")
	table.SetBorder(true)
	table.SetSelectable(true, false)
	table.SetFixed(1, 0) // Fix the header row
	// Set selection style
	table.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary))

	tl := &TasksList{
		Table: table,
		tasks: make([]*api.ClusterTask, 0),
	}

	tl.setupKeyHandlers()

	return tl
}

// SetApp sets the application reference.
func (tl *TasksList) SetApp(app *App) {
	tl.app = app
}

// SetTasks updates the tasks list with new data.
func (tl *TasksList) SetTasks(tasks []*api.ClusterTask) {
	tl.tasks = tasks
	tl.updateTable()
}

// SetFilteredTasks updates the tasks list with filtered data.
func (tl *TasksList) SetFilteredTasks(tasks []*api.ClusterTask) {
	tl.tasks = tasks
	tl.updateTable()
}

// GetSelectedTask returns the currently selected task.
func (tl *TasksList) GetSelectedTask() *api.ClusterTask {
	row, _ := tl.GetSelection()
	if row <= 0 || row > len(tl.tasks) {
		return nil
	}

	return tl.tasks[row-1] // -1 because row 0 is the header
}

// Select wraps the table Select method to match the interface.
func (tl *TasksList) Select(row, column int) *tview.Table {
	return tl.Table.Select(row, column)
}

func (tl *TasksList) noTasksCell() *tview.TableCell {
	return tview.NewTableCell("No tasks available").
		SetTextColor(theme.Colors.Warning).
		SetAlign(tview.AlignCenter)
}

// Clear clears the tasks list.
func (tl *TasksList) Clear() *tview.Table {
	tl.Table.Clear()
	tl.SetCell(0, 0, tl.noTasksCell())

	return tl.Table
}

// updateTable refreshes the table content.
func (tl *TasksList) updateTable() {
	tl.Clear()

	if len(tl.tasks) == 0 {
		tl.SetCell(0, 0, tl.noTasksCell())

		return
	}

	// Sort tasks by start time (newest first)
	sortedTasks := make([]*api.ClusterTask, len(tl.tasks))
	copy(sortedTasks, tl.tasks)
	sort.Slice(sortedTasks, func(i, j int) bool {
		return sortedTasks[i].StartTime > sortedTasks[j].StartTime
	})

	// Set headers: Time, Node, Type, Status, User, ID, Duration
	headers := []string{"Time", "Node", "Type", "Status", "User", "ID", "Duration"}
	for i, header := range headers {
		tc := tview.NewTableCell(header).
			SetTextColor(theme.Colors.HeaderText).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		tl.SetCell(0, i, tc)
	}

	// Add task data
	for i, task := range sortedTasks {
		row := i + 1 // +1 because row 0 is the header

		// Format start time
		startTime := "N/A"
		if task.StartTime > 0 {
			startTime = time.Unix(task.StartTime, 0).Format("01-02-2006 15:04:05")
		}

		tl.SetCell(row, 0, tview.NewTableCell(startTime).SetTextColor(theme.Colors.Secondary).SetAlign(tview.AlignLeft))

		// Node (truncate if too long)
		node := task.Node
		if len(node) > 16 {
			node = node[:13] + "..."
		}

		tl.SetCell(row, 1, tview.NewTableCell(node).SetTextColor(theme.Colors.Primary).SetAlign(tview.AlignLeft))

		// Type (friendly name)
		typeStr := formatTaskType(task.Type)
		if len(typeStr) > 16 {
			typeStr = typeStr[:13] + "..."
		}

		tl.SetCell(row, 2, tview.NewTableCell(typeStr).SetTextColor(theme.Colors.Info).SetAlign(tview.AlignLeft))

		// Status with color coding
		statusCell := createStatusCell(task.Status)
		tl.SetCell(row, 3, statusCell)

		// User (cleaned up)
		user := formatUser(task.User)
		if len(user) > 18 {
			user = user[:15] + "..."
		}

		tl.SetCell(row, 4, tview.NewTableCell(user).SetTextColor(theme.Colors.Tertiary).SetAlign(tview.AlignLeft))

		// ID (truncate if too long)
		id := task.ID
		if len(id) > 24 {
			id = id[:21] + "..."
		}

		tl.SetCell(row, 5, tview.NewTableCell(id).SetTextColor(theme.Colors.Secondary).SetAlign(tview.AlignLeft))

		// Duration (friendly)
		duration := "N/A"

		if task.StartTime > 0 {
			var endTime int64
			if task.EndTime > 0 {
				endTime = task.EndTime
			} else {
				endTime = time.Now().Unix()
			}

			durationTime := time.Duration(endTime-task.StartTime) * time.Second
			duration = formatDuration(durationTime)
		}

		tl.SetCell(row, 6, tview.NewTableCell(duration).SetTextColor(theme.Colors.Secondary).SetAlign(tview.AlignLeft))
	}

	// Select first task if available
	if len(sortedTasks) > 0 {
		tl.Select(1, 0)
	}
}

// setupKeyHandlers configures vi-style navigation.
func (tl *TasksList) setupKeyHandlers() {
	tl.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}
		}

		return event
	})
}
