package ui

import (
    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"
)

// newSummary returns a summary table with initial loading placeholder.
func newSummary() *tview.Table {
    summary := tview.NewTable().SetBorders(false)
    summary.Clear()
    summary.SetCell(0, 0, tview.NewTableCell("Loading summary...").SetTextColor(tcell.ColorWhite))
    return summary
}
