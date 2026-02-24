package components

import (
	"errors"
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestParseEditableNetworkConfigLXCIPModes(t *testing.T) {
	dhcp := parseEditableNetworkConfig(api.VMTypeLXC, "name=eth0,bridge=vmbr0,ip=dhcp,firewall=1")
	require.Equal(t, "dhcp", dhcp.IPMode)
	require.True(t, dhcp.IPModeSet)
	require.Equal(t, "", dhcp.IP)
	require.True(t, dhcp.Firewall)
	require.True(t, dhcp.FirewallSet)

	static := parseEditableNetworkConfig(api.VMTypeLXC, "name=eth0,bridge=vmbr0,ip=10.0.0.10/24,gw=10.0.0.1")
	require.Equal(t, "static", static.IPMode)
	require.True(t, static.IPModeSet)
	require.Equal(t, "10.0.0.10/24", static.IP)
	require.Equal(t, "10.0.0.1", static.Gateway)
}

func TestBuildEditableNetworkRawLXCIPModes(t *testing.T) {
	dhcpRaw := buildEditableNetworkRaw(api.VMTypeLXC, editableNetworkConfig{
		Name:        "eth0",
		Bridge:      "vmbr0",
		IPMode:      "dhcp",
		IPModeSet:   true,
		Gateway:     "10.0.0.1", // should be ignored for dhcp mode
		Firewall:    true,
		FirewallSet: true,
	})
	require.Contains(t, dhcpRaw, "ip=dhcp")
	require.NotContains(t, dhcpRaw, "gw=10.0.0.1")
	require.Contains(t, dhcpRaw, "firewall=1")

	staticRaw := buildEditableNetworkRaw(api.VMTypeLXC, editableNetworkConfig{
		Name:        "eth0",
		Bridge:      "vmbr0",
		IPMode:      "static",
		IPModeSet:   true,
		IP:          "10.0.0.10/24",
		Gateway:     "10.0.0.1",
		Firewall:    true,
		FirewallSet: true,
	})
	require.Contains(t, staticRaw, "ip=10.0.0.10/24")
	require.Contains(t, staticRaw, "gw=10.0.0.1")
}

func TestBuildEditableNetworkRawLXCPreservesUnsetOptionalKeys(t *testing.T) {
	raw := buildEditableNetworkRaw(api.VMTypeLXC, editableNetworkConfig{
		Name:    "eth0",
		Bridge:  "vmbr0",
		IPMode:  "dhcp", // default mode in UI for LXC
		MACAddr: "BC:24:11:AA:BB:CC",
	})

	require.NotContains(t, raw, "ip=dhcp")
	require.NotContains(t, raw, "firewall=0")
}

func TestSummarizeConfigSaveErrorExtractsProxmoxMessage(t *testing.T) {
	err := errors.New(`request failed after 1 attempts: API request failed with status 400: {"data":"parameter verification failed - net0: invalid format"}`)
	msg := summarizeConfigSaveError(err)
	require.Contains(t, msg, "parameter verification failed")
	require.Contains(t, msg, "net0")
}

func TestSummarizeConfigSaveErrorFallsBackToRaw(t *testing.T) {
	err := errors.New("plain failure")
	require.Equal(t, "Failed to save config: plain failure", summarizeConfigSaveError(err))
}

func TestIsValidLXCIPv4Config(t *testing.T) {
	require.True(t, isValidLXCIPv4Config("192.168.99.24/24"))
	require.True(t, isValidLXCIPv4Config("manual"))
	require.False(t, isValidLXCIPv4Config("192.168.99.24"))
	require.False(t, isValidLXCIPv4Config("dhcp"))
}

func TestNetworkConfigMapsEqual(t *testing.T) {
	a := map[string]string{"net0": "name=eth0,bridge=vmbr0,ip=dhcp"}
	b := map[string]string{"net0": "name=eth0,bridge=vmbr0,ip=dhcp"}
	c := map[string]string{"net0": "name=eth0,bridge=vmbr0,ip=10.0.0.10/24"}
	require.True(t, networkConfigMapsEqual(a, b))
	require.False(t, networkConfigMapsEqual(a, c))
}
