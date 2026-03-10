package components

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// selSnap is a snapshot of the user's current selection state in the UI.
// It must be captured on the tview UI goroutine (inside a QueueUpdateDraw callback
// or before any goroutine is spawned when the caller is already on the UI goroutine).
type selSnap struct {
	hasSelectedNode  bool
	selectedNodeName string
	hasSelectedVM    bool
	selectedVMID     int
	selectedVMNode   string
	searchWasActive  bool
}

// captureSelections captures the user's current UI selections.
// MUST be called from the tview UI goroutine (inside QueueUpdateDraw or equivalent).
func (a *App) captureSelections() selSnap {
	var s selSnap
	if node := a.nodeList.GetSelectedNode(); node != nil {
		s.hasSelectedNode = true
		s.selectedNodeName = node.Name
	}
	if vm := a.vmList.GetSelectedVM(); vm != nil {
		s.hasSelectedVM = true
		s.selectedVMID = vm.ID
		s.selectedVMNode = vm.Node
	}
	s.searchWasActive = a.mainLayout.GetItemCount() > 4
	return s
}

// manualRefresh refreshes all data manually.
// Acquires the refresh guard and delegates to doManualRefresh.
func (a *App) manualRefresh() {
	token, ok := a.startRefresh()
	if !ok {
		return
	}
	a.doManualRefresh(token)
}

// doManualRefresh is the internal implementation of a full refresh.
// Caller must have already acquired the refresh guard (token != 0).
func (a *App) doManualRefresh(token uint64) {
	// Check if there are any pending operations
	if models.GlobalState.HasPendingOperations() {
		a.endRefresh(token)
		a.showMessageSafe("Cannot refresh data while there are pending operations in progress")
		return
	}

	// Show loading indicator
	a.header.ShowLoading("Refreshing data...")
	a.footer.SetLoading(true)

	// NOTE: UI state reads (selections, search) are intentionally NOT captured here.
	// For the group mode path, they are captured inside QueueUpdateDraw (on the UI
	// goroutine) before calling enrichGroupNodesParallel. For the single-profile path,
	// doEnrichNodes captures them inside its own QueueUpdateDraw callback. This ensures
	// all tview reads happen on the UI goroutine and are race-free.

	// Run data refresh in goroutine to avoid blocking UI
	go func() {
		// Snapshot connection state under lock so reads are race-free.
		conn := a.snapConn()

		if conn.isGroupMode {
			// Group mode logic
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			nodes, vms, err := conn.groupManager.GetGroupClusterResources(ctx, true)
			if err != nil {
				a.endRefresh(token)
				a.QueueUpdateDraw(func() {
					a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
					a.footer.SetLoading(false)
				})

				return
			}

			a.QueueUpdateDraw(func() {
				// Capture selections on the UI goroutine before starting background enrichment.
				snap := a.captureSelections()

				// Update GlobalState nodes/VMs; UI lists will be updated after enrichment to reduce flicker.
				models.GlobalState.OriginalNodes = nodes
				models.GlobalState.OriginalVMs = vms

				// Apply node filter if active
				nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
				if nodeSearchState != nil && nodeSearchState.Filter != "" {
					models.FilterNodes(nodeSearchState.Filter)
				} else {
					models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
					copy(models.GlobalState.FilteredNodes, nodes)
				}

				// Apply VM filter if active
				vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
				if vmSearchState != nil && vmSearchState.HasActiveVMFilter() {
					models.FilterVMs(vmSearchState.Filter)
				} else {
					models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
					copy(models.GlobalState.FilteredVMs, vms)
				}

				// Start background enrichment for detailed node stats
				// Pass false for isInitialLoad since this is a manual refresh
				a.enrichGroupNodesParallel(token, nodes,
					snap.hasSelectedNode, snap.selectedNodeName,
					snap.hasSelectedVM, snap.selectedVMID, snap.selectedVMNode,
					snap.searchWasActive, false)
			})
		} else {
			// Single profile logic
			// Fetch fresh data bypassing cache
			cluster, err := conn.client.GetFreshClusterStatus()
			if err != nil {
				a.endRefresh(token)
				a.QueueUpdateDraw(func() {
					a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
					a.footer.SetLoading(false)
				})

				return
			}

			// Cluster resources often omit node version fields. Keep the last known
			// version during the initial refresh update to avoid brief UI flicker.
			a.preserveClusterVersionIfMissing(cluster)

			// Initial UI update and enrichment. Selections are captured inside
			// doEnrichNodes' QueueUpdateDraw callback on the UI goroutine.
			a.applyInitialClusterUpdate(cluster)
			// Manual refresh: show "Data refreshed successfully" after node enrichment completes
			// (no VM enrichment callback follows in this path).
			a.doEnrichNodes(cluster, conn.client, token, true)
		}
	}()
}

