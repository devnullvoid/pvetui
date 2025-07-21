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
	nd.SetCell(row, 0, tview.NewTableCell("🆔 ID").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.ID).SetTextColor(theme.Colors.Secondary))
	row++

	nd.SetCell(row, 0, tview.NewTableCell("📛 Name").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.Name).SetTextColor(theme.Colors.Secondary))
	row++

	nd.SetCell(row, 0, tview.NewTableCell("📍 IP").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(node.IP).SetTextColor(theme.Colors.Secondary))
	row++

	// Status Info
	statusText := "Online"
	if !node.Online {
		statusText = "Offline"
	}

	// Determine color for status text
	var statusColor tcell.Color
	var statusEmoji string

	if node.Online {
		statusEmoji = "🟢"
		statusColor = theme.Colors.StatusRunning
	} else {
		statusEmoji = "🔴"
		statusColor = theme.Colors.StatusStopped
	}

	nd.SetCell(row, 0, tview.NewTableCell(statusEmoji+" Status").SetTextColor(theme.Colors.Primary))
	nd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))
	row++

	// Resource Usage
	nd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(theme.Colors.Primary))
	cpuValue := api.StringNA
	if node.CPUUsage >= 0 {
		cpuValue = fmt.Sprintf("%.1f%%", node.CPUUsage*100)
	}
	nd.SetCell(row, 1, tview.NewTableCell(cpuValue).SetTextColor(theme.Colors.Secondary))
	row++

	nd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(theme.Colors.Primary))
	memValue := api.StringNA
	if node.MemoryTotal > 0 {
		memUsedFormatted := utils.FormatBytes(int64(node.MemoryUsed * 1073741824))   // Convert GB to bytes
		memTotalFormatted := utils.FormatBytes(int64(node.MemoryTotal * 1073741824)) // Convert GB to bytes
		memoryPercent := utils.CalculatePercentage(node.MemoryUsed, node.MemoryTotal)
		memValue = fmt.Sprintf("%.2f%% (%s) / %s", memoryPercent, memUsedFormatted, memTotalFormatted)
	}
	nd.SetCell(row, 1, tview.NewTableCell(memValue).SetTextColor(theme.Colors.Secondary))
	row++

	nd.SetCell(row, 0, tview.NewTableCell("💾 Disk").SetTextColor(theme.Colors.Primary))
	diskValue := api.StringNA
	if node.TotalStorage > 0 {
		diskUsedFormatted := utils.FormatBytes(node.UsedStorage * 1073741824)   // Convert GB to bytes
		diskTotalFormatted := utils.FormatBytes(node.TotalStorage * 1073741824) // Convert GB to bytes
		diskPercent := utils.CalculatePercentageInt(node.UsedStorage, node.TotalStorage)
		diskValue = fmt.Sprintf("%.2f%% (%s) / %s", diskPercent, diskUsedFormatted, diskTotalFormatted)
	}
	nd.SetCell(row, 1, tview.NewTableCell(diskValue).SetTextColor(theme.Colors.Secondary))
	row++

	nd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(theme.Colors.Primary))
	uptimeValue := api.StringNA
	if node.Uptime > 0 {
		uptimeValue = utils.FormatUptime(int(node.Uptime))
	}
	nd.SetCell(row, 1, tview.NewTableCell(uptimeValue).SetTextColor(theme.Colors.Secondary))
	row++

	// VM Count
	nd.SetCell(row, 0, tview.NewTableCell("🖥️ VMs").SetTextColor(theme.Colors.Primary))
	vmCount := 0
	for _, n := range allNodes {
		if n.Name == node.Name {
			vmCount = len(n.VMs)
			break
		}
	}
	nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", vmCount)).SetTextColor(theme.Colors.Secondary))
	row++

	// Storage Information
	if node.Storage != nil {
		nd.SetCell(row, 0, tview.NewTableCell("💾 Storage").SetTextColor(theme.Colors.Primary))
		row++

		// Show storage information if available
		storage := node.Storage
		if storage.MaxDisk > 0 {
			// Calculate usage percentage
			var usedPercent float64
			if storage.MaxDisk > 0 {
				usedPercent = float64(storage.Disk) / float64(storage.MaxDisk) * 100
			} else {
				usedPercent = 0
			}

			// Choose color based on usage percentage
			usageColor := theme.GetUsageColor(usedPercent)

			nd.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("  • %s", storage.Name)).SetTextColor(theme.Colors.Info))
			nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s/%s)",
				usedPercent,
				utils.FormatBytes(storage.Disk),
				utils.FormatBytes(storage.MaxDisk))).SetTextColor(usageColor))
		} else {
			// Storage exists but has no size data
			nd.SetCell(row, 0, tview.NewTableCell("  • No size data").SetTextColor(theme.Colors.Info))
			nd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
		}
	} else {
		nd.SetCell(row, 0, tview.NewTableCell("💾 Storage").SetTextColor(theme.Colors.Primary))
		nd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
	}

	// Scroll to the top to ensure the most important information (basic details) is visible
	nd.ScrollToBeginning()
}
