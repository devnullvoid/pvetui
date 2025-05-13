package ui

import "github.com/lonepie/proxmox-tui/pkg/api"

// FormatNodeName adds status emoji to node names
func FormatNodeName(node api.Node) string {
	if node.Online {
		return "🟢 " + node.Name
	}
	return node.Name
}
