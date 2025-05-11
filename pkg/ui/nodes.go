package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// CreateNodeList creates a list of nodes
func CreateNodeList(nodes []api.Node) *tview.List {
	nodeList := tview.NewList().ShowSecondaryText(false)
	nodeList.SetBorder(true).SetTitle("Nodes")

	if len(nodes) == 0 {
		nodeList.AddItem("No nodes available", "", 0, nil)
		return nodeList
	}

	for i, n := range nodes {
		status := "🔴 Offline"
		if n.Online {
			status = "🟢 Online"
		}
		nodeList.AddItem(
			fmt.Sprintf("%s %s", status, n.Name),
			fmt.Sprintf("IP: %s | Version: %s", n.IP, n.Version),
			0,
			nil,
		)
		fmt.Printf("DEBUG - Node %d: %+v\n", i, n)
	}

	return nodeList
}

// SetupNodeHandlers configures the node list change and selection handlers
func SetupNodeHandlers(
	app *tview.Application,
	client *api.Client,
	nodeList *tview.List,
	nodes []api.Node,
	summary *tview.Table,
	detailsTable *tview.Table,
	header *tview.TextView,
	detailsPages *tview.Pages,
) (int, int) {
	var activeIndex, highlightedIndex int
	activeIndex = 0
	highlightedIndex = 0

	// Define updateSelected: refresh summary for node n
	updateSelected := func(n api.Node) {
		header.SetText(fmt.Sprintf("Loading %s...", n.Name)).SetTextColor(tcell.ColorYellow)
		summary.Clear()

		status, err := client.GetNodeStatus(n.Name)
		if err != nil {
			header.SetText(fmt.Sprintf("Error fetching %s: %v", n.Name, err)).SetTextColor(tcell.ColorRed)
			return
		}

		if status == nil {
			header.SetText(fmt.Sprintf("No data for %s", n.Name)).SetTextColor(tcell.ColorOrange)
			return
		}

		// Update summary panel with live data
		summary.Clear()
		UpdateSummary(summary, n, status)
		header.SetText(fmt.Sprintf("✅ Loaded %s", n.Name)).SetTextColor(tcell.ColorGreen)
	}

	// Define updateDetails: refresh details for highlighted node
	updateDetails := func(index int, mainText string, secondaryText string, shortcut rune) {
		if index < 0 || index >= len(nodes) {
			return
		}
		n := nodes[index]
		status, err := client.GetNodeStatus(n.Name)
		if err != nil {
			detailsTable.Clear()
			detailsTable.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).SetTextColor(tcell.ColorRed))
			return
		}

		// Fill detailsTable like summary
		detailsTable.Clear()

		// Row 0: Node
		detailsTable.SetCell(0, 0, tview.NewTableCell("📶 Node").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(0, 1, tview.NewTableCell(n.Name).SetTextColor(tcell.ColorWhite))

		// PVE and Kernel
		var pveVer, kernelRel string
		if pv, ok := status["pveversion"].(string); ok {
			parts := strings.Split(pv, "/")
			if len(parts) >= 2 {
				pveVer = parts[1]
			} else {
				pveVer = pv
			}
		}
		if ck, ok := status["current-kernel"].(map[string]interface{}); ok {
			if rs, ok2 := ck["release"].(string); ok2 {
				kernelRel = strings.Split(rs, " ")[0]
			}
		}
		detailsTable.SetCell(1, 0, tview.NewTableCell("📛 PVE").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(1, 1, tview.NewTableCell(pveVer).SetTextColor(tcell.ColorWhite))
		detailsTable.SetCell(2, 0, tview.NewTableCell("🔌 Kernel").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(2, 1, tview.NewTableCell(kernelRel).SetTextColor(tcell.ColorWhite))
	}

	nodeList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		highlightedIndex = index
		updateDetails(index, mainText, secondaryText, shortcut)
	})

	nodeList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		activeIndex = index
		n := nodes[index]
		updateSelected(n)
	})

	return activeIndex, highlightedIndex
}
