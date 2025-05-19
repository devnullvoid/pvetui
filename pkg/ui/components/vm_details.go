package components

import (
	"fmt"
	
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// VMDetails encapsulates the VM details panel
type VMDetails struct {
	*tview.Table
}

// NewVMDetails creates a new VM details panel
func NewVMDetails() *VMDetails {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetTitle(" Guest Details ")
	table.SetBorder(true)
	table.Clear()
	table.SetCell(0, 0, tview.NewTableCell("Select a guest").SetTextColor(tcell.ColorWhite))
	
	return &VMDetails{
		Table: table,
	}
}

// Update fills the VM details table for the given VM
func (vd *VMDetails) Update(vm *api.VM) {
	if vm == nil {
		vd.Clear()
		vd.SetCell(0, 0, tview.NewTableCell("Select a guest").SetTextColor(tcell.ColorWhite))
		return
	}
	
	vd.Clear()
	row := 0

	// Basic Info
	vd.SetCell(row, 0, tview.NewTableCell("🆔 ID").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", vm.ID)).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📛 Name").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Name).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📍 Node").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Node).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📦 Type").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Type).SetTextColor(tcell.ColorWhite))
	row++

	// Status Info
	statusEmoji := "🟢"
	if vm.Status == "stopped" {
		statusEmoji = "🔴"
	} else if vm.Status != "running" {
		statusEmoji = "🟡"
	}
	
	vd.SetCell(row, 0, tview.NewTableCell(statusEmoji + " Status").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Status).SetTextColor(utils.StatusColor(vm.Status)))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📡 IP").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(vm.IP).SetTextColor(tcell.ColorWhite))
	row++

	// Resource Usage
	vd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.1f%%", vm.CPU)).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d / %d MB", vm.Mem/1024, vm.MaxMem/1024)).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("💾 Disk").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d / %d GB", vm.Disk/1024, vm.MaxDisk/1024)).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(utils.FormatUptime(int(vm.Uptime))).SetTextColor(tcell.ColorWhite))
	row++
} 