// Package vnc provides VNC connection services for Proxmox VMs and nodes.
// This package implements a WebSocket reverse proxy that allows noVNC clients
// to connect to Proxmox VNC sessions without requiring users to be logged
// into the Proxmox web interface.
package vnc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// ProxyConfig holds configuration for the VNC WebSocket proxy.
type ProxyConfig struct {
	// VNC proxy details from Proxmox API
	Port     string
	Ticket   string
	Password string

	// Proxmox server details
	ProxmoxHost string
	NodeName    string
	VMID        int
	VMType      string // "qemu" or "lxc"

	// Authentication
	AuthToken string

	// Connection settings
	Timeout time.Duration
}

// WebSocketProxy handles the bidirectional WebSocket proxy between
// noVNC client and Proxmox VNC websocket endpoint.
type WebSocketProxy struct {
	config   *ProxyConfig
	upgrader websocket.Upgrader
	logger   *logger.Logger
	session  SessionNotifier // Interface for session lifecycle notifications
}

// SessionNotifier interface for notifying session about connection events.
type SessionNotifier interface {
	OnClientConnected()
	OnClientDisconnected()
	UpdateLastUsed()
}

// NewWebSocketProxy creates a new WebSocket proxy with the given configuration.
func NewWebSocketProxy(config *ProxyConfig) *WebSocketProxy {
	return NewWebSocketProxyWithSession(config, nil)
}

// NewWebSocketProxyWithSession creates a new WebSocket proxy with session notifications.
func NewWebSocketProxyWithSession(config *ProxyConfig, session SessionNotifier) *WebSocketProxy {
	return NewWebSocketProxyWithSessionAndLogger(config, session, nil)
}

// NewWebSocketProxyWithSessionAndLogger creates a new WebSocket proxy with session notifications and shared logger.
func NewWebSocketProxyWithSessionAndLogger(config *ProxyConfig, session SessionNotifier, sharedLogger *logger.Logger) *WebSocketProxy {
	var proxyLogger *logger.Logger

	if sharedLogger != nil {
		proxyLogger = sharedLogger
	} else {
		// Use the global logger system for unified logging
		proxyLogger = logger.GetPackageLoggerConcrete("vnc-proxy")
	}

	proxyLogger.Info("Creating new WebSocket proxy for %s (Type: %s, Node: %s)",
		getTargetName(config), config.VMType, config.NodeName)
	proxyLogger.Debug("Proxy config - Port: %s, Proxmox Host: %s", config.Port, config.ProxmoxHost)

	return &WebSocketProxy{
		config:  config,
		logger:  proxyLogger,
		session: session,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from localhost only for security
				origin := r.Header.Get("Origin")
				if origin == "" {
					proxyLogger.Debug("WebSocket connection with no origin header (non-browser)")

					return true // Allow non-browser connections
				}
				u, err := url.Parse(origin)
				if err != nil {
					proxyLogger.Error("Failed to parse origin header: %s", origin)

					return false
				}
				allowed := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
				if !allowed {
					proxyLogger.Error("WebSocket connection rejected from origin: %s", origin)
				} else {
					proxyLogger.Debug("WebSocket connection allowed from origin: %s", origin)
				}

				return allowed
			},
		},
	}
}

// getTargetName returns a descriptive name for the target (VM name or node name).
func getTargetName(config *ProxyConfig) string {
	if config.VMType == "node" {
		return config.NodeName
	}

	return fmt.Sprintf("VM-%d", config.VMID)
}

