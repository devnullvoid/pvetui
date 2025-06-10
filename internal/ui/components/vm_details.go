package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// VMDetails encapsulates the VM details panel
type VMDetails struct {
	*tview.Table
	app *App
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

// SetApp sets the parent app reference for focus management
func (vd *VMDetails) SetApp(app *App) {
	vd.app = app

	// Set up input capture for arrow keys and VI-like navigation (hjkl)
	vd.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			if vd.app != nil {
				vd.app.SetFocus(vd.app.vmList)
				return nil
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'h': // VI-like left navigation
				if vd.app != nil {
					vd.app.SetFocus(vd.app.vmList)
					return nil
				}
			case 'j': // VI-like down navigation
				// Let the table handle down navigation naturally
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k': // VI-like up navigation
				// Let the table handle up navigation naturally
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'l': // VI-like right navigation - no action for VM details (already at rightmost)
				return nil
			}
		}
		return event
	})
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
	case api.VMStatusRunning:
		statusEmoji = "🟢"
		statusColor = tcell.ColorGreen
	case api.VMStatusStopped:
		statusEmoji = "🔴"
		statusColor = tcell.ColorRed
	default:
		statusEmoji = "🟡" // Default to yellow for other statuses
		statusColor = tcell.ColorYellow
	}

	vd.SetCell(row, 0, tview.NewTableCell(statusEmoji+" Status").SetTextColor(tcell.ColorYellow))
	vd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))
	row++

	// Tags (if set)
	if vm.Tags != "" {
		vd.SetCell(row, 0, tview.NewTableCell("🏷️ Tags").SetTextColor(tcell.ColorYellow))
		vd.SetCell(row, 1, tview.NewTableCell(vm.Tags).SetTextColor(tcell.ColorLightBlue))
		row++
	}

	vd.SetCell(row, 0, tview.NewTableCell("📡 IP").SetTextColor(tcell.ColorYellow))
	ipValue := api.StringNA
	if vm.IP != "" {
		ipValue = vm.IP
	}
	vd.SetCell(row, 1, tview.NewTableCell(ipValue).SetTextColor(tcell.ColorWhite))
	row++

	// Resource Usage
	vd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(tcell.ColorYellow))
	cpuValue := api.StringNA
	if vm.CPU >= 0 {
		cpuValue = fmt.Sprintf("%.1f%%", vm.CPU*100)
	}
	vd.SetCell(row, 1, tview.NewTableCell(cpuValue).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(tcell.ColorYellow))
	memValue := api.StringNA
	if vm.MaxMem > 0 {
		memUsedFormatted := utils.FormatBytes(vm.Mem)
		memTotalFormatted := utils.FormatBytes(vm.MaxMem)
		memoryPercent := utils.CalculatePercentageInt(vm.Mem, vm.MaxMem)
		memValue = fmt.Sprintf("%.2f%% (%s) / %s", memoryPercent, memUsedFormatted, memTotalFormatted)
	}
	vd.SetCell(row, 1, tview.NewTableCell(memValue).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("💾 Disk").SetTextColor(tcell.ColorYellow))
	diskValue := api.StringNA
	if vm.MaxDisk > 0 {
		diskUsedFormatted := utils.FormatBytes(vm.Disk)
		diskTotalFormatted := utils.FormatBytes(vm.MaxDisk)
		diskPercent := utils.CalculatePercentageInt(vm.Disk, vm.MaxDisk)
		diskValue = fmt.Sprintf("%.2f%% (%s) / %s", diskPercent, diskUsedFormatted, diskTotalFormatted)
	}
	vd.SetCell(row, 1, tview.NewTableCell(diskValue).SetTextColor(tcell.ColorWhite))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(tcell.ColorYellow))
	uptimeValue := api.StringNA
	if vm.Uptime > 0 {
		uptimeValue = utils.FormatUptime(int(vm.Uptime))
	}
	vd.SetCell(row, 1, tview.NewTableCell(uptimeValue).SetTextColor(tcell.ColorWhite))
	row++

	// Network IO if available
	vd.SetCell(row, 0, tview.NewTableCell("🔄 Network").SetTextColor(tcell.ColorYellow))
	if vm.NetIn > 0 || vm.NetOut > 0 {
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("In: %s, Out: %s",
			utils.FormatBytes(vm.NetIn), utils.FormatBytes(vm.NetOut))).SetTextColor(tcell.ColorWhite))
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(tcell.ColorWhite))
	}
	row++

	// Disk IO if available
	vd.SetCell(row, 0, tview.NewTableCell("💽 Disk IO").SetTextColor(tcell.ColorYellow))
	if vm.DiskRead > 0 || vm.DiskWrite > 0 {
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("Read: %s, Write: %s",
			utils.FormatBytes(vm.DiskRead), utils.FormatBytes(vm.DiskWrite))).SetTextColor(tcell.ColorWhite))
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(tcell.ColorWhite))
	}
	row++

	// Guest agent status (only for QEMU VMs)
	if vm.Type == api.VMTypeQemu {
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
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(tcell.ColorWhite))
	}

	// Show filesystem information if available
	vd.SetCell(row, 0, tview.NewTableCell("📂 Storage").SetTextColor(tcell.ColorYellow))
	if len(vm.Filesystems) > 0 {
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d volumes", len(vm.Filesystems))).SetTextColor(tcell.ColorWhite))
		row++

		// Sort filesystems to show root/system drive first, then by used percentage
		sortedFilesystems := make([]api.Filesystem, len(vm.Filesystems))
		copy(sortedFilesystems, vm.Filesystems)

		sort.Slice(sortedFilesystems, func(i, j int) bool {
			// Root filesystem comes first
			if sortedFilesystems[i].IsRoot {
				return true
			}
			if sortedFilesystems[j].IsRoot {
				return false
			}

			// System drive comes next
			if sortedFilesystems[i].IsSystemDrive {
				return true
			}
			if sortedFilesystems[j].IsSystemDrive {
				return false
			}

			// Then sort by used percentage (descending)
			// Handle potential division by zero
			if sortedFilesystems[i].TotalBytes == 0 {
				return false // Place filesystems with no total at the end
			}
			if sortedFilesystems[j].TotalBytes == 0 {
				return true // Place filesystems with no total at the end
			}

			usedPercentI := float64(sortedFilesystems[i].UsedBytes) / float64(sortedFilesystems[i].TotalBytes)
			usedPercentJ := float64(sortedFilesystems[j].UsedBytes) / float64(sortedFilesystems[j].TotalBytes)
			return usedPercentI > usedPercentJ
		})

		// Display up to 5 filesystems to avoid cluttering the UI
		maxFsToShow := 5
		if len(sortedFilesystems) < maxFsToShow {
			maxFsToShow = len(sortedFilesystems)
		}

		for i := 0; i < maxFsToShow; i++ {
			fs := sortedFilesystems[i]

			// Skip filesystems with no size info (should be filtered out already)
			if fs.TotalBytes == 0 {
				continue
			}

			// Get a suitable display name
			fsName := getFriendlyFilesystemName(fs)

			// Calculate usage percentage with safety check
			var usedPercent float64
			if fs.TotalBytes > 0 {
				usedPercent = float64(fs.UsedBytes) / float64(fs.TotalBytes) * 100
			} else {
				usedPercent = 0
			}

			// Choose color based on usage percentage
			usageColor := tcell.ColorGreen
			if usedPercent > 90 {
				usageColor = tcell.ColorRed
			} else if usedPercent > 75 {
				usageColor = tcell.ColorYellow
			}

			vd.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("  • %s", fsName)).SetTextColor(tcell.ColorLightSkyBlue))
			vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s/%s)",
				usedPercent,
				utils.FormatBytes(fs.UsedBytes),
				utils.FormatBytes(fs.TotalBytes))).SetTextColor(usageColor))
			row++
		}

		// Show a message if there are more filesystems
		if len(sortedFilesystems) > maxFsToShow {
			vd.SetCell(row, 0, tview.NewTableCell("  •").SetTextColor(tcell.ColorLightSkyBlue))
			vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("... and %d more", len(sortedFilesystems)-maxFsToShow)).SetTextColor(tcell.ColorGray))
		}
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(tcell.ColorWhite))
	}

	// Show network interfaces from guest agent if available
	vd.SetCell(row, 0, tview.NewTableCell("🌐 Network Interfaces").SetTextColor(tcell.ColorYellow))
	if len(vm.NetInterfaces) > 0 {
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
				if ip.Type == api.IPTypeIPv6 {
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
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(tcell.ColorWhite))
	}

	// Scroll to the top to ensure the most important information (basic details) is visible
	vd.ScrollToBeginning()
}