// doFastRefresh is the implementation of fast refresh.
// Callers that already hold a generation token (e.g. profile switches via forceNewRefresh)
// should call this directly instead of acquiring a new token first.
// This is used for profile switching where perceived speed matters most —
// the user sees node and guest lists right away, then details (CPU, filesystems, guest agent)
// fill in progressively.
func (a *App) doFastRefresh(token uint64) {
	if models.GlobalState.HasPendingOperations() {
		a.endRefresh(token)
		a.showMessageSafe("Cannot refresh data while there are pending operations in progress")
		return
	}

	a.header.ShowLoading("Loading data...")
	a.footer.SetLoading(true)

	go func() {
		// Snapshot connection state under lock so reads are race-free.
		conn := a.snapConn()

		if conn.isGroupMode {
			// Group mode: delegate to manual refresh internal logic (already shows data before enrichment).
			// We already hold the refresh token, so pass it through directly.
			a.doManualRefresh(token)
			return
		}

		// Capture the client instance that is starting this refresh.
		// Used for making API calls with the correct client during enrichment.
		refreshClient := conn.client

		// Single profile: get basic data fast (no VM enrichment), show it, then enrich in background
		cluster, err := refreshClient.GetFastFreshClusterStatus(func(enrichErr error) {
			// VM enrichment complete callback — update VM list with enriched data
			a.QueueUpdateDraw(func() {
				// Stale guard: abort only if a *newer* refresh has superseded this one.
				// refreshGen == 0 means this refresh completed normally via endRefresh —
				// we still need to run to clear the header loading indicator and update
				// VM details (doEnrichNodes calls endRefresh before this callback fires).
				if current := a.refreshGen.Load(); current != 0 && current != token {
					return
				}

				if refreshClient.Cluster == nil {
					return
				}

				// Rebuild VM list from enriched cluster data
				var enrichedVMs []*api.VM
				for _, node := range refreshClient.Cluster.Nodes {
					if node != nil {
						for _, vm := range node.VMs {
							if vm != nil {
								enrichedVMs = append(enrichedVMs, vm)
							}
						}
					}
				}

				if len(enrichedVMs) > 0 {
					// Preserve current VM selection
					var selectedVMID int
					var selectedVMNode string
					var hasSelectedVM bool
					if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
						selectedVMID = selectedVM.ID
						selectedVMNode = selectedVM.Node
						hasSelectedVM = true
					}

					models.GlobalState.OriginalVMs = make([]*api.VM, len(enrichedVMs))
					copy(models.GlobalState.OriginalVMs, enrichedVMs)

					vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
					if vmSearchState != nil && vmSearchState.HasActiveVMFilter() {
						models.FilterVMs(vmSearchState.Filter)
						a.vmList.SetVMs(models.GlobalState.FilteredVMs)
					} else {
						models.GlobalState.FilteredVMs = make([]*api.VM, len(enrichedVMs))
						copy(models.GlobalState.FilteredVMs, enrichedVMs)
						a.vmList.SetVMs(models.GlobalState.FilteredVMs)
					}

					// Restore VM selection
					if hasSelectedVM {
						vmList := a.vmList.GetVMs()
						for i, vm := range vmList {
							if vm != nil && vm.ID == selectedVMID && vm.Node == selectedVMNode {
								a.vmList.SetCurrentItem(i)
								break
							}
						}
					}

					if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
						a.vmDetails.Update(selectedVM)
					}
				}

				if enrichErr != nil {
					a.header.ShowWarning("Guest agent enrichment partially failed")
				} else {
					a.header.ShowSuccess("Guest agent data loaded")
				}
			})
		})
		if err != nil {
			a.endRefresh(token)
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
				a.footer.SetLoading(false)
			})
			return
		}

		// We have basic cluster data — show it immediately
		a.preserveClusterVersionIfMissing(cluster)
		a.applyInitialClusterUpdate(cluster)

		// Start node enrichment in background (Version, disks, updates).
		// Selections are captured inside doEnrichNodes' QueueUpdateDraw on the UI goroutine.
		// Fast refresh path: VM enrichment callback will show the final message, not this.
		a.doEnrichNodes(cluster, refreshClient, token, false)
	}()
}

