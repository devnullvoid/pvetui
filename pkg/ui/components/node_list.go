package components

import (
	"github.com/devnullvoid/proxmox-tui/pkg/api"
	"github.com/devnullvoid/proxmox-tui/pkg/ui/utils"
	"github.com/rivo/tview"
)

// NodeList encapsulates the node list panel
type NodeList struct {
	*tview.List
	nodes     []*api.Node
	onSelect  func(*api.Node)
	onChanged func(*api.Node)
}

// NewNodeList creates a new node list component
func NewNodeList() *NodeList {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle("Nodes")

	return &NodeList{
		List:  list,
		nodes: nil,
	}
}

// SetNodes updates the list with the provided nodes
func (nl *NodeList) SetNodes(nodes []*api.Node) {
	nl.Clear()
	nl.nodes = nodes

	for _, node := range nodes {
		if node != nil {
			var statusString string
			if node.Online {
				statusString = "online"
			} else {
				statusString = "offline"
			}
			// Format the node name with status indicator
			mainText := utils.FormatStatusIndicator(statusString) + node.Name
			nl.AddItem(mainText, "", 0, nil)
		}
	}

	// If there are nodes, select the first one by default
	if len(nodes) > 0 {
		nl.SetCurrentItem(0)
		if nl.onSelect != nil {
			nl.onSelect(nodes[0])
		}
	}
}

// GetSelectedNode returns the currently selected node
func (nl *NodeList) GetSelectedNode() *api.Node {
	idx := nl.GetCurrentItem()
	if idx >= 0 && idx < len(nl.nodes) {
		return nl.nodes[idx]
	}
	return nil
}

// SetSelectedFunc sets the function to be called when a node is selected
func (nl *NodeList) SetNodeSelectedFunc(handler func(*api.Node)) {
	nl.onSelect = handler

	nl.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(nl.nodes) {
			if nl.onSelect != nil {
				nl.onSelect(nl.nodes[index])
			}
		}
	})
}

// SetChangedFunc sets the function to be called when selection changes
func (nl *NodeList) SetNodeChangedFunc(handler func(*api.Node)) {
	nl.onChanged = handler

	nl.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(nl.nodes) {
			if nl.onChanged != nil {
				nl.onChanged(nl.nodes[index])
			}
		}
	})
}
