package vnc

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// Service handles VNC connections for VMs and nodes
type Service struct {
	client *api.Client
	server *Server
	logger *logger.Logger
}

// NewService creates a new VNC service
func NewService(client *api.Client) *Service {
	// Create a logger for VNC operations
	vncLogger, err := logger.NewInternalLogger(logger.LevelDebug, "")
	if err != nil {
		// Fallback to a simple logger if file logging fails
		vncLogger = logger.NewSimpleLogger(logger.LevelInfo)
	}

	vncLogger.Info("Creating new VNC service")

	return &Service{
		client: client,
		server: NewServer(),
		logger: vncLogger,
	}
}

// ConnectToVM opens a VNC connection to a VM in the user's browser
// Note: Validation should be done using GetVMVNCStatus before calling this method
func (s *Service) ConnectToVM(vm *api.VM) error {
	s.logger.Info("Connecting to VM VNC: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	// Generate the VNC URL
	vncURL, err := s.client.GenerateVNCURL(vm)
	if err != nil {
		s.logger.Error("Failed to generate VNC URL for VM %s: %v", vm.Name, err)
		return fmt.Errorf("failed to generate VNC URL: %w", err)
	}

	s.logger.Debug("Generated VNC URL for VM %s: %s", vm.Name, vncURL)

	// Open the URL in the default browser
	err = openBrowser(vncURL)
	if err != nil {
		s.logger.Error("Failed to open browser for VM %s VNC: %v", vm.Name, err)
		return err
	}

	s.logger.Info("Successfully opened VNC connection for VM %s", vm.Name)
	return nil
}

// ConnectToNode opens a VNC shell connection to a node in the user's browser
func (s *Service) ConnectToNode(nodeName string) error {
	s.logger.Info("Connecting to node VNC shell: %s", nodeName)

	// Generate the VNC shell URL
	vncURL, err := s.client.GenerateNodeVNCURL(nodeName)
	if err != nil {
		s.logger.Error("Failed to generate VNC shell URL for node %s: %v", nodeName, err)
		return fmt.Errorf("failed to generate VNC shell URL: %w", err)
	}

	s.logger.Debug("Generated VNC shell URL for node %s: %s", nodeName, vncURL)

	// Open the URL in the default browser
	err = openBrowser(vncURL)
	if err != nil {
		s.logger.Error("Failed to open browser for node %s VNC shell: %v", nodeName, err)
		return err
	}

	s.logger.Info("Successfully opened VNC shell connection for node %s", nodeName)
	return nil
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
	s.logger.Debug("Checking VNC status for VM: %s (Type: %s, Status: %s)", vm.Name, vm.Type, vm.Status)

	if vm.Type != "qemu" && vm.Type != "lxc" {
		s.logger.Debug("VNC not available for VM %s: unsupported type %s", vm.Name, vm.Type)
		return false, "VNC only available for QEMU VMs and LXC containers"
	}

	if vm.Status != "running" {
		s.logger.Debug("VNC not available for VM %s: VM not running (status: %s)", vm.Name, vm.Status)
		return false, "VM must be running"
	}

	s.logger.Debug("VNC available for VM %s", vm.Name)
	return true, "VNC available"
}

// GetNodeVNCStatus checks if VNC shell is available for a node
func (s *Service) GetNodeVNCStatus(nodeName string) (bool, string) {
	s.logger.Debug("Checking VNC shell status for node: %s", nodeName)

	// Node VNC shells don't work with API token authentication
	if s.client.IsUsingTokenAuth() {
		s.logger.Debug("VNC shell not available for node %s: using API token authentication", nodeName)
		return false, "Node VNC shells are not supported with API token authentication.\n\nThis is a Proxmox limitation - node VNC shells require password authentication.\n\nTo use node VNC shells:\n1. Configure password authentication instead of API tokens\n2. Set PROXMOX_PASSWORD environment variable\n3. Remove PROXMOX_TOKEN_ID and PROXMOX_TOKEN_SECRET"
	}

	s.logger.Debug("VNC shell available for node %s", nodeName)
	// For nodes with password auth, VNC shell is available if the node is online
	return true, "VNC shell available"
}

// ConnectToVMEmbedded opens an embedded VNC connection to a VM using the built-in noVNC client
// This method does not require users to be logged into the Proxmox web interface
func (s *Service) ConnectToVMEmbedded(vm *api.VM) error {
	s.logger.Info("Starting embedded VNC connection for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	// Stop any existing server
	if s.server.IsRunning() {
		s.logger.Debug("Stopping existing embedded VNC server before starting new connection")
		s.server.Stop()
	}

	// Start embedded VNC server for the VM
	s.logger.Debug("Starting embedded VNC server for VM %s", vm.Name)
	vncURL, err := s.server.StartVMVNCServer(s.client, vm)
	if err != nil {
		s.logger.Error("Failed to start embedded VNC server for VM %s: %v", vm.Name, err)
		return fmt.Errorf("failed to start embedded VNC server: %w", err)
	}

	s.logger.Info("Embedded VNC server started for VM %s on port %d", vm.Name, s.server.GetPort())
	s.logger.Debug("Generated embedded VNC URL for VM %s: %s", vm.Name, vncURL)

	// Open the embedded VNC client in the default browser
	err = openBrowser(vncURL)
	if err != nil {
		s.logger.Error("Failed to open embedded VNC client for VM %s: %v", vm.Name, err)
		return err
	}

	s.logger.Info("Successfully opened embedded VNC client for VM %s", vm.Name)
	return nil
}

// ConnectToNodeEmbedded opens an embedded VNC shell connection to a node using the built-in noVNC client
// This method does not require users to be logged into the Proxmox web interface
func (s *Service) ConnectToNodeEmbedded(nodeName string) error {
	s.logger.Info("Starting embedded VNC shell connection for node: %s", nodeName)

	// Stop any existing server
	if s.server.IsRunning() {
		s.logger.Debug("Stopping existing embedded VNC server before starting new connection")
		s.server.Stop()
	}

	// Start embedded VNC server for the node
	s.logger.Debug("Starting embedded VNC server for node %s", nodeName)
	vncURL, err := s.server.StartNodeVNCServer(s.client, nodeName)
	if err != nil {
		s.logger.Error("Failed to start embedded VNC server for node %s: %v", nodeName, err)
		return fmt.Errorf("failed to start embedded VNC server: %w", err)
	}

	s.logger.Info("Embedded VNC server started for node %s on port %d", nodeName, s.server.GetPort())
	s.logger.Debug("Generated embedded VNC URL for node %s: %s", nodeName, vncURL)

	// Open the embedded VNC client in the default browser
	err = openBrowser(vncURL)
	if err != nil {
		s.logger.Error("Failed to open embedded VNC client for node %s: %v", nodeName, err)
		return err
	}

	s.logger.Info("Successfully opened embedded VNC client for node %s", nodeName)
	return nil
}

// StopEmbeddedServer stops the embedded VNC server
func (s *Service) StopEmbeddedServer() error {
	if s.server.IsRunning() {
		s.logger.Info("Stopping embedded VNC server on port %d", s.server.GetPort())
		err := s.server.Stop()
		if err != nil {
			s.logger.Error("Failed to stop embedded VNC server: %v", err)
			return err
		}
		s.logger.Info("Embedded VNC server stopped successfully")
	} else {
		s.logger.Debug("Embedded VNC server not running, no action needed")
	}
	return nil
}

// IsEmbeddedServerRunning returns whether the embedded VNC server is running
func (s *Service) IsEmbeddedServerRunning() bool {
	running := s.server.IsRunning()
	s.logger.Debug("Embedded VNC server running status: %t", running)
	return running
}

// GetEmbeddedServerPort returns the port the embedded VNC server is running on
func (s *Service) GetEmbeddedServerPort() int {
	port := s.server.GetPort()
	s.logger.Debug("Embedded VNC server port: %d", port)
	return port
}
