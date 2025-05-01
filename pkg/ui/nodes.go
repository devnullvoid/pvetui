package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-util/pkg/api"
	"github.com/rivo/tview"
)

// CreateNodeList creates a list of nodes
func CreateNodeList(nodes []api.Node) *tview.List {
	nodeList := tview.NewList().ShowSecondaryText(false)
	nodeList.SetBorder(true).SetTitle("Nodes")
	
	for _, n := range nodes {
		nodeList.AddItem(n.Name, "", 0, nil)
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
) (int, int) {
	var activeIndex, highlightedIndex int
	activeIndex = 0
	highlightedIndex = 0
	
	// Define updateSelected: refresh summary for node n
	updateSelected := func(n api.Node) {
		status, err := client.GetNodeStatus(n.Name)
		if err != nil {
			header.SetText(fmt.Sprintf("Error fetching status: %v", err))
		} else {
			updateSelectedWithStatus(n, status, summary)
		}
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
		
		// CPU metrics
		cpuPercent := toFloat(status["cpu"]) * 100
		var memUsed, memTotal float64
		if mObj, ok := status["memory"].(map[string]interface{}); ok {
			memUsed = toFloat(mObj["used"])
			memTotal = toFloat(mObj["total"])
		}
		rfUsed := toFloat(status["rootfs"].(map[string]interface{})["used"])
		rfTotal := toFloat(status["rootfs"].(map[string]interface{})["total"])
		
		// Load average
		loadStr := ""
		if la, ok := status["loadavg"].([]interface{}); ok {
			var sa []string
			for _, v := range la {
				sa = append(sa, fmt.Sprint(v))
			}
			loadStr = strings.Join(sa, "/")
		}
		
		// CPU info
		cores := int(toFloat(status["cpuinfo"].(map[string]interface{})["cores"]))
		threads := int(toFloat(status["cpuinfo"].(map[string]interface{})["cpus"]))
		model := ""
		if m, ok := status["cpuinfo"].(map[string]interface{})["model"].(string); ok {
			model = m
		}
		
		// Resource utilization
		detailsTable.SetCell(3, 0, tview.NewTableCell("🔝 CPU").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(3, 1, tview.NewTableCell(fmt.Sprintf("%.2f%%", cpuPercent)).SetTextColor(tcell.ColorWhite))
		detailsTable.SetCell(4, 0, tview.NewTableCell("💾 Mem").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(4, 1, tview.NewTableCell(fmt.Sprintf("%.0fMiB/%.0fMiB", memUsed/1024/1024, memTotal/1024/1024)).SetTextColor(tcell.ColorWhite))
		detailsTable.SetCell(5, 0, tview.NewTableCell("📈 Load").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(5, 1, tview.NewTableCell(loadStr).SetTextColor(tcell.ColorWhite))
		detailsTable.SetCell(6, 0, tview.NewTableCell("💽 RootFS").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(6, 1, tview.NewTableCell(fmt.Sprintf("%.0fMiB/%.0fMiB", rfUsed/1024/1024, rfTotal/1024/1024)).SetTextColor(tcell.ColorWhite))
		
		// CPU hardware
		detailsTable.SetCell(7, 0, tview.NewTableCell("💻 Model").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(7, 1, tview.NewTableCell(model).SetTextColor(tcell.ColorWhite))
		detailsTable.SetCell(8, 0, tview.NewTableCell("🧆 Cores").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(8, 1, tview.NewTableCell(fmt.Sprintf("%d", cores)).SetTextColor(tcell.ColorWhite))
		detailsTable.SetCell(9, 0, tview.NewTableCell("🔀 Threads").SetTextColor(tcell.ColorYellow))
		detailsTable.SetCell(9, 1, tview.NewTableCell(fmt.Sprintf("%d", threads)).SetTextColor(tcell.ColorWhite))
	}
	
	// Auto-refresh summary and details every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if activeIndex >= 0 && activeIndex < len(nodes) {
				// Fetch node data outside UI thread
				go func(node api.Node, idx int) {
					updateSelected(node)
					
					// Update details if needed
					if highlightedIndex >= 0 && highlightedIndex < len(nodes) {
						app.QueueUpdateDraw(func() {
							updateDetails(highlightedIndex, "", "", 0)
						})
					}
				}(nodes[activeIndex], activeIndex)
			}
		}
	}()
	
	// Set up change function to update details
	nodeList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		highlightedIndex = index
		updateDetails(index, mainText, secondaryText, shortcut)
	})
	
	// Set up selected function to update active node and refresh details
	nodeList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(nodes) {
			activeIndex = index
			highlightedIndex = index
			updateSelected(nodes[activeIndex])
			updateDetails(highlightedIndex, "", "", 0)
		}
	})
	
	// Initial summary and details load for active/highlighted nodes
	if activeIndex >= 0 && activeIndex < len(nodes) {
		updateSelected(nodes[activeIndex])
		updateDetails(highlightedIndex, "", "", 0)
	}
	
	return activeIndex, highlightedIndex
}

