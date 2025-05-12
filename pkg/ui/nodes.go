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
			fmt.Sprintf("IP: %s | Version: %s", n.IP, func() string {
				if n.Version == "" {
					return "unknown"
				}
				parts := strings.Split(n.Version, "/")
				if len(parts) > 1 {
					return "v" + parts[1]
				}
				return "v" + parts[0]
			}()),
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
	cluster *api.Cluster, // Added cluster parameter
	nodeList *tview.List,
	nodes []api.Node,
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

		// Update summary panel with existing cluster data
		summary.Clear()
		UpdateClusterStatus(summary, resourceTable, cluster) // Use passed cluster data
		// header.SetText(fmt.Sprintf("✅ Loaded %s", n.Name)).SetTextColor(tcell.ColorGreen)
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
		// Format PVE version from "pve-manager/8.3.5/dac3aa88bac3f300" to "8.3.5"
		pveVerParts := strings.Split(status.Version, "/")
		pveVer := "unknown"
		if len(pveVerParts) >= 2 {
			pveVer = pveVerParts[1]
		}

		// Format kernel version from "6.8.12-8-pve" to "6.8.12"
		kernelParts := strings.Split(status.KernelVersion, "-")
		kernelRel := "unknown"
		if len(kernelParts) > 0 {
			kernelRel = kernelParts[0]
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

	return activeIndex, highlightedIndex, updateDetails
}
