package api

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Cluster represents aggregated Proxmox cluster metrics
type Cluster struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	Quorate        bool            `json:"quorate"`
	TotalNodes     int             `json:"total_nodes"`
	OnlineNodes    int             `json:"online"`
	TotalCPU       float64         `json:"total_cpu"`
	CPUUsage       float64         `json:"cpu_usage"`
	MemoryTotal    float64         `json:"memory_total"`
	MemoryUsed     float64         `json:"memory_used"`
	StorageTotal   int64           `json:"storage_total"`
	StorageUsed    int64           `json:"storage_used"`
	Nodes          []*Node         `json:"nodes"`
	StorageManager *StorageManager `json:"-"` // Storage manager for handling deduplication

	// For metrics tracking
	lastUpdate time.Time
}

// ClusterTask represents a cluster task from the Proxmox API
type ClusterTask struct {
	ID        string `json:"id"`
	Node      string `json:"node"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	User      string `json:"user"`
	TokenID   string `json:"tokenid,omitempty"`
	UPID      string `json:"upid"`
	Saved     string `json:"saved"`
	StartTime int64  `json:"starttime"`
	EndTime   int64  `json:"endtime"`
}

// GetClusterStatus retrieves high-level cluster status and node list
func (c *Client) GetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{
		Nodes:          make([]*Node, 0),
		StorageManager: NewStorageManager(),
		lastUpdate:     time.Now(),
	}

	// 1. Get basic cluster status
	if err := c.getClusterBasicStatus(cluster); err != nil {
		return nil, err
	}

	// 2. Get cluster resources for VMs and storage
	if err := c.processClusterResources(cluster); err != nil {
		return nil, err
	}

	// 3. Enrich VMs with detailed status information
	if err := c.EnrichVMs(cluster); err != nil {
		// Log error but continue
		c.logger.Debug("[CLUSTER] Error enriching VM data: %v", err)
	}

	// 4. Calculate cluster-wide totals
	c.calculateClusterTotals(cluster)

	c.Cluster = cluster
	return cluster, nil
}

// FastGetClusterStatus retrieves only essential cluster status without VM enrichment
// for fast application startup. VM details will be loaded in the background.
// The onEnrichmentComplete callback is called when background VM enrichment finishes.
func (c *Client) FastGetClusterStatus(onEnrichmentComplete func()) (*Cluster, error) {
	cluster := &Cluster{
		Nodes:          make([]*Node, 0),
		StorageManager: NewStorageManager(),
		lastUpdate:     time.Now(),
	}

	// 1. Get basic cluster status and node list
	if err := c.getClusterBasicStatus(cluster); err != nil {
		return nil, err
	}

	// 2. Get cluster resources and populate all nodes, VMs, and storage
	if err := c.processClusterResources(cluster); err != nil {
		return nil, err
	}

	// 3. Selectively enrich nodes with missing details (Version, KernelVersion, CPUInfo, LoadAvg)
	if err := c.enrichMissingNodeDetails(cluster); err != nil {
		return nil, err
	}

	// 4. Calculate cluster-wide totals
	c.calculateClusterTotals(cluster)

	// 5. Store the cluster in the client
	c.Cluster = cluster

	// 6. Start background VM enrichment
	go func() {
		c.logger.Debug("[BACKGROUND] Starting VM enrichment for %d nodes", len(cluster.Nodes))

		// Count VMs that will be enriched
		var runningVMCount int
		for _, node := range cluster.Nodes {
			if node.Online && node.VMs != nil {
				for _, vm := range node.VMs {
					if vm.Status == VMStatusRunning {
						runningVMCount++
					}
				}
			}
		}
		c.logger.Debug("[BACKGROUND] Found %d running VMs to enrich", runningVMCount)

		// Reset guestAgentChecked for all VMs before enrichment
		for _, node := range cluster.Nodes {
			if node.Online && node.VMs != nil {
				for _, vm := range node.VMs {
					vm.guestAgentChecked = false
				}
			}
		}

		if err := c.EnrichVMs(cluster); err != nil {
			c.logger.Debug("[BACKGROUND] Error enriching VM data: %v", err)
		} else {
			c.logger.Debug("[BACKGROUND] Successfully enriched VM data for %d running VMs", runningVMCount)
		}

		// Wait a bit and try to enrich VMs that might not have had guest agent ready
		time.Sleep(3 * time.Second)
		c.logger.Debug("[BACKGROUND] Starting delayed enrichment retry for QEMU VMs with missing guest agent data")

		// Second pass: try to enrich QEMU VMs that still don't have guest agent data
		// LXC containers don't have guest agents, so we skip them
		// Only retry VMs that have guest agent enabled in their config
		var retryCount int
		for _, node := range cluster.Nodes {
			if !node.Online || node.VMs == nil {
				continue
			}
			for _, vm := range node.VMs {
				// Only retry QEMU VMs that are running, have guest agent enabled, and don't have guest agent data
				if vm.Status == VMStatusRunning && vm.Type == VMTypeQemu && vm.AgentEnabled && (!vm.AgentRunning || len(vm.NetInterfaces) == 0) {
					retryCount++
					c.logger.Debug("[BACKGROUND] Retrying enrichment for QEMU VM %s (%d) - agent running: %v, interfaces: %d",
						vm.Name, vm.ID, vm.AgentRunning, len(vm.NetInterfaces))

					// Try to enrich this specific VM again, but only if the last error was not 'guest agent is not running'
					err := c.GetVmStatus(vm)
					if err != nil && strings.Contains(err.Error(), "guest agent is not running") {
						c.logger.Debug("[BACKGROUND] Skipping further retries for VM %s: guest agent is not running", vm.Name)
						continue
					}
					if err != nil {
						c.logger.Debug("[BACKGROUND] Retry failed for VM %s: %v", vm.Name, err)
					}
				}
			}
		}

		c.logger.Debug("[BACKGROUND] Completed enrichment process. Initial: %d VMs, QEMU Retry: %d VMs", runningVMCount, retryCount)

		// Call the callback only once after both initial enrichment and retry are complete
		if onEnrichmentComplete != nil {
			c.logger.Debug("[BACKGROUND] Calling enrichment complete callback")
			onEnrichmentComplete()
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

// enrichMissingNodeDetails selectively enriches nodes with data not available in cluster resources
func (c *Client) enrichMissingNodeDetails(cluster *Cluster) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(cluster.Nodes))
	done := make(chan struct{})

	// Start a goroutine to collect errors
	var errors []error
	go func() {
		for err := range errChan {
			if err != nil {
				errors = append(errors, err)
			}
		}
		close(done)
	}()

	// Process nodes concurrently, but only for missing details
	for i := range cluster.Nodes {
		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()
			errChan <- c.enrichNodeMissingDetails(node)
		}(cluster.Nodes[i])
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)
	<-done // Wait for error collection to finish

	// Log individual node errors but don't fail unless ALL nodes are unreachable
	if len(errors) > 0 {
		c.logger.Debug("[CLUSTER] Node detail enrichment completed with %d errors out of %d nodes", len(errors), len(cluster.Nodes))
		for _, err := range errors {
			c.logger.Debug("[CLUSTER] Node detail error: %v", err)
		}

		// Only fail if ALL nodes failed to respond
		if len(errors) == len(cluster.Nodes) {
			return fmt.Errorf("all nodes unreachable for detail enrichment: %d errors", len(errors))
		}

		// If some nodes succeeded, continue with a warning
		c.logger.Debug("[CLUSTER] Continuing with %d nodes having complete details (%d missing details)",
			len(cluster.Nodes)-len(errors), len(errors))
	}

	return nil
}

// enrichNodeMissingDetails enriches a single node with details not available in cluster resources
func (c *Client) enrichNodeMissingDetails(node *Node) error {
	// If the node is already marked as offline, skip detailed metrics
	if !node.Online {
		c.logger.Debug("[CLUSTER] Skipping detail enrichment for offline node: %s", node.Name)
		return nil
	}

	fullStatus, err := c.GetNodeStatus(node.Name)
	if err != nil {
		// Mark node as offline if we can't reach it
		node.Online = false
		c.logger.Debug("[CLUSTER] Node %s appears to be offline or unreachable for detail enrichment: %v", node.Name, err)

		// Return error for logging but don't make it critical
		return fmt.Errorf("node %s offline/unreachable for details: %w", node.Name, err)
	}

	// Only update fields not available in cluster resources
	node.Version = fullStatus.Version
	node.KernelVersion = fullStatus.KernelVersion
	node.CPUInfo = fullStatus.CPUInfo
	node.LoadAvg = fullStatus.LoadAvg
	node.lastMetricsUpdate = time.Now()

	c.logger.Debug("[CLUSTER] Successfully enriched missing details for node: %s", node.Name)
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

		switch resType {
		case "node":
			// Handle node resources - populate all available data from cluster resources
			if node, exists := nodeMap[nodeName]; exists {
				// Update all metrics available in cluster resources
				if cpuUsage := getFloat(resource, "cpu"); cpuUsage >= 0 {
					node.CPUUsage = cpuUsage
				}

				// Memory (convert bytes to GB)
				if memUsed := getFloat(resource, "mem"); memUsed > 0 {
					node.MemoryUsed = memUsed / 1073741824
				}
				if memMax := getFloat(resource, "maxmem"); memMax > 0 {
					node.MemoryTotal = memMax / 1073741824
				}

				// CPU count (available as maxcpu in cluster resources)
				if cpuCount := getFloat(resource, "maxcpu"); cpuCount > 0 {
					node.CPUCount = cpuCount
				}

				// Storage (convert bytes to GB)
				if diskUsed := getFloat(resource, "disk"); diskUsed > 0 {
					node.UsedStorage = int64(diskUsed / 1073741824)
				}
				if diskMax := getFloat(resource, "maxdisk"); diskMax > 0 {
					node.TotalStorage = int64(diskMax / 1073741824)
				}

				// Uptime
				if uptime := getFloat(resource, "uptime"); uptime > 0 {
					node.Uptime = int64(uptime)
				}

				c.logger.Debug("[CLUSTER] Populated node %s from cluster resources: CPU=%.2f%%, Mem=%.1fGB/%.1fGB, CPUs=%.0f",
					nodeName, node.CPUUsage*100, node.MemoryUsed, node.MemoryTotal, node.CPUCount)
			}
		case "storage":
			node, exists := nodeMap[nodeName]
			if !exists {
				continue
			}
			storage := &Storage{
				ID:         getString(resource, "id"),
				Name:       getString(resource, "storage"),
				Content:    getString(resource, "content"),
				Disk:       int64(getFloat(resource, "disk")),
				MaxDisk:    int64(getFloat(resource, "maxdisk")),
				Node:       nodeName,
				Plugintype: getString(resource, "plugintype"),
				Status:     getString(resource, "status"),
				Shared:     getInt(resource, "shared"),
				Type:       getString(resource, "type"),
			}

			// Initialize Storage slice if it doesn't exist
			if node.Storage == nil {
				node.Storage = make([]*Storage, 0)
			}

			// Append storage to the node's storage pools
			node.Storage = append(node.Storage, storage)

			// Add to storage manager for proper deduplication
			cluster.StorageManager.AddStorage(storage)
		case VMTypeQemu, VMTypeLXC:
			node, exists := nodeMap[nodeName]
			if !exists {
				continue
			}
			node.VMs = append(node.VMs, &VM{
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

	// Calculate storage totals using StorageManager (handles deduplication)
	cluster.StorageUsed = cluster.StorageManager.GetTotalUsage()
	cluster.StorageTotal = cluster.StorageManager.GetTotalCapacity()

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

// GetClusterTasks retrieves recent cluster tasks
func (c *Client) GetClusterTasks() ([]*ClusterTask, error) {
	var result map[string]interface{}
	if err := c.Get("/cluster/tasks", &result); err != nil {
		return nil, fmt.Errorf("failed to get cluster tasks: %w", err)
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for cluster tasks data")
	}

	var tasks []*ClusterTask
	for _, item := range data {
		taskData, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		task := &ClusterTask{
			ID:      SafeStringValue(taskData["id"]),
			Node:    SafeStringValue(taskData["node"]),
			Type:    SafeStringValue(taskData["type"]),
			Status:  SafeStringValue(taskData["status"]),
			User:    SafeStringValue(taskData["user"]),
			TokenID: SafeStringValue(taskData["tokenid"]),
			UPID:    SafeStringValue(taskData["upid"]),
			Saved:   SafeStringValue(taskData["saved"]),
		}

		// Parse timestamps
		if startTime, ok := taskData["starttime"].(float64); ok {
			task.StartTime = int64(startTime)
		}
		if endTime, ok := taskData["endtime"].(float64); ok {
			task.EndTime = int64(endTime)
		}

		tasks = append(tasks, task)
	}

	// Sort by start time (newest first)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartTime > tasks[j].StartTime
	})

	return tasks, nil
}
