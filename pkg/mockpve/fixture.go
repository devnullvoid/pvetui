package mockpve

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// MockFixture describes optional mock API state loaded from a YAML fixture.
type MockFixture struct {
	Replace        bool                 `yaml:"replace"`
	NextID         int                  `yaml:"next_id"`
	TaskDelayMS    int                  `yaml:"task_delay_ms"`
	Nodes          []*MockNode          `yaml:"nodes"`
	Storage        []*MockStorage       `yaml:"storage"`
	VMs            []*MockVM            `yaml:"vms"`
	StorageContent []*MockStorageVolume `yaml:"storage_content"`
	Backups        []*MockBackup        `yaml:"backups"`
	Tasks          []*MockTask          `yaml:"tasks"`
}

// LoadFixture reads a YAML fixture from path.
func LoadFixture(path string) (*MockFixture, error) {
	// #nosec G304 -- fixture paths are an explicit CLI input for local mock data.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}

	var fixture MockFixture
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return nil, fmt.Errorf("parse fixture: %w", err)
	}

	return &fixture, nil
}

// ApplyFixture overlays fixture data onto the mock state.
func (s *MockState) ApplyFixture(fixture *MockFixture) error {
	if fixture == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if fixture.Replace {
		s.Nodes = nil
		s.VMs = make(map[string]*MockVM)
		s.Storage = nil
		s.StorageContent = make(map[string]map[string]*MockStorageVolume)
		s.Backups = make(map[string]*MockBackup)
		s.Tasks = make(map[string]*MockTask)
		s.NextID = 102
	}

	if fixture.NextID > 0 {
		s.NextID = fixture.NextID
	}
	if fixture.TaskDelayMS > 0 {
		s.taskDelay = time.Duration(fixture.TaskDelayMS) * time.Millisecond
	}

	s.Nodes = append(s.Nodes, fixture.Nodes...)
	s.Storage = append(s.Storage, fixture.Storage...)

	for _, vm := range fixture.VMs {
		if vm == nil {
			continue
		}
		normalizeFixtureVM(vm)
		s.VMs[strconv.Itoa(vm.ID)] = vm
		if vm.ID >= s.NextID {
			s.NextID = vm.ID + 1
		}
	}

	for _, volume := range fixture.StorageContent {
		if volume != nil {
			s.addStorageVolumeLocked(volume)
		}
	}

	for _, backup := range fixture.Backups {
		if backup != nil {
			s.Backups[backup.VolID] = backup
			s.addStorageVolumeLocked(&MockStorageVolume{
				VolID:     backup.VolID,
				Node:      backupNode(backup, s.Storage),
				Storage:   backup.Storage,
				Content:   backup.Content,
				Format:    backup.Format,
				Size:      backup.Size,
				Used:      backup.Size,
				VMID:      backup.VMID,
				CreatedAt: backup.Date,
				Notes:     backup.Notes,
			})
		}
	}

	for _, task := range fixture.Tasks {
		if task == nil {
			continue
		}
		normalizeFixtureTask(task)
		s.Tasks[task.UPID] = task
	}

	s.recalculateStorageUsageLocked()
	return nil
}

func normalizeFixtureVM(vm *MockVM) {
	if vm.Config == nil {
		vm.Config = make(map[string]interface{})
	}
	if vm.Type == guestTypeLXC {
		if _, ok := vm.Config["hostname"]; !ok && vm.Name != "" {
			vm.Config["hostname"] = vm.Name
		}
	} else if _, ok := vm.Config["name"]; !ok && vm.Name != "" {
		vm.Config["name"] = vm.Name
	}
}

func normalizeFixtureTask(task *MockTask) {
	if task.Node == "" {
		task.Node = "pve"
	}
	if task.User == "" {
		task.User = "root@pam"
	}
	if task.StartTime == 0 {
		task.StartTime = time.Now().Add(-10 * time.Minute).Unix()
	}
	if task.Status == "" {
		if task.EndTime > 0 {
			task.Status = taskStatusStopped
		} else {
			task.Status = taskStatusRunning
		}
	}
	if task.EndTime > 0 && task.ExitStatus == "" {
		task.ExitStatus = "OK"
	}
	if task.UPID == "" {
		task.UPID = fmt.Sprintf("UPID:%s:%X:00000000:00000000:%s:%s:%s:", task.Node, task.StartTime, task.Type, task.ID, task.User)
	}
}

func backupNode(backup *MockBackup, storage []*MockStorage) string {
	for _, entry := range storage {
		if entry != nil && entry.ID == backup.Storage {
			return entry.Node
		}
	}
	return "pve"
}
