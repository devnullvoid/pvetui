package components

import (
	"fmt"
	"sort"
	"time"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/taskmanager"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// TasksList encapsulates the tasks list panel.
type TasksList struct {
	*tview.Flex

	activeTable  *tview.Table
	historyTable *tview.Table
	showActive   bool

	tasks []*api.ClusterTask
	app   *App
}

var _ TasksListComponent = (*TasksList)(nil)

// tasksTableColumns defines the header labels and expansion factors for each
// column. Expansion values control how the table distributes leftover width so
// that the Tasks table spans the entire page.
var tasksTableColumns = []struct {
	title     string
	expansion int
}{
	{title: "Time", expansion: 2},
	{title: "Node", expansion: 2},
	{title: "Type", expansion: 2},
	{title: "Status", expansion: 1},
	{title: "User", expansion: 2},
	{title: "ID", expansion: 3},
	{title: "Duration", expansion: 1},
}

// activeTasksColumns for the active tasks table
var activeTasksColumns = []struct {
	title     string
	expansion int
}{
	{title: "Operation", expansion: 2},
	{title: "Target", expansion: 2},
	{title: "Status", expansion: 1},
	{title: "Started", expansion: 2},
	{title: "UPID", expansion: 3},
}

const (
	historyPanelRatio = 2
	queuePanelRatio   = 1
)

// NewTasksList creates a new tasks list panel.
func NewTasksList() *TasksList {
	activeTable := tview.NewTable()
	activeTable.SetBorders(false)
	activeTable.SetBorder(true)
	activeTable.SetSelectable(true, false)
	activeTable.SetFixed(1, 0)
	activeTable.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary).Attributes(tcell.AttrReverse))

	historyTable := tview.NewTable()
	historyTable.SetBorders(false)
	historyTable.SetTitle(" Task History ")
	historyTable.SetBorder(true)
	historyTable.SetSelectable(true, false)
	historyTable.SetFixed(1, 0)
	historyTable.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary).Attributes(tcell.AttrReverse))

	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	tl := &TasksList{
		Flex:         flex,
		activeTable:  activeTable,
		historyTable: historyTable,
		showActive:   true,
		tasks:        make([]*api.ClusterTask, 0),
	}

	tl.updateActiveTableTitle()
	tl.setupKeyHandlers()
	tl.syncLayout()

	return tl
}

// SetApp sets the application reference.
func (tl *TasksList) SetApp(app *App) {
	tl.app = app
	tl.updateActiveTableTitle()
}

// SetTasks updates the tasks list with new data.
func (tl *TasksList) SetTasks(tasks []*api.ClusterTask) {
	tl.tasks = tasks
	tl.updateHistoryTable()
	// Also refresh active tasks since we are updating
	tl.Refresh()
}

// SetFilteredTasks updates the tasks list with filtered data.
func (tl *TasksList) SetFilteredTasks(tasks []*api.ClusterTask) {
	tl.tasks = tasks
	tl.updateHistoryTable()
}

// GetSelectedTask returns the currently selected task from history table.
func (tl *TasksList) GetSelectedTask() *api.ClusterTask {
	// Only return selected task if history table is focused
	if tl.app != nil && tl.app.GetFocus() == tl.historyTable {
		row, _ := tl.historyTable.GetSelection()
		if row <= 0 || row > len(tl.tasks) {
			return nil
		}
		// Sort logic matches updateHistoryTable
		sortedTasks := make([]*api.ClusterTask, len(tl.tasks))
		copy(sortedTasks, tl.tasks)
		sort.Slice(sortedTasks, func(i, j int) bool {
			return sortedTasks[i].StartTime > sortedTasks[j].StartTime
		})

		if row-1 < len(sortedTasks) {
			return sortedTasks[row-1]
		}
	}
	return nil
}

// Select wraps the table Select method to match the interface.
// Delegates to history table.
func (tl *TasksList) Select(row, column int) *tview.Table {
	return tl.historyTable.Select(row, column)
}

func (tl *TasksList) noTasksCell() *tview.TableCell {
	return tview.NewTableCell("No tasks available").
		SetTextColor(theme.Colors.Warning).
		SetAlign(tview.AlignCenter).
		SetExpansion(1)
}

// Clear clears the tasks list.
func (tl *TasksList) Clear() *tview.Table {
	tl.historyTable.Clear()
	tl.historyTable.SetCell(0, 0, tl.noTasksCell())
	return tl.historyTable
}

