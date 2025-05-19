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

// Extract IP addresses from config data
func getIPAddresses(config map[string]interface{}) []string {
	var ips []string
	
	// Look for net0, net1, etc. in config
	for k, v := range config {
		if !strings.HasPrefix(k, "net") {
			continue
		}
		
		netStr, ok := v.(string)
		if !ok {
			continue
		}
		
		// Parse IP from config string like "virtio=XX:XX:XX:XX:XX:XX,bridge=vmbr0,ip=192.168.1.100/24"
		parts := strings.Split(netStr, ",")
		for _, part := range parts {
			if strings.HasPrefix(part, "ip=") {
				ip := strings.TrimPrefix(part, "ip=")
				// Remove subnet mask if present
				if idx := strings.Index(ip, "/"); idx > 0 {
					ip = ip[:idx]
				}
				ips = append(ips, ip)
				break
			}
		}
	}
	
	return ips
}
