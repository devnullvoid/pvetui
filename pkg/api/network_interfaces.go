package api

import (
	"fmt"
	"strings"
)

// NetworkInterfaceStatistics represents network interface statistics from QEMU guest agent
type NetworkInterfaceStatistics struct {
	RxBytes   int64 `json:"rx-bytes"`
	RxDropped int64 `json:"rx-dropped"`
	RxErrors  int64 `json:"rx-errs"`
	RxPackets int64 `json:"rx-packets"`
	TxBytes   int64 `json:"tx-bytes"`
	TxDropped int64 `json:"tx-dropped"`
	TxErrors  int64 `json:"tx-errs"`
	TxPackets int64 `json:"tx-packets"`
}

// IPAddress represents an IP address from QEMU guest agent
type IPAddress struct {
	Address string `json:"ip-address"`
	Type    string `json:"ip-address-type"` // ipv4 or ipv6
	Prefix  int    `json:"prefix"`
}

// NetworkInterface represents a network interface from QEMU guest agent
type NetworkInterface struct {
	Name        string                     `json:"name"`
	MACAddress  string                     `json:"hardware-address"`
	IPAddresses []IPAddress                `json:"ip-addresses"`
	Statistics  NetworkInterfaceStatistics `json:"statistics"`
	IsLoopback  bool                       `json:"-"` // Determined by name (lo)
}

// GetGuestAgentInterfaces retrieves network interface information from the QEMU guest agent
func (c *Client) GetGuestAgentInterfaces(vm *VM) ([]NetworkInterface, error) {
	if vm.Type != VMTypeQemu || vm.Status != VMStatusRunning {
		return nil, fmt.Errorf("guest agent not applicable for this VM type or status")
	}

	var res map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", vm.Node, vm.ID)

	if err := c.GetWithCache(endpoint, &res, VMDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get network interfaces from guest agent: %w", err)
	}

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format from guest agent")
	}

	resultArray, ok := data["result"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result format from guest agent")
	}

	var interfaces []NetworkInterface

	for _, iface := range resultArray {
		ifaceMap, ok := iface.(map[string]interface{})
		if !ok {
			continue
		}

		netInterface := NetworkInterface{}

		// Get interface name and MAC address
		if name, ok := ifaceMap["name"].(string); ok {
			netInterface.Name = name
			netInterface.IsLoopback = name == "lo" || strings.HasPrefix(name, "lo:")
		}

		if mac, ok := ifaceMap["hardware-address"].(string); ok {
			netInterface.MACAddress = mac
		}

		// Parse IP addresses
		if ipAddresses, ok := ifaceMap["ip-addresses"].([]interface{}); ok {
			for _, ipData := range ipAddresses {
				ipMap, ok := ipData.(map[string]interface{})
				if !ok {
					continue
				}

				ipAddress := IPAddress{}

				if addr, ok := ipMap["ip-address"].(string); ok {
					ipAddress.Address = addr
				}

				if ipType, ok := ipMap["ip-address-type"].(string); ok {
					ipAddress.Type = ipType
				}

				if prefix, ok := ipMap["prefix"].(float64); ok {
					ipAddress.Prefix = int(prefix)
				}

				netInterface.IPAddresses = append(netInterface.IPAddresses, ipAddress)
			}
		}

		// Parse statistics
		if stats, ok := ifaceMap["statistics"].(map[string]interface{}); ok {
			if rxBytes, ok := stats["rx-bytes"].(float64); ok {
				netInterface.Statistics.RxBytes = int64(rxBytes)
			}
			if rxDropped, ok := stats["rx-dropped"].(float64); ok {
				netInterface.Statistics.RxDropped = int64(rxDropped)
			}
			if rxErrs, ok := stats["rx-errs"].(float64); ok {
				netInterface.Statistics.RxErrors = int64(rxErrs)
			}
			if rxPackets, ok := stats["rx-packets"].(float64); ok {
				netInterface.Statistics.RxPackets = int64(rxPackets)
			}
			if txBytes, ok := stats["tx-bytes"].(float64); ok {
				netInterface.Statistics.TxBytes = int64(txBytes)
			}
			if txDropped, ok := stats["tx-dropped"].(float64); ok {
				netInterface.Statistics.TxDropped = int64(txDropped)
			}
			if txErrs, ok := stats["tx-errs"].(float64); ok {
				netInterface.Statistics.TxErrors = int64(txErrs)
			}
			if txPackets, ok := stats["tx-packets"].(float64); ok {
				netInterface.Statistics.TxPackets = int64(txPackets)
			}
		}

		interfaces = append(interfaces, netInterface)
	}

	return interfaces, nil
}

