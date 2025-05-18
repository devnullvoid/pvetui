package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// CreateClusterStatusPanel creates the cluster-wide status panel
func CreateClusterStatusPanel() (*tview.Flex, *tview.Table, *tview.Table) {
	// Create panel with two side-by-side tables
	// No changes needed - just confirming correct implementation
	// No changes needed - just confirming the correct signature
	// Create container panel
	panel := tview.NewFlex().SetDirection(tview.FlexColumn)
	panel.SetBorder(true).SetTitle("Cluster Status")

	// Create summary table
	summary := tview.NewTable().SetBorders(false)
	summary.SetTitleAlign(tview.AlignLeft)

	// Column headers
	headers := []string{"Metric", "Value", "Details"}
	for col, text := range headers {
		cell := tview.NewTableCell(text).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter)
		summary.SetCell(0, col, cell)
	}

	// Initial cluster status rows
	rows := [][]string{
		{"Cluster", "Loading...", "Loading...", ""},
		{"PVE Version", "Loading...", "Loading...", ""},
		{"Nodes Online", "Loading...", "Loading...", "🟢"},
		{"CPU Cores", "Loading...", "Loading...", ""},
		{"Memory", "Loading...", "Loading...", ""},
		{"Storage", "Loading...", "Loading...", ""},
	}
	for row, fields := range rows {
		for col, text := range fields {
			cell := tview.NewTableCell(text).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignLeft)
			summary.SetCell(row+1, col, cell)
		}
	}

	// Layout configuration
	// Create resource table
	resourceTable := tview.NewTable().SetBorders(false)
	resourceTable.SetTitleAlign(tview.AlignLeft)

	// Resource table headers
	resourceHeaders := []string{"Resource", "Total", "Used"}
	for col, text := range resourceHeaders {
		cell := tview.NewTableCell(text).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter)
		resourceTable.SetCell(0, col, cell)
	}

	// Add both tables to panel with equal space
	panel.AddItem(summary, 0, 1, false)
	panel.AddItem(resourceTable, 0, 1, false)
	return panel, summary, resourceTable
}

// UpdateClusterStatus populates both tables with current cluster data
func UpdateClusterStatus(summaryTable *tview.Table, resourceTable *tview.Table, cluster *api.Cluster) {
	// Clear existing content except headers in both tables
	for _, tbl := range []*tview.Table{summaryTable, resourceTable} {
		for row := 1; row <= 6; row++ {
			for col := 0; col < 4; col++ {
				tbl.SetCell(row, col, tview.NewTableCell(""))
			}
		}
	}

	// Update summary table (left panel)
	summaryTable.SetCell(1, 0, tview.NewTableCell("Cluster Name").SetTextColor(tcell.ColorYellow))
	summaryTable.SetCell(1, 1, tview.NewTableCell(cluster.Name).SetTextColor(tcell.ColorWhite))

	summaryTable.SetCell(2, 0, tview.NewTableCell("Proxmox VE").SetTextColor(tcell.ColorYellow))
	summaryTable.SetCell(2, 1, tview.NewTableCell(cluster.Version).SetTextColor(tcell.ColorWhite))

	summaryTable.SetCell(3, 0, tview.NewTableCell("Nodes Online").SetTextColor(tcell.ColorYellow))
	summaryTable.SetCell(3, 1, tview.NewTableCell(fmt.Sprintf("%d/%d 🟢", cluster.OnlineNodes, cluster.TotalNodes)).SetTextColor(tcell.ColorWhite))

	// Update resource table (right panel)
	resourceTable.SetCell(1, 0, tview.NewTableCell("CPU Cores").SetTextColor(tcell.ColorYellow))
	resourceTable.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%.1f", cluster.TotalCPU)).SetTextColor(tcell.ColorWhite))
	resourceTable.SetCell(1, 2, tview.NewTableCell(fmt.Sprintf("%.1f%%", cluster.CPUUsage*100)).SetTextColor(tcell.ColorWhite))

	resourceTable.SetCell(2, 0, tview.NewTableCell("Memory").SetTextColor(tcell.ColorYellow))
	resourceTable.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%.1f GB", cluster.MemoryTotal)).SetTextColor(tcell.ColorWhite))
	resourceTable.SetCell(2, 2, tview.NewTableCell(fmt.Sprintf("%.1f GB", cluster.MemoryUsed)).SetTextColor(tcell.ColorWhite))

	// Storage removed from cluster-level summary since it's node-specific
}
