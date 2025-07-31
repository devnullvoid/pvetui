package api

import (
	"context"
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
	// Whether to include VM state (memory dump) - QEMU only
	VMState bool `json:"vmstate,omitempty"`
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
	}

	c.logger.Info("Creating snapshot '%s' for %s %s (ID: %d)", name, vm.Type, vm.Name, vm.ID)

	var result map[string]interface{}
	if err := c.PostWithResponse(path, data, &result); err != nil {
		return err
	}

	// Check for API-level errors in the response
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("snapshot creation failed: %s", errMsg)
	}

	return nil
}

// DeleteSnapshot deletes a snapshot from a VM or container.
func (c *Client) DeleteSnapshot(vm *VM, snapshotName string) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s", vm.Node, vm.Type, vm.ID, snapshotName)

	c.logger.Info("Deleting snapshot '%s' from %s %s (ID: %d)", snapshotName, vm.Type, vm.Name, vm.ID)

	var result map[string]interface{}
	if err := c.httpClient.Delete(context.Background(), path, &result); err != nil {
		return err
	}

	// Check for API-level errors in the response
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("snapshot deletion failed: %s", errMsg)
	}

	return nil
}

// RollbackToSnapshot rolls back a VM or container to a specific snapshot.
func (c *Client) RollbackToSnapshot(vm *VM, snapshotName string) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s/rollback", vm.Node, vm.Type, vm.ID, snapshotName)

	c.logger.Info("Rolling back %s %s (ID: %d) to snapshot '%s'", vm.Type, vm.Name, vm.ID, snapshotName)

	var result map[string]interface{}
	if err := c.PostWithResponse(path, nil, &result); err != nil {
		return err
	}

	// Check for API-level errors in the response
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("snapshot rollback failed: %s", errMsg)
	}

	return nil
}
