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

// FormatBytes formats a byte count to a human-readable format with dynamic units
// Always shows 2 decimal places and chooses the most appropriate unit (GB, TB, PB)
func FormatBytes(bytes int64) string {
	const (
		GB = 1024 * 1024 * 1024
		TB = 1024 * GB
		PB = 1024 * TB
	)

	bytesFloat := float64(bytes)

	switch {
	case bytes >= PB:
		return fmt.Sprintf("%.2f PB", bytesFloat/PB)
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", bytesFloat/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", bytesFloat/GB)
	default:
		// For values less than 1GB, still show in GB with decimals
		return fmt.Sprintf("%.2f GB", bytesFloat/GB)
	}
}

// FormatBytesFloat converts float64 GB values to human-readable format
// Input is assumed to be in GB, converts to appropriate units with 2 decimal places
func FormatBytesFloat(gb float64) string {
	const (
		TB_IN_GB = 1024
		PB_IN_GB = 1024 * 1024
	)

	switch {
	case gb >= PB_IN_GB:
		return fmt.Sprintf("%.2f PB", gb/PB_IN_GB)
	case gb >= TB_IN_GB:
		return fmt.Sprintf("%.2f TB", gb/TB_IN_GB)
	default:
		return fmt.Sprintf("%.2f GB", gb)
	}
}

// FormatStatusIndicator returns a string with a colored status emoji.
// Green ▲ for online/running, Red ▼ for offline/stopped, Yellow ● for others.
func FormatStatusIndicator(status string) string {
	status = strings.ToLower(status)
	switch status {
	case "running", "online":
		return "[green]▲[-] "
	case "stopped", "offline":
		return "[red]▼[-] "
	default:
		return "[yellow]●[-] "
	}
}
