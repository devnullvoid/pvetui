package api

import "fmt"

// Cluster represents aggregated Proxmox cluster metrics
type Cluster struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Quorate     bool    `json:"quorate"`
	TotalNodes  int     `json:"nodes"`
	OnlineNodes int     `json:"online"`
	TotalCPU    float64 `json:"total_cpu"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryTotal int64   `json:"memory_total"`
	MemoryUsed  int64   `json:"memory_used"`
	Nodes       []*Node `json:"nodes"`

	// Index for quick node lookups
	nodeIndex map[string]*Node
}

// GetClusterStatus retrieves cluster status from API
func (c *Client) GetClusterStatus() (*Cluster, error) {
	var resp map[string]interface{}
	err := c.Get("/cluster/status", &resp)
	if err != nil {
		return nil, err
	}

	// Debug: Print raw API response
	fmt.Printf("Raw cluster status response:\n%+v\n", resp)

	cluster := &Cluster{Nodes: []*Node{}}

	// Process API response
	data, ok := resp["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster status response format")
	}

	for _, item := range data {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue // Skip invalid entries
		}

		itemType := getString(itemMap, "type")
		if itemType == "cluster" {
			cluster.Name = getString(itemMap, "name")
			// Version comes from node data, initialize as unknown
			cluster.Version = "Unknown"
			cluster.Quorate = getBool(itemMap, "quorate")
			cluster.TotalNodes = getInt(itemMap, "nodes")
		} else if itemType == "node" {
			nodeName := getString(itemMap, "name")
			nodeIP := getString(itemMap, "ip")
			// Get full node details
			// Create node with core identity fields
			node := &Node{
				ID:   nodeName,
				Name: nodeName,
				IP:   nodeIP,
			}

			// Get status and version details
			if status, err := c.GetNodeStatus(nodeName); err == nil {
				node.Online = status.Online
				node.CPUCount = status.CPUCount
				node.CPUUsage = status.CPUUsage
				node.MemoryTotal = status.MemoryTotal
				node.MemoryUsed = status.MemoryUsed
				node.TotalStorage = status.TotalStorage
				node.UsedStorage = status.UsedStorage
				node.Uptime = status.Uptime
				node.Version = status.Version
				node.KernelVersion = status.KernelVersion
			} else {
				// Fallback to cluster status data if detailed status fails
				node.Online = getInt(itemMap, "online") == 1
				node.Version = getString(itemMap, "pveversion")
			}

			cluster.Nodes = append(cluster.Nodes, node)
			if node.Online {
				cluster.OnlineNodes++
			}
			cluster.TotalCPU += node.CPUCount
			cluster.CPUUsage += node.CPUUsage
			// API returns memory in bytes
			cluster.MemoryTotal += node.MemoryTotal
			cluster.MemoryUsed += node.MemoryUsed
		}
	}

	// Set cluster version from first node's version if available
	if len(cluster.Nodes) > 0 && cluster.Nodes[0] != nil {
		cluster.Version = cluster.Nodes[0].Version
	}

	return cluster, nil
}
