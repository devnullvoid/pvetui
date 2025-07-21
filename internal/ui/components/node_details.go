package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// NodeDetails encapsulates the node details panel
type NodeDetails struct {
	*tview.Table
	app *App
}

var _ NodeDetailsComponent = (*NodeDetails)(nil)

// NewNodeDetails creates a new node details panel
func NewNodeDetails() *NodeDetails {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetTitle(" Node Details ")
	table.SetBorder(true)
	table.Clear()
	table.SetCell(0, 0, tview.NewTableCell("Select a node").SetTextColor(theme.Colors.Primary))

	return &NodeDetails{
		Table: table,
	}
}

// Clear wraps the table Clear method to satisfy the interface
func (nd *NodeDetails) Clear() *tview.Table {
	return nd.Table.Clear()
}

// SetApp sets the parent app reference for focus management
func (nd *NodeDetails) SetApp(app *App) {
	nd.app = app

	// Set up input capture for arrow keys and VI-like navigation (hjkl)
	nd.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			if nd.app != nil {
				nd.app.SetFocus(nd.app.nodeList)
				return nil
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'h': // VI-like left navigation
				if nd.app != nil {
					nd.app.SetFocus(nd.app.nodeList)
					return nil
				}
			case 'j': // VI-like down navigation
				// Let the table handle down navigation naturally
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k': // VI-like up navigation
				// Let the table handle up navigation naturally
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'l': // VI-like right navigation - no action for node details (already at rightmost)
				return nil
			}
		}
		return event
	})
}

