package guestlist

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/components"
	"github.com/devnullvoid/pvetui/internal/ui/theme"
	"github.com/devnullvoid/pvetui/internal/ui/utils"
	"github.com/devnullvoid/pvetui/pkg/api"
)

const (
	guestListModalWidth = 120
)

type guestListView struct {
	app  *components.App
	node *api.Node

	allRows        []guestRow
	rows           []guestRow
	filterValue    string
	includeStopped bool
	sortKey        sortKey
	sortDesc       bool

	statsText   *tview.TextView
	table       *tview.Table
	filterInput *tview.InputField
	helpText    *tview.TextView
	frame       *tview.Frame

	helpBase   string
	prevFocus  tview.Primitive
	refreshing atomic.Bool
}

func newGuestListView(app *components.App, node *api.Node, rows []guestRow) *guestListView {
	return &guestListView{
		app:            app,
		node:           node,
		allRows:        rows,
		includeStopped: false,
		sortKey:        sortByCPU,
		sortDesc:       true,
		helpBase:       "↑/↓ navigate  •  enter/g jump to guest  •  / search  •  a toggle stopped  •  n/c/m/u/i sort  •  r refresh metrics  •  esc close",
	}
}

func (v *guestListView) show(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	v.prevFocus = v.app.GetFocus()
	v.buildLayout()
	v.applyState()

	modal := centerModal(v.frame, guestListModalWidth)
	v.app.Pages().AddPage(guestListModalPageName, modal, true, true)
	v.app.SetFocus(v.table)

	v.refreshMetrics(ctx, true)
}

func (v *guestListView) buildLayout() {
	v.statsText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(theme.Colors.Primary)

	v.filterInput = tview.NewInputField().
		SetLabel("Filter: ").
		SetFieldWidth(0).
		SetPlaceholder("name, IP, tag or ID")

	v.filterInput.SetChangedFunc(func(text string) {
		v.filterValue = text
		v.applyState()
	})

	v.filterInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter, tcell.KeyTab:
			v.app.SetFocus(v.table)
		case tcell.KeyEscape:
			v.close()
		}
	})

	v.table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	v.table.SetBorder(true)
	v.table.SetBorderColor(theme.Colors.Border)
	v.table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			v.close()
		}
	})

	v.table.SetSelectedFunc(func(row, _ int) {
		v.jumpToRow(row)
	})

	v.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				v.app.SetFocus(v.filterInput)
				return nil
			case 'a', 'A':
				v.toggleIncludeStopped()
				return nil
			case 'c', 'C':
				v.setSort(sortByCPU)
				return nil
			case 'm', 'M':
				v.setSort(sortByMemory)
				return nil
			case 'u', 'U':
				v.setSort(sortByUptime)
				return nil
			case 'n', 'N':
				v.setSort(sortByName)
				return nil
			case 'i', 'I':
				v.setSort(sortByID)
				return nil
			case 'g', 'G':
				row, _ := v.table.GetSelection()
				v.jumpToRow(row)
				return nil
			case 'r', 'R':
				v.refreshMetrics(context.Background(), true)
				return nil
			case 'q', 'Q':
				v.close()
				return nil
			}
		case tcell.KeyEnter:
			row, _ := v.table.GetSelection()
			v.jumpToRow(row)
			return nil
		case tcell.KeyEscape:
			v.close()
			return nil
		}

		return event
	})

	v.helpText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(theme.Colors.Secondary)
	v.helpText.SetText(v.helpBase)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.statsText, 2, 0, false).
		AddItem(v.filterInput, 1, 0, false).
		AddItem(v.table, 0, 1, true).
		AddItem(v.helpText, 1, 0, false)

	v.frame = tview.NewFrame(layout)
	v.frame.SetBorder(true)
	v.frame.SetBorderColor(theme.Colors.Border)
	v.frame.SetTitle(fmt.Sprintf(" Guest Insights • %s ", nodeDisplayName(v.node)))
	v.frame.SetTitleColor(theme.Colors.Primary)
}

func (v *guestListView) applyState() {
	v.rows = filterGuestRows(v.allRows, v.includeStopped, v.filterValue)
	sortGuestRows(v.rows, v.sortKey, v.sortDesc)
	v.populateTable()
	v.updateStats()
	v.helpText.SetText(v.helpBase)
}

