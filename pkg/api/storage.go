package api

// Storage represents a Proxmox storage resource
type Storage struct {
	ID         string `json:"id"`         // Full ID like "storage/saturn/bigdiggus-ssd"
	Name       string `json:"storage"`    // Storage name like "bigdiggus-ssd"
	Content    string `json:"content"`    // Content types: "vztmpl,snippets,iso,rootdir,images"
	Disk       int64  `json:"disk"`       // Used space in bytes
	MaxDisk    int64  `json:"maxdisk"`    // Total space in bytes
	Node       string `json:"node"`       // Node name
	Plugintype string `json:"plugintype"` // Storage type: nfs, dir, lvmthin, zfspool, etc.
	Status     string `json:"status"`     // Status: available, etc.
	Shared     int    `json:"shared"`     // Whether storage is shared across nodes (1/0 from API)
	Type       string `json:"type"`       // Always "storage" from API
}

// IsShared returns true if this storage is shared across multiple nodes
func (s *Storage) IsShared() bool {
	return s.Shared == 1
}

// GetUsagePercent returns the storage usage as a percentage
func (s *Storage) GetUsagePercent() float64 {
	if s.MaxDisk == 0 {
		return 0
	}
	return (float64(s.Disk) / float64(s.MaxDisk)) * 100
}

// GetUsageGB returns used space in GB
func (s *Storage) GetUsageGB() float64 {
	return float64(s.Disk) / 1024 / 1024 / 1024
}

// GetTotalGB returns total space in GB
func (s *Storage) GetTotalGB() float64 {
	return float64(s.MaxDisk) / 1024 / 1024 / 1024
}

// StorageManager handles storage aggregation and deduplication
type StorageManager struct {
	// AllStorages contains all storage entries (including duplicates for shared storage)
	AllStorages []*Storage

	// UniqueStorages contains deduplicated storage entries
	// For shared storage, only one entry is kept
	// For local storage, all entries are kept since they're unique per node
	UniqueStorages []*Storage

	// SharedStorages contains only shared storage entries (deduplicated)
	SharedStorages []*Storage

	// LocalStorages contains only local storage entries (per node)
	LocalStorages []*Storage
}

// NewStorageManager creates a new storage manager
func NewStorageManager() *StorageManager {
	return &StorageManager{
		AllStorages:    make([]*Storage, 0),
		UniqueStorages: make([]*Storage, 0),
		SharedStorages: make([]*Storage, 0),
		LocalStorages:  make([]*Storage, 0),
	}
}

// AddStorage adds a storage entry and handles deduplication
func (sm *StorageManager) AddStorage(storage *Storage) {
	sm.AllStorages = append(sm.AllStorages, storage)

	if storage.IsShared() {
		// For shared storage, only add if we haven't seen this storage name before
		found := false
		for _, existing := range sm.SharedStorages {
			if existing.Name == storage.Name {
				found = true
				break
			}
		}
		if !found {
			sm.SharedStorages = append(sm.SharedStorages, storage)
			sm.UniqueStorages = append(sm.UniqueStorages, storage)
		}
	} else {
		// For local storage, always add since each node has unique storage
		sm.LocalStorages = append(sm.LocalStorages, storage)
		sm.UniqueStorages = append(sm.UniqueStorages, storage)
	}
}

// GetTotalUsage returns total used space across all unique storages
func (sm *StorageManager) GetTotalUsage() int64 {
	var total int64
	for _, storage := range sm.UniqueStorages {
		total += storage.Disk
	}
	return total
}

// GetTotalCapacity returns total capacity across all unique storages
func (sm *StorageManager) GetTotalCapacity() int64 {
	var total int64
	for _, storage := range sm.UniqueStorages {
		total += storage.MaxDisk
	}
	return total
}