func (a *App) preserveClusterVersionIfMissing(cluster *api.Cluster) {
	if cluster == nil || cluster.Version != "" {
		return
	}
	for _, existingNode := range models.GlobalState.OriginalNodes {
		if existingNode != nil && existingNode.Version != "" {
			cluster.Version = fmt.Sprintf("Proxmox VE %s", existingNode.Version)
			return
		}
	}
}

// applyInitialClusterUpdate updates global state and UI with basic cluster data and rebuilt VM list
func (a *App) applyInitialClusterUpdate(cluster *api.Cluster) {
	a.QueueUpdateDraw(func() {
		// Update global state nodes from cluster resources
		models.GlobalState.OriginalNodes = make([]*api.Node, len(cluster.Nodes))
		copy(models.GlobalState.OriginalNodes, cluster.Nodes)

		// Apply node filter if active
		if nodeState := models.GlobalState.GetSearchState(api.PageNodes); nodeState != nil && nodeState.Filter != "" {
			models.FilterNodes(nodeState.Filter)
		} else {
			models.GlobalState.FilteredNodes = make([]*api.Node, len(cluster.Nodes))
			copy(models.GlobalState.FilteredNodes, cluster.Nodes)
		}
		a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
		a.syncStorageBrowserNodes()

		// Rebuild VM list from fresh cluster resources so new guests appear immediately
		var vms []*api.VM
		for _, n := range cluster.Nodes {
			if n != nil {
				for _, vm := range n.VMs {
					if vm != nil {
						vms = append(vms, vm)
					}
				}
			}
		}
		models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
		copy(models.GlobalState.OriginalVMs, vms)

		// Apply VM filter if active
		if vmState := models.GlobalState.GetSearchState(api.PageGuests); vmState != nil && vmState.HasActiveVMFilter() {
			models.FilterVMs(vmState.Filter)
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		} else {
			models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
			copy(models.GlobalState.FilteredVMs, vms)
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
		}

		// Update cluster summary/status
		a.clusterStatus.Update(cluster)
	})
}

