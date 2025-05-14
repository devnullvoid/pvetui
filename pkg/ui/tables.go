package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
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
	tbl.SetCell(row, 1, tview.NewTableCell(vm.Status).SetTextColor(statusColor(vm.Status)))
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
	tbl.SetCell(row, 1, tview.NewTableCell(formatUptime(vm.Uptime)).SetTextColor(tcell.ColorWhite))
	row++
}

func statusColor(status string) tcell.Color {
	switch status {
	case "running":
		return tcell.ColorGreen
	case "stopped":
		return tcell.ColorRed
	default:
		return tcell.ColorYellow
	}
}

func formatUptime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	minutes %= 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := hours / 24
	hours %= 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
