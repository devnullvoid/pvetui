package main

import (
	"fmt"
	"sync"
)

type MockState struct {
	mu      sync.RWMutex
	Nodes   []*MockNode
	VMs     map[string]*MockVM // Key: vmid (string)
	Storage []*MockStorage
}

type MockNode struct {
	Name    string
	ID      string
	Online  int // 1 or 0
	IP      string
	Uptime  int64
	MaxCPU  float64
	MaxMem  int64
	MaxDisk int64
    // Detailed stats
    KernelVersion string
    PVEVersion    string
    CPUModel      string
    CPUSockets    int
    CPUCores      int
}

type MockVM struct {
	ID        int
	Name      string
	Node      string
	Type      string // "qemu" or "lxc"
	Status    string // "running", "stopped"
	MaxMem    int64
	MaxDisk   int64
	CPUs      float64
	Uptime    int64
	NetIn     int64
	NetOut    int64
	DiskRead  int64
	DiskWrite int64

    // Configuration
    Config map[string]interface{}
}

type MockStorage struct {
	ID         string // "local"
	Node       string
	Type       string
	Content    string
	Disk       int64
	MaxDisk    int64
	Status     string // "active"
	Shared     int
}

func NewMockState() *MockState {
	state := &MockState{
		VMs: make(map[string]*MockVM),
	}

	// Default Node
	node := &MockNode{
		Name:          "pve",
		ID:            "node/pve",
		Online:        1,
		IP:            "127.0.0.1",
		Uptime:        10000,
		MaxCPU:        16,
		MaxMem:        32 * 1024 * 1024 * 1024, // 32GB
		MaxDisk:       1000 * 1024 * 1024 * 1024, // 1TB
        KernelVersion: "6.5.11-7-pve",
        PVEVersion:    "8.1.3",
        CPUModel:      "Intel(R) Xeon(R) CPU E5-2670 v2 @ 2.50GHz",
        CPUSockets:    2,
        CPUCores:      8,
	}
	state.Nodes = append(state.Nodes, node)

	// Default Storage
	storage := &MockStorage{
		ID:      "local",
		Node:    "pve",
		Type:    "dir",
		Content: "iso,vztmpl,backup",
		Disk:    10 * 1024 * 1024 * 1024,
		MaxDisk: 100 * 1024 * 1024 * 1024,
		Status:  "active",
		Shared:  0,
	}
	storageZfs := &MockStorage{
		ID:      "local-zfs",
		Node:    "pve",
		Type:    "zfspool",
		Content: "images,rootdir",
		Disk:    50 * 1024 * 1024 * 1024,
		MaxDisk: 900 * 1024 * 1024 * 1024,
		Status:  "active",
		Shared:  0,
	}
	state.Storage = append(state.Storage, storage, storageZfs)

	// Default VMs
	vm1 := &MockVM{
		ID:      100,
		Name:    "test-vm",
		Node:    "pve",
		Type:    "qemu",
		Status:  "running",
		MaxMem:  4 * 1024 * 1024 * 1024,
		MaxDisk: 32 * 1024 * 1024 * 1024,
		CPUs:    2,
		Uptime:  3600,
        Config: map[string]interface{}{
            "name": "test-vm",
            "memory": "4096",
            "cores": "2",
            "sockets": "1",
            "net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
            "scsi0": "local-zfs:vm-100-disk-0,size=32G",
            "ostype": "l26",
            "boot": "order=scsi0;ide2;net0",
        },
	}
	vm2 := &MockVM{
		ID:      101,
		Name:    "test-ct",
		Node:    "pve",
		Type:    "lxc",
		Status:  "stopped",
		MaxMem:  512 * 1024 * 1024,
		MaxDisk: 8 * 1024 * 1024 * 1024,
		CPUs:    1,
		Uptime:  0,
        Config: map[string]interface{}{
            "hostname": "test-ct",
            "memory": "512",
            "cores": "1",
            "net0": "name=eth0,bridge=vmbr0,hwaddr=AA:BB:CC:DD:EE:01,ip=dhcp",
            "rootfs": "local-zfs:subvol-101-disk-0,size=8G",
            "ostype": "debian",
        },
	}
	state.VMs["100"] = vm1
	state.VMs["101"] = vm2

	return state
}

func (s *MockState) GetClusterResources() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var resources []map[string]interface{}

	// Nodes
	for _, n := range s.Nodes {
		resources = append(resources, map[string]interface{}{
			"id":      n.ID,
			"type":    "node",
			"node":    n.Name,
			"status":  "online",
			"maxcpu":  n.MaxCPU,
			"maxmem":  n.MaxMem,
			"maxdisk": n.MaxDisk,
			"uptime":  n.Uptime,
			"cpu":     0.1, // mock usage
			"mem":     1024 * 1024 * 1024,
			"disk":    1024 * 1024 * 1024,
		})
	}

	// Storage
	for _, st := range s.Storage {
		resources = append(resources, map[string]interface{}{
			"id":         "storage/" + st.Node + "/" + st.ID,
			"storage":    st.ID,
			"type":       "storage",
			"node":       st.Node,
			"status":     st.Status,
			"maxdisk":    st.MaxDisk,
			"disk":       st.Disk,
			"content":    st.Content,
			"plugintype": st.Type,
			"shared":     st.Shared,
		})
	}

	// VMs
	for _, vm := range s.VMs {
		resources = append(resources, map[string]interface{}{
			"id":        fmt.Sprintf("%s/%d", vm.Type, vm.ID),
			"vmid":      vm.ID,
			"type":      vm.Type,
			"node":      vm.Node,
			"status":    vm.Status,
			"name":      vm.Name,
			"maxcpu":    vm.CPUs,
			"maxmem":    vm.MaxMem,
			"maxdisk":   vm.MaxDisk,
			"uptime":    vm.Uptime,
			"netin":     vm.NetIn,
			"netout":    vm.NetOut,
			"diskread":  vm.DiskRead,
			"diskwrite": vm.DiskWrite,
			"cpu":       0.05, // mock usage
			"mem":       vm.MaxMem / 2,
            "template":  0,
		})
	}

	return resources
}

func (s *MockState) UpdateVMStatus(vmid string, action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.VMs[vmid]
	if !ok {
		return fmt.Errorf("vm not found")
	}

	switch action {
	case "start":
		vm.Status = "running"
		vm.Uptime = 1 // Just started
	case "stop", "shutdown":
		vm.Status = "stopped"
		vm.Uptime = 0
	case "reboot":
		vm.Status = "running"
		vm.Uptime = 1
	}
	return nil
}

func (s *MockState) DeleteVM(vmid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.VMs[vmid]; !ok {
		return fmt.Errorf("vm not found")
	}
	delete(s.VMs, vmid)
	return nil
}

func (s *MockState) CreateVM(vmid int, name string, vmType string, node string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.VMs[fmt.Sprintf("%d", vmid)] = &MockVM{
		ID:     vmid,
		Name:   name,
		Node:   node,
		Type:   vmType,
		Status: "stopped",
		MaxMem: 1024 * 1024 * 1024,
		CPUs:   1,
        Config: map[string]interface{}{
            "name": name,
            "memory": "1024",
            "cores": "1",
        },
	}
}

func (s *MockState) UpdateVMConfig(vmid string, config map[string]interface{}) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    vm, ok := s.VMs[vmid]
    if !ok {
        return fmt.Errorf("vm not found")
    }

    for k, v := range config {
        vm.Config[k] = v
        if k == "name" || k == "hostname" {
            if val, ok := v.(string); ok {
                vm.Name = val
            }
        }
    }
    return nil
}
