package components

import (
	"context"
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
	header        HeaderComponent
	footer        FooterComponent
	nodeList      NodeListComponent
	vmList        VMListComponent
	nodeDetails   NodeDetailsComponent
	vmDetails     VMDetailsComponent
	tasksList     TasksListComponent
	clusterStatus ClusterStatusComponent
	helpModal     *HelpModal
	mainLayout    *tview.Flex
	searchInput   *tview.InputField
	contextMenu   *tview.List
	isMenuOpen    bool
	lastFocus     tview.Primitive

	ctx    context.Context
	cancel context.CancelFunc

	// Auto-refresh functionality
	autoRefreshEnabled       bool
	autoRefreshTicker        *time.Ticker
	autoRefreshStop          chan bool
	autoRefreshCountdown     int
	autoRefreshCountdownStop chan bool
}

// NewApp creates a new application instance with all UI components
func NewApp(ctx context.Context, client *api.Client, cfg *config.Config) *App {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Creating new App instance")

	// Get the shared logger for VNC service
	sharedLogger := models.GetUILogger()

	// Convert interface to concrete logger type for VNC service
	var vncLogger *logger.Logger
	if loggerAdapter, ok := sharedLogger.(*adapters.LoggerAdapter); ok {
		vncLogger = loggerAdapter.GetInternalLogger()
	}

	ctx, cancel := context.WithCancel(ctx)
	app := &App{
		Application:        tview.NewApplication(),
		client:             client,
		config:             *cfg,
		vncService:         vnc.NewServiceWithLogger(client, vncLogger),
		pages:              tview.NewPages(),
		autoRefreshEnabled: false,
		ctx:                ctx,
		cancel:             cancel,
	}

	uiLogger.Debug("Initializing UI components")

	// Initialize components
	app.header = NewHeader()
	app.footer = NewFooter()
	app.footer.UpdateKeybindings(FormatFooterText(cfg.KeyBindings))
	app.nodeList = NewNodeList()
	app.vmList = NewVMList()
	app.nodeDetails = NewNodeDetails()
	app.vmDetails = NewVMDetails()
	app.tasksList = NewTasksList()
	app.clusterStatus = NewClusterStatus()
	app.helpModal = NewHelpModal(cfg.KeyBindings)

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
				copy(models.GlobalState.OriginalVMs, enrichedVMs)

				// Check if there's an active search filter and apply it
				vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
				if vmSearchState != nil && vmSearchState.Filter != "" {
					// Apply existing filter to the enriched data
					models.FilterVMs(vmSearchState.Filter)
					app.vmList.SetVMs(models.GlobalState.FilteredVMs)
					uiLogger.Debug("Updated VM list with enriched data and preserved filter: %s", vmSearchState.Filter)
				} else {
					// No filter, use original enriched data
					models.GlobalState.FilteredVMs = make([]*api.VM, len(enrichedVMs))
					copy(models.GlobalState.FilteredVMs, enrichedVMs)
					app.vmList.SetVMs(models.GlobalState.FilteredVMs)
					uiLogger.Debug("Updated VM list with enriched data (no filter)")
				}

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
		SearchStates:          make(map[string]*models.SearchState),
		OriginalNodes:         make([]*api.Node, 0),
		FilteredNodes:         make([]*api.Node, 0),
		OriginalVMs:           make([]*api.VM, len(vms)),
		FilteredVMs:           make([]*api.VM, len(vms)),
		OriginalTasks:         make([]*api.ClusterTask, 0),
		FilteredTasks:         make([]*api.ClusterTask, 0),
		PendingVMOperations:   make(map[string]string),
		PendingNodeOperations: make(map[string]string),
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

// Run starts the application
func (a *App) Run() error {
	uiLogger := models.GetUILogger()
	uiLogger.Debug("Starting application")

	a.startAutoRefresh()
	defer func() {
		a.stopAutoRefresh()
		a.cancel()
	}()

	if err := a.Application.Run(); err != nil {
		uiLogger.Error("Application run failed: %v", err)
		return err
	}

	uiLogger.Debug("Application stopped normally")
	// Clean up VNC sessions on exit
	uiLogger.Debug("Cleaning up VNC sessions on application exit")
	if closeErr := a.vncService.CloseAllSessions(); closeErr != nil {
		uiLogger.Error("Failed to close VNC sessions on exit: %v", closeErr)
	}
	return nil
}
