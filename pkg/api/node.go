package api

import (
	"fmt"
	"strings"
)

// Storage represents a Proxmox storage resource
type Storage struct {
	ID         string `json:"id"`
	Content    string `json:"content,omitempty"`
	Disk       int64  `json:"disk,omitempty"`
	MaxDisk    int64  `json:"maxdisk,omitempty"`
	Node       string `json:"node,omitempty"`
	Plugintype string `json:"plugintype,omitempty"`
	Status     string `json:"status,omitempty"`
}

// Node represents a Proxmox cluster node
type Node struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	IP            string   `json:"ip"`
	CPUCount      float64  `json:"maxcpu"`
	CPUUsage      float64  `json:"cpu"`
	MemoryTotal   float64  `json:"maxmem"`
	MemoryUsed    float64  `json:"mem"`
	TotalStorage  int64    `json:"maxdisk"`
	UsedStorage   int64    `json:"disk"`
	Uptime        int64    `json:"uptime"`
	Version       string   `json:"pveversion"`
	KernelVersion string   `json:"kversion"`
	Online        bool     `json:"-"`
	CGroupMode    int      `json:"cgroup_mode,omitempty"`
	Level         string   `json:"level,omitempty"`
	Storage       *Storage `json:"storage,omitempty"`
	VMs           []*VM    `json:"vms,omitempty"`
}

// ListNodes retrieves nodes from cached cluster data
func (c *Client) ListNodes() ([]Node, error) {
	if c.Cluster == nil {
		if _, err := c.GetClusterStatus(); err != nil {
			return nil, err
		}
	}

	nodes := make([]Node, 0, len(c.Cluster.Nodes))
	for _, clusterNode := range c.Cluster.Nodes {
		if clusterNode != nil {
			nodes = append(nodes, *clusterNode)
		}
	}
	return nodes, nil
}

// GetNodeStatus retrieves real-time status for a specific node
func (c *Client) GetNodeStatus(nodeName string) (*Node, error) {
	var res map[string]interface{}

	if err := c.Get(fmt.Sprintf("/nodes/%s/status", nodeName), &res); err != nil {
		return nil, fmt.Errorf("failed to get status for node %s: %w", nodeName, err)
	}

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid status response format for node %s", nodeName)
	}

	node := &Node{
		Name:          nodeName,
		Online:        strings.EqualFold(getString(data, "status"), "online"),
		CPUCount:      getFloat(data, "maxcpu"),
		CPUUsage:      getFloat(data, "cpu"),
		KernelVersion: getString(data, "kversion"),
		MemoryTotal:   getFloat(data, "maxmem") / 1073741824, // Bytes to GB
		MemoryUsed:    getFloat(data, "mem") / 1073741824,    // Bytes to GB
		TotalStorage:  int64(getFloat(data, "maxdisk")),
		UsedStorage:   int64(getFloat(data, "disk")),
		Uptime:        int64(getFloat(data, "uptime")),
		Version:       getString(data, "pveversion"),
	}

	// Fallback to version endpoint if pveversion not in status
	if node.Version == "" {
		var versionRes map[string]interface{}

		if err := c.Get(fmt.Sprintf("/nodes/%s/version", nodeName), &versionRes); err == nil {
			if versionData, ok := versionRes["data"].(map[string]interface{}); ok {
				node.Version = getString(versionData, "version")
			}
		}
	}

	return node, nil
}

// GetNodeConfig retrieves configuration for a given node
func (c *Client) GetNodeConfig(nodeName string) (map[string]interface{}, error) {
	var res map[string]interface{}
	if err := c.Get(fmt.Sprintf("/nodes/%s/config", nodeName), &res); err != nil {
		return nil, err
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for node config")
	}
	return data, nil
}
