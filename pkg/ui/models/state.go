package models

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/rivo/tview"
)

// SearchState holds the state for a search operation
type SearchState struct {
	CurrentPage   string
	SearchText    string
	SelectedIndex int
}

// State holds all UI state components
type State struct {
	NodeList     tview.Primitive
	VMList       tview.Primitive
	SearchStates map[string]*SearchState
	
	// Current filtered lists
	FilteredNodes []*api.Node
	FilteredVMs   []*api.VM
	
	// Original lists
	OriginalNodes []*api.Node
	OriginalVMs   []*api.VM
}

// GlobalState is the singleton instance for UI state
var GlobalState = State{
	SearchStates: make(map[string]*SearchState),
	FilteredNodes: make([]*api.Node, 0),
	FilteredVMs:   make([]*api.VM, 0),
	OriginalNodes: make([]*api.Node, 0),
	OriginalVMs:   make([]*api.VM, 0),
}
