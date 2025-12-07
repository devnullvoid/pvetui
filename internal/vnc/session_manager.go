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

	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// SessionType represents the type of VNC session.
type SessionType string

const (
	SessionTypeVM   SessionType = "vm"
	SessionTypeLXC  SessionType = "lxc"
	SessionTypeNode SessionType = "node"
)

// SessionState represents the current state of a VNC session.
type SessionState int

const (
	SessionStateActive       SessionState = iota // Session is active and ready for connections
	SessionStateConnected                        // Client is currently connected
	SessionStateDisconnected                     // Client disconnected, session may be reusable
	SessionStateClosed                           // Session is closed and should be cleaned up
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
	CreatedAt time.Time    // When this session was created
	LastUsed  time.Time    // Last time this session was accessed
	State     SessionState // Current connection state

	// VNC connection details
	ProxyConfig *ProxyConfig // VNC proxy configuration from Proxmox API

	// Server management
	Server *Server // The HTTP server instance for this session

	// Connection tracking
	activeConnections int             // Number of active WebSocket connections
	disconnectChan    chan struct{}   // Channel to signal disconnection
	sessionManager    *SessionManager // Reference to session manager for cleanup notifications

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

// OnClientConnected is called when a client connects to this session.
func (s *VNCSession) OnClientConnected() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.activeConnections++
	s.State = SessionStateConnected
	s.LastUsed = time.Now()
}

// OnClientDisconnected is called when a client disconnects from this session.
func (s *VNCSession) OnClientDisconnected() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.activeConnections > 0 {
		s.activeConnections--
	}

	if s.activeConnections == 0 {
		s.State = SessionStateDisconnected
		// Signal disconnection for immediate cleanup consideration
		select {
		case s.disconnectChan <- struct{}{}:
		default: // Non-blocking send
		}
	}
}

// IsReusable returns true if this session can be reused for new connections.
func (s *VNCSession) IsReusable() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.State == SessionStateActive || s.State == SessionStateDisconnected
}

// GetConnectionCount returns the number of active connections to this session.
func (s *VNCSession) GetConnectionCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.activeConnections
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

	// Mark session as closed
	s.State = SessionStateClosed

	// Cancel the session context
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// Close the disconnect channel
	if s.disconnectChan != nil {
		close(s.disconnectChan)
		s.disconnectChan = nil
	}

	// Stop the HTTP server
	if s.Server != nil {
		return s.Server.Stop()
	}

	return nil
}

// SessionCountCallback is called when the session count changes.
type SessionCountCallback func(count int)

// SessionManager manages multiple concurrent VNC sessions with comprehensive
// lifecycle management, automatic cleanup, and smart session reuse.
//
// The SessionManager provides:
// - Thread-safe session creation and management
// - Dynamic port allocation to avoid conflicts
// - Automatic cleanup of inactive sessions
// - Session reuse for the same target
// - Background monitoring and maintenance
// - Real-time session count notifications via callbacks
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

	// Callback for session count changes
	sessionCountCallback SessionCountCallback // Called when session count changes

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
	return NewSessionManagerWithLogger(client, nil)
}

// NewSessionManagerWithLogger creates a new VNC session manager with a shared logger.
func NewSessionManagerWithLogger(client *api.Client, sharedLogger *logger.Logger) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())

	var sessionLogger *logger.Logger
	if sharedLogger != nil {
		sessionLogger = sharedLogger
	} else {
		// Use the global logger system for unified logging
		sessionLogger = logger.GetPackageLoggerConcrete("vnc-session-manager")
	}

	manager := &SessionManager{
		sessions:       make(map[string]*VNCSession),
		usedPorts:      make(map[int]bool),
		client:         client,
		logger:         sessionLogger,
		ctx:            ctx,
		cancelFunc:     cancel,
		sessionTimeout: 24 * time.Hour, // Sessions expire after 24 hours (effectively disabled)
	}

	// Configure port range for dynamic allocation
	manager.portRange.start = 8080
	manager.portRange.end = 8180

	// Start background cleanup process
	manager.startCleanupProcess()

	if sessionLogger != nil {
		sessionLogger.Info("VNC Session Manager initialized: port_range=%s, session_timeout=%v, cleanup_interval=30m",
			fmt.Sprintf("%d-%d", manager.portRange.start, manager.portRange.end), manager.sessionTimeout)
	}

	return manager
}

