// Package api provides Proxmox API client functionality.
package api

import (
	"context"
	"fmt"
	"sort"
)

// GetAggregatedNodes retrieves nodes from all connected profiles in the aggregate.
// Each node's SourceProfile field is set to identify which profile it came from.
// Returns a combined list of all nodes across all profiles.
func (m *AggregateClientManager) GetAggregatedNodes(ctx context.Context) ([]*Node, error) {
	operation := func(profileName string, client *Client) (interface{}, error) {
		nodes, err := client.ListNodes()
		if err != nil {
			return nil, fmt.Errorf("failed to list nodes: %w", err)
		}

		// Convert to pointers and set SourceProfile
		nodePointers := make([]*Node, len(nodes))
		for i := range nodes {
			nodePointers[i] = &nodes[i]
			nodePointers[i].SourceProfile = profileName
		}

		return nodePointers, nil
	}

	aggregateFunc := func(results []ProfileResult) (interface{}, error) {
		var allNodes []*Node

		for _, result := range results {
			if nodes, ok := result.Data.([]*Node); ok {
				allNodes = append(allNodes, nodes...)
			}
		}

		// Sort nodes by name for consistent ordering
		sort.Slice(allNodes, func(i, j int) bool {
			if allNodes[i] == nil || allNodes[j] == nil {
				return allNodes[i] != nil
			}
			// Sort by profile name first, then by node name
			if allNodes[i].SourceProfile != allNodes[j].SourceProfile {
				return allNodes[i].SourceProfile < allNodes[j].SourceProfile
			}
			return allNodes[i].Name < allNodes[j].Name
		})

		return allNodes, nil
	}

	data, err := m.GetAggregatedData(ctx, operation, aggregateFunc)
	if err != nil {
		return nil, err
	}

	nodes, ok := data.([]*Node)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from aggregation")
	}

	return nodes, nil
}

// GetAggregatedVMs retrieves VMs from all connected profiles in the aggregate.
// Each VM's SourceProfile field is set to identify which profile it came from.
// Returns a combined list of all VMs across all profiles.
func (m *AggregateClientManager) GetAggregatedVMs(ctx context.Context) ([]*VM, error) {
	operation := func(profileName string, client *Client) (interface{}, error) {
		// Get cluster status to retrieve nodes with their VMs
		cluster, err := client.GetClusterStatus()
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster status: %w", err)
		}

		// Collect all VMs from all nodes
		var vms []*VM
		for _, node := range cluster.Nodes {
			if node != nil && node.VMs != nil {
				for _, vm := range node.VMs {
					if vm != nil {
						vm.SourceProfile = profileName
						vms = append(vms, vm)
					}
				}
			}
		}

		return vms, nil
	}

	aggregateFunc := func(results []ProfileResult) (interface{}, error) {
		var allVMs []*VM

		for _, result := range results {
			if vms, ok := result.Data.([]*VM); ok {
				allVMs = append(allVMs, vms...)
			}
		}

		// Sort VMs by profile name, then by node, then by ID
		sort.Slice(allVMs, func(i, j int) bool {
			if allVMs[i] == nil || allVMs[j] == nil {
				return allVMs[i] != nil
			}
			// Sort by profile name first
			if allVMs[i].SourceProfile != allVMs[j].SourceProfile {
				return allVMs[i].SourceProfile < allVMs[j].SourceProfile
			}
			// Then by node name
			if allVMs[i].Node != allVMs[j].Node {
				return allVMs[i].Node < allVMs[j].Node
			}
			// Finally by VM ID
			return allVMs[i].ID < allVMs[j].ID
		})

		return allVMs, nil
	}

	data, err := m.GetAggregatedData(ctx, operation, aggregateFunc)
	if err != nil {
		return nil, err
	}

	vms, ok := data.([]*VM)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from aggregation")
	}

	return vms, nil
}

// GetAggregatedClusterResources retrieves cluster resources from all profiles.
// This provides a unified view of all resources across all connected profiles.
func (m *AggregateClientManager) GetAggregatedClusterResources(ctx context.Context) ([]*Node, []*VM, error) {
	// Use goroutines to fetch nodes and VMs concurrently for better performance
	type result struct {
		nodes []*Node
		vms   []*VM
		err   error
	}

	nodesChan := make(chan result, 1)
	vmsChan := make(chan result, 1)

	// Fetch nodes
	go func() {
		nodes, err := m.GetAggregatedNodes(ctx)
		nodesChan <- result{nodes: nodes, err: err}
	}()

	// Fetch VMs
	go func() {
		vms, err := m.GetAggregatedVMs(ctx)
		vmsChan <- result{vms: vms, err: err}
	}()

	// Wait for both operations to complete
	nodesResult := <-nodesChan
	vmsResult := <-vmsChan

	if nodesResult.err != nil {
		return nil, nil, fmt.Errorf("failed to get aggregated nodes: %w", nodesResult.err)
	}

	if vmsResult.err != nil {
		return nil, nil, fmt.Errorf("failed to get aggregated VMs: %w", vmsResult.err)
	}

	return nodesResult.nodes, vmsResult.vms, nil
}

