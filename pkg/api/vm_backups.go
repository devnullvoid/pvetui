package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Backup represents a Proxmox backup (vzdump) file.
type Backup struct {
	VolID        string    `json:"volid"`          // Volume ID (e.g., "local:backup/vzdump-qemu-100-....vma.zst")
	Name         string    `json:"name,omitempty"` // Derived name
	Date         time.Time `json:"date"`           // Backup timestamp
	Size         int64     `json:"size"`           // Size in bytes
	Format       string    `json:"format"`         // Format (vma, tar, etc.)
	Notes        string    `json:"notes"`          // Notes/Description
	VMID         int       `json:"vmid"`           // VM ID
	Storage      string    `json:"storage"`        // Storage name
	Content      string    `json:"content"`        // Content type (backup)
	Verification string    `json:"verification"`   // Verification status
}

// BackupOptions contains options for creating backups.
type BackupOptions struct {
	Mode         string // "snapshot", "suspend", "stop"
	Compression  string // "zstd", "gzip", "lzo"
	Notes        string // Description
	Storage      string // Target storage
	Remove       bool   // Remove old backups (prune)
	Notification string // "auto", "always", "never"
}

// GetBackups retrieves all backups for a VM across all available storages.
func (c *Client) GetBackups(vm *VM) ([]Backup, error) {
	c.logger.Debug("GetBackups: Starting backup retrieval for VM %d on node %s", vm.ID, vm.Node)

	// 1. Get storages on the node
	storages, err := c.GetNodeStorages(vm.Node)
	if err != nil {
		return nil, fmt.Errorf("failed to get node storages: %w", err)
	}

	c.logger.Debug("GetBackups: Found %d storages on node %s", len(storages), vm.Node)

	var allBackups []Backup
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 2. Iterate over storages in parallel
	for _, storage := range storages {
		// Check if storage supports backups
		// Storage content is a comma-separated string, e.g. "iso,backup"
		if !strings.Contains(storage.Content, "backup") {
			continue
		}

		wg.Add(1)
		go func(s *Storage) {
			defer wg.Done()

			c.logger.Debug("GetBackups: Checking storage %s for backups", s.Name)

			// 3. List content of type "backup"
			path := fmt.Sprintf("/nodes/%s/storage/%s/content", vm.Node, s.Name)

			// We need to pass content=backup as query param.
			fullPath := fmt.Sprintf("%s?content=backup", path)

			var result map[string]interface{}
			// Use GetWithCache to improve performance, especially for slow network storages
			// Backups don't change instantly without user action, and we clear cache on operations
			if err := c.GetWithCache(fullPath, &result, NodeDataTTL); err != nil {
				c.logger.Debug("Failed to list backups on storage %s: %v", s.Name, err)
				return
			}

			data, ok := result["data"].([]interface{})
			if !ok {
				c.logger.Debug("GetBackups: No data in response for storage %s", s.Name)
				return
			}

			c.logger.Debug("GetBackups: Found %d items on storage %s", len(data), s.Name)

			var storageBackups []Backup

			for _, item := range data {
				if backupData, ok := item.(map[string]interface{}); ok {
					// Filter by VMID
					itemVMID := 0
					if v, ok := backupData["vmid"].(float64); ok {
						itemVMID = int(v)
					} else if v, ok := backupData["vmid"].(string); ok {
						itemVMID, _ = strconv.Atoi(v)
					}

					// If VMID matches, add to list
					if itemVMID == vm.ID {
						volID := getString(backupData, "volid")
						c.logger.Debug("GetBackups: Found matching backup %s", volID)

						backup := Backup{
							VolID:        volID,
							Size:         int64(getFloat(backupData, "size")),
							Notes:        getString(backupData, "notes"),
							VMID:         itemVMID,
							Storage:      s.Name,
							Format:       getString(backupData, "format"),
							Content:      getString(backupData, "content"),
							Verification: getString(backupData, "verification"),
						}

						// Parse ctime (creation time)
						if ctime, ok := backupData["ctime"].(float64); ok {
							backup.Date = time.Unix(int64(ctime), 0)
						}

						// Derive a friendly name from VolID if needed
						parts := strings.Split(volID, "/")
						if len(parts) > 0 {
							backup.Name = parts[len(parts)-1]
						} else {
							backup.Name = volID
						}

						storageBackups = append(storageBackups, backup)
					}
				}
			}

			mu.Lock()
			allBackups = append(allBackups, storageBackups...)
			mu.Unlock()
		}(storage)
	}

	wg.Wait()

	c.logger.Debug("GetBackups: Total matching backups found: %d", len(allBackups))
	return allBackups, nil
}

