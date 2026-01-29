// Package api provides Proxmox API client functionality.
package api

import (
	"context"
	"fmt"
	"sort"
)

// GetGroupNodes retrieves nodes from all connected profiles in the group.
// Each node's SourceProfile field is set to identify which profile it came from.
// Returns a combined list of all nodes across all profiles.
func (m *GroupClientManager) GetGroupNodes(ctx context.Context) ([]*Node, error) {
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

	groupFunc := func(results []ProfileResult) (interface{}, error) {
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

	data, err := m.GetGroupData(ctx, operation, groupFunc)

	// Handle the case where no profiles returned results (possibly all offline)
	var nodes []*Node
	if err != nil {
		if err.Error() == "no profiles returned successful results" {
			nodes = []*Node{}
		} else {
			return nil, err
		}
	} else {
		var ok bool
		nodes, ok = data.([]*Node)
		if !ok {
			return nil, fmt.Errorf("unexpected data type returned from grouping")
		}
	}

	// Identify which profiles are represented in the results
	representedProfiles := make(map[string]bool)
	for _, node := range nodes {
		if node != nil {
			representedProfiles[node.SourceProfile] = true
		}
	}

	// Add placeholder nodes for missing/offline profiles
	allClients := m.GetAllClients()
	for _, pc := range allClients {
		if !representedProfiles[pc.ProfileName] {
			status, lastErr := pc.GetStatus()

			// Determine reason for missing data
			errorMsg := "Unknown Error"
			if status == ProfileStatusConnected {
				// Connected but returned no nodes?
				errorMsg = "No Data"
			} else {
				errorMsg = "Offline"
				if lastErr != nil {
					// Use a short error message
					errorMsg = "Connection Failed"
				}
			}

			// Create placeholder node
			placeholder := &Node{
				ID:            fmt.Sprintf("offline-%s", pc.ProfileName),
				Name:          pc.ProfileName,
				Online:        false,
				SourceProfile: pc.ProfileName,
				Version:       errorMsg, // Use Version field to potentially show error
			}
			nodes = append(nodes, placeholder)
		}
	}

	// Sort nodes again to ensure offline nodes are interleaved correctly
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i] == nil || nodes[j] == nil {
			return nodes[i] != nil
		}
		// Sort by profile name first, then by node name
		if nodes[i].SourceProfile != nodes[j].SourceProfile {
			return nodes[i].SourceProfile < nodes[j].SourceProfile
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

// GetGroupVMs retrieves VMs from all connected profiles in the group.
// Each VM's SourceProfile field is set to identify which profile it came from.
// Returns a combined list of all VMs across all profiles.
func (m *GroupClientManager) GetGroupVMs(ctx context.Context) ([]*VM, error) {
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

	groupFunc := func(results []ProfileResult) (interface{}, error) {
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

	data, err := m.GetGroupData(ctx, operation, groupFunc)
	if err != nil {
		return nil, err
	}

	vms, ok := data.([]*VM)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from grouping")
	}
	return vms, nil
}

// GetGroupClusterResources retrieves cluster resources from all profiles in the group.
// This provides a unified view of all resources across all connected profiles.
// fresh flag bypasses caches when true.
func (m *GroupClientManager) GetGroupClusterResources(ctx context.Context, fresh bool) ([]*Node, []*VM, error) {
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
		nodes, err := m.GetGroupNodes(ctx)
		if fresh {
			// Invalidate node status caches for each profile before returning
			for _, pc := range m.GetConnectedClients() {
				if pc.Client != nil && pc.Client.cache != nil {
					_ = pc.Client.cache.Clear()
				}
			}
		}
		nodesChan <- result{nodes: nodes, err: err}
	}()

	// Fetch VMs
	go func() {
		vms, err := m.GetGroupVMs(ctx)
		vmsChan <- result{vms: vms, err: err}
	}()

	// Wait for both operations to complete
	nodesResult := <-nodesChan
	vmsResult := <-vmsChan

	if nodesResult.err != nil {
		return nil, nil, fmt.Errorf("failed to get group nodes: %w", nodesResult.err)
	}

	if vmsResult.err != nil {
		return nil, nil, fmt.Errorf("failed to get group VMs: %w", vmsResult.err)
	}

	return nodesResult.nodes, vmsResult.vms, nil
}

// GetNodeFromGroup retrieves a specific node from a specific profile.
// This is useful when you need to perform operations on a node and need to ensure
// you're using the correct profile's client.
func (m *GroupClientManager) GetNodeFromGroup(
	ctx context.Context,
	profileName string,
	nodeName string,
) (*Node, error) {
	operation := func(pName string, client *Client) (interface{}, error) {
		// Clear per-node caches to ensure fresh status
		if client.cache != nil {
			_ = client.cache.Clear()
		}
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

// GetVMFromGroup retrieves detailed VM information from a specific profile.
// This is useful when you need to perform operations on a VM and need to ensure
// you're using the correct profile's client.
func (m *GroupClientManager) GetVMFromGroup(
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

// FindVMByIDInGroup searches for a VM with the given ID across all profiles.
// Returns the VM and its source profile name, or an error if not found.
// Note: VM IDs should be unique within a profile, but may overlap across profiles.
func (m *GroupClientManager) FindVMByIDInGroup(ctx context.Context, vmID int) (*VM, string, error) {
	vms, err := m.GetGroupVMs(ctx)
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

// FindNodeByNameInGroup searches for a node with the given name across all profiles.
// Returns the node and its source profile name, or an error if not found.
func (m *GroupClientManager) FindNodeByNameInGroup(ctx context.Context, nodeName string) (*Node, string, error) {
	nodes, err := m.GetGroupNodes(ctx)
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

// GetGroupTasks retrieves tasks from all connected profiles in the group.
// Each task's SourceProfile field is set to identify which profile it came from.
// Returns a combined list of all tasks across all profiles.
func (m *GroupClientManager) GetGroupTasks(ctx context.Context) ([]*ClusterTask, error) {
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

	groupFunc := func(results []ProfileResult) (interface{}, error) {
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

	data, err := m.GetGroupData(ctx, operation, groupFunc)
	if err != nil {
		return nil, err
	}

	tasks, ok := data.([]*ClusterTask)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from grouping")
	}

	return tasks, nil
}
