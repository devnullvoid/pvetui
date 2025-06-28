package components

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/adapters"
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/vnc"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// App is the main application component
type App struct {
	*tview.Application
	client        *api.Client
	config        config.Config
	vncService    *vnc.Service
	pages         *tview.Pages
	header        *Header
	footer        *Footer
	nodeList      *NodeList
	vmList        *VMList
	nodeDetails   *NodeDetails
	vmDetails     *VMDetails
	tasksList     *TasksList
	clusterStatus *ClusterStatus
	helpModal     *HelpModal
	mainLayout    *tview.Flex
	searchInput   *tview.InputField
	contextMenu   *tview.List
	isMenuOpen    bool
	lastFocus     tview.Primitive

	// Auto-refresh functionality
	autoRefreshEnabled       bool
	autoRefreshTicker        *time.Ticker
	autoRefreshStop          chan bool
	autoRefreshCountdown     int
	autoRefreshCountdownStop chan bool
}

// NewApp creates a new application instance with all UI components
func NewApp(client *api.Client, cfg *config.Config) *App {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Creating new App instance")

	// Get the shared logger for VNC service
	sharedLogger := models.GetUILogger()

	// Convert interface to concrete logger type for VNC service
	var vncLogger *logger.Logger
	if loggerAdapter, ok := sharedLogger.(*adapters.LoggerAdapter); ok {
		vncLogger = loggerAdapter.GetInternalLogger()
	}

	app := &App{
		Application: tview.NewApplication(),
		client:      client,
		config:      *cfg,
		vncService:  vnc.NewServiceWithLogger(client, vncLogger),
		pages:       tview.NewPages(),
	}

	uiLogger.Debug("Initializing UI components")

	// Initialize components
	app.header = NewHeader()
	app.footer = NewFooter()
	app.nodeList = NewNodeList()
	app.vmList = NewVMList()
	app.nodeDetails = NewNodeDetails()
	app.vmDetails = NewVMDetails()
	app.tasksList = NewTasksList()
	app.clusterStatus = NewClusterStatus()
	app.helpModal = NewHelpModal()

	// Set app reference for components that need it
	app.header.SetApp(app.Application)

	uiLogger.Debug("Loading initial cluster data")

	// Show loading indicator for guest data enrichment
	app.header.ShowLoading("Loading guest agent data")

	// Load initial data with error handling
	if _, err := client.FastGetClusterStatus(func() {
		// This callback is called when background VM enrichment completes
		uiLogger.Debug("VM enrichment callback triggered")
		app.QueueUpdateDraw(func() {
			uiLogger.Debug("Processing enriched VM data")

			// Store current VM selection to preserve user's position
			var selectedVMID int
			var selectedVMNode string
			var hasSelectedVM bool

			if selectedVM := app.vmList.GetSelectedVM(); selectedVM != nil {
				selectedVMID = selectedVM.ID
				selectedVMNode = selectedVM.Node
				hasSelectedVM = true
				uiLogger.Debug("Preserving selection for VM %d (%s) on node %s", selectedVMID, selectedVM.Name, selectedVMNode)
			}

			// Update the cluster status display
			if client.Cluster != nil {
				uiLogger.Debug("Updating cluster status with %d nodes", len(client.Cluster.Nodes))
				app.clusterStatus.Update(client.Cluster)
			}

			// Rebuild VM list from enriched cluster data
			var enrichedVMs []*api.VM
			if client.Cluster != nil {
				for _, node := range client.Cluster.Nodes {
					if node != nil {
						for _, vm := range node.VMs {
							if vm != nil {
								enrichedVMs = append(enrichedVMs, vm)
							}
						}
					}
				}
			}

			uiLogger.Debug("Found %d enriched VMs", len(enrichedVMs))

			// Update global state with enriched VM data
			if len(enrichedVMs) > 0 {
				models.GlobalState.OriginalVMs = make([]*api.VM, len(enrichedVMs))
				models.GlobalState.FilteredVMs = make([]*api.VM, len(enrichedVMs))
				copy(models.GlobalState.OriginalVMs, enrichedVMs)
				copy(models.GlobalState.FilteredVMs, enrichedVMs)

				// Update the VM list display
				app.vmList.SetVMs(models.GlobalState.FilteredVMs)
				uiLogger.Debug("Updated VM list with enriched data")

				// Restore the user's VM selection if they had one
				if hasSelectedVM {
					// Get the VM list's internal sorted slice, not the global unsorted one
					vmList := app.vmList.GetVMs()
					uiLogger.Debug("Attempting to restore selection for VM %d on node %s among %d VMs", selectedVMID, selectedVMNode, len(vmList))
					found := false
					for i, vm := range vmList {
						if vm != nil {
							uiLogger.Debug("Checking VM at index %d: ID=%d, Name=%s, Node=%s", i, vm.ID, vm.Name, vm.Node)
							if vm.ID == selectedVMID && vm.Node == selectedVMNode {
								app.vmList.SetCurrentItem(i)
								uiLogger.Debug("MATCH FOUND: Restored selection to VM %d (%s) on node %s at index %d", selectedVMID, vm.Name, selectedVMNode, i)

								// Verify what's actually selected after SetCurrentItem
								currentIndex := app.vmList.GetCurrentItem()
								actualSelected := app.vmList.GetSelectedVM()
								if actualSelected != nil {
									uiLogger.Debug("VERIFICATION: Current index is %d, selected VM is %d (%s) on node %s", currentIndex, actualSelected.ID, actualSelected.Name, actualSelected.Node)
								} else {
									uiLogger.Debug("VERIFICATION: Current index is %d, but GetSelectedVM returned nil", currentIndex)
								}

								found = true
								break
							}
						}
					}
					if !found {
						uiLogger.Debug("WARNING: No matching VM found for ID=%d, Node=%s. Selection will remain at default position.", selectedVMID, selectedVMNode)
					}
				}
			}

			// Refresh the currently selected VM details if there is one
			if selectedVM := app.vmList.GetSelectedVM(); selectedVM != nil {
				uiLogger.Debug("Refreshing details for selected VM: %s", selectedVM.Name)
				// Find the enriched version of the selected VM
				for _, enrichedVM := range enrichedVMs {
					if enrichedVM.ID == selectedVM.ID && enrichedVM.Node == selectedVM.Node {
						app.vmDetails.Update(enrichedVM)
						break
					}
				}
			}

			// Stop the loading indicator and show success notification
			app.header.StopLoading()
			app.header.ShowSuccess("Guest agent data loaded")
			uiLogger.Debug("VM enrichment completed successfully")
		})
	}); err != nil {
		uiLogger.Error("Failed to load cluster status: %v", err)
		app.header.StopLoading()
		app.header.ShowError("Failed to connect to Proxmox API: " + err.Error())
		// Continue with empty state rather than crashing
	}

	uiLogger.Debug("Initializing VM list from cluster data")

	// Initialize VM list from all nodes
	var vms []*api.VM
	if client.Cluster != nil {
		for _, node := range client.Cluster.Nodes {
			if node != nil {
				for _, vm := range node.VMs {
					if vm != nil {
						vms = append(vms, vm)
					}
				}
			}
		}
	}

	uiLogger.Debug("Found %d VMs across all nodes", len(vms))

	models.GlobalState = models.State{
		SearchStates:  make(map[string]*models.SearchState),
		OriginalNodes: make([]*api.Node, 0),
		FilteredNodes: make([]*api.Node, 0),
		OriginalVMs:   make([]*api.VM, len(vms)),
		FilteredVMs:   make([]*api.VM, len(vms)),
	}

	if client.Cluster != nil {
		uiLogger.Debug("Initializing node state with %d nodes", len(client.Cluster.Nodes))
		models.GlobalState.OriginalNodes = make([]*api.Node, len(client.Cluster.Nodes))
		models.GlobalState.FilteredNodes = make([]*api.Node, len(client.Cluster.Nodes))
		copy(models.GlobalState.OriginalNodes, client.Cluster.Nodes)
		copy(models.GlobalState.FilteredNodes, client.Cluster.Nodes)
	}
	copy(models.GlobalState.OriginalVMs, vms)
	copy(models.GlobalState.FilteredVMs, vms)

	uiLogger.Debug("Setting up component connections")

	// Set up component connections
	app.setupComponentConnections()

	// Configure root layout
	app.mainLayout = app.createMainLayout()

	// Register keyboard handlers
	app.setupKeyboardHandlers()

	// Set the root and focus
	app.SetRoot(app.mainLayout, true)
	app.SetFocus(app.nodeList)

	// Start VNC session monitoring
	app.startVNCSessionMonitoring()

	// Register callback for immediate session count updates
	app.registerVNCSessionCallback()

	uiLogger.Debug("App initialization completed successfully")

	return app
}

