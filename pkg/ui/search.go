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
	
	// Initialize filtered lists
	models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
	copy(models.GlobalState.FilteredNodes, nodes)
	models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
	copy(models.GlobalState.FilteredVMs, vms)
	
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
		
	// Set focus to input field immediately
	app.SetFocus(inputField)
	
	// Input field is already focused and text is set, no need for cursor positioning

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

	// Set up DoneFunc to handle Enter key
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
						if node != nil && strings.Contains(strings.ToLower(node.Name), searchTerm) {
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
						if vm != nil && strings.Contains(strings.ToLower(vm.Name), searchTerm) {
							filteredVMs = append(filteredVMs, vm)
						}
					}
				}
				updateVMSelection(vmList, filteredVMs)
			}
			
			// Close the modal
			pages.RemovePage("Search")
			
			// Set focus back to the appropriate list
			if currentPage == "Nodes" {
				app.SetFocus(nodeList)
			} else {
				app.SetFocus(vmList)
			}
		}
	})

	// Handle Escape key in the main input capture
	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Clear the search text and reset to original lists
			models.GlobalState.SearchStates[currentPage].SearchText = ""
			
			// Reset to original lists
			if currentPage == "Nodes" {
				// Reset to original nodes
				filteredNodes := make([]*api.Node, len(models.GlobalState.OriginalNodes))
				copy(filteredNodes, models.GlobalState.OriginalNodes)
				updateNodeSelection(nodeList, filteredNodes)
			} else {
				// Reset to original VMs
				filteredVMs := make([]*api.VM, len(models.GlobalState.OriginalVMs))
				copy(filteredVMs, models.GlobalState.OriginalVMs)
				updateVMSelection(vmList, filteredVMs)
			}
			
			// Close the search modal
			pages.RemovePage("Search")
			
			// Set focus back to the appropriate list
			if currentPage == "Nodes" {
				app.SetFocus(nodeList)
			} else {
				app.SetFocus(vmList)
			}
			return nil
		}
		return event
	})

	// Configure input field after declaration
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
			
		} else if currentPage == "Guests" {
			// Filter VMs
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

	// Create search bar as centered modal
	inputField.SetTitle(" Search ").
		SetBorder(true).
		SetBackgroundColor(tcell.ColorDefault)

	// Create flex layout to center the search bar
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(inputField, 3, 1, true).
			AddItem(nil, 0, 1, false),
			40, 1, true).
		AddItem(nil, 0, 1, false)

	// Add as overlay page instead of replacing root
	pages.AddPage("Search", modal, true, true)
	app.SetFocus(inputField)
}
