package vnc

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// Service provides VNC connection management with support for multiple concurrent sessions
type Service struct {
	client         *api.Client
	sessionManager *SessionManager
	logger         *logger.Logger
}

// NewService creates a new VNC service with session management capabilities
func NewService(client *api.Client) *Service {
	// Create a logger for VNC operations
	vncLogger, err := logger.NewInternalLogger(logger.LevelDebug, "")
	if err != nil {
		// Fallback to a simple logger if file logging fails
		vncLogger = logger.NewSimpleLogger(logger.LevelInfo)
	}

	vncLogger.Info("Creating new VNC service with session management")

	return &Service{
		client:         client,
		sessionManager: NewSessionManager(client),
		logger:         vncLogger,
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
// This method supports multiple concurrent sessions - each VM gets its own session
func (s *Service) ConnectToVMEmbedded(vm *api.VM) error {
	s.logger.Info("Starting embedded VNC connection for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	// Create or get existing session for this VM
	session, err := s.sessionManager.CreateVMSession(vm)
	if err != nil {
		s.logger.Error("Failed to create VM session for %s: %v", vm.Name, err)
		return fmt.Errorf("failed to create VM session: %w", err)
	}

	s.logger.Info("VM VNC session ready: %s (Port: %d, Session: %s)", vm.Name, session.Port, session.ID)

	// Open the embedded VNC client in the default browser
	err = openBrowser(session.URL)
	if err != nil {
		s.logger.Error("Failed to open embedded VNC client for VM %s: %v", vm.Name, err)
		return err
	}

	s.logger.Info("Successfully opened embedded VNC client for VM %s (Session: %s)", vm.Name, session.ID)
	return nil
}

// ConnectToNodeEmbedded opens an embedded VNC shell connection to a node using the built-in noVNC client
// This method supports multiple concurrent sessions - each node gets its own session
func (s *Service) ConnectToNodeEmbedded(nodeName string) error {
	s.logger.Info("Starting embedded VNC shell connection for node: %s", nodeName)

	// Create or get existing session for this node
	session, err := s.sessionManager.CreateNodeSession(nodeName)
	if err != nil {
		s.logger.Error("Failed to create node session for %s: %v", nodeName, err)
		return fmt.Errorf("failed to create node session: %w", err)
	}

	s.logger.Info("Node VNC session ready: %s (Port: %d, Session: %s)", nodeName, session.Port, session.ID)

	// Open the embedded VNC client in the default browser
	err = openBrowser(session.URL)
	if err != nil {
		s.logger.Error("Failed to open embedded VNC client for node %s: %v", nodeName, err)
		return err
	}

	s.logger.Info("Successfully opened embedded VNC client for node %s (Session: %s)", nodeName, session.ID)
	return nil
}

// Session Management Methods

// ListActiveSessions returns all active VNC sessions
func (s *Service) ListActiveSessions() []*VNCSession {
	sessions := s.sessionManager.ListSessions()
	s.logger.Debug("Retrieved %d active VNC sessions", len(sessions))
	return sessions
}

// GetActiveSessionCount returns the number of active VNC sessions
func (s *Service) GetActiveSessionCount() int {
	count := s.sessionManager.GetSessionCount()
	s.logger.Debug("Active VNC session count: %d", count)
	return count
}

// CloseSession closes a specific VNC session by ID
func (s *Service) CloseSession(sessionID string) error {
	s.logger.Info("Closing VNC session: %s", sessionID)
	err := s.sessionManager.CloseSession(sessionID)
	if err != nil {
		s.logger.Error("Failed to close VNC session %s: %v", sessionID, err)
		return err
	}
	s.logger.Info("VNC session closed successfully: %s", sessionID)
	return nil
}

// CloseAllSessions closes all active VNC sessions
func (s *Service) CloseAllSessions() error {
	s.logger.Info("Closing all VNC sessions")
	err := s.sessionManager.CloseAllSessions()
	if err != nil {
		s.logger.Error("Failed to close all VNC sessions: %v", err)
		return err
	}
	s.logger.Info("All VNC sessions closed successfully")
	return nil
}

// GetSessionByTarget finds a session by target (VM name or node name)
func (s *Service) GetSessionByTarget(sessionType SessionType, target string) (*VNCSession, bool) {
	session, exists := s.sessionManager.GetSessionByTarget(sessionType, target)
	if exists {
		s.logger.Debug("Found existing VNC session for %s %s: %s", sessionType, target, session.ID)
	} else {
		s.logger.Debug("No existing VNC session found for %s %s", sessionType, target)
	}
	return session, exists
}

// CleanupInactiveSessions removes sessions that haven't been accessed recently
func (s *Service) CleanupInactiveSessions(maxAge time.Duration) {
	s.logger.Debug("Cleaning up inactive VNC sessions (max age: %s)", maxAge.String())
	s.sessionManager.CleanupInactiveSessions(maxAge)
}

// Legacy Methods (maintained for backward compatibility)

// StopEmbeddedServer stops all embedded VNC servers (legacy method - now closes all sessions)
func (s *Service) StopEmbeddedServer() error {
	s.logger.Info("Legacy StopEmbeddedServer called - closing all VNC sessions")
	return s.CloseAllSessions()
}

// IsEmbeddedServerRunning returns whether any embedded VNC servers are running
func (s *Service) IsEmbeddedServerRunning() bool {
	running := s.GetActiveSessionCount() > 0
	s.logger.Debug("Embedded VNC server running status: %t (%d active sessions)", running, s.GetActiveSessionCount())
	return running
}

// GetEmbeddedServerPort returns the port of the first active session (legacy method)
func (s *Service) GetEmbeddedServerPort() int {
	sessions := s.ListActiveSessions()
	if len(sessions) > 0 {
		port := sessions[0].Port
		s.logger.Debug("Legacy GetEmbeddedServerPort returning first session port: %d", port)
		return port
	}
	s.logger.Debug("Legacy GetEmbeddedServerPort: no active sessions")
	return 0
}
