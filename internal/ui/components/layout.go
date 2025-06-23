package components

import (
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

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

	// Configure tasks list
	a.tasksList.SetApp(a)

	// Load initial tasks data
	go func() {
		tasks, err := a.client.GetClusterTasks()
		if err == nil {
			a.QueueUpdateDraw(func() {
				a.tasksList.SetTasks(tasks)
			})
		}
	}()

	// Configure help modal
	a.helpModal.SetApp(a)
}
