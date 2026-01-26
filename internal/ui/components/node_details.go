package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// Common status constants.
const (
	statusRunning = "running"
	statusStopped = "stopped"
)

// NodeDetails encapsulates the node details panel.
type NodeDetails struct {
	*tview.Table

	app *App
}

var _ NodeDetailsComponent = (*NodeDetails)(nil)

// NewNodeDetails creates a new node details panel.
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

// Clear wraps the table Clear method to satisfy the interface.
func (nd *NodeDetails) Clear() *tview.Table {
	return nd.Table.Clear()
}

// SetApp sets the parent app reference for focus management.
func (nd *NodeDetails) SetApp(app *App) {
	nd.app = app

	// Set up input capture for arrow keys and VI-like navigation (hjkl)
	nd.SetInputCapture(createNavigationInputCapture(nd.app, nd.app.nodeList, nil))
}

// Update fills the node details table for the given node.
func (nd *NodeDetails) Update(node *api.Node, allNodes []*api.Node) {
	if node == nil {
		nd.Clear()
		nd.SetCell(0, 0, tview.NewTableCell("Select a node").SetTextColor(theme.Colors.Primary))

		return
	}

	nd.Clear()

	showIcons := true
	if nd.app != nil {
		showIcons = nd.app.config.ShowIcons
	}

	row := 0

	// Basic Info
	// nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Name", "📛", showIcons)).SetTextColor(theme.Colors.HeaderText))
	// nd.SetCell(row, 1, tview.NewTableCell(node.Name).SetTextColor(theme.Colors.Primary))
	// row++

	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("ID", "🆔", showIcons)).SetTextColor(theme.Colors.HeaderText))
	nd.SetCell(row, 1, tview.NewTableCell(node.ID).SetTextColor(theme.Colors.Primary))

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

	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Status", utils.GetStatusEmoji(statusText, showIcons), showIcons)).SetTextColor(theme.Colors.HeaderText))
	nd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))

	row++

	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("IP", "📡", showIcons)).SetTextColor(theme.Colors.HeaderText))
	nd.SetCell(row, 1, tview.NewTableCell(node.IP).SetTextColor(theme.Colors.Primary))

	row++

	// CPU Usage
	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("CPU", "🧮", showIcons)).SetTextColor(theme.Colors.HeaderText))

	cpuValue := api.StringNA
	cpuUsageColor := theme.Colors.Primary

	if node.CPUUsage >= 0 && node.CPUCount > 0 {
		cpuPercent := node.CPUUsage * 100
		cpuValue = fmt.Sprintf("%.1f%% of %.0f cores", cpuPercent, node.CPUCount)
		cpuUsageColor = theme.GetUsageColor(cpuPercent)
	} else if node.CPUUsage >= 0 {
		cpuPercent := node.CPUUsage * 100
		cpuValue = fmt.Sprintf("%.1f%%", cpuPercent)
		cpuUsageColor = theme.GetUsageColor(cpuPercent)
	}

	nd.SetCell(row, 1, tview.NewTableCell(cpuValue).SetTextColor(cpuUsageColor))

	row++

	// Load Average
	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Load Avg", "📊", showIcons)).SetTextColor(theme.Colors.HeaderText))

	loadAvg := api.StringNA
	if len(node.LoadAvg) >= 3 {
		loadAvg = fmt.Sprintf("%s, %s, %s", node.LoadAvg[0], node.LoadAvg[1], node.LoadAvg[2])
	}

	nd.SetCell(row, 1, tview.NewTableCell(loadAvg).SetTextColor(theme.Colors.Primary))

	row++

	// Memory Usage
	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Memory", "🧠", showIcons)).SetTextColor(theme.Colors.HeaderText))

	memValue := api.StringNA
	memUsageColor := theme.Colors.Primary

	if node.MemoryTotal > 0 {
		memUsedFormatted := utils.FormatBytes(int64(node.MemoryUsed * 1073741824))
		memTotalFormatted := utils.FormatBytes(int64(node.MemoryTotal * 1073741824))
		memoryPercent := utils.CalculatePercentage(node.MemoryUsed, node.MemoryTotal)
		memValue = fmt.Sprintf("%.2f%% (%s) / %s", memoryPercent, memUsedFormatted, memTotalFormatted)
		memUsageColor = theme.GetUsageColor(memoryPercent)
	}

	nd.SetCell(row, 1, tview.NewTableCell(memValue).SetTextColor(memUsageColor))

	row++

	// Storage Usage
	// Remove the Rootfs row

	// Uptime
	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Uptime", "🕒", showIcons)).SetTextColor(theme.Colors.HeaderText))

	uptimeValue := api.StringNA
	if node.Uptime > 0 {
		uptimeValue = utils.FormatUptime(int(node.Uptime))
	}

	nd.SetCell(row, 1, tview.NewTableCell(uptimeValue).SetTextColor(theme.Colors.Primary))

	row++

	// Version
	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Version", "🔧", showIcons)).SetTextColor(theme.Colors.HeaderText))
	nd.SetCell(row, 1, tview.NewTableCell(node.Version).SetTextColor(theme.Colors.Primary))

	row++

	// Kernel
	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Kernel", "🧬", showIcons)).SetTextColor(theme.Colors.HeaderText))

	kernelValue := node.KernelVersion
	if idx := strings.Index(kernelValue, "#"); idx != -1 {
		kernelValue = strings.TrimSpace(kernelValue[:idx])
	}

	nd.SetCell(row, 1, tview.NewTableCell(kernelValue).SetTextColor(theme.Colors.Primary))

	row++

	// CGroup Mode (int)
	if node.CGroupMode != 0 {
		nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("CGroup Mode", "🧩", showIcons)).SetTextColor(theme.Colors.HeaderText))
		nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", node.CGroupMode)).SetTextColor(theme.Colors.Primary))

		row++
	}
	// Level
	if node.Level != "" {
		nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Level", "📈", showIcons)).SetTextColor(theme.Colors.HeaderText))
		nd.SetCell(row, 1, tview.NewTableCell(node.Level).SetTextColor(theme.Colors.Primary))

		row++
	}

	// VMs (running/stopped/templates)
	vmRunning, vmStopped, vmTemplates := 0, 0, 0

	for _, n := range allNodes {
		if n.Name == node.Name {
			for _, vm := range n.VMs {
				switch vm.Status {
				case statusRunning:
					vmRunning++
				case statusStopped:
					vmStopped++
				}

				if vm.Template {
					vmTemplates++
				}
			}

			break
		}
	}

	greenTag := theme.ColorToTag(theme.Colors.StatusRunning)
	redTag := theme.ColorToTag(theme.Colors.StatusStopped)
	yellowTag := theme.ColorToTag(theme.Colors.Warning)
	vmText := fmt.Sprintf("[%s]%d running[-], [%s]%d stopped[-], [%s]%d templates[-]", greenTag, vmRunning, redTag, vmStopped, yellowTag, vmTemplates)

	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("VMs", "💻", showIcons)).SetTextColor(theme.Colors.HeaderText))
	nd.SetCell(row, 1, tview.NewTableCell(vmText))

	row++

	// LXC (running/stopped)
	lxcRunning, lxcStopped := 0, 0

	for _, n := range allNodes {
		if n.Name == node.Name {
			for _, vm := range n.VMs {
				if vm.Type == vmTypeLXC {
					if vm.Status == statusRunning {
						lxcRunning++
					} else {
						lxcStopped++
					}
				}
			}

			break
		}
	}

	lxcText := fmt.Sprintf("[%s]%d running[-], [%s]%d stopped[-]", greenTag, lxcRunning, redTag, lxcStopped)

	nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("LXC", "📦", showIcons)).SetTextColor(theme.Colors.HeaderText))
	nd.SetCell(row, 1, tview.NewTableCell(lxcText))

	row++

	// Storage Information (per-pool breakdown)
	if len(node.Storage) > 0 {
		nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Storage", "💾", showIcons)).SetTextColor(theme.Colors.HeaderText))

		row++

		for _, storage := range node.Storage {
			if storage.MaxDisk > 0 {
				var usedPercent float64
				if storage.MaxDisk > 0 {
					usedPercent = float64(storage.Disk) / float64(storage.MaxDisk) * 100
				} else {
					usedPercent = 0
				}

				usageColor := theme.GetUsageColor(usedPercent)
				nd.SetCell(row, 0, tview.NewTableCell("  • "+storage.Name).SetTextColor(theme.Colors.Info))
				nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s/%s)",
					usedPercent,
					utils.FormatBytes(storage.Disk),
					utils.FormatBytes(storage.MaxDisk))).SetTextColor(usageColor))

				row++
			} else {
				nd.SetCell(row, 0, tview.NewTableCell("  • "+storage.Name).SetTextColor(theme.Colors.Info))
				nd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Primary))

				row++
			}
			// Sub-row: storage type (with shared status if applicable)
			typeLabel := storage.Plugintype
			if storage.IsShared() {
				typeLabel += " (shared)"
			}

			nd.SetCell(row, 0, tview.NewTableCell("").SetTextColor(theme.Colors.Info))
			nd.SetCell(row, 1, tview.NewTableCell(typeLabel).SetTextColor(theme.Colors.Secondary))

			row++
		}
	}

	// Disks & SMART Info
	if len(node.Disks) > 0 {
		nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Disks", "💿", showIcons)).SetTextColor(theme.Colors.HeaderText))
		row++
		for _, disk := range node.Disks {
			// Skip partitions if any slipped through
			if disk.Used == "partition" {
				continue
			}

			diskInfo := fmt.Sprintf("%s (%s) - %s", disk.DevPath, utils.FormatBytes(disk.Size), disk.Model)
			nd.SetCell(row, 0, tview.NewTableCell("  • "+disk.Type).SetTextColor(theme.Colors.Info))
			nd.SetCell(row, 1, tview.NewTableCell(diskInfo).SetTextColor(theme.Colors.Primary))
			row++

			// SMART Status - Assuming health is available directly in disk list or enriched elsewhere
			// The current node.Disks struct has Health field
			smartStatus := fmt.Sprintf("Status: %s", disk.Health)
			smartColor := theme.Colors.Secondary
			if disk.Health == "PASSED" || disk.Health == "OK" {
				smartColor = theme.Colors.StatusRunning
			} else if disk.Health != "" && disk.Health != "UNKNOWN" {
				smartColor = theme.Colors.StatusStopped
			}

			nd.SetCell(row, 1, tview.NewTableCell(smartStatus).SetTextColor(smartColor))
			row++
		}
	}

	// System Updates
	if len(node.Updates) > 0 {
		nd.SetCell(row, 0, tview.NewTableCell(utils.GetIconLabel("Updates", "📦", showIcons)).SetTextColor(theme.Colors.HeaderText))
		updateText := fmt.Sprintf("%d updates available", len(node.Updates))
		nd.SetCell(row, 1, tview.NewTableCell(updateText).SetTextColor(theme.Colors.Warning))
		row++

		// Show first few updates as preview
		limit := 5
		for i, update := range node.Updates {
			if i >= limit {
				nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("...and %d more", len(node.Updates)-limit)).SetTextColor(theme.Colors.Secondary))
				// Note: row++ removed here as it's not used after this point
				break
			}
			nd.SetCell(row, 0, tview.NewTableCell("  • "+update.Package).SetTextColor(theme.Colors.Info))
			nd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%s -> %s", update.OldVersion, update.Version)).SetTextColor(theme.Colors.Secondary))
			row++
		}
	}

	nd.ScrollToBeginning()
}
