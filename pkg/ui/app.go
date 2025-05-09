package ui

import (
	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/lonepie/proxmox-tui/pkg/ui/models"
	"github.com/rivo/tview"
)

// NewAppUI creates the root UI component with node tree and VM list.
func NewAppUI(app *tview.Application, client *api.Client) tview.Primitive {
	// Create UI components
	header := CreateHeader()
	summaryPanel, summary := CreateNodeSummaryPanel()
	footer := CreateFooter()

	// Get all nodes from Proxmox API
	nodes, err := client.ListNodes()
	if err != nil {
		header.SetText("Error fetching nodes: " + err.Error())
		return tview.NewBox().SetTitle("Error")
	}
	if len(nodes) == 0 {
		header.SetText("No nodes found")
		return tview.NewBox().SetTitle("Error")
	}

	// Create node components
	nodeList := CreateNodeList(nodes)
	nodeList.SetTitle("Nodes")
	nodeList.SetBorder(true).SetTitle("Nodes")
	models.GlobalState.NodeList = nodeList

	detailsPanel, detailsTable := CreateDetailsPanel()

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
	SetupNodeHandlers(app, client, nodeList, nodes, summary, detailsTable, header)
	// Shell access is now handled in SetupKeyboardHandlers

	// Set up keyboard shortcuts
	pages = SetupKeyboardHandlers(app, pages, nodeList, vmList, vmsAll, nodes, vmDetails, header)

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
		AddItem(summaryPanel, 5, 0, false).
		AddItem(pages, 0, 1, true).
		AddItem(footer, 1, 0, false)

	// Set up all keyboard handlers (including shell info functionality)
	SetupKeyboardHandlers(app, pages, nodeList, vmList, vmsAll, nodes, vmDetails, header)

	return mainFlex
}
