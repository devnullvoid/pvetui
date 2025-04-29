package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-util/pkg/api"
	"github.com/rivo/tview"
)

// NewAppUI creates the root UI component with node tree and VM list.
func NewAppUI(app *tview.Application, client *api.Client) tview.Primitive {
	// Header
	header := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("Proxmox CLI UI")

	// Summary panel as table
	summary := tview.NewTable().SetBorders(false)
	summary.SetBorder(true)
	summary.SetTitle("Node Summary")

	// Model panel as table
	modelTable := tview.NewTable().SetBorders(false)
	modelTable.SetBorder(true)
	modelTable.SetTitle("Node CPU")

	// Combined summary and model into a flex for right-aligned modelTable
	summaryFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(summary, 0, 4, false).
		AddItem(modelTable, 0, 1, false)

	// Node tree
	nodes, err := client.ListNodes()
	if err != nil {
		header.SetText(fmt.Sprintf("Error listing nodes: %v", err))
	}
	rootNode := tview.NewTreeNode("Nodes").SetColor(tcell.ColorGreen)
	tree := tview.NewTreeView().
		SetRoot(rootNode).
		SetCurrentNode(rootNode)
	tree.SetBorder(true)
	tree.SetTitle("Nodes")
	for _, n := range nodes {
		node := tview.NewTreeNode(n.Name).
			SetReference(n).
			SetSelectable(true)
		rootNode.AddChild(node)
	}
	// Auto-select first child node by default
	if len(rootNode.GetChildren()) > 0 {
		tree.SetCurrentNode(rootNode.GetChildren()[0])
	}

	// VM table
	table := tview.NewTable().SetBorders(true)
	table.SetBorder(true)
	table.SetTitle("VMs")
	// Table header
	table.SetCell(0, 0, tview.NewTableCell("VM ID").SetAttributes(tcell.AttrBold))
	table.SetCell(0, 1, tview.NewTableCell("Name").SetAttributes(tcell.AttrBold))
	table.SetCell(0, 2, tview.NewTableCell("Type").SetAttributes(tcell.AttrBold))

	// Helper to convert various types to float64
	toFloat := func(val interface{}) float64 {
		if val == nil {
			return 0
		}
		switch t := val.(type) {
		case float64:
			return t
		case float32:
			return float64(t)
		case int:
			return float64(t)
		case int8:
			return float64(t)
		case int16:
			return float64(t)
		case int32:
			return float64(t)
		case int64:
			return float64(t)
		case uint:
			return float64(t)
		case uint8:
			return float64(t)
		case uint16:
			return float64(t)
		case uint32:
			return float64(t)
		case uint64:
			return float64(t)
		case json.Number:
			if f, err := t.Float64(); err == nil {
				return f
			}
		case string:
			if f, err := strconv.ParseFloat(t, 64); err == nil {
				return f
			}
		}
		// Fallback: try generic string representation
		if s := fmt.Sprint(val); s != "" {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
		}
		return 0
	}

	// Prepare pages (tabs)
	pages := tview.NewPages()
	// Nodes tab
	nodesContent := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tree, 0, 1, true)
	pages.AddPage("Nodes", nodesContent, true, false)
	// VMs/LXCs tab
	vmsTable := tview.NewTable().SetBorders(true)
	vmsTable.SetBorder(true)
	vmsTable.SetTitle("VMs/LXCs")
	// Table header
	vmsTable.SetCell(0, 0, tview.NewTableCell("VM ID").SetAttributes(tcell.AttrBold))
	vmsTable.SetCell(0, 1, tview.NewTableCell("Name").SetAttributes(tcell.AttrBold))
	vmsTable.SetCell(0, 2, tview.NewTableCell("Node").SetAttributes(tcell.AttrBold))
	vmsTable.SetCell(0, 3, tview.NewTableCell("Type").SetAttributes(tcell.AttrBold))
	// Populate all VMs
	row := 1
	for _, n := range nodes {
		vms, _ := client.ListVMs(n.Name)
		for _, vm := range vms {
			vmsTable.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("%d", vm.ID)))
			vmsTable.SetCell(row, 1, tview.NewTableCell(vm.Name))
			vmsTable.SetCell(row, 2, tview.NewTableCell(vm.Node))
			vmsTable.SetCell(row, 3, tview.NewTableCell(vm.Type))
			row++
		}
	}
	pages.AddPage("VMs/LXCs", vmsTable, true, true)
	// Storage tab (TODO)
	storageView := tview.NewTextView().SetText("[::b]Storage view coming soon")
	storageView.SetBorder(true)
	storageView.SetTitle("Storage")
	pages.AddPage("Storage", storageView, true, false)
	// Network tab (TODO)
	networkView := tview.NewTextView().SetText("[::b]Network view coming soon")
	networkView.SetBorder(true)
	networkView.SetTitle("Network")
	pages.AddPage("Network", networkView, true, false)
	// Tasks/Logs tab (TODO)
	tasksView := tview.NewTextView().SetText("[::b]Tasks/Logs view coming soon")
	tasksView.SetBorder(true)
	tasksView.SetTitle("Tasks/Logs")
	pages.AddPage("Tasks/Logs", tasksView, true, false)

	// Tab navigation state
	pageNames := []string{"Nodes", "VMs/LXCs", "Storage", "Network", "Tasks/Logs"}
	currentTab := 0

	// Footer with key hints
	footer := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText("[yellow]F1:[white]Nodes  [yellow]F2:[white]VMs/LXCs  [yellow]F3:[white]Storage  [yellow]F4:[white]Network  [yellow]F5:[white]Tasks/Logs  [yellow]Tab:[white]Next Tab  [yellow]Q/Esc:[white]Quit")

	// Define updateSelected: refresh summary and VMs tab for node n
	updateSelected := func(n api.Node) {
		status, err := client.GetNodeStatus(n.Name)
		if err != nil {
			header.SetText(fmt.Sprintf("Error fetching status: %v", err))
		} else {
			// Compute metrics
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
			// CPU info
			cores := int(toFloat(status["cpuinfo"].(map[string]interface{})["cores"]))
			threads := int(toFloat(status["cpuinfo"].(map[string]interface{})["cpus"]))
			model := ""
			if m, ok := status["cpuinfo"].(map[string]interface{})["model"].(string); ok {
				model = m
			}
			// Populate summary: 3-item rows with colored labels and values
			summary.Clear()
			// Row 0: Node, PVE, Kernel
			summary.SetCell(0, 0, tview.NewTableCell("Node").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			summary.SetCell(0, 1, tview.NewTableCell(n.Name).SetTextColor(tcell.ColorWhite))
			summary.SetCell(0, 2, tview.NewTableCell("PVE").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			summary.SetCell(0, 3, tview.NewTableCell(pveVer).SetTextColor(tcell.ColorWhite))
			summary.SetCell(0, 4, tview.NewTableCell("Kernel").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			summary.SetCell(0, 5, tview.NewTableCell(kernelRel).SetTextColor(tcell.ColorWhite))
			// Row 1: CPU, Mem, Load
			summary.SetCell(1, 0, tview.NewTableCell("CPU").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			summary.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%.2f%%", cpuPercent)).SetTextColor(tcell.ColorWhite))
			summary.SetCell(1, 2, tview.NewTableCell("Mem").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			summary.SetCell(1, 3, tview.NewTableCell(fmt.Sprintf("%.0fMiB/%.0fMiB", memUsed/1024/1024, memTotal/1024/1024)).SetTextColor(tcell.ColorWhite))
			summary.SetCell(1, 4, tview.NewTableCell("Load").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			summary.SetCell(1, 5, tview.NewTableCell(loadStr).SetTextColor(tcell.ColorWhite))
			// Row 2: RootFS only
			summary.SetCell(2, 0, tview.NewTableCell("RootFS").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			summary.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%.0fMiB/%.0fMiB", rfUsed/1024/1024, rfTotal/1024/1024)).SetTextColor(tcell.ColorWhite))
			// Clear remaining summary columns
			summary.SetCell(2, 2, tview.NewTableCell(""))
			summary.SetCell(2, 3, tview.NewTableCell(""))
			summary.SetCell(2, 4, tview.NewTableCell(""))
			summary.SetCell(2, 5, tview.NewTableCell(""))
			// Populate modelTable with Model, Cores, Threads
			modelTable.Clear()
			modelTable.SetCell(0, 0, tview.NewTableCell("Model").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			modelTable.SetCell(0, 1, tview.NewTableCell(model).SetTextColor(tcell.ColorWhite))
			modelTable.SetCell(1, 0, tview.NewTableCell("Cores").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			modelTable.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%d", cores)).SetTextColor(tcell.ColorWhite))
			modelTable.SetCell(2, 0, tview.NewTableCell("Threads").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
			modelTable.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%d", threads)).SetTextColor(tcell.ColorWhite))
			// Update VMs/LXCs tab
			vmsTable.Clear()
			vmsTable.SetCell(0, 0, tview.NewTableCell("VM ID").SetAttributes(tcell.AttrBold))
			vmsTable.SetCell(0, 1, tview.NewTableCell("Name").SetAttributes(tcell.AttrBold))
			vmsTable.SetCell(0, 2, tview.NewTableCell("Node").SetAttributes(tcell.AttrBold))
			vmsTable.SetCell(0, 3, tview.NewTableCell("Type").SetAttributes(tcell.AttrBold))
			vms, err := client.ListVMs(n.Name)
			if err != nil {
				header.SetText(fmt.Sprintf("Error listing VMs: %v", err))
			} else {
				for i, vm := range vms {
					vmsTable.SetCell(i+1, 0, tview.NewTableCell(fmt.Sprintf("%d", vm.ID)))
					vmsTable.SetCell(i+1, 1, tview.NewTableCell(vm.Name))
					vmsTable.SetCell(i+1, 2, tview.NewTableCell(vm.Node))
					vmsTable.SetCell(i+1, 3, tview.NewTableCell(vm.Type))
				}
			}
		}
	}

	// On node select, refresh for selected node
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		if ref := node.GetReference(); ref != nil {
			if n, ok := ref.(api.Node); ok {
				updateSelected(n)
			}
		}
	})

	// Initial display: select first node by default
	if len(nodes) > 0 {
		updateSelected(nodes[0])
	}

	// Input capture: Tab/F-keys for navigation, Esc/Q to quit
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			currentTab = (currentTab + 1) % len(pageNames)
			pages.SwitchToPage(pageNames[currentTab])
			return nil
		case tcell.KeyF1:
			pages.SwitchToPage("Nodes")
			currentTab = 0
			return nil
		case tcell.KeyF2:
			pages.SwitchToPage("VMs/LXCs")
			currentTab = 1
			return nil
		case tcell.KeyF3:
			pages.SwitchToPage("Storage")
			currentTab = 2
			return nil
		case tcell.KeyF4:
			pages.SwitchToPage("Network")
			currentTab = 3
			return nil
		case tcell.KeyF5:
			pages.SwitchToPage("Tasks/Logs")
			currentTab = 4
			return nil
		case tcell.KeyEsc:
			app.Stop()
			return nil
		}
		if r := event.Rune(); r == 'q' || r == 'Q' {
			app.Stop()
			return nil
		}
		return event
	})

	// Auto-refresh summary every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			current := tree.GetCurrentNode()
			if ref := current.GetReference(); ref != nil {
				if n, ok := ref.(api.Node); ok {
					app.QueueUpdateDraw(func() {
						updateSelected(n)
					})
				}
			}
		}
	}()

	// Main layout: summary, pages, footer
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(summaryFlex, 5, 0, false).
		AddItem(pages, 0, 1, true).
		AddItem(footer, 1, 0, false)
	return mainFlex
}
