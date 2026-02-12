package components

import (
	"context"
	"fmt"
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// autoRefreshDataWithFooter sets loading state and starts the data fetch in a new goroutine.
func (a *App) autoRefreshDataWithFooter() {
	a.QueueUpdateDraw(func() {
		a.footer.SetLoading(true)
	})

	go a.autoRefreshData()
}

// autoRefreshData performs a lightweight refresh of performance data.
func (a *App) autoRefreshData() {
	uiLogger := models.GetUILogger()

	// Store current selections to preserve them
	var selectedVMID int

	var selectedVMNode string

	var selectedNodeName string

	var hasSelectedVM bool

	var hasSelectedNode bool

	if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
		selectedVMID = selectedVM.ID
		selectedVMNode = selectedVM.Node
		hasSelectedVM = true
	}

	if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
		selectedNodeName = selectedNode.Name
		hasSelectedNode = true
	}

	// Check if search is currently active
	searchWasActive := a.mainLayout.GetItemCount() > 4

	if a.isGroupMode {
		// Group mode logic
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		nodes, vms, err := a.groupManager.GetGroupClusterResources(ctx, true)
		if err != nil {
			uiLogger.Debug("Auto-refresh failed (group): %v", err)
			a.QueueUpdateDraw(func() {
				a.footer.SetLoading(false)
			})
			return
		}

		a.QueueUpdateDraw(func() {
			// Get current search states
			nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

			// Preserve detailed node data
			for _, freshNode := range nodes {
				if freshNode != nil {
					// Find the corresponding existing node with detailed data
					for _, existingNode := range models.GlobalState.OriginalNodes {
						if existingNode != nil && existingNode.Name == freshNode.Name && existingNode.SourceProfile == freshNode.SourceProfile {
							// Preserve detailed fields
							freshNode.Version = existingNode.Version
							freshNode.KernelVersion = existingNode.KernelVersion
							freshNode.CPUInfo = existingNode.CPUInfo
							freshNode.LoadAvg = existingNode.LoadAvg
							freshNode.CGroupMode = existingNode.CGroupMode
							freshNode.Level = existingNode.Level
							freshNode.Storage = existingNode.Storage
							break
						}
					}
				}
			}

			// Update global state with fresh data
			models.GlobalState.OriginalNodes = make([]*api.Node, len(nodes))
			models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
			models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
			models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))

			copy(models.GlobalState.OriginalNodes, nodes)
			copy(models.GlobalState.FilteredNodes, nodes)
			copy(models.GlobalState.OriginalVMs, vms)
			copy(models.GlobalState.FilteredVMs, vms)

			a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
				hasSelectedNode, selectedNodeName, nodeSearchState)

			// Defer list/detail updates until enrichment completes to reduce flicker.

			// Refresh tasks if on tasks page
			currentPage, _ := a.pages.GetFrontPage()
			if currentPage == api.PageTasks {
				go func() {
					ctxTasks, cancelTasks := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancelTasks()
					tasks, err := a.groupManager.GetGroupTasks(ctxTasks)
					if err == nil {
						a.QueueUpdateDraw(func() {
							if state := models.GlobalState.GetSearchState(api.PageTasks); state != nil && state.Filter != "" {
								models.GlobalState.OriginalTasks = make([]*api.ClusterTask, len(tasks))
								copy(models.GlobalState.OriginalTasks, tasks)
								models.FilterTasks(state.Filter)
								a.tasksList.SetFilteredTasks(models.GlobalState.FilteredTasks)
							} else {
								a.tasksList.SetTasks(tasks)
							}
						})
					}
				}()
			}

			a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)

			// Start background enrichment for detailed node stats
			// Pass false for isInitialLoad since this is auto-refresh
			a.enrichGroupNodesSequentially(nodes, hasSelectedNode, selectedNodeName, hasSelectedVM, selectedVMID, selectedVMNode, searchWasActive, false)

			a.autoRefreshCountdown = 10

			a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)

		})

		return

	}

	// Fetch fresh cluster resources data (this includes performance metrics)
	cluster, err := a.client.GetFreshClusterStatus()
	if err != nil {
		uiLogger.Debug("Auto-refresh failed: %v", err)
		a.QueueUpdateDraw(func() {
			a.footer.SetLoading(false)
		})

		return
	}

	// Update UI with new data
	a.QueueUpdateDraw(func() {
		// Get current search states
		nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
		vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

		// Preserve cluster version from existing data
		if len(models.GlobalState.OriginalNodes) > 0 {
			// Find existing cluster version by checking if we have any node with version info
			for _, existingNode := range models.GlobalState.OriginalNodes {
				if existingNode != nil && existingNode.Version != "" {
					cluster.Version = fmt.Sprintf("Proxmox VE %s", existingNode.Version)

					break
				}
			}
		}

		// Update cluster status (this shows updated CPU/memory/storage totals)
		a.clusterStatus.Update(cluster)

		// Preserve detailed node data while updating performance metrics
		for _, freshNode := range cluster.Nodes {
			if freshNode != nil {
				// Find the corresponding existing node with detailed data
				for _, existingNode := range models.GlobalState.OriginalNodes {
					if existingNode != nil && existingNode.Name == freshNode.Name {
						// Preserve detailed fields that aren't in cluster resources
						freshNode.Version = existingNode.Version
						freshNode.KernelVersion = existingNode.KernelVersion
						freshNode.CPUInfo = existingNode.CPUInfo
						freshNode.LoadAvg = existingNode.LoadAvg
						freshNode.CGroupMode = existingNode.CGroupMode
						freshNode.Level = existingNode.Level
						freshNode.Storage = existingNode.Storage

						break
					}
				}
			}
		}

		// Rebuild VM list from fresh cluster data
		var vms []*api.VM

		for _, node := range cluster.Nodes {
			if node != nil {
				for _, vm := range node.VMs {
					if vm != nil {
						vms = append(vms, vm)
					}
				}
			}
		}

		// Update global state with fresh data
		models.GlobalState.OriginalNodes = make([]*api.Node, len(cluster.Nodes))
		models.GlobalState.FilteredNodes = make([]*api.Node, len(cluster.Nodes))
		models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
		models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))

		copy(models.GlobalState.OriginalNodes, cluster.Nodes)
		copy(models.GlobalState.FilteredNodes, cluster.Nodes)
		copy(models.GlobalState.OriginalVMs, vms)
		copy(models.GlobalState.FilteredVMs, vms)

		// Apply filters if active, otherwise use all data
		if nodeSearchState != nil && nodeSearchState.Filter != "" {
			models.FilterNodes(nodeSearchState.Filter)
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
		} else {
			a.nodeList.SetNodes(models.GlobalState.OriginalNodes)
		}

		if vmSearchState != nil && vmSearchState.Filter != "" {
			models.FilterVMs(vmSearchState.Filter)
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		} else {
			a.vmList.SetVMs(models.GlobalState.OriginalVMs)
		}

		a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
			hasSelectedNode, selectedNodeName, nodeSearchState)

		// Update details if items are selected
		if node := a.nodeList.GetSelectedNode(); node != nil {
			a.nodeDetails.Update(node, cluster.Nodes)
		}

		if vm := a.vmList.GetSelectedVM(); vm != nil {
			a.vmDetails.Update(vm)
		}

		// Refresh tasks if on tasks page
		currentPage, _ := a.pages.GetFrontPage()
		if currentPage == api.PageTasks {
			// Refresh tasks data without showing loading indicator (background refresh)
			go func() {
				tasks, err := a.client.GetClusterTasks()
				if err == nil {
					a.QueueUpdateDraw(func() {
						// Check if there's an active search filter
						if state := models.GlobalState.GetSearchState(api.PageTasks); state != nil && state.Filter != "" {
							// Update global state and apply filter
							models.GlobalState.OriginalTasks = make([]*api.ClusterTask, len(tasks))
							copy(models.GlobalState.OriginalTasks, tasks)
							models.FilterTasks(state.Filter)
							a.tasksList.SetFilteredTasks(models.GlobalState.FilteredTasks)
						} else {
							// No filter active, just update normally
							a.tasksList.SetTasks(tasks)
						}
					})
				}
			}()
		}

		// Restore search input UI state if it was active before refresh
		a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)

		// Show success message
		a.header.ShowSuccess("Data refreshed successfully")
		a.footer.SetLoading(false)

		// Reset countdown after refresh is complete
		a.autoRefreshCountdown = 10
		a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
	})
}
