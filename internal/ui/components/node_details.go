package components

import (
	"fmt"
	"strings"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NodeDetails encapsulates the node details panel
type NodeDetails struct {
	*tview.Table
	app *App
}

// NewNodeDetails creates a new node details panel
func NewNodeDetails() *NodeDetails {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(false, false)
	table.SetTitle(" Node Details ")
	table.SetBorder(true)

	return &NodeDetails{
		Table: table,
	}
}

// SetApp sets the parent app reference for focus management
func (nd *NodeDetails) SetApp(app *App) {
	nd.app = app

	// Set up input capture for arrow keys
	nd.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			if nd.app != nil {
				nd.app.SetFocus(nd.app.nodeList)
				return nil
			}
		}
		return event
	})
}

// Update updates the node details panel with the given node
func (nd *NodeDetails) Update(node *api.Node, fullNodeList []*api.Node) {
	// Clear existing rows
	nd.Clear()

	if node == nil {
		nd.SetCell(0, 0, tview.NewTableCell("Select a node").SetTextColor(tcell.ColorWhite))
		return
	}

	row := 0

	// Basic info
	nd.SetCell(row, 0, tview.NewTableCell("📛 Name").SetTextColor(tcell.ColorYellow))
	nd.SetCell(row, 1, tview.NewTableCell(node.Name).SetTextColor(tcell.ColorWhite))
	row++

	// Status
	statusEmoji := "🟢"
	statusText := "Online"
	statusColor := tcell.ColorGreen
	if !node.Online {
		statusEmoji = "🔴"
		statusText = "Offline"
		statusColor = tcell.ColorRed
	}
	nd.SetCell(row, 0, tview.NewTableCell(statusEmoji+" Status").SetTextColor(tcell.ColorYellow))
	nd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))
	row++

	// CPU
	cpuInfo := fmt.Sprintf("%.1f%% of %.0f cores", node.CPUUsage*100, node.CPUCount)
	if node.CPUInfo != nil {
		cpuInfo = fmt.Sprintf("%.1f%% of %d cores (%d sockets)",
			node.CPUUsage*100, node.CPUInfo.Cores, node.CPUInfo.Sockets)

		if node.CPUInfo.Model != "" {
			cpuInfo += "\n" + node.CPUInfo.Model
		}
	}
	nd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(tcell.ColorYellow))
	nd.SetCell(row, 1, tview.NewTableCell(cpuInfo).SetTextColor(tcell.ColorWhite))
	row++

	// Load Average
	if len(node.LoadAvg) > 0 {
		loadStr := strings.Join(node.LoadAvg, ", ")
		nd.SetCell(row, 0, tview.NewTableCell("📊 Load Avg").SetTextColor(tcell.ColorYellow))
		nd.SetCell(row, 1, tview.NewTableCell(loadStr).SetTextColor(tcell.ColorWhite))
		row++
	}

	// Memory
	nd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(tcell.ColorYellow))
	nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.1f GB / %.1f GB (%.1f%%)",
		node.MemoryUsed,
		node.MemoryTotal,
		(node.MemoryUsed/node.MemoryTotal)*100)).SetTextColor(tcell.ColorWhite))
	row++

	// Storage
	storageGB := float64(node.UsedStorage) / 1024 / 1024 / 1024
	totalStorageGB := float64(node.TotalStorage) / 1024 / 1024 / 1024
	storagePercent := 0.0
	if node.TotalStorage > 0 {
		storagePercent = (float64(node.UsedStorage) / float64(node.TotalStorage)) * 100
	}

	nd.SetCell(row, 0, tview.NewTableCell("💾 Storage").SetTextColor(tcell.ColorYellow))
	nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.1f GB / %.1f GB (%.1f%%)",
		storageGB, totalStorageGB, storagePercent)).SetTextColor(tcell.ColorWhite))
	row++

	// Uptime
	if node.Uptime > 0 {
		nd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(tcell.ColorYellow))
		nd.SetCell(row, 1, tview.NewTableCell(utils.FormatUptime(int(node.Uptime))).SetTextColor(tcell.ColorWhite))
		row++
	}

	// Version
	if node.Version != "" {
		nd.SetCell(row, 0, tview.NewTableCell("🔄 Version").SetTextColor(tcell.ColorYellow))
		nd.SetCell(row, 1, tview.NewTableCell(node.Version).SetTextColor(tcell.ColorWhite))
		row++
	}

	// Kernel Version
	if node.KernelVersion != "" {
		nd.SetCell(row, 0, tview.NewTableCell("🐧 Kernel").SetTextColor(tcell.ColorYellow))
		nd.SetCell(row, 1, tview.NewTableCell(node.KernelVersion).SetTextColor(tcell.ColorWhite))
		row++
	}

	// IP Address
	if node.IP != "" {
		nd.SetCell(row, 0, tview.NewTableCell("📡 IP").SetTextColor(tcell.ColorYellow))
		nd.SetCell(row, 1, tview.NewTableCell(node.IP).SetTextColor(tcell.ColorWhite))
		row++
	}

	// VM Count
	vmCount := 0
	if node.VMs != nil {
		vmCount = len(node.VMs)
	}
	nd.SetCell(row, 0, tview.NewTableCell("📦 VMs").SetTextColor(tcell.ColorYellow))
	nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", vmCount)).SetTextColor(tcell.ColorWhite))
	row++
}
