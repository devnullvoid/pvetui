package api

import (
	"fmt"
	"strings"
	"sync"
)

// EnrichVMs enriches all VMs in the cluster with detailed status information
func (c *Client) EnrichVMs(cluster *Cluster) error {
	const maxConcurrentRequests = 5 // Limit concurrent API requests

	var wg sync.WaitGroup
	errChan := make(chan error, 100) // Buffer for potential errors
	vmChan := make(chan *VM, 100)    // Channel for VM tasks

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

	// Start workers with limited concurrency
	for i := 0; i < maxConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vm := range vmChan {
				// Store the current disk usage values from /cluster/resources
				diskUsage := vm.Disk
				maxDiskUsage := vm.MaxDisk

				// Get regular VM status info including guest agent data
				err := c.GetVmStatus(vm)

				// Restore disk usage values from cluster resources if they got overwritten or are zero
				if vm.Disk == 0 && diskUsage > 0 {
					vm.Disk = diskUsage
				}

				if vm.MaxDisk == 0 && maxDiskUsage > 0 {
					vm.MaxDisk = maxDiskUsage
				}

				errChan <- err
			}
		}()
	}

	// Queue VMs for processing
	for _, node := range cluster.Nodes {
		if !node.Online || node.VMs == nil {
			continue
		}

		for i := range node.VMs {
			if node.VMs[i].Status != VMStatusRunning {
				continue // Only enrich running VMs to avoid API overhead
			}
			vmChan <- node.VMs[i]
		}
	}

	// Close VM channel to signal workers that all tasks are queued
	close(vmChan)

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)
	<-done // Wait for error collection to finish

	if len(errors) > 0 {
		return fmt.Errorf("errors updating VM statuses: %v", errors)
	}

	return nil
}

// populateConfiguredMACs extracts MAC addresses from the VM configuration (net0, net1, etc.)
func populateConfiguredMACs(vm *VM, configData map[string]interface{}) {
	vm.ConfiguredMACs = make(map[string]bool)
	for k, v := range configData {
		if strings.HasPrefix(k, "net") && len(k) > 3 && k[3] >= '0' && k[3] <= '9' {
			netStr, ok := v.(string)
			if !ok {
				continue
			}
			// QEMU Example net string: virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0
			// LXC Example net string: name=eth0,hwaddr=AA:BB:CC:DD:EE:FF,bridge=vmbr0,ip=dhcp
			parts := strings.Split(netStr, ",")
			for _, part := range parts {
				var macAddress string
				if strings.HasPrefix(part, "hwaddr=") { // LXC MAC format
					macAddress = strings.ToUpper(strings.TrimPrefix(part, "hwaddr="))
				} else if strings.Contains(part, "=") { // QEMU MAC format (e.g., virtio=...)
					macParts := strings.SplitN(part, "=", 2)
					if len(macParts) == 2 {
						if len(macParts[1]) == 17 && strings.Count(macParts[1], ":") == 5 {
							macAddress = strings.ToUpper(macParts[1])
						}
					}
				} else { // QEMU MAC format (just the MAC)
					if len(part) == 17 && strings.Count(part, ":") == 5 {
						macAddress = strings.ToUpper(part)
					}
				}

				if macAddress != "" && len(macAddress) == 17 && strings.Count(macAddress, ":") == 5 {
					vm.ConfiguredMACs[macAddress] = true
					break // Found MAC for this netX device
				}
			}
		}
	}
}

// populateConfigDetails extracts detailed configuration information from VM config data.
//
// This function parses the raw configuration data returned by the Proxmox API config endpoint
// and populates the VM struct with structured configuration information including:
//   - CPU configuration (cores, sockets)
//   - System settings (architecture, OS type, boot order)
//   - Administrative settings (description, auto-start)
//   - Network interface configuration
//   - Storage device configuration
//
// The function handles both QEMU VMs and LXC containers, adapting the parsing logic
// based on the VM type. It safely handles missing fields and type conversions.
//
// Parameters:
//   - vm: The VM struct to populate with configuration details
//   - configData: Raw configuration data from the Proxmox API config endpoint
func populateConfigDetails(vm *VM, configData map[string]interface{}) {
	// Parse CPU configuration
	if cores, ok := configData["cores"]; ok {
		if coresFloat, ok := cores.(float64); ok {
			vm.CPUCores = int(coresFloat)
		}
	}

	if sockets, ok := configData["sockets"]; ok {
		if socketsFloat, ok := sockets.(float64); ok {
			vm.CPUSockets = int(socketsFloat)
		}
	}

	// Parse architecture and OS type
	if arch, ok := configData["arch"].(string); ok {
		vm.Architecture = arch
	}

	if ostype, ok := configData["ostype"].(string); ok {
		vm.OSType = ostype
	}

	// Parse description
	if desc, ok := configData["description"].(string); ok {
		vm.Description = desc
	}

	// Parse boot order
	if boot, ok := configData["boot"].(string); ok {
		vm.BootOrder = boot
	}

	// Parse onboot setting
	if onboot, ok := configData["onboot"]; ok {
		switch v := onboot.(type) {
		case bool:
			vm.OnBoot = v
		case int:
			vm.OnBoot = v != 0
		case float64:
			vm.OnBoot = v != 0
		case string:
			vm.OnBoot = v == "1" || strings.ToLower(v) == StringTrue
		}
	}

	// Parse network interfaces
	vm.ConfiguredNetworks = parseNetworkConfig(configData, vm.Type)

	// Parse storage devices
	vm.StorageDevices = parseStorageConfig(configData, vm.Type)
}

