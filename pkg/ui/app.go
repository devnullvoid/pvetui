package ui

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/models"
	"github.com/rivo/tview"
	// "github.com/gdamore/tcell/v2"
)

// NewAppUI creates the root UI component with node tree and VM list.
type AppUI struct {
	*tview.Flex
	app         *tview.Application
	client      *api.Client
	config      config.Config
	vmDetails   *tview.Table // VM details panel
	nodeDetails *tview.Table // Node details panel
}

// updateNodeDetails updates the node details panel with the given node
func (a *AppUI) updateNodeDetails(node *api.Node) {
	if a.nodeDetails != nil && node != nil {
		// Use the new centralized display function.
		// We need the full list of nodes to allow DisplayNodeDetailsInTable to find the complete status.
		// Assuming client.Cluster.Nodes is the authoritative full list.
		// If models.GlobalState.OriginalNodes is guaranteed to be populated and correct, it could also be used.
		var fullNodeList []*api.Node
		if a.client != nil && a.client.Cluster != nil {
			fullNodeList = a.client.Cluster.Nodes
		} else {
			// Fallback or handle error if full list isn't available
			// For now, pass nil, DisplayNodeDetailsInTable should handle it by showing limited info or error.
			config.DebugLog("updateNodeDetails: Warning - full client.Cluster.Nodes list not available for DisplayNodeDetailsInTable")
		}
		DisplayNodeDetailsInTable(a.nodeDetails, node, fullNodeList)

		// Update the selected node in global state if it exists in the current filtered list
		for i, n := range models.GlobalState.FilteredNodes {
			if n != nil && n.Name == node.Name {
				if state, exists := models.GlobalState.SearchStates["Nodes"]; exists {
					state.SelectedIndex = i
				}
				break
			}
		}
	}
}

// updateVMDetails updates the VM details panel with the given VM
func (a *AppUI) updateVMDetails(vm *api.VM) {
	if a.vmDetails != nil && vm != nil {
		populateVmDetails(a.vmDetails, vm)
		// Update the selected VM in global state if it exists in the current filtered list
		for i, v := range models.GlobalState.FilteredVMs {
			if v != nil && v.ID == vm.ID {
				if state, exists := models.GlobalState.SearchStates["Guests"]; exists {
					state.SelectedIndex = i
				}
				break
			}
		}
	}
}

// updateVMSelectionHandlers updates the VM list selection handlers with the current filtered list
func (a *AppUI) updateVMSelectionHandlers(vmList *tview.List, vms []*api.VM, vmDetails *tview.Table) {
	vmList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(vms) {
			a.updateVMDetails(vms[index])
			// Update the selected index in global state
			if state, exists := models.GlobalState.SearchStates["Guests"]; exists {
				state.SelectedIndex = index
			}
		}
	})

	vmList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(vms) {
			a.updateVMDetails(vms[index])
			// Update the selected index in global state
			if state, exists := models.GlobalState.SearchStates["Guests"]; exists {
				state.SelectedIndex = index
			}
		}
	})
}

// updateNodeSelectionHandlers updates the node list selection handlers with the current filtered list
func (a *AppUI) updateNodeSelectionHandlers(nodeList *tview.List, nodes []*api.Node) {
	nodeList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(nodes) {
			a.updateNodeDetails(nodes[index])
			// Update the selected index in global state
			if state, exists := models.GlobalState.SearchStates["Nodes"]; exists {
				state.SelectedIndex = index
			}
		}
	})

	nodeList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(nodes) {
			a.updateNodeDetails(nodes[index])
			// Update the selected index in global state
			if state, exists := models.GlobalState.SearchStates["Nodes"]; exists {
				state.SelectedIndex = index
			}
		}
	})
}

