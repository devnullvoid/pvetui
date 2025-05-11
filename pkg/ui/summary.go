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

	// Column headers for cluster status
	headers := []string{"Metric", "Value", "Nodes", "Health"}
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
func UpdateSummary(table *tview.Table, clusterStatus map[string]map[string]interface{}, nodes []api.Node) {
	// Calculate cluster-wide metrics
	var onlineNodes, totalNodes int
	var totalCPU, usedMem, totalMem float64

	totalNodes = len(nodes)
	for _, node := range nodes {
		if node.Online {
			onlineNodes++
		}
		totalCPU += node.CPUUsage
		usedMem += float64(node.MemoryUsed)
		totalMem += float64(node.MemoryTotal)
	}

	// Clear existing content except headers
	for row := 1; row <= 4; row++ {
		for col := 0; col < 4; col++ {
			table.SetCell(row, col, tview.NewTableCell(""))
		}
	}

	// Nodes Row
	table.SetCell(1, 0, tview.NewTableCell("Nodes Online").SetTextColor(tcell.ColorYellow))
	table.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%d/%d", onlineNodes, totalNodes)).SetTextColor(tcell.ColorWhite))
	table.SetCell(1, 3, tview.NewTableCell("🟢").SetTextColor(tcell.ColorGreen))

	// CPU Row
	table.SetCell(2, 0, tview.NewTableCell("Total CPU").SetTextColor(tcell.ColorYellow))
	table.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%.1f%%", totalCPU/float64(totalNodes))).SetTextColor(tcell.ColorWhite))

	// Memory Row
	table.SetCell(3, 0, tview.NewTableCell("Total Memory").SetTextColor(tcell.ColorYellow))
	table.SetCell(3, 1, tview.NewTableCell(fmt.Sprintf("%.1f/%.1f GB",
		usedMem/1024/1024/1024, totalMem/1024/1024/1024)).SetTextColor(tcell.ColorWhite))

	// Storage Row
	table.SetCell(4, 0, tview.NewTableCell("Total Storage").SetTextColor(tcell.ColorYellow))
	table.SetCell(4, 1, tview.NewTableCell("N/A").SetTextColor(tcell.ColorWhite))
}
