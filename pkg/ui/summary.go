package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// CreateNodeSummaryPanel creates the node resource summary panel
func CreateNodeSummaryPanel() (*tview.Flex, *tview.Table) {
	// Create container panel
	panel := tview.NewFlex().SetDirection(tview.FlexColumn)
	panel.SetBorder(true).SetTitle("Node Summary")

	// Create summary table
	summary := tview.NewTable().SetBorders(false)
	summary.SetTitleAlign(tview.AlignLeft)

	// Column headers
	headers := []string{"Resource", "Usage", "Allocated", "Max"}
	for col, text := range headers {
		cell := tview.NewTableCell(text).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter)
		summary.SetCell(0, col, cell)
	}

	// Initial rows
	rows := [][]string{
		{"CPU", "Loading...", "", ""},
		{"Memory", "Loading...", "", ""},
		{"Storage", "Loading...", "", ""},
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

// UpdateSummary populates the summary table with node status data
func UpdateSummary(table *tview.Table, n api.Node, status map[string]interface{}) {
	// Compute metrics from status
	cpuPercent := ToFloat(status["cpu"]) * 100
	memUsed := ToFloat(status["mem"])
	memTotal := ToFloat(status["maxmem"])
	storageUsed := ToFloat(status["rootfs"].(map[string]interface{})["used"])
	storageTotal := ToFloat(status["rootfs"].(map[string]interface{})["total"])

	// Clear existing content except headers
	for row := 1; row <= 3; row++ {
		for col := 0; col < 4; col++ {
			table.SetCell(row, col, tview.NewTableCell(""))
		}
	}

	// CPU Row
	table.SetCell(1, 0, tview.NewTableCell("CPU").SetTextColor(tcell.ColorYellow))
	table.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%.1f%%", cpuPercent)).SetTextColor(tcell.ColorWhite))

	// Memory Row
	table.SetCell(2, 0, tview.NewTableCell("Memory").SetTextColor(tcell.ColorYellow))
	table.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%.1f/%.1f GB",
		memUsed/1024/1024/1024, memTotal/1024/1024/1024)).SetTextColor(tcell.ColorWhite))

	// Storage Row
	table.SetCell(3, 0, tview.NewTableCell("Storage").SetTextColor(tcell.ColorYellow))
	table.SetCell(3, 1, tview.NewTableCell(fmt.Sprintf("%.1f/%.1f GB",
		storageUsed/1024/1024/1024, storageTotal/1024/1024/1024)).SetTextColor(tcell.ColorWhite))
}
