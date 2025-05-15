package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CreateDetailsPanel creates the node details panel
func CreateDetailsPanel() (*tview.Flex, *tview.Table) {
	// Create container panel
	panel := tview.NewFlex().SetDirection(tview.FlexRow)
	panel.SetBorder(true).SetTitle("Node Details")

	// Create details table
	details := tview.NewTable().SetBorders(false)
	details.SetTitleAlign(tview.AlignLeft)

	// Column headers
	headers := []string{"Property", "Value"}
	for col, text := range headers {
		cell := tview.NewTableCell(text).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft)
		details.SetCell(0, col, cell)
	}

	// Initial rows
	rows := [][]string{
		{"Name", "Loading..."},
		{"Status", "Loading..."},
		{"CPU Usage", "Loading..."},
		{"Memory", "Loading..."},
		{"Storage", "Loading..."},
	}
	for row, fields := range rows {
		for col, text := range fields {
			cell := tview.NewTableCell(text).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignLeft)
			details.SetCell(row+1, col, cell)
		}
	}

	// Layout configuration
	panel.AddItem(details, 0, 1, false)
	return panel, details
}
