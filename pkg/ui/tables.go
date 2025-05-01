package ui

import (
    "fmt"

    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"
    "github.com/lonepie/proxmox-util/pkg/api"
)

// newVmsTable builds a VMs/LXCs table with header.
func newVmsTable() *tview.Table {
    tbl := tview.NewTable().SetBorders(true)
    tbl.SetBorder(true).SetTitle("Guests")
    tbl.SetCell(0, 0, tview.NewTableCell("VM ID").SetAttributes(tcell.AttrBold))
    tbl.SetCell(0, 1, tview.NewTableCell("Name").SetAttributes(tcell.AttrBold))
    tbl.SetCell(0, 2, tview.NewTableCell("Node").SetAttributes(tcell.AttrBold))
    tbl.SetCell(0, 3, tview.NewTableCell("Type").SetAttributes(tcell.AttrBold))
    return tbl
}

// newVmDetails returns an empty VM details table.
func newVmDetails() *tview.Table {
    tbl := tview.NewTable().SetBorders(false)
    tbl.Clear()
    tbl.SetCell(0, 0, tview.NewTableCell("Select a guest").SetTextColor(tcell.ColorWhite))
    return tbl
}

// populateVmDetails fills the VM details table for the given VM.
func populateVmDetails(tbl *tview.Table, vm api.VM) {
    tbl.Clear()
    tbl.SetCell(0, 0, tview.NewTableCell("🆔 ID").SetTextColor(tcell.ColorYellow))
    tbl.SetCell(0, 1, tview.NewTableCell(fmt.Sprintf("%d", vm.ID)).SetTextColor(tcell.ColorWhite))
    tbl.SetCell(1, 0, tview.NewTableCell("📛 Name").SetTextColor(tcell.ColorYellow))
    tbl.SetCell(1, 1, tview.NewTableCell(vm.Name).SetTextColor(tcell.ColorWhite))
    tbl.SetCell(2, 0, tview.NewTableCell("📍 Node").SetTextColor(tcell.ColorYellow))
    tbl.SetCell(2, 1, tview.NewTableCell(vm.Node).SetTextColor(tcell.ColorWhite))
    tbl.SetCell(3, 0, tview.NewTableCell("📦 Type").SetTextColor(tcell.ColorYellow))
    tbl.SetCell(3, 1, tview.NewTableCell(vm.Type).SetTextColor(tcell.ColorWhite))
}
