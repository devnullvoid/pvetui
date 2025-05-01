package ui

import (
    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"
)

// newDetails returns a details table with initial loading placeholder.
func newDetails() *tview.Table {
    details := tview.NewTable().SetBorders(false)
    details.Clear()
    details.SetCell(0, 0, tview.NewTableCell("Loading...").SetTextColor(tcell.ColorWhite))
    return details
}
