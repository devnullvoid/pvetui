package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	// "github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/rivo/tview"
)

// CreateNodeList creates a list of nodes
func CreateNodeList(nodes []*api.Node) *tview.List {
	nodeList := tview.NewList().ShowSecondaryText(false)
	nodeList.SetBorder(true).SetTitle("Nodes")

	if len(nodes) == 0 {
		nodeList.AddItem("No nodes available", "", 0, nil)
		return nodeList
	}

	for _, n := range nodes {
		if n == nil {
			continue
		}
		status := "🔴"
		if n.Online {
			status = "🟢"
		}
		nodeList.AddItem(fmt.Sprintf("%s %s", status, n.Name), "", 0, nil)
	}

	return nodeList
}

// DisplayNodeDetailsInTable populates the given table with detailed information for the selectedNode.
// It uses fullNodeList to find the most up-to-date status for the node.
func DisplayNodeDetailsInTable(detailsTable *tview.Table, selectedNode *api.Node, fullNodeList []*api.Node) {
	if selectedNode == nil {
		detailsTable.Clear()
		detailsTable.SetCell(0, 0, tview.NewTableCell("No node selected or node data is nil").SetTextColor(tcell.ColorRed))
		return
	}

	var status *api.Node
	for _, n := range fullNodeList {
		if n != nil && n.Name == selectedNode.Name {
			status = n
			break
		}
	}

	if status == nil {
		detailsTable.Clear()
		// Keep selectedNode.Name if status is not found, as selectedNode itself is not nil.
		detailsTable.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("Node %s not found in full list for detailed status", selectedNode.Name)).SetTextColor(tcell.ColorRed))
		return
	}

	// Fill detailsTable (logic extracted from original updateDetails)
	detailsTable.Clear()

	// Row 0: Node
	detailsTable.SetCell(0, 0, tview.NewTableCell("📶 Node").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(0, 1, tview.NewTableCell(status.Name).
		SetTextColor(tcell.ColorWhite))

	// PVE and Kernel
	pveVerParts := strings.Split(status.Version, "/")
	pveVer := "unknown"
	if len(pveVerParts) >= 2 {
		pveVer = pveVerParts[1]
	}

	kernelParts := strings.Split(status.KernelVersion, "-")
	kernelRel := "unknown"
	if len(kernelParts) > 0 {
		kernelRel = kernelParts[0]
	}
	detailsTable.SetCell(1, 0, tview.NewTableCell("📛 PVE").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(1, 1, tview.NewTableCell(pveVer).
		SetTextColor(tcell.ColorWhite))

	detailsTable.SetCell(2, 0, tview.NewTableCell("🔌 Kernel").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(2, 1, tview.NewTableCell(kernelRel).
		SetTextColor(tcell.ColorWhite))

	// Additional node details
	detailsTable.SetCell(3, 0, tview.NewTableCell("🌐 IP").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(3, 1, tview.NewTableCell(status.IP).
		SetTextColor(tcell.ColorWhite))

	detailsTable.SetCell(4, 0, tview.NewTableCell("⚡ CPU Usage").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(4, 1, tview.NewTableCell(fmt.Sprintf("%.1f%% of %.0f cores", status.CPUUsage*100, status.CPUCount)).
		SetTextColor(tcell.ColorWhite))

	// CPU Details
	cpuModel := "N/A"
	coresInfo := "N/A"
	if status.CPUInfo != nil {
		cpuModel = status.CPUInfo.Model
		coresInfo = fmt.Sprintf("%d cores, %d sockets", status.CPUInfo.Cores, status.CPUInfo.Sockets)
	}
	detailsTable.SetCell(5, 0, tview.NewTableCell("🖥️ CPU Model").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(5, 1, tview.NewTableCell(cpuModel).
		SetTextColor(tcell.ColorWhite))

	detailsTable.SetCell(6, 0, tview.NewTableCell("🔢 Cores/Sockets").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(6, 1, tview.NewTableCell(coresInfo).
		SetTextColor(tcell.ColorWhite))

	// Load averages
	loadAvg := "N/A"
	if len(status.LoadAvg) >= 3 {
		loadAvg = fmt.Sprintf("1m: %s, 5m: %s, 15m: %s", status.LoadAvg[0], status.LoadAvg[1], status.LoadAvg[2])
	}
	detailsTable.SetCell(7, 0, tview.NewTableCell("📈 Load Avg").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(7, 1, tview.NewTableCell(loadAvg).
		SetTextColor(tcell.ColorWhite))

	// Memory
	detailsTable.SetCell(8, 0, tview.NewTableCell("💾 Memory").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(8, 1, tview.NewTableCell(
		fmt.Sprintf("%.1f GB / %.1f GB",
			status.MemoryUsed,
			status.MemoryTotal),
	).SetTextColor(tcell.ColorWhite))

	// Storage
	detailsTable.SetCell(9, 0, tview.NewTableCell("💽 Storage").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(9, 1, tview.NewTableCell(
		fmt.Sprintf("%.1f GB / %.1f GB",
			float64(status.UsedStorage)/1024/1024/1024,
			float64(status.TotalStorage)/1024/1024/1024),
	).SetTextColor(tcell.ColorWhite))

	// Uptime
	uptimeDuration := time.Duration(status.Uptime) * time.Second
	detailsTable.SetCell(10, 0, tview.NewTableCell("⏱️ Uptime").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(10, 1, tview.NewTableCell(
		fmt.Sprintf("%d days %d hrs %d min",
			int(uptimeDuration.Hours()/24),
			int(uptimeDuration.Hours())%24,
			int(uptimeDuration.Minutes())%60),
	).SetTextColor(tcell.ColorWhite))

	onlineStatusText := "🔴 Offline"
	if status.Online {
		onlineStatusText = "🟢 Online"
	}
	detailsTable.SetCell(11, 0, tview.NewTableCell("📡 Status").
		SetTextColor(tcell.ColorYellow))
	detailsTable.SetCell(11, 1, tview.NewTableCell(onlineStatusText).
		SetTextColor(tcell.ColorWhite))
}

// SetupNodeHandlers configures the node list change and selection handlers
func SetupNodeHandlers(
	app *tview.Application,
	client *api.Client,
	cluster *api.Cluster, // Added cluster parameter
	nodeList *tview.List,
	nodes []*api.Node,
	summary *tview.Table,
	resourceTable *tview.Table,
	detailsTable *tview.Table,
	header *tview.TextView,
	detailsPages *tview.Pages,
) (int, int, func(int, string, string, rune)) {
	var activeIndex, highlightedIndex int
	activeIndex = 0
	highlightedIndex = 0

	// Define updateSelected: refresh summary for node n
	updateSelected := func(n *api.Node) {
		if n == nil {
			return
		}
		header.SetText(fmt.Sprintf("Loading %s...", n.Name)).SetTextColor(tcell.ColorYellow)
		summary.Clear()

		var status *api.Node
		for _, node := range cluster.Nodes {
			if node.Name == n.Name {
				status = node
				break
			}
		}

		if status == nil {
			header.SetText(fmt.Sprintf("Node %s not found in cluster", n.Name)).SetTextColor(tcell.ColorRed)
			return
		}

		// Update summary panel with existing cluster data
		summary.Clear()
		UpdateClusterStatus(summary, resourceTable, cluster) // Use passed cluster data
		
		// Show success message temporarily
		header.SetText(fmt.Sprintf("✅ Loaded %s", n.Name)).SetTextColor(tcell.ColorGreen)
		
		// Use a goroutine to handle the delayed update
		go func() {
			time.Sleep(2 * time.Second)
			app.QueueUpdateDraw(func() {
				header.SetText("Proxmox TUI").SetTextColor(tcell.ColorWhite)
			})
		}()
	}

	// Define updateDetails: refresh details for highlighted node
	updateDetailsLocal := func(index int, mainText string, secondaryText string, shortcut rune) {
		if index < 0 || index >= len(nodes) { // 'nodes' is the list SetupNodeHandlers was given (e.g. client.Cluster.Nodes)
			DisplayNodeDetailsInTable(detailsTable, nil, nil) // Clear table or show "no selection"
			return
		}
		selectedNodeFromList := nodes[index]
		// Pass client.Cluster.Nodes (or the 'cluster.Nodes' param) as the full list for status lookup
		DisplayNodeDetailsInTable(detailsTable, selectedNodeFromList, cluster.Nodes)
	}

	nodeList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		highlightedIndex = index
		updateDetailsLocal(index, mainText, secondaryText, shortcut)
	})

	nodeList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		activeIndex = index
		n := nodes[index]
		updateSelected(n)
	})

	return activeIndex, highlightedIndex, updateDetailsLocal
}
