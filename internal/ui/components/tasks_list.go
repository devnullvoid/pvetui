package components

import (
	"fmt"
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
	tasks []*api.ClusterTask
	app   *App
}

var _ TasksListComponent = (*TasksList)(nil)

// NewTasksList creates a new tasks list panel
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

	// Set headers: Time, Node, Type, Status, User, ID, Duration
	headers := []string{"Time", "Node", "Type", "Status", "User", "ID", "Duration"}
	for i, header := range headers {
		tc := tview.NewTableCell(header).
			SetTextColor(theme.Colors.Primary).
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
			startTime = time.Unix(task.StartTime, 0).Format("01-02 15:04:05")
		}
		tl.SetCell(row, 0, tview.NewTableCell(startTime).SetTextColor(theme.Colors.Secondary).SetAlign(tview.AlignLeft))

		// Node (truncate if too long)
		node := task.Node
		if len(node) > 16 {
			node = node[:13] + "..."
		}
		tl.SetCell(row, 1, tview.NewTableCell(node).SetTextColor(theme.Colors.Secondary).SetAlign(tview.AlignLeft))

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
		tl.SetCell(row, 4, tview.NewTableCell(user).SetTextColor(theme.Colors.Info).SetAlign(tview.AlignLeft))

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

// createStatusCell creates a colored status cell (theme-aware)
func createStatusCell(status string) *tview.TableCell {
	var color tcell.Color
	switch strings.ToLower(status) {
	case "ok", "success", "running":
		color = theme.Colors.StatusRunning
	case "stopped", "failed":
		color = theme.Colors.StatusStopped
	case "pending":
		color = theme.Colors.StatusPending
	case "error", "job errors":
		color = theme.Colors.StatusError
	default:
		color = theme.Colors.Primary
	}
	return tview.NewTableCell(status).
		SetTextColor(color).
		SetAlign(tview.AlignLeft)
}

// formatTaskType returns a friendly name for a task type
func formatTaskType(taskType string) string {
	switch taskType {
	case "qmstart":
		return "VM Start"
	case "qmstop":
		return "VM Stop"
	case "qmrestart":
		return "VM Restart"
	case "qmshutdown":
		return "VM Shutdown"
	case "qmreset":
		return "VM Reset"
	case "qmreboot":
		return "VM Reboot"
	case "qmcreate":
		return "VM Create"
	case "qmdestroy":
		return "VM Delete"
	case "qmclone":
		return "VM Clone"
	case "qmmigrate":
		return "VM Migrate"
	case "qmrestore":
		return "VM Restore"
	case "qmtemplate":
		return "VM Template"
	case "qmconfig":
		return "VM Config"
	case "qmresize":
		return "VM Resize"
	case "resize":
		return "Resiza Volume"
	case "vzdump":
		return "Backup"
	case "pvestatd":
		return "Statistics"
	case "spiceproxy":
		return "SPICE Proxy"
	case "vncproxy":
		return "VNC Proxy"
	case "vncshell":
		return "VNC Shell"
	case "pct":
		return "Container"
	case "pctstart":
		return "CT Start"
	case "pctstop":
		return "CT Stop"
	case "pctrestart":
		return "CT Restart"
	case "pctshutdown":
		return "CT Shutdown"
	case "pctcreate":
		return "CT Create"
	case "pctdestroy":
		return "CT Delete"
	case "pctclone":
		return "CT Clone"
	case "pctmigrate":
		return "CT Migrate"
	case "pctrestore":
		return "CT Restore"
	case "vzcreate":
		return "CT Create"
	case "vzstart":
		return "CT Start"
	case "vzstop":
		return "CT Stop"
	case "vzdestroy":
		return "CT Delete"
	case "vzreboot":
		return "CT Reboot"
	case "vzshutdown":
		return "CT Shutdown"
	case "vzclone":
		return "CT Clone"
	case "vzmigrate":
		return "CT Migrate"
	case "vzrestore":
		return "CT Restore"
	case "vztemplate":
		return "CT Template"
	case "aptupdate":
		return "APT Update"
	case "aptupgrade":
		return "APT Upgrade"
	case "startall":
		return "Start All VMs"
	case "stopall":
		return "Stop All VMs"
	case "srvstart":
		return "Service Start"
	case "srvstop":
		return "Service Stop"
	case "srvrestart":
		return "Service Restart"
	case "srvreload":
		return "Service Reload"
	case "imgcopy":
		return "Image Copy"
	case "imgdel":
		return "Image Delete"
	case "download":
		return "Download"
	case "upload":
		return "Upload"
	default:
		return taskType
	}
}

// formatUser cleans up the user string for display
func formatUser(user string) string {
	if len(user) > 4 {
		if user[len(user)-4:] == "@pam" || user[len(user)-4:] == "@pve" {
			return user[:len(user)-4]
		}
	}
	return user
}

// formatDuration returns a friendly duration string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// setupKeyHandlers configures vi-style navigation
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
