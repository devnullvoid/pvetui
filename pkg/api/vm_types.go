package api

import (
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
	Tags     string `json:"tags,omitempty"`     // Semicolon-separated tags
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

	// Group cluster support
	// SourceProfile is the profile name this VM came from in group cluster mode.
	// Empty for non-group mode. Used to track which Proxmox cluster a VM belongs to
	// when viewing multiple clusters together.
	SourceProfile string `json:"source_profile,omitempty"`

	// Internal fields for concurrency and state management
	mu                sync.RWMutex // Protects concurrent access to VM data
	Enriched          bool         `json:"-"` // Whether VM has been enriched with detailed information
	guestAgentChecked bool         // internal: true if guest agent API was already called this cycle
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
// Example LXC network config: "name=eth0,hwaddr=AA:BB:CC:DD:EE:FF,bridge=vmbr0,ip=dhcp".
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
// Example direct device: "/dev/disk/by-id/ata-SAMSUNG-SSD,size=500G,ssd=1,discard=on".
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
	Media     string `json:"media,omitempty"`     // Media type (e.g., "cdrom")
}

// Filesystem represents filesystem information from QEMU guest agent.
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