// UpdateClient updates the session manager's client (used when switching profiles).
func (sm *SessionManager) UpdateClient(client *api.Client) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.logger.Info("Updating session manager client for profile switch")
	sm.client = client
}

// CreateVMSession creates a new VNC session for a VM.
// If a session already exists for the same VM, it may be reused.
func (sm *SessionManager) CreateVMSession(vm *api.VM) (*VNCSession, error) {
	return sm.CreateVMSessionWithClient(sm.client, vm)
}

// CreateVMSessionWithClient creates a new VNC session for a VM using a specific client.
func (sm *SessionManager) CreateVMSessionWithClient(client *api.Client, vm *api.VM) (*VNCSession, error) {
	var sessionType SessionType
	if vm.Type == "qemu" {
		sessionType = SessionTypeVM
	} else if vm.Type == "lxc" {
		sessionType = SessionTypeLXC
	} else {
		return nil, fmt.Errorf("unsupported VM type: %s", vm.Type)
	}

	return sm.CreateSessionWithClient(context.Background(), client, sessionType, vm.Node, strconv.Itoa(vm.ID), vm.Name)
}

// CreateNodeSession creates a new VNC session for a node shell.
// If a session already exists for the same node, it may be reused.
func (sm *SessionManager) CreateNodeSession(nodeName string) (*VNCSession, error) {
	return sm.CreateNodeSessionWithClient(sm.client, nodeName)
}

// CreateNodeSessionWithClient creates a new VNC session for a node shell using a specific client.
func (sm *SessionManager) CreateNodeSessionWithClient(client *api.Client, nodeName string) (*VNCSession, error) {
	return sm.CreateSessionWithClient(context.Background(), client, SessionTypeNode, nodeName, "", nodeName)
}

// CreateSession creates a new VNC session for the specified target using the default client.
func (sm *SessionManager) CreateSession(ctx context.Context, sessionType SessionType, nodeName, vmid, targetName string) (*VNCSession, error) {
	return sm.CreateSessionWithClient(ctx, sm.client, sessionType, nodeName, vmid, targetName)
}

