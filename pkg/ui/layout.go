package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-util/pkg/api"
	"github.com/rivo/tview"
)

// CreateMainLayout creates the main application layout
func CreateMainLayout(summaryPanel *tview.Flex, pages *tview.Pages, footer *tview.TextView) *tview.Flex {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(summaryPanel, 5, 0, false).
		AddItem(pages, 0, 1, true).
		AddItem(footer, 1, 0, false)
}

// SetupKeyboardHandlers configures global keyboard shortcuts
func SetupKeyboardHandlers(
	app *tview.Application,
	pages *tview.Pages,
	nodeList *tview.List,
	vmList *tview.List,
	vms []api.VM,
	vmDetails *tview.Table,
	header *tview.TextView,
) *tview.Pages {
	// Set up keyboard input handling
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle global keys
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			app.Stop()
			return nil
		case tcell.KeyTab:
			// Cycle focus between panels based on current page
			if curPage, _ := pages.GetFrontPage(); curPage == "Nodes" {
				if nodeList.HasFocus() {
					app.SetFocus(vmList)
				} else {
					app.SetFocus(nodeList)
				}
			} else if curPage == "Guests" {
				if vmList.HasFocus() {
					app.SetFocus(vmDetails)
				} else {
					app.SetFocus(vmList)
				}
			}
			return nil
		case tcell.KeyF1:
			pages.SwitchToPage("Nodes")
			app.SetFocus(nodeList)
			return nil
		case tcell.KeyF2:
			pages.SwitchToPage("Guests")
			app.SetFocus(vmList)
			return nil
		}
		return event
	})
	
	return pages
}

// CreatePagesContainer creates the tab container for different views
func CreatePagesContainer() *tview.Pages {
	return tview.NewPages()
}

// AddNodesPage adds the nodes view to the pages container
func AddNodesPage(pages *tview.Pages, nodeContent tview.Primitive) {
	pages.AddPage("Nodes", nodeContent, true, true)
}

// AddGuestsPage adds the VMs/containers view to the pages container
func AddGuestsPage(pages *tview.Pages, vmList *tview.List, vmDetails *tview.Table) {
	// Set up guests tab with VM list and details side by side
	guestsContent := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(vmList, 0, 1, true).
		AddItem(vmDetails, 0, 2, false)
	
	pages.AddPage("Guests", guestsContent, true, false)
}

// SetupVMHandlers configures VM list handlers
func SetupVMHandlers(vmList *tview.List, vmDetails *tview.Table, vms []api.VM, client *api.Client) {
	// Update details on hover
	vmList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(vms) {
			populateVmDetails(vmDetails, vms[index])
		}
	})
	
	// Update details on selection
	vmList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(vms) {
			populateVmDetails(vmDetails, vms[index])
		}
	})
}
