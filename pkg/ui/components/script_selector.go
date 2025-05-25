package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/scripts"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ScriptSelector represents a modal dialog for selecting and running community scripts
type ScriptSelector struct {
	*tview.Modal
	app                  *App
	user                 string
	nodeIP               string
	node                 *api.Node
	vm                   *api.VM
	categories           []scripts.ScriptCategory
	scripts              []scripts.Script
	categoryList         *tview.List
	scriptList           *tview.List
	backButton           *tview.Button
	layout               *tview.Flex
	pages                *tview.Pages
	isForNode            bool
	originalInputCapture func(*tcell.EventKey) *tcell.EventKey
}

// NewScriptSelector creates a new script selector dialog
func NewScriptSelector(app *App, node *api.Node, vm *api.VM, user string) *ScriptSelector {
	selector := &ScriptSelector{
		app:        app,
		user:       user,
		node:       node,
		vm:         vm,
		nodeIP:     node.IP,
		isForNode:  vm == nil,
		categories: scripts.GetScriptCategories(),
		Modal:      tview.NewModal(),
	}

	// Create the category list
	selector.categoryList = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorBlue).
		SetSelectedTextColor(tcell.ColorWhite)

	// Add categories to the list
	for i, category := range selector.categories {
		selector.categoryList.AddItem(
			category.Name,
			category.Description,
			rune('a'+i),
			nil, // Remove selection function - we handle Enter manually
		)
	}

	// Add a test item if no categories were loaded
	if len(selector.categories) == 0 {
		selector.categoryList.AddItem("No categories found", "Check script configuration", 'x', nil)
	}

	// Create the script list
	selector.scriptList = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorBlue).
		SetSelectedTextColor(tcell.ColorWhite)

	// Create a back button for the script list
	selector.backButton = tview.NewButton("Back").
		SetSelectedFunc(func() {
			selector.pages.SwitchToPage("categories")
			app.SetFocus(selector.categoryList)
		})

	// Create pages to switch between category and script lists
	selector.pages = tview.NewPages()

	// Set up the category page with title - simplified for testing
	categoryPage := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewTextView().
			SetText(fmt.Sprintf("Select a Script Category (%d categories)", len(selector.categories))).
			SetTextAlign(tview.AlignCenter), 1, 0, false).
		AddItem(selector.categoryList, 0, 1, true)

	// Set up the script page with title and back button
	// Create a flex container for the back button to make it focusable
	backButtonContainer := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(selector.backButton, 10, 0, true).
		AddItem(nil, 0, 1, false)

	scriptPage := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewTextView().
								SetText("Select a Script to Install (Backspace: Back)").
								SetTextAlign(tview.AlignCenter), 1, 0, false).
		AddItem(selector.scriptList, 18, 0, true). // Fixed height of 18 rows
		AddItem(backButtonContainer, 1, 0, false)

	// Add pages
	selector.pages.AddPage("categories", categoryPage, true, true)
	selector.pages.AddPage("scripts", scriptPage, true, false)

	// Set border and title directly on the pages component
	selector.pages.SetBorder(true).
		SetTitle(" Script Selection ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorBlue)

	// Use the pages component directly as the layout
	selector.layout = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(selector.pages, 22, 0, true).
			AddItem(nil, 0, 1, false), 80, 0, true).
		AddItem(nil, 0, 1, false)

	return selector
}

