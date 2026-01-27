package components

import (
	"fmt"
	"net"
	"strings"

	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/internal/ssh"
	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/internal/vnc"
	"github.com/devnullvoid/pvetui/pkg/api"
)

var shellLogger = logger.GetPackageLogger("ui-shell")

// openNodeShell opens an SSH session to the currently selected node.
func (a *App) openNodeShell() {
	node := a.nodeList.GetSelectedNode()
	if node == nil || node.IP == "" {
		a.showMessage("Node IP address not available")
		return
	}

	// Log node IP details for debugging
	shellLogger.Debug("Node shell for %s: IP from node object: '%s' (len=%d, bytes=%v)",
		node.Name, node.IP, len(node.IP), []byte(node.IP))

	// Determine SSH user
	sshUser := a.config.SSHUser
	jumpHost := a.config.SSHJumpHost

	// In group mode, try to get the SSH user from the node's source profile
	if a.isGroupMode && node.SourceProfile != "" {
		if profile, exists := a.config.Profiles[node.SourceProfile]; exists {
			if profile.SSHUser != "" {
				sshUser = profile.SSHUser
			}
			if profile.SSHJumpHost.Addr != "" {
				jumpHost = profile.SSHJumpHost
			}
		}
	}

	if sshUser == "" {
		a.showMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")
		return
	}

	// Temporarily suspend the UI
	a.Suspend(func() {
		// Display connecting message
		fmt.Printf("\nConnecting to node %s (%s) as user %s...\n", node.Name, node.IP, sshUser)

		// Execute SSH command
		err := ssh.ExecuteNodeShell(sshUser, node.IP, jumpHost)
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

		var vncURL string
		var err error

		client, clientErr := a.getClientForNode(node)
		if clientErr != nil {
			err = clientErr
		} else {
			vncURL, err = vncService.ConnectToNodeEmbeddedWithClient(client, node.Name)
		}

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

		var vncURL string
		var err error

		client, clientErr := a.getClientForVM(vm)
		if clientErr != nil {
			err = clientErr
		} else {
			vncURL, err = vncService.ConnectToVMEmbeddedWithClient(client, vm)
		}

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

	// Get client for node
	client, err := a.getClientForNode(node)
	if err != nil {
		errorModal := CreateErrorDialog("VNC Error", fmt.Sprintf("Failed to get client for node: %v", err), func() {
			a.pages.RemovePage("vnc_error")
		})
		a.pages.AddPage("vnc_error", errorModal, false, true)
		return
	}

	// Check if VNC is available for this node
	available, reason := vncService.GetNodeVNCStatusWithClient(client, node.Name)
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
	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		a.showMessageSafe("Selected VM not found")
		return
	}

	// Determine SSH users
	hostShellUser := a.config.SSHUser
	vmShellUser := a.config.VMSSHUser
	jumpHost := a.config.SSHJumpHost

	// In group mode, try to get users from the VM's source profile
	if a.isGroupMode && vm.SourceProfile != "" {
		if profile, exists := a.config.Profiles[vm.SourceProfile]; exists {
			if profile.SSHUser != "" {
				hostShellUser = profile.SSHUser
			}
			if profile.VMSSHUser != "" {
				vmShellUser = profile.VMSSHUser
			}
			if profile.SSHJumpHost.Addr != "" {
				jumpHost = profile.SSHJumpHost
			}
		}
	}

	if vmShellUser == "" {
		vmShellUser = hostShellUser
	}

	if vm.Type == vmTypeLXC && hostShellUser == "" {
		a.showMessageSafe("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")
		return
	}
	if vm.Type == vmTypeQEMU && vmShellUser == "" {
		a.showMessageSafe("VM SSH user not configured. Set vm_ssh_user (or fallback ssh_user) to use VM shells.")
		return
	}

	// Get node IP from the cluster
	var nodeIP string
	var originalNodeIP string

	client, err := a.getClientForVM(vm)
	if err != nil {
		a.showMessageSafe(fmt.Sprintf("Error finding VM cluster: %v", err))
		return
	}

	if client.Cluster != nil {
		for _, node := range client.Cluster.Nodes {
			if node.Name == vm.Node {
				nodeIP = node.IP
				originalNodeIP = nodeIP
				// Log the IP immediately when obtained from cluster data
				shellLogger.Debug("VM shell for %s (ID: %d): Found node %s with IP from cluster: '%s' (len=%d, bytes=%v)",
					vm.Name, vm.ID, vm.Node, nodeIP, len(nodeIP), []byte(nodeIP))
				break
			}
		}
	}

	if nodeIP == "" && client != nil {
		fallback := client.BaseHostname()
		shellLogger.Debug("Node %s missing IP in cluster data (original='%s'); falling back to API host %s", vm.Node, originalNodeIP, fallback)
		nodeIP = fallback
	}

	if net.ParseIP(nodeIP) == nil && client != nil {
		fallback := client.BaseHostname()
		shellLogger.Debug("Node %s has malformed IP '%s' (original='%s', valid=%v); falling back to API host %s",
			vm.Node, nodeIP, originalNodeIP, net.ParseIP(nodeIP) != nil, fallback)
		nodeIP = fallback
	}

	if nodeIP == "" || net.ParseIP(nodeIP) == nil {
		a.showMessageSafe("Host node IP address not available")
		return
	}

	// Check for QEMU VMs without IP address before suspending UI
	if vm.Type == vmTypeQEMU && vm.IP == "" {
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

		if vm.Type == vmTypeLXC {
			// Determine container type for display
			containerType := "LXC container"
			if vm.OSType == "nixos" || vm.OSType == "nix" {
				containerType = "NixOS LXC container"
			}

			fmt.Printf("\nConnecting to %s %s (ID: %d) on node %s (%s)...\n",
				containerType, vm.Name, vm.ID, vm.Node, nodeIP)

			// Execute LXC shell command with NixOS detection
			err := ssh.ExecuteLXCShellWithVM(hostShellUser, nodeIP, vm, jumpHost)
			if err != nil {
				fmt.Printf("\nError connecting to %s: %v\n", containerType, err)
			}
		} else if vm.Type == vmTypeQEMU {
			// For QEMU VMs, use direct SSH connection
			fmt.Printf("\nConnecting to QEMU VM %s (ID: %d) via SSH as %s@%s...\n",
				vm.Name, vm.ID, vmShellUser, vm.IP)

			err := ssh.ExecuteQemuShell(vmShellUser, vm.IP, jumpHost)
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
