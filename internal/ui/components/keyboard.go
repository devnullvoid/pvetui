package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/keys"
	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// keyMatch checks if an event matches a key specification string.
func keyMatch(ev *tcell.EventKey, spec string) bool {
	key, r, mod, err := keys.Parse(spec)
	if err != nil {
		if config.DebugEnabled {
			models.GetUILogger().Debug("invalid key spec %s: %v", spec, err)
		}
		return false
	}
	evKey, evRune, evMod := keys.NormalizeEvent(ev)

	// For rune keys, we want to match if the event has at least the modifiers
	// from the spec. This allows "a" to match "Shift+a". For non-rune keys
	// we require an exact modifier match.
	if key == tcell.KeyRune {
		if evMod&mod != mod {
			if config.DebugEnabled {
				models.GetUILogger().Debug("mod mismatch (rune) spec=%s want=%d got=%d", spec, mod, evMod)
			}
			return false
		}
	} else {
		if evMod != mod {
			if config.DebugEnabled {
				models.GetUILogger().Debug("mod mismatch (non-rune) spec=%s want=%d got=%d", spec, mod, evMod)
			}
			return false
		}
	}

	if key == tcell.KeyRune {
		match := evKey == tcell.KeyRune && r != 0 && strings.EqualFold(string(evRune), string(r))
		if config.DebugEnabled {
			models.GetUILogger().Debug("keyMatch spec=%s rune=%q event=%q mod=%d match=%t", spec, r, evRune, mod, match)
		}
		return match
	}
	match := evKey == key
	if config.DebugEnabled {
		models.GetUILogger().Debug("keyMatch spec=%s key=%d event=%d mod=%d match=%t", spec, key, evKey, mod, match)
	}
	return match
}

// setupKeyboardHandlers configures global keyboard shortcuts
func (a *App) setupKeyboardHandlers() {
	a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if config.DebugEnabled {
			key, r, mod := keys.NormalizeEvent(event)
			models.GetUILogger().Debug("input key=%d rune=%q mod=%d", key, r, mod)
		}
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

		// Handle configured switch view shortcut
		if keyMatch(event, a.config.KeyBindings.SwitchView) {
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
		}

		if keyMatch(event, a.config.KeyBindings.NodesPage) {
			a.pages.SwitchToPage(api.PageNodes)
			a.SetFocus(a.nodeList)
			return nil
		}
		if keyMatch(event, a.config.KeyBindings.GuestsPage) {
			a.pages.SwitchToPage(api.PageGuests)
			a.SetFocus(a.vmList)
			return nil
		}
		if keyMatch(event, a.config.KeyBindings.TasksPage) {
			a.pages.SwitchToPage(api.PageTasks)
			a.SetFocus(a.tasksList)
			return nil
		}
		if keyMatch(event, a.config.KeyBindings.Refresh) {
			a.manualRefresh()
			return nil
		}
		if event.Key() == tcell.KeyRune {
			if keyMatch(event, a.config.KeyBindings.Quit) {
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
			} else if keyMatch(event, a.config.KeyBindings.Search) {
				// Activate search
				a.activateSearch()
				return nil
			} else if keyMatch(event, a.config.KeyBindings.Shell) {
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
			} else if keyMatch(event, a.config.KeyBindings.Menu) {
				// Open context menu based on current page
				currentPage, _ := a.pages.GetFrontPage()
				if currentPage == api.PageNodes {
					a.ShowNodeContextMenu()
				} else if currentPage == api.PageGuests {
					a.ShowVMContextMenu()
				}
				return nil
			} else if keyMatch(event, a.config.KeyBindings.Scripts) {
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
			} else if keyMatch(event, a.config.KeyBindings.AutoRefresh) {
				// Toggle auto-refresh
				a.toggleAutoRefresh()
				return nil
			} else if keyMatch(event, a.config.KeyBindings.VNC) {
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
			} else if keyMatch(event, a.config.KeyBindings.Help) {
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