// fetchScriptsForCategory fetches scripts for the selected category
func (s *ScriptSelector) fetchScriptsForCategory(category scripts.ScriptCategory) {
	// Fetch scripts synchronously - no need for goroutine since we're just reading local definitions
	fetchedScripts, err := scripts.GetScriptsByCategory(category.Path)
	if err != nil {
		// Show error message
		s.pages.SwitchToPage("categories")
		s.app.SetFocus(s.categoryList)
		s.app.showMessage(fmt.Sprintf("Error fetching scripts: %v", err))
		return
	}

	// Sort scripts alphabetically by name
	sort.Slice(fetchedScripts, func(i, j int) bool {
		return fetchedScripts[i].Name < fetchedScripts[j].Name
	})

	// Store scripts
	s.scripts = fetchedScripts

	// Clear the existing script list
	s.scriptList.Clear()

	// Add scripts to the existing list
	for i, script := range s.scripts {
		// Add more detailed information in the secondary text
		var secondaryText string
		if script.Type == "ct" {
			secondaryText = fmt.Sprintf("Container: %s", script.Description)
		} else if script.Type == "vm" {
			secondaryText = fmt.Sprintf("VM: %s", script.Description)
		} else {
			secondaryText = script.Description
		}

		// Truncate description if too long
		if len(secondaryText) > 70 {
			secondaryText = secondaryText[:67] + "..."
		}

		// Add item without selection function - we handle Enter manually
		s.scriptList.AddItem(script.Name, secondaryText, rune('a'+i), nil)
	}

	// Set up input capture on the script list (only once, not every time)
	if s.scriptList.GetInputCapture() == nil {
		s.scriptList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
				// Go back to category list (handle both backspace variants)
				s.pages.SwitchToPage("categories")
				s.app.SetFocus(s.categoryList)
				return nil
			} else if event.Key() == tcell.KeyEnter {
				// Manually trigger the script selection
				idx := s.scriptList.GetCurrentItem()
				if idx >= 0 && idx < len(s.scripts) {
					script := s.scripts[idx]
					selectFunc := s.createScriptSelectFunc(script)
					if selectFunc != nil {
						selectFunc()
					}
				}
				return nil
			} else if event.Key() == tcell.KeyTab {
				// Tab to the back button
				s.app.SetFocus(s.backButton)
				return nil
			}
			// Let all other keys (including arrows) pass through normally
			return event
		})
	}

	// Switch to scripts page and set focus
	s.pages.SwitchToPage("scripts")
	s.app.SetFocus(s.scriptList)
}

// createScriptSelectFunc creates a script selection handler for a specific script
func (s *ScriptSelector) createScriptSelectFunc(script scripts.Script) func() {
	return func() {
		// Create a simple modal using tview.Modal for the script details
		scriptInfo := s.formatScriptInfo(script)

		modal := tview.NewModal().
			SetText(scriptInfo).
			SetBackgroundColor(tcell.ColorBlack).
			SetTextColor(tcell.ColorWhite).
			SetButtonBackgroundColor(tcell.ColorDarkGray).
			SetButtonTextColor(tcell.ColorWhite).
			AddButtons([]string{"Install", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				s.app.pages.RemovePage("scriptInfo")
				if buttonLabel == "Install" {
					go s.installScript(script)
				} else {
					s.app.SetFocus(s.scriptList)
				}
			})

		// Show the modal
		s.app.pages.AddPage("scriptInfo", modal, true, true)
	}
}

// formatScriptInfo formats the script information for display
func (s *ScriptSelector) formatScriptInfo(script scripts.Script) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[yellow]Name:[white] %s\n\n", script.Name))
	sb.WriteString(fmt.Sprintf("[yellow]Description:[white] %s\n\n", script.Description))

	if script.Type == "ct" {
		sb.WriteString("[yellow]Type:[white] Container Template\n")
	} else if script.Type == "vm" {
		sb.WriteString("[yellow]Type:[white] Virtual Machine\n")
	} else {
		sb.WriteString(fmt.Sprintf("[yellow]Type:[white] %s\n", script.Type))
	}

	if script.ScriptPath != "" {
		sb.WriteString(fmt.Sprintf("[yellow]Script Path:[white] %s\n", script.ScriptPath))
	}

	if script.Website != "" {
		sb.WriteString(fmt.Sprintf("[yellow]Website:[white] %s\n", script.Website))
	}

	if script.Documentation != "" {
		sb.WriteString(fmt.Sprintf("[yellow]Documentation:[white] %s\n", script.Documentation))
	}

	if script.DateCreated != "" {
		sb.WriteString(fmt.Sprintf("[yellow]Date Created:[white] %s\n", script.DateCreated))
	}

	sb.WriteString(fmt.Sprintf("\n[yellow]Target:[white] Node %s\n", s.node.Name))
	if s.vm != nil {
		sb.WriteString(fmt.Sprintf("[yellow]Context:[white] VM %s\n", s.vm.Name))
	}

	sb.WriteString("\n[yellow]Note:[white] This will execute the script on the selected node via SSH.")
	if script.Type == "ct" {
		sb.WriteString(" This will create a new LXC container.")
	} else if script.Type == "vm" {
		sb.WriteString(" This will create a new virtual machine.")
	}

	return sb.String()
}

