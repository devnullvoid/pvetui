package components

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/devnullvoid/pvetui/pkg/api"
	// "github.com/devnullvoid/pvetui/pkg/config".

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/models"
)

// activateSearch shows the search input field and sets up filtering.
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

	// Add the search input field above the footer
	if a.mainLayout.GetItemCount() == 4 { // Already has header, cluster status, pages, footer
		// Remove footer temporarily, add search input, then add footer back
		a.mainLayout.RemoveItem(a.footer)
		a.mainLayout.AddItem(a.searchInput, 1, 0, true)
		a.mainLayout.AddItem(a.footer, 1, 0, false)
		a.SetFocus(a.searchInput)
	}

	// Function to remove search input
	removeSearchInput := func() {
		if a.mainLayout.GetItemCount() > 4 {
			// Remove search input and reorder: remove footer, remove search, add footer back
			a.mainLayout.RemoveItem(a.footer)
			a.mainLayout.RemoveItem(a.searchInput)
			a.mainLayout.AddItem(a.footer, 1, 0, false)
		}

		if currentPage == api.PageNodes {
			a.SetFocus(a.nodeList)
		} else if currentPage == api.PageTasks {
			a.SetFocus(a.tasksList)
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

			a.nodeList.SetCurrentItem(idx)
			// Manually trigger the node changed callback to update details
			if selectedNode := a.nodeList.GetSelectedNode(); selectedNode != nil {
				a.nodeDetails.Update(selectedNode, models.GlobalState.OriginalNodes)
			}
		} else {
			a.nodeDetails.Clear()

			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				state.SelectedIndex = 0
			}
		}
	}

	// Function to update VM selection with filtered results
	updateVMSelection := func() {
		a.refreshVMSelection(currentPage)
	}

	// Function to update tasks selection with filtered results
	updateTaskSelection := func() {
		// Update tasks list with filtered tasks
		a.tasksList.SetFilteredTasks(models.GlobalState.FilteredTasks)

		// Update selected index if needed
		if len(models.GlobalState.FilteredTasks) > 0 {
			idx := 0
			if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
				idx = state.SelectedIndex
				if idx < 0 || idx >= len(models.GlobalState.FilteredTasks) {
					idx = 0
				}

				state.SelectedIndex = idx
			}

			a.tasksList.Select(idx+1, 0) // +1 because row 0 is header
		} else {
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

		if currentPage == api.PageNodes {
			// Use our common filter function for nodes
			models.FilterNodes(filterTerm)
			updateNodeSelection()
		} else if currentPage == api.PageTasks {
			// Use our common filter function for tasks
			models.FilterTasks(filterTerm)
			updateTaskSelection()
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
			// Per user request, Escape should clear the search filter
			a.searchInput.SetText("")
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

func (a *App) refreshVMSelection(currentPage string) {
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

		a.vmList.SetCurrentItem(idx)
		// Manually trigger the VM changed callback to update details
		if selectedVM := a.vmList.GetSelectedVM(); selectedVM != nil {
			a.vmDetails.Update(selectedVM)
		}
	} else {
		a.vmDetails.Clear()

		if state, exists := models.GlobalState.SearchStates[currentPage]; exists {
			state.SelectedIndex = 0
		}
	}
}

func uniqueSortedVMStatuses(vms []*api.VM) []string {
	set := map[string]struct{}{}
	for _, vm := range vms {
		if vm == nil {
			continue
		}
		status := strings.TrimSpace(vm.Status)
		if status == "" {
			continue
		}
		set[status] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for status := range set {
		out = append(out, status)
	}
	sort.Strings(out)

	return out
}

func uniqueSortedVMNodes(vms []*api.VM) []string {
	set := map[string]struct{}{}
	for _, vm := range vms {
		if vm == nil {
			continue
		}
		node := strings.TrimSpace(vm.Node)
		if node == "" {
			continue
		}
		set[node] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for node := range set {
		out = append(out, node)
	}
	sort.Strings(out)

	return out
}

func indexOfOption(options []string, target string) int {
	for i, option := range options {
		if strings.EqualFold(option, target) {
			return i
		}
	}
	return 0
}

// showAdvancedGuestFilterModal displays structured filters for the Guests page.
func (a *App) showAdvancedGuestFilterModal() {
	currentPage, _ := a.pages.GetFrontPage()
	if currentPage != api.PageGuests {
		return
	}

	state, exists := models.GlobalState.SearchStates[api.PageGuests]
	if !exists {
		state = &models.SearchState{CurrentPage: api.PageGuests}
		models.GlobalState.SearchStates[api.PageGuests] = state
	}

	statusOptions := append([]string{"Any"}, uniqueSortedVMStatuses(models.GlobalState.OriginalVMs)...)
	typeOptions := []string{"Any", api.VMTypeQemu, api.VMTypeLXC}
	nodeOptions := append([]string{"Any"}, uniqueSortedVMNodes(models.GlobalState.OriginalVMs)...)

	form := newStandardForm()
	form.SetBorder(true)
	form.SetTitle(" Advanced Guest Filter ")

	form.AddInputField("Query", state.Filter, 0, nil, nil)
	form.AddDropDown("Status", statusOptions, indexOfOption(statusOptions, state.VMFilters.Status), nil)
	form.AddDropDown("Type", typeOptions, indexOfOption(typeOptions, state.VMFilters.Type), nil)
	form.AddDropDown("Node", nodeOptions, indexOfOption(nodeOptions, state.VMFilters.Node), nil)
	form.AddInputField("Tag Contains", state.VMFilters.TagContains, 0, nil, nil)

	form.AddButton("Apply", func() {
		query := strings.TrimSpace(form.GetFormItemByLabel("Query").(*tview.InputField).GetText())
		state.Filter = query

		_, selectedStatus := form.GetFormItemByLabel("Status").(*tview.DropDown).GetCurrentOption()
		if strings.EqualFold(selectedStatus, "Any") {
			selectedStatus = ""
		}

		_, selectedType := form.GetFormItemByLabel("Type").(*tview.DropDown).GetCurrentOption()
		if strings.EqualFold(selectedType, "Any") {
			selectedType = ""
		}

		_, selectedNode := form.GetFormItemByLabel("Node").(*tview.DropDown).GetCurrentOption()
		if strings.EqualFold(selectedNode, "Any") {
			selectedNode = ""
		}

		tagContains := strings.TrimSpace(form.GetFormItemByLabel("Tag Contains").(*tview.InputField).GetText())

		state.VMFilters = models.VMFilterOptions{
			Status:      selectedStatus,
			Type:        selectedType,
			Node:        selectedNode,
			TagContains: tagContains,
		}
		state.SelectedIndex = 0

		models.FilterVMs(state.Filter)
		a.refreshVMSelection(api.PageGuests)
		a.removePageIfPresent("advancedGuestFilter")

		activeCriteria := 0
		if strings.TrimSpace(state.Filter) != "" {
			activeCriteria++
		}
		if strings.TrimSpace(state.VMFilters.Status) != "" {
			activeCriteria++
		}
		if strings.TrimSpace(state.VMFilters.Type) != "" {
			activeCriteria++
		}
		if strings.TrimSpace(state.VMFilters.Node) != "" {
			activeCriteria++
		}
		if strings.TrimSpace(state.VMFilters.TagContains) != "" {
			activeCriteria++
		}

		if activeCriteria == 0 {
			a.header.ShowSuccess("Guest filters cleared")
		} else {
			a.header.ShowSuccess(fmt.Sprintf("Applied guest filters (%d active)", activeCriteria))
		}
	})

	form.AddButton("Clear", func() {
		state.Filter = ""
		state.VMFilters = models.VMFilterOptions{}
		state.SelectedIndex = 0

		models.FilterVMs("")
		a.refreshVMSelection(api.PageGuests)
		a.removePageIfPresent("advancedGuestFilter")
		a.header.ShowSuccess("Guest filters cleared")
	})

	form.AddButton("Cancel", func() {
		a.removePageIfPresent("advancedGuestFilter")
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.removePageIfPresent("advancedGuestFilter")
			return nil
		}

		return event
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 14, 0, true).
			AddItem(nil, 0, 1, false), 72, 1, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("advancedGuestFilter", modal, true, true)
	a.SetFocus(form)
}

// restoreSearchUI restores the search input UI state if it was active before a refresh.
func (a *App) restoreSearchUI(searchWasActive bool, nodeSearchState, vmSearchState *models.SearchState) {
	if !searchWasActive {
		return
	}
	// Give a small delay to ensure all UI updates are complete
	go func() {
		time.Sleep(50 * time.Millisecond)
		a.QueueUpdateDraw(func() {
			// Check if search input is still in layout but focus was lost
			if a.mainLayout.GetItemCount() > 4 && a.searchInput != nil {
				// Restore focus to search input
				a.SetFocus(a.searchInput)
			} else if a.searchInput != nil {
				// Search input was removed, re-add it if there's an active filter
				currentPage, _ := a.pages.GetFrontPage()

				var hasActiveFilter bool

				if currentPage == api.PageNodes && nodeSearchState != nil && nodeSearchState.Filter != "" {
					hasActiveFilter = true
				} else if currentPage == api.PageGuests && vmSearchState != nil && vmSearchState.HasActiveVMFilter() {
					hasActiveFilter = true
				} else if currentPage == api.PageTasks {
					if taskSearchState := models.GlobalState.GetSearchState(api.PageTasks); taskSearchState != nil && taskSearchState.Filter != "" {
						hasActiveFilter = true
					}
				}

				if hasActiveFilter {
					// Re-add search input above footer and restore focus
					a.mainLayout.RemoveItem(a.footer)
					a.mainLayout.AddItem(a.searchInput, 1, 0, true)
					a.mainLayout.AddItem(a.footer, 1, 0, false)
					a.SetFocus(a.searchInput)
				}
			}
		})
	}()
}
