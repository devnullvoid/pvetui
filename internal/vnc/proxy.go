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

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gorilla/websocket"
)

// ProxyConfig holds configuration for the VNC WebSocket proxy
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
// noVNC client and Proxmox VNC websocket endpoint
type WebSocketProxy struct {
	config   *ProxyConfig
	upgrader websocket.Upgrader
}

// NewWebSocketProxy creates a new WebSocket proxy with the given configuration
func NewWebSocketProxy(config *ProxyConfig) *WebSocketProxy {
	return &WebSocketProxy{
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from localhost only for security
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // Allow non-browser connections
				}
				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				return u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
			},
		},
	}
}

// HandleWebSocketProxy handles incoming WebSocket connections from noVNC client
// and proxies them to the Proxmox VNC websocket endpoint
func (p *WebSocketProxy) HandleWebSocketProxy(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to WebSocket
	clientConn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to upgrade connection: %v", err), http.StatusBadRequest)
		return
	}
	defer clientConn.Close()

	// Create connection to Proxmox VNC websocket
	proxmoxConn, err := p.connectToProxmox()
	if err != nil {
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr,
				fmt.Sprintf("Failed to connect to Proxmox: %v", err)))
		return
	}
	defer proxmoxConn.Close()

	// Start bidirectional proxy
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	// Channel to signal when either connection closes
	done := make(chan error, 2)

	// Proxy from client to Proxmox
	go func() {
		done <- p.proxyMessages(clientConn, proxmoxConn, "client->proxmox")
	}()

	// Proxy from Proxmox to client
	go func() {
		done <- p.proxyMessages(proxmoxConn, clientConn, "proxmox->client")
	}()

	// Wait for either connection to close or timeout
	select {
	case err := <-done:
		if err != nil {
			// Error occurred, connection will be closed
			return
		}
	case <-ctx.Done():
		// Timeout occurred, connection will be closed
		return
	}
}

// connectToProxmox establishes a WebSocket connection to the Proxmox VNC endpoint
func (p *WebSocketProxy) connectToProxmox() (*websocket.Conn, error) {
	// Build the Proxmox VNC websocket URL
	// Format: wss://hostname:port/api2/json/nodes/{node}/qemu/{vmid}/vncwebsocket?port={port}&vncticket={ticket}
	var vncPath string
	if p.config.VMType == "qemu" {
		vncPath = fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/vncwebsocket",
			p.config.NodeName, p.config.VMID)
	} else if p.config.VMType == "lxc" {
		vncPath = fmt.Sprintf("/api2/json/nodes/%s/lxc/%d/vncwebsocket",
			p.config.NodeName, p.config.VMID)
	} else {
		// For node VNC shells
		vncPath = fmt.Sprintf("/api2/json/nodes/%s/vncwebsocket", p.config.NodeName)
	}

	// Add query parameters
	vncURL := fmt.Sprintf("wss://%s%s?port=%s&vncticket=%s",
		p.config.ProxmoxHost, vncPath, p.config.Port, url.QueryEscape(p.config.Ticket))

	// Create WebSocket dialer with TLS config
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip TLS verification for self-signed certs
		},
		HandshakeTimeout: 10 * time.Second,
	}

	// Set up headers for authentication
	headers := make(http.Header)
	if p.config.AuthToken != "" {
		// Check if it's an API token or cookie
		if strings.HasPrefix(p.config.AuthToken, "PVEAPIToken") {
			headers.Set("Authorization", p.config.AuthToken)
		} else if strings.HasPrefix(p.config.AuthToken, "PVEAuthCookie=") {
			headers.Set("Cookie", p.config.AuthToken)
		} else {
			// Assume API token format
			headers.Set("Authorization", p.config.AuthToken)
		}
	}

	// Connect to Proxmox VNC websocket
	conn, resp, err := dialer.Dial(vncURL, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("failed to connect to Proxmox VNC websocket (status %d): %w",
				resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to connect to Proxmox VNC websocket: %w", err)
	}

	return conn, nil
}

// proxyMessages handles message forwarding between WebSocket connections
func (p *WebSocketProxy) proxyMessages(src, dst *websocket.Conn, direction string) error {
	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return fmt.Errorf("unexpected close error (%s): %w", direction, err)
			}
			return nil // Normal close
		}

		err = dst.WriteMessage(messageType, message)
		if err != nil {
			return fmt.Errorf("write error (%s): %w", direction, err)
		}
	}
}

// CreateVMProxyConfig creates a proxy configuration for a VM VNC connection
func CreateVMProxyConfig(client *api.Client, vm *api.VM) (*ProxyConfig, error) {
	// Get VNC proxy details from Proxmox API
	proxy, err := client.GetVNCProxyWithWebSocket(vm)
	if err != nil {
		return nil, fmt.Errorf("failed to create VNC proxy: %w", err)
	}

	// Extract hostname from client base URL
	baseURL := client.GetBaseURL()
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Get authentication token
	authToken := client.GetAuthToken()

	// For LXC containers, use the ticket as password if no password is generated
	password := proxy.Password
	if password == "" && vm.Type == "lxc" {
		password = proxy.Ticket
	}

	return &ProxyConfig{
		Port:        proxy.Port,
		Ticket:      proxy.Ticket,
		Password:    password,
		ProxmoxHost: u.Host,
		NodeName:    vm.Node,
		VMID:        vm.ID,
		VMType:      vm.Type,
		AuthToken:   authToken,
		Timeout:     30 * time.Second,
	}, nil
}

// CreateNodeProxyConfig creates a proxy configuration for a node VNC shell connection
func CreateNodeProxyConfig(client *api.Client, nodeName string) (*ProxyConfig, error) {
	// Get VNC shell proxy details from Proxmox API
	proxy, err := client.GetNodeVNCShellWithWebSocket(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to create node VNC shell: %w", err)
	}

	// Extract hostname from client base URL
	baseURL := client.GetBaseURL()
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Get authentication token
	authToken := client.GetAuthToken()

	return &ProxyConfig{
		Port:        proxy.Port,
		Ticket:      proxy.Ticket,
		Password:    proxy.Password,
		ProxmoxHost: u.Host,
		NodeName:    nodeName,
		VMID:        0, // Not applicable for node shells
		VMType:      "node",
		AuthToken:   authToken,
		Timeout:     30 * time.Second,
	}, nil
}