// CreateSessionWithClient creates a new VNC session for the specified target using a specific client.
func (sm *SessionManager) CreateSessionWithClient(ctx context.Context, client *api.Client, sessionType SessionType, nodeName, vmid, targetName string) (*VNCSession, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	targetKey := fmt.Sprintf("%s:%s:%s", sessionType, nodeName, vmid)

	// Check if we can reuse an existing session
	if existingSession := sm.findReusableSession(targetKey); existingSession != nil {
		existingSession.UpdateLastUsed()

		// If session was disconnected, reset it to active state
		if existingSession.State == SessionStateDisconnected {
			existingSession.mutex.Lock()
			existingSession.State = SessionStateActive
			existingSession.mutex.Unlock()

			if sm.logger != nil {
				sm.logger.Info("Reactivating disconnected VNC session: session_id=%s, target_type=%s, node=%s, vmid=%s, port=%d",
					existingSession.ID, sessionType, nodeName, vmid, existingSession.Port)
			}
		} else {
			if sm.logger != nil {
				sm.logger.Info("Reusing existing VNC session: session_id=%s, target_type=%s, node=%s, vmid=%s, port=%d",
					existingSession.ID, sessionType, nodeName, vmid, existingSession.Port)
			}
		}

		return existingSession, nil
	}

	// Create new session
	sessionID := fmt.Sprintf("vnc_%d_%s", time.Now().Unix(), targetKey)

	// Create the session
	session := &VNCSession{
		ID:                sessionID,
		TargetType:        sessionType,
		NodeName:          nodeName,
		VMID:              vmid,
		TargetName:        targetName,
		CreatedAt:         time.Now(),
		LastUsed:          time.Now(),
		State:             SessionStateActive,
		activeConnections: 0,
		disconnectChan:    make(chan struct{}, 1),
		sessionManager:    sm,
	}

	// Create and start the VNC server for this session
	server := NewServerWithLogger(sm.logger)
	session.Server = server

	var vncURL string

	var err error

	// Start the appropriate server based on session type
	switch sessionType {
	case SessionTypeVM, SessionTypeLXC:
		// Find the VM in the cluster
		var targetVM *api.VM

		if client.Cluster != nil {
			for _, node := range client.Cluster.Nodes {
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

		vncURL, err = server.StartVMVNCServerWithSession(client, targetVM, session)
		if err != nil {
			return nil, fmt.Errorf("failed to start VM VNC server: %w", err)
		}

	case SessionTypeNode:
		vncURL, err = server.StartNodeVNCServerWithSession(client, nodeName, session)
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

	// Notify callback of session count change
	sm.notifySessionCountChange()

	// Start monitoring for disconnections
	go sm.monitorSessionDisconnect(session)

	if sm.logger != nil {
		sm.logger.Info("Created new VNC session: session_id=%s, target_type=%s, node=%s, vmid=%s, target_name=%s, port=%d, total_sessions=%d",
			sessionID, sessionType, nodeName, vmid, targetName, session.Port, len(sm.sessions))
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

// SetSessionCountCallback registers a callback function that will be called
// whenever the session count changes. This allows for real-time UI updates.
func (sm *SessionManager) SetSessionCountCallback(callback SessionCountCallback) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.sessionCountCallback = callback
}

// notifySessionCountChange calls the registered callback with the current session count.
// This method must be called with the manager mutex held.
func (sm *SessionManager) notifySessionCountChange() {
	if sm.sessionCountCallback != nil {
		count := len(sm.sessions)
		// Call callback in a separate goroutine to avoid blocking session management
		go sm.sessionCountCallback(count)
	}
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

// CloseSession closes a specific session by ID.
func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if sm.logger != nil {
		sm.logger.Info("Closing VNC session: session_id=%s, target_type=%s, target_name=%s",
			sessionID, session.TargetType, session.TargetName)
	}

	// Shutdown the session
	err := session.Shutdown()
	if err != nil && sm.logger != nil {
		sm.logger.Error("Failed to shutdown session: session_id=%s, error=%v", sessionID, err)
	}

	// Remove from sessions map
	delete(sm.sessions, sessionID)

	// Notify callback of session count change
	sm.notifySessionCountChange()

	return err
}

// CloseAllSessions closes all active sessions.
func (sm *SessionManager) CloseAllSessions() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.logger != nil {
		sm.logger.Info("Closing all VNC sessions: count=%d", len(sm.sessions))
	}

	var errors []error

	for sessionID, session := range sm.sessions {
		if err := session.Shutdown(); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown session %s: %w", sessionID, err))
		}
	}

	// Clear all sessions
	sm.sessions = make(map[string]*VNCSession)

	// Notify callback of session count change
	sm.notifySessionCountChange()

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors closing sessions: %v", len(errors), errors)
	}

	return nil
}

// GetSessionByTarget finds a session by target information.
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

// CleanupInactiveSessions removes sessions that haven't been accessed recently.
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

	// Only log and clean up if there are expired sessions
	if len(expiredSessions) == 0 {
		return
	}

	if sm.logger != nil {
		sm.logger.Info("Found %d expired VNC sessions to clean up", len(expiredSessions))
	}

	// Clean up expired sessions
	for _, sessionID := range expiredSessions {
		session := sm.sessions[sessionID]

		if sm.logger != nil {
			sm.logger.Info("Cleaning up expired VNC session: session_id=%s, target_type=%s, target_name=%s, age=%v, inactive_for=%v",
				sessionID, session.TargetType, session.TargetName, time.Since(session.CreatedAt), time.Since(session.LastUsed))
		}

		// Shutdown the session
		if err := session.Shutdown(); err != nil && sm.logger != nil {
			sm.logger.Error("Failed to shutdown expired session: session_id=%s, error=%v", sessionID, err)
		}

		// Remove from sessions map
		delete(sm.sessions, sessionID)
	}

	// Notify callback of session count change
	sm.notifySessionCountChange()

	if sm.logger != nil {
		sm.logger.Info("Completed VNC session cleanup: cleaned_sessions=%d, remaining_sessions=%d",
			len(expiredSessions), len(sm.sessions))
	}
}

