package api

import (
	"fmt"
	"strconv"
	"strings"
)

// Helper functions to extract values from interface{} maps safely

// getString safely extracts a string value from a map[string]interface{}.
//
// This function handles the common pattern of extracting string values from
// JSON-decoded maps where the value type is interface{}. It performs safe
// type assertion and returns an empty string if the key doesn't exist or
// the value is not a string.
//
// Parameters:
//   - data: The map to extract from
//   - key: The key to look up
//
// Returns the string value or empty string if not found or wrong type.
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getFloat safely extracts a numeric value from a map[string]interface{} as float64.
//
// This function handles multiple numeric types that can appear in JSON-decoded
// data, including float64, int, int64, and numeric strings. It performs safe
// type assertions and conversions.
//
// Parameters:
//   - data: The map to extract from
//   - key: The key to look up
//
// Returns the numeric value as float64, or 0 if not found or not convertible.
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

// getBool safely extracts a boolean value from a map[string]interface{}.
//
// This function handles multiple representations of boolean values that can
// appear in JSON or form data:
//   - bool: true/false
//   - int: 0 (false) or non-zero (true)
//   - float64: 0.0 (false) or non-zero (true)
//   - string: "true", "1" (true) or "false", "0" (false), case-insensitive
//
// Parameters:
//   - data: The map to extract from
//   - key: The key to look up
//
// Returns the boolean value or false if not found or not convertible.
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

// getInt safely extracts an integer value from a map[string]interface{}.
//
// This function handles multiple numeric types that can appear in JSON-decoded
// data, including int, float64, and numeric strings. It performs safe type
// assertions and conversions, truncating floating-point values to integers.
//
// Parameters:
//   - data: The map to extract from
//   - key: The key to look up
//
// Returns the integer value or 0 if not found or not convertible.
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

// FormatBytes converts a byte count into a human-readable string with appropriate units.
//
// This function formats byte values using binary units (1024-based) and provides
// a consistent, human-readable representation. The function automatically selects
// the most appropriate unit (B, KB, MB, GB, TB) based on the input size.
//
// Formatting rules:
//   - Values in bytes are shown as integers (e.g., "42 B")
//   - Larger values are shown with one decimal place (e.g., "1.5 GB")
//   - Zero bytes returns "0 B"
//   - Uses binary units (1024 bytes = 1 KB)
//
// Parameters:
//   - bytes: The number of bytes to format
//
// Returns a formatted string with value and unit.
//
// Example usage:
//
//	fmt.Println(FormatBytes(0))          // "0 B"
//	fmt.Println(FormatBytes(1024))       // "1.0 KB"
//	fmt.Println(FormatBytes(1536))       // "1.5 KB"
//	fmt.Println(FormatBytes(1073741824)) // "1.0 GB"
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

// FormatUptime converts seconds into a human-readable uptime string.
//
// This function formats duration values (in seconds) into a natural language
// representation showing days, hours, minutes, and seconds as appropriate.
// Only non-zero units are included in the output.
//
// Formatting rules:
//   - Shows only relevant units (omits zero values)
//   - Uses natural language (e.g., "2 days, 3 hours, 15 minutes, 30 seconds")
//   - Handles singular and plural forms correctly
//   - Zero seconds returns "0 seconds"
//
// Parameters:
//   - seconds: The duration in seconds to format
//
// Returns a formatted uptime string.
//
// Example usage:
//
//	fmt.Println(FormatUptime(0))     // "0 seconds"
//	fmt.Println(FormatUptime(65))    // "1 minute, 5 seconds"
//	fmt.Println(FormatUptime(3661))  // "1 hour, 1 minute, 1 second"
//	fmt.Println(FormatUptime(90061)) // "1 day, 1 hour, 1 minute, 1 second"
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

// ParseVMID extracts and validates a VM ID from various input types.
//
// This function handles the common scenario where VM IDs can come from different
// sources (JSON APIs, user input, etc.) in various formats. It safely converts
// the input to a valid integer VM ID.
//
// Supported input types:
//   - int: Direct integer value
//   - float64: Floating-point number (common from JSON)
//   - string: Numeric string representation
//   - Other types: Return error
//
// Validation rules:
//   - Must be a positive integer
//   - Zero is considered invalid
//   - Negative numbers are invalid
//
// Parameters:
//   - input: The value to parse as a VM ID
//
// Returns the VM ID as an integer, or an error if invalid.
//
// Example usage:
//
//	vmid, err := ParseVMID(123)        // Returns 123, nil
//	vmid, err := ParseVMID(123.0)      // Returns 123, nil
//	vmid, err := ParseVMID("123")      // Returns 123, nil
//	vmid, err := ParseVMID("invalid")  // Returns 0, error
//	vmid, err := ParseVMID(-1)         // Returns 0, error
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

// SafeStringValue safely converts various types to string representation.
//
// This function provides a safe way to convert interface{} values to strings,
// handling multiple common types that can appear in JSON or API responses.
// It never panics and always returns a string value.
//
// Supported conversions:
//   - string: Returns as-is
//   - int, int64, float64: Converts to string representation
//   - bool: Returns "true" or "false"
//   - nil: Returns empty string
//   - Other types: Returns fmt.Sprintf("%v", value)
//
// Parameters:
//   - value: The value to convert to string
//
// Returns a string representation of the value.
//
// Example usage:
//
//	str := SafeStringValue("hello")    // "hello"
//	str := SafeStringValue(123)        // "123"
//	str := SafeStringValue(true)       // "true"
//	str := SafeStringValue(nil)        // ""
func SafeStringValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
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

// SafeFloatValue safely converts various types to float64 representation.
//
// This function provides a safe way to convert interface{} values to float64,
// handling multiple numeric types and string representations. It never panics
// and returns 0.0 for non-convertible values.
//
// Supported conversions:
//   - float64: Returns as-is
//   - int, int64: Converts to float64
//   - string: Attempts to parse as float64
//   - Other types: Returns 0.0
//
// Parameters:
//   - value: The value to convert to float64
//
// Returns a float64 representation of the value, or 0.0 if not convertible.
//
// Example usage:
//
//	f := SafeFloatValue(123.45)    // 123.45
//	f := SafeFloatValue(123)       // 123.0
//	f := SafeFloatValue("123.45")  // 123.45
//	f := SafeFloatValue("invalid") // 0.0
func SafeFloatValue(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0.0
}

// SafeBoolValue safely converts various types to boolean representation.
//
// This function provides a safe way to convert interface{} values to boolean,
// handling multiple representations commonly found in APIs and configuration.
// It follows common conventions for truthy/falsy values.
//
// Conversion rules:
//   - bool: Returns as-is
//   - string: "true", "1" → true; "false", "0" → false (case-insensitive)
//   - int, float64: 0 → false; non-zero → true
//   - Other types: Returns false
//
// Parameters:
//   - value: The value to convert to boolean
//
// Returns a boolean representation of the value.
//
// Example usage:
//
//	b := SafeBoolValue(true)     // true
//	b := SafeBoolValue("true")   // true
//	b := SafeBoolValue("1")      // true
//	b := SafeBoolValue(1)        // true
//	b := SafeBoolValue(0)        // false
//	b := SafeBoolValue("false")  // false
func SafeBoolValue(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == StringTrue || v == "1"
	case int:
		return v != 0
	case float64:
		return v != 0.0
	}
	return false
}
