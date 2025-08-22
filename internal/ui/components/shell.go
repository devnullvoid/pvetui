package components

import (
	"fmt"
	"strings"

	"github.com/devnullvoid/peevetui/pkg/api"

	// "github.com/devnullvoid/peevetui/pkg/config".
	"github.com/devnullvoid/peevetui/internal/ssh"
	"github.com/devnullvoid/peevetui/internal/ui/models"
	"github.com/devnullvoid/peevetui/internal/vnc"
)

// openNodeShell opens an SSH session to the currently selected node.
func (a *App) openNodeShell() {
	if a.config.SSHUser == "" {
		a.showMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")

		return
	}

	node := a.nodeList.GetSelectedNode()
	if node == nil || node.IP == "" {
		a.showMessage("Node IP address not available")

		return
	}

	// Temporarily suspend the UI
	a.Suspend(func() {
		// Display connecting message
		fmt.Printf("\nConnecting to node %s (%s) as user %s...\n", node.Name, node.IP, a.config.SSHUser)

		// Execute SSH command
		err := ssh.ExecuteNodeShell(a.config.SSHUser, node.IP)
		if err != nil {
			fmt.Printf("\nError connecting to node: %v\n", err)
		}
		// Wait for user to press Enter
		// fmt.Print("\nPress Vj`Enter to return to the TUI...")
		// utils.WaitForEnter()
	})

	// Fix for tview suspend/resume issue - comprehensive terminal state restoration
	a.Sync()
}

// handleVNCOutcome centralizes UI handling for VNC connection results to avoid duplicated code.
func (a *App) handleVNCOutcome(kind string, name string, vncURL string, err error) {
	if err != nil {
		// Specific handling for missing xdg-open
		if strings.Contains(err.Error(), "xdg-open not found") {
			message := fmt.Sprintf("Cannot open browser automatically on this system.\n\nxdg-open is not installed or not available.\n\nThe VNC server is still running and ready for connection.\n\nTo connect:\n1. Copy the shortened URL below\n2. Paste it into your browser\n3. It will automatically redirect to the full VNC session\n4. Connect quickly before the session expires\n\nShortened URL:\n%s", vncURL)

			modal := CreateErrorDialogWithScrollableText("Browser Not Available", message, func() {
				a.pages.RemovePage("vnc_error")
			})
			a.pages.AddPage("vnc_error", modal, false, true)
			a.SetFocus(modal)

			return
		}

		// Generic error dialog
		context := "VNC connection"
		if kind == "node" {
			context = fmt.Sprintf("VNC shell for %s", name)
		} else if kind == "vm" {
			context = fmt.Sprintf("VNC console for %s", name)
		}

		errorModal := CreateErrorDialog("VNC Connection Error",
			fmt.Sprintf("Failed to start %s:\n\n%s", context, err.Error()),
			func() {
				a.pages.RemovePage("vnc_error")
			})
		a.pages.AddPage("vnc_error", errorModal, false, true)

		return
	}

	// Success path: show fallback URL and header success
	var title, startedHeader, headerMsg string
	if kind == "node" {
		title = "VNC Shell Started"
		startedHeader = fmt.Sprintf("VNC shell started successfully for %s!", name)
		headerMsg = fmt.Sprintf("Embedded VNC shell started for %s", name)
	} else {
		title = "VNC Console Started"
		startedHeader = fmt.Sprintf("VNC console started successfully for %s!", name)
		headerMsg = fmt.Sprintf("Embedded VNC console started for %s", name)
	}

	message := fmt.Sprintf("%s\n\nOpening in your default browser...\n\nIf the browser doesn't open automatically, you can use this URL as a fallback:\n\n%s", startedHeader, vncURL)
	modal := CreateSuccessDialogWithURL(title, message, func() {
		a.pages.RemovePage("vnc_success")
	})
	a.pages.AddPage("vnc_success", modal, false, true)
	a.SetFocus(modal)
	a.header.ShowSuccess(headerMsg)
}

// connectToNodeVNC performs the actual node VNC connection using embedded noVNC client.
func (a *App) connectToNodeVNC(node *api.Node, vncService *vnc.Service) {
	// Show loading message
	a.header.ShowLoading(fmt.Sprintf("Starting embedded VNC shell for %s...", node.Name))

	// Open embedded VNC connection in a goroutine to avoid blocking UI
	go func() {
		uiLogger := models.GetUILogger()
		uiLogger.Debug("Starting VNC connection for node %s with client addr: %s", node.Name, a.config.GetAddr())

		vncURL, err := vncService.ConnectToNodeEmbedded(node.Name)

		a.QueueUpdateDraw(func() {
			// Clear the loading message from header
			a.header.StopLoading()
			a.updateHeaderWithActiveProfile() // Restore header with active profile

			// Unified outcome handling
			a.handleVNCOutcome("node", node.Name, vncURL, err)
		})
	}()
}

