package api

import (
	"context"
	"fmt"
	"time"
)

// Cluster represents aggregated Proxmox cluster metrics
type Cluster struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Quorate     bool    `json:"quorate"`
	TotalNodes  int     `json:"total_nodes"`
	OnlineNodes int     `json:"online"`
	TotalCPU    float64 `json:"total_cpu"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryTotal int64   `json:"memory_total"`
	MemoryUsed  int64   `json:"memory_used"`
	Nodes       []*Node `json:"nodes"`
}

// GetClusterStatus retrieves cluster status and node metrics
func (c *Client) GetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{Nodes: []*Node{}}

	// 1. First get node metrics from /nodes endpoint
	var nodesResp map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.ProxClient.GetJsonRetryable(ctx, "/nodes", &nodesResp, 3); err != nil {
		return nil, fmt.Errorf("failed to get node metrics: %w", err)
	}

	// Create node metrics lookup map
	nodeMetrics := make(map[string]map[string]interface{})
	if nodesData, ok := nodesResp["data"].([]interface{}); ok {
		for _, item := range nodesData {
			if m, ok := item.(map[string]interface{}); ok {
				nodeName := getString(m, "node")
				nodeMetrics[nodeName] = m
			}
		}
	}

	// 2. Get cluster topology and online status
	var clusterResp map[string]interface{}
	if err := c.Get("/cluster/status", &clusterResp); err != nil {
		return nil, err
	}
	fmt.Printf("Raw cluster status response:\n%+v\n", clusterResp)

	data, ok := clusterResp["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster status response format")
	}

	for _, item := range data {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType := getString(itemMap, "type")
		if itemType == "cluster" {
			cluster.Name = getString(itemMap, "name")
			cluster.Quorate = getBool(itemMap, "quorate")
			cluster.TotalNodes = getInt(itemMap, "nodes")
			cluster.Version = "Unknown" // Will be set from nodes
		} else if itemType == "node" {
			nodeName := getString(itemMap, "name")
			node := &Node{
				ID:     nodeName,
				Name:   nodeName,
				IP:     getString(itemMap, "ip"),
				Online: getInt(itemMap, "online") == 1,
			}

			// Get metrics from nodes endpoint data
			if metrics, ok := nodeMetrics[nodeName]; ok {
				node.CPUCount = getFloat(metrics, "maxcpu")
				node.CPUUsage = getFloat(metrics, "cpu")
				node.MemoryTotal = int64(getFloat(metrics, "maxmem"))
				node.MemoryUsed = int64(getFloat(metrics, "mem"))
				node.TotalStorage = int64(getFloat(metrics, "maxdisk"))
				node.UsedStorage = int64(getFloat(metrics, "disk"))
				node.Uptime = int64(getFloat(metrics, "uptime"))
			}

			// Get version info if missing
			if node.Version == "" {
				if status, err := c.GetNodeStatus(nodeName); err == nil {
					node.Version = status.Version
					node.KernelVersion = status.KernelVersion
				}
			}

			cluster.Nodes = append(cluster.Nodes, node)
			if node.Online {
				cluster.OnlineNodes++
			}
			cluster.TotalCPU += node.CPUCount
			cluster.CPUUsage += node.CPUUsage
			cluster.MemoryTotal += node.MemoryTotal
			cluster.MemoryUsed += node.MemoryUsed
		}
	}

	// Set cluster version from first node
	if len(cluster.Nodes) > 0 {
		cluster.Version = cluster.Nodes[0].Version
	}

	return cluster, nil
}
