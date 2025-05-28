package components

import (
	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App is the main application component
type App struct {
	*tview.Application
	client        *api.Client
	config        config.Config
	pages         *tview.Pages
	header        *Header
	footer        *Footer
	nodeList      *NodeList
	vmList        *VMList
	nodeDetails   *NodeDetails
	vmDetails     *VMDetails
	clusterStatus *ClusterStatus
	mainLayout    *tview.Flex
	searchInput   *tview.InputField
	contextMenu   *tview.List
	isMenuOpen    bool
	lastFocus     tview.Primitive
}

// NewApp creates a new application instance with all UI components
func NewApp(client *api.Client, cfg *config.Config) *App {
	app := &App{
		Application: tview.NewApplication(),
		client:      client,
		config:      *cfg,
	}

	// Set application theme and background color
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorBlack
	// Alternative colors you can try:
	// tcell.ColorDarkBlue, tcell.ColorDarkGreen, tcell.ColorDarkCyan,
	// tcell.ColorDarkRed, tcell.ColorDarkMagenta, tcell.ColorDarkYellow,
	// tcell.ColorNavy, tcell.ColorMaroon, tcell.ColorTeal, tcell.ColorSilver

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
		if _, err := client.FastGetClusterStatus(); err != nil {
			app.header.ShowError("Error fetching cluster: " + err.Error())
			return app
		}
	}

	// Initialize VM list from all nodes
	var vms []*api.VM
	for _, node := range client.Cluster.Nodes {
		if node != nil {
			for _, vm := range node.VMs {
				if vm != nil {
					vms = append(vms, vm)
				}
			}
		}
	}

	models.GlobalState = models.State{
		SearchStates:  make(map[string]*models.SearchState),
		OriginalNodes: make([]*api.Node, len(client.Cluster.Nodes)),
		FilteredNodes: make([]*api.Node, len(client.Cluster.Nodes)),
		OriginalVMs:   make([]*api.VM, len(vms)),
		FilteredVMs:   make([]*api.VM, len(vms)),
	}

	copy(models.GlobalState.OriginalNodes, client.Cluster.Nodes)
	copy(models.GlobalState.FilteredNodes, client.Cluster.Nodes)
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