// GetNodeFromProfile retrieves a specific node from a specific profile.
// This is useful when you need to perform operations on a node and need to ensure
// you're using the correct profile's client.
func (m *AggregateClientManager) GetNodeFromProfile(
	ctx context.Context,
	profileName string,
	nodeName string,
) (*Node, error) {
	operation := func(pName string, client *Client) (interface{}, error) {
		node, err := client.GetNodeStatus(nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to get node status: %w", err)
		}
		node.SourceProfile = pName
		return node, nil
	}

	data, err := m.ExecuteOnProfile(ctx, profileName, operation)
	if err != nil {
		return nil, err
	}

	node, ok := data.(*Node)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned")
	}

	return node, nil
}

// GetVMFromProfile retrieves detailed VM information from a specific profile.
// This is useful when you need to perform operations on a VM and need to ensure
// you're using the correct profile's client.
func (m *AggregateClientManager) GetVMFromProfile(
	ctx context.Context,
	profileName string,
	nodeName string,
	vmType string,
	vmID int,
) (*VM, error) {
	operation := func(pName string, client *Client) (interface{}, error) {
		vm, err := client.GetDetailedVmInfo(nodeName, vmType, vmID)
		if err != nil {
			return nil, fmt.Errorf("failed to get VM info: %w", err)
		}
		vm.SourceProfile = pName
		return vm, nil
	}

	data, err := m.ExecuteOnProfile(ctx, profileName, operation)
	if err != nil {
		return nil, err
	}

	vm, ok := data.(*VM)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned")
	}

	return vm, nil
}

// FindVMByID searches for a VM with the given ID across all profiles.
// Returns the VM and its source profile name, or an error if not found.
// Note: VM IDs should be unique within a profile, but may overlap across profiles.
func (m *AggregateClientManager) FindVMByID(ctx context.Context, vmID int) (*VM, string, error) {
	vms, err := m.GetAggregatedVMs(ctx)
	if err != nil {
		return nil, "", err
	}

	for _, vm := range vms {
		if vm.ID == vmID {
			return vm, vm.SourceProfile, nil
		}
	}

	return nil, "", fmt.Errorf("VM with ID %d not found in any profile", vmID)
}

// FindNodeByName searches for a node with the given name across all profiles.
// Returns the node and its source profile name, or an error if not found.
func (m *AggregateClientManager) FindNodeByName(ctx context.Context, nodeName string) (*Node, string, error) {
	nodes, err := m.GetAggregatedNodes(ctx)
	if err != nil {
		return nil, "", err
	}

	for _, node := range nodes {
		if node.Name == nodeName {
			return node, node.SourceProfile, nil
		}
	}

	return nil, "", fmt.Errorf("node with name %s not found in any profile", nodeName)
}

// GetAggregatedTasks retrieves tasks from all connected profiles in the aggregate.
// Each task's SourceProfile field is set to identify which profile it came from.
// Returns a combined list of all tasks across all profiles.
func (m *AggregateClientManager) GetAggregatedTasks(ctx context.Context) ([]*ClusterTask, error) {
	operation := func(profileName string, client *Client) (interface{}, error) {
		tasks, err := client.GetClusterTasks()
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster tasks: %w", err)
		}

		// Set SourceProfile for each task
		for _, task := range tasks {
			task.SourceProfile = profileName
		}

		return tasks, nil
	}

	aggregateFunc := func(results []ProfileResult) (interface{}, error) {
		var allTasks []*ClusterTask

		for _, result := range results {
			if tasks, ok := result.Data.([]*ClusterTask); ok {
				allTasks = append(allTasks, tasks...)
			}
		}

		// Sort tasks by StartTime desc
		sort.Slice(allTasks, func(i, j int) bool {
			if allTasks[i] == nil || allTasks[j] == nil {
				return allTasks[i] != nil
			}
			return allTasks[i].StartTime > allTasks[j].StartTime
		})

		return allTasks, nil
	}

	data, err := m.GetAggregatedData(ctx, operation, aggregateFunc)
	if err != nil {
		return nil, err
	}

	tasks, ok := data.([]*ClusterTask)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from aggregation")
	}

	return tasks, nil
}
