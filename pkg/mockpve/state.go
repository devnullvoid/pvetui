package mockpve

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	taskStatusRunning  = "running"
	taskStatusStopped  = "stopped"
	guestStatusRunning = "running"
	guestStatusStopped = "stopped"
	guestTypeQEMU      = "qemu"
	guestTypeLXC       = "lxc"
	contentTemplate    = "vztmpl"
	contentImport      = "import"
)

type MockState struct {
	mu             sync.RWMutex
	Nodes          []*MockNode
	VMs            map[string]*MockVM // Key: vmid (string)
	Storage        []*MockStorage
	StorageContent map[string]map[string]*MockStorageVolume // Key: node/storage -> volid -> volume
	Backups        map[string]*MockBackup                   // Key: volid
	Tasks          map[string]*MockTask                     // Key: upid
	NextID         int
	taskDelay      time.Duration
}

type MockTask struct {
	UPID       string
	Node       string
	Type       string
	ID         string // e.g. "100"
	User       string
	StartTime  int64
	EndTime    int64
	Status     string // "running", "stopped", "OK", "ERR"
	ExitStatus string // "OK", "ERROR"
	onComplete func()
}

type MockBackup struct {
	VolID   string
	VMID    int
	Date    int64 // Unix timestamp
	Size    int64
	Storage string
	Format  string
	Notes   string
	Content string // "backup"
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
	ID      string // "local"
	Node    string
	Type    string
	Content string
	Disk    int64
	MaxDisk int64
	Status  string // "active"
	Shared  int
}

type MockStorageVolume struct {
	VolID     string
	Node      string
	Storage   string
	Content   string
	Format    string
	Size      int64
	Used      int64
	VMID      int
	CreatedAt int64
	Notes     string
	Parent    string
	Protected bool
}

const defaultMockTaskDelay = 250 * time.Millisecond

