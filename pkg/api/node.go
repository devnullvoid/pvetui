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

// ListNodes retrieves all nodes from the cluster using already cached data
func (c *Client) ListNodes() ([]Node, error) {
	cluster, err := c.GetClusterStatus()
	if err != nil {
		return nil, err
	}

	nodes := make([]Node, len(cluster.Nodes))
	for i, clusterNode := range cluster.Nodes {
		nodes[i] = *clusterNode
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
		KernelVersion: getString(data, "kversion"),
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
