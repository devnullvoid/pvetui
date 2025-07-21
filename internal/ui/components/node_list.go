package components

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/devnullvoid/proxmox-tui/internal/ui/models"
	"github.com/devnullvoid/proxmox-tui/internal/ui/theme"
	"github.com/devnullvoid/proxmox-tui/internal/ui/utils"
	"github.com/devnullvoid/proxmox-tui/pkg/api"
)

// NodeList encapsulates the node list panel
type NodeList struct {
	*tview.List
	nodes     []*api.Node
	onSelect  func(*api.Node)
	onChanged func(*api.Node)
	app       *App
}

// Ensure NodeList implements NodeListComponent
var _ NodeListComponent = (*NodeList)(nil)

// NewNodeList creates a new node list component
func NewNodeList() *NodeList {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Nodes ")
	list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Colors.Selection).Foreground(theme.Colors.Primary))

	return &NodeList{
		List:  list,
		nodes: nil,
	}
}

// SetCurrentItem wraps the list method to match the interface
func (nl *NodeList) SetCurrentItem(index int) *tview.List {
	return nl.List.SetCurrentItem(index)
}

// SetApp sets the parent app reference for focus management
func (nl *NodeList) SetApp(app *App) {
	nl.app = app

	// Set up input capture for arrow keys and VI-like navigation (hjkl)
	nl.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRight:
			if nl.app != nil {
				nl.app.SetFocus(nl.app.nodeDetails)
				return nil
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'l': // VI-like right navigation
				if nl.app != nil {
					nl.app.SetFocus(nl.app.nodeDetails)
					return nil
				}
			case 'j': // VI-like down navigation
				// Let the list handle down navigation naturally
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k': // VI-like up navigation
				// Let the list handle up navigation naturally
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'h': // VI-like left navigation - no action for node list (already at leftmost)
				return nil
			}
		}
		return event
	})
}

// SetNodes updates the list with the provided nodes
func (nl *NodeList) SetNodes(nodes []*api.Node) {
	nl.Clear()

	// Create a copy of the nodes slice to avoid modifying the original
	nodesCopy := make([]*api.Node, len(nodes))
	copy(nodesCopy, nodes)

	// Sort nodes by name for consistent ordering
	sort.Slice(nodesCopy, func(i, j int) bool {
		if nodesCopy[i] == nil || nodesCopy[j] == nil {
			return nodesCopy[i] != nil
		}
		return nodesCopy[i].Name < nodesCopy[j].Name
	})

	nl.nodes = nodesCopy

	for _, node := range nl.nodes {
		if node != nil {
			// Determine node status string
			var statusString string
			if node.Online {
				statusString = "online"
			} else {
				statusString = "offline"
			}

			// Check if this node has a pending operation
			isPending, operation := models.GlobalState.IsNodePending(node)

			// Format the node name with status indicator (including pending state)
			statusIndicator := utils.FormatPendingStatusIndicator(statusString, isPending, operation)

			var mainText string
			if isPending {
				// For pending nodes, apply a dimmed effect to the entire item
				mainText = statusIndicator + fmt.Sprintf("[gray]%s[-]", node.Name)
			} else {
				// Normal formatting
				mainText = statusIndicator + node.Name
			}

			nl.AddItem(mainText, "", 0, nil)
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

// GetNodes returns the current nodes slice
func (nl *NodeList) GetNodes() []*api.Node {
	return nl.nodes
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
