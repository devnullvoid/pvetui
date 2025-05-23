package api

import (
	"fmt"
	"strings"
)

// NetworkInterfaceStatistics represents network interface statistics from QEMU guest agent
type NetworkInterfaceStatistics struct {
	RxBytes    int64 `json:"rx-bytes"`
	RxDropped  int64 `json:"rx-dropped"`
	RxErrors   int64 `json:"rx-errs"`
	RxPackets  int64 `json:"rx-packets"`
	TxBytes    int64 `json:"tx-bytes"`
	TxDropped  int64 `json:"tx-dropped"`
	TxErrors   int64 `json:"tx-errs"`
	TxPackets  int64 `json:"tx-packets"`
}

// IPAddress represents an IP address from QEMU guest agent
type IPAddress struct {
	Address string `json:"ip-address"`
	Type    string `json:"ip-address-type"` // ipv4 or ipv6
	Prefix  int    `json:"prefix"`
}

// NetworkInterface represents a network interface from QEMU guest agent
type NetworkInterface struct {
	Name            string                   `json:"name"`
	MACAddress      string                   `json:"hardware-address"`
	IPAddresses     []IPAddress              `json:"ip-addresses"`
	Statistics      NetworkInterfaceStatistics `json:"statistics"`
	IsLoopback      bool                     `json:"-"` // Determined by name (lo)
}

// GetGuestAgentInterfaces retrieves network interface information from the QEMU guest agent
func (c *Client) GetGuestAgentInterfaces(vm *VM) ([]NetworkInterface, error) {
	if vm.Type != "qemu" || vm.Status != "running" {
		return nil, fmt.Errorf("guest agent not applicable for this VM type or status")
	}

	var res map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", vm.Node, vm.ID)
	
	if err := c.Get(endpoint, &res); err != nil {
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

// GetFirstNonLoopbackIP returns the first non-loopback IP address from network interfaces
func GetFirstNonLoopbackIP(interfaces []NetworkInterface, preferIPv4 bool) string {
	// First look for preferred IP version
	for _, iface := range interfaces {
		if iface.IsLoopback {
			continue
		}
		
		for _, ip := range iface.IPAddresses {
			if preferIPv4 && ip.Type == "ipv4" {
				return ip.Address
			} else if !preferIPv4 && ip.Type == "ipv6" {
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