// parseNetworkConfig extracts network interface configuration from VM config data.
//
// This function parses network interface configuration strings from the VM config
// and returns a slice of ConfiguredNetwork structs. It handles the different formats
// used by QEMU VMs and LXC containers:
//
// QEMU format: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,tag=100,firewall=1"
// LXC format: "name=eth0,hwaddr=AA:BB:CC:DD:EE:FF,bridge=vmbr0,ip=dhcp,gw=192.168.1.1"
//
// The function extracts all network-related configuration including MAC addresses,
// bridge assignments, VLAN tags, IP configuration, and firewall settings.
//
// Parameters:
//   - configData: Raw configuration data from the Proxmox API
//   - vmType: VM type ("qemu" or "lxc") to determine parsing format
//
// Returns a slice of ConfiguredNetwork structs containing parsed network configuration.
func parseNetworkConfig(configData map[string]interface{}, vmType string) []ConfiguredNetwork {
	var networks []ConfiguredNetwork

	for key, value := range configData {
		if !strings.HasPrefix(key, "net") || len(key) < 4 {
			continue
		}

		// Check if this is a network interface (net0, net1, etc.)
		if key[3] < '0' || key[3] > '9' {
			continue
		}

		netStr, ok := value.(string)
		if !ok {
			continue
		}

		network := ConfiguredNetwork{
			Interface: key,
		}

		// Parse network configuration string
		parts := strings.Split(netStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			if strings.Contains(part, "=") {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])

				switch key {
				case "name":
					if vmType == VMTypeLXC {
						network.Model = val // For LXC, this is the interface name
					}
				case "hwaddr":
					network.MACAddr = strings.ToUpper(val)
				case "bridge":
					network.Bridge = val
				case "tag":
					network.VLAN = val
				case "rate":
					network.Rate = val
				case "ip":
					network.IP = val
				case "gw":
					network.Gateway = val
				case "firewall":
					network.Firewall = val == "1" || strings.ToLower(val) == StringTrue
				default:
					// For QEMU, check if this is a model=MAC pair
					if vmType == VMTypeQemu && network.Model == "" {
						// Check if this looks like a network model (virtio, e1000, etc.)
						if len(val) == 17 && strings.Count(val, ":") == 5 {
							// This is model=MAC format
							network.Model = key
							network.MACAddr = strings.ToUpper(val)
						}
					}
				}
			}
		}

		networks = append(networks, network)
	}

	return networks
}

