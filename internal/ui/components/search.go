package components

import (
	"strings"

	// "github.com/devnullvoid/proxmox-tui/pkg/api"
	// "github.com/devnullvoid/proxmox-tui/pkg/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
)

// activateSearch shows the search input field and sets up filtering
func (a *App) activateSearch() {
	// Get current page context
	currentPage, _ := a.pages.GetFrontPage()

	// Initialize or update search state
	if _, exists := models.GlobalState.SearchStates[currentPage]; !exists {
		models.GlobalState.SearchStates[currentPage] = &models.SearchState{
			CurrentPage:   currentPage,
			Filter:        "",
			SelectedIndex: 0,
		}
	}

	// Create input field with current filter text if any
	filterText := ""
	if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
		filterText = state.Filter
	}

	// Create search input field if it doesn't exist
	if a.searchInput == nil {
		a.searchInput = tview.NewInputField().
			SetLabel("Search: ").
			SetFieldWidth(0).
			SetPlaceholder("Filter active list... press Enter/Esc to return to list")
	}

	// Set current filter text
	a.searchInput.SetText(filterText)

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
	updateNodeSelection := func() {
		// Update node list with filtered nodes
		a.nodeList.SetNodes(models.GlobalState.FilteredNodes)

		// Update selected index if needed
		if len(models.GlobalState.FilteredNodes) > 0 {
			idx := 0
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				idx = state.SelectedIndex
				if idx < 0 || idx >= len(models.GlobalState.FilteredNodes) {
					idx = 0
				}
				state.SelectedIndex = idx
			}
			a.nodeList.List.SetCurrentItem(idx)
			a.nodeDetails.Update(models.GlobalState.FilteredNodes[idx], a.client.Cluster.Nodes)
		} else {
			a.nodeDetails.Clear()
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = 0
			}
		}
	}

	// Function to update VM selection with filtered results
	updateVMSelection := func() {
		// Update VM list with filtered VMs
		a.vmList.SetVMs(models.GlobalState.FilteredVMs)

		// Update selected index if needed
		if len(models.GlobalState.FilteredVMs) > 0 {
			idx := 0
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				idx = state.SelectedIndex
				if idx < 0 || idx >= len(models.GlobalState.FilteredVMs) {
					idx = 0
				}
				state.SelectedIndex = idx
			}
			a.vmList.List.SetCurrentItem(idx)
			a.vmDetails.Update(models.GlobalState.FilteredVMs[idx])
		} else {
			a.vmDetails.Clear()
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = 0
			}
		}
	}

	// Handle search text changes
	a.searchInput.SetChangedFunc(func(text string) {
		filterTerm := strings.TrimSpace(text)

		// Save filter text in state
		if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
			state.Filter = filterTerm
		}

		if currentPage == "Nodes" {
			// Use our common filter function for nodes
			models.FilterNodes(filterTerm)
			updateNodeSelection()
		} else {
			// Use our common filter function for VMs
			models.FilterVMs(filterTerm)
			updateVMSelection()
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
