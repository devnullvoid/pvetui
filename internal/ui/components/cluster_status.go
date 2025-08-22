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

// ClusterStatus encapsulates the cluster status panel.
type ClusterStatus struct {
	*tview.Flex

	SummaryTable  *tview.Table
	ResourceTable *tview.Table
	app           *App
}

var _ ClusterStatusComponent = (*ClusterStatus)(nil)

// NewClusterStatus creates a new cluster status panel.
func NewClusterStatus() *ClusterStatus {
	// Create container panel
	panel := tview.NewFlex()
	panel.SetDirection(tview.FlexColumn)
	panel.SetBorder(true)
	panel.SetTitle(" Cluster Status ")

	// Create summary table
	summary := tview.NewTable()
	summary.SetBorders(false)
	summary.SetTitleAlign(tview.AlignLeft)

	// Initial cluster status rows (no header, "Details" column removed)
	rows := [][]string{
		{"Cluster", "Loading..."},
		{"PVE Version", "Loading..."},
		{"Nodes Online", "Loading..."},
		{"Quorate", "Loading..."},
	}
	for row, fields := range rows {
		for col, text := range fields {
			cell := tview.NewTableCell(text).
				SetTextColor(theme.Colors.Primary).
				SetAlign(tview.AlignLeft)
			summary.SetCell(row, col, cell) // Data starts at row 0
		}
	}

	// Create resource table
	resourceTable := tview.NewTable()
	resourceTable.SetBorders(false)
	resourceTable.SetTitleAlign(tview.AlignLeft)

	// Resource table headers
	resourceHeaders := []string{"Resource", "Used", "Total"}
	for col, text := range resourceHeaders {
		cell := tview.NewTableCell(text).
			SetTextColor(theme.Colors.HeaderText).
			SetAlign(tview.AlignLeft)
		resourceTable.SetCell(0, col, cell)
	}

	// Add both tables to panel with equal space
	panel.AddItem(summary, 0, 1, false)
	panel.AddItem(resourceTable, 0, 1, false)

	return &ClusterStatus{
		Flex:          panel,
		SummaryTable:  summary,
		ResourceTable: resourceTable,
	}
}

// SetApp sets the application reference.
func (cs *ClusterStatus) SetApp(app *App) {
	cs.app = app
}

// Update populates both tables with current cluster data.
func (cs *ClusterStatus) Update(cluster *api.Cluster) {
	if cluster == nil {
		return
	}

	// Update summary table
	cs.SummaryTable.SetCell(0, 0, tview.NewTableCell("Cluster Name").SetTextColor(theme.Colors.HeaderText))
	cs.SummaryTable.SetCell(0, 1, tview.NewTableCell(cluster.Name).SetTextColor(theme.Colors.Primary))

	// Show only the version number (e.g., '8.3.5') in the 'Proxmox VE' row
	ver := cluster.Version
	if parts := strings.Split(ver, "/"); len(parts) > 1 {
		ver = parts[1]
	}

	cs.SummaryTable.SetCell(1, 0, tview.NewTableCell("Proxmox VE").SetTextColor(theme.Colors.HeaderText))
	cs.SummaryTable.SetCell(1, 1, tview.NewTableCell(ver).SetTextColor(theme.Colors.Primary))

	cs.SummaryTable.SetCell(2, 0, tview.NewTableCell("Nodes Online").SetTextColor(theme.Colors.HeaderText))

	// Show different indicators based on node status with proper colors
	var nodeStatusText string

	var nodeStatusColor tcell.Color

	if cluster.OnlineNodes == cluster.TotalNodes {
		// All nodes online
		nodeStatusText = fmt.Sprintf("%d/%d 🟢", cluster.OnlineNodes, cluster.TotalNodes)
		nodeStatusColor = theme.Colors.StatusRunning
	} else if cluster.OnlineNodes > 0 {
		// Some nodes offline
		nodeStatusText = fmt.Sprintf("%d/%d ⚠️", cluster.OnlineNodes, cluster.TotalNodes)
		nodeStatusColor = theme.Colors.Warning
	} else {
		// All nodes offline (critical)
		nodeStatusText = fmt.Sprintf("%d/%d 🔴", cluster.OnlineNodes, cluster.TotalNodes)
		nodeStatusColor = theme.Colors.StatusStopped
	}

	cs.SummaryTable.SetCell(2, 1, tview.NewTableCell(nodeStatusText).SetTextColor(nodeStatusColor))

	// Quorate status
	cs.SummaryTable.SetCell(3, 0, tview.NewTableCell("Quorate").SetTextColor(theme.Colors.HeaderText))

	var quorateText string

	var quorateColor tcell.Color

	if cluster.Quorate {
		quorateText = "Yes 🟢"
		quorateColor = theme.Colors.StatusRunning
	} else {
		quorateText = "No  🔴"
		quorateColor = theme.Colors.StatusStopped
	}

	cs.SummaryTable.SetCell(3, 1, tview.NewTableCell(quorateText).SetTextColor(quorateColor))

	// Update resource table (headers are already set in NewClusterStatus)
	// CPU row
	cpuUsageColor := theme.GetUsageColor(cluster.CPUUsage * 100)
	cs.ResourceTable.SetCell(1, 0, tview.NewTableCell("CPU Cores").SetTextColor(theme.Colors.Info).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%.1f%%", cluster.CPUUsage*100)).SetTextColor(cpuUsageColor).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(1, 2, tview.NewTableCell(fmt.Sprintf("%.1f", cluster.TotalCPU)).SetTextColor(theme.Colors.Primary).SetAlign(tview.AlignLeft))

	// Memory row
	memoryUsed := utils.FormatBytesFloat(cluster.MemoryUsed)
	memoryTotal := utils.FormatBytesFloat(cluster.MemoryTotal)
	memoryPercent := utils.CalculatePercentage(cluster.MemoryUsed, cluster.MemoryTotal)
	memoryUsageColor := theme.GetUsageColor(memoryPercent)
	cs.ResourceTable.SetCell(2, 0, tview.NewTableCell("Memory").SetTextColor(theme.Colors.Info).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s)", memoryPercent, memoryUsed)).SetTextColor(memoryUsageColor).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(2, 2, tview.NewTableCell(memoryTotal).SetTextColor(theme.Colors.Primary).SetAlign(tview.AlignLeft))

	// Storage row
	storageUsed := utils.FormatBytes(cluster.StorageUsed)
	storageTotal := utils.FormatBytes(cluster.StorageTotal)
	storagePercent := utils.CalculatePercentageInt(cluster.StorageUsed, cluster.StorageTotal)
	storageUsageColor := theme.GetUsageColor(storagePercent)
	cs.ResourceTable.SetCell(3, 0, tview.NewTableCell("Storage").SetTextColor(theme.Colors.Info).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(3, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s)", storagePercent, storageUsed)).SetTextColor(storageUsageColor).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(3, 2, tview.NewTableCell(storageTotal).SetTextColor(theme.Colors.Primary).SetAlign(tview.AlignLeft))
}
