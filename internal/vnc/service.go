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
	server *Server
}

// NewService creates a new VNC service
func NewService(client *api.Client) *Service {
	return &Service{
		client: client,
		server: NewServer(),
	}
}

// ConnectToVM opens a VNC connection to a VM in the user's browser
// Note: Validation should be done using GetVMVNCStatus before calling this method
func (s *Service) ConnectToVM(vm *api.VM) error {
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
		return false, "Node VNC shells are not supported with API token authentication.\n\nThis is a Proxmox limitation - node VNC shells require password authentication.\n\nTo use node VNC shells:\n1. Configure password authentication instead of API tokens\n2. Set PROXMOX_PASSWORD environment variable\n3. Remove PROXMOX_TOKEN_ID and PROXMOX_TOKEN_SECRET"
	}

	// For nodes with password auth, VNC shell is available if the node is online
	return true, "VNC shell available"
}

// ConnectToVMEmbedded opens an embedded VNC connection to a VM using the built-in noVNC client
// This method does not require users to be logged into the Proxmox web interface
func (s *Service) ConnectToVMEmbedded(vm *api.VM) error {
	// Stop any existing server
	s.server.Stop()

	// Start embedded VNC server for the VM
	vncURL, err := s.server.StartVMVNCServer(s.client, vm)
	if err != nil {
		return fmt.Errorf("failed to start embedded VNC server: %w", err)
	}

	// Open the embedded VNC client in the default browser
	return openBrowser(vncURL)
}

// ConnectToNodeEmbedded opens an embedded VNC shell connection to a node using the built-in noVNC client
// This method does not require users to be logged into the Proxmox web interface
func (s *Service) ConnectToNodeEmbedded(nodeName string) error {
	// Stop any existing server
	s.server.Stop()

	// Start embedded VNC server for the node
	vncURL, err := s.server.StartNodeVNCServer(s.client, nodeName)
	if err != nil {
		return fmt.Errorf("failed to start embedded VNC server: %w", err)
	}

	// Open the embedded VNC client in the default browser
	return openBrowser(vncURL)
}

// StopEmbeddedServer stops the embedded VNC server
func (s *Service) StopEmbeddedServer() error {
	return s.server.Stop()
}

// IsEmbeddedServerRunning returns whether the embedded VNC server is running
func (s *Service) IsEmbeddedServerRunning() bool {
	return s.server.IsRunning()
}

// GetEmbeddedServerPort returns the port the embedded VNC server is running on
func (s *Service) GetEmbeddedServerPort() int {
	return s.server.GetPort()
}
