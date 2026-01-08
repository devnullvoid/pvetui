package api

import (
	"fmt"
)

// NodeUpdate represents an available update for a package.
type NodeUpdate struct {
	Package     string `json:"Package"`
	Title       string `json:"Title"`
	Version     string `json:"Version"`
	OldVersion  string `json:"OldVersion"`
	Arch        string `json:"Arch"`
	Description string `json:"Description"`
	Origin      string `json:"Origin"`
}

// GetNodeUpdates retrieves the list of available updates for a specific node.
func (c *Client) GetNodeUpdates(nodeName string) ([]NodeUpdate, error) {
	var res map[string]interface{}

	// /nodes/{node}/apt/update returns the list of upgradable packages (from `apt list --upgradable`).
	if err := c.GetWithCache(fmt.Sprintf("/nodes/%s/apt/update", nodeName), &res, NodeDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get node updates: %w", err)
	}

	data, ok := res["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid updates response format")
	}

	var updates []NodeUpdate
	for _, item := range data {
		if uData, ok := item.(map[string]interface{}); ok {
			update := NodeUpdate{
				Package:     getString(uData, "Package"),
				Title:       getString(uData, "Title"),
				Version:     getString(uData, "Version"),
				OldVersion:  getString(uData, "OldVersion"),
				Arch:        getString(uData, "Arch"),
				Description: getString(uData, "Description"),
				Origin:      getString(uData, "Origin"),
			}
			updates = append(updates, update)
		}
	}
	return updates, nil
}
