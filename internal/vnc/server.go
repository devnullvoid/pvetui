// Package vnc provides VNC connection services for Proxmox VMs and nodes.
package vnc

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

//go:embed novnc/*
var novncFiles embed.FS

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
	// Create a logger for VNC server operations
	serverLogger, err := logger.NewInternalLogger(logger.LevelDebug, "")
	if err != nil {
		// Fallback to a simple logger if file logging fails
		serverLogger = logger.NewSimpleLogger(logger.LevelInfo)
	}

	serverLogger.Debug("Creating new VNC server instance")

	return &Server{
		logger: serverLogger,
	}
}

// StartVMVNCServer starts the embedded server for a VM VNC connection
func (s *Server) StartVMVNCServer(client *api.Client, vm *api.VM) (string, error) {
	s.logger.Info("Starting VM VNC server for: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	// Create proxy configuration
	s.logger.Debug("Creating VNC proxy configuration for VM %s", vm.Name)
	config, err := CreateVMProxyConfig(client, vm)
	if err != nil {
		s.logger.Error("Failed to create VM proxy config for %s: %v", vm.Name, err)
		return "", fmt.Errorf("failed to create VM proxy config: %w", err)
	}

	s.logger.Debug("VM proxy config created - Port: %s, VM Type: %s", config.Port, config.VMType)

	// Create WebSocket proxy
	s.logger.Debug("Creating WebSocket proxy for VM %s", vm.Name)
	s.proxy = NewWebSocketProxy(config)

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
	s.logger.Info("Starting node VNC server for: %s", nodeName)

	// Create proxy configuration
	s.logger.Debug("Creating VNC proxy configuration for node %s", nodeName)
	config, err := CreateNodeProxyConfig(client, nodeName)
	if err != nil {
		s.logger.Error("Failed to create node proxy config for %s: %v", nodeName, err)
		return "", fmt.Errorf("failed to create node proxy config: %w", err)
	}

	s.logger.Debug("Node proxy config created - Port: %s", config.Port)

	// Create WebSocket proxy
	s.logger.Debug("Creating WebSocket proxy for node %s", nodeName)
	s.proxy = NewWebSocketProxy(config)

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

	// Serve all noVNC client files from embedded filesystem
	s.logger.Debug("Setting up noVNC file server")
	novncFS, err := fs.Sub(novncFiles, "novnc")
	if err != nil {
		s.logger.Error("Failed to create noVNC filesystem: %v", err)
		return fmt.Errorf("failed to create noVNC filesystem: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(novncFS)))

	// WebSocket proxy endpoint
	s.logger.Debug("Setting up WebSocket proxy endpoint")
	mux.HandleFunc("/vnc-proxy", s.proxy.HandleWebSocketProxy)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", s.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
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
