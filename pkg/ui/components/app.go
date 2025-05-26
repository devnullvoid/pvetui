package components

import (
	"fmt"
	"strings"
	"time"

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
	a.nodeList.SetApp(a)
	a.nodeList.SetNodeSelectedFunc(func(node *api.Node) {
		a.nodeDetails.Update(node, a.client.Cluster.Nodes)
		// No longer filtering VM list based on node selection
	})
	a.nodeList.SetNodeChangedFunc(func(node *api.Node) {
		a.nodeDetails.Update(node, a.client.Cluster.Nodes)
		// No longer filtering VM list based on node selection
	})

	// Configure node details
	a.nodeDetails.SetApp(a)

	// Select first node to populate node details on startup
	if len(models.GlobalState.OriginalNodes) > 0 {
		a.nodeDetails.Update(models.GlobalState.OriginalNodes[0], a.client.Cluster.Nodes)
	}

	// Set up VM list with all VMs
	a.vmList.SetApp(a)

	// Configure VM list callbacks BEFORE setting VMs
	a.vmList.SetVMSelectedFunc(func(vm *api.VM) {
		a.vmDetails.Update(vm)
	})
	a.vmList.SetVMChangedFunc(func(vm *api.VM) {
		a.vmDetails.Update(vm)
	})

	// Now set the VMs - this will trigger the onSelect callback for the first VM
	a.vmList.SetVMs(models.GlobalState.OriginalVMs)

	// Configure VM details
	a.vmDetails.SetApp(a)
}

// setupKeyboardHandlers configures global keyboard shortcuts
func (a *App) setupKeyboardHandlers() {
	a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check if search is active by seeing if the search input is in the main layout
		searchActive := a.mainLayout.GetItemCount() > 4

		// Check if any modal page is active
		pageName, _ := a.pages.GetFrontPage()
		modalActive := strings.HasPrefix(pageName, "script") ||
			a.pages.HasPage("scriptInfo") ||
			a.pages.HasPage("scriptSelector") ||
			a.pages.HasPage("message") ||
			a.pages.HasPage("confirmation")

		// If search is active, let the search input handle the keys
		if searchActive {
			// Let the search input handle all keys when search is active
			return event
		}

		// If a modal dialog is active, let it handle its own keys
		if modalActive {
			return event
		}

		// If context menu is open, let it handle keys
		if a.isMenuOpen && a.contextMenu != nil {
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
		case tcell.KeyF5:
			// Manual refresh
			a.manualRefresh()
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
			} else if event.Rune() == 'm' {
				// Open context menu based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == "Nodes" {
					a.showNodeContextMenu()
				} else if currentPage == "Guests" {
					a.showVMContextMenu()
				}
				return nil
			} else if event.Rune() == 'c' || event.Rune() == 'C' {
				// Open community scripts installer - only available for nodes
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == "Nodes" {
					node := a.nodeList.GetSelectedNode()
					if node != nil {
						a.openScriptSelector(node, nil)
					}
				} else if currentPage == "Guests" {
					// Community scripts are not available for individual VMs
					a.showMessage("Community scripts can only be installed on nodes. Switch to the Nodes tab to install scripts.")
				}
				return nil
			} else if event.Rune() == 'r' || event.Rune() == 'R' {
				// Manual refresh
				a.manualRefresh()
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

// showConfirmationDialog displays a confirmation dialog with Yes/No options
func (a *App) showConfirmationDialog(message string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("confirmation")
			if buttonIndex == 0 {
				// Yes was selected
				onConfirm()
			}
		})

	a.pages.AddPage("confirmation", modal, false, true)
}

