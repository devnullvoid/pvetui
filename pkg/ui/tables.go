package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// newVmDetails returns an empty VM details table.
func newVmDetails() *tview.Table {
	tbl := tview.NewTable().SetBorders(false)
	tbl.Clear()
	tbl.SetCell(0, 0, tview.NewTableCell("Select a guest").SetTextColor(tcell.ColorWhite))
	return tbl
}

// populateVmDetails fills the VM details table for the given VM.
func populateVmDetails(tbl *tview.Table, vm *api.VM) {
	tbl.Clear()
	row := 0

	// Basic Info
	tbl.SetCell(row, 0, tview.NewTableCell("🆔 ID").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", vm.ID)).SetTextColor(tcell.ColorWhite))
	row++

	tbl.SetCell(row, 0, tview.NewTableCell("📛 Name").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(vm.Name).SetTextColor(tcell.ColorWhite))
	row++

	tbl.SetCell(row, 0, tview.NewTableCell("📍 Node").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(vm.Node).SetTextColor(tcell.ColorWhite))
	row++

	tbl.SetCell(row, 0, tview.NewTableCell("📦 Type").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(vm.Type).SetTextColor(tcell.ColorWhite))
	row++

	// Status Info
	tbl.SetCell(row, 0, tview.NewTableCell("🟢 Status").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(vm.Status).SetTextColor(StatusColor(vm.Status)))
	row++

	tbl.SetCell(row, 0, tview.NewTableCell("📡 IP").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(vm.IP).SetTextColor(tcell.ColorWhite))
	row++

	// Resource Usage
	tbl.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.1f%%", vm.CPU)).SetTextColor(tcell.ColorWhite))
	row++

	tbl.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d / %d MB", vm.Mem/1024, vm.MaxMem/1024)).SetTextColor(tcell.ColorWhite))
	row++

	tbl.SetCell(row, 0, tview.NewTableCell("💾 Disk").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d / %d GB", vm.Disk/1024, vm.MaxDisk/1024)).SetTextColor(tcell.ColorWhite))
	row++

	tbl.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(tcell.ColorYellow))
	tbl.SetCell(row, 1, tview.NewTableCell(FormatUptime(vm.Uptime)).SetTextColor(tcell.ColorWhite))
	row++
}
