package ui

import (
	"fmt"

	"github.com/lonepie/proxmox-util/pkg/api"
	"github.com/rivo/tview"
)

// GetShellCommand returns the appropriate shell command string for a VM or container
func GetShellCommand(vm api.VM) string {
	var shellCmd string

	if vm.Type == "lxc" {
		// For LXC containers
		shellCmd = fmt.Sprintf("ssh %s -t 'sudo pct exec %d -- /bin/bash'", vm.Node, vm.ID)
	} else if vm.Type == "qemu" && vm.IP != "" {
		// For QEMU VMs with IPs
		shellCmd = fmt.Sprintf("ssh %s", vm.IP)
	} else {
		// For VMs without IPs or unsupported types
		shellCmd = fmt.Sprintf("# Cannot connect to %s: No IP available or unsupported type: %s",
			vm.Name, vm.Type)
	}

	return shellCmd
}

// CreateShellInfoPanel creates an informational panel displaying shell commands
func CreateShellInfoPanel() *tview.TextView {
	shellInfoPanel := tview.NewTextView()
	shellInfoPanel.SetBorder(true)
	shellInfoPanel.SetTitle("Shell Command Info")
	shellInfoPanel.SetDynamicColors(true)
	shellInfoPanel.SetText("Press 'S' to see SSH command for the selected VM/container")
	return shellInfoPanel
}
