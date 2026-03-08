package components

import (
	"math"
	"strings"
	"testing"

	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/pkg/api"
)

func findDetailValueByLabel(vd *VMDetails, label string) (string, bool) {
	for row := 0; row < 80; row++ {
		keyCell := vd.GetCell(row, 0)
		if keyCell == nil {
			continue
		}
		if strings.Contains(keyCell.Text, label) {
			valCell := vd.GetCell(row, 1)
			if valCell == nil {
				return "", true
			}

			return valCell.Text, true
		}
	}

	return "", false
}

func TestVMDetailsShowsDescriptionRowWithNAWhenEmpty(t *testing.T) {
	vd := NewVMDetails()
	vd.SetApp(&App{config: config.Config{ShowIcons: false}})

	vm := &api.VM{
		ID:          101,
		Name:        "test-lxc",
		Type:        api.VMTypeLXC,
		Node:        "pve1",
		Status:      api.VMStatusRunning,
		Description: "",
	}

	vd.Update(vm)

	value, found := findDetailValueByLabel(vd, "Description")
	if !found {
		t.Fatalf("expected Description row to be present")
	}
	if value != api.StringNA {
		t.Fatalf("expected Description value %q, got %q", api.StringNA, value)
	}
}

func TestVMDetailsShowsZeroCPUForRunningGuestWithNonFiniteMetric(t *testing.T) {
	vd := NewVMDetails()
	vd.SetApp(&App{config: config.Config{ShowIcons: false}})

	vm := &api.VM{
		ID:       101,
		Name:     "test-lxc",
		Type:     api.VMTypeLXC,
		Node:     "pve1",
		Status:   api.VMStatusRunning,
		CPU:      math.NaN(),
		CPUCores: 2,
	}

	vd.Update(vm)

	value, found := findDetailValueByLabel(vd, "CPU")
	if !found {
		t.Fatalf("expected CPU row to be present")
	}
	if value != "0.0% of 2 cores" {
		t.Fatalf("expected CPU value %q, got %q", "0.0% of 2 cores", value)
	}
}

func TestVMDetailsLabelsTemplates(t *testing.T) {
	vd := NewVMDetails()
	vd.SetApp(&App{config: config.Config{ShowIcons: false}})

	vm := &api.VM{
		ID:       900,
		Name:     "ubuntu-template",
		Type:     api.VMTypeQemu,
		Node:     "pve1",
		Status:   api.VMStatusStopped,
		Template: true,
	}

	vd.Update(vm)

	typeValue, found := findDetailValueByLabel(vd, "Type")
	if !found {
		t.Fatalf("expected Type row to be present")
	}
	if typeValue != "QEMU Template" {
		t.Fatalf("expected Type value %q, got %q", "QEMU Template", typeValue)
	}

	statusValue, found := findDetailValueByLabel(vd, "Status")
	if !found {
		t.Fatalf("expected Status row to be present")
	}
	if statusValue != "Template" {
		t.Fatalf("expected Status value %q, got %q", "Template", statusValue)
	}

	if _, found := findDetailValueByLabel(vd, "Template"); found {
		t.Fatalf("did not expect a separate Template row")
	}
}
