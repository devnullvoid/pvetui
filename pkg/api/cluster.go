package api

import (
	"strings"
)

// Cluster represents aggregated Proxmox cluster metrics
type Cluster struct {
	TotalNodes    int
	OnlineNodes   int
	TotalCPU      float64
	CPUUsage      float64
	TotalMemory   int64
	UsedMemory    int64
	TotalStorage  int64
	UsedStorage   int64
	ClusterName   string
	PVEVersion    string
	KernelVersion string
}

// GetClusterStatus retrieves and parses cluster status into structured format
func (c *Client) GetClusterStatus() (*Cluster, error) {
	cluster := &Cluster{}

	nodes, err := c.ListNodes()
	if err != nil {
		return nil, err
	}

	cluster.TotalNodes = len(nodes)
	for _, node := range nodes {
		if node.Online {
			cluster.OnlineNodes++
		}
		cluster.TotalCPU += node.CPUCount
		cluster.CPUUsage += node.CPUUsage
		if cluster.KernelVersion == "" {
			cluster.KernelVersion = node.KernelVersion
		}
		cluster.TotalMemory += node.MemoryTotal
		cluster.UsedMemory += node.MemoryUsed
		cluster.TotalStorage += node.TotalStorage
		cluster.UsedStorage += node.UsedStorage

		if cluster.PVEVersion == "" {
			cluster.PVEVersion = node.Version
		}
	}

	if len(nodes) > 0 {
		parts := strings.Split(nodes[0].Name, ".")
		if len(parts) > 1 {
			cluster.ClusterName = parts[1]
		} else {
			cluster.ClusterName = "proxmox"
		}
	}

	return cluster, nil
}
