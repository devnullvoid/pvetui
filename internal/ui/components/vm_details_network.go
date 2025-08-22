package components

import (
	"strings"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// EnhancedNetworkInterface represents enhanced network information with both config and runtime data.
type EnhancedNetworkInterface struct {
	// From configuration
	Interface    string
	Model        string
	MACAddr      string
	Bridge       string
	VLAN         string
	Rate         string
	ConfiguredIP string
	Gateway      string
	Firewall     bool

	// From guest agent
	RuntimeName   string
	RuntimeIPs    []string
	IsUp          bool
	HasGuestAgent bool
	IsGuestOnly   bool // True if this interface is only visible via guest agent
}

func mergeNetworkInterfaces(configuredNets []api.ConfiguredNetwork, guestInterfaces []api.NetworkInterface) []EnhancedNetworkInterface {
	var enhanced []EnhancedNetworkInterface

	// Create a map of guest interfaces by MAC for quick lookup
	guestByMAC := make(map[string]api.NetworkInterface)

	for _, iface := range guestInterfaces {
		if iface.MACAddress != "" {
			guestByMAC[strings.ToUpper(iface.MACAddress)] = iface
		}
	}

	// Process configured networks first (these are authoritative)
	for _, configured := range configuredNets {
		enhancedNet := EnhancedNetworkInterface{
			Interface:    configured.Interface,
			Model:        configured.Model,
			MACAddr:      configured.MACAddr,
			Bridge:       configured.Bridge,
			VLAN:         configured.VLAN,
			Rate:         configured.Rate,
			ConfiguredIP: configured.IP,
			Gateway:      configured.Gateway,
			Firewall:     configured.Firewall,
		}

		// Try to find matching guest interface by MAC
		if configured.MACAddr != "" {
			if guest, found := guestByMAC[strings.ToUpper(configured.MACAddr)]; found {
				enhancedNet.RuntimeName = guest.Name
				// Convert IPAddress slice to string slice
				for _, ip := range guest.IPAddresses {
					enhancedNet.RuntimeIPs = append(enhancedNet.RuntimeIPs, ip.Address)
				}
				// Determine if interface is up based on having IP addresses
				enhancedNet.IsUp = len(guest.IPAddresses) > 0
				enhancedNet.HasGuestAgent = true
				// Remove from map so we don't show it again
				delete(guestByMAC, strings.ToUpper(configured.MACAddr))
			}
		}

		enhanced = append(enhanced, enhancedNet)
	}

	// Add any remaining guest interfaces that didn't match configured ones
	for _, guest := range guestByMAC {
		if guest.IsLoopback {
			continue // Skip loopback interfaces
		}

		enhancedNet := EnhancedNetworkInterface{
			RuntimeName:   guest.Name,
			MACAddr:       guest.MACAddress,
			HasGuestAgent: true,
			IsGuestOnly:   true, // Flag to indicate this is guest-agent only
		}

		// Convert IPAddress slice to string slice
		for _, ip := range guest.IPAddresses {
			enhancedNet.RuntimeIPs = append(enhancedNet.RuntimeIPs, ip.Address)
		}

		enhancedNet.IsUp = len(guest.IPAddresses) > 0

		enhanced = append(enhanced, enhancedNet)
	}

	return enhanced
}