// installScript installs the selected script
func (s *ScriptSelector) installScript(script scripts.Script) {
	// Close the script selector modal first
	s.app.QueueUpdateDraw(func() {
		// Remove the script selector
		s.app.pages.RemovePage("scriptSelector")

		// Restore the original input capture
		if s.originalInputCapture != nil {
			s.app.SetInputCapture(s.originalInputCapture)
		} else {
			// Clear any input capture if there was none originally
			s.app.SetInputCapture(nil)
		}

		// Restore focus to the appropriate list based on current page
		pageName, _ := s.app.pages.GetFrontPage()
		if pageName == "Nodes" {
			s.app.SetFocus(s.app.nodeList)
		} else if pageName == "Guests" {
			s.app.SetFocus(s.app.vmList)
		}
	})

	// Temporarily suspend the UI for interactive script installation
	s.app.Suspend(func() {
		// Display installation message
		fmt.Printf("\nInstalling %s on node %s (%s)...\n", script.Name, s.node.Name, s.nodeIP)
		fmt.Printf("Script: %s\n", script.ScriptPath)
		fmt.Printf("This script may require interactive input. Please follow the prompts.\n\n")

		// Install the script interactively
		err := scripts.InstallScript(s.user, s.nodeIP, script.ScriptPath)

		if err != nil {
			fmt.Printf("\nScript installation failed: %v\n", err)
		} else {
			fmt.Printf("\n%s installed successfully!\n", script.Name)
			fmt.Printf("You may need to refresh your node/guest list to see any new resources.\n")
		}

		// Wait for user to press Enter
		fmt.Print("\nPress Enter to return to the TUI...")
		fmt.Scanln()
	})
}

// Show displays the script selector
func (s *ScriptSelector) Show() {
	// Ensure we have a valid node IP
	if s.nodeIP == "" {
		s.app.showMessage("Node IP address not available. Cannot connect to install scripts.")
		return
	}

	// We still need SSH connection for script execution, so validate it
	err := scripts.ValidateConnection(s.user, s.nodeIP)
	if err != nil {
		s.app.showMessage(fmt.Sprintf("SSH connection failed: %v", err))
		return
	}

	// Store the original input capture
	s.originalInputCapture = s.app.GetInputCapture()

	// Set up a minimal app-level input capture that only handles Escape
	// All other keys will be passed through to allow normal navigation
	s.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Remove any script info modal first
			if s.app.pages.HasPage("scriptInfo") {
				s.app.pages.RemovePage("scriptInfo")
				s.app.SetFocus(s.scriptList)
				return nil
			}

			// Restore original input capture and close modal
			if s.originalInputCapture != nil {
				s.app.SetInputCapture(s.originalInputCapture)
			} else {
				s.app.SetInputCapture(nil)
			}
			s.app.pages.RemovePage("scriptSelector")

			// Restore focus to the appropriate list based on current page
			pageName, _ := s.app.pages.GetFrontPage()
			if pageName == "Nodes" {
				s.app.SetFocus(s.app.nodeList)
			} else if pageName == "Guests" {
				s.app.SetFocus(s.app.vmList)
			}
			return nil
		}
		// Pass ALL other events through to the focused component (including backspace)
		return event
	})

	// Remove individual input captures - let the lists handle navigation normally
	// The Enter key selection will be handled by the list's selected functions
	s.categoryList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			// Manually trigger the selection
			idx := s.categoryList.GetCurrentItem()
			if idx >= 0 && idx < len(s.categories) {
				category := s.categories[idx]
				// Try without goroutine to see if that fixes UI distortion
				s.fetchScriptsForCategory(category)
			}
			return nil
		} else if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			// Backspace on category list closes the modal (handle both backspace variants)
			// Restore original input capture and close modal
			if s.originalInputCapture != nil {
				s.app.SetInputCapture(s.originalInputCapture)
			} else {
				s.app.SetInputCapture(nil)
			}
			s.app.pages.RemovePage("scriptSelector")

			// Restore focus to the appropriate list based on current page
			pageName, _ := s.app.pages.GetFrontPage()
			if pageName == "Nodes" {
				s.app.SetFocus(s.app.nodeList)
			} else if pageName == "Guests" {
				s.app.SetFocus(s.app.vmList)
			}
			return nil
		}
		// Let arrow keys pass through for navigation
		return event
	})

	// Set input capture on back button to handle Tab back to script list
	s.backButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			// Tab back to script list
			s.app.SetFocus(s.scriptList)
			return nil
		} else if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			// Backspace also goes back to categories (handle both backspace variants)
			s.pages.SwitchToPage("categories")
			s.app.SetFocus(s.categoryList)
			return nil
		}
		// Let other keys pass through (Enter will trigger the button)
		return event
	})

	// Add the selector to the pages and focus the category list
	s.app.pages.AddPage("scriptSelector", s.layout, true, true)
	s.app.SetFocus(s.categoryList)
}