// HandleWebSocketProxy handles incoming WebSocket connections from noVNC client
// and proxies them to the Proxmox VNC websocket endpoint.
func (p *WebSocketProxy) HandleWebSocketProxy(w http.ResponseWriter, r *http.Request) {
	targetName := getTargetName(p.config)
	p.logger.Info("Handling WebSocket proxy request for %s from %s", targetName, r.RemoteAddr)
	p.logger.Debug("Request headers: User-Agent=%s, Origin=%s",
		r.Header.Get("User-Agent"), r.Header.Get("Origin"))

	// Upgrade the HTTP connection to WebSocket
	p.logger.Debug("Upgrading HTTP connection to WebSocket for %s", targetName)

	clientConn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Error("Failed to upgrade connection to WebSocket for %s: %v", targetName, err)
		http.Error(w, fmt.Sprintf("Failed to upgrade connection: %v", err), http.StatusBadRequest)

		return
	}

	defer func() {
		p.logger.Debug("Closing client WebSocket connection for %s", targetName)
		clientConn.Close()
	}()

	p.logger.Info("WebSocket connection established with client for %s", targetName)

	// Notify session about client connection
	if p.session != nil {
		p.session.OnClientConnected()
		p.logger.Debug("Notified session about client connection for %s", targetName)
	}

	// Ensure we notify session about disconnection
	defer func() {
		if p.session != nil {
			p.session.OnClientDisconnected()
			p.logger.Debug("Notified session about client disconnection for %s", targetName)
		}
	}()

	// Create connection to Proxmox VNC websocket
	p.logger.Debug("Establishing connection to Proxmox VNC websocket for %s", targetName)

	proxmoxConn, err := p.connectToProxmox()
	if err != nil {
		p.logger.Error("Failed to connect to Proxmox VNC websocket for %s: %v", targetName, err)

		if writeErr := clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr,
				fmt.Sprintf("Failed to connect to Proxmox: %v", err))); writeErr != nil {
			p.logger.Debug("Failed to send close message to client: %v", writeErr)
		}

		return
	}

	defer func() {
		p.logger.Debug("Closing Proxmox WebSocket connection for %s", targetName)
		proxmoxConn.Close()
	}()

	p.logger.Info("WebSocket connection established with Proxmox for %s", targetName)

	// Start bidirectional proxy
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	p.logger.Info("Starting bidirectional WebSocket proxy for %s (timeout: %v)", targetName, p.config.Timeout)

	// Channel to signal when either connection closes
	done := make(chan error, 2)

	// Start ping ticker to keep connections alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-pingTicker.C:
				// Send ping to client
				if err := clientConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					p.logger.Debug("Failed to ping client for %s: %v", targetName, err)

					return
				}
				// Send ping to Proxmox
				if err := proxmoxConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					p.logger.Debug("Failed to ping Proxmox for %s: %v", targetName, err)

					return
				}

				p.logger.Debug("Sent keepalive pings for %s", targetName)

				if p.session != nil {
					p.session.UpdateLastUsed()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Proxy from client to Proxmox
	go func() {
		p.logger.Debug("Starting client->Proxmox message relay for %s", targetName)
		done <- p.proxyMessages(clientConn, proxmoxConn, "client->proxmox", targetName)
	}()

	// Proxy from Proxmox to client
	go func() {
		p.logger.Debug("Starting Proxmox->client message relay for %s", targetName)
		done <- p.proxyMessages(proxmoxConn, clientConn, "proxmox->client", targetName)
	}()

	// Wait for either connection to close or timeout
	select {
	case err := <-done:
		if err != nil {
			p.logger.Error("WebSocket proxy error for %s: %v", targetName, err)
		} else {
			p.logger.Info("WebSocket proxy connection closed normally for %s", targetName)
		}
	case <-ctx.Done():
		p.logger.Info("WebSocket proxy timeout reached for %s", targetName)
	}

	p.logger.Info("WebSocket proxy session ended for %s", targetName)
}

