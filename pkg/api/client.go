package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/Telmate/proxmox-api-go/proxmox"
)

// Client is a Proxmox API client
type Client struct {
	client *proxmox.Client
}

// NewClient initializes a new Proxmox API client with optimized defaults
func NewClient(addr, user, password string, insecure bool) (*Client, error) {
	// Reuse default transport with modifications
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure}
	transport.MaxIdleConns = 100
	transport.MaxConnsPerHost = 50
	transport.MaxIdleConnsPerHost = 20

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second, // Optimal timeout based on testing
	}

	// Create proxmox client with extended timeout
	proxClient, err := proxmox.NewClient(addr, httpClient, "", transport.TLSClientConfig, "", 600)
	if err != nil {
		return nil, err
	}
	if err := proxClient.Login(context.Background(), user, password, ""); err != nil {
		return nil, err
	}
	return &Client{client: proxClient}, nil
}

// Node represents a Proxmox cluster node.
type Node struct {
	ID   string
	Name string
}

// ListNodes retrieves all nodes from the cluster with caching
func (c *Client) ListNodes() ([]Node, error) {
	var res map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.client.GetJsonRetryable(ctx, "/nodes", &res, 3); err != nil {
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

		nodes[i] = Node{ID: nodeName, Name: nodeName}
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

	raw, err := c.client.GetVmList(ctx)
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
	if err := c.client.GetJsonRetryable(context.Background(), fmt.Sprintf("/nodes/%s/status", nodeName), &res, 3); err != nil {
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
	if err := c.client.GetJsonRetryable(context.Background(), fmt.Sprintf("/nodes/%s/config", nodeName), &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for node config")
	}
	return data, nil
}

// GetClusterStatus retrieves and parses cluster status efficiently
func (c *Client) GetClusterStatus() (map[string]map[string]interface{}, error) {
	var res map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.client.GetJsonRetryable(ctx, "/cluster/status", &res, 3); err != nil {
		return nil, fmt.Errorf("GetClusterStatus failed: %w", err)
	}

	data, ok := res["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format for cluster status")
	}

	items := make(map[string]map[string]interface{}, len(data))
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if name, ok := m["name"].(string); ok {
			items[name] = m
		}
	}
	return items, nil
}

// GetVmStatus retrieves current status metrics for a VM or LXC.
func (c *Client) GetVmStatus(vm VM) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res map[string]interface{}
	// Use full=true to retrieve extended metrics (disk, network, maxdisk, etc.)
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current?full=1", vm.Node, vm.Type, vm.ID)
	if err := c.client.GetJsonRetryable(ctx, endpoint, &res, 3); err != nil {
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
	if err := c.client.GetJsonRetryable(context.Background(), endpoint, &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM config")
	}
	return data, nil
}

// TODO: add methods: StartVM, StopVM, etc.