// GetLxcInterfaces retrieves network interface information for an LXC container.
func (c *Client) GetLxcInterfaces(vm *VM) ([]NetworkInterface, error) {
	if vm.Type != VMTypeLXC || vm.Status != VMStatusRunning {
		return nil, fmt.Errorf("network interface endpoint not applicable for this guest type or status")
	}

	var apiResponse map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", vm.Node, vm.ID)

	if err := c.GetWithCache(endpoint, &apiResponse, VMDataTTL); err != nil {
		// Based on previous handling, API might return 500 if feature not available or container stopped.
		// Treat this as "no interfaces found" rather than a hard error for GetVmStatus.
		c.logger.Debug("Failed to get LXC interfaces for VM %d on node %s (may be expected): %v", vm.ID, vm.Node, err)
		return nil, nil
	}

	responseData, ok := apiResponse["data"].([]interface{})
	if !ok {
		// It's possible this specific endpoint returns the array directly without a "data" wrapper.
		// Let's try to type assert apiResponse itself to []interface{}. This is unusual for Proxmox API.
		// config.DebugLog("LXC interfaces: 'data' key not found or not an array. Full response: %+v", apiResponse)
		// This part is tricky. If the user-provided example `[{"hwaddr":...}]` is accurate for the *entire* response body,
		// then `c.Get` which expects `*map[string]interface{}` will fail to unmarshal it directly.
		// The user's confidence in `c.Get` working suggests the API *does* conform, or their example was just the *value* of the "data" field.
		// For now, assuming the standard `{"data": [...]}` structure based on other working code.
		// If it still fails, this is where the GetRaw approach or direct HTTP call would be needed if the endpoint is truly different.
		return nil, fmt.Errorf("unexpected response format for LXC interfaces: 'data' field missing or not an array. VM: %d, Node: %s", vm.ID, vm.Node)
	}

	var interfaces []NetworkInterface
	for _, ifaceDataItem := range responseData {
		ifaceMap, ok := ifaceDataItem.(map[string]interface{})
		if !ok {
			c.logger.Debug("LXC interface item is not a map[string]interface{}: %+v", ifaceDataItem)
			continue
		}

		netInterface := NetworkInterface{}
		if name, ok := ifaceMap["name"].(string); ok {
			netInterface.Name = name
			netInterface.IsLoopback = (name == "lo")
		}
		if hwaddr, ok := ifaceMap["hwaddr"].(string); ok {
			netInterface.MACAddress = hwaddr
		}

		var ipAddresses []IPAddress
		if inet, ok := ifaceMap["inet"].(string); ok {
			if ip, valid := parseIPCIDR(inet, IPTypeIPv4); valid {
				ipAddresses = append(ipAddresses, ip)
			}
		}
		if inet6, ok := ifaceMap["inet6"].(string); ok {
			if ip, valid := parseIPCIDR(inet6, IPTypeIPv6); valid {
				ipAddresses = append(ipAddresses, ip)
			}
		}
		netInterface.IPAddresses = ipAddresses
		interfaces = append(interfaces, netInterface)
	}

	return interfaces, nil
}

// GetFirstNonLoopbackIP returns the first non-loopback IP address from network interfaces
func GetFirstNonLoopbackIP(interfaces []NetworkInterface, preferIPv4 bool) string {
	// First look for preferred IP version
	for _, iface := range interfaces {
		if iface.IsLoopback {
			continue
		}

		for _, ip := range iface.IPAddresses {
			if preferIPv4 && ip.Type == IPTypeIPv4 {
				return ip.Address
			} else if !preferIPv4 && ip.Type == IPTypeIPv6 {
				return ip.Address
			}
		}
	}

	// If not found, look for any non-loopback IP
	for _, iface := range interfaces {
		if iface.IsLoopback {
			continue
		}

		for _, ip := range iface.IPAddresses {
			return ip.Address
		}
	}

	return ""
}

// parseIPCIDR parses an IP address string with CIDR notation.
func parseIPCIDR(ipCIDR string, ipType string) (IPAddress, bool) {
	if ipCIDR == "" {
		return IPAddress{}, false
	}
	parts := strings.Split(ipCIDR, "/")
	if len(parts) == 0 {
		return IPAddress{}, false
	}

	ipAddr := IPAddress{Type: ipType}
	ipAddr.Address = parts[0]

	if len(parts) == 2 {
		prefix, err := parseInt(parts[1]) // Assuming you have a helper like strconv.Atoi or similar
		if err == nil {
			ipAddr.Prefix = prefix
		}
		// Could log an error here if prefix parsing fails but IP is present
		// For now, we still consider it a valid IP, just without a prefix
	}
	return ipAddr, true
}

// Helper function to parse int, assuming it might be missing in this context
// For a real scenario, use strconv.Atoi
func parseInt(s string) (int, error) {
	var i int
	n, err := fmt.Sscan(s, &i)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("failed to parse int: %s", s)
	}
	return i, nil
}
