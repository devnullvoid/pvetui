package components

import (
	"regexp"
	"strings"
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestNodeDetailsGuestCounts(t *testing.T) {
	for _, tc := range []struct {
		name          string
		guests        []*api.VM
		vms           string
		lxcs          string
		duplicateNode bool
	}{
		{
			name: "containers are not virtual machines",
			guests: []*api.VM{
				{Type: api.VMTypeLXC, Status: api.VMStatusRunning},
				{Type: api.VMTypeLXC, Status: api.VMStatusRunning},
				{Type: api.VMTypeLXC, Status: api.VMStatusStopped},
			},
			vms: "0 running, 0 stopped, 0 templates", lxcs: "2 running, 1 stopped, 0 templates",
		},
		{
			name: "mixed guests and templates",
			guests: []*api.VM{
				{Type: api.VMTypeQemu, Status: api.VMStatusRunning},
				{Type: api.VMTypeQemu, Status: api.VMStatusStopped},
				{Type: api.VMTypeQemu, Status: api.VMStatusStopped, Template: true},
				{Type: api.VMTypeLXC, Status: api.VMStatusRunning},
				{Type: api.VMTypeLXC, Status: api.VMStatusStopped},
				{Type: api.VMTypeLXC, Status: api.VMStatusStopped, Template: true},
			},
			vms: "1 running, 1 stopped, 1 templates", lxcs: "1 running, 1 stopped, 1 templates",
		},
		{
			name: "unknown states and types are not stopped guests",
			guests: []*api.VM{
				{Type: api.VMTypeQemu, Status: "paused"},
				{Type: api.VMTypeLXC, Status: "unknown"},
				{Type: "unknown", Status: api.VMStatusRunning},
				nil,
			},
			vms: "0 running, 0 stopped, 0 templates", lxcs: "0 running, 0 stopped, 0 templates",
		},
		{name: "empty node", vms: "0 running, 0 stopped, 0 templates", lxcs: "0 running, 0 stopped, 0 templates"},
		{
			name:   "same node name in another source profile",
			guests: []*api.VM{{Type: api.VMTypeLXC, Status: api.VMStatusRunning}},
			vms:    "0 running, 0 stopped, 0 templates", lxcs: "1 running, 0 stopped, 0 templates",
			duplicateNode: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := &api.Node{Name: "mars", SourceProfile: "live", VMs: tc.guests}
			other := &api.Node{Name: "saturn", SourceProfile: "test", VMs: []*api.VM{{Type: api.VMTypeQemu, Status: api.VMStatusRunning}}}
			if tc.duplicateNode {
				other.Name = node.Name
			}
			details := NewNodeDetails()
			details.Update(node, []*api.Node{other, node})
			tags := regexp.MustCompile(`\[[^\]]*\]`)
			for label, want := range map[string]string{"VMs": tc.vms, "LXC": tc.lxcs} {
				found := false
				for row := 0; row < details.GetRowCount(); row++ {
					if !strings.HasSuffix(details.GetCell(row, 0).Text, label) {
						continue
					}
					found = true
					if got := tags.ReplaceAllString(details.GetCell(row, 1).Text, ""); got != want {
						t.Errorf("%s counts = %q, want %q", label, got, want)
					}
				}
				if !found {
					t.Errorf("missing %s row", label)
				}
			}
		})
	}
}
