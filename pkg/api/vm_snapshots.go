package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

	// Use direct HTTP client to get raw response
	var result map[string]interface{}
	err := c.httpClient.Post(context.Background(), path, data, &result)
	if err != nil {
		c.logger.Debug("CreateSnapshot HTTP error: %v", err)
		// Check if the error contains the response body
		if strings.Contains(err.Error(), "failed to parse response JSON") {
			// Extract the response body from the error message
			// The error format is: "failed to parse response JSON: <response body>"
			parts := strings.SplitN(err.Error(), "failed to parse response JSON: ", 2)
			if len(parts) == 2 {
				responseBody := parts[1]
				c.logger.Debug("CreateSnapshot response body: %s", responseBody)
				// Check if the response contains an error message
				if strings.Contains(responseBody, "snapshot feature is not available") ||
					strings.Contains(responseBody, "error") ||
					strings.Contains(responseBody, "failed") {
					errorMsg := fmt.Sprintf("snapshot creation failed: %s", strings.TrimSpace(responseBody))
					c.logger.Debug("CreateSnapshot returning error: %s", errorMsg)
					return errors.New(errorMsg)
				}
			}
		}
		c.logger.Debug("CreateSnapshot returning original error: %v", err)
		return err
	}

	c.logger.Debug("CreateSnapshot successful HTTP response, checking result: %+v", result)

	// Check for API-level errors in the response
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		c.logger.Debug("CreateSnapshot found error in result: %s", errMsg)
		return fmt.Errorf("snapshot creation failed: %s", errMsg)
	}

	// Check if the response contains a UPID (task ID) - this means the operation was queued
	if upid, ok := result["data"].(string); ok && strings.HasPrefix(upid, "UPID:") {
		c.logger.Debug("CreateSnapshot task queued with UPID: %s", upid)
		// Poll for task completion
		return c.waitForTaskCompletion(upid, "snapshot creation")
	}

	// Check if the response contains error messages in the data field
	if data, ok := result["data"].(string); ok {
		c.logger.Debug("CreateSnapshot response data: %s", data)
		if strings.Contains(data, "snapshot feature is not available") ||
			strings.Contains(data, "error") ||
			strings.Contains(data, "failed") {
			errorMsg := fmt.Sprintf("snapshot creation failed: %s", strings.TrimSpace(data))
			c.logger.Debug("CreateSnapshot returning error from data: %s", errorMsg)
			return errors.New(errorMsg)
		}
	}

	c.logger.Debug("CreateSnapshot operation completed successfully")
	return nil
}

