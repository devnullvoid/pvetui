// Package vnc provides VNC session management and concurrent session handling.
// This package implements a comprehensive session management system that allows
// multiple concurrent VNC sessions to different targets (VMs, containers, nodes).
//
// The SessionManager handles:
// - Dynamic port allocation for each VNC session
// - Session lifecycle management with automatic cleanup
// - Smart session reuse for the same target
// - Background monitoring and cleanup of inactive sessions
// - Thread-safe operations for concurrent access
//
// Example usage:
//
//	manager := NewSessionManager(logger)
//	defer manager.Shutdown()
//
//	// Create a new session for a VM
//	session, err := manager.CreateSession(ctx, "vm", "node1", "100", client)
//	if err != nil {
//		return fmt.Errorf("failed to create VNC session: %w", err)
//	}
//
//	// Session is automatically managed and cleaned up
//	fmt.Printf("VNC session available at: http://localhost:%d", session.Port)
package vnc

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// SessionType represents the type of VNC session
type SessionType string

const (
	SessionTypeVM   SessionType = "vm"
	SessionTypeLXC  SessionType = "lxc"
	SessionTypeNode SessionType = "node"
)

// VNCSession represents an active VNC session with comprehensive metadata
// and lifecycle management. Each session corresponds to a single VNC connection
// to a Proxmox target (VM, container, or node shell).
type VNCSession struct {
	// ID is a unique identifier for this session, used for tracking and management
	ID string

	// Target information for the VNC connection
	TargetType SessionType // "vm", "lxc", or "node"
	NodeName   string      // Proxmox node name
	VMID       string      // VM/Container ID (empty for node shells)
	TargetName string      // Display name for the target

	// Network configuration
	Port int    // Local HTTP server port for this session
	URL  string // Full URL to access this session

	// Session lifecycle tracking
	CreatedAt time.Time // When this session was created
	LastUsed  time.Time // Last time this session was accessed

	// VNC connection details
	ProxyConfig *ProxyConfig // VNC proxy configuration from Proxmox API

	// Server management
	Server *Server // The HTTP server instance for this session

	// Cleanup management
	cancelFunc context.CancelFunc // Function to cancel the session context
	mutex      sync.RWMutex       // Protects concurrent access to session fields
}

// UpdateLastUsed updates the last used timestamp for this session.
// This is called whenever the session is accessed to track activity
// for cleanup purposes.
func (s *VNCSession) UpdateLastUsed() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.LastUsed = time.Now()
}

// IsExpired checks if this session has been inactive for longer than the
// specified timeout duration. Expired sessions are candidates for cleanup.
func (s *VNCSession) IsExpired(timeout time.Duration) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return time.Since(s.LastUsed) > timeout
}

// GetTargetKey returns a unique key identifying the target of this session.
// Sessions with the same target key can potentially be reused.
func (s *VNCSession) GetTargetKey() string {
	return fmt.Sprintf("%s:%s:%s", s.TargetType, s.NodeName, s.VMID)
}

// Shutdown gracefully shuts down this VNC session, stopping the HTTP server
// and cleaning up all associated resources.
func (s *VNCSession) Shutdown() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Cancel the session context
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// Stop the HTTP server
	if s.Server != nil {
		return s.Server.Stop()
	}

	return nil
}

// SessionManager manages multiple concurrent VNC sessions with comprehensive
// lifecycle management, automatic cleanup, and smart session reuse.
//
// The SessionManager provides:
// - Thread-safe session creation and management
// - Dynamic port allocation to avoid conflicts
// - Automatic cleanup of inactive sessions
// - Session reuse for the same target
// - Background monitoring and maintenance
//
// All operations are thread-safe and can be called concurrently from
// multiple goroutines.
type SessionManager struct {
	// Session storage and management
	sessions map[string]*VNCSession // Active sessions by ID

	// Port management for dynamic allocation
	portRange struct {
		start int // Starting port number for allocation
		end   int // Ending port number for allocation
	}
	usedPorts map[int]bool // Track allocated ports

	// Configuration and dependencies
	client *api.Client    // Proxmox API client
	logger *logger.Logger // Logger instance for debugging and monitoring

	// Lifecycle management
	ctx        context.Context    // Manager context for shutdown coordination
	cancelFunc context.CancelFunc // Function to cancel manager operations

	// Background maintenance
	cleanupTicker *time.Ticker // Ticker for periodic cleanup operations

	// Thread safety
	mutex sync.RWMutex // Protects concurrent access to manager state

	// Session configuration
	sessionTimeout time.Duration // How long sessions remain active without use
}

