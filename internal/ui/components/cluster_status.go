package components

import (
	"fmt"
	"strings"

	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ClusterStatus encapsulates the cluster status panel
type ClusterStatus struct {
	*tview.Flex
	SummaryTable  *tview.Table
	ResourceTable *tview.Table
}

// NewClusterStatus creates a new cluster status panel
func NewClusterStatus() *ClusterStatus {
	// Create container panel
	panel := tview.NewFlex()
	panel.SetDirection(tview.FlexColumn)
	panel.SetBorder(true)
	panel.SetTitle("Cluster Status")

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
				SetTextColor(tcell.ColorWhite).
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
			SetTextColor(tcell.ColorYellow).
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

// Update populates both tables with current cluster data
func (cs *ClusterStatus) Update(cluster *api.Cluster) {
	if cluster == nil {
		return
	}

	// Update summary table
	cs.SummaryTable.SetCell(0, 0, tview.NewTableCell("Cluster Name").SetTextColor(tcell.ColorYellow))
	cs.SummaryTable.SetCell(0, 1, tview.NewTableCell(cluster.Name).SetTextColor(tcell.ColorWhite))

	// Show only the version number (e.g., '8.3.5') in the 'Proxmox VE' row
	ver := cluster.Version
	if parts := strings.Split(ver, "/"); len(parts) > 1 {
		ver = parts[1]
	}
	cs.SummaryTable.SetCell(1, 0, tview.NewTableCell("Proxmox VE").SetTextColor(tcell.ColorYellow))
	cs.SummaryTable.SetCell(1, 1, tview.NewTableCell(ver).SetTextColor(tcell.ColorWhite))

	cs.SummaryTable.SetCell(2, 0, tview.NewTableCell("Nodes Online").SetTextColor(tcell.ColorYellow))

	// Show different indicators based on node status
	var nodeStatusText string
	if cluster.OnlineNodes == cluster.TotalNodes {
		// All nodes online
		nodeStatusText = fmt.Sprintf("[green]%d/%d 🟢[-]", cluster.OnlineNodes, cluster.TotalNodes)
	} else if cluster.OnlineNodes > 0 {
		// Some nodes offline
		nodeStatusText = fmt.Sprintf("[yellow]%d/%d ⚠️[-]", cluster.OnlineNodes, cluster.TotalNodes)
	} else {
		// All nodes offline (critical)
		nodeStatusText = fmt.Sprintf("[red]%d/%d 🔴[-]", cluster.OnlineNodes, cluster.TotalNodes)
	}

	cs.SummaryTable.SetCell(2, 1, tview.NewTableCell(nodeStatusText).SetTextColor(tcell.ColorWhite))

	// Quorate status
	cs.SummaryTable.SetCell(3, 0, tview.NewTableCell("Quorate").SetTextColor(tcell.ColorYellow))
	var quorateText string
	if cluster.Quorate {
		quorateText = "[green]Yes 🟢[-]"
	} else {
		quorateText = "[red]No  🔴[-]"
	}
	cs.SummaryTable.SetCell(3, 1, tview.NewTableCell(quorateText).SetTextColor(tcell.ColorWhite))

	// Update resource table (headers are already set in NewClusterStatus)
	// CPU row
	cs.ResourceTable.SetCell(1, 0, tview.NewTableCell("CPU Cores").SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%.1f%%", cluster.CPUUsage*100)).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(1, 2, tview.NewTableCell(fmt.Sprintf("%.1f", cluster.TotalCPU)).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft))

	// Memory row
	cs.ResourceTable.SetCell(2, 0, tview.NewTableCell("Memory").SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(2, 1, tview.NewTableCell(utils.FormatBytesFloat(cluster.MemoryUsed)).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(2, 2, tview.NewTableCell(utils.FormatBytesFloat(cluster.MemoryTotal)).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft))

	// Storage row
	storageUsed := utils.FormatBytes(cluster.StorageUsed)
	storageTotal := utils.FormatBytes(cluster.StorageTotal)
	storagePercent := 0.0
	if cluster.StorageTotal > 0 {
		storagePercent = (float64(cluster.StorageUsed) / float64(cluster.StorageTotal)) * 100
	}
	cs.ResourceTable.SetCell(3, 0, tview.NewTableCell("Storage").SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(3, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s)", storagePercent, storageUsed)).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft))
	cs.ResourceTable.SetCell(3, 2, tview.NewTableCell(storageTotal).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft))
}
