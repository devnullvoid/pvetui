package components

import (
	"github.com/gdamore/tcell/v2"
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
	mainLayout      *tview.Flex
	searchInput     *tview.InputField
	contextMenu     *tview.List
	isMenuOpen      bool
	lastFocus       tview.Primitive
	vncWarningShown bool // Track if VNC warning has been shown
}

// NewApp creates a new application instance with all UI components
func NewApp(client *api.Client, cfg *config.Config) *App {
	app := &App{
		Application: tview.NewApplication(),
		client:      client,
		config:      *cfg,
	}

	// Set application theme and background color
	// tview.Styles.PrimitiveBackgroundColor = tcell.ColorBlack
	tview.Styles.ContrastBackgroundColor = tcell.ColorGray
	// tview.Styles.MoreContrastBackgroundColor = tcell.ColorDarkGray
	// tview.Styles.BorderColor = tcell.ColorWhite
	// tview.Styles.TitleColor = tcell.ColorWhite
	// tview.Styles.PrimaryTextColor = tcell.ColorWhite
	// tview.Styles.SecondaryTextColor = tcell.ColorYellow
	// tview.Styles.TertiaryTextColor = tcell.ColorGreen
	tview.Styles.InverseTextColor = tcell.ColorBlack
	// tview.Styles.ContrastSecondaryTextColor = tcell.ColorGray

	// Create UI components
	app.header = NewHeader()
	app.header.SetApp(app.Application) // Set app reference for loading animations
	app.footer = NewFooter()
	app.nodeList = NewNodeList()
	app.vmList = NewVMList()
	app.nodeDetails = NewNodeDetails()
	app.vmDetails = NewVMDetails()
	app.clusterStatus = NewClusterStatus()
	app.pages = tview.NewPages()

	// Initialize global state
	if client.Cluster == nil {
		if _, err := client.FastGetClusterStatus(func() {
			// This callback is called when background VM enrichment completes
			app.QueueUpdateDraw(func() {
				// Refresh the currently selected VM details if there is one
				if selectedVM := app.vmList.GetSelectedVM(); selectedVM != nil {
					app.vmDetails.Update(selectedVM)
				}
			})
		}); err != nil {
			app.header.ShowError("Failed to connect to Proxmox API: " + err.Error())
			// Continue with empty state rather than crashing
		}
	}

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

	models.GlobalState = models.State{
		SearchStates:  make(map[string]*models.SearchState),
		OriginalNodes: make([]*api.Node, 0),
		FilteredNodes: make([]*api.Node, 0),
		OriginalVMs:   make([]*api.VM, len(vms)),
		FilteredVMs:   make([]*api.VM, len(vms)),
	}

	if client.Cluster != nil {
		models.GlobalState.OriginalNodes = make([]*api.Node, len(client.Cluster.Nodes))
		models.GlobalState.FilteredNodes = make([]*api.Node, len(client.Cluster.Nodes))
		copy(models.GlobalState.OriginalNodes, client.Cluster.Nodes)
		copy(models.GlobalState.FilteredNodes, client.Cluster.Nodes)
	}
	copy(models.GlobalState.OriginalVMs, vms)
	copy(models.GlobalState.FilteredVMs, vms)

	// Set up component connections
	app.setupComponentConnections()

	// Configure root layout
	app.mainLayout = app.createMainLayout()

	// Register keyboard handlers
	app.setupKeyboardHandlers()

	// Set the root and focus
	app.SetRoot(app.mainLayout, true)
	app.SetFocus(app.nodeList)

	return app
}

// Run starts the application
func (a *App) Run() error {
	// We're disabling automatic background refresh to prevent UI issues
	// The user can manually refresh with a key if needed

	// Start the app
	return a.Application.Run()
}
