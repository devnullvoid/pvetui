package components

import (
	"fmt"
	"strings"

	// "github.com/devnullvoid/proxmox-tui/pkg/api"
	// "github.com/devnullvoid/proxmox-tui/pkg/config"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// activateSearch shows the search input field and sets up filtering
func (a *App) activateSearch() {
	// Get current page context
	currentPage, _ := a.pages.GetFrontPage()

	// Initialize or update search state
	if _, exists := models.GlobalState.SearchStates[currentPage]; !exists {
		models.GlobalState.SearchStates[currentPage] = &models.SearchState{
			CurrentPage:   currentPage,
			SearchText:    "",
			SelectedIndex: 0,
		}
	}

	// Create input field with current search text if any
	searchText := ""
	if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
		searchText = state.SearchText
	}

	// Create search input field if it doesn't exist
	if a.searchInput == nil {
		a.searchInput = tview.NewInputField().
			SetLabel("Search: ").
			SetFieldWidth(0).
			SetPlaceholder("Filter active list... press Enter/Esc to return to list")
	}

	// Set current search text
	a.searchInput.SetText(searchText)

	// Add the search input field to the bottom of the layout
	if a.mainLayout.GetItemCount() == 4 { // Already has header, cluster status, pages, footer
		a.mainLayout.AddItem(a.searchInput, 1, 0, true)
		a.SetFocus(a.searchInput)
	}

	// Function to remove search input
	removeSearchInput := func() {
		if a.mainLayout.GetItemCount() > 4 {
			a.mainLayout.RemoveItem(a.searchInput)
		}
		if currentPage == "Nodes" {
			a.SetFocus(a.nodeList)
		} else {
			a.SetFocus(a.vmList)
		}
	}

	// Function to update node selection with filtered results
	updateNodeSelection := func(nodes []*api.Node) {
		// Store the filtered nodes in global state
		models.GlobalState.FilteredNodes = make([]*api.Node, len(nodes))
		copy(models.GlobalState.FilteredNodes, nodes)

		// Update node list
		a.nodeList.SetNodes(nodes)

		// Update selected index if needed
		if len(nodes) > 0 {
			idx := 0
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				idx = state.SelectedIndex
				if idx < 0 || idx >= len(nodes) {
					idx = 0
				}
				state.SelectedIndex = idx
			}
			a.nodeList.List.SetCurrentItem(idx)
			a.nodeDetails.Update(nodes[idx], a.client.Cluster.Nodes)
		} else {
			a.nodeDetails.Clear()
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = 0
			}
		}
	}

	// Function to update VM selection with filtered results
	updateVMSelection := func(vms []*api.VM) {
		// Store the filtered VMs in global state
		models.GlobalState.FilteredVMs = make([]*api.VM, len(vms))
		copy(models.GlobalState.FilteredVMs, vms)

		// Update VM list
		a.vmList.SetVMs(vms)

		// Update selected index if needed
		if len(vms) > 0 {
			idx := 0
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				idx = state.SelectedIndex
				if idx < 0 || idx >= len(vms) {
					idx = 0
				}
				state.SelectedIndex = idx
			}
			a.vmList.List.SetCurrentItem(idx)
			a.vmDetails.Update(vms[idx])
		} else {
			a.vmDetails.Clear()
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = 0
			}
		}
	}

	// Handle search text changes
	a.searchInput.SetChangedFunc(func(text string) {
		searchTerm := strings.TrimSpace(strings.ToLower(text))

		if currentPage == "Nodes" {
			// Filter nodes based on search term
			var filteredNodes []*api.Node
			if searchTerm == "" {
				// Show all nodes if search is empty
				filteredNodes = make([]*api.Node, len(models.GlobalState.OriginalNodes))
				copy(filteredNodes, models.GlobalState.OriginalNodes)
			} else {
				// Filter nodes that match search term
				for _, node := range models.GlobalState.OriginalNodes {
					if node != nil && strings.Contains(strings.ToLower(node.Name), searchTerm) {
						filteredNodes = append(filteredNodes, node)
					}
				}
			}
			updateNodeSelection(filteredNodes)
		} else {
			// Filter VMs based on search term
			var filteredVMs []*api.VM
			if searchTerm == "" {
				// Show all VMs if search is empty
				filteredVMs = make([]*api.VM, len(models.GlobalState.OriginalVMs))
				copy(filteredVMs, models.GlobalState.OriginalVMs)
			} else {
				// Filter VMs that match search term by name, ID, node, or type
				for _, vm := range models.GlobalState.OriginalVMs {
					if vm != nil {
						// Convert VM ID to string for matching
						vmIDStr := fmt.Sprintf("%d", vm.ID)

						// Match if name, ID, node name, or VM type contains search term
						if strings.Contains(strings.ToLower(vm.Name), searchTerm) ||
							strings.Contains(vmIDStr, searchTerm) ||
							strings.Contains(strings.ToLower(vm.Node), searchTerm) ||
							strings.Contains(strings.ToLower(vm.Type), searchTerm) {
							filteredVMs = append(filteredVMs, vm)
						}
					}
				}
			}
			updateVMSelection(filteredVMs)
		}

		// Save search text in state
		if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
			state.SearchText = text
		}
	})

	// Handle Enter/Escape/Tab keys in search input
	a.searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			removeSearchInput()
			return nil
		case tcell.KeyEnter:
			removeSearchInput()
			return nil
		case tcell.KeyTab:
			// Prevent Tab from propagating when search is active
			return nil
		}

		// Handle 'q' key to prevent app from quitting during search
		if event.Key() == tcell.KeyRune && event.Rune() == 'q' {
			// Just handle it as a normal key for the input field
			return event
		}

		return event
	})
}
