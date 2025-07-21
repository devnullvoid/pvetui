package components

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// VMDetails encapsulates the VM details panel
type VMDetails struct {
	*tview.Table
	app *App
}

var _ VMDetailsComponent = (*VMDetails)(nil)

// NewVMDetails creates a new VM details panel
func NewVMDetails() *VMDetails {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetTitle(" Guest Details ")
	table.SetBorder(true)
	table.Clear()
	table.SetCell(0, 0, tview.NewTableCell("Select a guest").SetTextColor(theme.Colors.Primary))

	return &VMDetails{
		Table: table,
	}
}

// Clear wraps the table Clear method to satisfy the interface
func (vd *VMDetails) Clear() *tview.Table {
	return vd.Table.Clear()
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
		vd.SetCell(0, 0, tview.NewTableCell("Select a guest").SetTextColor(theme.Colors.Primary))
		return
	}

	vd.Clear()
	row := 0

	// Basic Info
	vd.SetCell(row, 0, tview.NewTableCell("🆔 ID").SetTextColor(theme.Colors.Primary))
	vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", vm.ID)).SetTextColor(theme.Colors.Secondary))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📛 Name").SetTextColor(theme.Colors.Primary))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Name).SetTextColor(theme.Colors.Secondary))
	row++

	// Show description if available
	if vm.Description != "" {
		cleanDesc := sanitizeDescription(vm.Description)
		if cleanDesc != "" {
			vd.SetCell(row, 0, tview.NewTableCell("📝 Description").SetTextColor(theme.Colors.Primary))
			vd.SetCell(row, 1, tview.NewTableCell(cleanDesc).SetTextColor(theme.Colors.Info))
			row++
		}
	}

	vd.SetCell(row, 0, tview.NewTableCell("📍 Node").SetTextColor(theme.Colors.Primary))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Node).SetTextColor(theme.Colors.Secondary))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📦 Type").SetTextColor(theme.Colors.Primary))
	vd.SetCell(row, 1, tview.NewTableCell(strings.ToUpper(vm.Type)).SetTextColor(theme.Colors.Secondary))
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
		statusColor = theme.Colors.StatusRunning
	case api.VMStatusStopped:
		statusEmoji = "🔴"
		statusColor = theme.Colors.StatusStopped
	default:
		statusEmoji = "🟡" // Default to yellow for other statuses
		statusColor = theme.Colors.StatusPending
	}

	vd.SetCell(row, 0, tview.NewTableCell(statusEmoji+" Status").SetTextColor(theme.Colors.Primary))
	vd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))
	row++

	// Tags (if set)
	if vm.Tags != "" {
		vd.SetCell(row, 0, tview.NewTableCell("🏷️ Tags").SetTextColor(theme.Colors.Primary))
		vd.SetCell(row, 1, tview.NewTableCell(vm.Tags).SetTextColor(theme.Colors.Info))
		row++
	}

	vd.SetCell(row, 0, tview.NewTableCell("📡 IP").SetTextColor(theme.Colors.Primary))
	ipValue := api.StringNA
	if vm.IP != "" {
		ipValue = vm.IP
	}
	vd.SetCell(row, 1, tview.NewTableCell(ipValue).SetTextColor(theme.Colors.Secondary))
	row++

	// Resource Usage
	vd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(theme.Colors.Primary))
	cpuValue := api.StringNA
	if vm.CPU >= 0 {
		cpuValue = fmt.Sprintf("%.1f%%", vm.CPU*100)
	}
	vd.SetCell(row, 1, tview.NewTableCell(cpuValue).SetTextColor(theme.Colors.Secondary))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(theme.Colors.Primary))
	memValue := api.StringNA
	if vm.MaxMem > 0 {
		memUsedFormatted := utils.FormatBytes(vm.Mem)
		memTotalFormatted := utils.FormatBytes(vm.MaxMem)
		memoryPercent := utils.CalculatePercentageInt(vm.Mem, vm.MaxMem)
		memValue = fmt.Sprintf("%.2f%% (%s) / %s", memoryPercent, memUsedFormatted, memTotalFormatted)
	}
	vd.SetCell(row, 1, tview.NewTableCell(memValue).SetTextColor(theme.Colors.Secondary))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("💾 Disk").SetTextColor(theme.Colors.Primary))
	diskValue := api.StringNA
	if vm.MaxDisk > 0 {
		diskUsedFormatted := utils.FormatBytes(vm.Disk)
		diskTotalFormatted := utils.FormatBytes(vm.MaxDisk)
		diskPercent := utils.CalculatePercentageInt(vm.Disk, vm.MaxDisk)
		diskValue = fmt.Sprintf("%.2f%% (%s) / %s", diskPercent, diskUsedFormatted, diskTotalFormatted)
	}
	vd.SetCell(row, 1, tview.NewTableCell(diskValue).SetTextColor(theme.Colors.Secondary))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(theme.Colors.Primary))
	uptimeValue := api.StringNA
	if vm.Uptime > 0 {
		uptimeValue = utils.FormatUptime(int(vm.Uptime))
	}
	vd.SetCell(row, 1, tview.NewTableCell(uptimeValue).SetTextColor(theme.Colors.Secondary))
	row++

	// Network Interfaces
	vd.SetCell(row, 0, tview.NewTableCell("🔄 Network").SetTextColor(theme.Colors.Primary))

	// Show network interfaces from guest agent
	if len(vm.NetInterfaces) > 0 {
		// Sort networks by name for consistent display
		sortedInterfaces := make([]api.NetworkInterface, len(vm.NetInterfaces))
		copy(sortedInterfaces, vm.NetInterfaces)
		sort.Slice(sortedInterfaces, func(i, j int) bool {
			return sortedInterfaces[i].Name < sortedInterfaces[j].Name
		})

		// Show first few networks (up to 3 to avoid cluttering)
		maxNetworksToShow := 3
		for i := 0; i < len(sortedInterfaces) && i < maxNetworksToShow; i++ {
			net := sortedInterfaces[i]

			// Build network display string
			var netInfo []string
			if len(net.IPAddresses) > 0 {
				netInfo = append(netInfo, net.IPAddresses[0].Address)
			}
			if net.MACAddress != "" {
				netInfo = append(netInfo, net.MACAddress)
			}

			netText := net.Name
			if len(netInfo) > 0 {
				netText += fmt.Sprintf(" (%s)", strings.Join(netInfo, ", "))
			}

			vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("  • %s", netText)).SetTextColor(theme.Colors.Info))
			row++
		}

		// Show a message if there are more networks
		if len(sortedInterfaces) > maxNetworksToShow {
			vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("  • ... and %d more", len(sortedInterfaces)-maxNetworksToShow)).SetTextColor(theme.Colors.Secondary))
			row++
		}
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
		row++
	}

	// Filesystems (if available)
	if len(vm.Filesystems) > 0 {
		vd.SetCell(row, 0, tview.NewTableCell("💾 Filesystems").SetTextColor(theme.Colors.Primary))
		row++

		// Sort filesystems by usage percentage (highest first)
		sortedFilesystems := make([]api.Filesystem, len(vm.Filesystems))
		copy(sortedFilesystems, vm.Filesystems)
		sort.Slice(sortedFilesystems, func(i, j int) bool {
			// Calculate usage percentage for sorting
			var usageI, usageJ float64
			if sortedFilesystems[i].TotalBytes > 0 {
				usageI = float64(sortedFilesystems[i].UsedBytes) / float64(sortedFilesystems[i].TotalBytes) * 100
			}
			if sortedFilesystems[j].TotalBytes > 0 {
				usageJ = float64(sortedFilesystems[j].UsedBytes) / float64(sortedFilesystems[j].TotalBytes) * 100
			}
			return usageI > usageJ // Sort by usage (highest first)
		})

		// Show first few filesystems (up to 3 to avoid cluttering)
		maxFsToShow := 3

		for i := 0; i < maxFsToShow && i < len(sortedFilesystems); i++ {
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
			usageColor := theme.GetUsageColor(usedPercent)

			vd.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("  • %s", fsName)).SetTextColor(theme.Colors.Info))
			vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s/%s)",
				usedPercent,
				utils.FormatBytes(fs.UsedBytes),
				utils.FormatBytes(fs.TotalBytes))).SetTextColor(usageColor))
			row++
		}

		// Show a message if there are more filesystems
		if len(sortedFilesystems) > maxFsToShow {
			vd.SetCell(row, 0, tview.NewTableCell("  •").SetTextColor(theme.Colors.Info))
			vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("... and %d more", len(sortedFilesystems)-maxFsToShow)).SetTextColor(theme.Colors.Secondary))
			row++
		}
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
		row++
	}

	// Configuration Section
	vd.SetCell(row, 0, tview.NewTableCell("⚙️ Configuration").SetTextColor(theme.Colors.Primary))
	vd.SetCell(row, 1, tview.NewTableCell("").SetTextColor(theme.Colors.Secondary))
	row++

	// CPU Configuration
	if vm.CPUCores > 0 || vm.CPUSockets > 0 {
		cpuText := ""
		if vm.CPUCores > 0 {
			cpuText = fmt.Sprintf("%d cores", vm.CPUCores)
		}
		if vm.CPUSockets > 0 {
			if cpuText != "" {
				cpuText += fmt.Sprintf(", %d sockets", vm.CPUSockets)
			} else {
				cpuText = fmt.Sprintf("%d sockets", vm.CPUSockets)
			}
		}
		if cpuText != "" {
			vd.SetCell(row, 0, tview.NewTableCell("  • CPU").SetTextColor(theme.Colors.Info))
			vd.SetCell(row, 1, tview.NewTableCell(cpuText).SetTextColor(theme.Colors.Secondary))
			row++
		}
	}

	// Architecture and OS Type
	if vm.Architecture != "" || vm.OSType != "" {
		archText := ""
		if vm.Architecture != "" {
			archText = vm.Architecture
		}
		if vm.OSType != "" {
			if archText != "" {
				archText += fmt.Sprintf(" (%s)", vm.OSType)
			} else {
				archText = vm.OSType
			}
		}
		if archText != "" {
			vd.SetCell(row, 0, tview.NewTableCell("  • Architecture").SetTextColor(theme.Colors.Info))
			vd.SetCell(row, 1, tview.NewTableCell(archText).SetTextColor(theme.Colors.Secondary))
			row++
		}
	}

	// Boot Order
	if vm.BootOrder != "" {
		vd.SetCell(row, 0, tview.NewTableCell("  • Boot Order").SetTextColor(theme.Colors.Info))
		vd.SetCell(row, 1, tview.NewTableCell(vm.BootOrder).SetTextColor(theme.Colors.Secondary))
		row++
	}

	// Auto-start
	vd.SetCell(row, 0, tview.NewTableCell("  • Auto-start").SetTextColor(theme.Colors.Info))
	autoStartText := "Disabled"
	autoStartColor := theme.Colors.Secondary
	if vm.OnBoot {
		autoStartText = "Enabled"
		autoStartColor = theme.Colors.Success
	}
	vd.SetCell(row, 1, tview.NewTableCell(autoStartText).SetTextColor(autoStartColor))

	// Scroll to the top to ensure the most important information (basic details) is visible
	vd.ScrollToBeginning()
}

// getFriendlyFilesystemName returns a user-friendly name for a filesystem
func getFriendlyFilesystemName(fs api.Filesystem) string {
	// Try to extract a meaningful name from the mount point
	if fs.Mountpoint != "" {
		// Remove leading slash and common prefixes
		name := strings.TrimPrefix(fs.Mountpoint, "/")
		name = strings.TrimPrefix(name, "mnt/")
		name = strings.TrimPrefix(name, "media/")

		// If we have a reasonable name, use it
		if name != "" && name != "mnt" && name != "media" {
			return name
		}
	}

	// Fall back to the filesystem type
	if fs.Type != "" {
		return fs.Type
	}

	// Last resort
	return "Unknown"
}

// sanitizeDescription cleans up VM description text for display
func sanitizeDescription(desc string) string {
	// Remove common HTML-like tags and excessive whitespace
	desc = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(desc, "")
	desc = regexp.MustCompile(`\s+`).ReplaceAllString(desc, " ")
	desc = strings.TrimSpace(desc)

	// Limit length to avoid cluttering the display
	if len(desc) > 100 {
		desc = desc[:97] + "..."
	}

	return desc
}
