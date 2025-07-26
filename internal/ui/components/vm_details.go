package components

import (
	"fmt"
	"regexp"
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
	vd.SetInputCapture(createNavigationInputCapture(vd.app, vd.app.vmList, nil))
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
	vd.SetCell(row, 0, tview.NewTableCell("🆔 ID").SetTextColor(theme.Colors.HeaderText))
	vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", vm.ID)).SetTextColor(theme.Colors.Primary))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📛 Name").SetTextColor(theme.Colors.HeaderText))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Name).SetTextColor(theme.Colors.Primary))
	row++

	// Show description if available
	if vm.Description != "" {
		cleanDesc := sanitizeDescription(vm.Description)
		if cleanDesc != "" {
			vd.SetCell(row, 0, tview.NewTableCell("📝 Description").SetTextColor(theme.Colors.HeaderText))
			vd.SetCell(row, 1, tview.NewTableCell(cleanDesc).SetTextColor(theme.Colors.Info))
			row++
		}
	}

	vd.SetCell(row, 0, tview.NewTableCell("📍 Node").SetTextColor(theme.Colors.HeaderText))
	vd.SetCell(row, 1, tview.NewTableCell(vm.Node).SetTextColor(theme.Colors.Primary))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("📦 Type").SetTextColor(theme.Colors.HeaderText))
	vd.SetCell(row, 1, tview.NewTableCell(strings.ToUpper(vm.Type)).SetTextColor(theme.Colors.Primary))
	row++

	// Status Info
	statusText := vm.Status
	if len(statusText) > 0 {
		statusText = strings.ToUpper(statusText[:1]) + statusText[1:]
	}
	var statusColor tcell.Color
	var statusEmoji string
	switch strings.ToLower(vm.Status) {
	case api.VMStatusRunning:
		statusEmoji = "🟢"
		statusColor = theme.Colors.StatusRunning
	case api.VMStatusStopped:
		statusEmoji = "🔴"
		statusColor = theme.Colors.StatusStopped
	default:
		statusEmoji = "🟡"
		statusColor = theme.Colors.StatusPending
	}
	vd.SetCell(row, 0, tview.NewTableCell(statusEmoji+" Status").SetTextColor(theme.Colors.HeaderText))
	vd.SetCell(row, 1, tview.NewTableCell(statusText).SetTextColor(statusColor))
	row++

	// Tags (if set)
	vd.SetCell(row, 0, tview.NewTableCell("🏷️ Tags").SetTextColor(theme.Colors.HeaderText))
	if vm.Tags != "" {
		vd.SetCell(row, 1, tview.NewTableCell(vm.Tags).SetTextColor(theme.Colors.Info))
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
	}
	row++

	// IP Address
	vd.SetCell(row, 0, tview.NewTableCell("📡 IP").SetTextColor(theme.Colors.HeaderText))
	ipValue := api.StringNA
	if vm.IP != "" {
		ipValue = vm.IP
	}
	vd.SetCell(row, 1, tview.NewTableCell(ipValue).SetTextColor(theme.Colors.Primary))
	row++

	// CPU Usage
	vd.SetCell(row, 0, tview.NewTableCell("💻 CPU").SetTextColor(theme.Colors.HeaderText))
	cpuValue := api.StringNA
	cpuUsageColor := theme.Colors.Primary
	if vm.CPU >= 0 && vm.CPUCores > 0 {
		cpuPercent := vm.CPU * 100
		cpuValue = fmt.Sprintf("%.1f%% of %d cores", cpuPercent, vm.CPUCores)
		cpuUsageColor = theme.GetUsageColor(cpuPercent)
	} else if vm.CPU >= 0 {
		cpuPercent := vm.CPU * 100
		cpuValue = fmt.Sprintf("%.1f%%", cpuPercent)
		cpuUsageColor = theme.GetUsageColor(cpuPercent)
	}
	vd.SetCell(row, 1, tview.NewTableCell(cpuValue).SetTextColor(cpuUsageColor))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("🧠 Memory").SetTextColor(theme.Colors.HeaderText))
	memValue := api.StringNA
	memUsageColor := theme.Colors.Primary
	if vm.MaxMem > 0 {
		memUsedFormatted := utils.FormatBytes(vm.Mem)
		memTotalFormatted := utils.FormatBytes(vm.MaxMem)
		memoryPercent := utils.CalculatePercentageInt(vm.Mem, vm.MaxMem)
		memValue = fmt.Sprintf("%.2f%% (%s) / %s", memoryPercent, memUsedFormatted, memTotalFormatted)
		memUsageColor = theme.GetUsageColor(memoryPercent)
	}
	vd.SetCell(row, 1, tview.NewTableCell(memValue).SetTextColor(memUsageColor))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("💾 Disk").SetTextColor(theme.Colors.HeaderText))
	diskValue := api.StringNA
	diskUsageColor := theme.Colors.Primary
	if vm.MaxDisk > 0 {
		diskUsedFormatted := utils.FormatBytes(vm.Disk)
		diskTotalFormatted := utils.FormatBytes(vm.MaxDisk)
		diskPercent := utils.CalculatePercentageInt(vm.Disk, vm.MaxDisk)
		diskValue = fmt.Sprintf("%.2f%% (%s) / %s", diskPercent, diskUsedFormatted, diskTotalFormatted)
		diskUsageColor = theme.GetUsageColor(diskPercent)
	}
	vd.SetCell(row, 1, tview.NewTableCell(diskValue).SetTextColor(diskUsageColor))
	row++

	vd.SetCell(row, 0, tview.NewTableCell("⏱️ Uptime").SetTextColor(theme.Colors.HeaderText))
	uptimeValue := api.StringNA
	if vm.Uptime > 0 {
		uptimeValue = utils.FormatUptime(int(vm.Uptime))
	}
	vd.SetCell(row, 1, tview.NewTableCell(uptimeValue).SetTextColor(theme.Colors.Primary))
	row++

	// Network IO summary
	vd.SetCell(row, 0, tview.NewTableCell("🔄 Network IO").SetTextColor(theme.Colors.HeaderText))
	if vm.NetIn > 0 || vm.NetOut > 0 {
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("In: %s, Out: %s", utils.FormatBytes(vm.NetIn), utils.FormatBytes(vm.NetOut))).SetTextColor(theme.Colors.Primary))
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
	}
	row++

	// Disk IO summary
	vd.SetCell(row, 0, tview.NewTableCell("🗄️ Disk IO").SetTextColor(theme.Colors.HeaderText))
	if vm.DiskRead > 0 || vm.DiskWrite > 0 {
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("Read: %s, Write: %s", utils.FormatBytes(vm.DiskRead), utils.FormatBytes(vm.DiskWrite))).SetTextColor(theme.Colors.Primary))
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
	}
	row++

	// Guest Agent (QEMU only)
	if vm.Type == api.VMTypeQemu {
		vd.SetCell(row, 0, tview.NewTableCell("👾 Guest Agent").SetTextColor(theme.Colors.HeaderText))
		agentStatus := "Not enabled"
		agentColor := theme.Colors.Secondary
		if vm.AgentEnabled {
			if vm.AgentRunning {
				agentStatus = "Running"
				agentColor = theme.Colors.StatusRunning
			} else {
				agentStatus = "Enabled but not running"
				agentColor = theme.Colors.StatusPending
			}
		}
		vd.SetCell(row, 1, tview.NewTableCell(agentStatus).SetTextColor(agentColor))
		row++
	}

	// Filesystems (detailed storage breakdown)
	if len(vm.Filesystems) > 0 {
		vd.SetCell(row, 0, tview.NewTableCell("📂 Filesystems").SetTextColor(theme.Colors.HeaderText))
		vd.SetCell(row, 1, tview.NewTableCell("").SetTextColor(theme.Colors.Primary))
		row++
		for _, fs := range vm.Filesystems {
			fsName := fs.Mountpoint
			if fsName == "" {
				fsName = getFriendlyFilesystemName(fs)
			}
			var usedPercent float64
			if fs.TotalBytes > 0 {
				usedPercent = float64(fs.UsedBytes) / float64(fs.TotalBytes) * 100
			} else {
				usedPercent = 0
			}
			usageColor := theme.GetUsageColor(usedPercent)
			vd.SetCell(row, 0, tview.NewTableCell("  • "+fsName).SetTextColor(theme.Colors.Info))
			vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%.2f%% (%s/%s)%s",
				usedPercent,
				utils.FormatBytes(fs.UsedBytes),
				utils.FormatBytes(fs.TotalBytes),
				func() string {
					if fs.Type != "" {
						return " [" + fs.Type + "]"
					} else {
						return ""
					}
				}(),
			)).SetTextColor(usageColor))
			row++
		}
	}

	// Detailed Network Interfaces (merged config + guest agent)
	enhancedNetworks := mergeNetworkInterfaces(vm.ConfiguredNetworks, vm.NetInterfaces)
	vd.SetCell(row, 0, tview.NewTableCell("🌐 Network Interfaces").SetTextColor(theme.Colors.HeaderText))
	if len(enhancedNetworks) > 0 {
		vd.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d interface(s)", len(enhancedNetworks))).SetTextColor(theme.Colors.Primary))
		row++
		for _, net := range enhancedNetworks {
			// Interface name with model/type and status
			interfaceText := ""
			if net.Interface != "" {
				interfaceText = net.Interface
				if net.Model != "" {
					interfaceText += fmt.Sprintf(" (%s)", net.Model)
				}
			} else if net.RuntimeName != "" {
				interfaceText = net.RuntimeName
			}
			// Add status indicator if we have guest agent data
			if net.HasGuestAgent {
				if net.IsUp {
					interfaceText += " 🟢"
				} else {
					interfaceText += " 🔴"
				}
			}
			// Mark guest-only interfaces
			if net.IsGuestOnly {
				interfaceText += " (guest only)"
			}
			vd.SetCell(row, 0, tview.NewTableCell("  • "+interfaceText).SetTextColor(theme.Colors.Info))
			// MAC address in right column
			macText := net.MACAddr
			if macText == "" {
				macText = "Auto-generated"
			}
			vd.SetCell(row, 1, tview.NewTableCell(macText).SetTextColor(theme.Colors.Secondary))
			row++
			// IP configuration details in right column (indented)
			var ipParts []string
			if net.ConfiguredIP != "" {
				if net.ConfiguredIP == "dhcp" {
					ipParts = append(ipParts, "DHCP")
				} else {
					ipParts = append(ipParts, net.ConfiguredIP)
				}
				if net.Gateway != "" {
					ipParts = append(ipParts, "GW: "+net.Gateway)
				}
			}
			if len(net.RuntimeIPs) > 0 {
				if len(ipParts) > 0 {
					ipParts = append(ipParts, "Runtime: "+strings.Join(net.RuntimeIPs, ", "))
				} else {
					ipParts = append(ipParts, "IPs: "+strings.Join(net.RuntimeIPs, ", "))
				}
			}
			if len(ipParts) > 0 {
				vd.SetCell(row, 0, tview.NewTableCell("").SetTextColor(theme.Colors.Info))
				vd.SetCell(row, 1, tview.NewTableCell(strings.Join(ipParts, " | ")).SetTextColor(theme.Colors.Secondary))
				row++
			}
			// Network configuration details in gray in right column
			var configParts []string
			if net.Bridge != "" {
				configParts = append(configParts, "Bridge: "+net.Bridge)
			}
			if net.VLAN != "" {
				configParts = append(configParts, "VLAN: "+net.VLAN)
			}
			if net.Rate != "" {
				configParts = append(configParts, "Rate: "+net.Rate)
			}
			if net.Firewall {
				configParts = append(configParts, "Firewall: enabled")
			}
			if len(configParts) > 0 {
				vd.SetCell(row, 0, tview.NewTableCell("").SetTextColor(theme.Colors.Info))
				vd.SetCell(row, 1, tview.NewTableCell(strings.Join(configParts, ", ")).SetTextColor(theme.Colors.Secondary))
				row++
			}
		}
	} else {
		vd.SetCell(row, 1, tview.NewTableCell(api.StringNA).SetTextColor(theme.Colors.Secondary))
		row++
	}

	// Storage Devices (from config)
	if len(vm.StorageDevices) > 0 {
		vd.SetCell(row, 0, tview.NewTableCell("💽 Storage Devices").SetTextColor(theme.Colors.HeaderText))
		vd.SetCell(row, 1, tview.NewTableCell("").SetTextColor(theme.Colors.Primary))
		row++
		for _, storage := range vm.StorageDevices {
			deviceText := storage.Device
			if storage.Size != "" {
				deviceText += fmt.Sprintf(" (%s)", storage.Size)
			}
			vd.SetCell(row, 0, tview.NewTableCell("  • "+deviceText).SetTextColor(theme.Colors.Info))
			storageText := storage.Storage
			if storage.Format != "" {
				storageText += fmt.Sprintf(" [%s]", storage.Format)
			}
			vd.SetCell(row, 1, tview.NewTableCell(storageText).SetTextColor(theme.Colors.Secondary))
			row++
			var options []string
			if storage.Cache != "" {
				options = append(options, fmt.Sprintf("Cache: %s", storage.Cache))
			}
			if storage.IOThread {
				options = append(options, "IOThread")
			}
			if storage.SSD {
				options = append(options, "SSD")
			}
			if storage.Discard != "" {
				options = append(options, fmt.Sprintf("Discard: %s", storage.Discard))
			}
			if storage.Serial != "" {
				options = append(options, fmt.Sprintf("Serial: %s", storage.Serial))
			}
			if !storage.Backup {
				options = append(options, "No Backup")
			}
			if storage.Replicate {
				options = append(options, "Replicate")
			}
			if len(options) > 0 {
				vd.SetCell(row, 0, tview.NewTableCell("").SetTextColor(theme.Colors.Info))
				vd.SetCell(row, 1, tview.NewTableCell(strings.Join(options, ", ")).SetTextColor(theme.Colors.Secondary))
				row++
			}
		}
	}

	// Configuration Section
	vd.SetCell(row, 0, tview.NewTableCell("⚙️ Configuration").SetTextColor(theme.Colors.HeaderText))
	vd.SetCell(row, 1, tview.NewTableCell("").SetTextColor(theme.Colors.Primary))
	row++

	// CPU Configuration (always show)
	cpuText := api.StringNA
	if vm.CPUCores > 0 && vm.CPUSockets > 0 {
		cpuText = fmt.Sprintf("%d cores, %d sockets", vm.CPUCores, vm.CPUSockets)
	} else if vm.CPUCores > 0 {
		cpuText = fmt.Sprintf("%d cores", vm.CPUCores)
	} else if vm.CPUSockets > 0 {
		cpuText = fmt.Sprintf("%d sockets", vm.CPUSockets)
	}
	vd.SetCell(row, 0, tview.NewTableCell("  • CPU").SetTextColor(theme.Colors.Info))
	vd.SetCell(row, 1, tview.NewTableCell(cpuText).SetTextColor(theme.Colors.Primary))
	row++

	// Architecture and OS Type (always show)
	archText := api.StringNA
	if vm.Architecture != "" && vm.OSType != "" {
		archText = fmt.Sprintf("%s (%s)", vm.Architecture, vm.OSType)
	} else if vm.Architecture != "" {
		archText = vm.Architecture
	} else if vm.OSType != "" {
		archText = vm.OSType
	}
	vd.SetCell(row, 0, tview.NewTableCell("  • Architecture").SetTextColor(theme.Colors.Info))
	vd.SetCell(row, 1, tview.NewTableCell(archText).SetTextColor(theme.Colors.Primary))
	row++

	// Boot Order
	if vm.BootOrder != "" {
		vd.SetCell(row, 0, tview.NewTableCell("  • Boot Order").SetTextColor(theme.Colors.Info))
		vd.SetCell(row, 1, tview.NewTableCell(vm.BootOrder).SetTextColor(theme.Colors.Primary))
		row++
	}

	// Auto-start
	autoStartText := "Disabled"
	autoStartColor := theme.Colors.Secondary
	if vm.OnBoot {
		autoStartText = "Enabled"
		autoStartColor = theme.Colors.Success
	}
	vd.SetCell(row, 0, tview.NewTableCell("  • Auto-start").SetTextColor(theme.Colors.Info))
	vd.SetCell(row, 1, tview.NewTableCell(autoStartText).SetTextColor(autoStartColor))

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