func NewMockState() *MockState {
	state := &MockState{
		VMs:            make(map[string]*MockVM),
		StorageContent: make(map[string]map[string]*MockStorageVolume),
		Backups:        make(map[string]*MockBackup),
		Tasks:          make(map[string]*MockTask),
		NextID:         102,
		taskDelay:      defaultMockTaskDelay,
	}

	// Default Node
	node := &MockNode{
		Name:          "pve",
		ID:            "node/pve",
		Online:        1,
		IP:            "127.0.0.1",
		Uptime:        10000,
		MaxCPU:        16,
		MaxMem:        32 * 1024 * 1024 * 1024,   // 32GB
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
		Content: "iso,vztmpl,backup,snippets",
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
		Type:    guestTypeQEMU,
		Status:  guestStatusRunning,
		MaxMem:  4 * 1024 * 1024 * 1024,
		MaxDisk: 32 * 1024 * 1024 * 1024,
		CPUs:    2,
		Uptime:  3600,
		Config: map[string]interface{}{
			"name":    "test-vm",
			"memory":  "4096",
			"cores":   "2",
			"sockets": "1",
			"net0":    "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
			"scsi0":   "local-zfs:vm-100-disk-0,size=32G",
			"ostype":  "l26",
			"boot":    "order=scsi0;ide2;net0",
		},
	}
	vm2 := &MockVM{
		ID:      101,
		Name:    "test-ct",
		Node:    "pve",
		Type:    guestTypeLXC,
		Status:  guestStatusStopped,
		MaxMem:  512 * 1024 * 1024,
		MaxDisk: 8 * 1024 * 1024 * 1024,
		CPUs:    1,
		Uptime:  0,
		Config: map[string]interface{}{
			"hostname": "test-ct",
			"memory":   "512",
			"cores":    "1",
			"net0":     "name=eth0,bridge=vmbr0,hwaddr=AA:BB:CC:DD:EE:01,ip=dhcp",
			"rootfs":   "local-zfs:subvol-101-disk-0,size=8G",
			"ostype":   "debian",
		},
	}
	state.VMs["100"] = vm1
	state.VMs["101"] = vm2

	state.addStorageVolumeLocked(&MockStorageVolume{
		VolID:     "local:iso/debian-12.5.iso",
		Node:      "pve",
		Storage:   "local",
		Content:   "iso",
		Format:    "iso",
		Size:      900 * 1024 * 1024,
		Used:      900 * 1024 * 1024,
		CreatedAt: 1704067200,
	})
	state.addStorageVolumeLocked(&MockStorageVolume{
		VolID:     "local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst",
		Node:      "pve",
		Storage:   "local",
		Content:   "vztmpl",
		Format:    "tar.zst",
		Size:      180 * 1024 * 1024,
		Used:      180 * 1024 * 1024,
		CreatedAt: 1706745600,
	})
	state.addStorageVolumeLocked(&MockStorageVolume{
		VolID:     "local:snippets/cloud-init-user-data.yaml",
		Node:      "pve",
		Storage:   "local",
		Content:   "snippets",
		Format:    "yaml",
		Size:      2048,
		Used:      2048,
		CreatedAt: 1709251200,
	})
	state.addStorageVolumeLocked(&MockStorageVolume{
		VolID:     "local:import/alpine-latest.oci",
		Node:      "pve",
		Storage:   "local",
		Content:   contentImport,
		Format:    "oci",
		Size:      350 * 1024 * 1024,
		Used:      350 * 1024 * 1024,
		CreatedAt: 1709500000,
		Notes:     "docker.io/library/alpine:latest",
	})
	state.addStorageVolumeLocked(&MockStorageVolume{
		VolID:     "local-zfs:vm-100-disk-0",
		Node:      "pve",
		Storage:   "local-zfs",
		Content:   "images",
		Format:    "raw",
		Size:      32 * 1024 * 1024 * 1024,
		Used:      32 * 1024 * 1024 * 1024,
		VMID:      100,
		CreatedAt: 1704067200,
	})
	state.addStorageVolumeLocked(&MockStorageVolume{
		VolID:     "local-zfs:subvol-101-disk-0",
		Node:      "pve",
		Storage:   "local-zfs",
		Content:   "rootdir",
		Format:    "subvol",
		Size:      8 * 1024 * 1024 * 1024,
		Used:      8 * 1024 * 1024 * 1024,
		VMID:      101,
		CreatedAt: 1704067200,
	})

	// Default Backups
	backup1 := &MockBackup{
		VolID:   "local:backup/vzdump-qemu-100-2023_01_01-12_00_00.vma.zst",
		VMID:    100,
		Date:    1672574400,
		Size:    2 * 1024 * 1024 * 1024,
		Storage: "local",
		Format:  "vma.zst",
		Notes:   "Initial backup",
		Content: "backup",
	}
	state.Backups[backup1.VolID] = backup1
	state.addStorageVolumeLocked(&MockStorageVolume{
		VolID:     backup1.VolID,
		Node:      "pve",
		Storage:   backup1.Storage,
		Content:   backup1.Content,
		Format:    backup1.Format,
		Size:      backup1.Size,
		Used:      backup1.Size,
		VMID:      backup1.VMID,
		CreatedAt: backup1.Date,
		Notes:     backup1.Notes,
	})

	state.recalculateStorageUsageLocked()

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

func (s *MockState) CreateTask(node, taskType, id, user string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	hexTime := fmt.Sprintf("%X", time.Now().Unix())
	upid := fmt.Sprintf("UPID:%s:%s:%s:%s:%s:%s:%s:", node, hexTime, "00000000", "00000000", taskType, id, user)

	task := &MockTask{
		UPID:      upid,
		Node:      node,
		Type:      taskType,
		ID:        id,
		User:      user,
		StartTime: time.Now().Unix(),
		Status:    taskStatusRunning,
	}
	s.Tasks[upid] = task

	// Auto-complete task after a short delay so tests can observe async transitions.
	go func(upid string) {
		time.Sleep(s.taskDelay)
		s.CompleteTask(upid, "OK")
	}(upid)

	return upid
}

func (s *MockState) createTaskWithCompletion(node, taskType, id, user string, onComplete func()) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	hexTime := fmt.Sprintf("%X", time.Now().Unix())
	upid := fmt.Sprintf("UPID:%s:%s:%s:%s:%s:%s:%s:", node, hexTime, "00000000", "00000000", taskType, id, user)

	task := &MockTask{
		UPID:       upid,
		Node:       node,
		Type:       taskType,
		ID:         id,
		User:       user,
		StartTime:  time.Now().Unix(),
		Status:     taskStatusRunning,
		onComplete: onComplete,
	}
	s.Tasks[upid] = task

	go func(upid string) {
		time.Sleep(s.taskDelay)
		s.CompleteTask(upid, "OK")
	}(upid)

	return upid
}

func (s *MockState) CompleteTask(upid, exitStatus string) {
	s.mu.Lock()
	task, ok := s.Tasks[upid]
	if !ok || task.Status == taskStatusStopped {
		s.mu.Unlock()
		return
	}
	task.Status = taskStatusStopped
	task.ExitStatus = exitStatus
	task.EndTime = time.Now().Unix()
	onComplete := task.onComplete
	s.mu.Unlock()

	if exitStatus == "OK" && onComplete != nil {
		onComplete()
	}
}

