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
}

// NewServer creates a new embedded HTTP server for VNC connections
func NewServer() *Server {
	return &Server{}
}

// StartVMVNCServer starts the embedded server for a VM VNC connection
func (s *Server) StartVMVNCServer(client *api.Client, vm *api.VM) (string, error) {
	// Create proxy configuration
	config, err := CreateVMProxyConfig(client, vm)
	if err != nil {
		return "", fmt.Errorf("failed to create VM proxy config: %w", err)
	}

	// Create WebSocket proxy
	s.proxy = NewWebSocketProxy(config)

	// Start HTTP server
	if err := s.startHTTPServer(); err != nil {
		return "", fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Generate noVNC URL with parameters expected by vnc_lite.html
	vncURL := fmt.Sprintf("http://localhost:%d/vnc_lite.html?host=localhost&port=%d&password=%s&path=vnc-proxy",
		s.port, s.port, url.QueryEscape(config.Password))

	return vncURL, nil
}

// StartNodeVNCServer starts the embedded server for a node VNC shell connection
func (s *Server) StartNodeVNCServer(client *api.Client, nodeName string) (string, error) {
	// Create proxy configuration
	config, err := CreateNodeProxyConfig(client, nodeName)
	if err != nil {
		return "", fmt.Errorf("failed to create node proxy config: %w", err)
	}

	// Create WebSocket proxy
	s.proxy = NewWebSocketProxy(config)

	// Start HTTP server
	if err := s.startHTTPServer(); err != nil {
		return "", fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Generate noVNC URL with parameters expected by vnc_lite.html
	vncURL := fmt.Sprintf("http://localhost:%d/vnc_lite.html?host=localhost&port=%d&password=%s&path=vnc-proxy",
		s.port, s.port, url.QueryEscape(config.Password))

	return vncURL, nil
}

// startHTTPServer starts the embedded HTTP server on an available port
func (s *Server) startHTTPServer() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil // Already running
	}

	// Find an available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}
	s.port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Create HTTP server
	mux := http.NewServeMux()

	// Serve all noVNC client files from embedded filesystem
	novncFS, err := fs.Sub(novncFiles, "novnc")
	if err != nil {
		return fmt.Errorf("failed to create noVNC filesystem: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(novncFS)))

	// WebSocket proxy endpoint
	mux.HandleFunc("/vnc-proxy", s.proxy.HandleWebSocketProxy)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", s.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Server error occurred, but we can't log to stdout as it corrupts the TUI
			// The error will be handled by the calling code
		}
	}()

	s.running = true
	return nil
}

// Stop stops the embedded HTTP server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(ctx)
	s.running = false
	s.httpServer = nil
	s.proxy = nil

	return err
}

// GetPort returns the port the server is running on
func (s *Server) GetPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
