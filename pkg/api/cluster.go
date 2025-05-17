package api

import (
	"fmt"

	"github.com/lonepie/proxmox-tui/pkg/config"
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
	MemoryTotal float64 `json:"memory_total"`
	MemoryUsed  float64 `json:"memory_used"`
	Nodes       []*Node `json:"nodes"`
}

// GetClusterStatus retrieves high-level cluster status and node list
func (c *Client) GetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{Nodes: []*Node{}}

	// 1. Get basic cluster status
	if err := c.getClusterBasicStatus(cluster); err != nil {
		return nil, err
	}

	// 2. Enrich nodes with their full status data
	if err := c.enrichNodeStatuses(cluster); err != nil {
		return nil, err
	}

	// 3. Get cluster resources for VMs and storage
	if err := c.processClusterResources(cluster); err != nil {
		return nil, err
	}

	// 4. Calculate cluster-wide totals
	c.calculateClusterTotals(cluster)

	c.Cluster = cluster
	return cluster, nil
}

// getClusterBasicStatus retrieves basic cluster info and node list
func (c *Client) getClusterBasicStatus(cluster *Cluster) error {
	var statusResp map[string]interface{}
	if err := c.Get("/cluster/status", &statusResp); err != nil {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	statusData, ok := statusResp["data"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid cluster status response format")
	}

	for _, item := range statusData {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		switch getString(itemMap, "type") {
		case "cluster":
			cluster.Name = getString(itemMap, "name")
			cluster.Quorate = getBool(itemMap, "quorate")
			cluster.TotalNodes = getInt(itemMap, "nodes")
		case "node":
			nodeName := getString(itemMap, "name")
			cluster.Nodes = append(cluster.Nodes, &Node{
				ID:     nodeName,
				Name:   nodeName,
				IP:     getString(itemMap, "ip"),
				Online: getInt(itemMap, "online") == 1,
			})
		}
	}
	return nil
}

// enrichNodeStatuses populates detailed node data from individual node status calls
func (c *Client) enrichNodeStatuses(cluster *Cluster) error {
	for _, node := range cluster.Nodes {
		fullStatus, err := c.GetNodeStatus(node.Name)
		if err != nil {
			config.DebugLog("[CLUSTER] Error getting status for node %s: %v", node.Name, err)
			continue
		}

		// Merge only metrics from node status
		node.Version = fullStatus.Version
		node.KernelVersion = fullStatus.KernelVersion
		node.CPUCount = fullStatus.CPUCount
		node.CPUUsage = fullStatus.CPUUsage
		node.MemoryTotal = fullStatus.MemoryTotal
		node.MemoryUsed = fullStatus.MemoryUsed
		node.TotalStorage = fullStatus.TotalStorage
		node.UsedStorage = fullStatus.UsedStorage
		node.Uptime = fullStatus.Uptime
		node.CPUInfo = fullStatus.CPUInfo
		node.LoadAvg = fullStatus.LoadAvg
		// IP and Online come from cluster/status
		// Storage and VMs come from cluster/resources
	}
	return nil
}

// processClusterResources handles storage and VM data from cluster resources
func (c *Client) processClusterResources(cluster *Cluster) error {
	var resourcesResp map[string]interface{}
	if err := c.Get("/cluster/resources", &resourcesResp); err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}

	resourcesData, ok := resourcesResp["data"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid cluster resources response format")
	}

	nodeMap := make(map[string]*Node)
	for _, node := range cluster.Nodes {
		nodeMap[node.Name] = node
	}

	for _, item := range resourcesData {
		resource, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		resType := getString(resource, "type")
		nodeName := getString(resource, "node")

		switch resType {
		case "storage":
			if nodeName == "" {
				continue
			}
			if node, exists := nodeMap[nodeName]; exists {
				node.Storage = &Storage{
					ID:         getString(resource, "id"),
					Content:    getString(resource, "content"),
					Disk:       int64(getFloat(resource, "disk")),
					MaxDisk:    int64(getFloat(resource, "maxdisk")),
					Node:       nodeName,
					Plugintype: getString(resource, "plugintype"),
					Status:     getString(resource, "status"),
				}
			}
		case "qemu", "lxc":
			if node, exists := nodeMap[nodeName]; exists {
				vm := &VM{
					ID:       getInt(resource, "vmid"),
					Name:     getString(resource, "name"),
					Node:     nodeName,
					Type:     resType,
					Status:   getString(resource, "status"),
					IP:       getString(resource, "ip"),
					CPU:      getFloat(resource, "cpu"),
					Mem:      int64(getFloat(resource, "mem")),
					MaxMem:   int64(getFloat(resource, "maxmem")),
					Disk:     int64(getFloat(resource, "disk")),
					MaxDisk:  int64(getFloat(resource, "maxdisk")),
					Uptime:   int64(getFloat(resource, "uptime")),
					HAState:  getString(resource, "hastate"),
					Lock:     getString(resource, "lock"),
					Tags:     getString(resource, "tags"),
					Template: getBool(resource, "template"),
					Pool:     getString(resource, "pool"),
				}
				node.VMs = append(node.VMs, vm)
			}
		}
	}
	return nil
}

// calculateClusterTotals aggregates node metrics for cluster summary
func (c *Client) calculateClusterTotals(cluster *Cluster) {
	var totalCPU, totalMem, usedMem float64
	var onlineNodes int

	for _, node := range cluster.Nodes {
		if node.Online {
			onlineNodes++
			totalCPU += node.CPUCount
			totalMem += node.MemoryTotal
			usedMem += node.MemoryUsed
			cluster.CPUUsage += node.CPUUsage
		}
	}

	cluster.OnlineNodes = onlineNodes
	cluster.TotalCPU = totalCPU
	cluster.MemoryTotal = totalMem
	cluster.MemoryUsed = usedMem

	if onlineNodes > 0 {
		cluster.CPUUsage /= float64(onlineNodes)
	}

	if len(cluster.Nodes) > 0 {
		cluster.Version = fmt.Sprintf("Proxmox VE %s", cluster.Nodes[0].Version)
	}

}
