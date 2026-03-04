package ansible

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestBuildInventory_GeneratesNodeAndGuestGroups(t *testing.T) {
	nodes := []*api.Node{
		{Name: "pve-a", IP: "10.0.0.10", Online: true},
		{Name: "pve-b", IP: "10.0.0.11", Online: false},
	}

	guests := []*api.VM{
		{ID: 100, Name: "web-1", IP: "10.0.10.20", Node: "pve-a", Type: api.VMTypeQemu, Status: api.VMStatusRunning, Tags: "prod;web"},
		{ID: 200, Name: "db 1", IP: "10.0.10.30", Node: "pve-b", Type: api.VMTypeLXC, Status: api.VMStatusStopped},
	}

	result := BuildInventory(nodes, guests, InventoryDefaults{NodeSSHUser: "root", VMSSHUser: "ubuntu"})

	require.NotEmpty(t, result.Text)
	require.Len(t, result.Hosts, 4)

	require.Contains(t, result.Text, "[proxmox_nodes]")
	require.Contains(t, result.Text, "[proxmox_guests]")
	require.Contains(t, result.Text, "[qemu]")
	require.Contains(t, result.Text, "[lxc]")
	require.Contains(t, result.Text, "[running]")
	require.Contains(t, result.Text, "[stopped]")
	require.Contains(t, result.Text, "pvetui_tags=\"prod;web\"")
	require.Contains(t, result.Text, "ansible_user=ubuntu")
	require.Contains(t, result.Text, "ansible_user=root")
}

func TestBuildInventoryWithFormat_YAML(t *testing.T) {
	nodes := []*api.Node{{Name: "pve-a", IP: "10.0.0.10", Online: true}}
	guests := []*api.VM{{ID: 100, Name: "web-1", IP: "10.0.10.20", Node: "pve-a", Type: api.VMTypeQemu, Status: api.VMStatusRunning}}

	result := BuildInventoryWithFormat(nodes, guests, InventoryDefaults{
		NodeSSHUser:       "root",
		VMSSHUser:         "ubuntu",
		SSHPrivateKeyFile: "~/.ssh/id_ed25519",
		DefaultPassword:   "secret",
	}, InventoryFormatYAML)

	require.Equal(t, InventoryFormatYAML, result.Format)
	require.Contains(t, result.Text, "all:")
	require.Contains(t, result.Text, "children:")
	require.Contains(t, result.Text, "proxmox_nodes:")
	require.Contains(t, result.Text, "proxmox_guests:")
	require.Contains(t, result.Text, "ansible_ssh_private_key_file")
	require.Contains(t, result.Text, "ansible_password")
}

func TestBuildInventory_DeduplicatesAliases(t *testing.T) {
	guests := []*api.VM{
		{ID: 101, Name: "same-name", IP: "192.168.1.10", Node: "node-1", Type: api.VMTypeQemu, Status: api.VMStatusRunning},
		{ID: 102, Name: "same-name", IP: "192.168.1.11", Node: "node-1", Type: api.VMTypeQemu, Status: api.VMStatusRunning},
	}

	result := BuildInventory(nil, guests, InventoryDefaults{NodeSSHUser: "root", VMSSHUser: "ubuntu"})

	require.Len(t, result.Hosts, 2)
	require.NotEqual(t, result.Hosts[0].Alias, result.Hosts[1].Alias)
	require.True(t, strings.HasPrefix(result.Hosts[0].Alias, "guest_"))
	require.True(t, strings.HasPrefix(result.Hosts[1].Alias, "guest_"))
}

func TestBuildInventory_ExcludesTemplates(t *testing.T) {
	guests := []*api.VM{
		{ID: 100, Name: "base-template", IP: "10.0.10.10", Node: "pve-a", Type: api.VMTypeQemu, Status: api.VMStatusStopped, Template: true},
		{ID: 101, Name: "real-vm", IP: "10.0.10.11", Node: "pve-a", Type: api.VMTypeQemu, Status: api.VMStatusRunning},
	}

	result := BuildInventory(nil, guests, InventoryDefaults{NodeSSHUser: "root", VMSSHUser: "ubuntu"})

	require.Len(t, result.Hosts, 1)
	require.Equal(t, "guest_101_real_vm", result.Hosts[0].Alias)
	require.NotContains(t, result.Text, "base_template")
}

func TestSanitizeIdentifier(t *testing.T) {
	require.Equal(t, "web_01", sanitizeIdentifier("Web-01"))
	require.Equal(t, "db_server_eu", sanitizeIdentifier("db server@eu"))
	require.Equal(t, "unknown", sanitizeIdentifier("***"))
}

func TestNormalizeInventoryFormat(t *testing.T) {
	require.Equal(t, InventoryFormatYAML, NormalizeInventoryFormat("yaml"))
	require.Equal(t, InventoryFormatINI, NormalizeInventoryFormat("INI"))
	require.Equal(t, InventoryFormatYAML, NormalizeInventoryFormat("unknown"))
}

func TestNormalizeInventoryStyle(t *testing.T) {
	require.Equal(t, InventoryStyleCompact, NormalizeInventoryStyle("compact"))
	require.Equal(t, InventoryStyleExpanded, NormalizeInventoryStyle("EXPANDED"))
	require.Equal(t, InventoryStyleCompact, NormalizeInventoryStyle("unknown"))
}

func TestBuildInventoryWithFormat_CompactYAMLMovesSharedVarsToAllVars(t *testing.T) {
	nodes := []*api.Node{
		{Name: "pve-a", IP: "10.0.0.10", Online: true},
		{Name: "pve-b", IP: "10.0.0.11", Online: true},
	}

	result := BuildInventoryWithFormat(nodes, nil, InventoryDefaults{
		NodeSSHUser:       "ansible",
		VMSSHUser:         "ansible",
		SSHPrivateKeyFile: "~/.ssh/id_ed25519",
		DefaultPassword:   "secret",
		Style:             InventoryStyleCompact,
	}, InventoryFormatYAML)

	require.Contains(t, result.Text, "all:")
	require.Contains(t, result.Text, "ansible_user: ansible")
	require.Contains(t, result.Text, "ansible_ssh_private_key_file: ~/.ssh/id_ed25519")
	require.Contains(t, result.Text, "ansible_password: secret")
	require.NotContains(t, result.Text, "node_pve_a:\n      ansible_user:")
	require.NotContains(t, result.Text, "node_pve_b:\n      ansible_user:")
}

func TestBuildInventoryWithFormat_ExpandedINIDoesNotLiftSharedVars(t *testing.T) {
	nodes := []*api.Node{
		{Name: "pve-a", IP: "10.0.0.10", Online: true},
		{Name: "pve-b", IP: "10.0.0.11", Online: true},
	}

	result := BuildInventoryWithFormat(nodes, nil, InventoryDefaults{
		NodeSSHUser:       "ansible",
		VMSSHUser:         "ansible",
		SSHPrivateKeyFile: "~/.ssh/id_ed25519",
		DefaultPassword:   "secret",
		Style:             InventoryStyleExpanded,
	}, InventoryFormatINI)

	require.Contains(t, result.Text, "[all:vars]")
	require.NotContains(t, result.Text, "\nansible_user=ansible\n")
	require.Contains(t, result.Text, "node_pve_a ansible_host=10.0.0.10 ansible_password=secret")
	require.Contains(t, result.Text, "ansible_ssh_private_key_file=~/.ssh/id_ed25519")
	require.Contains(t, result.Text, "ansible_user=ansible")
}