func (s *MockState) UpdateVMStatus(vmid string, action string) (string, error) {
	s.mu.Lock()
	// Check if VM exists
	vm, ok := s.VMs[vmid]
	s.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("vm not found")
	}

	// Queue task
	// NOTE: We queue the task first, but the state update (running -> stopped) happens when the task completes.
	// OR for simplicity in testing, we might update it immediately or inside the goroutine.
	// To test "Waiting", we should update it in the goroutine.

	upid := s.createTaskWithCompletion(vm.Node, "qm"+action, vmid, "root@pam", func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if vm, ok := s.VMs[vmid]; ok {
			switch action {
			case "start":
				vm.Status = guestStatusRunning
				vm.Uptime = 1
			case "stop", "shutdown":
				vm.Status = guestStatusStopped
				vm.Uptime = 0
			case "reboot":
				vm.Status = guestStatusRunning
				vm.Uptime = 1
			}
		}
	})

	return upid, nil
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
		Status: guestStatusStopped,
		MaxMem: 1024 * 1024 * 1024,
		CPUs:   1,
		Config: map[string]interface{}{
			"name":   name,
			"memory": "1024",
			"cores":  "1",
		},
	}
}

func (s *MockState) GetNextID(requested int) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if requested > 0 {
		if _, exists := s.VMs[strconv.Itoa(requested)]; exists {
			return 0, fmt.Errorf("vmid %d already exists", requested)
		}
		return requested, nil
	}

	nextID := s.NextID
	for {
		if _, exists := s.VMs[strconv.Itoa(nextID)]; !exists {
			return nextID, nil
		}
		nextID++
	}
}

func (s *MockState) ListGuests(node, vmType string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	guests := make([]map[string]interface{}, 0)
	for _, vm := range s.VMs {
		if vm.Node != node || vm.Type != vmType {
			continue
		}

		row := map[string]interface{}{
			"vmid":      vm.ID,
			"name":      vm.Name,
			"status":    vm.Status,
			"cpus":      vm.CPUs,
			"maxmem":    vm.MaxMem,
			"maxdisk":   vm.MaxDisk,
			"diskread":  vm.DiskRead,
			"diskwrite": vm.DiskWrite,
			"netin":     vm.NetIn,
			"netout":    vm.NetOut,
			"cpu":       0.05,
			"uptime":    vm.Uptime,
			"template":  0,
		}
		if vmType == guestTypeLXC {
			row["disk"] = vm.MaxDisk
		}
		guests = append(guests, row)
	}

	sort.Slice(guests, func(i, j int) bool {
		return guests[i]["vmid"].(int) < guests[j]["vmid"].(int)
	})

	return guests
}

func (s *MockState) ListNodeStorages(node string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	storages := make([]map[string]interface{}, 0)
	for _, st := range s.Storage {
		if st.Node != node {
			continue
		}
		storages = append(storages, map[string]interface{}{
			"storage": st.ID,
			"type":    st.Type,
			"content": st.Content,
			"used":    st.Disk,
			"total":   st.MaxDisk,
			"shared":  st.Shared,
			"enabled": 1,
			"active":  st.Status == "active",
		})
	}

	sort.Slice(storages, func(i, j int) bool {
		return storages[i]["storage"].(string) < storages[j]["storage"].(string)
	})

	return storages
}

func (s *MockState) ListStorageContent(node, storage, content string, vmid int) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows := make([]map[string]interface{}, 0)
	for _, item := range s.storageContentForLocked(node, storage) {
		if content != "" && item.Content != content {
			continue
		}
		if vmid > 0 && item.VMID != vmid {
			continue
		}

		row := map[string]interface{}{
			"volid":   item.VolID,
			"content": item.Content,
			"size":    item.Size,
			"used":    item.Used,
			"ctime":   item.CreatedAt,
			"format":  item.Format,
		}
		if item.Notes != "" {
			row["notes"] = item.Notes
		}
		if item.Parent != "" {
			row["parent"] = item.Parent
		}
		if item.VMID > 0 {
			row["vmid"] = item.VMID
		}
		if item.Content == "backup" {
			row["verification"] = "ok"
			row["protected"] = item.Protected
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i]["volid"].(string) < rows[j]["volid"].(string)
	})

	return rows
}