// parseStorageConfig extracts storage device configuration from VM config data.
//
// This function parses storage device configuration strings from the VM config
// and returns a slice of StorageDevice structs. It handles different storage types
// for QEMU VMs and LXC containers:
//
// QEMU devices: scsi0, ide0, virtio0, sata0, efidisk0, etc.
// LXC devices: rootfs, mp0, mp1, etc. (mount points)
//
// Example configurations:
//
//	QEMU: "local-lvm:vm-100-disk-0,size=32G,cache=writeback,iothread=1,ssd=1"
//	LXC: "local-lvm:vm-101-disk-0,size=8G"
//	Direct device: "/dev/disk/by-id/ata-SAMSUNG-SSD,size=500G,ssd=1,discard=on"
//
// The function extracts storage pool/path, size, format, performance options,
// and backup/replication settings.
//
// Parameters:
//   - configData: Raw configuration data from the Proxmox API
//   - vmType: VM type ("qemu" or "lxc") to determine device types to parse
//
// Returns a slice of StorageDevice structs containing parsed storage configuration.
func parseStorageConfig(configData map[string]interface{}, vmType string) []StorageDevice {
	var devices []StorageDevice

	// Storage device prefixes to look for
	var devicePrefixes []string
	if vmType == VMTypeQemu {
		devicePrefixes = []string{"scsi", "ide", "virtio", "sata", "efidisk"}
	} else {
		devicePrefixes = []string{"rootfs", "mp"} // LXC mount points
	}

	for key, value := range configData {
		var isStorageDevice bool
		for _, prefix := range devicePrefixes {
			if strings.HasPrefix(key, prefix) {
				isStorageDevice = true
				break
			}
		}

		if !isStorageDevice {
			continue
		}

		deviceStr, ok := value.(string)
		if !ok {
			continue
		}

		device := StorageDevice{
			Device:    key,
			Backup:    true,  // Default to true
			Replicate: false, // Default to false
		}

		// Parse device configuration string
		parts := strings.Split(deviceStr, ",")
		if len(parts) > 0 {
			// First part is usually storage:size or path
			mainPart := strings.TrimSpace(parts[0])
			if strings.Contains(mainPart, ":") {
				storageParts := strings.SplitN(mainPart, ":", 2)
				device.Storage = storageParts[0]
				// The second part might contain size info
				if len(storageParts) > 1 {
					// For direct device paths, store the full path
					if strings.HasPrefix(storageParts[0], "/dev/") {
						device.Storage = mainPart
					}
				}
			} else {
				device.Storage = mainPart
			}
		}

		// Parse additional options
		for i := 1; i < len(parts); i++ {
			part := strings.TrimSpace(parts[i])
			if part == "" {
				continue
			}

			if strings.Contains(part, "=") {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) != 2 {
					continue
				}

				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])

				switch key {
				case "size":
					device.Size = val
				case "format":
					device.Format = val
				case "cache":
					device.Cache = val
				case "iothread":
					device.IOThread = val == "1" || strings.ToLower(val) == StringTrue
				case "ssd":
					device.SSD = val == "1" || strings.ToLower(val) == "true"
				case "discard":
					device.Discard = val
				case "serial":
					device.Serial = val
				case "backup":
					device.Backup = val != "0" && strings.ToLower(val) != "false"
				case "replicate":
					device.Replicate = val == "1" || strings.ToLower(val) == "true"
				case "media":
					if val == "cdrom" {
						device.Media = "cdrom"
					}
				}
			} else {
				// Handle boolean flags without values
				switch part {
				case "iothread":
					device.IOThread = true
				case "ssd":
					device.SSD = true
				case "discard=on":
					device.Discard = "on"
				}
			}
		}

		devices = append(devices, device)
	}

	return devices
}

// GetGuestAgentFilesystems retrieves filesystem information from the QEMU guest agent
func (c *Client) GetGuestAgentFilesystems(vm *VM) ([]Filesystem, error) {
	if vm.Type != VMTypeQemu || vm.Status != VMStatusRunning {
		return nil, fmt.Errorf("guest agent not applicable for this VM type or status")
	}

	if !vm.AgentEnabled {
		return nil, fmt.Errorf("guest agent is not enabled for this VM")
	}

	var res map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/qemu/%d/agent/get-fsinfo", vm.Node, vm.ID)

	if err := c.GetWithCache(endpoint, &res, VMDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get filesystem info from guest agent: %w", err)
	}

	// Check if result exists in the response
	resultArray, ok := res["result"].([]interface{})
	if !ok {
		// Try checking if data.result exists (some Proxmox API endpoints use different formats)
		data, dataOk := res["data"].(map[string]interface{})
		if !dataOk {
			return nil, fmt.Errorf("unexpected response format from guest agent")
		}

		resultArray, ok = data["result"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected result format from guest agent")
		}
	}

	var filesystems []Filesystem

	for _, fs := range resultArray {
		fsMap, ok := fs.(map[string]interface{})
		if !ok {
			continue
		}

		filesystem := Filesystem{}

		// Get filesystem properties
		if name, ok := fsMap["name"].(string); ok {
			filesystem.Name = name
		}

		if mountpoint, ok := fsMap["mountpoint"].(string); ok {
			filesystem.Mountpoint = mountpoint
			// Check if it's the root filesystem in Linux
			filesystem.IsRoot = mountpoint == "/"
			// Check if it's the system drive in Windows (C:\ or C:/)
			filesystem.IsSystemDrive = strings.HasPrefix(strings.ToLower(mountpoint), "c:")
		}

		if fsType, ok := fsMap["type"].(string); ok {
			filesystem.Type = fsType
		}

		if totalBytes, ok := fsMap["total-bytes"].(float64); ok {
			filesystem.TotalBytes = int64(totalBytes)
		}

		if usedBytes, ok := fsMap["used-bytes"].(float64); ok {
			filesystem.UsedBytes = int64(usedBytes)
		}

		// Get the first disk device if available
		if diskArray, ok := fsMap["disk"].([]interface{}); ok && len(diskArray) > 0 {
			if diskMap, ok := diskArray[0].(map[string]interface{}); ok {
				if dev, ok := diskMap["dev"].(string); ok {
					filesystem.Device = dev
				}
			}
		}

		filesystems = append(filesystems, filesystem)
	}

	return filesystems, nil
}
