package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// GetShellCommand returns the appropriate shell command string for a VM or Node
func GetShellCommand(target interface{}) string {
	var shellCmd string

	switch t := target.(type) {
	case api.Node:
		// For Proxmox nodes
		shellCmd = fmt.Sprintf("ssh %s", t.Name)
	case api.VM:
		if t.Type == "lxc" {
			// For LXC containers
			shellCmd = fmt.Sprintf("ssh %s -t 'sudo pct exec %d -- /bin/bash -l'", t.Node, t.ID)
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
func HandleShellExecution(app *tview.Application, target interface{}) {
	var shellCmd string

	switch t := target.(type) {
	case api.VM, api.Node:
		shellCmd = GetShellCommand(t)
	default:
		return
	}

	if strings.HasPrefix(shellCmd, "#") {
		app.QueueUpdateDraw(func() {
			app.Stop()
			fmt.Println(shellCmd)
			app.Draw()
		})
		return
	}

	app.Suspend(func() {
		cmd := exec.Command("sh", "-c", shellCmd)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	})
}
