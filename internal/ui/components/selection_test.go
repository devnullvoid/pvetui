package components

import (
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestShouldRestoreVMSelection(t *testing.T) {
	tests := []struct {
		name    string
		hasVM   bool
		vmID    int
		vmNode  string
		current *api.VM
		want    bool
	}{
		{
			name:    "restore when no current selection",
			hasVM:   true,
			vmID:    101,
			vmNode:  "pve1",
			current: nil,
			want:    true,
		},
		{
			name:   "restore when selection unchanged",
			hasVM:  true,
			vmID:   101,
			vmNode: "pve1",
			current: &api.VM{
				ID:   101,
				Node: "pve1",
			},
			want: true,
		},
		{
			name:   "do not restore when user changed vm selection",
			hasVM:  true,
			vmID:   101,
			vmNode: "pve1",
			current: &api.VM{
				ID:   202,
				Node: "pve1",
			},
			want: false,
		},
		{
			name:   "do not restore when user changed node selection",
			hasVM:  true,
			vmID:   101,
			vmNode: "pve1",
			current: &api.VM{
				ID:   101,
				Node: "pve2",
			},
			want: false,
		},
		{
			name:    "disabled restore",
			hasVM:   false,
			vmID:    101,
			vmNode:  "pve1",
			current: nil,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRestoreVMSelection(tt.hasVM, tt.vmID, tt.vmNode, tt.current)
			if got != tt.want {
				t.Fatalf("shouldRestoreVMSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldRestoreNodeSelection(t *testing.T) {
	tests := []struct {
		name    string
		hasNode bool
		node    string
		current *api.Node
		want    bool
	}{
		{
			name:    "restore when no current selection",
			hasNode: true,
			node:    "pve1",
			current: nil,
			want:    true,
		},
		{
			name:    "restore when selection unchanged",
			hasNode: true,
			node:    "pve1",
			current: &api.Node{Name: "pve1"},
			want:    true,
		},
		{
			name:    "do not restore when user changed node selection",
			hasNode: true,
			node:    "pve1",
			current: &api.Node{Name: "pve2"},
			want:    false,
		},
		{
			name:    "disabled restore",
			hasNode: false,
			node:    "pve1",
			current: nil,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRestoreNodeSelection(tt.hasNode, tt.node, tt.current)
			if got != tt.want {
				t.Fatalf("shouldRestoreNodeSelection() = %v, want %v", got, tt.want)
			}
		})
	}
}