// mergeNetworkInterfaces combines configured networks with guest agent interfaces
// Returns enhanced network information with both config and runtime data
type EnhancedNetworkInterface struct {
	// From configuration
	Interface    string
	Model        string
	MACAddr      string
	Bridge       string
	VLAN         string
	Rate         string
	ConfiguredIP string
	Gateway      string
	Firewall     bool

	// From guest agent
	RuntimeName   string
	RuntimeIPs    []string
	IsUp          bool
	HasGuestAgent bool
	IsGuestOnly   bool // True if this interface is only visible via guest agent
}

func mergeNetworkInterfaces(configuredNets []api.ConfiguredNetwork, guestInterfaces []api.NetworkInterface) []EnhancedNetworkInterface {
	var enhanced []EnhancedNetworkInterface

	// Create a map of guest interfaces by MAC for quick lookup
	guestByMAC := make(map[string]api.NetworkInterface)
	for _, iface := range guestInterfaces {
		if iface.MACAddress != "" {
			guestByMAC[strings.ToUpper(iface.MACAddress)] = iface
		}
	}

	// Process configured networks first (these are authoritative)
	for _, configured := range configuredNets {
		enhancedNet := EnhancedNetworkInterface{
			Interface:    configured.Interface,
			Model:        configured.Model,
			MACAddr:      configured.MACAddr,
			Bridge:       configured.Bridge,
			VLAN:         configured.VLAN,
			Rate:         configured.Rate,
			ConfiguredIP: configured.IP,
			Gateway:      configured.Gateway,
			Firewall:     configured.Firewall,
		}

		// Try to find matching guest interface by MAC
		if configured.MACAddr != "" {
			if guest, found := guestByMAC[strings.ToUpper(configured.MACAddr)]; found {
				enhancedNet.RuntimeName = guest.Name
				// Convert IPAddress slice to string slice
				for _, ip := range guest.IPAddresses {
					enhancedNet.RuntimeIPs = append(enhancedNet.RuntimeIPs, ip.Address)
				}
				// Determine if interface is up based on having IP addresses
				enhancedNet.IsUp = len(guest.IPAddresses) > 0
				enhancedNet.HasGuestAgent = true
				// Remove from map so we don't show it again
				delete(guestByMAC, strings.ToUpper(configured.MACAddr))
			}
		}

		enhanced = append(enhanced, enhancedNet)
	}

	// Add any remaining guest interfaces that didn't match configured ones
	for _, guest := range guestByMAC {
		if guest.IsLoopback {
			continue // Skip loopback interfaces
		}

		enhancedNet := EnhancedNetworkInterface{
			RuntimeName:   guest.Name,
			MACAddr:       guest.MACAddress,
			HasGuestAgent: true,
			IsGuestOnly:   true, // Flag to indicate this is guest-agent only
		}

		// Convert IPAddress slice to string slice
		for _, ip := range guest.IPAddresses {
			enhancedNet.RuntimeIPs = append(enhancedNet.RuntimeIPs, ip.Address)
		}
		enhancedNet.IsUp = len(guest.IPAddresses) > 0

		enhanced = append(enhanced, enhancedNet)
	}

	return enhanced
}