// NewSessionManager creates a new VNC session manager with comprehensive
// configuration and automatic background maintenance.
//
// The manager will:
// - Allocate ports dynamically in the range 8080-8180
// - Clean up inactive sessions every 5 minutes
// - Expire sessions after 30 minutes of inactivity
// - Provide thread-safe concurrent access
//
// The manager must be shut down properly using Shutdown() to clean up
// background goroutines and active sessions.
func NewSessionManager(client *api.Client) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a logger for session management
	sessionLogger, err := logger.NewInternalLogger(logger.LevelDebug, "")
	if err != nil {
		// Fallback to a simple logger if file logging fails
		sessionLogger = logger.NewSimpleLogger(logger.LevelInfo)
	}

	manager := &SessionManager{
		sessions:       make(map[string]*VNCSession),
		usedPorts:      make(map[int]bool),
		client:         client,
		logger:         sessionLogger,
		ctx:            ctx,
		cancelFunc:     cancel,
		sessionTimeout: 30 * time.Minute, // Sessions expire after 30 minutes
	}

	// Configure port range for dynamic allocation
	manager.portRange.start = 8080
	manager.portRange.end = 8180

	// Start background cleanup process
	manager.startCleanupProcess()

	if sessionLogger != nil {
		sessionLogger.Info("VNC Session Manager initialized",
			"port_range", fmt.Sprintf("%d-%d", manager.portRange.start, manager.portRange.end),
			"session_timeout", manager.sessionTimeout,
			"cleanup_interval", "5m")
	}

	return manager
}

// CreateVMSession creates a new VNC session for a VM.
// If a session already exists for the same VM, it may be reused.
func (sm *SessionManager) CreateVMSession(vm *api.VM) (*VNCSession, error) {
	var sessionType SessionType
	if vm.Type == "qemu" {
		sessionType = SessionTypeVM
	} else if vm.Type == "lxc" {
		sessionType = SessionTypeLXC
	} else {
		return nil, fmt.Errorf("unsupported VM type: %s", vm.Type)
	}

	return sm.CreateSession(context.Background(), sessionType, vm.Node, strconv.Itoa(vm.ID), vm.Name)
}

// CreateNodeSession creates a new VNC session for a node shell.
// If a session already exists for the same node, it may be reused.
func (sm *SessionManager) CreateNodeSession(nodeName string) (*VNCSession, error) {
	return sm.CreateSession(context.Background(), SessionTypeNode, nodeName, "", nodeName)
}