// getFriendlyFilesystemName returns a user-friendly name for a filesystem
func getFriendlyFilesystemName(fs api.Filesystem) string {
	// Start with mountpoint as the primary identifier
	fsName := fs.Mountpoint

	// Fall back to name if mountpoint is empty
	if fsName == "" {
		fsName = fs.Name
	}

	// Format Windows paths more nicely
	if strings.Contains(fsName, "\\") {
		// Remove trailing backslash
		fsName = strings.TrimSuffix(fsName, "\\")

		// For drive letters (like C:), just show the drive letter
		if len(fsName) >= 2 && fsName[1] == ':' {
			driveLetter := strings.ToUpper(fsName[:2])

			// Check if it's a standard Windows drive path
			if driveLetter == "C:" {
				return "System Drive (C:)"
			} else if driveLetter == "D:" {
				return "Data Drive (D:)"
			} else {
				return fmt.Sprintf("%s Drive", driveLetter)
			}
		}

		// For "System Reserved" or other special Windows partitions
		if fsName == "System Reserved" {
			return "System Reserved"
		}

		// For complex Windows paths, just show the drive letter if possible
		if strings.Contains(fsName, ":\\") {
			parts := strings.SplitN(fsName, ":\\", 2)
			if len(parts) == 2 && len(parts[0]) == 1 {
				driveLetter := strings.ToUpper(parts[0])
				return fmt.Sprintf("%s: Drive", driveLetter)
			}
		}

		// For Volume{GUID} paths, extract just the first part of the path
		if strings.Contains(fsName, "Volume{") {
			return "Windows Volume"
		}
	}

	// Handle Linux root and common paths
	if fsName == "/" {
		return "Root Filesystem (/)"
	} else if fsName == "/boot" {
		return "Boot Partition (/boot)"
	} else if fsName == "/boot/efi" {
		return "EFI Partition (/boot/efi)"
	} else if fsName == "/home" {
		return "Home Partition (/home)"
	} else if strings.HasPrefix(fsName, "/mnt/") || strings.HasPrefix(fsName, "/media/") {
		// For mounted external drives, just show the last part of the path
		parts := strings.Split(fsName, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if lastPart != "" {
				return fmt.Sprintf("Volume: %s", lastPart)
			}
		}
	}

	// If all else fails, return the original name but truncated if it's too long
	if len(fsName) > 30 {
		return fsName[:27] + "..."
	}

	return fsName
}