// GetVNCService returns the VNC service instance
func (a *App) GetVNCService() *vnc.Service {
	return a.vncService
}

// startVNCSessionMonitoring starts a background goroutine to monitor and update VNC session count
func (a *App) startVNCSessionMonitoring() {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Starting VNC session monitoring")

	go func() {
		ticker := time.NewTicker(5 * time.Second) // Reduced from 30 seconds to 5 seconds as backup
		defer ticker.Stop()

		lastSessionCount := -1 // Track last count to only log changes

		for {
			<-ticker.C
			// Get current session count
			sessionCount := a.vncService.GetActiveSessionCount()

			// Update footer with session count
			a.QueueUpdateDraw(func() {
				a.footer.UpdateVNCSessionCount(sessionCount)
			})

			// Only log when session count changes
			if sessionCount != lastSessionCount {
				uiLogger.Debug("VNC session count changed (polling): %d -> %d", lastSessionCount, sessionCount)
				lastSessionCount = sessionCount
			}

			// Clean up inactive sessions (older than 30 minutes) - but don't log every time
			a.vncService.CleanupInactiveSessions(30 * time.Minute)
		}
	}()
}

// registerVNCSessionCallback registers a callback for immediate VNC session count updates
func (a *App) registerVNCSessionCallback() {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Registering VNC session count callback for immediate updates")

	a.vncService.SetSessionCountCallback(func(count int) {
		uiLogger.Debug("VNC session count changed (callback): %d", count)

		// Update the UI immediately
		a.QueueUpdateDraw(func() {
			a.footer.UpdateVNCSessionCount(count)
		})
	})
}