func (v *guestListView) populateTable() {
	v.table.Clear()
	headers := []struct {
		label     string
		align     int
		expansion int
	}{
		{"Guest", tview.AlignLeft, 2},
		{"Type", tview.AlignLeft, 1},
		{"Status", tview.AlignLeft, 1},
		{"CPU", tview.AlignRight, 1},
		{"Memory", tview.AlignLeft, 2},
		{"Uptime", tview.AlignLeft, 1},
		{"IP", tview.AlignLeft, 2},
		{"Tags", tview.AlignLeft, 2},
		{"Agent", tview.AlignLeft, 1},
	}

	for idx, header := range headers {
		cell := tview.NewTableCell(header.label).
			SetTextColor(theme.Colors.HeaderText).
			SetSelectable(false).
			SetAlign(header.align).
			SetExpansion(header.expansion)
		v.table.SetCell(0, idx, cell)
	}

	if len(v.rows) == 0 {
		message := "No matching guests. Press 'a' to include stopped guests."
		cell := tview.NewTableCell(message).
			SetTextColor(theme.Colors.Warning).
			SetSelectable(false).
			SetAlign(tview.AlignCenter)
		v.table.SetCell(1, 0, cell)
		for col := 1; col < len(headers); col++ {
			v.table.SetCell(1, col, tview.NewTableCell("").SetSelectable(false))
		}

		return
	}

	for i, row := range v.rows {
		rowIdx := i + 1
		v.table.SetCell(rowIdx, 0, tview.NewTableCell(formatGuestLabel(row)).
			SetTextColor(theme.Colors.Primary).
			SetAlign(tview.AlignLeft))

		v.table.SetCell(rowIdx, 1, tview.NewTableCell(row.typeLabel).
			SetTextColor(theme.Colors.Secondary))

		statusText := formatStatusText(row.status)
		statusColor := statusColorFor(row.status)
		v.table.SetCell(rowIdx, 2, tview.NewTableCell(statusText).
			SetTextColor(statusColor))

		cpuText := fmt.Sprintf("%.1f%%", row.cpuPercent)
		v.table.SetCell(rowIdx, 3, tview.NewTableCell(cpuText).
			SetAlign(tview.AlignRight).
			SetTextColor(theme.GetUsageColor(row.cpuPercent)))

		memText, memPct := formatMemoryCell(row)
		v.table.SetCell(rowIdx, 4, tview.NewTableCell(memText).
			SetTextColor(theme.GetUsageColor(memPct)))

		uptimeText := utils.FormatUptime(int(row.uptime))
		if uptimeText == "" {
			uptimeText = api.StringNA
		}
		v.table.SetCell(rowIdx, 5, tview.NewTableCell(uptimeText).
			SetTextColor(theme.Colors.Secondary))

		ipText := row.ip
		if strings.TrimSpace(ipText) == "" {
			ipText = api.StringNA
		}
		v.table.SetCell(rowIdx, 6, tview.NewTableCell(ipText).
			SetTextColor(theme.Colors.Primary))

		tagText := api.StringNA
		if len(row.tags) > 0 {
			tagText = strings.Join(row.tags, ", ")
		}
		v.table.SetCell(rowIdx, 7, tview.NewTableCell(tagText).
			SetTextColor(theme.Colors.Info))

		agentText, agentColor := agentStateCell(row)
		v.table.SetCell(rowIdx, 8, tview.NewTableCell(agentText).
			SetTextColor(agentColor))
	}
}

func (v *guestListView) updateStats() {
	summary := summarizeGuests(v.allRows, v.rows)
	modeLabel := "running only"
	if v.includeStopped {
		modeLabel = "all guests"
	}

	memUsed := utils.FormatBytes(summary.memUsedTotal)
	memTotal := api.StringNA
	if summary.memMaxTotal > 0 {
		memTotal = utils.FormatBytes(summary.memMaxTotal)
	}

	arrow := "↓"
	if !v.sortDesc {
		arrow = "↑"
	}

	stats := fmt.Sprintf("%s • Showing %d/%d guests (%s, %d running) • ΣCPU %.1f%% • ΣMem %s",
		nodeDisplayName(v.node), summary.visibleGuests, summary.totalGuests, modeLabel, summary.runningGuests, summary.cpuTotal, formatMemorySummary(memUsed, memTotal))
	stats = fmt.Sprintf("%s • Sort: %s %s", stats, v.sortKey.String(), arrow)
	stats = theme.ReplaceSemanticTags(fmt.Sprintf("[primary]%s[-]", stats))
	v.statsText.SetText(stats)
}

func (v *guestListView) toggleIncludeStopped() {
	v.includeStopped = !v.includeStopped
	v.applyState()
}

func (v *guestListView) setSort(key sortKey) {
	if v.sortKey == key {
		v.sortDesc = !v.sortDesc
	} else {
		v.sortKey = key
		v.sortDesc = key == sortByCPU || key == sortByMemory || key == sortByUptime
	}

	v.applyState()
}

func (v *guestListView) jumpToRow(row int) {
	if row <= 0 || row > len(v.rows) {
		return
	}
	selected := v.rows[row-1].vm
	if selected == nil {
		return
	}

	vmList := v.app.VMList()
	if vmList == nil {
		return
	}

	var focusTarget tview.Primitive
	if primitive, ok := vmList.(tview.Primitive); ok {
		focusTarget = primitive
	}

	vms := vmList.GetVMs()
	for idx, vm := range vms {
		if vm != nil && vm.ID == selected.ID && strings.EqualFold(vm.Node, selected.Node) {
			rowIdx := idx
			go func() {
				v.app.QueueUpdateDraw(func() {
					vmList.SetCurrentItem(rowIdx)
					v.app.Pages().SwitchToPage(api.PageGuests)
					v.closeWithFocus(focusTarget)
				})
			}()

			return
		}
	}

	v.app.ShowMessageSafe("Selected guest is not visible in the main list yet. Try refreshing the cluster data.")
}

