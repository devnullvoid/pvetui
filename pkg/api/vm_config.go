package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// VMConfig represents editable configuration for both QEMU and LXC guests.
type VMConfig struct {
	// Common fields (match Proxmox API)
	Name        string `json:"name,omitempty"`     // VM name (QEMU) or hostname (LXC)
	Hostname    string `json:"hostname,omitempty"` // LXC hostname (alternative to name)
	Cores       int    `json:"cores,omitempty"`
	Sockets     int    `json:"sockets,omitempty"`
	Memory      int64  `json:"memory,omitempty"` // in bytes
	Description string `json:"description,omitempty"`
	OnBoot      *bool  `json:"onboot,omitempty"`

	// QEMU-specific
	CPUType   string `json:"cpu,omitempty"`
	MaxMem    int64  `json:"maxmem,omitempty"`
	BootOrder string `json:"boot,omitempty"`
	// Add more QEMU fields as needed

	// LXC-specific
	Swap int64 `json:"swap,omitempty"`
	// Add more LXC fields as needed

	// Storage (for resizing, etc.)
	Disks map[string]int64 `json:"disks,omitempty"` // disk name -> size in bytes
}

// GetVMConfig fetches the configuration for a VM or container.
func (c *Client) GetVMConfig(vm *VM) (*VMConfig, error) {
	var result map[string]interface{}

	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.Node, vm.Type, vm.ID)
	if err := c.Get(endpoint, &result); err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected config response format")
	}

	return parseVMConfig(vm.Type, data), nil
}

// UpdateVMConfig updates the configuration for a VM or container.
// For LXC: uses PUT (synchronous, no task ID)
// For QEMU: uses POST (asynchronous, returns task ID).
func (c *Client) UpdateVMConfig(vm *VM, config *VMConfig) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.Node, vm.Type, vm.ID)
	data := buildConfigPayload(vm.Type, config)

	if vm.Type == VMTypeLXC {
		return c.httpClient.Put(context.Background(), endpoint, data, nil)
	} else if vm.Type == VMTypeQemu {
		return c.httpClient.Post(context.Background(), endpoint, data, nil)
	}

	return fmt.Errorf("unsupported VM type: %s", vm.Type)
}

// ResizeVMStorage resizes a disk for a VM or container.
func (c *Client) ResizeVMStorage(vm *VM, disk string, size string) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/resize", vm.Node, vm.Type, vm.ID)
	data := map[string]interface{}{
		"disk": disk,
		"size": size, // Proxmox expects size as string (e.g., "+10G")
	}

	return c.httpClient.Put(context.Background(), endpoint, data, nil)
}

// UpdateVMResources updates CPU and memory for a VM or container.
func (c *Client) UpdateVMResources(vm *VM, cores int, memory int64) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.Node, vm.Type, vm.ID)
	data := map[string]interface{}{
		"cores":  cores,
		"memory": memory / 1024 / 1024, // Proxmox expects memory in MB
	}

	if vm.Type == VMTypeLXC {
		return c.httpClient.Put(context.Background(), endpoint, data, nil)
	} else if vm.Type == VMTypeQemu {
		return c.httpClient.Post(context.Background(), endpoint, data, nil)
	}

	return fmt.Errorf("unsupported VM type: %s", vm.Type)
}

// parseVMConfig parses the config API response into a VMConfig struct.
func parseVMConfig(vmType string, data map[string]interface{}) *VMConfig {
	cfg := &VMConfig{}
	if v, ok := data["name"].(string); ok {
		cfg.Name = v
	}
	if v, ok := data["hostname"].(string); ok {
		cfg.Hostname = v
	}
	if v, ok := data["cores"].(float64); ok {
		cfg.Cores = int(v)
	}

	if v, ok := data["sockets"].(float64); ok {
		cfg.Sockets = int(v)
	}
	// Memory (MB) for both QEMU and LXC
	if memRaw, ok := data["memory"]; ok {
		switch v := memRaw.(type) {
		case string:
			// QEMU: memory is a string (MB)
			if mb, err := strconv.Atoi(v); err == nil {
				cfg.Memory = int64(mb) * 1024 * 1024
			}
		case float64:
			// LXC: memory is a float (MB)
			cfg.Memory = int64(v) * 1024 * 1024
		case int:
			cfg.Memory = int64(v) * 1024 * 1024
		}
	}

	if v, ok := data["description"].(string); ok {
		cfg.Description = v
	}

	if v, ok := data["onboot"].(float64); ok {
		b := v != 0
		cfg.OnBoot = &b
	}

	if v, ok := data["onboot"].(string); ok {
		b := v == "1" || strings.ToLower(v) == "yes"
		cfg.OnBoot = &b
	}

	if vmType == VMTypeQemu {
		if v, ok := data["cpu"].(string); ok {
			cfg.CPUType = v
		}

		if v, ok := data["maxmem"].(float64); ok {
			cfg.MaxMem = int64(v)
		}

		if v, ok := data["boot"].(string); ok {
			cfg.BootOrder = v
		}
	}

	if vmType == VMTypeLXC {
		if v, ok := data["swap"].(float64); ok {
			cfg.Swap = int64(v) * 1024 * 1024
		}
	}
	// Storage parsing can be added here
	return cfg
}

// buildConfigPayload builds the payload for updating VM/LXC config.
func buildConfigPayload(vmType string, config *VMConfig) map[string]interface{} {
	data := map[string]interface{}{}
	if config.Name != "" {
		data["name"] = config.Name
	}
	if config.Hostname != "" {
		data["hostname"] = config.Hostname
	}
	if config.Cores > 0 {
		data["cores"] = config.Cores
	}

	if config.Sockets > 0 {
		data["sockets"] = config.Sockets
	}

	if config.Memory > 0 {
		data["memory"] = config.Memory / 1024 / 1024 // MB
	}

	if config.Description != "" {
		data["description"] = config.Description
	}

	if config.OnBoot != nil {
		if *config.OnBoot {
			data["onboot"] = 1
		} else {
			data["onboot"] = 0
		}
	}

	if vmType == VMTypeQemu {
		if config.CPUType != "" {
			data["cpu"] = config.CPUType
		}

		if config.MaxMem > 0 {
			data["maxmem"] = config.MaxMem
		}

		if config.BootOrder != "" {
			data["boot"] = config.BootOrder
		}
	}

	if vmType == VMTypeLXC {
		if config.Swap > 0 {
			data["swap"] = config.Swap / 1024 / 1024 // MB
		}
	}

	return data
}
