package components

import (
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
	clusterStatus *ClusterStatus
	helpModal     *HelpModal
	mainLayout    *tview.Flex
	searchInput   *tview.InputField
	contextMenu   *tview.List
	isMenuOpen    bool
	lastFocus     tview.Primitive
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
			select {
			case <-ticker.C:
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
		// Clean up VNC sessions on exit
		uiLogger.Debug("Cleaning up VNC sessions on application exit")
		if closeErr := a.vncService.CloseAllSessions(); closeErr != nil {
			uiLogger.Error("Failed to close VNC sessions on exit: %v", closeErr)
		}
	}
	return err
}