func (s *MockState) QueueCreateGuest(node, vmType string, params map[string]interface{}, isRestore bool) (string, error) {
	s.mu.RLock()
	hasNode := false
	for _, n := range s.Nodes {
		if n.Name == node {
			hasNode = true
			break
		}
	}
	s.mu.RUnlock()
	if !hasNode {
		return "", fmt.Errorf("node not found")
	}

	requestedVMID := getIntParam(params, "vmid", 0)
	forceRestore := isRestore && getBoolParam(params, "force", false)

	vmid := requestedVMID
	var err error
	if forceRestore && requestedVMID > 0 {
		s.mu.RLock()
		_, exists := s.VMs[strconv.Itoa(requestedVMID)]
		s.mu.RUnlock()
		if !exists {
			vmid, err = s.GetNextID(requestedVMID)
		}
	} else {
		vmid, err = s.GetNextID(requestedVMID)
	}
	if err != nil {
		return "", err
	}

	vm, volumes, err := buildGuest(s, node, vmType, vmid, params)
	if err != nil {
		return "", err
	}

	taskType := map[string]string{
		guestTypeQEMU: "qmcreate",
		guestTypeLXC:  "vzcreate",
	}[vmType]
	if isRestore {
		taskType = map[string]string{
			guestTypeQEMU: "qmrestore",
			guestTypeLXC:  "vzrestore",
		}[vmType]
	}

	upid := s.createTaskWithCompletion(node, taskType, strconv.Itoa(vmid), "root@pam", func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if forceRestore {
			s.deleteGuestVolumesLocked(vmid)
		}
		s.VMs[strconv.Itoa(vmid)] = vm
		for _, volume := range volumes {
			s.addStorageVolumeLocked(volume)
		}
		if vmid >= s.NextID {
			s.NextID = vmid + 1
		}
		s.recalculateStorageUsageLocked()
	})

	return upid, nil
}

func (s *MockState) QueueResizeGuestDisk(node, vmType, vmid, disk, size string) (string, error) {
	s.mu.RLock()
	_, ok := s.VMs[vmid]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("vm not found")
	}

	taskType := "qmresize"
	if vmType == guestTypeLXC {
		taskType = "pctresize"
	}

	upid := s.createTaskWithCompletion(node, taskType, vmid, "root@pam", func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		vm, ok := s.VMs[vmid]
		if !ok {
			return
		}

		currentValue, ok := vm.Config[disk].(string)
		if !ok || currentValue == "" {
			return
		}

		volID := extractVolumeID(currentValue)
		if volID == "" {
			return
		}

		volume, ok := s.getStorageVolumeLocked(volID)
		if !ok {
			return
		}

		newSize, err := applySizeDelta(volume.Size, size)
		if err != nil {
			return
		}

		volume.Size = newSize
		volume.Used = newSize
		vm.MaxDisk = newSize
		vm.Config[disk] = replaceConfigSize(currentValue, newSize)
		s.recalculateStorageUsageLocked()
	})

	return upid, nil
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

func (s *MockState) CreateBackup(vmid int, storage string, mode string, notes string) string {
	ts := time.Now().Unix()
	timestampStr := time.Now().Format("2006_01_02-15_04_05")

	vmType := guestTypeQEMU
	s.mu.RLock()
	if vm, ok := s.VMs[fmt.Sprintf("%d", vmid)]; ok {
		vmType = vm.Type
	}
	s.mu.RUnlock()

	volID := fmt.Sprintf("%s:backup/vzdump-%s-%d-%s.vma.zst", storage, vmType, vmid, timestampStr)

	return s.createTaskWithCompletion("pve", "vzdump", strconv.Itoa(vmid), "root@pam", func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		backup := &MockBackup{
			VolID:   volID,
			VMID:    vmid,
			Date:    ts,
			Size:    1024 * 1024 * 100,
			Storage: storage,
			Format:  "vma.zst",
			Notes:   notes,
			Content: "backup",
		}

		s.Backups[volID] = backup
		s.addStorageVolumeLocked(&MockStorageVolume{
			VolID:     volID,
			Node:      "pve",
			Storage:   storage,
			Content:   "backup",
			Format:    backup.Format,
			Size:      backup.Size,
			Used:      backup.Size,
			VMID:      vmid,
			CreatedAt: ts,
			Notes:     notes,
		})
		s.recalculateStorageUsageLocked()
	})
}

func (s *MockState) GetBackups(storage string) []*MockBackup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var backups []*MockBackup
	for _, b := range s.Backups {
		if b.Storage == storage {
			backups = append(backups, b)
		}
	}
	return backups
}