// Helper function to update summary with pre-fetched status data
func updateSelectedWithStatus(n api.Node, status map[string]interface{}, summary *tview.Table) {
	// Compute metrics from provided status
	var memUsed, memTotal float64
	if mObj, ok := status["memory"].(map[string]interface{}); ok {
		memUsed = toFloat(mObj["used"])
		memTotal = toFloat(mObj["total"])
	} else {
		memUsed = toFloat(status["mem"])
		memTotal = toFloat(status["maxmem"])
	}
	
	rfUsed := toFloat(status["rootfs"].(map[string]interface{})["used"])
	rfTotal := toFloat(status["rootfs"].(map[string]interface{})["total"])
	cpuPercent := toFloat(status["cpu"]) * 100
	
	// Load average
	loadStr := ""
	if la, ok := status["loadavg"].([]interface{}); ok {
		var sa []string
		for _, v := range la {
			sa = append(sa, fmt.Sprint(v))
		}
		loadStr = strings.Join(sa, "/")
	}
	
	// PVE version trimmed
	pveVer := ""
	if pv, ok := status["pveversion"].(string); ok {
		parts := strings.Split(pv, "/")
		if len(parts) >= 2 {
			pveVer = parts[1]
		} else {
			pveVer = pv
		}
	}
	
	// Kernel trimmed
	kernelRel := ""
	if ck, ok := status["current-kernel"].(map[string]interface{}); ok {
		if rs, ok2 := ck["release"].(string); ok2 {
			kernelRel = strings.Split(rs, " ")[0]
		}
	}
	
	// CPU hardware info handled in details panel
	cores := int(toFloat(status["cpuinfo"].(map[string]interface{})["cores"]))
	threads := int(toFloat(status["cpuinfo"].(map[string]interface{})["cpus"]))
	model := ""
	if m, ok := status["cpuinfo"].(map[string]interface{})["model"].(string); ok {
		model = m
	}
	
	// Populate summary: 3-item rows with colored labels and values
	summary.Clear()
	
	// Row 0: Node, PVE, Kernel with icons
	summary.SetCell(0, 0, tview.NewTableCell("📶 Node").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	summary.SetCell(0, 1, tview.NewTableCell(n.Name).SetTextColor(tcell.ColorWhite))
	summary.SetCell(0, 2, tview.NewTableCell("📛 PVE").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	summary.SetCell(0, 3, tview.NewTableCell(pveVer).SetTextColor(tcell.ColorWhite))
	summary.SetCell(0, 4, tview.NewTableCell("🔌 Kernel").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	summary.SetCell(0, 5, tview.NewTableCell(kernelRel).SetTextColor(tcell.ColorWhite))
	
	// Row 1: CPU, Mem, Load, RootFS with icons
	summary.SetCell(1, 0, tview.NewTableCell("🔝 CPU").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	summary.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%.2f%%", cpuPercent)).SetTextColor(tcell.ColorWhite))
	summary.SetCell(1, 2, tview.NewTableCell("💾 Mem").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	summary.SetCell(1, 3, tview.NewTableCell(fmt.Sprintf("%.0fMiB/%.0fMiB", memUsed/1024/1024, memTotal/1024/1024)).SetTextColor(tcell.ColorWhite))
	summary.SetCell(1, 4, tview.NewTableCell("📈 Load").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	summary.SetCell(1, 5, tview.NewTableCell(loadStr).SetTextColor(tcell.ColorWhite))
	summary.SetCell(1, 6, tview.NewTableCell("💽 RootFS").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	summary.SetCell(1, 7, tview.NewTableCell(fmt.Sprintf("%.0fMiB/%.0fMiB", rfUsed/1024/1024, rfTotal/1024/1024)).SetTextColor(tcell.ColorWhite))
	
	// Row 2: Model, Cores, Threads with icons
	summary.SetCell(2, 0, tview.NewTableCell("💻 Model").SetTextColor(tcell.ColorYellow))
	summary.SetCell(2, 1, tview.NewTableCell(model).SetTextColor(tcell.ColorWhite))
	summary.SetCell(2, 2, tview.NewTableCell("🧆 Cores").SetTextColor(tcell.ColorYellow))
	summary.SetCell(2, 3, tview.NewTableCell(fmt.Sprintf("%d", cores)).SetTextColor(tcell.ColorWhite))
	summary.SetCell(2, 4, tview.NewTableCell("🔀 Threads").SetTextColor(tcell.ColorYellow))
	summary.SetCell(2, 5, tview.NewTableCell(fmt.Sprintf("%d", threads)).SetTextColor(tcell.ColorWhite))
}
