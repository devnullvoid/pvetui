package api

import (
	"fmt"
	"time"
)

// Snapshot represents a Proxmox VM or container snapshot.
type Snapshot struct {
	Name        string    `json:"name"`        // Snapshot name
	Description string    `json:"description"` // Snapshot description
	SnapTime    time.Time `json:"snaptime"`    // Creation timestamp (from API)
	Parent      string    `json:"parent"`      // Parent snapshot name (if any)
	VMState     bool      `json:"vmstate"`     // Whether VM state is included (QEMU only)
}

// SnapshotOptions contains options for creating snapshots.
type SnapshotOptions struct {
	// Description for the snapshot
	Description string `json:"description,omitempty"`
	// Whether to include VM state (memory dump)
	VMState bool `json:"vmstate,omitempty"`
	// Whether to include configuration
	Config bool `json:"config,omitempty"`
	// Whether to include disk state
	Disk bool `json:"disk,omitempty"`
}

// GetSnapshots retrieves all snapshots for a VM or container.
func (c *Client) GetSnapshots(vm *VM) ([]Snapshot, error) {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot", vm.Node, vm.Type, vm.ID)

	var result map[string]interface{}
	if err := c.Get(path, &result); err != nil {
		return nil, fmt.Errorf("failed to get snapshots: %w", err)
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid snapshot response format")
	}

	var snapshots []Snapshot
	for _, item := range data {
		if snapshotData, ok := item.(map[string]interface{}); ok {
			name := getString(snapshotData, "name")

			snapshot := Snapshot{
				Name:        name,
				Description: getString(snapshotData, "description"),
				Parent:      getString(snapshotData, "parent"),
				VMState:     getBool(snapshotData, "vmstate"),
			}

			// Parse snaptime
			if snaptime, ok := snapshotData["snaptime"].(float64); ok {
				snapshot.SnapTime = time.Unix(int64(snaptime), 0)
			}

			snapshots = append(snapshots, snapshot)
		}
	}

	return snapshots, nil
}

// CreateSnapshot creates a new snapshot for a VM or container.
func (c *Client) CreateSnapshot(vm *VM, name string, options *SnapshotOptions) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot", vm.Node, vm.Type, vm.ID)

	data := map[string]interface{}{
		"snapname": name,
	}

	if options != nil {
		if options.Description != "" {
			data["description"] = options.Description
		}
		if options.VMState && vm.Type == VMTypeQemu {
			data["vmstate"] = "1"
		}
		if options.Config {
			data["config"] = "1"
		}
		if options.Disk {
			data["disk"] = "1"
		}
	}

	c.logger.Info("Creating snapshot '%s' for %s %s (ID: %d)", name, vm.Type, vm.Name, vm.ID)

	return c.Post(path, data)
}

// DeleteSnapshot deletes a snapshot from a VM or container.
func (c *Client) DeleteSnapshot(vm *VM, snapshotName string) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s", vm.Node, vm.Type, vm.ID, snapshotName)

	c.logger.Info("Deleting snapshot '%s' from %s %s (ID: %d)", snapshotName, vm.Type, vm.Name, vm.ID)

	return c.Delete(path)
}

// RollbackToSnapshot rolls back a VM or container to a specific snapshot.
func (c *Client) RollbackToSnapshot(vm *VM, snapshotName string) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s/rollback", vm.Node, vm.Type, vm.ID, snapshotName)

	c.logger.Info("Rolling back %s %s (ID: %d) to snapshot '%s'", vm.Type, vm.Name, vm.ID, snapshotName)

	return c.Post(path, nil)
}
