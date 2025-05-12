package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// CreateClusterStatusPanel creates the cluster-wide status panel
func CreateClusterStatusPanel() (*tview.Flex, *tview.Table) {
	// Create container panel
	panel := tview.NewFlex().SetDirection(tview.FlexColumn)
	panel.SetBorder(true).SetTitle("Cluster Status")

	// Create summary table
	summary := tview.NewTable().SetBorders(false)
	summary.SetTitleAlign(tview.AlignLeft)

	// Column headers
	headers := []string{"Cluster Metric", "Value", "", "Health"}
	for col, text := range headers {
		cell := tview.NewTableCell(text).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter)
		summary.SetCell(0, col, cell)
	}

	// Initial cluster status rows
	rows := [][]string{
		{"Nodes Online", "Loading...", "", "🟢"},
		{"Total CPU", "Loading...", "", ""},
		{"Total Memory", "Loading...", "", ""},
		{"Total Storage", "Loading...", "", ""},
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
	panel.AddItem(summary, 0, 1, false)
	return panel, summary
}

// UpdateSummary populates the summary table with cluster status data
func UpdateSummary(table *tview.Table, cluster *api.Cluster) {
	// Clear existing content except headers
	for row := 1; row <= 6; row++ {
		for col := 0; col < 4; col++ {
			table.SetCell(row, col, tview.NewTableCell(""))
		}
	}

	// Cluster Info
	table.SetCell(1, 0, tview.NewTableCell("Cluster").SetTextColor(tcell.ColorYellow))
	table.SetCell(1, 1, tview.NewTableCell(cluster.ClusterName).SetTextColor(tcell.ColorWhite))

	// Version Info
	table.SetCell(2, 0, tview.NewTableCell("PVE Version").SetTextColor(tcell.ColorYellow))
	table.SetCell(2, 1, tview.NewTableCell(cluster.PVEVersion).SetTextColor(tcell.ColorWhite))

	// Nodes Status
	table.SetCell(3, 0, tview.NewTableCell("Nodes Online").SetTextColor(tcell.ColorYellow))
	table.SetCell(3, 1, tview.NewTableCell(fmt.Sprintf("%d/%d", cluster.OnlineNodes, cluster.TotalNodes)).SetTextColor(tcell.ColorWhite))
	table.SetCell(3, 3, tview.NewTableCell("🟢").SetTextColor(tcell.ColorGreen))

	// CPU Usage
	table.SetCell(4, 0, tview.NewTableCell("CPU Cores").SetTextColor(tcell.ColorYellow))
	table.SetCell(4, 1, tview.NewTableCell(fmt.Sprintf("%.1f", cluster.TotalCPU)).SetTextColor(tcell.ColorWhite))

	// Memory Usage
	table.SetCell(5, 0, tview.NewTableCell("Memory").SetTextColor(tcell.ColorYellow))
	table.SetCell(5, 1, tview.NewTableCell(fmt.Sprintf("%.1f/%.1f GB",
		float64(cluster.UsedMemory)/1024/1024/1024,
		float64(cluster.TotalMemory)/1024/1024/1024)).SetTextColor(tcell.ColorWhite))

	// Storage Usage
	table.SetCell(6, 0, tview.NewTableCell("Storage").SetTextColor(tcell.ColorYellow))
	table.SetCell(6, 1, tview.NewTableCell(fmt.Sprintf("%.1f/%.1f TB",
		float64(cluster.UsedStorage)/1024/1024/1024/1024,
		float64(cluster.TotalStorage)/1024/1024/1024/1024)).SetTextColor(tcell.ColorWhite))
}