func (s *MockState) DeleteBackup(volID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle cases where volID might be full "storage:backup/..." or just "backup/..."
	// But our map keys are full volids.

	if _, ok := s.Backups[volID]; ok {
		delete(s.Backups, volID)
		return nil
	}

	// Check if volID is partial?
	// If the user passed "backup/..." but key is "local:backup/..."
	for k := range s.Backups {
		if strings.HasSuffix(k, volID) {
			delete(s.Backups, k)
			return nil
		}
	}

	return fmt.Errorf("backup not found")
}

func (s *MockState) QueueDeleteStorageContent(volume string) (string, error) {
	s.mu.RLock()
	volID, item, ok := s.findStorageVolumeLocked(volume)
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("storage content not found")
	}

	taskType := "imgdel"
	if item.Content == "backup" {
		taskType = "vzdumpdel"
	}

	return s.createTaskWithCompletion(item.Node, taskType, volID, "root@pam", func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.deleteStorageVolumeLocked(volID)
		delete(s.Backups, volID)
		s.recalculateStorageUsageLocked()
	}), nil
}

func (s *MockState) QueueDownloadStorageContent(node, storage string, params map[string]interface{}) (string, error) {
	urlValue := getStringParam(params, "url", "")
	content := getStringParam(params, "content", "")
	filename := getStringParam(params, "filename", "")

	if strings.TrimSpace(urlValue) == "" {
		return "", fmt.Errorf("url is required")
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content is required")
	}

	storageEntry, err := s.getStorage(node, storage)
	if err != nil {
		return "", err
	}
	if err := validateStorageDownloadContent(storageEntry, content); err != nil {
		return "", err
	}

	volume, err := buildDownloadedStorageVolume(node, storage, content, urlValue, filename)
	if err != nil {
		return "", err
	}

	return s.createTaskWithCompletion(node, "download", volume.VolID, "root@pam", func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.addStorageVolumeLocked(volume)
		s.recalculateStorageUsageLocked()
	}), nil
}

func (s *MockState) QueueOCIPullStorageContent(node, storage string, params map[string]interface{}) (string, error) {
	reference := getStringParam(params, "reference", "")
	filename := getStringParam(params, "filename", "")

	if strings.TrimSpace(reference) == "" {
		return "", fmt.Errorf("reference is required")
	}

	storageEntry, err := s.getStorage(node, storage)
	if err != nil {
		return "", err
	}
	if err := validateStorageDownloadContent(storageEntry, "import"); err != nil {
		return "", err
	}

	volume := buildOCIStorageVolume(node, storage, reference, filename)
	return s.createTaskWithCompletion(node, "ocipull", volume.VolID, "root@pam", func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.addStorageVolumeLocked(volume)
		s.recalculateStorageUsageLocked()
	}), nil
}

func (s *MockState) addStorageVolumeLocked(volume *MockStorageVolume) {
	key := storageContentKey(volume.Node, volume.Storage)
	if _, ok := s.StorageContent[key]; !ok {
		s.StorageContent[key] = make(map[string]*MockStorageVolume)
	}
	s.StorageContent[key][volume.VolID] = volume
}

func (s *MockState) getStorage(node, storage string) (*MockStorage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entry := range s.Storage {
		if entry.Node == node && entry.ID == storage {
			return entry, nil
		}
	}

	return nil, fmt.Errorf("storage not found")
}

func (s *MockState) deleteStorageVolumeLocked(volID string) {
	for key, volumes := range s.StorageContent {
		if _, ok := volumes[volID]; ok {
			delete(volumes, volID)
			if len(volumes) == 0 {
				delete(s.StorageContent, key)
			}
			return
		}
	}
}

func (s *MockState) deleteGuestVolumesLocked(vmid int) {
	for key, volumes := range s.StorageContent {
		for volID, volume := range volumes {
			if volume.VMID != vmid {
				continue
			}
			delete(volumes, volID)
		}
		if len(volumes) == 0 {
			delete(s.StorageContent, key)
		}
	}
}

func (s *MockState) storageContentForLocked(node, storage string) []*MockStorageVolume {
	volumes := s.StorageContent[storageContentKey(node, storage)]
	if len(volumes) == 0 {
		return nil
	}

	items := make([]*MockStorageVolume, 0, len(volumes))
	for _, item := range volumes {
		items = append(items, item)
	}
	return items
}

func (s *MockState) getStorageVolumeLocked(volID string) (*MockStorageVolume, bool) {
	for _, volumes := range s.StorageContent {
		if item, ok := volumes[volID]; ok {
			return item, true
		}
	}
	return nil, false
}