// connectToVMVNC performs the actual VM VNC connection using embedded noVNC client.
func (a *App) connectToVMVNC(vm *api.VM, vncService *vnc.Service) {
	// Show loading message
	a.header.ShowLoading(fmt.Sprintf("Starting embedded VNC console for %s...", vm.Name))

	// Open embedded VNC connection in a goroutine to avoid blocking UI
	go func() {
		uiLogger := models.GetUILogger()
		uiLogger.Debug("Starting VNC connection for VM %s with client addr: %s", vm.Name, a.config.GetAddr())

		vncURL, err := vncService.ConnectToVMEmbedded(vm)

		a.QueueUpdateDraw(func() {
			// Clear the loading message from header
			a.header.StopLoading()
			a.updateHeaderWithActiveProfile() // Restore header with active profile

			// Unified outcome handling
			a.handleVNCOutcome("vm", vm.Name, vncURL, err)
		})
	}()
}

// openNodeVNC opens a VNC shell connection to the currently selected node.
func (a *App) openNodeVNC() {
	node := a.nodeList.GetSelectedNode()
	if node == nil {
		// Show error in modal dialog instead of header
		errorModal := CreateErrorDialog("VNC Error", "No node selected", func() {
			a.pages.RemovePage("vnc_error")
		})
		a.pages.AddPage("vnc_error", errorModal, false, true)

		return
	}

	logger := models.GetUILogger()
	logger.Debug("Opening VNC shell for node: %s", node.Name)

	// Use the shared VNC service instead of creating a new one
	vncService := a.GetVNCService()

	// Check if VNC is available for this node
	available, reason := vncService.GetNodeVNCStatus(node.Name)
	if !available {
		// Show error in modal dialog instead of header
		errorModal := CreateErrorDialog("VNC Not Available", reason, func() {
			a.pages.RemovePage("vnc_error")
		})
		a.pages.AddPage("vnc_error", errorModal, false, true)

		return
	}

	// Connect directly to VNC
	a.connectToNodeVNC(node, vncService)
}

// openVMVNC opens a VNC console connection to the currently selected VM.
func (a *App) openVMVNC() {
	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		// Show error in modal dialog instead of header
		errorModal := CreateErrorDialog("VNC Error", "No VM selected", func() {
			a.pages.RemovePage("vnc_error")
		})
		a.pages.AddPage("vnc_error", errorModal, false, true)

		return
	}

	// Use the shared VNC service instead of creating a new one
	vncService := a.GetVNCService()

	// Check if VNC is available for this VM
	available, reason := vncService.GetVMVNCStatus(vm)
	if !available {
		// Show error in modal dialog instead of header
		errorModal := CreateErrorDialog("VNC Not Available", reason, func() {
			a.pages.RemovePage("vnc_error")
		})
		a.pages.AddPage("vnc_error", errorModal, false, true)

		return
	}

	// Connect directly to VNC
	a.connectToVMVNC(vm, vncService)
}

// openVMShell opens a shell session to the currently selected VM/container.
func (a *App) openVMShell() {
	if a.config.SSHUser == "" {
		a.showMessageSafe("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")

		return
	}

	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		a.showMessageSafe("Selected VM not found")

		return
	}

	// Get node IP from the cluster
	var nodeIP string

	for _, node := range a.client.Cluster.Nodes {
		if node.Name == vm.Node {
			nodeIP = node.IP

			break
		}
	}

	if nodeIP == "" {
		a.showMessageSafe("Host node IP address not available")

		return
	}

	// Check for QEMU VMs without IP address before suspending UI
	if vm.Type == "qemu" && vm.IP == "" {
		// Show error dialog for QEMU VMs without IP
		errorModal := CreateErrorDialog("Cannot Open Shell",
			fmt.Sprintf("Cannot open shell: No IP address available for VM %s (ID: %d)\n\nTo connect to this VM:\n1. Configure the VM's network to obtain an IP address\n2. Use VNC console instead (if available)\n3. Check VM network configuration and ensure it's properly started", vm.Name, vm.ID),
			func() {
				a.pages.RemovePage("shell_error")
			})
		a.pages.AddPage("shell_error", errorModal, false, true)
		return
	}

	// Temporarily suspend the UI
	a.Suspend(func() {
		if vm.Type == "lxc" {
			// Determine container type for display
			containerType := "LXC container"
			if vm.OSType == "nixos" || vm.OSType == "nix" {
				containerType = "NixOS LXC container"
			}

			fmt.Printf("\nConnecting to %s %s (ID: %d) on node %s (%s)...\n",
				containerType, vm.Name, vm.ID, vm.Node, nodeIP)

			// Execute LXC shell command with NixOS detection
			err := ssh.ExecuteLXCShellWithVM(a.config.SSHUser, nodeIP, vm)
			if err != nil {
				fmt.Printf("\nError connecting to %s: %v\n", containerType, err)
			}
		} else if vm.Type == "qemu" {
			// For QEMU VMs, use direct SSH connection
			fmt.Printf("\nConnecting to QEMU VM %s (ID: %d) via SSH at %s...\n",
				vm.Name, vm.ID, vm.IP)

			err := ssh.ExecuteQemuShell(a.config.SSHUser, vm.IP)
			if err != nil {
				fmt.Printf("\nFailed to SSH to VM: %v\n", err)
			}
		} else {
			fmt.Printf("\nUnsupported VM type: %s\n", vm.Type)
		}
	})

	// Fix for tview suspend/resume issue - comprehensive terminal state restoration
	a.Sync()
}
