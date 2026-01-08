package api

import (
	"fmt"
	"net/url"
)

// NodeDisk represents a disk on a Proxmox node.
type NodeDisk struct {
	DevPath string `json:"devpath"`
	Health  string `json:"health"`
	Model   string `json:"model"`
	Serial  string `json:"serial"`
	Size    int64  `json:"size"`
	Type    string `json:"type"`
	Used    string `json:"used"`
}

// SmartAttribute represents a single SMART attribute.
type SmartAttribute struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value int    `json:"value"`
	Worst int    `json:"worst"`
	Thresh int   `json:"thresh"`
	Fail  bool   `json:"fail"`
	Raw   string `json:"raw"`
}

// SmartStatus represents the SMART status of a disk.
type SmartStatus struct {
	Health     string           `json:"health"`
	Type       string           `json:"type"`
	Text       string           `json:"text"`
	Attributes []SmartAttribute `json:"attributes"`
}

// GetNodeDisks retrieves the list of disks for a specific node.
func (c *Client) GetNodeDisks(nodeName string) ([]NodeDisk, error) {
	var res map[string]interface{}
	// include-partitions=0 (default) to get physical disks.
	if err := c.GetWithCache(fmt.Sprintf("/nodes/%s/disks/list", nodeName), &res, NodeDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get node disks: %w", err)
	}

	data, ok := res["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid disk response format")
	}

	var disks []NodeDisk
	for _, item := range data {
		if dData, ok := item.(map[string]interface{}); ok {
			disk := NodeDisk{
				DevPath: getString(dData, "devpath"),
				Health:  getString(dData, "health"),
				Model:   getString(dData, "model"),
				Serial:  getString(dData, "serial"),
				Type:    getString(dData, "type"),
				Used:    getString(dData, "used"),
				Size:    int64(getFloat(dData, "size")),
			}
			disks = append(disks, disk)
		}
	}
	return disks, nil
}

// GetNodeDiskSmart retrieves the SMART status for a specific disk on a node.
func (c *Client) GetNodeDiskSmart(nodeName, diskPath string) (*SmartStatus, error) {
	var res map[string]interface{}
	// /nodes/{node}/disks/smart?disk={diskPath}
	path := fmt.Sprintf("/nodes/%s/disks/smart?disk=%s", nodeName, url.QueryEscape(diskPath))

	if err := c.GetWithCache(path, &res, NodeDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get disk smart info: %w", err)
	}

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid smart response format")
	}

	smart := &SmartStatus{
		Health: getString(data, "health"),
		Type:   getString(data, "type"),
		Text:   getString(data, "text"),
	}

	if attrs, ok := data["attributes"].([]interface{}); ok {
		for _, a := range attrs {
			if aMap, ok := a.(map[string]interface{}); ok {
				attr := SmartAttribute{
					ID:     int(getFloat(aMap, "id")),
					Name:   getString(aMap, "name"),
					Value:  int(getFloat(aMap, "value")),
					Worst:  int(getFloat(aMap, "worst")),
					Thresh: int(getFloat(aMap, "thresh")),
					Fail:   getBool(aMap, "fail"), // Uses utils.go implementation
					Raw:    getString(aMap, "raw"),
				}
				smart.Attributes = append(smart.Attributes, attr)
			}
		}
	}

	return smart, nil
}
