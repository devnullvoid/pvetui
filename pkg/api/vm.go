package api

import (
	"context"
	"fmt"
	"time"
)

// VM represents a Proxmox VM or container
type VM struct {
	ID     int
	Name   string
	Node   string
	Type   string
	IP     string
	Status string
}

// ListVMs retrieves all virtual machines on the given node
func (c *Client) ListVMs(nodeName string) ([]VM, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw, err := c.ProxClient.GetVmList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM list: %w", err)
	}

	data, ok := raw["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM list")
	}

	var vms []VM
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if m["node"].(string) != nodeName {
			continue
		}

		vmid, ok := m["vmid"].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid VM ID in response: %v", m)
		}

		name, ok := m["name"].(string)
		if !ok {
			return nil, fmt.Errorf("VM %d missing name", int(vmid))
		}

		status, ok := m["status"].(string)
		if !ok {
			return nil, fmt.Errorf("VM %d (%s) missing status", int(vmid), name)
		}

		tp, _ := m["type"].(string)
		if tp == "" {
			tp = "qemu"
		}

		var ip string
		if ipVal, ok := m["ip"]; ok {
			if ipStr, ok := ipVal.(string); ok {
				ip = ipStr
			}
		}

		vms = append(vms, VM{
			ID:     int(vmid),
			Name:   name,
			Node:   nodeName,
			Type:   tp,
			Status: status,
			IP:     ip,
		})
	}
	return vms, nil
}

// GetVmStatus retrieves current status metrics for a VM or LXC
func (c *Client) GetVmStatus(vm VM) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/status/current?full=1", vm.Node, vm.Type, vm.ID)
	if err := c.ProxClient.GetJsonRetryable(ctx, endpoint, &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM status")
	}
	return data, nil
}

// GetVmConfig retrieves configuration for a given VM or LXC
func (c *Client) GetVmConfig(vm VM) (map[string]interface{}, error) {
	var res map[string]interface{}
	endpoint := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.Node, vm.Type, vm.ID)
	if err := c.ProxClient.GetJsonRetryable(context.Background(), endpoint, &res, 3); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for VM config")
	}
	return data, nil
}
