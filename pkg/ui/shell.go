package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// GetShellCommand returns the appropriate shell command string for a VM or container
func GetShellCommand(vm api.VM) string {
	var shellCmd string

	if vm.Type == "lxc" {
		// For LXC containers
		shellCmd = fmt.Sprintf("ssh %s -t 'sudo pct exec %d -- /bin/bash -l'", vm.Node, vm.ID)
	} else if vm.Type == "qemu" && vm.IP != "" {
		// For QEMU VMs with IPs
		shellCmd = fmt.Sprintf("ssh -t %s", vm.IP)
	} else {
		// For VMs without IPs or unsupported types
		shellCmd = fmt.Sprintf("# Cannot connect to %s: No IP available or unsupported type: %s",
			vm.Name, vm.Type)
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

// HandleShellExecution executes the appropriate shell command for a VM
func HandleShellExecution(app *tview.Application, vm api.VM) {
	if strings.HasPrefix(GetShellCommand(vm), "#") {
		// Invalid command, show error
		app.QueueUpdateDraw(func() {
			app.Stop()
			fmt.Println(GetShellCommand(vm))
			app.Draw()
		})
		return
	}

	app.Suspend(func() {
		cmd := exec.Command("sh", "-c", GetShellCommand(vm))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	})
}
