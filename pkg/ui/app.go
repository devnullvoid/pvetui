package ui

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/models"
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
	summaryPanel, summary, resourceTable := CreateClusterStatusPanel()
	footer := CreateFooter()

	// Use cached cluster data
	if client.Cluster == nil {
		if _, err := client.GetClusterStatus(); err != nil {
			header.SetText("Error fetching cluster: " + err.Error())
			return a
		}
	}
	cluster := client.Cluster

	// Get nodes from cached cluster data
	if len(cluster.Nodes) == 0 {
		header.SetText("No nodes found in cluster")
		return a
	}
	nodes := make([]api.Node, len(cluster.Nodes))
	for i, n := range cluster.Nodes {
		if n == nil {
			continue
		}
		nodes[i] = *n
	}

	// Create node components with emoji status
	nodeList := CreateNodeList(nodes)
	nodeList.SetTitle("Nodes")
	nodeList.SetBorder(true).SetTitle("Nodes")

	// Initialize global state
	models.GlobalState = models.State{
		NodeList:       nodeList,
		VMList:         nil, // Will be set after VM list creation
		LastSearchText: "",
	}

	detailsPanel, detailsTable := CreateDetailsPanel()

	// Create nodes tab content
	nodesContent := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nodeList, 0, 1, true).
		AddItem(detailsPanel, 0, 3, false)

	// Get all VMs from cached cluster data
	var vmsAll []api.VM
	for _, node := range client.Cluster.Nodes {
		if node != nil {
			for _, vm := range node.VMs {
				vmsAll = append(vmsAll, *vm)
			}
		}
	}

	// Create VM components with status coloring
	vmList := CreateVMList(vmsAll)
	BuildVMList(vmsAll, vmList)
	vmList.SetTitle("Guests")
	vmList.SetBorder(true).SetTitle("Guests")
	models.GlobalState.VMList = vmList

	vmDetails := newVmDetails()
	vmDetails.SetTitle("VM Details")
	vmDetails.SetBorder(true).SetTitle("VM Details")

	// Start VM status refresh background process
	// StartVMStatusRefresh(app, client, vmList, vmsAll)

	// Create pages container
	pages := CreatePagesContainer()

	// Add nodes and guests pages
	AddNodesPage(pages, nodesContent)
	AddGuestsPage(pages, vmList, vmDetails)

	// Set up handlers with cluster data
	SetupVMHandlers(vmList, vmDetails, vmsAll, client)
	activeIndex, _, updateDetails := SetupNodeHandlers(app, client, cluster, nodeList, nodes, summary, resourceTable, detailsTable, header, pages)

	// Trigger initial node selection
	if len(nodes) > 0 {
		nodeList.SetCurrentItem(activeIndex)
		if fn := nodeList.GetSelectedFunc(); fn != nil {
			fn(activeIndex, "", "", 0)
		}
		updateDetails(activeIndex, "", "", 0)
	}

	// Trigger initial VM selection
	if len(vmsAll) > 0 {
		vmList.SetCurrentItem(0)
		if fn := vmList.GetSelectedFunc(); fn != nil {
			fn(0, vmsAll[0].Name, vmsAll[0].Status, 0)
		}
	}

	// Set up keyboard shortcuts
	pages = a.SetupKeyboardHandlers(pages, nodeList, vmList, vmsAll, nodes, vmDetails, header)

	// Initialize tabs
	initTabs(pages)

	// Set initial focus to node list
	app.SetFocus(nodeList)

	// Main layout
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(summaryPanel, 8, 0, false).
		AddItem(pages, 0, 1, true).
		AddItem(footer, 1, 0, false)

	a.AddItem(mainFlex, 0, 1, true)
	return a
}

func initTabs(pages *tview.Pages) {
	// Tasks/Logs tab
	pages.AddPage("Tasks/Logs", tview.NewTextView().
		SetText("[::b]Tasks/Logs view coming soon").
		SetTitle("Tasks/Logs").
		SetBorder(true), true, false)

	// Storage tab
	pages.AddPage("Storage", tview.NewTextView().
		SetText("[::b]Storage view coming soon").
		SetTitle("Storage").
		SetBorder(true), true, false)

	// Network tab
	pages.AddPage("Network", tview.NewTextView().
		SetText("[::b]Network view coming soon").
		SetTitle("Network").
		SetBorder(true), true, false)
}
