package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func handleVimTopBottomRune(event *tcell.EventKey, pendingG *bool, jumpTop func(), jumpBottom func()) bool {
	if event.Key() != tcell.KeyRune {
		*pendingG = false
		return false
	}

	switch event.Rune() {
	case 'g':
		if *pendingG {
			*pendingG = false
			jumpTop()
		} else {
			*pendingG = true
		}
		return true
	case 'G':
		*pendingG = false
		jumpBottom()
		return true
	default:
		*pendingG = false
		return false
	}
}

func jumpListTop(list *tview.List) {
	if list == nil || list.GetItemCount() == 0 {
		return
	}
	list.SetCurrentItem(0)
}

func jumpListBottom(list *tview.List) {
	if list == nil {
		return
	}
	count := list.GetItemCount()
	if count <= 0 {
		return
	}
	list.SetCurrentItem(count - 1)
}

func jumpTableTop(table *tview.Table) {
	if table == nil {
		return
	}
	rows := table.GetRowCount()
	if rows <= 1 {
		table.Select(0, 0)
		return
	}
	table.Select(1, 0)
}

func jumpTableBottom(table *tview.Table) {
	if table == nil {
		return
	}
	rows := table.GetRowCount()
	if rows <= 1 {
		table.Select(0, 0)
		return
	}
	table.Select(rows-1, 0)
}
