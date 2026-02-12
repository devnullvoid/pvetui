package api

import (
	"fmt"
	"strings"
)

// StartVM starts a VM or container.
func (c *Client) StartVM(vm *VM) (string, error) {
	path := fmt.Sprintf("/nodes/%s/%s/%d/status/start", vm.Node, vm.Type, vm.ID)

	var response map[string]interface{}
	if err := c.PostWithResponse(path, nil, &response); err != nil {
		return "", err
	}

	return c.extractUPID(response)
}

// StopVM stops a VM or container.
func (c *Client) StopVM(vm *VM) (string, error) {
	path := fmt.Sprintf("/nodes/%s/%s/%d/status/stop", vm.Node, vm.Type, vm.ID)

	var response map[string]interface{}
	if err := c.PostWithResponse(path, nil, &response); err != nil {
		return "", err
	}

	return c.extractUPID(response)
}

// ShutdownVM requests a graceful shutdown via the guest OS.
// For both QEMU and LXC, Proxmox exposes `/status/shutdown`.
// The guest tools/agent should be installed for reliable behavior.
func (c *Client) ShutdownVM(vm *VM) (string, error) {
	path := fmt.Sprintf("/nodes/%s/%s/%d/status/shutdown", vm.Node, vm.Type, vm.ID)

	var response map[string]interface{}
	if err := c.PostWithResponse(path, nil, &response); err != nil {
		return "", err
	}

	return c.extractUPID(response)
}

// RestartVM restarts a VM or container
//
// Both QEMU VMs and LXC containers use the `/status/reboot` endpoint
// according to the official Proxmox VE API documentation.
//
// Parameters:
//   - vm: The VM or container to restart
//
// Returns the task UPID and an error if the restart operation fails.
func (c *Client) RestartVM(vm *VM) (string, error) {
	path := fmt.Sprintf("/nodes/%s/%s/%d/status/reboot", vm.Node, vm.Type, vm.ID)
	c.logger.Info("Rebooting %s %s (ID: %d) using /status/reboot endpoint", vm.Type, vm.Name, vm.ID)

	var response map[string]interface{}
	if err := c.PostWithResponse(path, nil, &response); err != nil {
		return "", err
	}

	return c.extractUPID(response)
}

// ResetVM performs a hard reset (like pressing the reset button).
// Only supported for QEMU VMs. Not applicable to LXC.
func (c *Client) ResetVM(vm *VM) (string, error) {
	if vm.Type != VMTypeQemu {
		return "", fmt.Errorf("reset is only supported for QEMU VMs")
	}

	path := fmt.Sprintf("/nodes/%s/%s/%d/status/reset", vm.Node, vm.Type, vm.ID)

	var response map[string]interface{}
	if err := c.PostWithResponse(path, nil, &response); err != nil {
		return "", err
	}

	return c.extractUPID(response)
}

// extractUPID extracts the UPID from a Proxmox API response.
func (c *Client) extractUPID(response map[string]interface{}) (string, error) {
	if dataField, ok := response["data"]; ok {
		if upidStr, ok := dataField.(string); ok && strings.HasPrefix(upidStr, "UPID:") {
			return upidStr, nil
		}
	}
	// Some operations might not return a UPID (sync ops), or return it differently
	// But most async ops return UPID. If missing, return empty string but no error if the call was successful.
	return "", nil
}

// MigrationOptions contains configuration options for migrating a VM or container.
//
// This struct provides comprehensive control over the migration process,
// supporting both QEMU VMs and LXC containers with their specific requirements.
type MigrationOptions struct {
	// Target specifies the destination node name for the migration.
	// This field is required and must be a valid, online node in the cluster
	// that is different from the source node.
	Target string `json:"target"`

	// Online controls whether to perform online (live) migration for QEMU VMs.
	// If nil, defaults to true for running QEMU VMs and false for stopped ones.
	// Online migration allows the QEMU VM to continue running during migration,
	// while offline migration requires stopping the VM first.
	// NOTE: This parameter is ignored for LXC containers as they don't support live migration.
	Online *bool `json:"online,omitempty"`

	// Force enables forced migration, bypassing locks and some safety checks.
	// Use with caution as this can potentially cause data corruption if the
	// VM/container is in an inconsistent state.
	Force bool `json:"force,omitempty"`

	// MigrationNetwork specifies the network interface to use for migration traffic.
	// This is useful in clusters with multiple networks to control which network
	// carries the migration data. If empty, uses the default migration network.
	MigrationNetwork string `json:"migration_network,omitempty"`

	// MigrationType specifies the migration method for LXC containers.
	// NOTE: This parameter is not supported by the current Proxmox API for LXC containers.
	// LXC migration is always a "restart" style operation by default.
	// This field is kept for potential future compatibility but is not sent to the API.
	// This option is ignored for QEMU VMs.
	MigrationType string `json:"migration_type,omitempty"`

	// BandwidthLimit sets the maximum bandwidth for migration in KB/s.
	// This helps control network usage during migration to avoid impacting
	// other services. A value of 0 means no limit.
	BandwidthLimit int `json:"bwlimit,omitempty"`

	// TargetStorage specifies the target storage for offline migrations.
	// This allows migrating VM disks to different storage on the target node.
	// Only applicable for offline migrations.
	TargetStorage string `json:"targetstorage,omitempty"`

	// Delete controls whether to remove the VM/container from the source node
	// after successful migration. When false (default), the VM/container
	// configuration remains on the source node but in a stopped state.
	Delete bool `json:"delete,omitempty"`
}

