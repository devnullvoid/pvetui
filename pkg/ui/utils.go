package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-tui/pkg/api"
)

// StatusColor returns color based on VM status
func StatusColor(status string) tcell.Color {
	switch status {
	case "running":
		return tcell.ColorGreen
	case "stopped":
		return tcell.ColorRed
	default:
		return tcell.ColorYellow
	}
}

// FormatNodeName adds status emoji to node names
func FormatNodeName(node api.Node) string {
	if node.Online {
		return "🟢 " + node.Name
	}
	return node.Name
}

// FormatUptime converts seconds to human-readable duration
func FormatUptime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	minutes %= 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := hours / 24
	hours %= 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