func (s *MockState) findStorageVolumeLocked(volume string) (string, *MockStorageVolume, bool) {
	for _, volumes := range s.StorageContent {
		if item, ok := volumes[volume]; ok {
			return volume, item, true
		}
		for volID, item := range volumes {
			if strings.HasSuffix(volID, volume) {
				return volID, item, true
			}
		}
	}
	return "", nil, false
}

func (s *MockState) recalculateStorageUsageLocked() {
	usage := make(map[string]int64)
	for key, volumes := range s.StorageContent {
		var total int64
		for _, volume := range volumes {
			total += volume.Size
		}
		usage[key] = total
	}

	for _, st := range s.Storage {
		st.Disk = usage[storageContentKey(st.Node, st.ID)]
	}
}

func validateStorageDownloadContent(storage *MockStorage, content string) error {
	if storage == nil {
		return fmt.Errorf("storage not found")
	}

	supported := splitStorageContent(storage.Content)
	switch content {
	case "iso", contentTemplate:
		if _, ok := supported[content]; ok {
			return nil
		}
		return fmt.Errorf("storage %s does not support %s content", storage.ID, content)
	case contentImport:
		if isFileBasedStorageType(storage.Type) {
			return nil
		}
		return fmt.Errorf("storage %s does not support import content", storage.ID)
	default:
		return fmt.Errorf("unsupported content type: %s", content)
	}
}

func splitStorageContent(raw string) map[string]struct{} {
	values := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values[part] = struct{}{}
	}
	return values
}

func isFileBasedStorageType(storageType string) bool {
	switch strings.ToLower(strings.TrimSpace(storageType)) {
	case "dir", "nfs", "cifs", "cephfs", "glusterfs", "btrfs":
		return true
	default:
		return false
	}
}

func buildDownloadedStorageVolume(node, storage, content, rawURL, filename string) (*MockStorageVolume, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid url: %w", err)
		}
		filename = path.Base(parsed.Path)
	}
	filename = normalizeStorageFilename(filename)
	if filename == "" {
		return nil, fmt.Errorf("filename could not be determined")
	}

	subdir := content
	if content == contentImport {
		subdir = contentImport
	}

	format := storageFormatFromFilename(filename, content)
	size := int64(700 * 1024 * 1024)
	switch content {
	case contentTemplate:
		size = 180 * 1024 * 1024
	case contentImport:
		size = 2 * 1024 * 1024 * 1024
	}

	return &MockStorageVolume{
		VolID:     fmt.Sprintf("%s:%s/%s", storage, subdir, filename),
		Node:      node,
		Storage:   storage,
		Content:   content,
		Format:    format,
		Size:      size,
		Used:      size,
		CreatedAt: time.Now().Unix(),
	}, nil
}

func buildOCIStorageVolume(node, storage, reference, filename string) *MockStorageVolume {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = normalizeStorageFilename(strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(reference))
	}
	if !strings.Contains(filename, ".") {
		filename += ".oci"
	}

	size := int64(350 * 1024 * 1024)
	return &MockStorageVolume{
		VolID:     fmt.Sprintf("%s:import/%s", storage, filename),
		Node:      node,
		Storage:   storage,
		Content:   contentImport,
		Format:    "oci",
		Size:      size,
		Used:      size,
		CreatedAt: time.Now().Unix(),
		Notes:     reference,
	}
}

func normalizeStorageFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, ":", "_")
	filename = strings.ReplaceAll(filename, " ", "_")
	filename = strings.Trim(filename, "._")
	return filename
}

func storageFormatFromFilename(filename, content string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".tar.zst"):
		return "tar.zst"
	case strings.HasSuffix(lower, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".qcow2"):
		return "qcow2"
	case strings.HasSuffix(lower, ".vma.zst"):
		return "vma.zst"
	case strings.HasSuffix(lower, ".ova"):
		return "ova"
	case strings.HasSuffix(lower, ".img"):
		return "img"
	case strings.HasSuffix(lower, ".iso"):
		return "iso"
	}

	if content == contentTemplate {
		return "tar"
	}
	if content == contentImport {
		return "raw"
	}
	return content
}

func buildGuest(state *MockState, node, vmType string, vmid int, params map[string]interface{}) (*MockVM, []*MockStorageVolume, error) {
	switch vmType {
	case guestTypeQEMU:
		return buildQEMUGuest(state, node, vmid, params)
	case guestTypeLXC:
		return buildLXCGuest(state, node, vmid, params)
	default:
		return nil, nil, fmt.Errorf("unsupported guest type: %s", vmType)
	}
}

