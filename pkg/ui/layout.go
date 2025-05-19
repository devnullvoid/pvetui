package ui

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/models"
	"github.com/gdamore/tcell/v2"
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
func (a *AppUI) SetupKeyboardHandlers(
	pages *tview.Pages,
	nodeList *tview.List,
	vmList *tview.List,
	originalVMs []*api.VM,
	originalNodes []*api.Node,
	vmDetails *tview.Table,
	header *tview.TextView,
) *tview.Pages {
	// Add tab change handler
	pages.SetChangedFunc(func() {
		currentPage, _ := pages.GetFrontPage()
		switch currentPage {
		case "Nodes":
			nodesToDisplay := models.GlobalState.OriginalNodes
			// Check if a search is active for the Nodes page
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SearchText != "" && len(models.GlobalState.FilteredNodes) > 0 {
				nodesToDisplay = models.GlobalState.FilteredNodes
			}
			nodeList.Clear()
			for _, node := range nodesToDisplay {
				if node != nil {
					nodeList.AddItem(FormatNodeName(node), "", 0, nil)
				}
			}
			a.updateNodeSelectionHandlers(nodeList, nodesToDisplay)
			if len(nodesToDisplay) > 0 {
				// Try to restore selected index if valid, else default to 0
				idx := 0
				if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SelectedIndex < len(nodesToDisplay) && state.SelectedIndex >= 0 {
					idx = state.SelectedIndex
				}
				nodeList.SetCurrentItem(idx)
				a.updateNodeDetails(nodesToDisplay[idx])
			} else {
				// Clear details if list is empty
				a.updateNodeDetails(nil)
			}
		case "Guests":
			vmsToDisplay := models.GlobalState.OriginalVMs
			// Check if a search is active for the Guests page
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SearchText != "" && len(models.GlobalState.FilteredVMs) > 0 {
				vmsToDisplay = models.GlobalState.FilteredVMs
			}
			BuildVMList(vmsToDisplay, vmList)
			a.updateVMSelectionHandlers(vmList, vmsToDisplay, vmDetails)
			if len(vmsToDisplay) > 0 {
				// Try to restore selected index if valid, else default to 0
				idx := 0
				if state, exists := models.GlobalState.SearchStates[currentPage]; exists && state.SelectedIndex < len(vmsToDisplay) && state.SelectedIndex >= 0 {
					idx = state.SelectedIndex
				}
				vmList.SetCurrentItem(idx)
				a.updateVMDetails(vmsToDisplay[idx])
			} else {
				// Clear details if list is empty
				a.updateVMDetails(nil)
			}
		}
	})

	// Trigger initial page load
	if currentPage, _ := pages.GetFrontPage(); currentPage == "Nodes" {
		nodeList.Clear()
		for _, node := range models.GlobalState.OriginalNodes {
			if node != nil {
				nodeList.AddItem(FormatNodeName(node), "", 0, nil)
			}
		}
		if len(models.GlobalState.OriginalNodes) > 0 {
			nodeList.SetCurrentItem(0)
			a.updateNodeDetails(models.GlobalState.OriginalNodes[0])
		}
		a.updateNodeSelectionHandlers(nodeList, models.GlobalState.OriginalNodes)
	} else if currentPage == "Guests" {
		vmList.Clear()
		BuildVMList(models.GlobalState.OriginalVMs, vmList)
		if len(models.GlobalState.OriginalVMs) > 0 {
			vmList.SetCurrentItem(0)
			a.updateVMDetails(models.GlobalState.OriginalVMs[0])
		}
		a.updateVMSelectionHandlers(vmList, models.GlobalState.OriginalVMs, vmDetails)
	}

	// Create shell info panel for displaying shell commands
	shellInfoPanel := CreateShellInfoPanel()
	// Add the shell info panel to a new page
	pages.AddPage("ShellInfo", shellInfoPanel, true, false)

	// Set initial focus based on the current page
	if currentPage, _ := pages.GetFrontPage(); currentPage == "Nodes" {
		a.app.SetFocus(nodeList)
	} else {
		a.app.SetFocus(vmList)
	}

	// Set up keyboard input handling
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// First, handle rune keys (like 'S')
		if event.Key() == tcell.KeyRune {
			// Handle shell session launch
			if event.Rune() == 's' || event.Rune() == 'S' {
				curPage, _ := pages.GetFrontPage()
				if curPage == "Guests" && vmList.HasFocus() {
					index := vmList.GetCurrentItem()
					if index >= 0 && index < len(originalVMs) {
						vm := originalVMs[index]
						a.HandleShellExecution(vm)
						return nil
					}
				} else if curPage == "Nodes" && nodeList.HasFocus() {
					index := nodeList.GetCurrentItem()
					if index >= 0 && index < len(originalNodes) {
						node := originalNodes[index]
						a.HandleShellExecution(node)
						return nil
					}
				}
				curPage, _ = pages.GetFrontPage()
				if curPage == "Guests" {
					index := vmList.GetCurrentItem()
					if index >= 0 && index < len(originalVMs) {
						vm := originalVMs[index]
						a.HandleShellExecution(vm)
						return nil
					}
				} else if curPage == "Nodes" {
					index := nodeList.GetCurrentItem()
					if index >= 0 && index < len(originalNodes) {
						node := originalNodes[index]
						a.HandleShellExecution(node)
						return nil
					}
				}
			} else if event.Rune() == 'q' {
				a.app.Stop()
				return nil
			} else if event.Rune() == '/' {
				a.handleSearchInput(a.app, pages, nodeList, vmList, originalNodes, originalVMs)
				return nil
			}

		}

		// Then handle special keys
		// Get the currently focused element
		focus := a.app.GetFocus()

		switch event.Key() {
		case tcell.KeyEscape:
			// If search input has focus, let it handle the Escape key
			if inputField, ok := focus.(*tview.InputField); ok && inputField.GetLabel() == "Search: " {
				return event
			}

			// Handle search mode
			if curPage, _ := pages.GetFrontPage(); curPage == "Search" {
				pages.RemovePage("Search")
				// Clear search state for the current page
				if basePage, _ := pages.GetFrontPage(); basePage != "" {
					if state, exists := models.GlobalState.SearchStates[basePage]; exists {
						state.SelectedIndex = 0
					}
				}

				// Get the underlying page after removing search
				basePage, _ := pages.GetFrontPage()
				if basePage == "Nodes" {
					nodeList.Clear()
					for _, node := range originalNodes {
						nodeList.AddItem(FormatNodeName(node), "", 0, nil)
					}
					// Restore node selection handlers and select first item
					a.updateNodeSelectionHandlers(nodeList, originalNodes)
					if len(originalNodes) > 0 {
						nodeList.SetCurrentItem(0)
					}
					a.app.SetFocus(nodeList)
				} else if basePage == "Guests" {
					BuildVMList(originalVMs, vmList)
					// Restore VM selection handlers and select first item
					a.updateVMSelectionHandlers(vmList, originalVMs, vmDetails)
					if len(originalVMs) > 0 {
						vmList.SetCurrentItem(0)
					}
					a.app.SetFocus(vmList)
				}
				return nil
			}
			// Otherwise, exit the application
			a.app.Stop()
			return nil
		case tcell.KeyCtrlC:
			a.app.Stop()
			return nil
		case tcell.KeyTab:
			// Cycle between pages
			curPage, _ := pages.GetFrontPage()
			if curPage == "Nodes" {
				pages.SwitchToPage("Guests")
				a.app.SetFocus(vmList)
			} else if curPage == "Guests" {
				pages.SwitchToPage("Nodes")
				a.app.SetFocus(nodeList)
			}
			return nil
		case tcell.KeyF1:
			pages.SwitchToPage("Nodes")
			a.app.SetFocus(nodeList)
			return nil
		case tcell.KeyF2:
			pages.SwitchToPage("Guests")
			a.app.SetFocus(vmList)
			return nil
		}
		return event
	})

	return pages
}

// CreatePagesContainer creates the pages container with tabs
func CreatePagesContainer() *tview.Pages {
	pages := tview.NewPages()
	pages.SetBorder(true)
	pages.SetTitle("Tabs")
	return pages
}

// AddNodesPage adds the nodes page to the pages container
func AddNodesPage(pages *tview.Pages, nodesContent *tview.Flex) {
	pages.AddPage("Nodes", nodesContent, true, true)
}

// AddGuestsPage adds the guests page to the pages container
func AddGuestsPage(pages *tview.Pages, vmList *tview.List, vmDetails *tview.Table) {
	guestsContent := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(vmList, 0, 1, true).
		AddItem(vmDetails, 0, 3, false)
	pages.AddPage("Guests", guestsContent, true, false)
}

// SetupVMHandlers configures VM list handlers
func SetupVMHandlers(vmList *tview.List, vmDetails *tview.Table, vms []*api.VM, client *api.Client) {
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
