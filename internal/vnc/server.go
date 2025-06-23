// Package vnc provides VNC connection services for Proxmox VMs and nodes.
package vnc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// Server represents an embedded HTTP server for serving noVNC client
type Server struct {
	httpServer *http.Server
	proxy      *WebSocketProxy
	port       int
	mu         sync.Mutex
	running    bool
	logger     *logger.Logger
}

// NewServer creates a new embedded HTTP server for VNC connections
func NewServer() *Server {
	return NewServerWithLogger(nil)
}

// NewServerWithLogger creates a new embedded HTTP server with a shared logger
func NewServerWithLogger(sharedLogger *logger.Logger) *Server {
	var serverLogger *logger.Logger

	if sharedLogger != nil {
		serverLogger = sharedLogger
	} else {
		// Use the global logger system for unified logging
		serverLogger = logger.GetPackageLoggerConcrete("vnc-server")
	}

	serverLogger.Debug("Creating new VNC server instance")

	return &Server{
		logger: serverLogger,
	}
}

// StartVMVNCServer starts the embedded server for a VM VNC connection
func (s *Server) StartVMVNCServer(client *api.Client, vm *api.VM) (string, error) {
	return s.StartVMVNCServerWithSession(client, vm, nil)
}

// StartVMVNCServerWithSession starts the embedded server for a VM VNC connection with session notifications
func (s *Server) StartVMVNCServerWithSession(client *api.Client, vm *api.VM, session SessionNotifier) (string, error) {
	s.logger.Info("Starting VM VNC server for: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	// Create proxy configuration
	s.logger.Debug("Creating VNC proxy configuration for VM %s", vm.Name)
	config, err := CreateVMProxyConfigWithLogger(client, vm, s.logger)
	if err != nil {
		s.logger.Error("Failed to create VM proxy config for %s: %v", vm.Name, err)
		return "", fmt.Errorf("failed to create VM proxy config: %w", err)
	}

	s.logger.Debug("VM proxy config created - Port: %s, VM Type: %s", config.Port, config.VMType)

	// Create WebSocket proxy with session notifications
	s.logger.Debug("Creating WebSocket proxy for VM %s", vm.Name)
	s.proxy = NewWebSocketProxyWithSessionAndLogger(config, session, s.logger)

	// Start HTTP server
	s.logger.Debug("Starting HTTP server for VM %s", vm.Name)
	if err := s.startHTTPServer(); err != nil {
		s.logger.Error("Failed to start HTTP server for VM %s: %v", vm.Name, err)
		return "", fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Generate noVNC URL with parameters expected by vnc_lite.html
	vncURL := fmt.Sprintf("http://localhost:%d/vnc_lite.html?host=localhost&port=%d&password=%s&path=vnc-proxy",
		s.port, s.port, url.QueryEscape(config.Password))

	s.logger.Info("VM VNC server started successfully for %s on port %d", vm.Name, s.port)
	s.logger.Debug("VM VNC URL generated: %s", vncURL)

	return vncURL, nil
}

// StartNodeVNCServer starts the embedded server for a node VNC shell connection
func (s *Server) StartNodeVNCServer(client *api.Client, nodeName string) (string, error) {
	return s.StartNodeVNCServerWithSession(client, nodeName, nil)
}

// StartNodeVNCServerWithSession starts the embedded server for a node VNC shell connection with session notifications
func (s *Server) StartNodeVNCServerWithSession(client *api.Client, nodeName string, session SessionNotifier) (string, error) {
	s.logger.Info("Starting node VNC server for: %s", nodeName)

	// Create proxy configuration
	s.logger.Debug("Creating VNC proxy configuration for node %s", nodeName)
	config, err := CreateNodeProxyConfigWithLogger(client, nodeName, s.logger)
	if err != nil {
		s.logger.Error("Failed to create node proxy config for %s: %v", nodeName, err)
		return "", fmt.Errorf("failed to create node proxy config: %w", err)
	}

	s.logger.Debug("Node proxy config created - Port: %s", config.Port)

	// Create WebSocket proxy with session notifications
	s.logger.Debug("Creating WebSocket proxy for node %s", nodeName)
	s.proxy = NewWebSocketProxyWithSessionAndLogger(config, session, s.logger)

	// Start HTTP server
	s.logger.Debug("Starting HTTP server for node %s", nodeName)
	if err := s.startHTTPServer(); err != nil {
		s.logger.Error("Failed to start HTTP server for node %s: %v", nodeName, err)
		return "", fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Generate noVNC URL with parameters expected by vnc_lite.html
	vncURL := fmt.Sprintf("http://localhost:%d/vnc_lite.html?host=localhost&port=%d&password=%s&path=vnc-proxy",
		s.port, s.port, url.QueryEscape(config.Password))

	s.logger.Info("Node VNC server started successfully for %s on port %d", nodeName, s.port)
	s.logger.Debug("Node VNC URL generated: %s", vncURL)

	return vncURL, nil
}

// startHTTPServer starts the embedded HTTP server on an available port
func (s *Server) startHTTPServer() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.logger.Debug("HTTP server already running on port %d", s.port)
		return nil // Already running
	}

	s.logger.Debug("Finding available port for HTTP server")

	// Find an available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		s.logger.Error("Failed to find available port: %v", err)
		return fmt.Errorf("failed to find available port: %w", err)
	}
	s.port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	s.logger.Info("Allocated port %d for HTTP server", s.port)

	// Create HTTP server
	mux := http.NewServeMux()

	// Serve all noVNC client files from submodule directory
	s.logger.Debug("Setting up noVNC file server")
	novncPath := filepath.Join("internal", "vnc", "novnc")
	mux.Handle("/", http.FileServer(http.Dir(novncPath)))

	// WebSocket proxy endpoint
	s.logger.Debug("Setting up WebSocket proxy endpoint")
	mux.HandleFunc("/vnc-proxy", s.proxy.HandleWebSocketProxy)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", s.port),
		Handler:      mux,
		ReadTimeout:  0,                // No timeout for WebSocket connections
		WriteTimeout: 0,                // No timeout for WebSocket connections
		IdleTimeout:  10 * time.Minute, // 10 minutes idle timeout
	}

	s.logger.Debug("Starting HTTP server on %s", s.httpServer.Addr)

	// Start server in background
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error: %v", err)
		} else if err == http.ErrServerClosed {
			s.logger.Debug("HTTP server closed normally")
		}
	}()

	s.running = true
	s.logger.Info("HTTP server started successfully on port %d", s.port)
	return nil
}

// Stop stops the embedded HTTP server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.httpServer == nil {
		s.logger.Debug("HTTP server not running, no action needed")
		return nil
	}

	s.logger.Info("Stopping HTTP server on port %d", s.port)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		s.logger.Error("Failed to shutdown HTTP server gracefully: %v", err)
	} else {
		s.logger.Debug("HTTP server shutdown gracefully")
	}

	s.running = false
	s.httpServer = nil
	s.proxy = nil

	s.logger.Info("HTTP server stopped successfully")
	return err
}

// GetPort returns the port the server is running on
func (s *Server) GetPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debug("Requested server port: %d", s.port)
	return s.port
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Debug("Requested server running status: %t", s.running)
	return s.running
}
