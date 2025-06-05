package models

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/config"
	"github.com/devnullvoid/proxmox-tui/internal/logger"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/api/interfaces"
)

// SearchState holds the state for a search operation
type SearchState struct {
	CurrentPage   string
	Filter        string
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
	SearchStates:  make(map[string]*SearchState),
	FilteredNodes: make([]*api.Node, 0),
	FilteredVMs:   make([]*api.VM, 0),
	OriginalNodes: make([]*api.Node, 0),
	OriginalVMs:   make([]*api.VM, 0),
}

// UI logger instance
var (
	uiLogger     interfaces.Logger
	uiLoggerOnce sync.Once
)

// getUILogger returns the UI logger, initializing it if necessary
func getUILogger() interfaces.Logger {
	uiLoggerOnce.Do(func() {
		// Create a logger for UI operations that logs to file
		level := logger.LevelInfo
		if config.DebugEnabled {
			level = logger.LevelDebug
		}
		var err error
		// Always use our new internal logger system
		uiLogger, err = logger.NewInternalLogger(level)
		if err != nil {
			// Fallback to simple logger if file logging fails
			uiLogger = logger.NewSimpleLogger(level)
		}
	})
	return uiLogger
}

// GetSearchState returns the search state for a given component
func (s *State) GetSearchState(component string) *SearchState {
	state, exists := s.SearchStates[component]
	if !exists {
		return nil
	}
	return state
}

// FilterNodes filters the nodes based on the given search string
func FilterNodes(filter string) {
	if filter == "" {
		// No filter, use all nodes
		GlobalState.FilteredNodes = make([]*api.Node, len(GlobalState.OriginalNodes))
		copy(GlobalState.FilteredNodes, GlobalState.OriginalNodes)
		return
	}

	// Convert filter to lowercase for case-insensitive search
	filter = strings.ToLower(filter)

	// Create a new filtered list
	GlobalState.FilteredNodes = make([]*api.Node, 0)

	// Add nodes that match the filter
	for _, node := range GlobalState.OriginalNodes {
		if node == nil {
			continue
		}

		// Check node name
		if strings.Contains(strings.ToLower(node.Name), filter) {
			GlobalState.FilteredNodes = append(GlobalState.FilteredNodes, node)
			continue
		}

		// Check node IP
		if strings.Contains(strings.ToLower(node.IP), filter) {
			GlobalState.FilteredNodes = append(GlobalState.FilteredNodes, node)
			continue
		}

		// Check node status (using online status instead)
		statusText := "offline"
		if node.Online {
			statusText = "online"
		}
		if strings.Contains(statusText, filter) {
			GlobalState.FilteredNodes = append(GlobalState.FilteredNodes, node)
			continue
		}
	}

	getUILogger().Debug("Filtered nodes from %d to %d with filter '%s'",
		len(GlobalState.OriginalNodes), len(GlobalState.FilteredNodes), filter)
}

// FilterVMs filters the VMs based on the given search string
func FilterVMs(filter string) {
	if filter == "" {
		// No filter, use all VMs
		GlobalState.FilteredVMs = make([]*api.VM, len(GlobalState.OriginalVMs))
		copy(GlobalState.FilteredVMs, GlobalState.OriginalVMs)
		return
	}

	// Convert filter to lowercase for case-insensitive search
	filter = strings.ToLower(filter)

	// Create a new filtered list
	GlobalState.FilteredVMs = make([]*api.VM, 0)

	// Add VMs that match the filter
	for _, vm := range GlobalState.OriginalVMs {
		if vm == nil {
			continue
		}

		// Check VM name
		if strings.Contains(strings.ToLower(vm.Name), filter) {
			GlobalState.FilteredVMs = append(GlobalState.FilteredVMs, vm)
			continue
		}

		// Check VM ID (convert int to string)
		vmIDStr := fmt.Sprintf("%d", vm.ID)
		if strings.Contains(vmIDStr, filter) {
			GlobalState.FilteredVMs = append(GlobalState.FilteredVMs, vm)
			continue
		}

		// Check VM type
		if strings.Contains(strings.ToLower(vm.Type), filter) {
			GlobalState.FilteredVMs = append(GlobalState.FilteredVMs, vm)
			continue
		}

		// Check VM status
		if strings.Contains(strings.ToLower(vm.Status), filter) {
			GlobalState.FilteredVMs = append(GlobalState.FilteredVMs, vm)
			continue
		}

		// Check VM node
		if strings.Contains(strings.ToLower(vm.Node), filter) {
			GlobalState.FilteredVMs = append(GlobalState.FilteredVMs, vm)
			continue
		}
	}

	getUILogger().Debug("Filtered VMs from %d to %d with filter '%s'",
		len(GlobalState.OriginalVMs), len(GlobalState.FilteredVMs), filter)
}
