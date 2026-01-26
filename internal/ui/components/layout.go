package components

import (
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/models"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// createMainLayout builds the main application layout.
func (a *App) createMainLayout() *tview.Flex {
	// Setup nodes page
	nodesPage := tview.NewFlex().
		AddItem(a.nodeList, 0, 1, true).
		AddItem(a.nodeDetails, 0, 2, false)

	// Setup VMs page
	vmsPage := tview.NewFlex().
		AddItem(a.vmList, 0, 1, true).
		AddItem(a.vmDetails, 0, 2, false)

	// Setup Tasks page
	tasksPage := a.tasksList

	// Add pages
	a.pages.AddPage(api.PageNodes, nodesPage, true, true)
	a.pages.AddPage(api.PageGuests, vmsPage, true, false)
	a.pages.AddPage(api.PageTasks, tasksPage, true, false)

	// Build main layout
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.clusterStatus, 6, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.footer, 1, 0, false)
}

// setupComponentConnections wires up the interactions between components.
func (a *App) setupComponentConnections() {
	// Set app reference for cluster status
	a.clusterStatus.SetApp(a)

	// Update cluster status
	a.clusterStatus.Update(a.getDisplayCluster())

	// Configure node list - check for existing search filters
	nodeSearchState := models.GlobalState.GetSearchState(api.PageNodes)
	if nodeSearchState != nil && nodeSearchState.Filter != "" {
		// Apply existing filter
		models.FilterNodes(nodeSearchState.Filter)
		a.nodeList.SetNodes(models.GlobalState.FilteredNodes)
	} else {
		// No filter, use original data
		a.nodeList.SetNodes(models.GlobalState.OriginalNodes)
	}

	a.nodeList.SetApp(a)
	a.nodeList.SetNodeSelectedFunc(func(node *api.Node) {
		a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
		// No longer filtering VM list based on node selection
	})
	a.nodeList.SetNodeChangedFunc(func(node *api.Node) {
		a.nodeDetails.Update(node, models.GlobalState.OriginalNodes)
		// No longer filtering VM list based on node selection
	})

	// Configure node details
	a.nodeDetails.SetApp(a)

	// Select first node to populate node details on startup
	// Use the selected node from the node list (which is sorted) instead of raw original nodes
	if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
		a.nodeDetails.Update(selectedNode, models.GlobalState.OriginalNodes)
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

	// Now set the VMs - check for existing search filters first
	vmSearchState := models.GlobalState.GetSearchState(api.PageGuests)
	if vmSearchState != nil && vmSearchState.Filter != "" {
		// Apply existing filter
		models.FilterVMs(vmSearchState.Filter)
		a.vmList.SetVMs(models.GlobalState.FilteredVMs)
	} else {
		// No filter, use original data
		a.vmList.SetVMs(models.GlobalState.OriginalVMs)
	}

	// Configure VM details
	a.vmDetails.SetApp(a)

	// Configure tasks list
	a.tasksList.SetApp(a)

	// Load initial tasks data
	a.loadTasksData()

	// Configure help modal
	a.helpModal.SetApp(a)
}