// connectToProxmox establishes a WebSocket connection to the Proxmox VNC endpoint.
func (p *WebSocketProxy) connectToProxmox() (*websocket.Conn, error) {
	targetName := getTargetName(p.config)
	p.logger.Debug("Building Proxmox VNC websocket URL for %s", targetName)

	// Build the Proxmox VNC websocket URL
	// Format: wss://hostname:port/api2/json/nodes/{node}/qemu/{vmid}/vncwebsocket?port={port}&vncticket={ticket}
	var vncPath string
	if p.config.VMType == api.VMTypeQemu {
		vncPath = fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/vncwebsocket",
			p.config.NodeName, p.config.VMID)
		p.logger.Debug("Using QEMU VNC path for %s: %s", targetName, vncPath)
	} else if p.config.VMType == api.VMTypeLXC {
		vncPath = fmt.Sprintf("/api2/json/nodes/%s/lxc/%d/vncwebsocket",
			p.config.NodeName, p.config.VMID)
		p.logger.Debug("Using LXC VNC path for %s: %s", targetName, vncPath)
	} else {
		// For node VNC shells
		vncPath = fmt.Sprintf("/api2/json/nodes/%s/vncwebsocket", p.config.NodeName)
		p.logger.Debug("Using node VNC path for %s: %s", targetName, vncPath)
	}

	// Add query parameters
	vncURL := fmt.Sprintf("wss://%s%s?port=%s&vncticket=%s",
		p.config.ProxmoxHost, vncPath, p.config.Port, url.QueryEscape(p.config.Ticket))

	p.logger.Debug("Proxmox VNC websocket URL for %s: %s", targetName, vncURL)

	// Create WebSocket dialer with TLS config
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip TLS verification for self-signed certs
		},
		HandshakeTimeout: 10 * time.Second,
	}

	p.logger.Debug("Configuring WebSocket dialer for %s (handshake timeout: 10s)", targetName)

	// Set up headers for authentication
	headers := make(http.Header)

	if p.config.AuthToken != "" {
		// Check if it's an API token or cookie
		if strings.HasPrefix(p.config.AuthToken, "PVEAPIToken") {
			headers.Set("Authorization", p.config.AuthToken)
			p.logger.Debug("Using API token authentication for %s", targetName)
		} else if strings.HasPrefix(p.config.AuthToken, "PVEAuthCookie=") {
			headers.Set("Cookie", p.config.AuthToken)
			p.logger.Debug("Using cookie authentication for %s", targetName)
		} else {
			// Assume API token format
			headers.Set("Authorization", p.config.AuthToken)
			p.logger.Debug("Using assumed API token authentication for %s", targetName)
		}
	} else {
		p.logger.Debug("No authentication token provided for %s", targetName)
	}

	// Connect to Proxmox VNC websocket
	p.logger.Info("Connecting to Proxmox VNC websocket for %s", targetName)

	conn, resp, err := dialer.Dial(vncURL, headers)
	if err != nil {
		if resp != nil {
			p.logger.Error("Failed to connect to Proxmox VNC websocket for %s (HTTP %d): %v",
				targetName, resp.StatusCode, err)
			resp.Body.Close() // Close response body on error

			return nil, fmt.Errorf("failed to connect to Proxmox VNC websocket (status %d): %w",
				resp.StatusCode, err)
		}

		p.logger.Error("Failed to connect to Proxmox VNC websocket for %s: %v", targetName, err)

		return nil, fmt.Errorf("failed to connect to Proxmox VNC websocket: %w", err)
	}
	// Close response body on success
	if resp != nil {
		resp.Body.Close()
	}

	p.logger.Info("Successfully connected to Proxmox VNC websocket for %s", targetName)

	return conn, nil
}