// populateNodeDetails updates the details table with node information - REMOVED as DisplayNodeDetailsInTable is now used.
/*
func populateNodeDetails(table *tview.Table, node *api.Node) {
	// Clear existing rows
	table.Clear()

	if node == nil {
		return
	}

	// Add header
	headers := []string{"Property", "Value"}
	for col, text := range headers {
		cell := tview.NewTableCell(text).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft)
		table.SetCell(0, col, cell)
	}

	// Add node details
	addDetailRow := func(row int, label, value string) {
		cell := tview.NewTableCell(label).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft)
		table.SetCell(row, 0, cell)

		cell = tview.NewTableCell(value).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft)
		table.SetCell(row, 1, cell)
	}

	// Basic info
	row := 1
	addDetailRow(row, "Name", node.Name)
	row++

	// Status
	status := "🔴 Offline"
	if node.Online {
		status = "🟢 Online"
	}
	addDetailRow(row, "Status", status)
	row++

	// CPU
	cpuInfo := fmt.Sprintf("%.1f%% of %.0f cores", node.CPUUsage*100, node.CPUCount)
	if node.CPUInfo != nil {
		cpuInfo = fmt.Sprintf("%s (%d cores, %d sockets)", cpuInfo, node.CPUInfo.Cores, node.CPUInfo.Sockets)
	}
	addDetailRow(row, "CPU", cpuInfo)
	row++

	// Memory
	addDetailRow(row, "Memory", fmt.Sprintf("%.1f GB / %.1f GB", node.MemoryUsed, node.MemoryTotal))
	row++

	// Storage
	addDetailRow(row, "Storage", fmt.Sprintf("%.1f GB / %.1f GB",
		float64(node.UsedStorage)/1024/1024/1024,
		float64(node.TotalStorage)/1024/1024/1024))
	row++

	// Version
	if node.Version != "" {
		addDetailRow(row, "Version", node.Version)
		row++
	}

	// IP
	if node.IP != "" {
		addDetailRow(row, "IP", node.IP)
		row++
	}
}
*/

