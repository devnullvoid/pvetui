package models

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rivo/tview"

	"github.com/devnullvoid/pvetui/internal/logger"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// SearchState holds the state for a search operation.
type SearchState struct {
	CurrentPage   string
	Filter        string
	SelectedIndex int
}

// State holds all UI state components.
type State struct {
	NodeList     tview.Primitive
	VMList       tview.Primitive
	SearchStates map[string]*SearchState

	// Current filtered lists
	FilteredNodes []*api.Node
	FilteredVMs   []*api.VM
	FilteredTasks []*api.ClusterTask

	// Original lists
	OriginalNodes []*api.Node
	OriginalVMs   []*api.VM
	OriginalTasks []*api.ClusterTask

	// Pending operations tracking
	PendingVMOperations   map[string]string // Key: "node:vmid", Value: operation description
	PendingNodeOperations map[string]string // Key: "nodename", Value: operation description
	pendingMutex          sync.RWMutex      // Thread-safe access to pending maps
}

// GlobalState is the singleton instance for UI state.
var GlobalState = State{
	SearchStates:          make(map[string]*SearchState),
	FilteredNodes:         make([]*api.Node, 0),
	FilteredVMs:           make([]*api.VM, 0),
	FilteredTasks:         make([]*api.ClusterTask, 0),
	OriginalNodes:         make([]*api.Node, 0),
	OriginalVMs:           make([]*api.VM, 0),
	OriginalTasks:         make([]*api.ClusterTask, 0),
	PendingVMOperations:   make(map[string]string),
	PendingNodeOperations: make(map[string]string),
}

// UI logger instance - will be set by the main application.
var uiLogger interfaces.Logger

// SetUILogger sets the shared logger instance for UI components.
func SetUILogger(logger interfaces.Logger) {
	uiLogger = logger
}

// GetUILogger returns the UI logger, with fallback if not set.
func GetUILogger() interfaces.Logger {
	if uiLogger != nil {
		return uiLogger
	}
	// Fallback to global logger if not set
	return logger.GetGlobalLogger()
}

// GetSearchState returns the search state for a given component.
func (s *State) GetSearchState(component string) *SearchState {
	state, exists := s.SearchStates[component]
	if !exists {
		return nil
	}

	return state
}

// FilterNodes filters the nodes based on the given search string.
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
	// GetUILogger().Debug("Filtered nodes from %d to %d with filter '%s'",
	//
	//	len(GlobalState.OriginalNodes), len(GlobalState.FilteredNodes), filter)
}

// FilterVMs filters the VMs based on the given search string.
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
	// GetUILogger().Debug("Filtered VMs from %d to %d with filter '%s'",
	//
	//	len(GlobalState.OriginalVMs), len(GlobalState.FilteredVMs), filter)
}

// FilterTasks filters the tasks based on the given search string.
func FilterTasks(filter string) {
	if filter == "" {
		// No filter, use all tasks
		GlobalState.FilteredTasks = make([]*api.ClusterTask, len(GlobalState.OriginalTasks))
		copy(GlobalState.FilteredTasks, GlobalState.OriginalTasks)

		return
	}

	// Convert filter to lowercase for case-insensitive search
	filter = strings.ToLower(filter)

	// Create a new filtered list
	GlobalState.FilteredTasks = make([]*api.ClusterTask, 0)

	// Add tasks that match the filter
	for _, task := range GlobalState.OriginalTasks {
		if task == nil {
			continue
		}

		// Check task ID
		if strings.Contains(strings.ToLower(task.ID), filter) {
			GlobalState.FilteredTasks = append(GlobalState.FilteredTasks, task)

			continue
		}

		// Check task node
		if strings.Contains(strings.ToLower(task.Node), filter) {
			GlobalState.FilteredTasks = append(GlobalState.FilteredTasks, task)

			continue
		}

		// Check task type
		if strings.Contains(strings.ToLower(task.Type), filter) {
			GlobalState.FilteredTasks = append(GlobalState.FilteredTasks, task)

			continue
		}

		// Check task status
		if strings.Contains(strings.ToLower(task.Status), filter) {
			GlobalState.FilteredTasks = append(GlobalState.FilteredTasks, task)

			continue
		}

		// Check task user
		if strings.Contains(strings.ToLower(task.User), filter) {
			GlobalState.FilteredTasks = append(GlobalState.FilteredTasks, task)

			continue
		}

		// Check UPID
		if strings.Contains(strings.ToLower(task.UPID), filter) {
			GlobalState.FilteredTasks = append(GlobalState.FilteredTasks, task)

			continue
		}
	}
	// GetUILogger().Debug("Filtered tasks from %d to %d with filter '%s'",
	//
	//	len(GlobalState.OriginalTasks), len(GlobalState.FilteredTasks), filter)
}

// SetVMPending marks a VM as having a pending operation.
func (s *State) SetVMPending(vm *api.VM, operation string) {
	s.pendingMutex.Lock()
	defer s.pendingMutex.Unlock()

	key := fmt.Sprintf("%s:%d", vm.Node, vm.ID)
	s.PendingVMOperations[key] = operation
}

// ClearVMPending removes the pending operation status for a VM.
func (s *State) ClearVMPending(vm *api.VM) {
	s.pendingMutex.Lock()
	defer s.pendingMutex.Unlock()

	key := fmt.Sprintf("%s:%d", vm.Node, vm.ID)
	delete(s.PendingVMOperations, key)
}

// IsVMPending checks if a VM has a pending operation.
func (s *State) IsVMPending(vm *api.VM) (bool, string) {
	s.pendingMutex.RLock()
	defer s.pendingMutex.RUnlock()

	key := fmt.Sprintf("%s:%d", vm.Node, vm.ID)
	operation, exists := s.PendingVMOperations[key]

	return exists, operation
}

// SetNodePending marks a node as having a pending operation.
func (s *State) SetNodePending(node *api.Node, operation string) {
	s.pendingMutex.Lock()
	defer s.pendingMutex.Unlock()
	s.PendingNodeOperations[node.Name] = operation
}

// ClearNodePending removes the pending operation status for a node.
func (s *State) ClearNodePending(node *api.Node) {
	s.pendingMutex.Lock()
	defer s.pendingMutex.Unlock()
	delete(s.PendingNodeOperations, node.Name)
}

// IsNodePending checks if a node has a pending operation.
func (s *State) IsNodePending(node *api.Node) (bool, string) {
	s.pendingMutex.RLock()
	defer s.pendingMutex.RUnlock()
	operation, exists := s.PendingNodeOperations[node.Name]

	return exists, operation
}

// HasPendingOperations checks if there are any pending VM or node operations.
func (s *State) HasPendingOperations() bool {
	s.pendingMutex.RLock()
	defer s.pendingMutex.RUnlock()

	return len(s.PendingVMOperations) > 0 || len(s.PendingNodeOperations) > 0
}
