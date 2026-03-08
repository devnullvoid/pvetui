package components

import (
	"strings"
	"testing"

	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/stretchr/testify/require"
)

func TestVMListLabelsTemplates(t *testing.T) {
	vl := NewVMList()
	vl.SetVMs([]*api.VM{
		{
			ID:       900,
			Name:     "ubuntu-template",
			Node:     "pve1",
			Type:     api.VMTypeQemu,
			Status:   api.VMStatusStopped,
			Template: true,
		},
	})

	main, _ := vl.GetItemText(0)
	require.True(t, strings.Contains(main, "(template)"), "expected template marker in list item, got %q", main)
}
