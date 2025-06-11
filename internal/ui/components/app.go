package components

import (
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// App is the main application component
type App struct {
	*tview.Application
	client          *api.Client
	config          config.Config
	pages           *tview.Pages
	header          *Header
	footer          *Footer
	nodeList        *NodeList
	vmList          *VMList
	nodeDetails     *NodeDetails
	vmDetails       *VMDetails
	clusterStatus   *ClusterStatus
	helpModal       *HelpModal
	mainLayout      *tview.Flex
	searchInput     *tview.InputField
	contextMenu     *tview.List
	isMenuOpen      bool
	lastFocus       tview.Primitive
	vncWarningShown bool // Track if VNC warning has been shown
}

// NewApp creates a new application instance with all UI components
func NewApp(client *api.Client, cfg *config.Config) *App {
	logger := models.GetUILogger()
	logger.Debug("Creating new App instance")

	app := &App{
		Application: tview.NewApplication(),
		client:      client,
		config:      *cfg,
		pages:       tview.NewPages(),
	}

	logger.Debug("Initializing UI components")

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

	logger.Debug("Loading initial cluster data")

	// Load initial data with error handling
	if _, err := client.FastGetClusterStatus(func() {
		// This callback is called when background VM enrichment completes
		logger.Debug("VM enrichment callback triggered")
		app.QueueUpdateDraw(func() {
			logger.Debug("Processing enriched VM data")

			// Update the cluster status display
			if client.Cluster != nil {
				logger.Debug("Updating cluster status with %d nodes", len(client.Cluster.Nodes))
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

			logger.Debug("Found %d enriched VMs", len(enrichedVMs))

			// Update global state with enriched VM data
			if len(enrichedVMs) > 0 {
				models.GlobalState.OriginalVMs = make([]*api.VM, len(enrichedVMs))
				models.GlobalState.FilteredVMs = make([]*api.VM, len(enrichedVMs))
				copy(models.GlobalState.OriginalVMs, enrichedVMs)
				copy(models.GlobalState.FilteredVMs, enrichedVMs)

				// Update the VM list display
				app.vmList.SetVMs(models.GlobalState.FilteredVMs)
				logger.Debug("Updated VM list with enriched data")
			}

			// Refresh the currently selected VM details if there is one
			if selectedVM := app.vmList.GetSelectedVM(); selectedVM != nil {
				logger.Debug("Refreshing details for selected VM: %s", selectedVM.Name)
				// Find the enriched version of the selected VM
				for _, enrichedVM := range enrichedVMs {
					if enrichedVM.ID == selectedVM.ID && enrichedVM.Node == selectedVM.Node {
						app.vmDetails.Update(enrichedVM)
						break
					}
				}
			}

			// Show a subtle notification that enrichment is complete
			app.header.ShowSuccess("Guest agent data loaded")
			logger.Debug("VM enrichment completed successfully")
		})
	}); err != nil {
		logger.Error("Failed to load cluster status: %v", err)
		app.header.ShowError("Failed to connect to Proxmox API: " + err.Error())
		// Continue with empty state rather than crashing
	}

	logger.Debug("Initializing VM list from cluster data")

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

	logger.Debug("Found %d VMs across all nodes", len(vms))

	models.GlobalState = models.State{
		SearchStates:  make(map[string]*models.SearchState),
		OriginalNodes: make([]*api.Node, 0),
		FilteredNodes: make([]*api.Node, 0),
		OriginalVMs:   make([]*api.VM, len(vms)),
		FilteredVMs:   make([]*api.VM, len(vms)),
	}

	if client.Cluster != nil {
		logger.Debug("Initializing node state with %d nodes", len(client.Cluster.Nodes))
		models.GlobalState.OriginalNodes = make([]*api.Node, len(client.Cluster.Nodes))
		models.GlobalState.FilteredNodes = make([]*api.Node, len(client.Cluster.Nodes))
		copy(models.GlobalState.OriginalNodes, client.Cluster.Nodes)
		copy(models.GlobalState.FilteredNodes, client.Cluster.Nodes)
	}
	copy(models.GlobalState.OriginalVMs, vms)
	copy(models.GlobalState.FilteredVMs, vms)

	logger.Debug("Setting up component connections")

	// Set up component connections
	app.setupComponentConnections()

	// Configure root layout
	app.mainLayout = app.createMainLayout()

	// Register keyboard handlers
	app.setupKeyboardHandlers()

	// Set the root and focus
	app.SetRoot(app.mainLayout, true)
	app.SetFocus(app.nodeList)

	logger.Debug("App initialization completed successfully")

	return app
}

// Run starts the application
func (a *App) Run() error {
	logger := models.GetUILogger()
	logger.Debug("Starting application")

	// We're disabling automatic background refresh to prevent UI issues
	// The user can manually refresh with a key if needed

	// Start the app
	err := a.Application.Run()
	if err != nil {
		logger.Error("Application run failed: %v", err)
	} else {
		logger.Debug("Application stopped normally")
	}
	return err
}
