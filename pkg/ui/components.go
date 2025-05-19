package ui

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/components"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/utils"
	"github.com/rivo/tview"
)

// CreateComponentHeader returns the application header bar using the component library
func CreateComponentHeader() *tview.TextView {
	return components.NewHeader().TextView
}

// CreateComponentFooter returns the application footer with key bindings using the component library
func CreateComponentFooter() *tview.TextView {
	return components.NewFooter().TextView
}

// CreateComponentClusterStatusPanel creates the cluster-wide status panel using the component library
func CreateComponentClusterStatusPanel() (*tview.Flex, *tview.Table, *tview.Table) {
	cs := components.NewClusterStatus()
	return cs.Flex, cs.SummaryTable, cs.ResourceTable
}

// CreateComponentVMList creates a list of VMs using the component library
func CreateComponentVMList(vms []api.VM) *tview.List {
	vmList := components.NewVMList()
	
	// Convert []api.VM to []*api.VM
	vmPtrs := make([]*api.VM, len(vms))
	for i := range vms {
		vmCopy := vms[i]
		vmPtrs[i] = &vmCopy
	}
	
	vmList.SetVMs(vmPtrs)
	return vmList.List
}

// UpdateNodeDetailsWithComponent displays node details in the provided table using the component library
func UpdateNodeDetailsWithComponent(table *tview.Table, node *api.Node, fullNodeList []*api.Node) {
	nodeDetails := components.NodeDetails{Table: table}
	nodeDetails.Update(node, fullNodeList)
}

// GetFormattedNodeName formats a node's name with its status indicator using the component library
func GetFormattedNodeName(node *api.Node) string {
	return utils.FormatNodeName(node)
}

// UpdateVMListWithComponent builds a list of VMs using the component library
func UpdateVMListWithComponent(vms []*api.VM, list *tview.List) {
	vmList := components.VMList{List: list}
	vmList.SetVMs(vms)
}