// doEnrichNodes enriches node data in parallel and finalizes the refresh.
// Selections are always captured inside the final QueueUpdateDraw callback (on the UI
// goroutine), ensuring race-free reads from tview widgets.
// The refreshClient parameter is the client used for API calls during enrichment.
// The token parameter is the generation token acquired at refresh start; if a newer
// refresh has started by the time enrichment completes, the callback is a no-op.
// When showFinalMessage is true, a "Data refreshed successfully" message is shown after
// enrichment completes. Set to false for the doFastRefresh path where the VM enrichment
// callback will show the final status message.
func (a *App) doEnrichNodes(cluster *api.Cluster, refreshClient *api.Client, token uint64, showFinalMessage bool) {
	go func() {
		var wg sync.WaitGroup

		// Start loading tasks in parallel with node enrichment (independent operation)
		a.loadTasksData()

		// Accumulate enriched nodes into a local slice, then swap into GlobalState
		// inside QueueUpdateDraw to avoid data races with the UI goroutine.
		enrichedNodes := make([]*api.Node, len(cluster.Nodes))
		copy(enrichedNodes, cluster.Nodes)

		for i, node := range cluster.Nodes {
			if node == nil {
				continue
			}

			wg.Add(1)
			go func(idx int, n *api.Node) {
				defer wg.Done()

				freshNode, err := refreshClient.RefreshNodeData(n.Name)
				if err == nil && freshNode != nil {
					// Preserve VMs from the FRESH cluster data
					if cluster.Nodes[idx] != nil {
						freshNode.VMs = cluster.Nodes[idx].VMs
					}
					enrichedNodes[idx] = freshNode
				}
			}(i, node)
		}

		wg.Wait()

		// Final update on the UI goroutine
		a.QueueUpdateDraw(func() {
			// Stale guard: if a newer refresh has started (e.g. profile switch), discard these results.
			// endRefresh is a no-op here since the token no longer matches.
			if a.refreshGen.Load() != token {
				return
			}

			// Capture selections on the UI goroutine (race-free tview access).
			snap := a.captureSelections()

			// Apply enriched nodes to global state
			models.GlobalState.OriginalNodes = make([]*api.Node, len(enrichedNodes))
			copy(models.GlobalState.OriginalNodes, enrichedNodes)

			// Re-apply node filters with enriched data
			nodeState := models.GlobalState.GetSearchState(api.PageNodes)
			if nodeState != nil && nodeState.Filter != "" {
				models.FilterNodes(nodeState.Filter)
			} else {
				models.GlobalState.FilteredNodes = make([]*api.Node, len(enrichedNodes))
				copy(models.GlobalState.FilteredNodes, enrichedNodes)
			}
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			a.syncStorageBrowserNodes()

			// Rebuild VM list from enriched nodes
			var vms []*api.VM
			for _, n := range enrichedNodes {
				if n != nil {
					for _, vm := range n.VMs {
						if vm != nil {
							vms = append(vms, vm)
						}
					}
				}
			}

			models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
			copy(models.GlobalState.OriginalVMs, vms)

			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
			if vmSearchState != nil && vmSearchState.HasActiveVMFilter() {
				models.FilterVMs(vmSearchState.Filter)
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			} else {
				models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
				copy(models.GlobalState.FilteredVMs, vms)
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			}

			// Update cluster version from enriched nodes
			for _, n := range enrichedNodes {
				if n != nil && n.Version != "" {
					cluster.Version = fmt.Sprintf("Proxmox VE %s", n.Version)
					break
				}
			}
			a.clusterStatus.Update(cluster)

			// Restore selection (use vmSearchState already fetched above)
			nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
			a.restoreSelection(snap.hasSelectedVM, snap.selectedVMID, snap.selectedVMNode, vmSearchState,
				snap.hasSelectedNode, snap.selectedNodeName, nodeSearchState)

			// Update details for whatever is now selected
			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
			}
			if vm := a.vmList.GetSelectedVM(); vm != nil {
				a.vmDetails.Update(vm)
			}

			a.restoreSearchUI(snap.searchWasActive, nodeSearchState, vmSearchState)
			if showFinalMessage {
				// Manual refresh path: no VM enrichment callback follows, so show success here.
				a.header.ShowSuccess("Data refreshed successfully")
			}
			// Fast refresh path: the VM enrichment callback (in doFastRefresh) will show
			// the final status message after guest agent data is loaded.
			a.footer.SetLoading(false)
			a.endRefresh(token)
		})
	}()
}

