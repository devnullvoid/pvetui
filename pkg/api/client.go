package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Telmate/proxmox-api-go/proxmox"
)

// Cluster represents aggregated Proxmox cluster metrics
type Cluster struct {
	TotalNodes   int
	OnlineNodes  int
	TotalCPU     float64
	TotalMemory  int64
	UsedMemory   int64
	TotalStorage int64
	UsedStorage  int64
	ClusterName  string
	PVEVersion   string
}

// GetClusterStatus retrieves and parses cluster status into structured format
func (c *Client) GetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{}

	// Get all nodes first to calculate cluster-wide metrics
	nodes, err := c.ListNodes()
	if err != nil {
		return nil, err
	}

	cluster.TotalNodes = len(nodes)
	for _, node := range nodes {
		if node.Online {
			cluster.OnlineNodes++
		}
		cluster.TotalCPU += node.CPUUsage
		cluster.TotalMemory += node.MemoryTotal
		cluster.UsedMemory += node.MemoryUsed
		// Aggregate storage metrics
		cluster.TotalStorage += node.TotalStorage
		cluster.UsedStorage += node.UsedStorage

		// Get version from first node
		if cluster.PVEVersion == "" {
			cluster.PVEVersion = node.Version
		}
	}

	// Get cluster name from first node's domain
	if len(nodes) > 0 {
		parts := strings.Split(nodes[0].Name, ".")
		if len(parts) > 1 {
			cluster.ClusterName = parts[1]
		} else {
			cluster.ClusterName = "proxmox"
		}
	}

	return cluster, nil
}

// Client is a Proxmox API client
type Client struct {
	ProxClient *proxmox.Client
}

// NewClient initializes a new Proxmox API client with optimized defaults
func NewClient(addr, user, password, realm string, insecure bool) (*Client, error) {
	// Validate input parameters
	if addr == "" {
		return nil, fmt.Errorf("proxmox address cannot be empty")
	}

	// Construct base URL
	baseURL := strings.TrimRight(addr, "/")
	if !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	if !strings.Contains(baseURL, ":8006") {
		baseURL += ":8006"
	}
	fmt.Printf("Proxmox API URL: %s\n", baseURL)

	// Configure TLS
	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	// Create HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Validate port presence
	if !strings.Contains(baseURL, ":") {
		return nil, fmt.Errorf("missing port in address %s", baseURL)
	}

	// Create proxmox client with correct parameters
	proxClient, err := proxmox.NewClient(
		baseURL,
		httpClient,
		"", // API token (empty for password auth)
		transport.TLSClientConfig,
		"",  // Logging prefix
		600, // Timeout
	)
	if err != nil {
		return nil, err
	}

	// Format credentials with realm
	if realm == "" {
		realm = "pam" // Default to pam authentication realm
	}

	// Construct proper proxmox username format
	authUser := user
	if !strings.Contains(authUser, "@") {
		authUser = fmt.Sprintf("%s@%s", user, realm)
	}

	fmt.Printf("Authentication parameters:\n- User: %s\n- Realm: %s\n- API: %s\n",
		authUser, realm, baseURL)

	// Perform authentication with formatted username (realm should be empty when using username@realm format)
	if err := proxClient.Login(context.Background(), authUser, password, ""); err != nil {
		fmt.Printf("DEBUG - Authentication parameters:\nUser: %s\nPassword: %s\nAPI: %s\n", authUser, password, baseURL)
		return nil, fmt.Errorf("authentication failed for %s at %s: %w\nCheck:\n1. Credentials format: username@realm\n2. Realm '%s' exists\n3. User has API permissions\n4. TLS certificate validity", authUser, baseURL, err, realm)
		// return nil, fmt.Errorf("authentication failed: %w\nTroubleshooting:\n1. Verify credentials\n2. Check network connectivity\n3. Validate TLS settings", err)
	}

	// Verify API connectivity
	if _, err := proxClient.GetVersion(context.Background()); err != nil {
		return nil, fmt.Errorf("API verification failed: %w", err)
	}

	return &Client{ProxClient: proxClient}, nil
}

// Node represents a Proxmox cluster node.
type Node struct {
	ID           string
	Name         string
	CPUUsage     float64 // Current CPU load percentage
	MemoryTotal  int64   // Total memory in bytes
	MemoryUsed   int64   // Used memory in bytes
	TotalStorage int64   // Total storage in bytes
	UsedStorage  int64   // Used storage in bytes
	Uptime       int64   // System uptime in seconds
	Version      string  // Proxmox version string
	IP           string  // Node IP address from cluster status
	Online       bool    // Node online status
}

