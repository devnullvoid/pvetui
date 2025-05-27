package api

import (
	"fmt"
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