// Refresh updates active tasks from TaskManager.
func (tl *TasksList) Refresh() {
	if tl.app == nil || tl.app.TaskManager() == nil {
		return
	}

	tasks := tl.app.TaskManager().GetAllTasks()

	// Update active table
	tl.activeTable.Clear()

	// Headers
	for i, column := range activeTasksColumns {
		tc := tview.NewTableCell(column.title).
			SetTextColor(theme.Colors.HeaderText).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(column.expansion)
		tl.activeTable.SetCell(0, i, tc)
	}

	if len(tasks) > 0 {
		// Sort by created at
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		})

		for i, task := range tasks {
			row := i + 1

			// Operation
			tl.activeTable.SetCell(row, 0, tview.NewTableCell(task.Description).
				SetTextColor(theme.Colors.Primary).
				SetExpansion(activeTasksColumns[0].expansion))

			// Target
			tl.activeTable.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%s (%s)", task.TargetName, task.TargetNode)).
				SetTextColor(theme.Colors.Secondary).
				SetExpansion(activeTasksColumns[1].expansion))

			// Status
			statusColor := theme.Colors.Info
			if task.Status == taskmanager.StatusRunning {
				statusColor = theme.Colors.Success
			} else if task.Status == taskmanager.StatusFailed || task.Status == taskmanager.StatusCancelled {
				statusColor = theme.Colors.Error
			}

			tl.activeTable.SetCell(row, 2, tview.NewTableCell(string(task.Status)).
				SetTextColor(statusColor).
				SetExpansion(activeTasksColumns[2].expansion))

			// Started
			started := "Pending"
			if !task.StartedAt.IsZero() {
				started = task.StartedAt.Format("15:04:05")
			}
			tl.activeTable.SetCell(row, 3, tview.NewTableCell(started).
				SetTextColor(theme.Colors.Secondary).
				SetExpansion(activeTasksColumns[3].expansion))

			// UPID
			tl.activeTable.SetCell(row, 4, tview.NewTableCell(task.UPID).
				SetTextColor(theme.Colors.Tertiary).
				SetExpansion(activeTasksColumns[4].expansion))

			// Store task ID in reference for key handler
			tl.activeTable.GetCell(row, 0).SetReference(task)
		}
	} else {
		tl.activeTable.SetCell(1, 0, tview.NewTableCell("No active operations").
			SetTextColor(theme.Colors.Secondary).
			SetAlign(tview.AlignCenter).
			SetSelectable(false).
			SetExpansion(1))
	}

}

// updateHistoryTable refreshes the history table content.
func (tl *TasksList) updateHistoryTable() {
	tl.historyTable.Clear()

	if len(tl.tasks) == 0 {
		tl.historyTable.SetCell(0, 0, tl.noTasksCell())
		return
	}

	// Sort tasks by start time (newest first)
	sortedTasks := make([]*api.ClusterTask, len(tl.tasks))
	copy(sortedTasks, tl.tasks)
	sort.Slice(sortedTasks, func(i, j int) bool {
		return sortedTasks[i].StartTime > sortedTasks[j].StartTime
	})

	// Set headers
	for i, column := range tasksTableColumns {
		tc := tview.NewTableCell(column.title).
			SetTextColor(theme.Colors.HeaderText).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(column.expansion)
		tl.historyTable.SetCell(0, i, tc)
	}

	// Add task data
	for i, task := range sortedTasks {
		row := i + 1

		startTime := "N/A"
		if task.StartTime > 0 {
			startTime = time.Unix(task.StartTime, 0).Format("01-02-2006 15:04:05")
		}

		tl.historyTable.SetCell(row, 0, tview.NewTableCell(startTime).
			SetTextColor(theme.Colors.Secondary).
			SetAlign(tview.AlignLeft).
			SetExpansion(tasksTableColumns[0].expansion))

		node := task.Node
		if len(node) > 16 {
			node = node[:13] + "..."
		}

		tl.historyTable.SetCell(row, 1, tview.NewTableCell(node).
			SetTextColor(theme.Colors.Primary).
			SetAlign(tview.AlignLeft).
			SetExpansion(tasksTableColumns[1].expansion))

		typeStr := formatTaskType(task.Type)
		if len(typeStr) > 16 {
			typeStr = typeStr[:13] + "..."
		}

		tl.historyTable.SetCell(row, 2, tview.NewTableCell(typeStr).
			SetTextColor(theme.Colors.Info).
			SetAlign(tview.AlignLeft).
			SetExpansion(tasksTableColumns[2].expansion))

		statusCell := createStatusCell(task.Status)
		statusCell.SetExpansion(tasksTableColumns[3].expansion)
		tl.historyTable.SetCell(row, 3, statusCell)

		user := formatUser(task.User)
		if len(user) > 18 {
			user = user[:15] + "..."
		}

		tl.historyTable.SetCell(row, 4, tview.NewTableCell(user).
			SetTextColor(theme.Colors.Tertiary).
			SetAlign(tview.AlignLeft).
			SetExpansion(tasksTableColumns[4].expansion))

		id := task.ID
		if len(id) > 24 {
			id = id[:21] + "..."
		}

		tl.historyTable.SetCell(row, 5, tview.NewTableCell(id).
			SetTextColor(theme.Colors.Secondary).
			SetAlign(tview.AlignLeft).
			SetExpansion(tasksTableColumns[5].expansion))

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

		tl.historyTable.SetCell(row, 6, tview.NewTableCell(duration).
			SetTextColor(theme.Colors.Secondary).
			SetAlign(tview.AlignLeft).
			SetExpansion(tasksTableColumns[6].expansion))
	}

	if len(sortedTasks) > 0 {
		tl.historyTable.Select(1, 0)
	}
}

