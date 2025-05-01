package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// toFloat converts various types to float64
func toFloat(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return f
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		// Try to convert to JSON and parse as number
		if data, err := json.Marshal(v); err == nil {
			var f float64
			if err := json.Unmarshal(data, &f); err == nil {
				return f
			}
		}
		// Log conversion error
		fmt.Printf("Warning: could not convert %T to float64\n", v)
		return 0
	}
}

// prettyJSON formats a value as indented JSON for display
func prettyJSON(val interface{}) string {
	data, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(data)
}