// enrichGroupNodesParallel enriches group node data in parallel and finalizes the refresh.
// token is the generation token that must be passed to endRefresh when enrichment completes.
func (a *App) enrichGroupNodesParallel(token uint64, nodes []*api.Node, hasSelectedNode bool, selectedNodeName string, hasSelectedVM bool, selectedVMID int, selectedVMNode string, searchWasActive bool, isInitialLoad bool) {
	go func() {
		var wg sync.WaitGroup

		// Start loading tasks in parallel with node enrichment (independent operation)
		a.loadTasksData()

		// Snapshot connection state under lock so reads are race-free.
		conn := a.snapConn()

		// Accumulate enriched nodes into a local slice to avoid data races.
		// Concurrent goroutines write to their own index; the slice is swapped into
		// GlobalState inside QueueUpdateDraw after all goroutines complete.
		enrichedNodes := make([]*api.Node, len(nodes))
		copy(enrichedNodes, nodes)

		// Create a context for the enrichment process
		ctx := context.Background()

		// Enrich nodes in parallel
		for i, node := range nodes {
			if node == nil || node.SourceProfile == "" {
				continue
			}

			wg.Add(1)
			go func(idx int, n *api.Node) {
				defer wg.Done()

				// Fetch the node status from the specific profile's client
				freshNode, err := conn.groupManager.GetNodeFromGroup(ctx, n.SourceProfile, n.Name)

				if err == nil && freshNode != nil {
					// Ensure Online status is set to true if we got a response
					freshNode.Online = true

					// Preserve VMs from the existing node list
					if nodes[idx] != nil {
						freshNode.VMs = nodes[idx].VMs
						// Preserve IP if missing in freshNode
						if freshNode.IP == "" {
							freshNode.IP = nodes[idx].IP
						}
						// Preserve ID if missing in freshNode
						if freshNode.ID == "" {
							freshNode.ID = nodes[idx].ID
						}
						// Preserve Storage if missing in freshNode
						if freshNode.Storage == nil && nodes[idx].Storage != nil {
							freshNode.Storage = nodes[idx].Storage
						}
					}

					// Ensure SourceProfile is preserved
					freshNode.SourceProfile = n.SourceProfile

					// Write to local slice only — no GlobalState mutation from goroutines.
					enrichedNodes[idx] = freshNode
				}
			}(i, node)
		}

		wg.Wait()

		// Final update — swap enriched nodes into GlobalState on the UI goroutine.
		a.QueueUpdateDraw(func() {
			// Apply enriched nodes to global state (race-free: single UI goroutine).
			models.GlobalState.OriginalNodes = make([]*api.Node, len(enrichedNodes))
			copy(models.GlobalState.OriginalNodes, enrichedNodes)

			// Re-apply node filters
			nodeState := models.GlobalState.GetSearchState(api.PageNodes)
			if nodeState != nil && nodeState.Filter != "" {
				models.FilterNodes(nodeState.Filter)
			} else {
				models.GlobalState.FilteredNodes = make([]*api.Node, len(enrichedNodes))
				copy(models.GlobalState.FilteredNodes, enrichedNodes)
			}
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			a.syncStorageBrowserNodes()

			// Update cluster status with the enriched nodes
			syntheticCluster := a.createSyntheticGroup(enrichedNodes)
			a.clusterStatus.Update(syntheticCluster)

			// Final selection restore and search UI restoration
			nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
			vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)

			a.restoreSelection(hasSelectedVM, selectedVMID, selectedVMNode, vmSearchState,
				hasSelectedNode, selectedNodeName, nodeSearchState)

			// Update details if items are selected
			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
			}

			if vm := a.vmList.GetSelectedVM(); vm != nil {
				a.vmDetails.Update(vm)
			}

			a.restoreSearchUI(searchWasActive, nodeSearchState, vmSearchState)
			// Update lists and cluster status after enrichment to minimize flicker
			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			a.syncStorageBrowserNodes()
			a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			a.clusterStatus.Update(a.getDisplayCluster())

			// Show appropriate success message based on context
			if isInitialLoad {
				a.header.ShowSuccess("Guest agent data loaded")
			} else {
				a.header.ShowSuccess("Data refreshed successfully")
			}
			a.footer.SetLoading(false)
			a.endRefresh(token)
		})
	}()
}

