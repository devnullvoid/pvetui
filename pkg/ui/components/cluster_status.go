package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// ClusterStatus encapsulates the cluster status panel
type ClusterStatus struct {
	*tview.Flex
	SummaryTable  *tview.Table
	ResourceTable *tview.Table
}

// NewClusterStatus creates a new cluster status panel
func NewClusterStatus() *ClusterStatus {
	// Create container panel
	panel := tview.NewFlex()
	panel.SetDirection(tview.FlexColumn)
	panel.SetBorder(true)
	panel.SetTitle("Cluster Status")

	// Create summary table
	summary := tview.NewTable()
	summary.SetBorders(false)
	summary.SetTitleAlign(tview.AlignLeft)

	// Initial cluster status rows (no header, "Details" column removed)
	rows := [][]string{
		{"Cluster", "Loading..."},
		{"PVE Version", "Loading..."},
		{"Nodes Online", "Loading..."},
	}
	for row, fields := range rows {
		for col, text := range fields {
			cell := tview.NewTableCell(text).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignLeft)
			summary.SetCell(row, col, cell) // Data starts at row 0
		}
	}

	// Create resource table
	resourceTable := tview.NewTable()
	resourceTable.SetBorders(false)
	resourceTable.SetTitleAlign(tview.AlignLeft)

	// Resource table headers
	resourceHeaders := []string{"Resource", "Total", "Used"}
	for col, text := range resourceHeaders {
		cell := tview.NewTableCell(text).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter)
		resourceTable.SetCell(0, col, cell)
	}

	// Add both tables to panel with equal space
	panel.AddItem(summary, 0, 1, false)
	panel.AddItem(resourceTable, 0, 1, false)
	
	return &ClusterStatus{
		Flex:          panel,
		SummaryTable:  summary,
		ResourceTable: resourceTable,
	}
}

// Update populates both tables with current cluster data
func (cs *ClusterStatus) Update(cluster *api.Cluster) {
	if cluster == nil {
		return
	}
	
	// Clear existing content except headers in both tables
	for _, tbl := range []*tview.Table{cs.SummaryTable, cs.ResourceTable} {
		for row := 1; row <= 6; row++ {
			for col := 0; col < 4; col++ {
				tbl.SetCell(row, col, tview.NewTableCell(""))
			}
		}
	}

	// Update summary table (left panel)
	// Data now starts at row 0
	cs.SummaryTable.SetCell(0, 0, tview.NewTableCell("Cluster Name").SetTextColor(tcell.ColorYellow))
	cs.SummaryTable.SetCell(0, 1, tview.NewTableCell(cluster.Name).SetTextColor(tcell.ColorWhite))

	// Show only the version number (e.g., '8.3.5') in the 'Proxmox VE' row
	ver := cluster.Version
	if parts := strings.Split(ver, "/"); len(parts) > 1 {
		ver = parts[1]
	}
	cs.SummaryTable.SetCell(1, 0, tview.NewTableCell("Proxmox VE").SetTextColor(tcell.ColorYellow))
	cs.SummaryTable.SetCell(1, 1, tview.NewTableCell(ver).SetTextColor(tcell.ColorWhite))

	cs.SummaryTable.SetCell(2, 0, tview.NewTableCell("Nodes Online").SetTextColor(tcell.ColorYellow))
	cs.SummaryTable.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%d/%d 🟢", cluster.OnlineNodes, cluster.TotalNodes)).SetTextColor(tcell.ColorWhite))

	// Update resource table (right panel)
	cs.ResourceTable.SetCell(1, 0, tview.NewTableCell("CPU Cores").SetTextColor(tcell.ColorYellow))
	cs.ResourceTable.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("%.1f", cluster.TotalCPU)).SetTextColor(tcell.ColorWhite))
	cs.ResourceTable.SetCell(1, 2, tview.NewTableCell(fmt.Sprintf("%.1f%%", cluster.CPUUsage*100)).SetTextColor(tcell.ColorWhite))

	cs.ResourceTable.SetCell(2, 0, tview.NewTableCell("Memory").SetTextColor(tcell.ColorYellow))
	cs.ResourceTable.SetCell(2, 1, tview.NewTableCell(fmt.Sprintf("%.1f GB", cluster.MemoryTotal)).SetTextColor(tcell.ColorWhite))
	cs.ResourceTable.SetCell(2, 2, tview.NewTableCell(fmt.Sprintf("%.1f GB", cluster.MemoryUsed)).SetTextColor(tcell.ColorWhite))
} 