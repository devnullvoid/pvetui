package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// setupKeyboardHandlers configures global keyboard shortcuts
func (a *App) setupKeyboardHandlers() {
	a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check if search is active by seeing if the search input is in the main layout
		searchActive := a.mainLayout.GetItemCount() > 4

		// Check if any modal page is active
		pageName, _ := a.pages.GetFrontPage()
		modalActive := strings.HasPrefix(pageName, "script") ||
			a.pages.HasPage("scriptInfo") ||
			a.pages.HasPage("scriptSelector") ||
			a.pages.HasPage("message") ||
			a.pages.HasPage("confirmation")

		// If search is active, let the search input handle the keys
		if searchActive {
			// Let the search input handle all keys when search is active
			return event
		}

		// If a modal dialog is active, let it handle its own keys
		if modalActive {
			return event
		}

		// If context menu is open, let it handle keys
		if a.isMenuOpen && a.contextMenu != nil {
			return event
		}

		// Handle tab for page switching when search is not active
		switch event.Key() {
		case tcell.KeyTab:
			currentPage, _ := a.pages.GetFrontPage()
			if currentPage == "Nodes" {
				a.pages.SwitchToPage("Guests")
				a.SetFocus(a.vmList)
			} else {
				a.pages.SwitchToPage("Nodes")
				a.SetFocus(a.nodeList)
			}
			return nil
		case tcell.KeyF1:
			a.pages.SwitchToPage("Nodes")
			a.SetFocus(a.nodeList)
			return nil
		case tcell.KeyF2:
			a.pages.SwitchToPage("Guests")
			a.SetFocus(a.vmList)
			return nil
		case tcell.KeyF5:
			// Manual refresh
			a.manualRefresh()
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				a.Stop()
				return nil
			} else if event.Rune() == '/' {
				// Activate search
				a.activateSearch()
				return nil
			} else if event.Rune() == 's' || event.Rune() == 'S' {
				// Open shell session based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == "Nodes" {
					// Handle node shell session
					a.openNodeShell()
				} else if currentPage == "Guests" {
					// Handle VM shell session
					a.openVMShell()
				}
				return nil
			} else if event.Rune() == 'm' {
				// Open context menu based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == "Nodes" {
					a.ShowNodeContextMenu()
				} else if currentPage == "Guests" {
					a.ShowVMContextMenu()
				}
				return nil
			} else if event.Rune() == 'c' || event.Rune() == 'C' {
				// Open community scripts installer - only available for nodes
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == "Nodes" {
					node := a.nodeList.GetSelectedNode()
					if node != nil {
						a.openScriptSelector(node, nil)
					}
				} else if currentPage == "Guests" {
					// Community scripts are not available for individual VMs
					a.showMessage("Community scripts can only be installed on nodes. Switch to the Nodes tab to install scripts.")
				}
				return nil
			} else if event.Rune() == 'r' || event.Rune() == 'R' {
				// Manual refresh
				a.manualRefresh()
				return nil
			}
		}
		return event
	})
}