// refreshNodeData refreshes data for a specific node and updates the UI.
func (a *App) refreshNodeData(node *api.Node) {
	a.header.ShowLoading(fmt.Sprintf("Refreshing node %s", node.Name))
	// Record the currently selected node's name
	selectedNodeName := ""
	if selected := a.nodeList.GetSelectedNode(); selected != nil {
		selectedNodeName = selected.Name
	}

	go func() {
		var freshNode *api.Node
		var err error

		client, clientErr := a.getClientForNode(node)
		if clientErr != nil {
			err = clientErr
		} else {
			freshNode, err = client.RefreshNodeData(node.Name)
		}

		a.QueueUpdateDraw(func() {
			if err != nil {
				a.header.ShowError(fmt.Sprintf("Error refreshing node %s: %v", node.Name, err))

				return
			}
			// Update node in global state
			for i, n := range models.GlobalState.OriginalNodes {
				if n != nil && n.Name == node.Name {
					// In group mode, ensure SourceProfile is preserved/set
					if a.isGroupMode {
						freshNode.SourceProfile = node.SourceProfile
					}
					models.GlobalState.OriginalNodes[i] = freshNode
					break
				}
			}

			for i, n := range models.GlobalState.FilteredNodes {
				if n != nil && n.Name == node.Name {
					// In group mode, ensure SourceProfile is preserved/set
					if a.isGroupMode {
						freshNode.SourceProfile = node.SourceProfile
					}
					models.GlobalState.FilteredNodes[i] = freshNode
					break
				}
			}

			a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			a.syncStorageBrowserNodes()
			a.nodeDetails.Update(freshNode, models.GlobalState.OriginalNodes)
			// Restore selection by previously selected node name using the tview list data
			restored := false

			nodeList := a.nodeList.GetNodes()
			for i, n := range nodeList {
				if n != nil && n.Name == selectedNodeName {
					a.nodeList.SetCurrentItem(i)
					// Manually trigger the node changed callback to update details
					if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
						a.nodeDetails.Update(selectedNode, models.GlobalState.OriginalNodes)
					}

					restored = true

					break
				}
			}

			if !restored && len(nodeList) > 0 {
				a.nodeList.SetCurrentItem(0)
				// Manually trigger the node changed callback to update details
				if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
					a.nodeDetails.Update(selectedNode, models.GlobalState.OriginalNodes)
				}
			}

			a.header.ShowSuccess(fmt.Sprintf("Node %s refreshed successfully", node.Name))
		})
	}()
}

// loadTasksData loads and updates task data with proper filtering.
func (a *App) loadTasksData() {
	go func() {
		// Snapshot connection state under lock so reads are race-free.
		conn := a.snapConn()

		var tasks []*api.ClusterTask
		var err error

		if conn.isGroupMode {
			// Create context with timeout for group operations
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tasks, err = conn.groupManager.GetGroupTasks(ctx)
		} else {
			tasks, err = conn.client.GetClusterTasks()
		}

		if err == nil {
			a.QueueUpdateDraw(func() {
				// Update global state with tasks
				models.GlobalState.OriginalTasks = make([]*api.ClusterTask, len(tasks))
				models.GlobalState.FilteredTasks = make([]*api.ClusterTask, len(tasks))
				copy(models.GlobalState.OriginalTasks, tasks)
				copy(models.GlobalState.FilteredTasks, tasks)

				// Check for existing search filters
				taskSearchState := models.GlobalState.GetSearchState(api.PageTasks)
				if taskSearchState != nil && taskSearchState.Filter != "" {
					// Apply existing filter
					models.FilterTasks(taskSearchState.Filter)
					a.tasksList.SetFilteredTasks(models.GlobalState.FilteredTasks)
				} else {
					// No filter, use original data
					a.tasksList.SetTasks(tasks)
				}
			})
		}
	}()
}
