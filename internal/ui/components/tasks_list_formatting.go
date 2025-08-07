package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
)

// createStatusCell creates a colored status cell (theme-aware).
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

// formatTaskType returns a friendly name for a task type.
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
	case "vzrollback":
		return "CT Rollback"
	case "vzdelsnapshot":
		return "CT Delete Snapshot"
	case "vzsnapshot":
		return "CT Snapshot"
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

// formatUser cleans up the user string for display.
func formatUser(user string) string {
	if len(user) > 4 {
		if user[len(user)-4:] == "@pam" || user[len(user)-4:] == "@pve" {
			return user[:len(user)-4]
		}
	}

	return user
}

// formatDuration returns a friendly duration string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
