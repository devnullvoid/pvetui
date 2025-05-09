package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/lonepie/proxmox-tui/pkg/api"
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
	nodes []api.Node,
	vmDetails *tview.Table,
	header *tview.TextView,
) *tview.Pages {
	// Create shell info panel for displaying shell commands
	shellInfoPanel := CreateShellInfoPanel()
	// Add the shell info panel to a new page
	pages.AddPage("ShellInfo", shellInfoPanel, true, false)

	// Set up keyboard input handling
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// First, handle rune keys (like 'S')
		if event.Key() == tcell.KeyRune {
			// Handle shell session launch
			if event.Rune() == 's' || event.Rune() == 'S' {
				curPage, _ := pages.GetFrontPage()
				if curPage == "Guests" && vmList.HasFocus() {
					index := vmList.GetCurrentItem()
					if index >= 0 && index < len(vms) {
						vm := vms[index]
						HandleShellExecution(app, vm)
						return nil
					}
				} else if curPage == "Nodes" && nodeList.HasFocus() {
					index := nodeList.GetCurrentItem()
					if index >= 0 && index < len(nodes) {
						node := nodes[index]
						HandleShellExecution(app, node)
						return nil
					}
				}
				curPage, _ = pages.GetFrontPage()
				if curPage == "Guests" {
					index := vmList.GetCurrentItem()
					if index >= 0 && index < len(vms) {
						vm := vms[index]
						HandleShellExecution(app, vm)
						return nil
					}
				} else if curPage == "Nodes" {
					index := nodeList.GetCurrentItem()
					if index >= 0 && index < len(nodes) {
						node := nodes[index]
						HandleShellExecution(app, node)
						return nil
					}
				}
			} else if event.Rune() == 'q' {
				app.Stop()
				return nil
			} else if event.Rune() == '/' {
				handleSearchInput(app, pages, nodeList, vmList, nodes, vms)
				return nil
			}

		}

		// Then handle special keys
		switch event.Key() {
		case tcell.KeyEscape:
			// Handle search mode first
			if curPage, _ := pages.GetFrontPage(); curPage == "Search" {
				pages.RemovePage("Search")
				lastSearchText = "" // Clear persisted search text

				// Get the underlying page after removing search
				basePage, _ := pages.GetFrontPage()
				if basePage == "Nodes" {
					nodeList.Clear()
					for _, node := range nodes {
						nodeList.AddItem(node.Name, "", 0, nil)
					}
					app.SetFocus(nodeList)
				} else if basePage == "Guests" {
					vmList.Clear()
					for _, vm := range vms {
						vmList.AddItem(vm.Name, vm.Status, 0, nil)
					}
					app.SetFocus(vmList)
				}
				return nil
			}
			// Then handle shell info panel
			if curPage, _ := pages.GetFrontPage(); curPage == "ShellInfo" {
				pages.SwitchToPage("Guests")
				app.SetFocus(vmList)
				return nil
			}
			// Otherwise, exit the application
			app.Stop()
			return nil
		case tcell.KeyCtrlC:
			app.Stop()
			return nil
		case tcell.KeyTab:
			// Cycle between pages
			curPage, _ := pages.GetFrontPage()
			if curPage == "Nodes" {
				pages.SwitchToPage("Guests")
				app.SetFocus(vmList)
			} else if curPage == "Guests" {
				pages.SwitchToPage("Nodes")
				app.SetFocus(nodeList)
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