func (v *guestListView) refreshMetrics(ctx context.Context, bustCache bool) {
	if !v.refreshing.CompareAndSwap(false, true) {
		v.showHelpStatus("Refresh already in progress", theme.Colors.Warning)

		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	v.showHelpStatus("Refreshing metrics…", theme.Colors.Info)

	client := v.app.Client()
	if client == nil {
		v.refreshing.Store(false)
		v.showHelpStatus("API client unavailable", theme.Colors.Error)

		return
	}
	rows := make([]guestRow, len(v.allRows))
	copy(rows, v.allRows)

	if len(rows) == 0 {
		v.refreshing.Store(false)
		v.showHelpStatus("No guests to refresh", theme.Colors.Secondary)

		return
	}

	hasRunning := false
	for _, row := range rows {
		if row.vm != nil && row.status == api.VMStatusRunning {
			hasRunning = true
			break
		}
	}

	if !hasRunning {
		v.refreshing.Store(false)
		v.showHelpStatus("No running guests to refresh", theme.Colors.Secondary)

		return
	}

	if bustCache {
		client.ClearAPICache()
	}

	go func() {
		defer v.refreshing.Store(false)

		var refreshErr error
		refreshed := false
		for _, row := range rows {
			if row.vm == nil || row.status != api.VMStatusRunning {
				continue
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := client.GetVmStatus(row.vm); err != nil {
				if refreshErr == nil {
					refreshErr = err
				}
				continue
			}

			refreshed = true
		}

		v.app.QueueUpdateDraw(func() {
			if refreshed {
				v.allRows = buildGuestRows(v.node)
			}
			v.applyState()
			switch {
			case refreshErr != nil:
				v.showHelpStatus(fmt.Sprintf("Refresh completed with errors: %v", refreshErr), theme.Colors.Warning)
			case !refreshed:
				v.showHelpStatus("No running guests to refresh", theme.Colors.Secondary)
			default:
				v.showHelpStatus("Metrics refreshed", theme.Colors.Success)
			}
		})
	}()
}

func (v *guestListView) showHelpStatus(message string, color tcell.Color) {
	tag := theme.ColorToTag(color)
	text := fmt.Sprintf("%s  •  [%s]%s[-]", v.helpBase, tag, message)
	v.helpText.SetText(text)

	go func() {
		time.Sleep(3 * time.Second)
		v.app.QueueUpdateDraw(func() {
			v.helpText.SetText(v.helpBase)
		})
	}()
}

func (v *guestListView) close() {
	v.closeWithFocus(nil)
}

func (v *guestListView) closeWithFocus(next tview.Primitive) {
	v.app.Pages().RemovePage(guestListModalPageName)
	target := next
	if target == nil {
		target = v.prevFocus
	}
	if target != nil {
		v.app.SetFocus(target)
	}
}

const (
	modalVerticalPadding = 2
)

func centerModal(content tview.Primitive, width int) tview.Primitive {
	column := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, modalVerticalPadding, 0, false).
		AddItem(content, 0, 1, true).
		AddItem(nil, modalVerticalPadding, 0, false)

	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(column, width, 0, true).
		AddItem(nil, 0, 1, false)
}

func formatGuestLabel(row guestRow) string {
	return fmt.Sprintf("%4d  %s", row.id, row.name)
}

func formatStatusText(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return api.StringNA
	}

	return strings.ToUpper(status[:1]) + status[1:]
}

func statusColorFor(status string) tcell.Color {
	switch strings.ToLower(status) {
	case api.VMStatusRunning:
		return theme.Colors.Success
	case api.VMStatusStopped:
		return theme.Colors.Error
	default:
		return theme.Colors.Warning
	}
}

func formatMemoryCell(row guestRow) (string, float64) {
	if row.memTotal <= 0 {
		return utils.FormatBytes(row.memUsed), 0
	}

	percent := memoryRatio(row) * 100
	text := fmt.Sprintf("%s / %s (%.0f%%)", utils.FormatBytes(row.memUsed), utils.FormatBytes(row.memTotal), percent)

	return text, percent
}

func formatMemorySummary(used, total string) string {
	if total == api.StringNA {
		return used
	}

	return fmt.Sprintf("%s / %s", used, total)
}

func agentStateCell(row guestRow) (string, tcell.Color) {
	switch {
	case row.agentEnabled && row.agentRunning:
		return "Running", theme.Colors.Success
	case row.agentEnabled:
		return "Enabled", theme.Colors.Warning
	default:
		return "Disabled", theme.Colors.Secondary
	}
}