// Run starts the application
func (a *App) Run() error {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Starting application")

	// We're disabling automatic background refresh to prevent UI issues
	// The user can manually refresh with a key if needed

	// Start the app
	err := a.Application.Run()
	if err != nil {
		uiLogger.Error("Application run failed: %v", err)
	} else {
		uiLogger.Debug("Application stopped normally")
		// Clean up auto-refresh
		a.stopAutoRefresh()
		// Clean up VNC sessions on exit
		uiLogger.Debug("Cleaning up VNC sessions on application exit")
		if closeErr := a.vncService.CloseAllSessions(); closeErr != nil {
			uiLogger.Error("Failed to close VNC sessions on exit: %v", closeErr)
		}
	}
	return err
}

// toggleAutoRefresh toggles the auto-refresh functionality on/off
func (a *App) toggleAutoRefresh() {
	uiLogger := models.GetUILogger()

	if a.autoRefreshEnabled {
		// Disable auto-refresh
		a.stopAutoRefresh()
		a.footer.UpdateAutoRefreshStatus(false)
		a.header.ShowSuccess("Auto-refresh disabled")
		uiLogger.Debug("Auto-refresh disabled by user")
	} else {
		// Enable auto-refresh
		a.startAutoRefresh()
		a.footer.UpdateAutoRefreshStatus(true)
		a.header.ShowSuccess("Auto-refresh enabled (10s interval)")
		uiLogger.Debug("Auto-refresh enabled by user")
	}
}

// startAutoRefresh starts the auto-refresh timer
func (a *App) startAutoRefresh() {
	if a.autoRefreshEnabled {
		return // Already running
	}

	a.autoRefreshEnabled = true
	a.autoRefreshStop = make(chan bool, 1)
	a.autoRefreshTicker = time.NewTicker(10 * time.Second) // 10 second interval
	a.autoRefreshCountdown = 10
	a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
	a.autoRefreshCountdownStop = make(chan bool, 1)

	// Start countdown goroutine
	go func() {
		for {
			select {
			case <-a.autoRefreshCountdownStop:
				return
			default:
				time.Sleep(1 * time.Second)
				if !a.autoRefreshEnabled {
					return
				}
				if a.footer.isLoading {
					continue // Pause countdown while loading
				}
				a.autoRefreshCountdown--
				if a.autoRefreshCountdown < 0 {
					a.autoRefreshCountdown = 0
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
			time.Sleep(100 * time.Millisecond)
			if !a.autoRefreshEnabled {
				return
			}
			if a.footer.isLoading {
				a.QueueUpdateDraw(func() {
					a.footer.spinnerIndex++
					a.footer.updateDisplay()
				})
			}
		}
	}()

	go func() {
		uiLogger := models.GetUILogger()
		uiLogger.Debug("Auto-refresh goroutine started")

		for {
			select {
			case <-a.autoRefreshStop:
				uiLogger.Debug("Auto-refresh stopped")
				if a.autoRefreshCountdownStop != nil {
					close(a.autoRefreshCountdownStop)
					a.autoRefreshCountdownStop = nil
				}
				return
			case <-a.autoRefreshTicker.C:
				// Only refresh if not currently loading something
				if !a.header.isLoading {
					uiLogger.Debug("Auto-refresh triggered")
					go a.autoRefreshDataWithFooter()
					a.autoRefreshCountdown = 10
					a.QueueUpdateDraw(func() {
						a.footer.UpdateAutoRefreshCountdown(a.autoRefreshCountdown)
					})
				} else {
					uiLogger.Debug("Auto-refresh skipped - operation in progress")
				}
			}
		}
	}()
}

// stopAutoRefresh stops the auto-refresh timer
func (a *App) stopAutoRefresh() {
	if !a.autoRefreshEnabled {
		return // Already stopped
	}

	a.autoRefreshEnabled = false

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
		nodeSearchState := models.GlobalState.GetSearchState("nodes")
		vmSearchState := models.GlobalState.GetSearchState("vms")

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

		// Restore VM selection
		if hasSelectedVM {
			vmList := a.vmList.GetVMs()
			for i, vm := range vmList {
				if vm != nil && vm.ID == selectedVMID && vm.Node == selectedVMNode {
					a.vmList.SetCurrentItem(i)
					if vmSearchState != nil {
						vmSearchState.SelectedIndex = i
					}
					break
				}
			}
		}

		// Restore node selection
		if hasSelectedNode {
			nodeList := a.nodeList.GetNodes()
			for i, node := range nodeList {
				if node != nil && node.Name == selectedNodeName {
					a.nodeList.SetCurrentItem(i)
					if nodeSearchState != nil {
						nodeSearchState.SelectedIndex = i
					}
					break
				}
			}
		}

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

		// Show success message
		a.header.ShowSuccess("Data refreshed successfully")
		a.footer.SetLoading(false)
	})
}
