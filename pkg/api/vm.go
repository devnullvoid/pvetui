package api

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
)

// VM represents a Proxmox VM or container with comprehensive configuration and runtime information.
//
// This struct contains both runtime metrics (CPU usage, memory, network I/O) and detailed
// configuration information parsed from the VM's config endpoint. The configuration details
// include network interfaces, storage devices, CPU settings, and other system configuration.
//
// The struct is populated through multiple API calls:
//   - Basic VM information from cluster resources
//   - Runtime metrics from status/current endpoint
//   - Configuration details from config endpoint
//   - Guest agent information (for QEMU VMs with agent enabled)
//
// Example usage:
//
//	vm, err := client.GetDetailedVmInfo("node1", "qemu", 100)
//	if err != nil {
//		return err
//	}
//
//	// Access runtime information
//	fmt.Printf("VM %s is %s, CPU: %.1f%%, Memory: %s\n",
//		vm.Name, vm.Status, vm.CPU*100, utils.FormatBytes(vm.Mem))
//
//	// Access configuration details
//	for _, net := range vm.ConfiguredNetworks {
//		fmt.Printf("Interface %s: %s on bridge %s\n",
//			net.Interface, net.MACAddr, net.Bridge)
//	}
type VM struct {
	// Basic identification and status
	ID     int    `json:"id"`           // VM ID (unique within cluster)
	Name   string `json:"name"`         // VM name
	Node   string `json:"node"`         // Proxmox node hosting this VM
	Type   string `json:"type"`         // VM type: "qemu" or "lxc"
	Status string `json:"status"`       // Current status: "running", "stopped", etc.
	IP     string `json:"ip,omitempty"` // Primary IP address (from config or guest agent)

	// Runtime resource usage metrics
	CPU       float64 `json:"cpu,omitempty"`       // CPU usage as percentage (0.0-1.0)
	Mem       int64   `json:"mem,omitempty"`       // Current memory usage in bytes
	MaxMem    int64   `json:"maxmem,omitempty"`    // Maximum memory allocation in bytes
	Disk      int64   `json:"disk,omitempty"`      // Current disk usage in bytes
	MaxDisk   int64   `json:"maxdisk,omitempty"`   // Maximum disk allocation in bytes
	Uptime    int64   `json:"uptime,omitempty"`    // Uptime in seconds
	DiskRead  int64   `json:"diskread,omitempty"`  // Total disk read bytes
	DiskWrite int64   `json:"diskwrite,omitempty"` // Total disk write bytes
	NetIn     int64   `json:"netin,omitempty"`     // Total network input bytes
	NetOut    int64   `json:"netout,omitempty"`    // Total network output bytes

	// Administrative and cluster information
	HAState  string `json:"hastate,omitempty"`  // High availability state
	Lock     string `json:"lock,omitempty"`     // Lock status if VM is locked
	Tags     string `json:"tags,omitempty"`     // Comma-separated tags
	Template bool   `json:"template,omitempty"` // Whether this is a template
	Pool     string `json:"pool,omitempty"`     // Resource pool assignment

	// Guest agent related fields (QEMU VMs only)
	AgentEnabled   bool               `json:"agent_enabled,omitempty"`  // Whether guest agent is enabled
	AgentRunning   bool               `json:"agent_running,omitempty"`  // Whether guest agent is responding
	NetInterfaces  []NetworkInterface `json:"net_interfaces,omitempty"` // Network interfaces from guest agent
	Filesystems    []Filesystem       `json:"filesystems,omitempty"`    // Filesystem information from guest agent
	ConfiguredMACs map[string]bool    `json:"-"`                        // MAC addresses from VM config (internal use)

	// Configuration details from config endpoint
	ConfiguredNetworks []ConfiguredNetwork `json:"configured_networks,omitempty"` // Network interface configuration
	StorageDevices     []StorageDevice     `json:"storage_devices,omitempty"`     // Storage device configuration
	BootOrder          string              `json:"boot_order,omitempty"`          // Boot device order
	CPUCores           int                 `json:"cpu_cores,omitempty"`           // Number of CPU cores
	CPUSockets         int                 `json:"cpu_sockets,omitempty"`         // Number of CPU sockets
	Architecture       string              `json:"architecture,omitempty"`        // CPU architecture (amd64, arm64, etc.)
	OSType             string              `json:"ostype,omitempty"`              // Operating system type
	Description        string              `json:"description,omitempty"`         // VM description
	OnBoot             bool                `json:"onboot,omitempty"`              // Whether VM starts automatically

	// Internal fields for concurrency and state management
	mu       sync.RWMutex // Protects concurrent access to VM data
	Enriched bool         `json:"-"` // Whether VM has been enriched with detailed information
}

