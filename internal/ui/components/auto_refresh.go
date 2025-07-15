package components

import (
	"fmt"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// toggleAutoRefresh toggles the auto-refresh functionality on/off
func (a *App) toggleAutoRefresh() {
	uiLogger := models.GetUILogger()

	if a.autoRefreshEnabled {
		// Disable auto-refresh
		a.stopAutoRefresh()
		a.autoRefreshEnabled = false
		a.footer.UpdateAutoRefreshStatus(false)
		a.header.ShowSuccess("Auto-refresh disabled")
		uiLogger.Debug("Auto-refresh disabled by user")
	} else {
		// Enable auto-refresh
		a.autoRefreshEnabled = true
		a.startAutoRefresh()
		a.footer.UpdateAutoRefreshStatus(true)
		a.header.ShowSuccess("Auto-refresh enabled (10s interval)")
		uiLogger.Debug("Auto-refresh enabled by user")
	}
}

// startAutoRefresh starts the auto-refresh timer
func (a *App) startAutoRefresh() {
	// Don't start if auto-refresh is not enabled
	if !a.autoRefreshEnabled {
		return
	}

	if a.autoRefreshTicker != nil {
		return // Already running
	}

	a.autoRefreshStop = make(chan bool, 1)
	a.autoRefreshTicker = time.NewTicker(10 * time.Second) // 10 second interval
	a.autoRefreshCountdown = 10
	a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
	a.autoRefreshCountdownStop = make(chan bool, 1)

	// Start countdown goroutine
	go func() {
		uiLogger := models.GetUILogger()
		for {
			select {
			case <-a.autoRefreshCountdownStop:
				return
			case <-a.ctx.Done():
				return
			default:
				time.Sleep(1 * time.Second)
				if !a.autoRefreshEnabled {
					return
				}
				if a.footer.IsLoading() {
					continue // Pause countdown while loading
				}
				a.autoRefreshCountdown--
				if a.autoRefreshCountdown < 0 {
					a.autoRefreshCountdown = 0
				}

				// Trigger refresh when countdown reaches 0
				if a.autoRefreshCountdown == 0 {
					// Only refresh if not currently loading something and no pending operations
					if !a.header.IsLoading() && !models.GlobalState.HasPendingOperations() {
						uiLogger.Debug("Auto-refresh triggered by countdown")
						go a.autoRefreshDataWithFooter()
					} else {
						if a.header.IsLoading() {
							uiLogger.Debug("Auto-refresh skipped - header loading operation in progress")
						} else {
							uiLogger.Debug("Auto-refresh skipped - pending VM/node operations in progress")
						}
						// Reset countdown to try again in 10 seconds
						a.autoRefreshCountdown = 10
					}
				}

				a.QueueUpdateDraw(func() {
					a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
				})
			}
		}
	}()

	// Spinner animation goroutine
	go func() {
		for {
			select {
			case <-a.ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
				if !a.autoRefreshEnabled {
					return
				}
				if a.footer.IsLoading() {
					a.QueueUpdateDraw(func() {
						a.footer.TickSpinner()
					})
				}
			}
		}
	}()
}

// stopAutoRefresh stops the auto-refresh timer
func (a *App) stopAutoRefresh() {
	// Always stop and nil out the ticker, close channels, and reset countdown
	if a.autoRefreshTicker != nil {
		a.autoRefreshTicker.Stop()
		a.autoRefreshTicker = nil
	}

	if a.autoRefreshStop != nil {
		select {
		case a.autoRefreshStop <- true:
		default:
		}
		close(a.autoRefreshStop)
		a.autoRefreshStop = nil
	}
	if a.autoRefreshCountdownStop != nil {
		close(a.autoRefreshCountdownStop)
		a.autoRefreshCountdownStop = nil
	}
	a.autoRefreshCountdown = 0
	a.footer.UpdateAutoRefreshCountdown(0)
}

// autoRefreshDataWithFooter sets loading state and starts the data fetch in a new goroutine
func (a *App) autoRefreshDataWithFooter() {
	a.QueueUpdateDraw(func() {
		a.footer.SetLoading(true)
	})
	go a.autoRefreshData()
}

// autoRefreshData performs a lightweight refresh of performance data
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
