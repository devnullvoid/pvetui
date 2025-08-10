package components

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/devnullvoid/proxmox-tui/internal/scripts"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
)

// formatScriptInfo formats the script information for display.
func (s *ScriptSelector) formatScriptInfo(script scripts.Script) string {
	var sb strings.Builder

	labelColor := theme.ColorToTag(theme.Colors.Warning)
	sb.WriteString(fmt.Sprintf("[%s]Name:[-] %s\n\n", labelColor, script.Name))
	sb.WriteString(fmt.Sprintf("[%s]Description:[-] %s\n\n", labelColor, script.Description))

	if script.Type == scriptTypeCT {
		sb.WriteString(fmt.Sprintf("[%s]Type:[-] Container Template\n", labelColor))
	} else if script.Type == scriptTypeVM {
		sb.WriteString(fmt.Sprintf("[%s]Type:[-] Virtual Machine\n", labelColor))
	} else {
		sb.WriteString(fmt.Sprintf("[%s]Type:[-] %s\n", labelColor, script.Type))
	}

	if script.ScriptPath != "" {
		sb.WriteString(fmt.Sprintf("[%s]Script Path:[-] %s\n", labelColor, script.ScriptPath))
	}

	if script.Website != "" {
		sb.WriteString(fmt.Sprintf("[%s]Website:[-] %s\n", labelColor, script.Website))
	}

	if script.Documentation != "" {
		sb.WriteString(fmt.Sprintf("[%s]Documentation:[-] %s\n", labelColor, script.Documentation))
	}

	if script.DateCreated != "" {
		sb.WriteString(fmt.Sprintf("[%s]Date Created:[-] %s\n", labelColor, script.DateCreated))
	}

	sb.WriteString(fmt.Sprintf("\n[%s]Target Node:[-] %s\n", labelColor, s.node.Name))

	if s.vm != nil {
		sb.WriteString(fmt.Sprintf("[%s]Context:[-] VM %s\n", labelColor, s.vm.Name))
	}

	sb.WriteString(fmt.Sprintf("\n[%s]Note:[-] This will execute the script on the selected node via SSH.", labelColor))

	if script.Type == scriptTypeCT {
		sb.WriteString(" This will create a new LXC container.")
	} else if script.Type == scriptTypeVM {
		sb.WriteString(" This will create a new virtual machine.")
	}

	return sb.String()
}

// installScript installs the selected script.
func (s *ScriptSelector) installScript(script scripts.Script) {
	// Temporarily suspend the UI for interactive script installation (same pattern as working shell functions)
	s.app.Suspend(func() {
		// Install the script interactively
		fmt.Printf("Installing %s...\n", script.Name)

		err := scripts.InstallScript(s.user, s.nodeIP, script.ScriptPath)
		if err != nil {
			fmt.Printf("\nScript installation failed: %v\n", err)
		}
		// No waiting inside suspend block - let it complete naturally like working shell functions
	})

	// Fix for tview suspend/resume issue - sync the application after suspend
	s.app.Sync()
	// Give the terminal a brief moment to fully restore before UI operations to avoid blank screens
	go func() {
		time.Sleep(150 * time.Millisecond)
		// Clear API cache, then close the selector overlay and refresh
		s.app.client.ClearAPICache()
		s.app.QueueUpdateDraw(func() {
			// Close selector to return to main UI before refreshing
			s.Hide()
		})
		// Kick off a full refresh; it manages its own UI updates
		s.app.manualRefresh()
	}()
}

// onSearchChanged is called when the search input changes.
func (s *ScriptSelector) onSearchChanged(text string) {
	// If search is empty, show all scripts
	if text == "" {
		s.filteredScripts = s.scripts
	} else {
		// Filter scripts based on search text
		s.filteredScripts = []scripts.Script{}
		searchLower := strings.ToLower(text)

		for _, script := range s.scripts {
			// Search in name, description, and type
			if strings.Contains(strings.ToLower(script.Name), searchLower) ||
				strings.Contains(strings.ToLower(script.Description), searchLower) ||
				strings.Contains(strings.ToLower(script.Type), searchLower) {
				s.filteredScripts = append(s.filteredScripts, script)
			}
		}
	}

	// Update the script list
	s.scriptList.Clear()

	for _, script := range s.filteredScripts {
		// Add more detailed information in the secondary text
		var secondaryText string
		if script.Type == scriptTypeCT {
			secondaryText = fmt.Sprintf("Container: %s", script.Description)
		} else if script.Type == scriptTypeVM {
			secondaryText = fmt.Sprintf("VM: %s", script.Description)
		} else {
			secondaryText = script.Description
		}

		// Truncate description if too long
		if len(secondaryText) > 100 {
			secondaryText = secondaryText[:99] + "..."
		}

		// Add item without selection function - we handle Enter manually
		s.scriptList.AddItem(script.Name, secondaryText, 0, nil) // Remove shortcut label
	}

	// If we're in search mode and there are results, reset selection to first item
	if s.searchActive && len(s.filteredScripts) > 0 {
		s.scriptList.SetCurrentItem(0)
	}
}

