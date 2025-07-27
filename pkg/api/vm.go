package api

import (
	"fmt"
	"strings"
)

// GetVmStatus retrieves current status metrics for a VM or LXC.
func (c *Client) GetVmStatus(vm *VM) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Store current disk values to preserve them if not updated from API
	currentDisk := vm.Disk
	currentMaxDisk := vm.MaxDisk

	var res map[string]interface{}

	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current", vm.Node, vm.Type, vm.ID)
	if err := c.GetWithCache(endpoint, &res, VMDataTTL); err != nil {
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

	// Get disk usage - only update if the API returns non-zero values
	diskFound := false

	if diskVal, ok := data["disk"]; ok {
		if diskFloat, ok := diskVal.(float64); ok && diskFloat > 0 {
			vm.Disk = int64(diskFloat)
			diskFound = true
		}
	}

	maxDiskFound := false

	if maxDiskVal, ok := data["maxdisk"]; ok {
		if maxDiskFloat, ok := maxDiskVal.(float64); ok && maxDiskFloat > 0 {
			vm.MaxDisk = int64(maxDiskFloat)
			maxDiskFound = true
		}
	}

	// Restore previous values if not found in API or if they were zero
	if !diskFound && currentDisk > 0 {
		vm.Disk = currentDisk
	}

	if !maxDiskFound && currentMaxDisk > 0 {
		vm.MaxDisk = currentMaxDisk
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

	// For QEMU VMs, check guest agent and get network interfaces
	if vm.Type == VMTypeQemu && vm.Status == VMStatusRunning {
		// Get VM config to identify configured MAC addresses
		var configRes map[string]interface{}

		configEndpoint := fmt.Sprintf("/nodes/%s/qemu/%d/config", vm.Node, vm.ID)
		if err := c.GetWithCache(configEndpoint, &configRes, VMDataTTL); err == nil {
			if configData, ok := configRes["data"].(map[string]interface{}); ok {
				populateConfiguredMACs(vm, configData)
				populateConfigDetails(vm, configData)
				// Populate AgentEnabled from config
				if agentVal, ok := configData["agent"]; ok {
					switch v := agentVal.(type) {
					case bool:
						vm.AgentEnabled = v
					case int:
						vm.AgentEnabled = v != 0
					case string:
						vm.AgentEnabled = v == "1" || v == StringTrue
					}
				}
			}
		}

		// Get network interfaces from guest agent (only if agent is enabled)
		if vm.AgentEnabled {
			if !vm.guestAgentChecked {
				vm.guestAgentChecked = true
				rawNetInterfaces, err := c.GetGuestAgentInterfaces(vm)

				if err == nil && len(rawNetInterfaces) > 0 {
					vm.AgentRunning = true

					var filteredInterfaces []NetworkInterface

					for _, iface := range rawNetInterfaces {
						// Skip loopback and veth interfaces, and check against configured MACs
						if !iface.IsLoopback && !strings.HasPrefix(iface.Name, "veth") && (vm.ConfiguredMACs == nil || vm.ConfiguredMACs[strings.ToUpper(iface.MACAddress)]) {
							iface.IPAddresses = prioritizeIPAddresses(iface.IPAddresses)
							filteredInterfaces = append(filteredInterfaces, iface)
						}
					}

					vm.NetInterfaces = filteredInterfaces

					// Update IP address if we don't have one yet and have interfaces
					if vm.IP == "" && len(vm.NetInterfaces) > 0 {
						vm.IP = GetFirstNonLoopbackIP(vm.NetInterfaces, true)
					}

					// If guest agent is running, also get filesystem information
					filesystems, fsErr := c.GetGuestAgentFilesystems(vm)
					if fsErr == nil && len(filesystems) > 0 {
						// Filter filesystems to only include actual hardware disks
						var filteredFilesystems []Filesystem

						for _, fs := range filesystems {
							// Skip filesystems we don't care about
							if strings.HasPrefix(fs.Mountpoint, "/snap") ||
								strings.HasPrefix(fs.Mountpoint, "/run") ||
								strings.HasPrefix(fs.Mountpoint, "/sys") ||
								strings.HasPrefix(fs.Mountpoint, "/proc") ||
								strings.HasPrefix(fs.Mountpoint, "/dev") ||
								strings.Contains(fs.Mountpoint, "snap/") {
								continue
							}

							// Skip Windows container paths and special Windows paths
							if strings.Contains(fs.Mountpoint, "\\Containers\\") ||
								strings.Contains(fs.Mountpoint, "/Containers/") ||
								strings.Contains(fs.Mountpoint, "\\WindowsApps\\") ||
								strings.Contains(fs.Mountpoint, "\\WpSystem\\") ||
								strings.Contains(fs.Mountpoint, "\\Config.Msi") {
								continue
							}

							// Skip long GUID paths that are typically system or virtual mounts
							if strings.Contains(fs.Mountpoint, "{") && strings.Contains(fs.Mountpoint, "}") &&
								len(fs.Mountpoint) > 50 {
								continue
							}

							// Skip if no size information
							if fs.TotalBytes == 0 {
								continue
							}

							// Skip small partitions (less than 50MB) that likely aren't real disks
							if fs.TotalBytes < 50*1024*1024 {
								continue
							}

							// Skip filesystem types that don't represent real disk space
							if fs.Type == "tmpfs" || fs.Type == "devtmpfs" || fs.Type == "proc" ||
								fs.Type == "sysfs" || fs.Type == "devpts" || fs.Type == "cgroup" ||
								fs.Type == "configfs" || fs.Type == "debugfs" || fs.Type == "mqueue" ||
								fs.Type == "hugetlbfs" || fs.Type == "securityfs" || fs.Type == "pstore" ||
								fs.Type == "autofs" || fs.Type == "UDF" {
								continue
							}

							filteredFilesystems = append(filteredFilesystems, fs)
						}

						vm.Filesystems = filteredFilesystems

						// Update disk usage from filesystem information if we have good data
						// This is more accurate than the API's disk usage values
						var totalDiskSpace int64

						var usedDiskSpace int64

						for _, fs := range filteredFilesystems {
							totalDiskSpace += fs.TotalBytes
							usedDiskSpace += fs.UsedBytes
						}

						// Only update if we got meaningful values
						if totalDiskSpace > 0 {
							vm.MaxDisk = totalDiskSpace
							vm.Disk = usedDiskSpace
						}
					}
				} else {
					vm.AgentRunning = false
					vm.NetInterfaces = nil
					// Only clear IP if it wasn't already set by config
					// This check is to preserve IP from config if guest agent fails
					if len(vm.ConfiguredMACs) == 0 {
						vm.IP = ""
					}
				}
			}
		} else {
			// Guest agent is disabled, set appropriate defaults
			vm.AgentRunning = false
			vm.NetInterfaces = nil
			// Don't clear IP if it was set from config
		}
	} else if vm.Type == VMTypeLXC && vm.Status == VMStatusRunning {
		// Get LXC config to identify configured MAC addresses (if any, often not explicitly set for LXC ethX)
		var configRes map[string]interface{}

		configEndpoint := fmt.Sprintf("/nodes/%s/lxc/%d/config", vm.Node, vm.ID)
		if err := c.GetWithCache(configEndpoint, &configRes, VMDataTTL); err == nil {
			if configData, ok := configRes["data"].(map[string]interface{}); ok {
				populateConfiguredMACs(vm, configData)
				populateConfigDetails(vm, configData)
			}
		}

		rawNetInterfaces, lxcErr := c.GetLxcInterfaces(vm) // Error from GetLxcInterfaces is already handled (returns nil if major issue)
		if lxcErr != nil {
			c.logger.Debug("[vm.go] Error calling GetLxcInterfaces for %s (%d): %v", vm.Name, vm.ID, lxcErr)
		}

		if len(rawNetInterfaces) > 0 {
			var filteredLxcInterfaces []NetworkInterface

			for _, iface := range rawNetInterfaces {
				// Skip loopback interfaces. For LXC, we might not always have MACs in config,
				// so if ConfiguredMACs is empty, we show all non-loopback by default.
				// If ConfiguredMACs is populated, then we filter by it.
				showInterface := !iface.IsLoopback
				if len(vm.ConfiguredMACs) > 0 { // Only filter by MAC if we have configured MACs
					showInterface = showInterface && vm.ConfiguredMACs[strings.ToUpper(iface.MACAddress)]
				}

				if showInterface {
					iface.IPAddresses = prioritizeIPAddresses(iface.IPAddresses)
					filteredLxcInterfaces = append(filteredLxcInterfaces, iface)
				}
			}

			vm.NetInterfaces = filteredLxcInterfaces
			if vm.IP == "" && len(vm.NetInterfaces) > 0 {
				vm.IP = GetFirstNonLoopbackIP(vm.NetInterfaces, true)
			}
		} else {
			vm.NetInterfaces = nil // No interfaces found or error in GetLxcInterfaces
			// Preserve IP if it was somehow set from LXC config (less common but possible)
			if len(vm.ConfiguredMACs) == 0 {
				vm.IP = ""
			}
		}
	}

	vm.Enriched = true

	return nil
}

// GetDetailedVmInfo retrieves complete information about a VM by combining status and config data (cached).
func (c *Client) GetDetailedVmInfo(node, vmType string, vmid int) (*VM, error) {
	vm := &VM{
		ID:   vmid,
		Node: node,
		Type: vmType,
	}

	// Get status information (cached)
	var statusRes map[string]interface{}

	statusEndpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current", node, vmType, vmid)
	if err := c.GetWithCache(statusEndpoint, &statusRes, VMDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get VM status: %w", err)
	}

	statusDataRaw := statusRes["data"]

	statusData, okStatusData := statusDataRaw.(map[string]interface{})
	if !okStatusData {
		return nil, fmt.Errorf("invalid VM status response format")
	}

	// Inline status parsing logic (as in GetVmStatus)
	if name, okName := statusData["name"].(string); okName {
		vm.Name = name
	}

	if status, okStatus := statusData["status"].(string); okStatus {
		vm.Status = status
	}

	if cpu, okCPU := statusData["cpu"].(float64); okCPU {
		vm.CPU = cpu
	}

	if mem, okMem := statusData["mem"].(float64); okMem {
		vm.Mem = int64(mem)
	}

	if maxmem, okMaxMem := statusData["maxmem"].(float64); okMaxMem {
		vm.MaxMem = int64(maxmem)
	}

	if disk, okDisk := statusData["disk"].(float64); okDisk {
		vm.Disk = int64(disk)
	}

	if maxdisk, okMaxDisk := statusData["maxdisk"].(float64); okMaxDisk {
		vm.MaxDisk = int64(maxdisk)
	}

	if uptime, okUptime := statusData["uptime"].(float64); okUptime {
		vm.Uptime = int64(uptime)
	}

	if diskread, okDiskRead := statusData["diskread"].(float64); okDiskRead {
		vm.DiskRead = int64(diskread)
	}

	if diskwrite, okDiskWrite := statusData["diskwrite"].(float64); okDiskWrite {
		vm.DiskWrite = int64(diskwrite)
	}

	if netin, okNetIn := statusData["netin"].(float64); okNetIn {
		vm.NetIn = int64(netin)
	}

	if netout, okNetOut := statusData["netout"].(float64); okNetOut {
		vm.NetOut = int64(netout)
	}

	if hastate, okHAState := statusData["hastate"].(string); okHAState {
		vm.HAState = hastate
	}

	if lock, okLock := statusData["lock"].(string); okLock {
		vm.Lock = lock
	}

	if tags, okTags := statusData["tags"].(string); okTags {
		vm.Tags = tags
	}

	if template, okTemplate := statusData["template"].(bool); okTemplate {
		vm.Template = template
	}

	if pool, okPool := statusData["pool"].(string); okPool {
		vm.Pool = pool
	}

	// Get config information (cached)
	var configRes map[string]interface{}

	configEndpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", node, vmType, vmid)
	if err := c.GetWithCache(configEndpoint, &configRes, VMDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get VM config: %w", err)
	}

	configData, ok := configRes["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid VM config response format")
	}

	populateConfigDetails(vm, configData)

	return vm, nil
}

// prioritizeIPAddresses selects the best IP address from a list, prioritizing IPv4 over IPv6.
func prioritizeIPAddresses(ipAddresses []IPAddress) []IPAddress {
	if len(ipAddresses) == 0 {
		return nil
	}

	// Prioritize IPv4, then first IPv6, then no IP for this interface if none match
	var bestIP IPAddress

	foundIP := false

	for _, ip := range ipAddresses {
		if ip.Type == IPTypeIPv4 {
			bestIP = ip
			foundIP = true

			break
		}
	}

	if !foundIP && len(ipAddresses) > 0 {
		// If no IPv4, take the first IPv6 (or any first IP if types are mixed unexpectedly)
		for _, ip := range ipAddresses {
			if ip.Type == IPTypeIPv6 { // Explicitly look for IPv6 first
				bestIP = ip
				foundIP = true

				break
			}
		}

		if !foundIP { // Fallback to literally the first IP if no IPv6 was marked
			bestIP = ipAddresses[0]
			foundIP = true
		}
	}

	if foundIP {
		return []IPAddress{bestIP}
	}

	return nil // No suitable IP found
}