// CreateBackup creates a new backup for the VM.
// It returns the UPID of the backup task.
func (c *Client) CreateBackup(vm *VM, options BackupOptions) (string, error) {
	path := fmt.Sprintf("/nodes/%s/vzdump", vm.Node)

	data := map[string]interface{}{
		"vmid": fmt.Sprintf("%d", vm.ID),
		"mode": options.Mode,
	}

	if options.Storage != "" {
		data["storage"] = options.Storage
	} else {
		return "", fmt.Errorf("target storage is required")
	}

	if options.Compression != "" {
		data["compress"] = options.Compression
	}

	if options.Notes != "" {
		// PVE uses 'notes-template' usually? No, 'notes-template' is for template string.
		// Wait, API says `notes-template`. But maybe we just want to set notes after?
		// Actually, vzdump usually doesn't take 'notes' directly as a simple string for the specific backup,
		// unless we use `notes-template` without variables?
		// Let's check API.
		// API: `notes-template`. "Template string for generating notes...".
		// There is no `notes` parameter for `vzdump` directly in the spec I saw earlier?
		// Let's assume we can't set arbitrary notes easily during creation via API parameter `notes`.
		// But we can use `notes-template` as the note if it doesn't contain variables.
		data["notes-template"] = options.Notes
	}

	if options.Remove {
		data["remove"] = "1"
	}

	// notification-mode?
	if options.Notification != "" {
		data["notification-mode"] = options.Notification
	}

	c.logger.Info("Starting backup for %s %s (ID: %d) to storage %s", vm.Type, vm.Name, vm.ID, options.Storage)

	var result map[string]interface{}
	err := c.httpClient.Post(context.Background(), path, data, &result)
	if err != nil {
		return "", fmt.Errorf("backup request failed: %w", err)
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("backup failed: %s", errMsg)
	}

	// Get UPID
	if upid, ok := result["data"].(string); ok && strings.HasPrefix(upid, "UPID:") {
		return upid, nil
	}

	return "", fmt.Errorf("failed to get backup task ID")
}

// DeleteBackup deletes a backup volume.
func (c *Client) DeleteBackup(vm *VM, volID string) error {
	// volID format: "storage:backup/filename"
	// We need to call DELETE /nodes/{node}/storage/{storage}/content/{volume}

	// Parse storage and volume from volID
	parts := strings.SplitN(volID, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid volume ID format: %s", volID)
	}

	storageName := parts[0]
	volumeName := parts[1] // "backup/filename"

	// API expects the volume param to be the full volid usually, or just the volume part?
	// DELETE /nodes/{node}/storage/{storage}/content/{volume}
	// "volume" parameter: "Volume identifier."

	path := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", vm.Node, storageName, volumeName)

	c.logger.Info("Deleting backup '%s' for %s %s", volID, vm.Type, vm.Name)

	var result map[string]interface{}
	err := c.httpClient.Delete(context.Background(), path, &result)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}

	// Check result
	if upid, ok := result["data"].(string); ok && strings.HasPrefix(upid, "UPID:") {
		// It returns a UPID.
		return c.waitForTaskCompletion(upid, "backup deletion")
	}

	return nil
}

// RestoreBackup restores a backup to the VM.
// If the VM exists, it will be overwritten.
func (c *Client) RestoreBackup(vm *VM, volID string) (string, error) {
	var path string
	var data map[string]interface{}

	if vm.Type == VMTypeQemu {
		path = fmt.Sprintf("/nodes/%s/qemu", vm.Node)
		data = map[string]interface{}{
			"vmid":    fmt.Sprintf("%d", vm.ID),
			"archive": volID,
			"force":   "1", // Overwrite existing VM
		}
	} else if vm.Type == VMTypeLXC {
		path = fmt.Sprintf("/nodes/%s/lxc", vm.Node)
		data = map[string]interface{}{
			"vmid":       fmt.Sprintf("%d", vm.ID),
			"ostemplate": volID,
			"restore":    "1",
			"force":      "1", // Overwrite existing CT
		}
	} else {
		return "", fmt.Errorf("unsupported VM type: %s", vm.Type)
	}

	c.logger.Info("Restoring backup '%s' to %s %s (ID: %d)", volID, vm.Type, vm.Name, vm.ID)

	var result map[string]interface{}
	err := c.httpClient.Post(context.Background(), path, data, &result)
	if err != nil {
		return "", fmt.Errorf("restore request failed: %w", err)
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("restore failed: %s", errMsg)
	}

	// Get UPID
	if upid, ok := result["data"].(string); ok && strings.HasPrefix(upid, "UPID:") {
		return upid, nil
	}

	return "", fmt.Errorf("failed to get restore task ID")
}
