package utils

import (
	"fmt"
	"strings"
	"time"
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

// FormatBytes formats a byte count to a human-readable format (B, KB, MB, GB, TB)
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), []string{"KB", "MB", "GB", "TB"}[exp])
}

// FormatStatusIndicator returns a string with a colored status emoji.
// Green ▲ for online/running, Red ▼ for offline/stopped, Yellow ▪ for others.
func FormatStatusIndicator(status string) string {
	status = strings.ToLower(status)
	switch status {
	case "running", "online":
		return "[green]▲[white] "
	case "stopped", "offline":
		return "[red]▼[white] "
	default:
		return "[yellow]▪[white] "
	}
}
