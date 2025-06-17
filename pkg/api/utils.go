package api

import (
	"fmt"
	"strconv"
	"strings"
)

// Helper functions to extract values from interface{} maps safely
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloat(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			// Try to convert string to float
			var f float64
			if n, err := fmt.Sscanf(v, "%f", &f); err == nil && n > 0 {
				return f
			}
		}
	}
	return 0
}

func getBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case int:
			return v != 0
		case float64:
			return v != 0
		case string:
			return v == "1" || strings.EqualFold(v, "true")
		}
	}
	return false
}

func getInt(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			var i int
			if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
				return i
			}
		}
	}
	return 0
}

// FormatBytes formats bytes into human-readable format
func FormatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", int64(size), units[unitIndex])
	}

	// Format with one decimal place for larger units
	if size == float64(int64(size)) {
		return fmt.Sprintf("%.1f %s", size, units[unitIndex])
	}
	return fmt.Sprintf("%.1f %s", size, units[unitIndex])
}

// FormatUptime formats seconds into human-readable uptime format
func FormatUptime(seconds int64) string {
	if seconds == 0 {
		return "0s"
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	var parts []string

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if secs > 0 {
		parts = append(parts, fmt.Sprintf("%ds", secs))
	}

	return strings.Join(parts, " ")
}

// ParseVMID parses a VM ID from various input types
func ParseVMID(input interface{}) (int, error) {
	switch v := input.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	case nil:
		return 0, fmt.Errorf("VMID cannot be nil")
	default:
		return 0, fmt.Errorf("invalid VMID type: %T", input)
	}
}

// SafeStringValue safely converts interface{} to string
func SafeStringValue(input interface{}) string {
	if input == nil {
		return ""
	}

	switch v := input.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// SafeFloatValue safely converts interface{} to float64
func SafeFloatValue(input interface{}) float64 {
	switch v := input.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0.0
}

// SafeBoolValue safely converts interface{} to bool
func SafeBoolValue(input interface{}) bool {
	switch v := input.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case int:
		return v != 0
	case float64:
		return v != 0.0
	}
	return false
}
