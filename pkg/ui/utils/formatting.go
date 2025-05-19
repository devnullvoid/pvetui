package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
)

// FormatUptime formats the uptime in seconds to a human-readable format
func FormatUptime(uptime int) string {
	if uptime <= 0 {
		return "N/A"
	}

	duration := time.Duration(uptime) * time.Second
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// StatusColor returns a color based on the status string
func StatusColor(status string) tcell.Color {
	status = strings.ToLower(status)
	switch status {
	case "running":
		return tcell.ColorGreen
	case "stopped":
		return tcell.ColorRed
	case "paused":
		return tcell.ColorYellow
	default:
		return tcell.ColorWhite
	}
}

// FormatNodeName formats a node name with status indicator
func FormatNodeName(node *api.Node) string {
	if node == nil {
		return "Unknown Node"
	}
	
	status := "🔴"
	if node.Online {
		status = "🟢"
	}
	
	return fmt.Sprintf("%s %s", status, node.Name)
} 