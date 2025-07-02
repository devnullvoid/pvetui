package components

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// TasksList represents the tasks list component
type TasksList struct {
	*tview.Table
	tasks []*api.ClusterTask
	app   *App
}

var _ TasksListComponent = (*TasksList)(nil)

// NewTasksList creates a new tasks list component
func NewTasksList() *TasksList {
	table := tview.NewTable()
	table.SetBorder(true)
	table.SetTitle(" Cluster Tasks ")
	table.SetSelectable(true, false)

	// Set up table headers
	headers := []string{"Time", "Node", "Type", "Status", "User", "ID", "Duration"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		table.SetCell(0, i, cell)
	}

	// Configure table appearance
	table.SetFixed(1, 0) // Fix the header row
	table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite))

	tl := &TasksList{
		Table: table,
		tasks: make([]*api.ClusterTask, 0),
	}

	// Set up keyboard navigation
	tl.setupKeyHandlers()

	return tl
}

// Select wraps the table Select method to match the interface
func (tl *TasksList) Select(row, column int) *tview.Table {
	return tl.Table.Select(row, column)
}

// SetApp sets the parent app reference
func (tl *TasksList) SetApp(app *App) {
	tl.app = app
}

// SetTasks updates the tasks list and global state
func (tl *TasksList) SetTasks(tasks []*api.ClusterTask) {
	// Update global state
	models.GlobalState.OriginalTasks = make([]*api.ClusterTask, len(tasks))
	models.GlobalState.FilteredTasks = make([]*api.ClusterTask, len(tasks))
	copy(models.GlobalState.OriginalTasks, tasks)
	copy(models.GlobalState.FilteredTasks, tasks)

	// Apply current filter if any
	if state := models.GlobalState.GetSearchState(api.PageTasks); state != nil && state.Filter != "" {
		models.FilterTasks(state.Filter)
		tl.tasks = models.GlobalState.FilteredTasks
	} else {
		tl.tasks = tasks
	}

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

// GetTasks returns the current tasks list
func (tl *TasksList) GetTasks() []*api.ClusterTask {
	return tl.tasks
}

// SetFilteredTasks updates the tasks list with filtered data
func (tl *TasksList) SetFilteredTasks(tasks []*api.ClusterTask) {
	tl.tasks = tasks
	tl.updateTable()
}

// updateTable refreshes the table content
func (tl *TasksList) updateTable() {
	// Clear existing rows (except header)
	for row := tl.GetRowCount() - 1; row > 0; row-- {
		tl.RemoveRow(row)
	}

	// Add task rows
	for i, task := range tl.tasks {
		row := i + 1 // +1 because row 0 is the header

		// Format start time
		startTime := time.Unix(task.StartTime, 0)
		timeStr := startTime.Format("01-02 15:04:05")

		// Calculate duration
		var durationStr string
		if task.EndTime > 0 {
			duration := time.Unix(task.EndTime, 0).Sub(startTime)
			durationStr = formatDuration(duration)
		} else {
			// Task is still running
			duration := time.Since(startTime)
			durationStr = formatDuration(duration) + " (running)"
		}

		// Format status with color
		statusCell := tl.createStatusCell(task.Status)

		// Create cells
		cells := []*tview.TableCell{
			tview.NewTableCell(timeStr).SetAlign(tview.AlignLeft),
			tview.NewTableCell(task.Node).SetAlign(tview.AlignLeft),
			tview.NewTableCell(formatTaskType(task.Type)).SetAlign(tview.AlignLeft),
			statusCell,
			tview.NewTableCell(formatUser(task.User)).SetAlign(tview.AlignLeft),
			tview.NewTableCell(task.ID).SetAlign(tview.AlignLeft),
			tview.NewTableCell(durationStr).SetAlign(tview.AlignLeft),
		}

		// Add cells to table
		for col, cell := range cells {
			tl.SetCell(row, col, cell)
		}
	}

	// Select first task if available
	if len(tl.tasks) > 0 {
		tl.Select(1, 0) // Select first data row
	}
}

// createStatusCell creates a colored status cell
func (tl *TasksList) createStatusCell(status string) *tview.TableCell {
	var color tcell.Color
	switch status {
	case "OK":
		color = tcell.ColorGreen
	case "stopped":
		color = tcell.ColorRed
	case "running":
		color = tcell.ColorYellow
	default:
		if status == "job errors" || status == "error" {
			color = tcell.ColorRed
		} else {
			color = tcell.ColorWhite
		}
	}

	return tview.NewTableCell(status).
		SetTextColor(color).
		SetAlign(tview.AlignLeft)
}

// formatTaskType formats the task type for display
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
	case "vzdump":
		return "Backup"
	case "pvestatd":
		return "Statistics"
	case "spiceproxy":
		return "SPICE Proxy"
	case "vncproxy":
		return "VNC Proxy"
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
	case "vzrestart":
		return "CT Restart"
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

// formatUser formats the user for display
func formatUser(user string) string {
	// Remove @pam, @pve suffixes for cleaner display
	if len(user) > 4 {
		if user[len(user)-4:] == "@pam" {
			return user[:len(user)-4]
		}
		if user[len(user)-4:] == "@pve" {
			return user[:len(user)-4]
		}
	}
	return user
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// setupKeyHandlers configures keyboard navigation
func (tl *TasksList) setupKeyHandlers() {
	tl.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j': // VI-like down navigation
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k': // VI-like up navigation
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'r', 'R': // Refresh tasks
				return event
			}
		}
		return event
	})
}