func NewAppUI(app *tview.Application, client *api.Client, cfg config.Config) *AppUI {
	// Create node details table
	nodeDetailsTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(false, false)
	nodeDetailsTable.SetTitle(" Node Details ").SetBorder(true)

	a := &AppUI{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		app:         app,
		client:      client,
		config:      cfg,
		nodeDetails: nodeDetailsTable,
	}
	a.nodeDetails.SetTitle(" Node Details ")

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

	// Initialize global state first
	models.GlobalState = models.State{
		NodeList:     nil, // Will be set after node list creation
		VMList:       nil, // Will be set after VM list creation
		SearchStates: make(map[string]*models.SearchState),
		OriginalNodes: make([]*api.Node, len(client.Cluster.Nodes)),
		FilteredNodes: make([]*api.Node, len(client.Cluster.Nodes)),
	}
	copy(models.GlobalState.OriginalNodes, client.Cluster.Nodes)
	copy(models.GlobalState.FilteredNodes, client.Cluster.Nodes)

	// Create node list with filtered nodes if available, otherwise all nodes
	nodesToShow := models.GlobalState.FilteredNodes
	if len(nodesToShow) == 0 {
		nodesToShow = models.GlobalState.OriginalNodes
	}

	// Create node list
	nodeList := tview.NewList().ShowSecondaryText(false)
	nodeList.SetBorder(true).SetTitle("Nodes")
	
	// Add nodes to the list immediately
	for _, node := range nodesToShow {
		if node != nil {
			nodeList.AddItem(FormatNodeName(node), "", 0, nil)
		}
	}

	// Set up initial selection and handlers
	if len(nodesToShow) > 0 {
		nodeList.SetCurrentItem(0)
		a.updateNodeDetails(nodesToShow[0])
	}
	a.updateNodeSelectionHandlers(nodeList, nodesToShow)

	// Update global state with node list
	models.GlobalState.NodeList = nodeList

	// Create nodes tab content
	nodesContent := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nodeList, 0, 1, true).
		AddItem(a.nodeDetails, 0, 3, false)

	// Get all VMs from cached cluster data
	var vmsAll []*api.VM
	for _, node := range client.Cluster.Nodes {
		if node != nil {
			for _, vm := range node.VMs {
				if vm != nil {
					vmsAll = append(vmsAll, vm)
				}
			}
		}
	}

	// Initialize global state for VMs if not already done
	if models.GlobalState.OriginalVMs == nil {
		models.GlobalState.OriginalVMs = make([]*api.VM, len(vmsAll))
		copy(models.GlobalState.OriginalVMs, vmsAll)
	}
	if models.GlobalState.FilteredVMs == nil {
		models.GlobalState.FilteredVMs = make([]*api.VM, len(vmsAll))
		copy(models.GlobalState.FilteredVMs, vmsAll)
	}

	// Create VM list with filtered VMs if available, otherwise all VMs
	vmsToShow := vmsAll
	if len(models.GlobalState.FilteredVMs) > 0 {
		vmsToShow = models.GlobalState.FilteredVMs
	}

	// Create VM components with status coloring
	vmList := tview.NewList().ShowSecondaryText(false)
	BuildVMList(vmsToShow, vmList)
	vmList.SetTitle("Guests")
	vmList.SetBorder(true)
	models.GlobalState.VMList = vmList

	// Initialize VM details panel
	a.vmDetails = newVmDetails()
	a.vmDetails.SetTitle("VM Details").SetBorder(true)

	// Set up initial selection handlers with the current filtered lists
	a.updateVMSelectionHandlers(vmList, vmsToShow, a.vmDetails)

	// Create pages container
	pages := CreatePagesContainer()

	// Add nodes and guests pages
	AddNodesPage(pages, nodesContent)
	AddGuestsPage(pages, vmList, a.vmDetails)

	// Set up handlers with cluster data
	SetupVMHandlers(vmList, a.vmDetails, vmsAll, client)
	// Pass a.nodeDetails to SetupNodeHandlers
	activeIndex, _, updateDetails := SetupNodeHandlers(app, client, cluster, nodeList, cluster.Nodes, summary, resourceTable, a.nodeDetails, header, pages)

	// Trigger initial node selection
	if len(cluster.Nodes) > 0 {
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

	// Set up keyboard handlers
	a.SetupKeyboardHandlers(pages, nodeList, vmList, vmsAll, client.Cluster.Nodes, a.vmDetails, header)

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

	// Add tab change handler
	pages.SetChangedFunc(func() {
		currentPage, _ := pages.GetFrontPage()
		switch currentPage {
		case "Nodes":
			nodesToDisplay := models.GlobalState.OriginalNodes
			// Check if a search is active for the Nodes page
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SearchText != "" && len(models.GlobalState.FilteredNodes) > 0 {
				nodesToDisplay = models.GlobalState.FilteredNodes
			}
			nodeList.Clear()
			for _, node := range nodesToDisplay {
				if node != nil {
					nodeList.AddItem(FormatNodeName(node), "", 0, nil)
				}
			}
			a.updateNodeSelectionHandlers(nodeList, nodesToDisplay)
			if len(nodesToDisplay) > 0 {
				// Try to restore selected index if valid, else default to 0
				idx := 0
				if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SelectedIndex < len(nodesToDisplay) && state.SelectedIndex >= 0 {
					idx = state.SelectedIndex
				}
				nodeList.SetCurrentItem(idx)
				a.updateNodeDetails(nodesToDisplay[idx])
			} else {
				// Clear details if list is empty
				a.updateNodeDetails(nil)
			}
		case "Guests":
			vmsToDisplay := models.GlobalState.OriginalVMs
			// Check if a search is active for the Guests page
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SearchText != "" && len(models.GlobalState.FilteredVMs) > 0 {
				vmsToDisplay = models.GlobalState.FilteredVMs
			}
			BuildVMList(vmsToDisplay, vmList)
			a.updateVMSelectionHandlers(vmList, vmsToDisplay, a.vmDetails)
			if len(vmsToDisplay) > 0 {
				// Try to restore selected index if valid, else default to 0
				idx := 0
				if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SelectedIndex < len(vmsToDisplay) && state.SelectedIndex >= 0 {
					idx = state.SelectedIndex
				}
				vmList.SetCurrentItem(idx)
				a.updateVMDetails(vmsToDisplay[idx])
			} else {
				// Clear details if list is empty
				a.updateVMDetails(nil)
			}
		}
	})

	// Trigger initial page change to populate the node list
	pages.SwitchToPage("Nodes")

	// Initialize search state for Nodes page
	models.GlobalState.SearchStates["Nodes"] = &models.SearchState{
		CurrentPage:   "Nodes",
		SearchText:    "",
		SelectedIndex: 0,
	}

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
