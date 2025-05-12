package ui

import (
	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/lonepie/proxmox-tui/pkg/config"
	"github.com/lonepie/proxmox-tui/pkg/ui/models"
	"github.com/rivo/tview"
)

// NewAppUI creates the root UI component with node tree and VM list.
type AppUI struct {
	*tview.Flex
	app    *tview.Application
	client *api.Client
	config config.Config
}

func NewAppUI(app *tview.Application, client *api.Client, cfg config.Config) *AppUI {
	a := &AppUI{
		Flex:   tview.NewFlex().SetDirection(tview.FlexRow),
		app:    app,
		client: client,
		config: cfg,
	}
	// Create UI components
	header := CreateHeader()
	summaryPanel, summary, resourceTable := CreateClusterStatusPanel() // Get both tables from panel
	footer := CreateFooter()

	// Get all nodes from Proxmox API
	nodes, err := client.ListNodes()
	if err != nil {
		header.SetText("Error fetching nodes: " + err.Error())
		return a
	}
	if len(nodes) == 0 {
		header.SetText("No nodes found")
		return a
	}

	// Create node components
	nodeList := CreateNodeList(nodes)
	nodeList.SetTitle("Nodes")
	nodeList.SetBorder(true).SetTitle("Nodes")
	models.GlobalState.NodeList = nodeList

	detailsPanel, detailsTable := CreateDetailsPanel() // Now implemented in details.go

	// Create nodes tab content
	nodesContent := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nodeList, 0, 1, true).
		AddItem(detailsPanel, 0, 3, false)

	// Get all VMs across all nodes
	var vmsAll []api.VM
	for _, n := range nodes {
		vms, _ := client.ListVMs(n.Name)
		vmsAll = append(vmsAll, vms...)
	}

	// Create VM components
	vmList := CreateVMList(vmsAll)
	vmList.SetTitle("Guests")
	vmList.SetBorder(true).SetTitle("Guests")
	models.GlobalState.VMList = vmList

	vmDetails := newVmDetails()
	vmDetails.SetTitle("VM Details")
	vmDetails.SetBorder(true).SetTitle("VM Details")

	// Start VM status refresh background process
	StartVMStatusRefresh(app, client, vmList, vmsAll)

	// Create pages container
	pages := CreatePagesContainer()

	// Add nodes and guests pages
	AddNodesPage(pages, nodesContent)
	AddGuestsPage(pages, vmList, vmDetails)

	// Set up handlers
	SetupVMHandlers(vmList, vmDetails, vmsAll, client)
	activeIndex, _, updateDetails := SetupNodeHandlers(app, client, nodeList, nodes, summary, resourceTable, detailsTable, header, pages)

	// Trigger initial node selection
	if len(nodes) > 0 {
		nodeList.SetCurrentItem(activeIndex)
		if fn := nodeList.GetSelectedFunc(); fn != nil {
			fn(activeIndex, "", "", 0) // Trigger selection handler
		}
		// Manually trigger details update for initial node
		updateDetails(activeIndex, "", "", 0)
	}

	// Set up keyboard shortcuts
	pages = a.SetupKeyboardHandlers(pages, nodeList, vmList, vmsAll, nodes, vmDetails, header)

	// Tasks/Logs tab (TODO)
	tasksView := tview.NewTextView().SetText("[::b]Tasks/Logs view coming soon")
	tasksView.SetTitle("Tasks/Logs")
	tasksView.SetBorder(true).SetTitle("Tasks/Logs")
	pages.AddPage("Tasks/Logs", tasksView, true, false)

	// Storage tab (TODO)
	storageView := tview.NewTextView().SetText("[::b]Storage view coming soon")
	storageView.SetTitle("Storage")
	storageView.SetBorder(true).SetTitle("Storage")
	pages.AddPage("Storage", storageView, true, false)

	// Network tab (TODO)
	networkView := tview.NewTextView().SetText("[::b]Network view coming soon")
	networkView.SetTitle("Network")
	networkView.SetBorder(true).SetTitle("Network")
	pages.AddPage("Network", networkView, true, false)

	// Set initial focus to node list
	app.SetFocus(nodeList)

	// Main layout: header, summary, pages, footer
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(summaryPanel, 8, 0, false). // Increased height by 1 more row to show all data
		AddItem(pages, 0, 1, true).
		AddItem(footer, 1, 0, false)

	// Set up all keyboard handlers (including shell info functionality)
	a.SetupKeyboardHandlers(pages, nodeList, vmList, vmsAll, nodes, vmDetails, header)

	a.AddItem(mainFlex, 0, 1, true)
	return a
}
