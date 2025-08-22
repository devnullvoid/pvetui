package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/ui/theme"
)

// createLayout creates the main layout for the script selector.
func (s *ScriptSelector) createLayout() {
	// Create the category list
	s.categoryList = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary))

	// Add categories to the list
	for _, category := range s.categories {
		s.categoryList.AddItem(
			category.Name,
			category.Description,
			0,   // Remove shortcut label
			nil, // Remove selection function - we handle Enter manually
		)
	}

	// Add a test item if no categories were loaded
	if len(s.categories) == 0 {
		s.categoryList.AddItem("No categories found", "Check script configuration", 'x', nil)
	}

	// Create the script list
	s.scriptList = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary))

	// Create search input field
	s.searchInput = tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(0).
		SetPlaceholder("Type to filter scripts...").
		SetChangedFunc(s.onSearchChanged)

	// Create a back button for the script list
	s.backButton = tview.NewButton("Back").
		SetSelectedFunc(func() {
			s.pages.SwitchToPage("categories")
			s.app.SetFocus(s.categoryList)
		})

	// Set up the category page with title
	categoryPage := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewTextView().
			SetText(fmt.Sprintf("Select a Script Category (%d categories)", len(s.categories))).
			SetTextAlign(tview.AlignCenter), 1, 0, false).
		AddItem(s.categoryList, 0, 1, true)

	// Set up the script page with title, search, and back button
	backButtonContainer := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(s.backButton, 10, 0, true).
		AddItem(nil, 0, 1, false)

	// Create script page header with instructions
	// headerText := tview.NewTextView().
	// 	SetText("Select a Script to Install (/ = Search, Backspace = Back, Escape = Clear Search)").
	// 	SetTextAlign(tview.AlignCenter)

	scriptPage := tview.NewFlex().
		SetDirection(tview.FlexRow).
		// AddItem(headerText, 1, 0, false).
		AddItem(s.searchInput, 1, 0, false).
		AddItem(s.scriptList, 0, 1, true).
		AddItem(backButtonContainer, 1, 0, false)

	// Add global Tab navigation to the script page
	scriptPage.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			// Handle Tab navigation globally for the script page
			currentFocus := s.app.GetFocus()
			if currentFocus == s.searchInput {
				// From search input to script list
				s.app.SetFocus(s.scriptList)

				return nil
			} else if currentFocus == s.scriptList {
				// From script list to back button
				s.app.SetFocus(s.backButton)

				return nil
			} else if currentFocus == s.backButton {
				// From back button to search input
				s.searchActive = true
				s.app.SetFocus(s.searchInput)

				return nil
			}
		}
		// Let other events pass through to focused components
		return event
	})

	// Create loading page
	loadingPage := s.createLoadingPage()

	// Add pages to internal page container
	s.pages.AddPage("categories", categoryPage, true, true)
	s.pages.AddPage("scripts", scriptPage, true, false)
	s.pages.AddPage("loading", loadingPage, true, false)

	// Set border and title on the pages component
	s.pages.SetBorder(true).
		SetTitle(" Script Selection ").
		SetTitleColor(theme.Colors.Title)

	// Create the main layout
	s.layout = s.createResponsiveLayout()

	// Add the main layout to the main pages container
	s.Pages.AddPage("script-selector", s.layout, true, true)
}

// createLoadingPage creates the loading page.
func (s *ScriptSelector) createLoadingPage() *tview.Flex {
	// Create loading text view
	s.loadingText = tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText("⠋ Fetching scripts from GitHub, please wait...")

	// Create loading page layout
	loadingPage := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(s.loadingText, 3, 0, true).
		AddItem(nil, 0, 1, false)

	return loadingPage
}

// createResponsiveLayout creates the responsive layout for the script selector.
func (s *ScriptSelector) createResponsiveLayout() *tview.Flex {
	return tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(s.pages, 80, 0, true).
		AddItem(nil, 0, 1, false)
}
