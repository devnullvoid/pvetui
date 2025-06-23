package components

import (
	"fmt"
	// "strings"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	// "github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/devnullvoid/proxmox-tui/internal/ssh"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/vnc"
)

// openNodeShell opens an SSH session to the currently selected node
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

// connectToNodeVNC performs the actual node VNC connection using embedded noVNC client
func (a *App) connectToNodeVNC(node *api.Node, vncService *vnc.Service) {
	// Show loading message
	a.header.ShowLoading(fmt.Sprintf("Starting embedded VNC shell for %s...", node.Name))

	// Open embedded VNC connection in a goroutine to avoid blocking UI
	go func() {
		err := vncService.ConnectToNodeEmbedded(node.Name)
		a.QueueUpdateDraw(func() {
			if err != nil {
				a.header.ShowError(fmt.Sprintf("Failed to start VNC shell: %v", err))
			} else {
				a.header.ShowSuccess(fmt.Sprintf("Embedded VNC shell started for %s", node.Name))
			}
		})
	}()
}

// connectToVMVNC performs the actual VM VNC connection using embedded noVNC client
func (a *App) connectToVMVNC(vm *api.VM, vncService *vnc.Service) {
	// Show loading message
	a.header.ShowLoading(fmt.Sprintf("Starting embedded VNC console for %s...", vm.Name))

	// Open embedded VNC connection in a goroutine to avoid blocking UI
	go func() {
		err := vncService.ConnectToVMEmbedded(vm)
		a.QueueUpdateDraw(func() {
			if err != nil {
				a.header.ShowError(fmt.Sprintf("Failed to start VNC console: %v", err))
			} else {
				a.header.ShowSuccess(fmt.Sprintf("Embedded VNC console started for %s", vm.Name))
			}
		})
	}()
}

// openNodeVNC opens a VNC shell connection to the currently selected node
func (a *App) openNodeVNC() {
	node := a.nodeList.GetSelectedNode()
	if node == nil {
		a.header.ShowError("No node selected")
		return
	}

	logger := models.GetUILogger()
	logger.Debug("Opening VNC shell for node: %s", node.Name)

	// Use the shared VNC service instead of creating a new one
	vncService := a.GetVNCService()

	// Check if VNC is available for this node
	available, reason := vncService.GetNodeVNCStatus(node.Name)
	if !available {
		a.header.ShowError(reason)
		return
	}

	// Connect directly to VNC
	a.connectToNodeVNC(node, vncService)
}

// openVMVNC opens a VNC console connection to the currently selected VM
func (a *App) openVMVNC() {
	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		a.header.ShowError("No VM selected")
		return
	}

	// Use the shared VNC service instead of creating a new one
	vncService := a.GetVNCService()

	// Check if VNC is available for this VM
	available, reason := vncService.GetVMVNCStatus(vm)
	if !available {
		a.header.ShowError(reason)
		return
	}

	// Connect directly to VNC
	a.connectToVMVNC(vm, vncService)
}

// openVMShell opens a shell session to the currently selected VM/container
func (a *App) openVMShell() {
	if a.config.SSHUser == "" {
		a.showMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")
		return
	}

	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		a.showMessage("Selected VM not found")
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
		a.showMessage("Host node IP address not available")
		return
	}

	// Temporarily suspend the UI
	a.Suspend(func() {
		if vm.Type == "lxc" {
			fmt.Printf("\nConnecting to LXC container %s (ID: %d) on node %s (%s)...\n",
				vm.Name, vm.ID, vm.Node, nodeIP)

			// Execute LXC shell command
			err := ssh.ExecuteLXCShell(a.config.SSHUser, nodeIP, vm.ID)
			if err != nil {
				fmt.Printf("\nError connecting to LXC container: %v\n", err)
			}
		} else if vm.Type == "qemu" {
			// For QEMU VMs, check if guest agent is running
			if vm.AgentRunning {
				fmt.Printf("\nConnecting to QEMU VM %s (ID: %d) via guest agent on node %s...\n",
					vm.Name, vm.ID, vm.Node)

				// Try using the guest agent
				err := ssh.ExecuteQemuGuestAgentShell(a.config.SSHUser, nodeIP, vm.ID)
				if err != nil {
					fmt.Printf("\nError connecting via guest agent: %v\n", err)

					// If guest agent fails and we have an IP, try direct SSH
					if vm.IP != "" {
						fmt.Printf("\nFalling back to direct SSH connection to %s...\n", vm.IP)
						err = ssh.ExecuteQemuShell(a.config.SSHUser, vm.IP)
						if err != nil {
							fmt.Printf("\nFailed to SSH to VM: %v\n", err)
						}
					}
				}
			} else if vm.IP != "" {
				// No guest agent, but we have an IP
				fmt.Printf("\nConnecting to QEMU VM %s (ID: %d) via SSH at %s...\n",
					vm.Name, vm.ID, vm.IP)

				err := ssh.ExecuteQemuShell(a.config.SSHUser, vm.IP)
				if err != nil {
					fmt.Printf("\nFailed to SSH to VM: %v\n", err)
				}
			} else {
				// No guest agent, no IP
				fmt.Println("\nNeither guest agent nor IP address available for this VM.")
				fmt.Println("To connect to this VM, either:")
				fmt.Println("1. Install QEMU guest agent in the VM")
				fmt.Println("2. Configure network to get an IP address")
				fmt.Println("3. Set up VNC access (not currently supported in TUI)")
			}
		} else {
			fmt.Printf("\nUnsupported VM type: %s\n", vm.Type)
		}

		// Wait for user to press Enter
		// fmt.Print("\nPress Enter to return to the TUI...")
		// utils.WaitForEnter()
	})

	// Fix for tview suspend/resume issue - comprehensive terminal state restoration
	a.Sync()
}
