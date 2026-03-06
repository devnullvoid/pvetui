package components

import (
	"context"
	"fmt"
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// preserveVMEnrichmentData carries forward guest agent and config data from the last
// full refresh onto a freshly-fetched VM that only has basic metrics.
// existingVMs is keyed by "ID|Node" (single-profile) or "ID|Node|SourceProfile" (group mode).
func preserveVMEnrichmentData(fresh *api.VM, existingVMs map[string]*api.VM) {
	if fresh == nil {
		return
	}
	// Try group mode key first (includes SourceProfile), fall back to single-profile key.
	key := fmt.Sprintf("%d|%s|%s", fresh.ID, fresh.Node, fresh.SourceProfile)
	existing, ok := existingVMs[key]
	if !ok {
		key = fmt.Sprintf("%d|%s", fresh.ID, fresh.Node)
		existing, ok = existingVMs[key]
	}
	if !ok {
		return
	}

	// Preserve guest agent data (only available from full enrichment)
	if fresh.IP == "" {
		fresh.IP = existing.IP
	}
	fresh.AgentEnabled = existing.AgentEnabled
	fresh.AgentRunning = existing.AgentRunning
	if len(fresh.NetInterfaces) == 0 {
		fresh.NetInterfaces = existing.NetInterfaces
	}
	if len(fresh.Filesystems) == 0 {
		fresh.Filesystems = existing.Filesystems
	}
	fresh.ConfiguredMACs = existing.ConfiguredMACs

	// Preserve config details (from config endpoint enrichment)
	if len(fresh.ConfiguredNetworks) == 0 {
		fresh.ConfiguredNetworks = existing.ConfiguredNetworks
	}
	if len(fresh.StorageDevices) == 0 {
		fresh.StorageDevices = existing.StorageDevices
	}
	if fresh.BootOrder == "" {
		fresh.BootOrder = existing.BootOrder
	}
	if fresh.CPUCores == 0 {
		fresh.CPUCores = existing.CPUCores
	}
	if fresh.CPUSockets == 0 {
		fresh.CPUSockets = existing.CPUSockets
	}
	if fresh.Architecture == "" {
		fresh.Architecture = existing.Architecture
	}
	if fresh.OSType == "" {
		fresh.OSType = existing.OSType
	}
	if fresh.Description == "" {
		fresh.Description = existing.Description
	}
	fresh.OnBoot = existing.OnBoot
	fresh.Enriched = existing.Enriched
}

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

			// Preserve VM guest agent / config data from last full refresh.
			existingVMMap := make(map[string]*api.VM, len(models.GlobalState.OriginalVMs))
			for _, vm := range models.GlobalState.OriginalVMs {
				if vm != nil {
					key := fmt.Sprintf("%d|%s|%s", vm.ID, vm.Node, vm.SourceProfile)
					existingVMMap[key] = vm
				}
			}
			for _, vm := range vms {
				if vm != nil {
					preserveVMEnrichmentData(vm, existingVMMap)
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

			// Apply filters and update UI lists immediately (no background enrichment).
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

			// Update cluster status with the fresh (metric-only) data
			syntheticCluster := a.createSyntheticGroup(nodes)
			a.clusterStatus.Update(syntheticCluster)

			// Update details if items are selected
			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
			}
			if vm := a.vmList.GetSelectedVM(); vm != nil {
				a.vmDetails.Update(vm)
			}

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

			// Auto-refresh is lightweight — no background enrichment, release guard now.
			a.header.ShowSuccess("Data refreshed successfully")
			a.footer.SetLoading(false)
			a.endRefresh(token)

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

		// Build lookup map for O(N) VM enrichment preservation instead of O(N*M).
		// Light cluster status returns only basic metrics (CPU, memory, status);
		// guest agent data (IPs, filesystems, configs) must be carried forward
		// from the last full refresh to avoid "clearing" data on auto-refresh.
		existingVMMap := make(map[string]*api.VM, len(models.GlobalState.OriginalVMs))
		for _, vm := range models.GlobalState.OriginalVMs {
			if vm != nil {
				key := fmt.Sprintf("%d|%s", vm.ID, vm.Node)
				existingVMMap[key] = vm
			}
		}

		// Rebuild VM list from fresh cluster data
		var vms []*api.VM

		for _, node := range cluster.Nodes {
			if node != nil {
				for _, vm := range node.VMs {
					if vm != nil {
						preserveVMEnrichmentData(vm, existingVMMap)
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
