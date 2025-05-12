package api

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Node represents a Proxmox cluster node
type Node struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	IP            string  `json:"ip"`
	CPUCount      float64 `json:"maxcpu"`
	CPUUsage      float64 `json:"cpu"`
	MemoryTotal   int64   `json:"maxmem"`
	MemoryUsed    int64   `json:"mem"`
	TotalStorage  int64   `json:"maxdisk"`
	UsedStorage   int64   `json:"disk"`
	Uptime        int64   `json:"uptime"`
	Version       string  `json:"pveversion"`
	KernelVersion string  `json:"kversion"`
	Online        bool    `json:"-"`
}

// ListNodes retrieves all nodes from the cluster with caching
func (c *Client) ListNodes() ([]Node, error) {
	// Get basic node metrics
	nodes, err := c.getBasicNodeMetrics()
	if err != nil {
		return nil, err
	}

	// Get cluster status for IP addresses
	ipMap, err := c.getNodeIPsFromClusterStatus()
	if err != nil {
		return nodes, fmt.Errorf("got basic node data but failed to get IPs: %w", err)
	}

	// Merge IP addresses into nodes
	for i := range nodes {
		if ip, exists := ipMap[nodes[i].Name]; exists {
			nodes[i].IP = ip
		}
	}

	return nodes, nil
}

func (c *Client) getBasicNodeMetrics() ([]Node, error) {
	var res map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.ProxClient.GetJsonRetryable(ctx, "/nodes", &res, 3); err != nil {
		return nil, fmt.Errorf("getBasicNodeMetrics failed: %w", err)
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

		nodes[i] = Node{
			ID:            nodeName,
			Name:          nodeName,
			Online:        strings.EqualFold(getString(m, "status"), "online"),
			CPUCount:      getFloat(m, "maxcpu"),
			CPUUsage:      getFloat(m, "cpu"),
			KernelVersion: getString(m, "kversion"),
			MemoryTotal:   int64(getFloat(m, "maxmem")),
			MemoryUsed:    int64(getFloat(m, "mem")),
			TotalStorage:  int64(getFloat(m, "maxdisk")),
			UsedStorage:   int64(getFloat(m, "disk")),
			Uptime:        int64(getFloat(m, "uptime")),
		}

		// Version will be populated by GetNodeStatus when needed
		nodes[i].Version = ""

		if !nodes[i].Online && (nodes[i].CPUUsage > 0 || nodes[i].MemoryUsed > 0) {
			nodes[i].Online = true
		}
	}

	return nodes, nil
}

// GetNodeStatus retrieves real-time status for a specific node
func (c *Client) GetNodeStatus(nodeName string) (*Node, error) {
	var res map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statusPath := fmt.Sprintf("/nodes/%s/status", nodeName)
	if err := c.ProxClient.GetJsonRetryable(ctx, statusPath, &res, 3); err != nil {
		return nil, fmt.Errorf("failed to get status for node %s: %w", nodeName, err)
	}

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid status response format for node %s", nodeName)
	}

	node := &Node{
		Name:          nodeName,
		Online:        strings.EqualFold(getString(data, "status"), "online"),
		CPUCount:      getFloat(data, "maxcpu"),
		CPUUsage:      getFloat(data, "cpu"),
		KernelVersion: getString(data, "current-kernel"),
		MemoryTotal:   int64(getFloat(data, "maxmem")),
		MemoryUsed:    int64(getFloat(data, "mem")),
		TotalStorage:  int64(getFloat(data, "maxdisk")),
		UsedStorage:   int64(getFloat(data, "disk")),
		Uptime:        int64(getFloat(data, "uptime")),
		Version:       getString(data, "pveversion"),
	}

	// Fallback to version endpoint if pveversion not in status
	if node.Version == "" {
		var versionRes map[string]interface{}
		versionCtx, versionCancel := context.WithTimeout(ctx, 2*time.Second)
		defer versionCancel()

		if err := c.ProxClient.GetJsonRetryable(versionCtx, fmt.Sprintf("/nodes/%s/version", nodeName), &versionRes, 1); err == nil {
			if versionData, ok := versionRes["data"].(map[string]interface{}); ok {
				node.Version = getString(versionData, "version")
			}
		}
	}

	return node, nil
}

func (c *Client) getNodeIPsFromClusterStatus() (map[string]string, error) {
	var res map[string]interface{}
	if err := c.ProxClient.GetJsonRetryable(context.Background(), "/cluster/status", &res, 3); err != nil {
		return nil, err
	}

	ipMap := make(map[string]string)
	data, ok := res["data"].([]interface{})
	if !ok {
		return ipMap, nil
	}

	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if getString(m, "type") == "node" {
			ipMap[getString(m, "name")] = getString(m, "ip")
		}
	}
	return ipMap, nil
}

// GetNodeConfig retrieves configuration for a given node
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
