package models

import (
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestFilterVMs_WithAdvancedFilters(t *testing.T) {
	originalVMsBackup := GlobalState.OriginalVMs
	filteredVMsBackup := GlobalState.FilteredVMs
	searchStatesBackup := GlobalState.SearchStates
	defer func() {
		GlobalState.OriginalVMs = originalVMsBackup
		GlobalState.FilteredVMs = filteredVMsBackup
		GlobalState.SearchStates = searchStatesBackup
	}()

	GlobalState.OriginalVMs = []*api.VM{
		{ID: 101, Name: "db-01", Node: "node-a", Type: api.VMTypeQemu, Status: api.VMStatusRunning, Tags: "prod;db"},
		{ID: 102, Name: "web-01", Node: "node-b", Type: api.VMTypeQemu, Status: api.VMStatusStopped, Tags: "prod;web"},
		{ID: 201, Name: "ct-cache", Node: "node-a", Type: api.VMTypeLXC, Status: api.VMStatusRunning, Tags: "cache;lab"},
	}
	GlobalState.FilteredVMs = nil
	GlobalState.SearchStates = map[string]*SearchState{}

	assertIDs := func(expected []int) {
		t.Helper()
		if len(GlobalState.FilteredVMs) != len(expected) {
			t.Fatalf("expected %d VMs, got %d", len(expected), len(GlobalState.FilteredVMs))
		}
		for i, vm := range GlobalState.FilteredVMs {
			if vm == nil {
				t.Fatalf("filtered VM %d is nil", i)
			}
			if vm.ID != expected[i] {
				t.Fatalf("expected VM ID %d at index %d, got %d", expected[i], i, vm.ID)
			}
		}
	}

	// No filters returns everything.
	GlobalState.SearchStates[api.PageGuests] = &SearchState{CurrentPage: api.PageGuests}
	FilterVMs("")
	assertIDs([]int{101, 102, 201})

	// Advanced filter by status.
	GlobalState.SearchStates[api.PageGuests] = &SearchState{
		CurrentPage: api.PageGuests,
		VMFilters:   VMFilterOptions{Status: api.VMStatusRunning},
	}
	FilterVMs("")
	assertIDs([]int{101, 201})

	// Combined advanced filters.
	GlobalState.SearchStates[api.PageGuests] = &SearchState{
		CurrentPage: api.PageGuests,
		VMFilters: VMFilterOptions{
			Status: api.VMStatusRunning,
			Type:   api.VMTypeQemu,
			Node:   "node-a",
		},
	}
	FilterVMs("")
	assertIDs([]int{101})

	// Combined text + advanced filters.
	GlobalState.SearchStates[api.PageGuests] = &SearchState{
		CurrentPage: api.PageGuests,
		Filter:      "db",
		VMFilters: VMFilterOptions{
			Status: api.VMStatusRunning,
		},
	}
	FilterVMs("db")
	assertIDs([]int{101})

	// Tag contains is case-insensitive.
	GlobalState.SearchStates[api.PageGuests] = &SearchState{
		CurrentPage: api.PageGuests,
		VMFilters:   VMFilterOptions{TagContains: "PROD"},
	}
	FilterVMs("")
	assertIDs([]int{101, 102})
}

func TestSearchStateHasActiveVMFilter(t *testing.T) {
	if (&SearchState{}).HasActiveVMFilter() {
		t.Fatal("expected empty search state to be inactive")
	}

	if !(&SearchState{Filter: "abc"}).HasActiveVMFilter() {
		t.Fatal("expected text filter to be active")
	}

	if !(&SearchState{VMFilters: VMFilterOptions{Node: "node-a"}}).HasActiveVMFilter() {
		t.Fatal("expected advanced filters to be active")
	}
}
