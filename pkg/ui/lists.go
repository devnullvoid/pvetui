package ui

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/lonepie/proxmox-tui/pkg/api"
)

// newNodeList builds a node list UI component from nodes.
func newNodeList(nodes []api.Node) *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle("Nodes")
	for _, n := range nodes {
		list.AddItem(n.Name, "", 0, nil)
	}
	return list
}

// newVmList builds a VM list UI component from VMs.
func newVmList(vms []api.VM) *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle("Guests")
	for _, vm := range vms {
		list.AddItem(fmt.Sprintf("%d - %s", vm.ID, vm.Name), "", 0, nil)
	}
	return list
}