// Shutdown gracefully shuts down the session manager and all active sessions.
// This should be called when the application is shutting down to ensure
// proper cleanup of resources.
func (sm *SessionManager) Shutdown() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.logger != nil {
		sm.logger.Info("Shutting down VNC Session Manager: active_sessions=%d", len(sm.sessions))
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
		if session.GetTargetKey() == targetKey && session.IsReusable() && !session.IsExpired(sm.sessionTimeout) {
			return session
		}
	}

	return nil
}

// startCleanupProcess starts a background goroutine that periodically
// cleans up expired sessions. This ensures that inactive sessions
// don't accumulate and consume resources indefinitely.
func (sm *SessionManager) startCleanupProcess() {
	sm.cleanupTicker = time.NewTicker(30 * time.Minute)

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
			sm.logger.Info("Cleaning up expired VNC session: session_id=%s, target_type=%s, target_name=%s, age=%v, inactive_for=%v",
				sessionID, session.TargetType, session.TargetName, time.Since(session.CreatedAt), time.Since(session.LastUsed))
		}

		// Shutdown the session
		if err := session.Shutdown(); err != nil && sm.logger != nil {
			sm.logger.Error("Failed to shutdown expired session: session_id=%s, error=%v", sessionID, err)
		}

		// Remove from sessions map
		delete(sm.sessions, sessionID)
	}

	if len(expiredSessions) > 0 && sm.logger != nil {
		sm.logger.Info("Completed VNC session cleanup: cleaned_sessions=%d, remaining_sessions=%d",
			len(expiredSessions), len(sm.sessions))
	}
}

// monitorSessionDisconnect monitors a session for client disconnections
// and handles immediate cleanup when clients disconnect.
func (sm *SessionManager) monitorSessionDisconnect(session *VNCSession) {
	for {
		select {
		case <-session.disconnectChan:
			// Client disconnected, consider cleanup after a grace period
			if sm.logger != nil {
				sm.logger.Info("Client disconnected from VNC session: session_id=%s, target_type=%s, target_name=%s, active_connections=%d",
					session.ID, session.TargetType, session.TargetName, session.GetConnectionCount())
			}

			// Wait a short grace period to allow for reconnections
			time.Sleep(5 * time.Second)

			// Check if session is still disconnected and remove it
			sm.mutex.Lock()
			if existingSession, exists := sm.sessions[session.ID]; exists {
				if existingSession.GetConnectionCount() == 0 && existingSession.State == SessionStateDisconnected {
					if sm.logger != nil {
						sm.logger.Info("Removing disconnected VNC session after grace period: session_id=%s, target_type=%s, target_name=%s",
							session.ID, session.TargetType, session.TargetName)
					}

					// Shutdown the session
					if err := existingSession.Shutdown(); err != nil && sm.logger != nil {
						sm.logger.Error("Failed to shutdown disconnected session: session_id=%s, error=%v", session.ID, err)
					}

					// Remove from sessions map
					delete(sm.sessions, session.ID)

					// Free up the port
					if existingSession.Port > 0 {
						delete(sm.usedPorts, existingSession.Port)
					}

					// Notify callback of session count change
					sm.notifySessionCountChange()
				}
			}
			sm.mutex.Unlock()

		case <-sm.ctx.Done():
			// Session manager is shutting down
			return
		}
	}
}
