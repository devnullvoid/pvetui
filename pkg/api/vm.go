package api

import (
	"fmt"
	"strings"
	"sync"
	
	// "github.com/devnullvoid/proxmox-tui/pkg/config"
)

// VM represents a Proxmox VM or container
type VM struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Node      string  `json:"node"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	IP        string  `json:"ip,omitempty"`
	CPU       float64 `json:"cpu,omitempty"`
	Mem       int64   `json:"mem,omitempty"`
	MaxMem    int64   `json:"maxmem,omitempty"`
	Disk      int64   `json:"disk,omitempty"`
	MaxDisk   int64   `json:"maxdisk,omitempty"`
	Uptime    int64   `json:"uptime,omitempty"`
	DiskRead  int64   `json:"diskread,omitempty"`
	DiskWrite int64   `json:"diskwrite,omitempty"`
	NetIn     int64   `json:"netin,omitempty"`
	NetOut    int64   `json:"netout,omitempty"`
	HAState   string  `json:"hastate,omitempty"`
	Lock      string  `json:"lock,omitempty"`
	Tags      string  `json:"tags,omitempty"`
	Template  bool    `json:"template,omitempty"`
	Pool      string  `json:"pool,omitempty"`
	
	// For metrics tracking
	mu         sync.RWMutex
	Enriched   bool   `json:"-"`
}

// GetVmStatus retrieves current status metrics for a VM or LXC
func (c *Client) GetVmStatus(vm *VM) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	var res map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current", vm.Node, vm.Type, vm.ID)
	if err := c.GetWithCache(endpoint, &res); err != nil {
		return err
	}

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected format for VM status")
	}

	// Enrich VM with additional metrics
	if cpuVal, ok := data["cpu"]; ok {
		if cpuFloat, ok := cpuVal.(float64); ok {
			vm.CPU = cpuFloat
		}
	}

	if memVal, ok := data["mem"]; ok {
		if memFloat, ok := memVal.(float64); ok {
			vm.Mem = int64(memFloat)
		}
	}

	if maxMemVal, ok := data["maxmem"]; ok {
		if maxMemFloat, ok := maxMemVal.(float64); ok {
			vm.MaxMem = int64(maxMemFloat)
		}
	}

	if diskReadVal, ok := data["diskread"]; ok {
		if diskReadFloat, ok := diskReadVal.(float64); ok {
			vm.DiskRead = int64(diskReadFloat)
		}
	}

	if diskWriteVal, ok := data["diskwrite"]; ok {
		if diskWriteFloat, ok := diskWriteVal.(float64); ok {
			vm.DiskWrite = int64(diskWriteFloat)
		}
	}

	if netInVal, ok := data["netin"]; ok {
		if netInFloat, ok := netInVal.(float64); ok {
			vm.NetIn = int64(netInFloat)
		}
	}

	if netOutVal, ok := data["netout"]; ok {
		if netOutFloat, ok := netOutVal.(float64); ok {
			vm.NetOut = int64(netOutFloat)
		}
	}

	if uptimeVal, ok := data["uptime"]; ok {
		if uptimeFloat, ok := uptimeVal.(float64); ok {
			vm.Uptime = int64(uptimeFloat)
		}
	}

	vm.Enriched = true
	return nil
}

// GetDetailedVmInfo retrieves complete information about a VM by combining status and config data
func (c *Client) GetDetailedVmInfo(node, vmType string, vmid int) (*VM, error) {
	vm := &VM{
		ID:   vmid,
		Node: node,
		Type: vmType,
	}
	
	// Get status information
	var statusRes map[string]interface{}
	statusEndpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current", node, vmType, vmid)
	if err := c.GetWithCache(statusEndpoint, &statusRes); err != nil {
		return nil, fmt.Errorf("failed to get VM status: %w", err)
	}
	
	statusData, ok := statusRes["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM status")
	}
	
	// Get config information
	var configRes map[string]interface{}
	configEndpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", node, vmType, vmid)
	if err := c.GetWithCache(configEndpoint, &configRes); err != nil {
		return nil, fmt.Errorf("failed to get VM config: %w", err)
	}
	
	configData, ok := configRes["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM config")
	}
	
	// Directly update the VM fields from the data maps
	if nameVal, ok := statusData["name"]; ok {
		if name, ok := nameVal.(string); ok {
			vm.Name = name
		}
	}
	
	if statusVal, ok := statusData["status"]; ok {
		if status, ok := statusVal.(string); ok {
			vm.Status = status
		}
	}
	
	if cpuVal, ok := statusData["cpu"]; ok {
		if cpu, ok := cpuVal.(float64); ok {
			vm.CPU = cpu
		}
	}
	
	if memVal, ok := statusData["mem"]; ok {
		if mem, ok := memVal.(float64); ok {
			vm.Mem = int64(mem)
		}
	}
	
	if maxMemVal, ok := statusData["maxmem"]; ok {
		if maxMem, ok := maxMemVal.(float64); ok {
			vm.MaxMem = int64(maxMem)
		}
	}
	
	if diskVal, ok := statusData["disk"]; ok {
		if disk, ok := diskVal.(float64); ok {
			vm.Disk = int64(disk)
		}
	}
	
	if maxDiskVal, ok := statusData["maxdisk"]; ok {
		if maxDisk, ok := maxDiskVal.(float64); ok {
			vm.MaxDisk = int64(maxDisk)
		}
	}
	
	if uptimeVal, ok := statusData["uptime"]; ok {
		if uptime, ok := uptimeVal.(float64); ok {
			vm.Uptime = int64(uptime)
		}
	}
	
	if diskReadVal, ok := statusData["diskread"]; ok {
		if diskRead, ok := diskReadVal.(float64); ok {
			vm.DiskRead = int64(diskRead)
		}
	}
	
	if diskWriteVal, ok := statusData["diskwrite"]; ok {
		if diskWrite, ok := diskWriteVal.(float64); ok {
			vm.DiskWrite = int64(diskWrite)
		}
	}
	
	if netInVal, ok := statusData["netin"]; ok {
		if netIn, ok := netInVal.(float64); ok {
			vm.NetIn = int64(netIn)
		}
	}
	
	if netOutVal, ok := statusData["netout"]; ok {
		if netOut, ok := netOutVal.(float64); ok {
			vm.NetOut = int64(netOut)
		}
	}
	
	// Add additional information from config
	if templateVal, ok := configData["template"]; ok {
		switch v := templateVal.(type) {
		case bool:
			vm.Template = v
		case int:
			vm.Template = v != 0
		case string:
			vm.Template = v == "1" || v == "true"
		}
	}
	
	if tagsVal, ok := configData["tags"]; ok {
		if tags, ok := tagsVal.(string); ok {
			vm.Tags = tags
		}
	}
	
	// Look for IPs in config (net0, net1, etc.)
	for k, v := range configData {
		if len(k) >= 3 && k[:3] == "net" {
			netStr, ok := v.(string)
			if !ok {
				continue
			}
			
			parts := strings.Split(netStr, ",")
			for _, part := range parts {
				if len(part) >= 3 && part[:3] == "ip=" {
					ip := part[3:] // Skip "ip="
					// Remove subnet mask if present
					if idx := strings.Index(ip, "/"); idx > 0 {
						ip = ip[:idx]
					}
					vm.IP = ip
					break
				}
			}
			
			if vm.IP != "" {
				break // Found an IP, no need to check other interfaces
			}
		}
	}
	
	vm.Enriched = true
	return vm, nil
}

// EnrichVMs enriches all VMs in the cluster with detailed status information
func (c *Client) EnrichVMs(cluster *Cluster) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 100) // Buffer for potential errors
	
	// Count total VMs for error channel sizing
	totalVMs := 0
	for _, node := range cluster.Nodes {
		if node.Online && node.VMs != nil {
			totalVMs += len(node.VMs)
		}
	}
	
	if totalVMs == 0 {
		return nil // No VMs to enrich
	}
	
	// Start error collector
	var errors []error
	done := make(chan struct{})
	go func() {
		for err := range errChan {
			if err != nil {
				errors = append(errors, err)
			}
		}
		close(done)
	}()
	
	// Process each node's VMs concurrently
	for _, node := range cluster.Nodes {
		if !node.Online || node.VMs == nil {
			continue
		}
		
		for i := range node.VMs {
			if node.VMs[i].Status != "running" {
				continue // Only enrich running VMs to avoid API overhead
			}
			
			wg.Add(1)
			go func(vm *VM) {
				defer wg.Done()
				err := c.GetVmStatus(vm)
				errChan <- err
			}(node.VMs[i])
		}
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)
	<-done // Wait for error collection to finish
	
	if len(errors) > 0 {
		return fmt.Errorf("errors updating VM statuses: %v", errors)
	}
	
	return nil
}


