package guestlist

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devnullvoid/pvetui/pkg/api"
)

// guestRow represents a VM/container entry rendered inside the guest list view.
type guestRow struct {
	vm             *api.VM
	nodeName       string
	id             int
	name           string
	typeLabel      string
	status         string
	cpuPercent     float64
	memUsed        int64
	memTotal       int64
	diskUsed       int64
	diskTotal      int64
	uptime         int64
	ip             string
	tags           []string
	agentEnabled   bool
	agentRunning   bool
	filterHaystack string
}

// guestSummary aggregates information about the currently visible guests.
type guestSummary struct {
	totalGuests   int
	visibleGuests int
	runningGuests int
	cpuTotal      float64
	memUsedTotal  int64
	memMaxTotal   int64
}

// sortKey controls column sorting inside the guest table.
type sortKey int

const (
	sortByCPU sortKey = iota
	sortByMemory
	sortByUptime
	sortByName
	sortByID
)

const selectedNodeDisplayName = "selected node"

func (s sortKey) String() string {
	switch s {
	case sortByCPU:
		return "CPU"
	case sortByMemory:
		return "Memory"
	case sortByUptime:
		return "Uptime"
	case sortByName:
		return "Name"
	case sortByID:
		return "ID"
	default:
		return "Unknown"
	}
}

func buildGuestRows(node *api.Node) []guestRow {
	if node == nil {
		return nil
	}

	rows := make([]guestRow, 0, len(node.VMs))
	for _, vm := range node.VMs {
		if vm == nil || vm.Template {
			continue
		}

		name := strings.TrimSpace(vm.Name)
		if name == "" {
			name = fmt.Sprintf("VM %d", vm.ID)
		}

		status := strings.ToLower(strings.TrimSpace(vm.Status))
		typeLabel := strings.ToUpper(strings.TrimSpace(vm.Type))
		if typeLabel == "" {
			typeLabel = "VM"
		}

		tags := normalizeTags(vm.Tags)
		haystack := strings.ToLower(strings.Join([]string{
			fmt.Sprintf("%d", vm.ID),
			name,
			status,
			typeLabel,
			strings.TrimSpace(vm.IP),
			strings.Join(tags, " "),
		}, " "))

		rows = append(rows, guestRow{
			vm:             vm,
			nodeName:       node.Name,
			id:             vm.ID,
			name:           name,
			typeLabel:      typeLabel,
			status:         status,
			cpuPercent:     vm.CPU * 100,
			memUsed:        vm.Mem,
			memTotal:       vm.MaxMem,
			diskUsed:       vm.Disk,
			diskTotal:      vm.MaxDisk,
			uptime:         vm.Uptime,
			ip:             strings.TrimSpace(vm.IP),
			tags:           tags,
			agentEnabled:   vm.AgentEnabled,
			agentRunning:   vm.AgentRunning,
			filterHaystack: haystack,
		})
	}

	return rows
}

func filterGuestRows(rows []guestRow, includeStopped bool, needle string) []guestRow {
	if len(rows) == 0 {
		return nil
	}

	needle = strings.ToLower(strings.TrimSpace(needle))
	filtered := make([]guestRow, 0, len(rows))
	for _, row := range rows {
		if !includeStopped && row.status != api.VMStatusRunning {
			continue
		}
		if needle != "" && !strings.Contains(row.filterHaystack, needle) {
			continue
		}
		filtered = append(filtered, row)
	}

	return filtered
}

func sortGuestRows(rows []guestRow, key sortKey, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		if desc {
			return compareRows(rows[j], rows[i], key)
		}
		return compareRows(rows[i], rows[j], key)
	})
}

func compareRows(a, b guestRow, key sortKey) bool {
	switch key {
	case sortByCPU:
		return a.cpuPercent < b.cpuPercent
	case sortByMemory:
		return memoryRatio(a) < memoryRatio(b)
	case sortByUptime:
		return a.uptime < b.uptime
	case sortByName:
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	case sortByID:
		return a.id < b.id
	default:
		return a.id < b.id
	}
}

func memoryRatio(row guestRow) float64 {
	if row.memTotal <= 0 {
		return 0
	}

	return float64(row.memUsed) / float64(row.memTotal)
}

func summarizeGuests(allRows, visibleRows []guestRow) guestSummary {
	summary := guestSummary{
		totalGuests:   len(allRows),
		visibleGuests: len(visibleRows),
	}

	for _, row := range allRows {
		if row.status == api.VMStatusRunning {
			summary.runningGuests++
		}
	}

	for _, row := range visibleRows {
		summary.cpuTotal += row.cpuPercent
		summary.memUsedTotal += row.memUsed
		if row.memTotal > 0 {
			summary.memMaxTotal += row.memTotal
		}
	}

	return summary
}

func normalizeTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})

	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}

	return tags
}

func nodeDisplayName(node *api.Node) string {
	if node == nil {
		return selectedNodeDisplayName
	}

	name := strings.TrimSpace(node.Name)
	if name == "" {
		return selectedNodeDisplayName
	}

	return name
}