// fetchScriptsForCategory fetches scripts for the selected category.
func (s *ScriptSelector) fetchScriptsForCategory(category scripts.ScriptCategory) {
	// Prevent multiple concurrent requests
	if s.isLoading {
		return
	}

	// Show loading indicator both in header and in modal
	s.isLoading = true
	s.app.header.ShowLoading(fmt.Sprintf("Fetching %s scripts", category.Name))

	// Switch to loading page immediately and set focus
	s.pages.SwitchToPage("loading")
	// Set focus to the pages component so the loading page can receive input
	s.app.SetFocus(s.pages)
	// Start the loading animation
	s.startLoadingAnimation()

	// Fetch scripts in a goroutine to prevent UI blocking
	go func() {
		fetchedScripts, err := scripts.GetScriptsByCategory(category.Path)

		// Update UI on the main thread
		s.app.QueueUpdateDraw(func() {
			// Stop loading indicator and reset loading state
			s.stopLoadingAnimation()
			s.isLoading = false
			s.app.header.StopLoading()

			if err != nil {
				// Show error message and go back to categories
				s.pages.SwitchToPage("categories")
				s.app.SetFocus(s.categoryList)
				s.app.showMessageSafe(fmt.Sprintf("Error fetching scripts: %v", err))

				return
			}

			// Sort scripts alphabetically by name
			sort.Slice(fetchedScripts, func(i, j int) bool {
				return fetchedScripts[i].Name < fetchedScripts[j].Name
			})

			// Store scripts and initialize filtered scripts
			s.scripts = fetchedScripts
			s.filteredScripts = fetchedScripts // Initially show all scripts

			// Clear the existing script list
			s.scriptList.Clear()

			// Add scripts to the existing list
			for _, script := range s.filteredScripts {
				// Add more detailed information in the secondary text
				var secondaryText string
				if script.Type == scriptTypeCT {
					secondaryText = fmt.Sprintf("Container: %s", script.Description)
				} else if script.Type == scriptTypeVM {
					secondaryText = fmt.Sprintf("VM: %s", script.Description)
				} else {
					secondaryText = script.Description
				}

				// Truncate description if too long
				if len(secondaryText) > 100 {
					secondaryText = secondaryText[:99] + "..."
				}

				// Add item without selection function - we handle Enter manually
				s.scriptList.AddItem(script.Name, secondaryText, 0, nil) // Remove shortcut label
			}

			// Set up input capture on the script list (only once, not every time)
			if s.scriptList.GetInputCapture() == nil {
				s.scriptList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
						// Go back to category list (handle both backspace variants)
						s.pages.SwitchToPage("categories")
						s.app.SetFocus(s.categoryList)

						return nil
					} else if event.Key() == tcell.KeyEscape {
						if len(s.searchInput.GetText()) > 0 {
							// Clear search and show all scripts
							s.searchInput.SetText("")
							s.searchActive = false
							s.app.SetFocus(s.scriptList)
						} else {
							s.pages.SwitchToPage("categories")
							s.app.SetFocus(s.categoryList)
						}

						return nil
					} else if event.Key() == tcell.KeyEnter {
						// Manually trigger the script selection using filtered scripts
						idx := s.scriptList.GetCurrentItem()
						if idx >= 0 && idx < len(s.filteredScripts) {
							script := s.filteredScripts[idx]

							selectFunc := s.createScriptSelectFunc(script)
							if selectFunc != nil {
								selectFunc()
							}
						}

						return nil
					} else if event.Key() == tcell.KeyRune {
						// Handle VI-like navigation and search activation
						switch event.Rune() {
						case 'j': // VI-like down navigation
							return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
						case 'k': // VI-like up navigation
							return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
						case 'h': // VI-like left navigation - go back to categories
							s.pages.SwitchToPage("categories")
							s.app.SetFocus(s.categoryList)

							return nil
						case 'l': // VI-like right navigation - no action (already at rightmost)
							return nil
						case '/': // Activate search
							s.searchActive = true
							s.app.SetFocus(s.searchInput)

							return nil
						}
					}
					// Let all other keys (including arrows and Tab) pass through normally
					return event
				})
			}

			// Set up input capture on search input field
			if s.searchInput.GetInputCapture() == nil {
				s.searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					if event.Key() == tcell.KeyEscape {
						// Clear search and return to script list
						s.searchInput.SetText("")
						s.searchActive = false
						s.app.SetFocus(s.scriptList)

						return nil
					} else if event.Key() == tcell.KeyEnter {
						// Move focus to script list
						s.searchActive = false
						s.app.SetFocus(s.scriptList)

						return nil
					}
					// Let all other keys (including Tab) pass through normally
					return event
				})
			}

			// Clear search input for new category
			s.searchInput.SetText("")
			s.searchActive = false

			// Switch to scripts page and set focus
			s.pages.SwitchToPage("scripts")
			s.app.SetFocus(s.scriptList)

			// Show success message in header
			s.app.header.ShowSuccess(fmt.Sprintf("Loaded %d %s scripts", len(fetchedScripts), category.Name))
		})
	}()
}

// createScriptSelectFunc creates a script selection handler for a specific script.
func (s *ScriptSelector) createScriptSelectFunc(script scripts.Script) func() {
	return func() {
		s.showScriptInfo(script)
	}
}
