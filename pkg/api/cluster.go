package api

import (
	"fmt"
	"sync"
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
	MemoryTotal float64 `json:"memory_total"`
	MemoryUsed  float64 `json:"memory_used"`
	Nodes       []*Node `json:"nodes"`

	// For metrics tracking
	lastUpdate time.Time
}

// GetClusterStatus retrieves high-level cluster status and node list
func (c *Client) GetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{
		Nodes:      make([]*Node, 0),
		lastUpdate: time.Now(),
	}

	// 1. Get basic cluster status
	if err := c.getClusterBasicStatus(cluster); err != nil {
		return nil, err
	}

	// 2. Enrich nodes with their full status data (concurrent)
	if err := c.enrichNodeStatuses(cluster); err != nil {
		return nil, err
	}

	// 3. Get cluster resources for VMs and storage
	if err := c.processClusterResources(cluster); err != nil {
		return nil, err
	}

	// 4. Enrich VMs with detailed status information
	if err := c.EnrichVMs(cluster); err != nil {
		// Log error but continue
		c.logger.Debug("[CLUSTER] Error enriching VM data: %v", err)
	}

	// 5. Calculate cluster-wide totals
	c.calculateClusterTotals(cluster)

	c.Cluster = cluster
	return cluster, nil
}

// FastGetClusterStatus retrieves only essential cluster status without VM enrichment
// for fast application startup. VM details will be loaded in the background.
func (c *Client) FastGetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{
		Nodes:      make([]*Node, 0),
		lastUpdate: time.Now(),
	}

	// 1. Get basic cluster status
	if err := c.getClusterBasicStatus(cluster); err != nil {
		return nil, err
	}

	// 2. Enrich nodes with their full status data (concurrent)
	if err := c.enrichNodeStatuses(cluster); err != nil {
		return nil, err
	}

	// 3. Get cluster resources for VMs and storage
	if err := c.processClusterResources(cluster); err != nil {
		return nil, err
	}

	// 4. Calculate cluster-wide totals
	c.calculateClusterTotals(cluster)

	// 5. Store the cluster in the client
	c.Cluster = cluster

	// 6. Start background VM enrichment
	go func() {
		if err := c.EnrichVMs(cluster); err != nil {
			c.logger.Debug("[BACKGROUND] Error enriching VM data: %v", err)
		} else {
			c.logger.Debug("[BACKGROUND] Successfully enriched VM data")
		}
	}()

	return cluster, nil
}

// getClusterBasicStatus retrieves basic cluster info and node list
func (c *Client) getClusterBasicStatus(cluster *Cluster) error {
	var statusResp map[string]interface{}
	if err := c.GetWithCache("/cluster/status", &statusResp, ClusterDataTTL); err != nil {
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

// enrichNodeStatuses populates detailed node data from individual node status calls concurrently
func (c *Client) enrichNodeStatuses(cluster *Cluster) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(cluster.Nodes))
	done := make(chan struct{})

	// Start a goroutine to collect errors
	var errors []error
	go func() {
		for err := range errChan {
			if err != nil {
				errors = append(errors, err)
				// Only consider it critical if ALL nodes fail
				// Individual node failures are expected in a cluster environment
			}
		}
		close(done)
	}()

	// Process nodes concurrently
	for i := range cluster.Nodes {
		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()
			errChan <- c.updateNodeMetrics(node)
		}(cluster.Nodes[i])
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)
	<-done // Wait for error collection to finish

	// Log individual node errors but don't fail unless ALL nodes are unreachable
	if len(errors) > 0 {
		c.logger.Debug("[CLUSTER] Node enrichment completed with %d errors out of %d nodes", len(errors), len(cluster.Nodes))
		for _, err := range errors {
			c.logger.Debug("[CLUSTER] Node error: %v", err)
		}

		// Only fail if ALL nodes failed to respond
		if len(errors) == len(cluster.Nodes) {
			return fmt.Errorf("all nodes unreachable: %d errors", len(errors))
		}

		// If some nodes succeeded, continue with a warning
		c.logger.Debug("[CLUSTER] Continuing with %d available nodes (%d offline)",
			len(cluster.Nodes)-len(errors), len(errors))
	}

	return nil
}