// setupKeyHandlers configures vi-style navigation.
func (tl *TasksList) setupKeyHandlers() {
	handler := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			if tl.showActive {
				tl.focusOtherPane()
				return nil
			}
		case tcell.KeyBacktab:
			if tl.showActive {
				tl.focusOtherPane()
				return nil
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}
		}
		if keyMatch(event, tl.tasksToggleQueueKeySpec()) {
			tl.toggleActiveQueueVisibility()
			return nil
		}
		return event
	}

	tl.historyTable.SetInputCapture(handler)

	tl.activeTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if keyMatch(event, tl.tasksToggleQueueKeySpec()) {
			tl.toggleActiveQueueVisibility()
			return nil
		}
		if keyMatch(event, tl.taskStopCancelKeySpec()) {
			// Stop task
			row, _ := tl.activeTable.GetSelection()
			if row > 0 {
				cell := tl.activeTable.GetCell(row, 0)
				if ref := cell.GetReference(); ref != nil {
					if task, ok := ref.(*taskmanager.Task); ok {
						if tl.app != nil {
							actionLabel := "Stop"
							successMessage := "Task stop requested"
							if task.Status == taskmanager.StatusQueued {
								actionLabel = "Cancel"
								successMessage = "Queued task cancelled"
							}
							tl.app.showConfirmationDialog(
								fmt.Sprintf("%s task '%s'?", actionLabel, task.Description),
								func() {
									go func() {
										if err := tl.app.TaskManager().CancelTask(task.ID); err != nil {
											tl.app.QueueUpdateDraw(func() {
												tl.app.header.ShowError(fmt.Sprintf("Failed to stop task: %v", err))
											})
										} else {
											tl.app.QueueUpdateDraw(func() {
												tl.app.header.ShowSuccess(successMessage)
											})
										}
									}()
								},
							)
						}
					}
				}
			}
			return nil
		}
		return handler(event)
	})
}

func (tl *TasksList) tasksToggleQueueKeySpec() string {
	if tl.app != nil && tl.app.config.KeyBindings.TasksToggleQueue != "" {
		return tl.app.config.KeyBindings.TasksToggleQueue
	}

	return config.DefaultKeyBindings().TasksToggleQueue
}

func (tl *TasksList) taskStopCancelKeySpec() string {
	if tl.app != nil && tl.app.config.KeyBindings.TaskStopCancel != "" {
		return tl.app.config.KeyBindings.TaskStopCancel
	}

	return config.DefaultKeyBindings().TaskStopCancel
}

func (tl *TasksList) updateActiveTableTitle() {
	tl.activeTable.SetTitle(fmt.Sprintf(
		" Active Operations [%s:toggle] [%s:stop-cancel] ",
		tl.tasksToggleQueueKeySpec(),
		tl.taskStopCancelKeySpec(),
	))
}

func (tl *TasksList) toggleActiveQueueVisibility() {
	tl.showActive = !tl.showActive
	tl.syncLayout()
}

func (tl *TasksList) syncLayout() {
	tl.Flex.Clear()
	tl.Flex.AddItem(tl.historyTable, 0, historyPanelRatio, true)

	if tl.showActive {
		tl.Flex.AddItem(tl.activeTable, 0, queuePanelRatio, false)
	}

	if tl.app != nil && !tl.showActive && tl.app.GetFocus() == tl.activeTable {
		tl.app.SetFocus(tl.historyTable)
	}
}

func (tl *TasksList) focusOtherPane() {
	if tl.app == nil || !tl.showActive {
		return
	}

	current := tl.app.GetFocus()
	if current == tl.activeTable {
		tl.app.SetFocus(tl.historyTable)
		return
	}
	if current == tl.historyTable {
		tl.app.SetFocus(tl.activeTable)
	}
}