// ConfiguredNetwork represents a network interface configuration from VM config endpoint.
//
// This struct contains the network configuration as defined in the VM's configuration,
// which may differ from the runtime network information available through the guest agent.
// It includes both the network model/type and bridge configuration details.
//
// For QEMU VMs, the Model field typically contains values like "virtio", "e1000", "rtl8139".
// For LXC containers, the Model field contains the interface name like "eth0", "eth1".
//
// Example QEMU network config: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,tag=100,firewall=1"
// Example LXC network config: "name=eth0,hwaddr=AA:BB:CC:DD:EE:FF,bridge=vmbr0,ip=dhcp"
type ConfiguredNetwork struct {
	Interface string `json:"interface"`          // Interface identifier (net0, net1, etc.)
	Model     string `json:"model"`              // Network model (QEMU) or interface name (LXC)
	MACAddr   string `json:"mac_address"`        // Hardware MAC address
	Bridge    string `json:"bridge"`             // Bridge name (vmbr0, vmbr1, etc.)
	VLAN      string `json:"vlan,omitempty"`     // VLAN tag if configured
	Rate      string `json:"rate,omitempty"`     // Rate limiting (e.g., "1000" for 1000 MB/s)
	IP        string `json:"ip,omitempty"`       // Static IP configuration or "dhcp"
	Gateway   string `json:"gateway,omitempty"`  // Gateway IP (LXC containers)
	Firewall  bool   `json:"firewall,omitempty"` // Whether firewall is enabled for this interface
}

// StorageDevice represents a storage device configuration from VM config endpoint.
//
// This struct contains detailed storage configuration including the storage backend,
// performance settings, and device-specific options. The configuration varies between
// QEMU VMs and LXC containers.
//
// For QEMU VMs, devices include SCSI, IDE, VirtIO, SATA, and EFI disk devices.
// For LXC containers, devices include rootfs and mount points (mp0, mp1, etc.).
//
// Example QEMU storage: "local-lvm:vm-100-disk-0,size=32G,cache=writeback,iothread=1"
// Example LXC storage: "local-lvm:vm-101-disk-0,size=8G" (rootfs)
// Example direct device: "/dev/disk/by-id/ata-SAMSUNG-SSD,size=500G,ssd=1,discard=on"
type StorageDevice struct {
	Device    string `json:"device"`              // Device identifier (scsi0, ide0, virtio0, rootfs, mp0, etc.)
	Storage   string `json:"storage"`             // Storage pool name or device path
	Size      string `json:"size,omitempty"`      // Size specification (e.g., "32G", "500G")
	Format    string `json:"format,omitempty"`    // Storage format (raw, qcow2, vmdk, etc.)
	Cache     string `json:"cache,omitempty"`     // Cache mode (none, writethrough, writeback, etc.)
	IOThread  bool   `json:"iothread,omitempty"`  // Whether to use dedicated I/O thread
	SSD       bool   `json:"ssd,omitempty"`       // Whether device is SSD (affects scheduler)
	Discard   string `json:"discard,omitempty"`   // Discard mode (on, ignore) for TRIM support
	Serial    string `json:"serial,omitempty"`    // Custom serial number
	Backup    bool   `json:"backup"`              // Whether device is included in backups (default: true)
	Replicate bool   `json:"replicate,omitempty"` // Whether device participates in replication
}

