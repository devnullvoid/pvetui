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

// GetClusterStatus retrieves cluster status and resources
func (c *Client) GetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{Nodes: []*Node{}}
	nodeMap := make(map[string]*Node)
	// Context now handled by Client.Get

	// 1. First get cluster status from /cluster/status
	var statusResp map[string]interface{}
	if err := c.Get("/cluster/status", &statusResp); err != nil {
		return nil, fmt.Errorf("failed to get cluster status: %w", err)
	}

	// Process cluster status data
	statusData, ok := statusResp["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster status response format")
	}

	for _, item := range statusData {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType := getString(itemMap, "type")
		if itemType == "cluster" {
			cluster.Name = getString(itemMap, "name")
			cluster.Quorate = getBool(itemMap, "quorate")
			cluster.TotalNodes = getInt(itemMap, "nodes")
		} else if itemType == "node" {
			nodeName := getString(itemMap, "name")
			node := &Node{
				ID:     nodeName,
				Name:   nodeName,
				IP:     getString(itemMap, "ip"),
				Online: getInt(itemMap, "online") == 1,
				// Version will be set from resources data
			}
			config.DebugLog("[CLUSTER] Initial node %s status - Online: %v (raw online: %v)",
				nodeName, node.Online, itemMap["online"])
			nodeMap[nodeName] = node
			cluster.Nodes = append(cluster.Nodes, node)
		}
	}

	// 2. Get node metrics from /nodes endpoint
	var nodesResp map[string]interface{}
	if err := c.Get("/nodes", &nodesResp); err != nil {
		return nil, fmt.Errorf("failed to get node metrics: %w", err)
	}

	// Process node metrics
	if nodesData, ok := nodesResp["data"].([]interface{}); ok {
		for _, item := range nodesData {
			resource, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			nodeName := getString(resource, "node")
			node, exists := nodeMap[nodeName]
			if exists {
				// Convert memory from bytes to GB
				node.MemoryTotal = getFloat(resource, "maxmem") / 1073741824
				node.MemoryUsed = getFloat(resource, "mem") / 1073741824
				node.CPUCount = getFloat(resource, "maxcpu")
				node.CPUUsage = getFloat(resource, "cpu")
				node.TotalStorage = int64(getFloat(resource, "maxdisk"))
				node.UsedStorage = int64(getFloat(resource, "disk"))
				node.Uptime = int64(getFloat(resource, "uptime"))

				// Refresh full node status and merge data
				if status, err := c.GetNodeStatus(nodeName); err == nil {
					// Preserve ALL cluster-managed fields before merge
					originalClusterData := &Node{
						Online:       node.Online,
						IP:           node.IP,
						VMs:          node.VMs,
						Storage:      node.Storage,
						CPUUsage:     node.CPUUsage,
						MemoryUsed:   node.MemoryUsed,
						MemoryTotal:  node.MemoryTotal,
						TotalStorage: node.TotalStorage,
						UsedStorage:  node.UsedStorage,
						Uptime:       node.Uptime,
						CPUCount:     node.CPUCount,
					}

					// Merge status data into node with debug logging
					config.DebugLog("[DEBUG] Merging node status data for %s: %+v", nodeName, status)
					*node = *status
					config.DebugLog("[DEBUG] Node after merge: %+v", node)

					// Restore ALL cluster-managed fields from preserved data
					node.Online = originalClusterData.Online
					node.IP = originalClusterData.IP
					node.VMs = originalClusterData.VMs
					node.Storage = originalClusterData.Storage
					node.CPUUsage = originalClusterData.CPUUsage
					node.MemoryUsed = originalClusterData.MemoryUsed
					node.MemoryTotal = originalClusterData.MemoryTotal
					node.TotalStorage = originalClusterData.TotalStorage
					node.UsedStorage = originalClusterData.UsedStorage
					node.Uptime = originalClusterData.Uptime
					node.CPUCount = originalClusterData.CPUCount

					// Keep resource metrics from /nodes endpoint
					node.MemoryTotal = getFloat(resource, "maxmem") / 1073741824
					node.MemoryUsed = getFloat(resource, "mem") / 1073741824
					node.CPUCount = getFloat(resource, "maxcpu")
					node.CPUUsage = getFloat(resource, "cpu")
					node.TotalStorage = int64(getFloat(resource, "maxdisk"))
					node.UsedStorage = int64(getFloat(resource, "disk"))
					node.Uptime = int64(getFloat(resource, "uptime"))
				}
			}
		}
	}

	// 3. Get cluster resources for VMs and storage
	var resourcesResp map[string]interface{}
	if err := c.Get("/cluster/resources", &resourcesResp); err != nil {
		return nil, fmt.Errorf("failed to get cluster resources: %w", err)
	}

	resourcesData, ok := resourcesResp["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster resources response format")
	}

	// Process storage and VMs from resources
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
		}
	}

	// Process VMs from resources data
	for _, item := range resourcesData {
		resource, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		resType := getString(resource, "type")
		nodeName := getString(resource, "node")
		if resType != "qemu" && resType != "lxc" {
			continue
		}

		node, exists := nodeMap[nodeName]
		if !exists {
			continue
		}

		vm := &VM{
			ID:        getInt(resource, "vmid"),
			Name:      getString(resource, "name"),
			Node:      nodeName,
			Type:      resType,
			Status:    getString(resource, "status"),
			IP:        getString(resource, "ip"),
			CPU:       getFloat(resource, "cpu"),
			Mem:       int64(getFloat(resource, "mem")),
			MaxMem:    int64(getFloat(resource, "maxmem")),
			Disk:      int64(getFloat(resource, "disk")),
			MaxDisk:   int64(getFloat(resource, "maxdisk")),
			Uptime:    int64(getFloat(resource, "uptime")),
			DiskRead:  int64(getFloat(resource, "diskread")),
			DiskWrite: int64(getFloat(resource, "diskwrite")),
			NetIn:     int64(getFloat(resource, "netin")),
			NetOut:    int64(getFloat(resource, "netout")),
			HAState:   getString(resource, "hastate"),
			Lock:      getString(resource, "lock"),
			Tags:      getString(resource, "tags"),
			Template:  getBool(resource, "template"),
			Pool:      getString(resource, "pool"),
		}

		node.VMs = append(node.VMs, vm)
		// Only track VMs here, node metrics handled in totals calculation
	}

	// Format cluster version and ensure correct values
	if len(cluster.Nodes) > 0 {
		if ver := cluster.Nodes[0].Version; ver != "" {
			cluster.Version = fmt.Sprintf("Proxmox VE %s", ver)
		}
	}

	// Calculate cluster-wide totals
	cluster.TotalNodes = len(cluster.Nodes)
	cluster.TotalCPU = 0
	cluster.CPUUsage = 0
	cluster.MemoryTotal = 0
	cluster.MemoryUsed = 0

	onlineNodes := 0
	for _, node := range cluster.Nodes {
		if node.Online {
			onlineNodes++
			cluster.TotalCPU += node.CPUCount
			cluster.CPUUsage += node.CPUUsage
			cluster.MemoryTotal += node.MemoryTotal
			cluster.MemoryUsed += node.MemoryUsed
		}
	}

	// Calculate average CPU usage
	if onlineNodes > 0 {
		cluster.CPUUsage = cluster.CPUUsage / float64(onlineNodes)
	}

	cluster.OnlineNodes = onlineNodes
	c.Cluster = cluster
	return cluster, nil
}