// updateNodeMetrics updates metrics for a single node
func (c *Client) updateNodeMetrics(node *Node) error {
	// node.mu.Lock()
	// defer node.mu.Unlock()

	// If the node is already marked as offline from cluster status, skip detailed metrics
	if !node.Online {
		c.logger.Debug("[CLUSTER] Skipping metrics for offline node: %s", node.Name)
		return nil
	}

	fullStatus, err := c.GetNodeStatus(node.Name)
	if err != nil {
		// Mark node as offline if we can't reach it
		node.Online = false
		c.logger.Debug("[CLUSTER] Node %s appears to be offline or unreachable: %v", node.Name, err)

		// Return error for logging but don't make it critical
		return fmt.Errorf("node %s offline/unreachable: %w", node.Name, err)
	}

	// Update node fields
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
	node.lastMetricsUpdate = time.Now()

	c.logger.Debug("[CLUSTER] Successfully updated metrics for node: %s", node.Name)
	return nil
}

// processClusterResources handles storage and VM data from cluster resources
func (c *Client) processClusterResources(cluster *Cluster) error {
	var resourcesResp map[string]interface{}
	if err := c.GetWithCache("/cluster/resources", &resourcesResp, ResourceDataTTL); err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}

	resourcesData, ok := resourcesResp["data"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid cluster resources response format")
	}

	// Create a map for quick node lookup
	nodeMap := make(map[string]*Node, len(cluster.Nodes))
	for i := range cluster.Nodes {
		nodeMap[cluster.Nodes[i].Name] = cluster.Nodes[i]
		// Initialize VMs slice if nil
		if cluster.Nodes[i].VMs == nil {
			cluster.Nodes[i].VMs = make([]*VM, 0)
		}
	}

	// Process resources in a single pass
	for _, item := range resourcesData {
		resource, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		resType := getString(resource, "type")
		nodeName := getString(resource, "node")
		node, exists := nodeMap[nodeName]
		if !exists {
			continue
		}

		switch resType {
		case "storage":
			node.Storage = &Storage{
				ID:         getString(resource, "id"),
				Content:    getString(resource, "content"),
				Disk:       int64(getFloat(resource, "disk")),
				MaxDisk:    int64(getFloat(resource, "maxdisk")),
				Node:       nodeName,
				Plugintype: getString(resource, "plugintype"),
				Status:     getString(resource, "status"),
			}
		case "qemu", "lxc":
			node.VMs = append(node.VMs, &VM{
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
			})
		}
	}
	return nil
}

// calculateClusterTotals aggregates node metrics for cluster summary
func (c *Client) calculateClusterTotals(cluster *Cluster) {
	var totalCPU, totalMem, usedMem float64
	var onlineNodes int
	var nodesWithMetrics int

	for _, node := range cluster.Nodes {
		if node.Online {
			onlineNodes++
			// Only include nodes that have valid metrics
			if node.CPUCount > 0 {
				totalCPU += node.CPUCount
				totalMem += node.MemoryTotal
				usedMem += node.MemoryUsed
				cluster.CPUUsage += node.CPUUsage
				nodesWithMetrics++
			}
		}
	}

	cluster.OnlineNodes = onlineNodes
	cluster.TotalCPU = totalCPU
	cluster.MemoryTotal = totalMem
	cluster.MemoryUsed = usedMem

	// Calculate average CPU usage only from nodes with valid metrics
	if nodesWithMetrics > 0 {
		cluster.CPUUsage /= float64(nodesWithMetrics)
	}

	// Set version from the first node that has version info
	for _, node := range cluster.Nodes {
		if node.Version != "" {
			cluster.Version = fmt.Sprintf("Proxmox VE %s", node.Version)
			break
		}
	}

	c.logger.Debug("[CLUSTER] Cluster totals calculated: %d/%d nodes online, %d with complete metrics",
		onlineNodes, len(cluster.Nodes), nodesWithMetrics)
}
