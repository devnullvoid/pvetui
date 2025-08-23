package components

import (
	"time"

	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/scripts"
	"github.com/devnullvoid/pvetui/pkg/api"
)

// Script type constants.
const (
	scriptTypeCT = "ct"
	scriptTypeVM = "vm"
)

// ScriptSelector represents a page-based script selector for installing community scripts.
type ScriptSelector struct {
	*tview.Pages

	app             *App
	user            string
	nodeIP          string
	node            *api.Node
	vm              *api.VM
	categories      []scripts.ScriptCategory
	scripts         []scripts.Script
	filteredScripts []scripts.Script // Filtered scripts based on search
	categoryList    *tview.List
	scriptList      *tview.List
	searchInput     *tview.InputField // Search input field
	backButton      *tview.Button
	layout          *tview.Flex
	pages           *tview.Pages
	isForNode       bool
	isLoading       bool            // Track loading state
	loadingText     *tview.TextView // For animation updates
	animationTicker *time.Ticker    // For loading animation
	searchActive    bool            // Whether search mode is active
}

// NewScriptSelector creates a new script selector.
func NewScriptSelector(app *App, node *api.Node, vm *api.VM, user string) *ScriptSelector {
	// Create the main pages container
	mainPages := tview.NewPages()

	s := &ScriptSelector{
		Pages:     mainPages,
		app:       app,
		user:      user,
		node:      node,
		vm:        vm,
		isForNode: vm == nil,
		pages:     tview.NewPages(), // Internal pages for categories/scripts
	}

	// Set node IP
	if node != nil {
		s.nodeIP = node.IP
	}

	// Initialize the layout
	s.categories = scripts.GetScriptCategories()
	s.createLayout()

	return s
}