func buildQEMUGuest(state *MockState, node string, vmid int, params map[string]interface{}) (*MockVM, []*MockStorageVolume, error) {
	name := getStringParam(params, "name", fmt.Sprintf("vm-%d", vmid))
	cores := getIntParam(params, "cores", 1)
	sockets := getIntParam(params, "sockets", 1)
	memoryMB := getIntParam(params, "memory", 1024)
	volumes := make([]*MockStorageVolume, 0)

	config := map[string]interface{}{
		"name":    name,
		"memory":  strconv.Itoa(memoryMB),
		"cores":   strconv.Itoa(cores),
		"sockets": strconv.Itoa(sockets),
		"ostype":  getStringParam(params, "ostype", "l26"),
	}

	maxDisk := int64(0)
	for _, diskKey := range []string{"scsi0", "virtio0", "sata0", "ide0"} {
		diskValue := getStringParam(params, diskKey, "")
		if diskValue == "" {
			continue
		}

		configValue, volume, err := buildAllocatedVolume(state, node, guestTypeQEMU, vmid, diskKey, diskValue, 0)
		if err != nil {
			return nil, nil, err
		}
		config[diskKey] = configValue
		if volume != nil {
			volumes = append(volumes, volume)
			maxDisk = volume.Size
		}
		break
	}

	if net0 := getStringParam(params, "net0", ""); net0 != "" {
		config["net0"] = net0
	}
	if cdrom := getStringParam(params, "cdrom", ""); cdrom != "" {
		config["ide2"] = ensureCDROMMedia(cdrom)
	} else if ide2 := getStringParam(params, "ide2", ""); ide2 != "" {
		config["ide2"] = ensureCDROMMedia(ide2)
	}
	if _, ok := config["boot"]; !ok {
		config["boot"] = "order=scsi0;ide2;net0"
	}

	vm := &MockVM{
		ID:      vmid,
		Name:    name,
		Node:    node,
		Type:    guestTypeQEMU,
		Status:  boolStatus(getBoolParam(params, "start", false)),
		MaxMem:  int64(memoryMB) * 1024 * 1024,
		MaxDisk: maxDisk,
		CPUs:    float64(cores * sockets),
		Config:  config,
	}
	if vm.Status == guestStatusRunning {
		vm.Uptime = 1
	}

	return vm, volumes, nil
}

func buildLXCGuest(state *MockState, node string, vmid int, params map[string]interface{}) (*MockVM, []*MockStorageVolume, error) {
	hostname := getStringParam(params, "hostname", fmt.Sprintf("ct-%d", vmid))
	cores := getIntParam(params, "cores", 1)
	memoryMB := getIntParam(params, "memory", 512)
	swapMB := getIntParam(params, "swap", 0)
	rootfs := getStringParam(params, "rootfs", "")
	if rootfs == "" {
		return nil, nil, fmt.Errorf("rootfs is required for LXC create")
	}

	configValue, volume, err := buildAllocatedVolume(state, node, guestTypeLXC, vmid, "rootfs", rootfs, 0)
	if err != nil {
		return nil, nil, err
	}

	config := map[string]interface{}{
		"hostname": hostname,
		"memory":   strconv.Itoa(memoryMB),
		"cores":    strconv.Itoa(cores),
		"rootfs":   configValue,
		"ostype":   getStringParam(params, "ostype", "debian"),
	}
	if template := getStringParam(params, "ostemplate", ""); template != "" {
		config["ostemplate"] = template
	}
	if net0 := getStringParam(params, "net0", ""); net0 != "" {
		config["net0"] = net0
	}
	if swapMB > 0 {
		config["swap"] = strconv.Itoa(swapMB)
	}
	if getBoolParam(params, "unprivileged", false) {
		config["unprivileged"] = "1"
	}

	vm := &MockVM{
		ID:      vmid,
		Name:    hostname,
		Node:    node,
		Type:    guestTypeLXC,
		Status:  boolStatus(getBoolParam(params, "start", false)),
		MaxMem:  int64(memoryMB) * 1024 * 1024,
		MaxDisk: volume.Size,
		CPUs:    float64(cores),
		Config:  config,
	}
	if vm.Status == guestStatusRunning {
		vm.Uptime = 1
	}

	return vm, []*MockStorageVolume{volume}, nil
}