// proxyMessages handles message forwarding between WebSocket connections.
func (p *WebSocketProxy) proxyMessages(src, dst *websocket.Conn, direction, targetName string) error {
	var messageCount int

	// Set initial read deadline
	if deadlineErr := src.SetReadDeadline(time.Now().Add(5 * time.Minute)); deadlineErr != nil {
		p.logger.Debug("Failed to set initial read deadline (%s) for %s: %v", direction, targetName, deadlineErr)
	}

	src.SetPongHandler(func(string) error {
		p.logger.Debug("Pong received (%s) for %s", direction, targetName)

		if deadlineErr := src.SetReadDeadline(time.Now().Add(5 * time.Minute)); deadlineErr != nil {
			p.logger.Debug("Failed to reset read deadline (%s) for %s: %v", direction, targetName, deadlineErr)
		}

		return nil
	})

	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				p.logger.Error("Unexpected close error (%s) for %s: %v", direction, targetName, err)

				return fmt.Errorf("unexpected close error (%s): %w", direction, err)
			}

			p.logger.Debug("Normal close for %s (%s) after %d messages", direction, targetName, messageCount)

			return nil // Normal close
		}

		// Reset read deadline on each message
		if deadlineErr := src.SetReadDeadline(time.Now().Add(5 * time.Minute)); deadlineErr != nil {
			p.logger.Debug("Failed to reset read deadline (%s) for %s: %v", direction, targetName, deadlineErr)
		}

		messageCount++
		if messageCount == 1 {
			p.logger.Debug("First message received (%s) for %s: type=%d, size=%d bytes",
				direction, targetName, messageType, len(message))
		} else if messageCount%100 == 0 {
			p.logger.Debug("Message count (%s) for %s: %d messages processed",
				direction, targetName, messageCount)
		}

		err = dst.WriteMessage(messageType, message)
		if err != nil {
			p.logger.Error("Write error (%s) for %s after %d messages: %v",
				direction, targetName, messageCount, err)

			return fmt.Errorf("write error (%s): %w", direction, err)
		}

		if p.session != nil {
			p.session.UpdateLastUsed()
		}
	}
}

// CreateVMProxyConfig creates a proxy configuration for a VM VNC connection.
func CreateVMProxyConfig(client *api.Client, vm *api.VM) (*ProxyConfig, error) {
	return CreateVMProxyConfigWithLogger(client, vm, nil)
}