// Update fills the node details table for the given node
func (nd *NodeDetails) Update(node *api.Node, allNodes []*api.Node) {
	if node == nil {
		nd.Clear()
		nd.SetCell(0, 0, tview.NewTableCell("Select a node").SetTextColor(theme.Colors.Primary))
		return
	}

	nd.Clear()
	row := 0

	// Basic Info
	nd.SetCell(row, 0, tview.NewTableCell("📛 Name").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.Name).SetTextColor(theme.Colors.Secondary))
	row++

	nd.SetCell(row, 0, tview.NewTableCell("🆔 ID").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.ID).SetTextColor(theme.Colors.Secondary))
	row++

	nd.SetCell(row, 0, tview.NewTableCell("📍 IP").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.IP).SetTextColor(theme.Colors.Secondary))
	row++

	// Status Info
	statusText := "Online"
	if !node.Online {
		statusText = "Offline"
	}
	var statusColor tcell.Color
	if node.Online {
		statusColor = theme.Colors.StatusRunning
	} else {
		statusColor = theme.Colors.StatusStopped
	}
	nd.SetCell(row, 0, tview.NewTableCell("🟢 Status").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))
	row++

	// CPU Usage
	nd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(theme.Colors.Primary))
	cpuValue := api.StringNA
	if node.CPUUsage >= 0 {
		if node.CPUInfo != nil && node.CPUInfo.Cores > 0 {
			cpuValue = fmt.Sprintf("%.1f%% of %d cores", node.CPUUsage*100, node.CPUInfo.Cores)
		} else {
			cpuValue = fmt.Sprintf("%.1f%%", node.CPUUsage*100)
		}
		if node.CPUInfo != nil && node.CPUInfo.Model != "" {
			cpuValue += " " + node.CPUInfo.Model
		}
	}
	nd.SetCell(row, 1, tview.NewTableCell(cpuValue).SetTextColor(theme.Colors.Secondary))
	row++

	// Load Average
	nd.SetCell(row, 0, tview.NewTableCell("📊 Load Avg").SetTextColor(theme.Colors.Primary))
	loadAvg := api.StringNA
	if len(node.LoadAvg) >= 3 {
		loadAvg = fmt.Sprintf("%s, %s, %s", node.LoadAvg[0], node.LoadAvg[1], node.LoadAvg[2])
	}
	nd.SetCell(row, 1, tview.NewTableCell(loadAvg).SetTextColor(theme.Colors.Secondary))
	row++

	// Memory Usage
	nd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(theme.Colors.Primary))
	memValue := api.StringNA
	if node.MemoryTotal > 0 {
		memUsedFormatted := utils.FormatBytes(int64(node.MemoryUsed * 1073741824))
		memTotalFormatted := utils.FormatBytes(int64(node.MemoryTotal * 1073741824))
		memoryPercent := utils.CalculatePercentage(node.MemoryUsed, node.MemoryTotal)
		memValue = fmt.Sprintf("%.2f%% (%s) / %s", memoryPercent, memUsedFormatted, memTotalFormatted)
	}
	nd.SetCell(row, 1, tview.NewTableCell(memValue).SetTextColor(theme.Colors.Secondary))
	row++

	// Storage Usage
	nd.SetCell(row, 0, tview.NewTableCell("💾 Storage").SetTextColor(theme.Colors.Primary))
	diskValue := api.StringNA
	if node.TotalStorage > 0 {
		diskUsedFormatted := utils.FormatBytes(node.UsedStorage * 1073741824)
		diskTotalFormatted := utils.FormatBytes(node.TotalStorage * 1073741824)
		diskPercent := utils.CalculatePercentageInt(node.UsedStorage, node.TotalStorage)
		diskValue = fmt.Sprintf("%.2f%% (%s) / %s", diskPercent, diskUsedFormatted, diskTotalFormatted)
	}
	nd.SetCell(row, 1, tview.NewTableCell(diskValue).SetTextColor(theme.Colors.Secondary))
	row++

	// Uptime
	nd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(theme.Colors.Primary))
	uptimeValue := api.StringNA
	if node.Uptime > 0 {
		uptimeValue = utils.FormatUptime(int(node.Uptime))
	}
	nd.SetCell(row, 1, tview.NewTableCell(uptimeValue).SetTextColor(theme.Colors.Secondary))
	row++

	// Version
	nd.SetCell(row, 0, tview.NewTableCell("🛠️ Version").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.Version).SetTextColor(theme.Colors.Secondary))
	row++

	// Kernel
	nd.SetCell(row, 0, tview.NewTableCell("🐧 Kernel").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.KernelVersion).SetTextColor(theme.Colors.Secondary))
	row++

	// CGroup Mode (int)
	if node.CGroupMode != 0 {
		nd.SetCell(row, 0, tview.NewTableCell("🧩 CGroup Mode").SetTextColor(theme.Colors.Primary))
		nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", node.CGroupMode)).SetTextColor(theme.Colors.Secondary))
		row++
	}
	// Level
	if node.Level != "" {
		nd.SetCell(row, 0, tview.NewTableCell("🔒 Level").SetTextColor(theme.Colors.Primary))
		nd.SetCell(row, 1, tview.NewTableCell(node.Level).SetTextColor(theme.Colors.Secondary))
		row++
	}

	// VMs (running/stopped/templates)
	vmRunning, vmStopped, vmTemplates := 0, 0, 0
	for _, n := range allNodes {
		if n.Name == node.Name {
			for _, vm := range n.VMs {
				switch vm.Status {
				case "running":
					vmRunning++
				case "stopped":
					vmStopped++
				}
				if vm.Template {
					vmTemplates++
				}
			}
			break
		}
	}
	vmText := fmt.Sprintf("[green]%d running[-], [red]%d stopped[-], [yellow]%d templates[-]", vmRunning, vmStopped, vmTemplates)
	nd.SetCell(row, 0, tview.NewTableCell("🖥️ VMs").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell("").SetText(fmt.Sprintf("%s", vmText)))
	row++

	// LXC (running/stopped)
	lxcRunning, lxcStopped := 0, 0
	for _, n := range allNodes {
		if n.Name == node.Name {
			for _, vm := range n.VMs {
				if vm.Type == "lxc" {
					if vm.Status == "running" {
						lxcRunning++
					} else {
						lxcStopped++
					}
				}
			}
			break
		}
	}
	lxcText := fmt.Sprintf("[green]%d running[-], [red]%d stopped[-]", lxcRunning, lxcStopped)
	nd.SetCell(row, 0, tview.NewTableCell("📦 LXC").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell("").SetText(fmt.Sprintf("%s", lxcText)))
	row++

	// Storage Information (per-pool breakdown)
	if node.Storage != nil {
		nd.SetCell(row, 0, tview.NewTableCell("💾 Storage").SetTextColor(theme.Colors.Primary))
		row++
		storage := node.Storage
		if storage.MaxDisk > 0 {
			var usedPercent float64
			if storage.MaxDisk > 0 {
				usedPercent = float64(storage.Disk) / float64(storage.MaxDisk) * 100
			} else {
				usedPercent = 0
			}
			usageColor := theme.GetUsageColor(usedPercent)
			nd.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("  • %s", storage.Name)).SetTextColor(theme.Colors.Info))
			nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s/%s)",
				usedPercent,
				utils.FormatBytes(storage.Disk),
				utils.FormatBytes(storage.MaxDisk))).SetTextColor(usageColor))
		} else {
			nd.SetCell(row, 0, tview.NewTableCell("  • No size data").SetTextColor(theme.Colors.Info))
			nd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
		}
	}

	nd.ScrollToBeginning()
}
