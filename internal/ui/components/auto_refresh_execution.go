package components

import (
	"context"
	"fmt"
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// autoRefreshDataWithFooter sets loading state, captures UI selections on the UI goroutine,
// and starts the data fetch in a new goroutine.
// Selection state must be read here (on the UI goroutine) before the goroutine is spawned.
func (a *App) autoRefreshDataWithFooter() {
	// Capture selections and set loading state while still on the UI goroutine.
	// QueueUpdateDraw executes synchronously when already on the UI goroutine (which
	// autoRefreshDataWithFooter always is, called from the ticker callback).
	var snap selSnap
	a.QueueUpdateDraw(func() {
		a.footer.SetLoading(true)
		snap = a.captureSelections()
	})

	go a.autoRefreshData(snap)
}

// autoRefreshData performs a lightweight refresh of performance data.
// snap contains the UI selections captured on the UI goroutine before this goroutine started.
func (a *App) autoRefreshData(snap selSnap) {
	// Acquire the refresh guard. The countdown check in startAutoRefresh already verified
	// isRefreshActive() == false, but we CAS here to prevent a race between that check
	// and the actual start of the refresh.
	token, ok := a.startRefresh()
	if !ok {
		a.QueueUpdateDraw(func() {
			a.footer.SetLoading(false)
		})
		return
	}

	uiLogger := models.GetUILogger()

	// Snapshot connection state under lock so reads are race-free.
	conn := a.snapConn()

	// Unpack UI selections captured on the UI goroutine before this goroutine was spawned.
	hasSelectedVM := snap.hasSelectedVM
	selectedVMID := snap.selectedVMID
	selectedVMNode := snap.selectedVMNode
	hasSelectedNode := snap.hasSelectedNode
	selectedNodeName := snap.selectedNodeName
	searchWasActive := snap.searchWasActive

	if conn.isGroupMode {
		// Group mode logic
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		nodes, vms, err := conn.groupManager.GetGroupClusterResources(ctx, true)
		if err != nil {
			uiLogger.Debug("Auto-refresh failed (group): %v", err)
			a.endRefresh(token)
			a.QueueUpdateDraw(func() {
				a.footer.SetLoading(false)
			})
			return
		}

		a.QueueUpdateDraw(func() {
			// Get current search states
			nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

			// Build lookup map for O(N) node detail preservation instead of O(N*M)
			existingNodeMap := make(map[string]*api.Node, len(models.GlobalState.OriginalNodes))
			for _, n := range models.GlobalState.OriginalNodes {
				if n != nil {
					existingNodeMap[n.Name+"|"+n.SourceProfile] = n
				}
			}

			// Preserve detailed node data using map lookup
			for _, freshNode := range nodes {
				if freshNode == nil {
					continue
				}

				if existing, ok := existingNodeMap[freshNode.Name+"|"+freshNode.SourceProfile]; ok {
					freshNode.Version = existing.Version
					freshNode.KernelVersion = existing.KernelVersion
					freshNode.CPUInfo = existing.CPUInfo
					freshNode.LoadAvg = existing.LoadAvg
					freshNode.CGroupMode = existing.CGroupMode
					freshNode.Level = existing.Level
					freshNode.Storage = existing.Storage
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
				groupMgr := conn.groupManager // capture under connMu snapshot, not a.groupManager
				go func() {
					ctxTasks, cancelTasks := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancelTasks()
					tasks, err := groupMgr.GetGroupTasks(ctxTasks)
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
			// Pass false for isInitialLoad since this is auto-refresh.
			// enrichGroupNodesParallel will call endRefresh(token) when done.
			a.enrichGroupNodesParallel(token, nodes, hasSelectedNode, selectedNodeName, hasSelectedVM, selectedVMID, selectedVMNode, searchWasActive, false)

			a.autoRefreshCountdown = 10

			a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)

		})

		return

	}

	// Fetch fresh cluster resources without VM enrichment (lighter for periodic refresh).
	// Guest agent data (IPs, filesystems) is preserved from the last full refresh.
	cluster, err := conn.client.GetLightClusterStatus()
	if err != nil {
		uiLogger.Debug("Auto-refresh failed: %v", err)
		a.endRefresh(token)
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

		// Build lookup map for O(N) node detail preservation instead of O(N*M)
		existingNodeMap := make(map[string]*api.Node, len(models.GlobalState.OriginalNodes))
		for _, n := range models.GlobalState.OriginalNodes {
			if n != nil {
				existingNodeMap[n.Name] = n
			}
		}

		// Preserve detailed node data while updating performance metrics
		for _, freshNode := range cluster.Nodes {
			if freshNode == nil {
				continue
			}

			if existing, ok := existingNodeMap[freshNode.Name]; ok {
				freshNode.Version = existing.Version
				freshNode.KernelVersion = existing.KernelVersion
				freshNode.CPUInfo = existing.CPUInfo
				freshNode.LoadAvg = existing.LoadAvg
				freshNode.CGroupMode = existing.CGroupMode
				freshNode.Level = existing.Level
				freshNode.Storage = existing.Storage
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

		if vmSearchState != nil && vmSearchState.HasActiveVMFilter() {
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
				tasks, err := conn.client.GetClusterTasks()
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
		a.endRefresh(token)

		// Reset countdown after refresh is complete
		a.autoRefreshCountdown = 10
		a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
	})
}
