package vnc

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// Service handles VNC connections for VMs and nodes
type Service struct {
	client *api.Client
}

// NewService creates a new VNC service
func NewService(client *api.Client) *Service {
	return &Service{
		client: client,
	}
}

// ConnectToVM opens a VNC connection to a VM in the user's browser
func (s *Service) ConnectToVM(vm *api.VM) error {
	if vm.Type != "qemu" && vm.Type != "lxc" {
		return fmt.Errorf("VNC connections are only available for QEMU VMs and LXC containers")
	}

	if vm.Status != "running" {
		return fmt.Errorf("VM must be running to establish VNC connection")
	}

	// Generate the VNC URL
	vncURL, err := s.client.GenerateVNCURL(vm)
	if err != nil {
		return fmt.Errorf("failed to generate VNC URL: %w", err)
	}

	// Open the URL in the default browser
	return openBrowser(vncURL)
}

// ConnectToNode opens a VNC shell connection to a node in the user's browser
func (s *Service) ConnectToNode(nodeName string) error {
	// Generate the VNC shell URL
	vncURL, err := s.client.GenerateNodeVNCURL(nodeName)
	if err != nil {
		return fmt.Errorf("failed to generate VNC shell URL: %w", err)
	}

	// Open the URL in the default browser
	return openBrowser(vncURL)
}

// openBrowser opens the specified URL in the user's default browser
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// GetVMVNCStatus checks if VNC is available for a VM
func (s *Service) GetVMVNCStatus(vm *api.VM) (bool, string) {
	if vm.Type != "qemu" && vm.Type != "lxc" {
		return false, "VNC only available for QEMU VMs and LXC containers"
	}
	
	if vm.Status != "running" {
		return false, "VM must be running"
	}
	
	return true, "VNC available"
}

// GetNodeVNCStatus checks if VNC shell is available for a node
func (s *Service) GetNodeVNCStatus(nodeName string) (bool, string) {
	// Node VNC shells don't work with API token authentication
	if s.client.IsUsingTokenAuth() {
		return false, "Node VNC shells require password authentication (not supported with API tokens)"
	}
	
	// For nodes with password auth, VNC shell is available if the node is online
	return true, "VNC shell available"
}