// MigrateVM migrates a VM or container to another node using the Proxmox API.
//
// The migration process supports both QEMU VMs and LXC containers with different
// options and behaviors:
//
// For QEMU VMs:
//   - Online migration (live migration) is supported for running VMs
//   - Offline migration requires the VM to be stopped first
//   - Supports bandwidth limiting and migration network specification
//
// For LXC containers:
//   - Migration type can be "secure" (default) or "insecure"
//   - Online migration is supported for running containers
//   - Supports bandwidth limiting
//
// The function performs validation to ensure:
//   - Target node is specified and exists in the cluster
//   - Target node is different from the source node
//   - Target node is online and available
//
// Migration is an asynchronous operation. The function returns a task UPID
// that can be monitored via the WaitForTaskCompletion method or the cluster tasks API.
//
// Example usage:
//
//	options := &api.MigrationOptions{
//		Target: "node2",
//		Online: &[]bool{true}[0], // Enable online migration
//		BandwidthLimit: 1000,     // Limit to 1000 KB/s
//	}
//	upid, err := client.MigrateVM(vm, options)
//	if err != nil {
//		return err
//	}
//	// Wait for migration to complete (migrations can take several minutes)
//	err = client.WaitForTaskCompletion(upid, "VM migration", 10*time.Minute)
//
// Parameters:
//   - vm: The VM or container to migrate
//   - options: Migration configuration options
//
// Returns the task UPID and an error if the migration cannot be initiated.
func (c *Client) MigrateVM(vm *VM, options *MigrationOptions) (string, error) {
	if options == nil || options.Target == "" {
		return "", fmt.Errorf("target node is required for migration")
	}

	// Validate target node exists
	if c.Cluster != nil {
		targetExists := false

		for _, node := range c.Cluster.Nodes {
			if node != nil && node.Name == options.Target {
				targetExists = true

				break
			}
		}

		if !targetExists {
			return "", fmt.Errorf("target node '%s' not found in cluster", options.Target)
		}
	}

	path := fmt.Sprintf("/nodes/%s/%s/%d/migrate", vm.Node, vm.Type, vm.ID)

	// Build migration data
	data := map[string]interface{}{
		"target": options.Target,
	}

	// Set migration parameters based on VM type
	if vm.Type == VMTypeQemu {
		// QEMU VMs use online parameter for live/offline migration
		if options.Online != nil {
			if *options.Online {
				data["online"] = "1"
			} else {
				data["online"] = "0"
			}
		} else {
			// Default: online migration for running VMs, offline for stopped VMs
			if vm.Status == VMStatusRunning {
				data["online"] = "1"
			} else {
				data["online"] = "0"
			}
		}
	} else if vm.Type == VMTypeLXC {
		// LXC containers use restart parameter (they don't support live migration)
		data["restart"] = "1"
	}

	// Add optional parameters
	if options.Force {
		data["force"] = "1"
	}

	if options.MigrationNetwork != "" {
		data["migration_network"] = options.MigrationNetwork
	}

	if options.BandwidthLimit > 0 {
		data["bwlimit"] = options.BandwidthLimit
	}

	if options.TargetStorage != "" {
		data["targetstorage"] = options.TargetStorage
	}

	if options.Delete {
		data["delete"] = "1"
	}

	// Note: LXC containers don't support migration_type parameter
	// LXC migration is always a "restart" style operation by default

	c.logger.Info("Migrating %s %s (ID: %d) from %s to %s", vm.Type, vm.Name, vm.ID, vm.Node, options.Target)
	c.logger.Debug("Migration parameters: %+v", data)

	// Use PostWithResponse to get the actual response for debugging
	var response map[string]interface{}
	if err := c.PostWithResponse(path, data, &response); err != nil {
		c.logger.Error("Migration API call failed: %v", err)

		return "", err
	}

	c.logger.Info("Migration API response: %+v", response)

	return c.extractUPID(response)
}

// DeleteVM permanently deletes a VM or container
// WARNING: This operation is irreversible and will destroy all VM data including disks.
func (c *Client) DeleteVM(vm *VM) (string, error) {
	return c.DeleteVMWithOptions(vm, nil)
}

// DeleteVMOptions contains options for deleting a VM.
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
// WARNING: This operation is irreversible and will destroy all VM data including disks.
func (c *Client) DeleteVMWithOptions(vm *VM, options *DeleteVMOptions) (string, error) {
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

	var response map[string]interface{}
	if err := c.DeleteWithResponse(path, &response); err != nil {
		return "", err
	}

	return c.extractUPID(response)
}