// WaitForTaskCompletion polls for task completion and returns an error if the task failed.
// This is a public wrapper that allows specifying a custom timeout.
//
// Proxmox task completion is determined by checking the EndTime field:
//   - If EndTime > 0, the task has finished
//   - If Status == "OK", the task succeeded
//   - If Status != "OK", the task failed and Status contains the error message
//
// Parameters:
//   - upid: The Proxmox task UPID to monitor
//   - operationName: A human-readable name for the operation (used in error messages)
//   - maxWait: Maximum time to wait for the task to complete
//
// Returns an error if the task fails or times out.
func (c *Client) WaitForTaskCompletion(upid string, operationName string, maxWait time.Duration) error {
	c.logger.Debug("Waiting for task completion: %s (timeout: %v)", upid, maxWait)

	pollInterval := 2 * time.Second
	start := time.Now()

	for time.Since(start) < maxWait {
		tasks, err := c.GetClusterTasks()
		if err != nil {
			c.logger.Debug("Failed to get cluster tasks: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		// Find our task
		for _, task := range tasks {
			if task.UPID == upid {
				c.logger.Debug("Found task %s, status: %q, endtime: %d", upid, task.Status, task.EndTime)

				// Task is complete when EndTime > 0
				if task.EndTime > 0 {
					// Check if task succeeded
					if task.Status == "OK" {
						c.logger.Debug("Task %s completed successfully", upid)
						return nil
					}
					// Task completed but failed - Status contains error message
					errorMsg := task.Status
					if errorMsg == "" {
						errorMsg = "unknown error (empty status)"
					}
					c.logger.Debug("Task %s failed with status: %s", upid, errorMsg)
					return fmt.Errorf("%s failed: %s", operationName, errorMsg)
				}
				// Task is still running (EndTime == 0), continue polling
				break
			}
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("%s timed out after %v waiting for task %s", operationName, maxWait, upid)
}

// waitForTaskCompletion is a private wrapper for backward compatibility with snapshots.
func (c *Client) waitForTaskCompletion(upid string, operationName string) error {
	// Snapshots typically complete within 2 minutes
	return c.WaitForTaskCompletion(upid, operationName, 2*time.Minute)
}

// DeleteSnapshot deletes a snapshot from a VM or container.
func (c *Client) DeleteSnapshot(vm *VM, snapshotName string) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s", vm.Node, vm.Type, vm.ID, snapshotName)

	c.logger.Info("Deleting snapshot '%s' from %s %s (ID: %d)", snapshotName, vm.Type, vm.Name, vm.ID)

	var result map[string]interface{}
	err := c.httpClient.Delete(context.Background(), path, &result)
	if err != nil {
		// Check if the error contains the response body
		if strings.Contains(err.Error(), "failed to parse response JSON") {
			// Extract the response body from the error message
			parts := strings.SplitN(err.Error(), "failed to parse response JSON: ", 2)
			if len(parts) == 2 {
				responseBody := parts[1]
				// Check if the response contains an error message
				if strings.Contains(responseBody, "error") ||
					strings.Contains(responseBody, "failed") {
					return fmt.Errorf("snapshot deletion failed: %s", strings.TrimSpace(responseBody))
				}
			}
		}
		return err
	}

	// Check for API-level errors in the response
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("snapshot deletion failed: %s", errMsg)
	}

	// Check if the response contains a UPID (task ID) - this means the operation was queued
	if upid, ok := result["data"].(string); ok && strings.HasPrefix(upid, "UPID:") {
		c.logger.Debug("DeleteSnapshot task queued with UPID: %s", upid)
		// Poll for task completion
		return c.waitForTaskCompletion(upid, "snapshot deletion")
	}

	// Check if the response contains error messages in the data field
	if data, ok := result["data"].(string); ok {
		c.logger.Debug("DeleteSnapshot response data: %s", data)
		if strings.Contains(data, "error") ||
			strings.Contains(data, "failed") {
			errorMsg := fmt.Sprintf("snapshot deletion failed: %s", strings.TrimSpace(data))
			c.logger.Debug("DeleteSnapshot returning error from data: %s", errorMsg)
			return errors.New(errorMsg)
		}
	}

	c.logger.Debug("DeleteSnapshot operation completed successfully")
	return nil
}

// RollbackToSnapshot rolls back a VM or container to a specific snapshot.
func (c *Client) RollbackToSnapshot(vm *VM, snapshotName string) error {
	path := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s/rollback", vm.Node, vm.Type, vm.ID, snapshotName)

	c.logger.Info("Rolling back %s %s (ID: %d) to snapshot '%s'", vm.Type, vm.Name, vm.ID, snapshotName)

	var result map[string]interface{}
	err := c.httpClient.Post(context.Background(), path, nil, &result)
	if err != nil {
		// Check if the error contains the response body
		if strings.Contains(err.Error(), "failed to parse response JSON") {
			// Extract the response body from the error message
			parts := strings.SplitN(err.Error(), "failed to parse response JSON: ", 2)
			if len(parts) == 2 {
				responseBody := parts[1]
				// Check if the response contains an error message
				if strings.Contains(responseBody, "error") ||
					strings.Contains(responseBody, "failed") {
					return fmt.Errorf("snapshot rollback failed: %s", strings.TrimSpace(responseBody))
				}
			}
		}
		return err
	}

	// Check for API-level errors in the response
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("snapshot rollback failed: %s", errMsg)
	}

	// Check if the response contains a UPID (task ID) - this means the operation was queued
	if upid, ok := result["data"].(string); ok && strings.HasPrefix(upid, "UPID:") {
		c.logger.Debug("RollbackToSnapshot task queued with UPID: %s", upid)
		// Poll for task completion
		return c.waitForTaskCompletion(upid, "snapshot rollback")
	}

	// Check if the response contains error messages in the data field
	if data, ok := result["data"].(string); ok {
		c.logger.Debug("RollbackToSnapshot response data: %s", data)
		if strings.Contains(data, "error") ||
			strings.Contains(data, "failed") {
			errorMsg := fmt.Sprintf("snapshot rollback failed: %s", strings.TrimSpace(data))
			c.logger.Debug("RollbackToSnapshot returning error from data: %s", errorMsg)
			return errors.New(errorMsg)
		}
	}

	c.logger.Debug("RollbackToSnapshot operation completed successfully")
	return nil
}
