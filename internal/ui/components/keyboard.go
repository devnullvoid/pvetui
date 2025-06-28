package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
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
			a.pages.HasPage("confirmation") ||
			a.pages.HasPage("migration") ||
			a.pages.HasPage("help")

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
			switch currentPage {
			case api.PageNodes:
				a.pages.SwitchToPage(api.PageGuests)
				a.SetFocus(a.vmList)
			case api.PageGuests:
				a.pages.SwitchToPage(api.PageTasks)
				a.SetFocus(a.tasksList)
			default:
				a.pages.SwitchToPage(api.PageNodes)
				a.SetFocus(a.nodeList)
			}
			return nil
		case tcell.KeyF1:
			a.pages.SwitchToPage(api.PageNodes)
			a.SetFocus(a.nodeList)
			return nil
		case tcell.KeyF2:
			a.pages.SwitchToPage(api.PageGuests)
			a.SetFocus(a.vmList)
			return nil
		case tcell.KeyF3:
			a.pages.SwitchToPage(api.PageTasks)
			a.SetFocus(a.tasksList)
			return nil
		case tcell.KeyF5:
			// Manual refresh
			a.manualRefresh()
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				// Check if there are active VNC sessions
				sessionCount := a.vncService.GetActiveSessionCount()
				if sessionCount > 0 {
					// Show confirmation dialog with session count
					var message string
					if sessionCount == 1 {
						message = "There is 1 active VNC session that will be disconnected.\n\nAre you sure you want to quit?"
					} else {
						message = fmt.Sprintf("There are %d active VNC sessions that will be disconnected.\n\nAre you sure you want to quit?", sessionCount)
					}

					a.showConfirmationDialog(message, func() {
						a.Stop()
					})
				} else {
					// No active sessions, quit immediately
					a.Stop()
				}
				return nil
			} else if event.Rune() == '/' {
				// Activate search
				a.activateSearch()
				return nil
			} else if event.Rune() == 's' || event.Rune() == 'S' {
				// Open shell session based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == api.PageNodes {
					// Handle node shell session
					a.openNodeShell()
				} else if currentPage == api.PageGuests {
					// Handle VM shell session
					a.openVMShell()
				}
				return nil
			} else if event.Rune() == 'm' {
				// Open context menu based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == api.PageNodes {
					a.ShowNodeContextMenu()
				} else if currentPage == api.PageGuests {
					a.ShowVMContextMenu()
				}
				return nil
			} else if event.Rune() == 'c' || event.Rune() == 'C' {
				// Open community scripts installer - only available for nodes
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == api.PageNodes {
					node := a.nodeList.GetSelectedNode()
					if node != nil {
						a.openScriptSelector(node, nil)
					}
				} else if currentPage == api.PageGuests {
					// Community scripts are not available for individual VMs
					a.showMessage("Community scripts can only be installed on nodes. Switch to the Nodes tab to install scripts.")
				}
				return nil
			} else if event.Rune() == 'r' || event.Rune() == 'R' {
				// Manual refresh
				a.manualRefresh()
				return nil
			} else if event.Rune() == 'a' || event.Rune() == 'A' {
				// Toggle auto-refresh
				a.toggleAutoRefresh()
				return nil
			} else if event.Rune() == 'v' || event.Rune() == 'V' {
				// Open VNC connection based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == api.PageNodes {
					// Handle node VNC shell session
					a.openNodeVNC()
				} else if currentPage == api.PageGuests {
					// Handle VM VNC console session
					a.openVMVNC()
				}
				return nil
			} else if event.Rune() == '?' {
				// Toggle help modal
				if a.pages.HasPage("help") {
					a.helpModal.Hide()
				} else {
					a.helpModal.Show()
				}
				return nil
			}
		}
		return event
	})
}