// Filesystem represents filesystem information from QEMU guest agent
type Filesystem struct {
	Name          string `json:"name"`
	Mountpoint    string `json:"mountpoint"`
	Type          string `json:"type"`
	TotalBytes    int64  `json:"total_bytes"`
	UsedBytes     int64  `json:"used_bytes"`
	Device        string `json:"device,omitempty"`
	IsRoot        bool   `json:"-"` // Determined by mountpoint ("/")
	IsSystemDrive bool   `json:"-"` // For Windows C: drive
}

// GetVmStatus retrieves current status metrics for a VM or LXC
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
			rawNetInterfaces, err := c.GetGuestAgentInterfaces(vm)
			if err == nil && len(rawNetInterfaces) > 0 {
				vm.AgentRunning = true
				var filteredInterfaces []NetworkInterface
				for _, iface := range rawNetInterfaces {
					// Skip loopback and veth interfaces, and check against configured MACs
					if !iface.IsLoopback && !strings.HasPrefix(iface.Name, "veth") && (vm.ConfiguredMACs == nil || vm.ConfiguredMACs[strings.ToUpper(iface.MACAddress)]) {
						// Prioritize IPv4, then first IPv6, then no IP for this interface if none match
						var bestIP IPAddress
						foundIP := false
						for _, ip := range iface.IPAddresses {
							if ip.Type == IPTypeIPv4 {
								bestIP = ip
								foundIP = true
								break
							}
						}
						if !foundIP && len(iface.IPAddresses) > 0 {
							// If no IPv4, take the first IPv6 (or any first IP if types are mixed unexpectedly)
							for _, ip := range iface.IPAddresses {
								if ip.Type == IPTypeIPv6 { // Explicitly look for IPv6 first
									bestIP = ip
									foundIP = true
									break
								}
							}
							if !foundIP { // Fallback to literally the first IP if no IPv6 was marked
								bestIP = iface.IPAddresses[0]
								foundIP = true
							}
						}

						if foundIP {
							iface.IPAddresses = []IPAddress{bestIP}
						} else {
							iface.IPAddresses = nil // No suitable IP found
						}
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
					// Prioritize IPv4, then first IPv6
					var bestIP IPAddress
					foundIP := false
					for _, ip := range iface.IPAddresses {
						if ip.Type == IPTypeIPv4 {
							bestIP = ip
							foundIP = true
							break
						}
					}
					if !foundIP && len(iface.IPAddresses) > 0 {
						for _, ip := range iface.IPAddresses { // Explicitly look for IPv6 first
							if ip.Type == IPTypeIPv6 {
								bestIP = ip
								foundIP = true
								break
							}
						}
						if !foundIP { // Fallback to literally the first IP if no IPv6 was marked
							bestIP = iface.IPAddresses[0]
							foundIP = true
						}
					}

					if foundIP {
						iface.IPAddresses = []IPAddress{bestIP}
					} else {
						iface.IPAddresses = nil
					}
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
	if err := c.GetWithCache(statusEndpoint, &statusRes, VMDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get VM status: %w", err)
	}

	statusData, ok := statusRes["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM status")
	}

	// Get config information
	var configRes map[string]interface{}
	configEndpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", node, vmType, vmid)
	if err := c.GetWithCache(configEndpoint, &configRes, VMDataTTL); err != nil {
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

	// Check for agent configuration in config data
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

	// Look for IPs in config (net0, net1, etc.)
	var foundIP bool
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
					// Only set IP if it's a valid IP address (skip "dhcp", "manual", etc.)
					if isValidIP(ip) {
						vm.IP = ip
						foundIP = true
						break
					}
				}
			}

			if foundIP {
				break // Found an IP, no need to check other interfaces
			}
		}
	}

	populateConfiguredMACs(vm, configData)
	populateConfigDetails(vm, configData)

	vm.Enriched = true
	return vm, nil
}