func buildAllocatedVolume(state *MockState, node, vmType string, vmid int, diskKey, value string, index int) (string, *MockStorageVolume, error) {
	storage, payload, found := strings.Cut(value, ":")
	if !found || storage == "" || payload == "" {
		return value, nil, nil
	}

	payloadParts := strings.Split(payload, ",")
	first := payloadParts[0]
	if strings.Contains(first, "/") || strings.HasPrefix(first, "vm-") || strings.HasPrefix(first, "subvol-") {
		return value, nil, nil
	}

	sizeBytes, err := parseDiskSize(first)
	if err != nil {
		return "", nil, fmt.Errorf("invalid disk size %q: %w", value, err)
	}

	format := "raw"
	content := "images"
	volName := fmt.Sprintf("vm-%d-disk-%d", vmid, index)
	if vmType == guestTypeLXC {
		format = "subvol"
		content = "rootdir"
		volName = fmt.Sprintf("subvol-%d-disk-%d", vmid, index)
	}

	importFrom := ""
	for _, part := range payloadParts[1:] {
		if strings.HasPrefix(part, "format=") {
			format = strings.TrimPrefix(part, "format=")
		}
		if strings.HasPrefix(part, "import-from=") {
			importFrom = strings.TrimPrefix(part, "import-from=")
		}
	}

	if importFrom != "" {
		sourceVolume, err := findStorageVolume(state, importFrom)
		if err != nil {
			return "", nil, err
		}
		sizeBytes = sourceVolume.Size
		if sourceVolume.Format != "" {
			format = sourceVolume.Format
		}
	}

	volID := fmt.Sprintf("%s:%s", storage, volName)
	configValue := fmt.Sprintf("%s,size=%dG", volID, sizeBytes/(1024*1024*1024))
	if strings.Contains(value, "media=cdrom") {
		configValue += ",media=cdrom"
	}

	return configValue, &MockStorageVolume{
		VolID:     volID,
		Node:      node,
		Storage:   storage,
		Content:   content,
		Format:    format,
		Size:      sizeBytes,
		Used:      sizeBytes,
		VMID:      vmid,
		CreatedAt: time.Now().Unix(),
		Parent:    importFrom,
	}, nil
}

func findStorageVolume(state *MockState, volID string) (*MockStorageVolume, error) {
	if state == nil {
		return nil, fmt.Errorf("storage state is required")
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	_, volume, ok := state.findStorageVolumeLocked(volID)
	if !ok || volume == nil {
		return nil, fmt.Errorf("source volume not found: %s", volID)
	}

	cloned := *volume

	return &cloned, nil
}

func parseDiskSize(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	if raw == "" {
		return 0, fmt.Errorf("empty size")
	}

	multiplier := int64(1024 * 1024 * 1024)
	switch {
	case strings.HasSuffix(raw, "K"):
		multiplier = 1024
		raw = strings.TrimSuffix(raw, "K")
	case strings.HasSuffix(raw, "M"):
		multiplier = 1024 * 1024
		raw = strings.TrimSuffix(raw, "M")
	case strings.HasSuffix(raw, "G"):
		raw = strings.TrimSuffix(raw, "G")
	case strings.HasSuffix(raw, "T"):
		multiplier = 1024 * 1024 * 1024 * 1024
		raw = strings.TrimSuffix(raw, "T")
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return value * multiplier, nil
}

func applySizeDelta(current int64, raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty size")
	}

	if strings.HasPrefix(raw, "+") {
		delta, err := parseDiskSize(strings.TrimPrefix(raw, "+"))
		if err != nil {
			return 0, err
		}
		return current + delta, nil
	}

	return parseDiskSize(raw)
}

func replaceConfigSize(config string, sizeBytes int64) string {
	sizeToken := fmt.Sprintf("size=%dG", sizeBytes/(1024*1024*1024))
	if strings.Contains(config, "size=") {
		parts := strings.Split(config, ",")
		for i, part := range parts {
			if strings.HasPrefix(part, "size=") {
				parts[i] = sizeToken
			}
		}
		return strings.Join(parts, ",")
	}
	return config + "," + sizeToken
}

func extractVolumeID(config string) string {
	parts := strings.Split(config, ",")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func ensureCDROMMedia(value string) string {
	if strings.Contains(value, "media=cdrom") {
		return value
	}
	return value + ",media=cdrom"
}

func boolStatus(start bool) string {
	if start {
		return guestStatusRunning
	}
	return guestStatusStopped
}

func getStringParam(params map[string]interface{}, key, fallback string) string {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			return fallback
		}
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		return fallback
	}
}

func getIntParam(params map[string]interface{}, key string, fallback int) int {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func getBoolParam(params map[string]interface{}, key string, fallback bool) bool {
	value, ok := params[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		default:
			return fallback
		}
	default:
		return fallback
	}
}

func storageContentKey(node, storage string) string {
	return node + "/" + storage
}
