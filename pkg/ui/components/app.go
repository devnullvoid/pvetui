package components

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/models"
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
}

// NewApp creates a new application instance with all UI components
func NewApp(client *api.Client, cfg config.Config) *App {
	app := &App{
		Application: tview.NewApplication(),
		client:      client,
		config:      cfg,
	}

	// Create UI components
	app.header = NewHeader()
	app.footer = NewFooter()
	app.nodeList = NewNodeList()
	app.vmList = NewVMList()
	app.nodeDetails = NewNodeDetails()
	app.vmDetails = NewVMDetails()
	app.clusterStatus = NewClusterStatus()
	app.pages = tview.NewPages()

	// Initialize global state
	if client.Cluster == nil {
		if _, err := client.GetClusterStatus(); err != nil {
			app.header.SetText("Error fetching cluster: " + err.Error())
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

// createMainLayout builds the main application layout
func (a *App) createMainLayout() *tview.Flex {
	// Setup nodes page
	nodesPage := tview.NewFlex().
		AddItem(a.nodeList, 0, 1, true).
		AddItem(a.nodeDetails, 0, 2, false)

	// Setup VMs page
	vmsPage := tview.NewFlex().
		AddItem(a.vmList, 0, 1, true).
		AddItem(a.vmDetails, 0, 2, false)

	// Add pages
	a.pages.AddPage("Nodes", nodesPage, true, true)
	a.pages.AddPage("Guests", vmsPage, true, false)

	// Build main layout
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.clusterStatus, 5, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.footer, 1, 0, false)
}

// setupComponentConnections wires up the interactions between components
func (a *App) setupComponentConnections() {
	// Update cluster status
	a.clusterStatus.Update(a.client.Cluster)

	// Configure node list
	a.nodeList.SetNodes(models.GlobalState.OriginalNodes)
	a.nodeList.SetNodeSelectedFunc(func(node *api.Node) {
		a.nodeDetails.Update(node, a.client.Cluster.Nodes)
		// No longer filtering VM list based on node selection
	})
	a.nodeList.SetNodeChangedFunc(func(node *api.Node) {
		a.nodeDetails.Update(node, a.client.Cluster.Nodes)
		// No longer filtering VM list based on node selection
	})

	// Select first node to populate node details on startup
	if len(models.GlobalState.OriginalNodes) > 0 {
		a.nodeDetails.Update(models.GlobalState.OriginalNodes[0], a.client.Cluster.Nodes)
	}

	// Set up VM list with all VMs
	a.vmList.SetVMs(models.GlobalState.OriginalVMs)

	// Configure VM list
	a.vmList.SetVMSelectedFunc(func(vm *api.VM) {
		a.vmDetails.Update(vm)
	})
	a.vmList.SetVMChangedFunc(func(vm *api.VM) {
		a.vmDetails.Update(vm)
	})

	// Update VM details if we have any VMs
	if len(models.GlobalState.OriginalVMs) > 0 {
		a.vmDetails.Update(models.GlobalState.OriginalVMs[0])
	}
}

// setupKeyboardHandlers configures global keyboard shortcuts
func (a *App) setupKeyboardHandlers() {
	a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check if search is active by seeing if the search input is in the main layout
		searchActive := a.mainLayout.GetItemCount() > 4

		// If search is active, let the search input handle the keys
		if searchActive {
			// Let the search input handle all keys when search is active
			return event
		}

		// Handle tab for page switching when search is not active
		switch event.Key() {
		case tcell.KeyTab:
			currentPage, _ := a.pages.GetFrontPage()
			if currentPage == "Nodes" {
				a.pages.SwitchToPage("Guests")
				a.SetFocus(a.vmList)
			} else {
				a.pages.SwitchToPage("Nodes")
				a.SetFocus(a.nodeList)
			}
			return nil
		case tcell.KeyF1:
			a.pages.SwitchToPage("Nodes")
			a.SetFocus(a.nodeList)
			return nil
		case tcell.KeyF2:
			a.pages.SwitchToPage("Guests")
			a.SetFocus(a.vmList)
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				a.Stop()
				return nil
			} else if event.Rune() == '/' {
				// Activate search
				a.activateSearch()
				return nil
			} else if event.Rune() == 's' || event.Rune() == 'S' {
				// Open shell session based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == "Nodes" {
					// Handle node shell session
					a.openNodeShell()
				} else if currentPage == "Guests" {
					// Handle VM shell session
					a.openVMShell()
				}
				return nil
			}
		}
		return event
	})
}

// showMessage displays a message to the user
func (a *App) showMessage(message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("message")
		})

	a.pages.AddPage("message", modal, false, true)
}

// Run starts the application
func (a *App) Run() error {
	return a.Application.Run()
}