// isValidIP checks if a string is a valid IP address
func isValidIP(ip string) bool {
	// Skip common non-IP values
	if ip == "" || ip == "dhcp" || ip == "manual" || ip == "static" {
		return false
	}

	// Parse as IP address
	return net.ParseIP(ip) != nil
}

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

// StartVM starts a VM or container
func (c *Client) StartVM(vm *VM) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/status/start", vm.Node, vm.Type, vm.ID)
	return c.Post(path, nil)
}

// StopVM stops a VM or container
func (c *Client) StopVM(vm *VM) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/status/stop", vm.Node, vm.Type, vm.ID)
	return c.Post(path, nil)
}

// RestartVM restarts a VM or container
func (c *Client) RestartVM(vm *VM) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/status/restart", vm.Node, vm.Type, vm.ID)
	return c.Post(path, nil)
}

// DeleteVM permanently deletes a VM or container
// WARNING: This operation is irreversible and will destroy all VM data including disks
func (c *Client) DeleteVM(vm *VM) error {
	return c.DeleteVMWithOptions(vm, nil)
}

// DeleteVMOptions contains options for deleting a VM
type DeleteVMOptions struct {
	// Force deletion even if VM is running
	Force bool `json:"force,omitempty"`
	// Skip lock checking
	SkipLock bool `json:"skiplock,omitempty"`
	// Destroy unreferenced disks owned by guest
	DestroyUnreferencedDisks bool `json:"destroy-unreferenced-disks,omitempty"`
	// Remove VMID from configurations (backup, replication jobs, HA)
	Purge bool `json:"purge,omitempty"`
}

// DeleteVMWithOptions permanently deletes a VM or container with specific options
// WARNING: This operation is irreversible and will destroy all VM data including disks
func (c *Client) DeleteVMWithOptions(vm *VM, options *DeleteVMOptions) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d", vm.Node, vm.Type, vm.ID)

	// Build query parameters
	params := make(map[string]interface{})
	if options != nil {
		if options.Force {
			params["force"] = "1"
		}
		if options.SkipLock {
			params["skiplock"] = "1"
		}
		if options.DestroyUnreferencedDisks {
			params["destroy-unreferenced-disks"] = "1"
		}
		if options.Purge {
			params["purge"] = "1"
		}
	}

	// Add query parameters to path if any
	if len(params) > 0 {
		queryParts := make([]string, 0, len(params))
		for key, value := range params {
			queryParts = append(queryParts, fmt.Sprintf("%s=%v", key, value))
		}
		path += "?" + strings.Join(queryParts, "&")
	}

	return c.Delete(path)
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