// showNodeContextMenu displays the context menu for node actions
func (a *App) showNodeContextMenu() {
	node := a.nodeList.GetSelectedNode()
	if node == nil {
		return
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on node state
	menuItems := []string{
		"Open Shell",
		"View Logs",
		"Install Community Script",
	}

	// Create and show context menu
	menu := NewContextMenu(" Node Actions ", menuItems, func(index int, action string) {
		switch action {
		case "Open Shell":
			a.openNodeShell()
		case "View Logs":
			a.showMessage("Viewing logs for node: " + node.Name)
		case "Install Community Script":
			a.openScriptSelector(node, nil)
		}
	})
	menu.SetApp(a)

	// Display the menu
	menuList := menu.Show()
	a.contextMenu = menuList
	a.isMenuOpen = true

	// Create a centered modal layout
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// showVMContextMenu displays the context menu for VM actions
func (a *App) showVMContextMenu() {
	vm := a.vmList.GetSelectedVM()
	if vm == nil {
		return
	}

	// Store last focused primitive
	a.lastFocus = a.GetFocus()

	// Create menu items based on VM state
	menuItems := []string{
		"Open Shell",
	}

	// Add state-dependent actions
	if vm.Status == "running" {
		menuItems = append(menuItems, "Shutdown", "Restart")
	} else if vm.Status == "stopped" {
		menuItems = append(menuItems, "Start")
	}

	// Note: Removed "Install Community Script" as it's only applicable to nodes

	// Create and show context menu
	menu := NewContextMenu(" Guest Actions ", menuItems, func(index int, action string) {
		switch action {
		case "Open Shell":
			a.openVMShell()
		case "Start":
			a.performVMOperation(vm, a.client.StartVM, "Starting")
		case "Shutdown":
			a.performVMOperation(vm, a.client.StopVM, "Shutting down")
		case "Restart":
			a.performVMOperation(vm, a.client.RestartVM, "Restarting")
		}
	})
	menu.SetApp(a)

	// Display the menu
	menuList := menu.Show()
	a.contextMenu = menuList
	a.isMenuOpen = true

	// Create a centered modal layout
	a.pages.AddPage("contextMenu", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(menuList, len(menuItems)+2, 1, true). // +2 for border
			AddItem(nil, 0, 1, false), 30, 1, true).
		AddItem(nil, 0, 1, false), true, true)
	a.SetFocus(menuList)
}

// openScriptSelector opens the script selector dialog
func (a *App) openScriptSelector(node *api.Node, vm *api.VM) {
	if a.config.SSHUser == "" {
		a.showMessage("SSH user not configured. Please set PROXMOX_SSH_USER environment variable or use --ssh-user flag.")
		return
	}

	selector := NewScriptSelector(a, node, vm, a.config.SSHUser)
	selector.Show()
}

// performVMOperation performs an asynchronous VM operation and shows status message
func (a *App) performVMOperation(vm *api.VM, operation func(*api.VM) error, operationName string) {
	// Show loading indicator
	a.header.ShowLoading(fmt.Sprintf("%s %s", operationName, vm.Name))

	// Run operation in goroutine to avoid blocking UI
	go func() {
		if err := operation(vm); err != nil {
			// Update message with error on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Error %s %s: %v", strings.ToLower(operationName), vm.Name, err))
			})
		} else {
			// Update message with success on main thread
			a.QueueUpdateDraw(func() {
				a.header.ShowSuccess(fmt.Sprintf("%s %s completed successfully", operationName, vm.Name))
			})

			// Wait a moment before refreshing to allow the operation to complete on the server
			time.Sleep(1 * time.Second)

			// Manually refresh data to show updated state
			a.manualRefresh()
		}
	}()
}

// closeContextMenu closes the context menu and restores the previous focus
func (a *App) closeContextMenu() {
	if a.isMenuOpen {
		a.pages.RemovePage("contextMenu")
		a.isMenuOpen = false
		a.contextMenu = nil
		if a.lastFocus != nil {
			a.SetFocus(a.lastFocus)
		}
	}
}

// Run starts the application
func (a *App) Run() error {
	// We're disabling automatic background refresh to prevent UI issues
	// The user can manually refresh with a key if needed

	// Start the app
	return a.Application.Run()
}

// manualRefresh refreshes data and updates the UI on user request
func (a *App) manualRefresh() {
	// Show animated loading indicator
	a.header.ShowLoading("Refreshing data")

	// Use goroutine to avoid blocking the UI
	go func() {
		// Fetch fresh data bypassing cache
		cluster, err := a.client.GetFreshClusterStatus()
		if err != nil {
			a.QueueUpdateDraw(func() {
				a.header.ShowError(fmt.Sprintf("Refresh failed: %v", err))
			})
			return
		}

		// Update UI with new data
		a.QueueUpdateDraw(func() {
			// Get current search states
			nodeSearchState := models.GlobalState.GetSearchState("nodes")
			vmSearchState := models.GlobalState.GetSearchState("vms")

			// Update component data
			a.clusterStatus.Update(cluster)

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
				// Re-filter with the current search term
				models.FilterNodes(nodeSearchState.Filter)
				a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
			} else {
				// No filter active, use all nodes
				a.nodeList.SetNodes(models.GlobalState.OriginalNodes)
			}

			// Same approach for VMs
			if vmSearchState != nil && vmSearchState.Filter != "" {
				// Re-filter with the current search term
				models.FilterVMs(vmSearchState.Filter)
				a.vmList.SetVMs(models.GlobalState.FilteredVMs)
			} else {
				// No filter active, use all VMs
				a.vmList.SetVMs(models.GlobalState.OriginalVMs)
			}

			// Update details if items are selected
			if node := a.nodeList.GetSelectedNode(); node != nil {
				a.nodeDetails.Update(node, cluster.Nodes)
			}

			if vm := a.vmList.GetSelectedVM(); vm != nil {
				a.vmDetails.Update(vm)
			}

			// Show success message
			a.header.ShowSuccess("Data refreshed successfully")
		})
	}()
}

// backgroundRefresh has been disabled to prevent UI issues
// func (a *App) backgroundRefresh() {
// 	// Disabled
// }
