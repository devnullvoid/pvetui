package components

import (
	"fmt"
	"strings"

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
	vd.SetCell(row, 1, tview.NewTableCell(strings.ToUpper(vm.Type)).SetTextColor(tcell.ColorWhite))
	row++

	// Status Info
	// Capitalize the first letter of status
	statusText := vm.Status
	if len(statusText) > 0 {
		statusText = strings.ToUpper(statusText[:1]) + statusText[1:]
	}

	// Determine color for status text
	var statusColor tcell.Color
	var statusEmoji string // For original emojis

	switch strings.ToLower(vm.Status) {
	case "running":
		statusEmoji = "🟢"
		statusColor = tcell.ColorGreen
	case "stopped":
		statusEmoji = "🔴"
		statusColor = tcell.ColorRed
	default:
		statusEmoji = "🟡" // Default to yellow for other statuses
		statusColor = tcell.ColorYellow
	}

	vd.SetCell(row, 0, tview.NewTableCell(statusEmoji+" Status").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📡 IP").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(vm.IP).SetTextColor(tcell.ColorWhite))
	row++

	// Resource Usage
	vd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(tcell.ColorYellow))
	cpuValue := "N/A"
	if vm.Enriched {
		cpuValue = fmt.Sprintf("%.1f%%", vm.CPU*100)
	}
	vd.SetCell(row, 1, tview.NewTableCell(cpuValue).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(tcell.ColorYellow))
	memValue := "N/A"
	if vm.Enriched {
		memValue = fmt.Sprintf("%.1f / %.1f GB", float64(vm.Mem)/(1024*1024*1024), float64(vm.MaxMem)/(1024*1024*1024))
	}
	vd.SetCell(row, 1, tview.NewTableCell(memValue).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("💾 Disk").SetTextColor(tcell.ColorYellow))
	diskValue := "N/A"
	if vm.Enriched {
		diskValue = fmt.Sprintf("%.1f / %.1f GB", float64(vm.Disk)/(1024*1024*1024), float64(vm.MaxDisk)/(1024*1024*1024))
	}
	vd.SetCell(row, 1, tview.NewTableCell(diskValue).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(tcell.ColorYellow))
	uptimeValue := "N/A"
	if vm.Enriched && vm.Uptime > 0 {
		uptimeValue = utils.FormatUptime(int(vm.Uptime))
	}
	vd.SetCell(row, 1, tview.NewTableCell(uptimeValue).SetTextColor(tcell.ColorWhite))
	row++

	// Network IO if available
	if vm.Enriched && (vm.NetIn > 0 || vm.NetOut > 0) {
		vd.SetCell(row, 0, tview.NewTableCell("🔄 Network").SetTextColor(tcell.ColorYellow))
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("In: %s, Out: %s",
			utils.FormatBytes(vm.NetIn), utils.FormatBytes(vm.NetOut))).SetTextColor(tcell.ColorWhite))
		row++
	}

	// Disk IO if available
	if vm.Enriched && (vm.DiskRead > 0 || vm.DiskWrite > 0) {
		vd.SetCell(row, 0, tview.NewTableCell("💽 Disk IO").SetTextColor(tcell.ColorYellow))
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("Read: %s, Write: %s",
			utils.FormatBytes(vm.DiskRead), utils.FormatBytes(vm.DiskWrite))).SetTextColor(tcell.ColorWhite))
		row++
	}

	// Guest agent status (only for QEMU VMs)
	if vm.Type == "qemu" {
		agentStatus := "Not enabled"
		agentColor := tcell.ColorGray

		if vm.AgentEnabled {
			if vm.AgentRunning {
				agentStatus = "Running"
				agentColor = tcell.ColorGreen
			} else {
				agentStatus = "Enabled but not running"
				agentColor = tcell.ColorYellow
			}
		}

		vd.SetCell(row, 0, tview.NewTableCell("👾 Guest Agent").SetTextColor(tcell.ColorYellow))
		vd.SetCell(row, 1, tview.NewTableCell(agentStatus).SetTextColor(agentColor))
		row++
	}

	// Show network interfaces from guest agent if available
	if len(vm.NetInterfaces) > 0 {
		vd.SetCell(row, 0, tview.NewTableCell("🌐 Network Interfaces").SetTextColor(tcell.ColorYellow))
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d available", len(vm.NetInterfaces))).SetTextColor(tcell.ColorWhite))
		row++

		// Show interface details
		for _, iface := range vm.NetInterfaces {
			// Skip loopback interfaces
			if iface.IsLoopback {
				continue
			}

			// Show interface name and MAC
			vd.SetCell(row, 0, tview.NewTableCell("  • "+iface.Name).SetTextColor(tcell.ColorLightSkyBlue))
			vd.SetCell(row, 1, tview.NewTableCell(iface.MACAddress).SetTextColor(tcell.ColorWhite))
			row++

			// Show IP addresses
			for _, ip := range iface.IPAddresses {
				ipColor := tcell.ColorWhite
				if ip.Type == "ipv6" {
					ipColor = tcell.ColorLightSkyBlue
				}
				vd.SetCell(row, 0, tview.NewTableCell("    "+ip.Type).SetTextColor(tcell.ColorGray))
				vd.SetCell(row, 1, tview.NewTableCell(ip.Address).SetTextColor(ipColor))
				row++
			}

			// Show interface traffic
			if iface.Statistics.RxBytes > 0 || iface.Statistics.TxBytes > 0 {
				vd.SetCell(row, 0, tview.NewTableCell("    Traffic").SetTextColor(tcell.ColorGray))
				vd.SetCell(row, 1, tview.NewTableCell(
					fmt.Sprintf("↓ %s ↑ %s",
						utils.FormatBytes(iface.Statistics.RxBytes),
						utils.FormatBytes(iface.Statistics.TxBytes))).SetTextColor(tcell.ColorWhite))
				row++
			}
		}
	}
}