// CreateVMProxyConfigWithLogger creates a proxy configuration for a VM VNC connection with shared logger.
func CreateVMProxyConfigWithLogger(client *api.Client, vm *api.VM, sharedLogger *logger.Logger) (*ProxyConfig, error) {
	var configLogger *logger.Logger

	if sharedLogger != nil {
		configLogger = sharedLogger
	} else {
		// Create a logger for proxy configuration
		var err error

		configLogger, err = logger.NewInternalLogger(logger.LevelDebug, "")
		if err != nil {
			configLogger = logger.NewSimpleLogger(logger.LevelInfo)
		}
	}

	configLogger.Info("Creating VNC proxy configuration for VM: %s (ID: %d, Type: %s, Node: %s)",
		vm.Name, vm.ID, vm.Type, vm.Node)

	// Get VNC proxy details from Proxmox API
	configLogger.Debug("Requesting VNC proxy from Proxmox API for VM %s", vm.Name)

	proxy, err := client.GetVNCProxyWithWebSocket(vm)
	if err != nil {
		configLogger.Error("Failed to get VNC proxy from API for VM %s: %v", vm.Name, err)

		return nil, fmt.Errorf("failed to create VNC proxy: %w", err)
	}

	configLogger.Debug("VNC proxy response for VM %s - Port: %s, Ticket length: %d, Password length: %d",
		vm.Name, proxy.Port, len(proxy.Ticket), len(proxy.Password))

	// Extract hostname from client base URL
	baseURL := client.GetBaseURL()

	u, err := url.Parse(baseURL)
	if err != nil {
		configLogger.Error("Failed to parse client base URL for VM %s: %v", vm.Name, err)

		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	configLogger.Debug("Extracted Proxmox host for VM %s: %s", vm.Name, u.Host)

	// Get authentication token
	authToken := client.GetAuthToken()
	configLogger.Debug("Authentication token type for VM %s: %s", vm.Name, getAuthTokenType(authToken))

	// For LXC containers, use the ticket as password if no password is generated
	password := proxy.Password
	if password == "" && vm.Type == api.VMTypeLXC {
		password = proxy.Ticket

		configLogger.Debug("Using ticket as password for LXC container %s", vm.Name)
	}

	config := &ProxyConfig{
		Port:        proxy.Port,
		Ticket:      proxy.Ticket,
		Password:    password,
		ProxmoxHost: u.Host,
		NodeName:    vm.Node,
		VMID:        vm.ID,
		VMType:      vm.Type,
		AuthToken:   authToken,
		Timeout:     30 * time.Minute, // Increased to 30 minutes for VNC sessions
	}

	configLogger.Info("VNC proxy configuration created successfully for VM %s", vm.Name)

	return config, nil
}

// CreateNodeProxyConfig creates a proxy configuration for a node VNC shell connection.
func CreateNodeProxyConfig(client *api.Client, nodeName string) (*ProxyConfig, error) {
	return CreateNodeProxyConfigWithLogger(client, nodeName, nil)
}

// CreateNodeProxyConfigWithLogger creates a proxy configuration for a node VNC shell connection with shared logger.
func CreateNodeProxyConfigWithLogger(client *api.Client, nodeName string, sharedLogger *logger.Logger) (*ProxyConfig, error) {
	var configLogger *logger.Logger

	if sharedLogger != nil {
		configLogger = sharedLogger
	} else {
		// Create a logger for proxy configuration
		var err error

		configLogger, err = logger.NewInternalLogger(logger.LevelDebug, "")
		if err != nil {
			configLogger = logger.NewSimpleLogger(logger.LevelInfo)
		}
	}

	configLogger.Info("Creating VNC proxy configuration for node: %s", nodeName)

	// Get VNC shell proxy details from Proxmox API
	configLogger.Debug("Requesting VNC shell proxy from Proxmox API for node %s", nodeName)

	proxy, err := client.GetNodeVNCShellWithWebSocket(nodeName)
	if err != nil {
		configLogger.Error("Failed to get VNC shell proxy from API for node %s: %v", nodeName, err)

		return nil, fmt.Errorf("failed to create node VNC shell: %w", err)
	}

	configLogger.Debug("VNC shell proxy response for node %s - Port: %s, Ticket length: %d, Password length: %d",
		nodeName, proxy.Port, len(proxy.Ticket), len(proxy.Password))

	// Extract hostname from client base URL
	baseURL := client.GetBaseURL()

	u, err := url.Parse(baseURL)
	if err != nil {
		configLogger.Error("Failed to parse client base URL for node %s: %v", nodeName, err)

		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	configLogger.Debug("Extracted Proxmox host for node %s: %s", nodeName, u.Host)

	// Get authentication token
	authToken := client.GetAuthToken()
	configLogger.Debug("Authentication token type for node %s: %s", nodeName, getAuthTokenType(authToken))

	// For node shells, use the ticket as password if no password is generated
	password := proxy.Password
	if password == "" {
		password = proxy.Ticket

		configLogger.Debug("Using ticket as password for node shell %s", nodeName)
	}

	config := &ProxyConfig{
		Port:        proxy.Port,
		Ticket:      proxy.Ticket,
		Password:    password,
		ProxmoxHost: u.Host,
		NodeName:    nodeName,
		VMID:        0, // Not applicable for node shells
		VMType:      "node",
		AuthToken:   authToken,
		Timeout:     30 * time.Minute, // Increased to 30 minutes for VNC sessions
	}

	configLogger.Info("VNC proxy configuration created successfully for node %s", nodeName)

	return config, nil
}

// getAuthTokenType returns a description of the authentication token type.
func getAuthTokenType(token string) string {
	if token == "" {
		return "none"
	}

	if strings.HasPrefix(token, "PVEAPIToken") {
		return "API token"
	}

	if strings.HasPrefix(token, "PVEAuthCookie=") {
		return "auth cookie"
	}

	return "unknown"
}
