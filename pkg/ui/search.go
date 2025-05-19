package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/models"
	"github.com/rivo/tview"
)

// handleSearchInput creates and manages the search input field
func (a *AppUI) handleSearchInput(app *tview.Application, pages *tview.Pages, nodeList *tview.List, vmList *tview.List, nodes []*api.Node, vms []*api.VM) {
	// Get current page context
	currentPage, _ := pages.GetFrontPage()
	
	// Initialize or update search state
	if _, exists := models.GlobalState.SearchStates[currentPage]; !exists {
		models.GlobalState.SearchStates[currentPage] = &models.SearchState{
			CurrentPage:   currentPage,
			SearchText:    "",
			SelectedIndex: 0,
		}
	}
	
	// Store original lists if not already stored
	if len(models.GlobalState.OriginalNodes) == 0 {
		models.GlobalState.OriginalNodes = make([]*api.Node, len(nodes))
		copy(models.GlobalState.OriginalNodes, nodes)
	}
	if len(models.GlobalState.OriginalVMs) == 0 {
		models.GlobalState.OriginalVMs = make([]*api.VM, len(vms))
		copy(models.GlobalState.OriginalVMs, vms)
	}
	
	// Initialize filtered lists if not already set
	if len(models.GlobalState.FilteredNodes) == 0 {
		models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
		copy(models.GlobalState.FilteredNodes, nodes)
	}
	if len(models.GlobalState.FilteredVMs) == 0 {
		models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
		copy(models.GlobalState.FilteredVMs, vms)
	}
	
	// Get current selection from the list and update global state
	if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
		if currentPage == "Nodes" {
			state.SelectedIndex = nodeList.GetCurrentItem()
		} else {
			state.SelectedIndex = vmList.GetCurrentItem()
		}
	}

	// Create input field with current search text if any
	searchText := models.GlobalState.SearchStates[currentPage].SearchText
	inputField := tview.NewInputField().
		SetLabel("Search: ").
		SetText(searchText)

	// Find the mainFlex (the root Flex layout)
	mainFlex, ok := a.Flex.GetItem(0).(*tview.Flex)
	if !ok {
		return // fallback: can't find main layout
	}

	// Add the input field to the bottom of the main layout
	mainFlex.AddItem(inputField, 1, 0, true)
	app.SetFocus(inputField)

	// Function to remove the search input field from the layout
	removeSearchInput := func() {
		// Remove the last item (input field) from mainFlex
		itemCount := mainFlex.GetItemCount()
		if itemCount > 0 {
			mainFlex.RemoveItem(inputField)
		}
	}

	// Function to update node selection and state
	updateNodeSelection := func(nodeList *tview.List, nodes []*api.Node) {
		nodeList.Clear()
		// Store the filtered nodes in global state
		models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
		copy(models.GlobalState.FilteredNodes, nodes)
		
		// Add nodes to the list
		for _, node := range nodes {
			nodeList.AddItem(FormatNodeName(node), "", 0, nil)
		}
		
		// Update selection handlers with the current node list
		a.updateNodeSelectionHandlers(nodeList, nodes)
		
		// Update selected index if needed
		if len(nodes) > 0 {
			idx := models.GlobalState.SearchStates[currentPage].SelectedIndex
			if idx < 0 || idx >= len(nodes) {
				idx = 0
			}
			nodeList.SetCurrentItem(idx)
			a.updateNodeDetails(nodes[idx])
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = idx
			}
		} else {
			a.nodeDetails.Clear()
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = 0
			}
		}
	}
	
	// Function to update VM selection and state
	updateVMSelection := func(vmList *tview.List, vms []*api.VM) {
		vmList.Clear()
		// Store the filtered VMs in global state
		models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
		copy(models.GlobalState.FilteredVMs, vms)
		
		// Use BuildVMList to ensure consistent VM list building
		BuildVMList(vms, vmList)
		
		// Update selection handlers with the current VM list
		a.updateVMSelectionHandlers(vmList, vms, a.vmDetails)
		
		// Update selected index if needed
		if len(vms) > 0 {
			idx := models.GlobalState.SearchStates[currentPage].SelectedIndex
			if idx < 0 || idx >= len(vms) {
				idx = 0
			}
			vmList.SetCurrentItem(idx)
			a.updateVMDetails(vms[idx])
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = idx
			}
		} else {
			a.vmDetails.Clear()
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = 0
			}
		}
	}

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			searchTerm := strings.TrimSpace(inputField.GetText())
			models.GlobalState.SearchStates[currentPage].SearchText = searchTerm
			
			// Store current selection index
			var currentIdx int
			if currentPage == "Nodes" {
				currentIdx = nodeList.GetCurrentItem()
			} else {
				currentIdx = vmList.GetCurrentItem()
			}
			models.GlobalState.SearchStates[currentPage].SelectedIndex = currentIdx
			
			// Update the UI with filtered results
			if currentPage == "Nodes" {
				var filteredNodes []*api.Node
				if searchTerm == "" {
					// Reset to original list if search is empty
					filteredNodes = make([]*api.Node, len(models.GlobalState.OriginalNodes))
					copy(filteredNodes, models.GlobalState.OriginalNodes)
				} else {
					// Filter nodes based on search term
					for _, node := range models.GlobalState.OriginalNodes {
						if node != nil && strings.Contains(strings.ToLower(node.Name), strings.ToLower(searchTerm)) {
							filteredNodes = append(filteredNodes, node)
						}
					}
				}
				updateNodeSelection(nodeList, filteredNodes)
			} else {
				// Handle VM search
				var filteredVMs []*api.VM
				if searchTerm == "" {
					// Reset to original list if search is empty
					filteredVMs = make([]*api.VM, len(models.GlobalState.OriginalVMs))
					copy(filteredVMs, models.GlobalState.OriginalVMs)
				} else {
					// Filter VMs based on search term
					for _, vm := range models.GlobalState.OriginalVMs {
						if vm != nil && strings.Contains(strings.ToLower(vm.Name), strings.ToLower(searchTerm)) {
							filteredVMs = append(filteredVMs, vm)
						}
					}
				}
				updateVMSelection(vmList, filteredVMs)
			}
			
			removeSearchInput()
			if currentPage == "Nodes" {
				app.SetFocus(nodeList)
			} else {
				app.SetFocus(vmList)
			}
		}
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			removeSearchInput()
			if currentPage == "Nodes" {
				app.SetFocus(nodeList)
			} else {
				app.SetFocus(vmList)
			}
			return nil
		}
		return event
	})

	// Set up real-time filtering as user types
	inputField.SetChangedFunc(func(text string) {
		searchTerm := strings.TrimSpace(strings.ToLower(text))
		
		if currentPage == "Nodes" {
			// Filter nodes
			var filteredNodes []*api.Node
			if searchTerm == "" {
				// If search is empty, show all nodes
				filteredNodes = make([]*api.Node, len(models.GlobalState.OriginalNodes))
				copy(filteredNodes, models.GlobalState.OriginalNodes)
			} else {
				// Otherwise filter nodes based on search term
				for _, node := range models.GlobalState.OriginalNodes {
					if node != nil && strings.Contains(strings.ToLower(node.Name), searchTerm) {
						filteredNodes = append(filteredNodes, node)
					}
				}
			}
			
			// Update the node list and selection handlers
			updateNodeSelection(nodeList, filteredNodes)
			
			// Update the details for the first item if available
			if len(filteredNodes) > 0 {
				currentIndex := nodeList.GetCurrentItem()
				if currentIndex < 0 || currentIndex >= len(filteredNodes) {
					currentIndex = 0
					nodeList.SetCurrentItem(0)
				}
				a.updateNodeDetails(filteredNodes[currentIndex])
			} else {
				a.nodeDetails.Clear()
			}
		} else {
			// Handle VM search
			var filteredVMs []*api.VM
			if searchTerm == "" {
				// If search is empty, show all VMs
				filteredVMs = make([]*api.VM, len(models.GlobalState.OriginalVMs))
				copy(filteredVMs, models.GlobalState.OriginalVMs)
			} else {
				// Otherwise filter VMs based on search term
				for _, vm := range models.GlobalState.OriginalVMs {
					if vm != nil && strings.Contains(strings.ToLower(vm.Name), searchTerm) {
						filteredVMs = append(filteredVMs, vm)
					}
				}
			}
			
			// Update the VM list and selection handlers
			updateVMSelection(vmList, filteredVMs)
			
			// Update the details for the first item if available
			if len(filteredVMs) > 0 {
				currentIndex := vmList.GetCurrentItem()
				if currentIndex < 0 || currentIndex >= len(filteredVMs) {
					currentIndex = 0
					vmList.SetCurrentItem(0)
				}
				a.updateVMDetails(filteredVMs[currentIndex])
			} else {
				a.vmDetails.Clear()
			}
		}
	})
}