// VNCProxyResponse represents the response from a VNC proxy request
type VNCProxyResponse struct {
	Ticket   string `json:"ticket"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Cert     string `json:"cert"`
	Password string `json:"password,omitempty"` // One-time password for WebSocket connections
}

// GetVNCProxy creates a VNC proxy for a VM and returns connection details
func (c *Client) GetVNCProxy(vm *VM) (*VNCProxyResponse, error) {
	c.logger.Info("Creating VNC proxy for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	if vm.Type != VMTypeQemu && vm.Type != VMTypeLXC {
		c.logger.Error("VNC proxy not supported for VM type: %s", vm.Type)
		return nil, fmt.Errorf("VNC proxy only available for QEMU VMs and LXC containers")
	}

	var res map[string]interface{}
	path := fmt.Sprintf("/nodes/%s/%s/%d/vncproxy", vm.Node, vm.Type, vm.ID)

	c.logger.Debug("VNC proxy API path for VM %s: %s", vm.Name, path)

	// POST request with websocket=1 parameter for noVNC compatibility
	data := map[string]interface{}{
		"websocket": 1,
	}

	c.logger.Debug("VNC proxy request data for VM %s: %+v", vm.Name, data)

	if err := c.PostWithResponse(path, data, &res); err != nil {
		c.logger.Error("Failed to create VNC proxy for VM %s: %v", vm.Name, err)
		return nil, fmt.Errorf("failed to create VNC proxy: %w", err)
	}

	c.logger.Debug("VNC proxy API response for VM %s: %+v", vm.Name, res)

	responseData, ok := res["data"].(map[string]interface{})
	if !ok {
		c.logger.Error("Unexpected VNC proxy response format for VM %s", vm.Name)
		return nil, fmt.Errorf("unexpected VNC proxy response format")
	}

	response := &VNCProxyResponse{}

	if ticket, ok := responseData["ticket"].(string); ok {
		response.Ticket = ticket
		c.logger.Debug("VNC proxy ticket obtained for VM %s (length: %d)", vm.Name, len(ticket))
	}

	if port, ok := responseData["port"].(string); ok {
		response.Port = port
		c.logger.Debug("VNC proxy port for VM %s: %s", vm.Name, port)
	} else if portFloat, ok := responseData["port"].(float64); ok {
		response.Port = fmt.Sprintf("%.0f", portFloat)
		c.logger.Debug("VNC proxy port for VM %s (converted from float): %s", vm.Name, response.Port)
	}

	if user, ok := responseData["user"].(string); ok {
		response.User = user
		c.logger.Debug("VNC proxy user for VM %s: %s", vm.Name, user)
	}

	if cert, ok := responseData["cert"].(string); ok {
		response.Cert = cert
		c.logger.Debug("VNC proxy certificate obtained for VM %s (length: %d)", vm.Name, len(cert))
	}

	c.logger.Info("VNC proxy created successfully for VM %s - Port: %s", vm.Name, response.Port)
	return response, nil
}

// GetVNCProxyWithWebSocket creates a VNC proxy for a VM with WebSocket support and one-time password
func (c *Client) GetVNCProxyWithWebSocket(vm *VM) (*VNCProxyResponse, error) {
	c.logger.Info("Creating VNC proxy with WebSocket for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	if vm.Type != VMTypeQemu && vm.Type != VMTypeLXC {
		c.logger.Error("VNC proxy with WebSocket not supported for VM type: %s", vm.Type)
		return nil, fmt.Errorf("VNC proxy only available for QEMU VMs and LXC containers")
	}

	var res map[string]interface{}
	path := fmt.Sprintf("/nodes/%s/%s/%d/vncproxy", vm.Node, vm.Type, vm.ID)

	c.logger.Debug("VNC proxy WebSocket API path for VM %s: %s", vm.Name, path)

	// Different parameters based on VM type
	// LXC containers don't support generate-password parameter
	var data map[string]interface{}
	if vm.Type == VMTypeLXC {
		// LXC containers only support websocket parameter
		data = map[string]interface{}{
			"websocket": 1,
		}
		c.logger.Debug("Using LXC-compatible parameters for VM %s (no generate-password)", vm.Name)
	} else {
		// QEMU VMs support both websocket and generate-password
		data = map[string]interface{}{
			"websocket":         1,
			"generate-password": 1,
		}
		c.logger.Debug("Using QEMU parameters for VM %s (with generate-password)", vm.Name)
	}

	c.logger.Debug("VNC proxy WebSocket request data for VM %s: %+v", vm.Name, data)

	if err := c.PostWithResponse(path, data, &res); err != nil {
		c.logger.Error("Failed to create VNC proxy with WebSocket for VM %s: %v", vm.Name, err)
		return nil, fmt.Errorf("failed to create VNC proxy with WebSocket: %w", err)
	}

	c.logger.Debug("VNC proxy WebSocket API response for VM %s: %+v", vm.Name, res)

	responseData, ok := res["data"].(map[string]interface{})
	if !ok {
		c.logger.Error("Unexpected VNC proxy WebSocket response format for VM %s", vm.Name)
		return nil, fmt.Errorf("unexpected VNC proxy response format")
	}

	response := &VNCProxyResponse{}

	if ticket, ok := responseData["ticket"].(string); ok {
		response.Ticket = ticket
		c.logger.Debug("VNC proxy WebSocket ticket obtained for VM %s (length: %d)", vm.Name, len(ticket))
	}

	if port, ok := responseData["port"].(string); ok {
		response.Port = port
		c.logger.Debug("VNC proxy WebSocket port for VM %s: %s", vm.Name, port)
	} else if portFloat, ok := responseData["port"].(float64); ok {
		response.Port = fmt.Sprintf("%.0f", portFloat)
		c.logger.Debug("VNC proxy WebSocket port for VM %s (converted from float): %s", vm.Name, response.Port)
	}

	if user, ok := responseData["user"].(string); ok {
		response.User = user
		c.logger.Debug("VNC proxy WebSocket user for VM %s: %s", vm.Name, user)
	}

	if cert, ok := responseData["cert"].(string); ok {
		response.Cert = cert
		c.logger.Debug("VNC proxy WebSocket certificate obtained for VM %s (length: %d)", vm.Name, len(cert))
	}

	// Password is only available for QEMU VMs with generate-password=1
	if password, ok := responseData["password"].(string); ok {
		response.Password = password
		c.logger.Debug("VNC proxy one-time password obtained for VM %s (length: %d)", vm.Name, len(password))
	} else if vm.Type == VMTypeLXC {
		c.logger.Debug("No one-time password for LXC container %s (expected behavior)", vm.Name)
	} else {
		c.logger.Debug("No one-time password in response for QEMU VM %s (unexpected)", vm.Name)
	}

	c.logger.Info("VNC proxy with WebSocket created successfully for VM %s - Port: %s, Has Password: %t",
		vm.Name, response.Port, response.Password != "")
	return response, nil
}

// GenerateVNCURL creates a noVNC console URL for the given VM
func (c *Client) GenerateVNCURL(vm *VM) (string, error) {
	c.logger.Info("Generating VNC URL for VM: %s (ID: %d, Type: %s, Node: %s)", vm.Name, vm.ID, vm.Type, vm.Node)

	// Get VNC proxy details
	c.logger.Debug("Requesting VNC proxy for URL generation for VM %s", vm.Name)
	proxy, err := c.GetVNCProxy(vm)
	if err != nil {
		c.logger.Error("Failed to get VNC proxy for URL generation for VM %s: %v", vm.Name, err)
		return "", err
	}

	// Extract server details from base URL
	serverURL := strings.TrimSuffix(c.baseURL, "/api2/json")
	c.logger.Debug("Base server URL for VM %s: %s", vm.Name, serverURL)

	// URL encode the VNC ticket (critical for avoiding 401 errors)
	encodedTicket := url.QueryEscape(proxy.Ticket)
	c.logger.Debug("VNC ticket encoded for VM %s (original length: %d, encoded length: %d)",
		vm.Name, len(proxy.Ticket), len(encodedTicket))

	// Determine console type based on VM type
	consoleType := "kvm"
	if vm.Type == VMTypeLXC {
		consoleType = "lxc"
	}
	c.logger.Debug("Console type for VM %s: %s", vm.Name, consoleType)

	// Build the noVNC console URL using the working format from the forum post
	// Format: https://server:8006/?console=kvm&novnc=1&vmid=100&vmname=vmname&node=nodename&resize=off&cmd=&vncticket=encoded_ticket
	vncURL := fmt.Sprintf("%s/?console=%s&novnc=1&vmid=%d&vmname=%s&node=%s&resize=off&cmd=&vncticket=%s",
		serverURL, consoleType, vm.ID, url.QueryEscape(vm.Name), vm.Node, encodedTicket)

	c.logger.Info("VNC URL generated successfully for VM %s", vm.Name)
	c.logger.Debug("VNC URL for VM %s: %s", vm.Name, vncURL)

	return vncURL, nil
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
			vm.OnBoot = v == "1" || strings.ToLower(v) == "true"
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
					network.Firewall = val == "1" || strings.ToLower(val) == "true"
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
					device.IOThread = val == "1" || strings.ToLower(val) == "true"
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