// CreateSession creates a new VNC session for the specified target.
// If a session already exists for the same target, it may be reused
// depending on the session state and age.
func (sm *SessionManager) CreateSession(ctx context.Context, sessionType SessionType, nodeName, vmid, targetName string) (*VNCSession, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	targetKey := fmt.Sprintf("%s:%s:%s", sessionType, nodeName, vmid)

	// Check if we can reuse an existing session
	if existingSession := sm.findReusableSession(targetKey); existingSession != nil {
		existingSession.UpdateLastUsed()
		if sm.logger != nil {
			sm.logger.Info("Reusing existing VNC session",
				"session_id", existingSession.ID,
				"target_type", sessionType,
				"node", nodeName,
				"vmid", vmid,
				"port", existingSession.Port)
		}
		return existingSession, nil
	}

	// Create new session
	sessionID := fmt.Sprintf("vnc_%d_%s", time.Now().Unix(), targetKey)

	// Create the session
	session := &VNCSession{
		ID:         sessionID,
		TargetType: sessionType,
		NodeName:   nodeName,
		VMID:       vmid,
		TargetName: targetName,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
	}

	// Create and start the VNC server for this session
	server := NewServer()
	session.Server = server

	var vncURL string
	var err error

	// Start the appropriate server based on session type
	switch sessionType {
	case SessionTypeVM, SessionTypeLXC:
		// Find the VM in the cluster
		var targetVM *api.VM
		if sm.client.Cluster != nil {
			for _, node := range sm.client.Cluster.Nodes {
				if node.Name == nodeName {
					for _, vm := range node.VMs {
						if strconv.Itoa(vm.ID) == vmid {
							targetVM = vm
							break
						}
					}
					break
				}
			}
		}

		if targetVM == nil {
			return nil, fmt.Errorf("VM not found: %s on node %s", vmid, nodeName)
		}

		vncURL, err = server.StartVMVNCServer(sm.client, targetVM)
		if err != nil {
			return nil, fmt.Errorf("failed to start VM VNC server: %w", err)
		}

	case SessionTypeNode:
		vncURL, err = server.StartNodeVNCServer(sm.client, nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to start node VNC server: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported session type: %s", sessionType)
	}

	// Set session details
	session.Port = server.GetPort()
	session.URL = vncURL

	// Store the session
	sm.sessions[sessionID] = session

	if sm.logger != nil {
		sm.logger.Info("Created new VNC session",
			"session_id", sessionID,
			"target_type", sessionType,
			"node", nodeName,
			"vmid", vmid,
			"target_name", targetName,
			"port", session.Port,
			"total_sessions", len(sm.sessions))
	}

	return session, nil
}

// GetSessionCount returns the number of currently active VNC sessions.
// This is useful for UI display and monitoring purposes.
func (sm *SessionManager) GetSessionCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.sessions)
}

// ListSessions returns a slice of all currently active VNC sessions.
// The returned sessions are copies and safe to use without additional locking.
func (sm *SessionManager) ListSessions() []*VNCSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	sessions := make([]*VNCSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// CloseSession closes a specific session by ID
func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if sm.logger != nil {
		sm.logger.Info("Closing VNC session",
			"session_id", sessionID,
			"target_type", session.TargetType,
			"target_name", session.TargetName)
	}

	// Shutdown the session
	err := session.Shutdown()
	if err != nil && sm.logger != nil {
		sm.logger.Error("Failed to shutdown session",
			"session_id", sessionID,
			"error", err)
	}

	// Remove from sessions map
	delete(sm.sessions, sessionID)

	return err
}

// CloseAllSessions closes all active sessions
func (sm *SessionManager) CloseAllSessions() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.logger != nil {
		sm.logger.Info("Closing all VNC sessions", "count", len(sm.sessions))
	}

	var errors []error
	for sessionID, session := range sm.sessions {
		if err := session.Shutdown(); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown session %s: %w", sessionID, err))
		}
	}

	// Clear all sessions
	sm.sessions = make(map[string]*VNCSession)

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors closing sessions: %v", len(errors), errors)
	}

	return nil
}

// GetSessionByTarget finds a session by target information
func (sm *SessionManager) GetSessionByTarget(sessionType SessionType, target string) (*VNCSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for _, session := range sm.sessions {
		if session.TargetType == sessionType && session.TargetName == target {
			return session, true
		}
	}

	return nil, false
}

