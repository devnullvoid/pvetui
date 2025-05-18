package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/models"
	"github.com/rivo/tview"
)

// handleSearchInput creates and manages the search input field
func handleSearchInput(app *tview.Application, pages *tview.Pages, nodeList *tview.List, vmList *tview.List, nodes []api.Node, vms []api.VM) {
	// Store original lists and current page context
	originalNodes := make([]api.Node, len(nodes))
	copy(originalNodes, nodes)
	originalVMs := make([]api.VM, len(vms))
	copy(originalVMs, vms)
	currentPage, _ := pages.GetFrontPage() // Store current active page

	// Create fresh input field each time
	var inputField *tview.InputField
	inputField = tview.NewInputField().
		SetLabel("Search: ").
		SetText(models.GlobalState.LastSearchText).
		SetDoneFunc(func(key tcell.Key) {
			pages.RemovePage("Search")
			// Save search text and keep filtered results
			models.GlobalState.LastSearchText = inputField.GetText() // Now properly references the inputField
			if currentPage == "Nodes" {
				app.SetFocus(nodeList)
			} else {
				app.SetFocus(vmList)
			}
		})

	// Configure input field after declaration
	inputField.
		SetChangedFunc(func(text string) {
			searchTerm := strings.ToLower(text)
			// Use stored current page context
			// Get current scroll position
			currentNodeIndex := nodeList.GetCurrentItem()
			currentVMIndex := vmList.GetCurrentItem()

			if currentPage == "Nodes" {
				// Filter nodes
				nodeList.Clear()
				for _, node := range originalNodes {
					if strings.Contains(strings.ToLower(node.Name), searchTerm) {
						nodeList.AddItem(FormatNodeName(node), "", 0, nil)
					}
				}
				// Restore scroll position if possible
				if currentNodeIndex < nodeList.GetItemCount() {
					nodeList.SetCurrentItem(currentNodeIndex)
				}
			} else if currentPage == "Guests" {
				// Filter VMs

				var filteredVMs []api.VM
				for _, vm := range originalVMs {
					if strings.Contains(strings.ToLower(vm.Name), searchTerm) {
						filteredVMs = append(filteredVMs, vm)
					}
				}
				BuildVMList(filteredVMs, vmList)
				// Restore scroll position with bounds checking
				newItemCount := vmList.GetItemCount()
				if newItemCount > 0 {
					if currentVMIndex >= newItemCount {
						currentVMIndex = newItemCount - 1
					}
					vmList.SetCurrentItem(currentVMIndex)
				}
			}
		}).
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				// Clear the search text and exit search mode
				inputField.SetText("")
				pages.RemovePage("Search")
				models.GlobalState.LastSearchText = "" // Clear persisted search text
				if currentPage == "Nodes" {
					app.SetFocus(nodeList)
				} else {
					app.SetFocus(vmList)
				}
				return nil
			}
			return event
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
