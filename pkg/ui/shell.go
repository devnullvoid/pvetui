package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/lonepie/proxmox-tui/pkg/config"
	"github.com/rivo/tview"
)

// GetShellCommand returns the appropriate shell command string for a VM or Node
func GetShellCommand(target interface{}, cfg config.Config) string {
	var shellCmd string

	switch t := target.(type) {
	case api.Node:
		// For Proxmox nodes
		shellCmd = fmt.Sprintf("ssh %s@%s", cfg.SSHUser, t.Name)
	case api.VM:
		if t.Type == "lxc" {
			// For LXC containers
			shellCmd = fmt.Sprintf("ssh %s@%s -t 'sudo pct exec %d -- /bin/bash -l'", cfg.SSHUser, t.Node, t.ID)
		} else if t.Type == "qemu" && t.IP != "" {
			// For QEMU VMs with IPs
			shellCmd = fmt.Sprintf("ssh -t %s", t.IP)
		} else {
			// For VMs without IPs or unsupported types
			shellCmd = fmt.Sprintf("# Cannot connect to %s: No IP available or unsupported type: %s",
				t.Name, t.Type)
		}
	default:
		shellCmd = "# Unsupported target type for shell connection"
	}

	return shellCmd
}

// CreateShellInfoPanel creates an informational panel for shell access
func CreateShellInfoPanel() *tview.TextView {
	shellInfoPanel := tview.NewTextView()
	shellInfoPanel.SetBorder(true)
	shellInfoPanel.SetTitle("Shell Access")
	shellInfoPanel.SetDynamicColors(true)
	shellInfoPanel.SetText("Press 'S' to launch shell session for selected VM/container")
	return shellInfoPanel
}

// HandleShellExecution executes the appropriate shell command for a VM or node
func (a *AppUI) HandleShellExecution(target interface{}) {
	var shellCmd string

	switch t := target.(type) {
	case api.VM, api.Node:
		shellCmd = GetShellCommand(t, a.config)
	default:
		return
	}

	if strings.HasPrefix(shellCmd, "#") {
		a.app.QueueUpdateDraw(func() {
			a.app.Stop()
			fmt.Println(shellCmd)
			a.app.Draw()
		})
		return
	}

	a.app.Suspend(func() {
		cmd := exec.Command("sh", "-c", shellCmd)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	})
}
