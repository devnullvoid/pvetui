package api

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Node represents a Proxmox cluster node
type Node struct {
	ID            string
	Name          string
	CPUCount      float64
	CPUUsage      float64
	MemoryTotal   int64
	MemoryUsed    int64
	TotalStorage  int64
	UsedStorage   int64
	Uptime        int64
	Version       string
	KernelVersion string
	IP            string
	Online        bool
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
			Version:       getString(m, "pveversion"),
		}

		if !node.Online && (node.CPUUsage > 0 || node.MemoryUsed > 0) {
			node.Online = true
		}

		nodes[i] = node
	}

	return nodes, nil
}

// GetNodeStatus retrieves metrics for a given node from Proxmox API
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