// CleanupInactiveSessions removes sessions that haven't been accessed recently
func (sm *SessionManager) CleanupInactiveSessions(maxAge time.Duration) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	var expiredSessions []string

	// Find expired sessions
	for sessionID, session := range sm.sessions {
		if session.IsExpired(maxAge) {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	// Clean up expired sessions
	for _, sessionID := range expiredSessions {
		session := sm.sessions[sessionID]

		if sm.logger != nil {
			sm.logger.Info("Cleaning up expired VNC session",
				"session_id", sessionID,
				"target_type", session.TargetType,
				"target_name", session.TargetName,
				"age", time.Since(session.CreatedAt),
				"inactive_for", time.Since(session.LastUsed))
		}

		// Shutdown the session
		if err := session.Shutdown(); err != nil && sm.logger != nil {
			sm.logger.Error("Failed to shutdown expired session",
				"session_id", sessionID,
				"error", err)
		}

		// Remove from sessions map
		delete(sm.sessions, sessionID)
	}

	if len(expiredSessions) > 0 && sm.logger != nil {
		sm.logger.Info("Completed VNC session cleanup",
			"cleaned_sessions", len(expiredSessions),
			"remaining_sessions", len(sm.sessions))
	}
}

// Shutdown gracefully shuts down the session manager and all active sessions.
// This should be called when the application is shutting down to ensure
// proper cleanup of resources.
func (sm *SessionManager) Shutdown() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.logger != nil {
		sm.logger.Info("Shutting down VNC Session Manager",
			"active_sessions", len(sm.sessions))
	}

	// Stop the cleanup process
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}

	// Cancel the manager context
	if sm.cancelFunc != nil {
		sm.cancelFunc()
	}

	// Shut down all active sessions
	var shutdownErrors []error
	for sessionID, session := range sm.sessions {
		if err := session.Shutdown(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to shutdown session %s: %w", sessionID, err))
		}
	}

	// Clear sessions and ports
	sm.sessions = make(map[string]*VNCSession)
	sm.usedPorts = make(map[int]bool)

	// Return any shutdown errors
	if len(shutdownErrors) > 0 {
		return fmt.Errorf("encountered %d errors during shutdown: %v", len(shutdownErrors), shutdownErrors)
	}

	return nil
}

// findReusableSession finds an existing session that can be reused for the given target.
// This method must be called with the manager mutex held.
func (sm *SessionManager) findReusableSession(targetKey string) *VNCSession {
	for _, session := range sm.sessions {
		if session.GetTargetKey() == targetKey && !session.IsExpired(sm.sessionTimeout) {
			return session
		}
	}
	return nil
}

// startCleanupProcess starts a background goroutine that periodically
// cleans up expired sessions. This ensures that inactive sessions
// don't accumulate and consume resources indefinitely.
func (sm *SessionManager) startCleanupProcess() {
	sm.cleanupTicker = time.NewTicker(5 * time.Minute)

	go func() {
		defer sm.cleanupTicker.Stop()

		for {
			select {
			case <-sm.ctx.Done():
				return
			case <-sm.cleanupTicker.C:
				sm.cleanupExpiredSessions()
			}
		}
	}()
}

// cleanupExpiredSessions removes and shuts down sessions that have been
// inactive for longer than the session timeout.
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	var expiredSessions []string

	// Find expired sessions
	for sessionID, session := range sm.sessions {
		if session.IsExpired(sm.sessionTimeout) {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	// Clean up expired sessions
	for _, sessionID := range expiredSessions {
		session := sm.sessions[sessionID]

		if sm.logger != nil {
			sm.logger.Info("Cleaning up expired VNC session",
				"session_id", sessionID,
				"target_type", session.TargetType,
				"target_name", session.TargetName,
				"age", time.Since(session.CreatedAt),
				"inactive_for", time.Since(session.LastUsed))
		}

		// Shutdown the session
		if err := session.Shutdown(); err != nil && sm.logger != nil {
			sm.logger.Error("Failed to shutdown expired session",
				"session_id", sessionID,
				"error", err)
		}

		// Remove from sessions map
		delete(sm.sessions, sessionID)
	}

	if len(expiredSessions) > 0 && sm.logger != nil {
		sm.logger.Info("Completed VNC session cleanup",
			"cleaned_sessions", len(expiredSessions),
			"remaining_sessions", len(sm.sessions))
	}
}