// ListNodes retrieves all nodes from the cluster with caching
func (c *Client) ListNodes() ([]Node, error) {
	var res map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.ProxClient.GetJsonRetryable(ctx, "/nodes", &res, 3); err != nil {
		return nil, fmt.Errorf("ListNodes failed: %w", err)
	}

	data, ok := res["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format for node list")
	}

	nodes := make([]Node, len(data))
	for i, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid node data at index %d", i)
		}

		nodeName, ok := m["node"].(string)
		if !ok {
			return nil, fmt.Errorf("missing node name at index %d", i)
		}

		node := Node{
			ID:           nodeName,
			Name:         nodeName,
			Online:       strings.EqualFold(getString(m, "status"), "online"),
			CPUUsage:     getFloat(m, "cpu"),
			MemoryTotal:  int64(getFloat(m, "maxmem")),
			MemoryUsed:   int64(getFloat(m, "mem")),
			TotalStorage: int64(getFloat(m, "maxdisk")),
			UsedStorage:  int64(getFloat(m, "disk")),
			Uptime:       int64(getFloat(m, "uptime")),
			Version:      getString(m, "pveversion"),
		}

		// Fallback online check if status field missing
		if !node.Online && (node.CPUUsage > 0 || node.MemoryUsed > 0) {
			node.Online = true
		}

		nodes[i] = node
	}

	return nodes, nil
}

// VM represents a Proxmox VM or container.
type VM struct {
	ID     int
	Name   string
	Node   string
	Type   string
	IP     string // Primary IP address (if available)
	Status string // Current status (running, stopped, etc.)
}

// ListVMs retrieves all virtual machines on the given node.
func (c *Client) ListVMs(nodeName string) ([]VM, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw, err := c.ProxClient.GetVmList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM list: %w", err)
	}

	data, ok := raw["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM list")
	}

	var vms []VM
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if m["node"].(string) != nodeName {
			continue
		}

		vmid, ok := m["vmid"].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid VM ID in response: %v", m)
		}

		name, ok := m["name"].(string)
		if !ok {
			return nil, fmt.Errorf("VM %d missing name", int(vmid))
		}

		status, ok := m["status"].(string)
		if !ok {
			return nil, fmt.Errorf("VM %d (%s) missing status", int(vmid), name)
		}

		// Handle type with fallback to "qemu" if missing
		tp, _ := m["type"].(string)
		if tp == "" {
			tp = "qemu" // Default type if not specified
		}

		// Safely handle optional IP field
		var ip string
		if ipVal, ok := m["ip"]; ok {
			if ipStr, ok := ipVal.(string); ok {
				ip = ipStr
			}
		}

		vms = append(vms, VM{
			ID:     int(vmid),
			Name:   name,
			Node:   nodeName,
			Type:   tp,
			Status: status,
			IP:     ip,
		})
	}
	return vms, nil
}

// GetNodeStatus retrieves metrics for a given node from Proxmox API.
func (c *Client) GetNodeStatus(nodeName string) (map[string]interface{}, error) {
	var res map[string]interface{}
	if err := c.ProxClient.GetJsonRetryable(context.Background(), fmt.Sprintf("/nodes/%s/status", nodeName), &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for node status")
	}
	return data, nil
}

// GetNodeConfig retrieves configuration for a given node.
func (c *Client) GetNodeConfig(nodeName string) (map[string]interface{}, error) {
	var res map[string]interface{}
	if err := c.ProxClient.GetJsonRetryable(context.Background(), fmt.Sprintf("/nodes/%s/config", nodeName), &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for node config")
	}
	return data, nil
}

// GetVmStatus retrieves current status metrics for a VM or LXC.
func (c *Client) GetVmStatus(vm VM) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res map[string]interface{}
	// Use full=true to retrieve extended metrics (disk, network, maxdisk, etc.)
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current?full=1", vm.Node, vm.Type, vm.ID)
	if err := c.ProxClient.GetJsonRetryable(ctx, endpoint, &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM status")
	}
	return data, nil
}

// GetVmConfig retrieves configuration for a given VM or LXC.
func (c *Client) GetVmConfig(vm VM) (map[string]interface{}, error) {
	var res map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.Node, vm.Type, vm.ID)
	if err := c.ProxClient.GetJsonRetryable(context.Background(), endpoint, &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM config")
	}
	return data, nil
}

// getString safely extracts a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getFloat safely extracts a float value from a map
func getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		if num, ok := val.(float64); ok {
			return num
		}
	}
	return 0
}

// TODO: add methods: StartVM, StopVM, etc.
