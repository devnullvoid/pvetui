package api

import (
	"fmt"
	"strings"
	"time"
)

// StorageContentItem represents one entry returned by the Proxmox storage content API.
type StorageContentItem struct {
	VolID     string    `json:"volid"`
	Content   string    `json:"content"`
	Format    string    `json:"format"`
	Notes     string    `json:"notes"`
	Parent    string    `json:"parent"`
	Size      int64     `json:"size"`
	Used      int64     `json:"used"`
	VMID      int       `json:"vmid"`
	CreatedAt time.Time `json:"ctime"`
	Protected bool      `json:"protected"`
}

// GetStorageContent retrieves storage content entries for a given node/storage pair.
// When contentType is empty, all content types are returned.
func (c *Client) GetStorageContent(nodeName, storageName, contentType string) ([]StorageContentItem, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", nodeName, storageName)
	if strings.TrimSpace(contentType) != "" {
		path += "?content=" + strings.TrimSpace(contentType)
	}

	var res map[string]interface{}
	if err := c.GetWithCache(path, &res, NodeDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get storage content for %s/%s: %w", nodeName, storageName, err)
	}

	data, ok := res["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid storage content response format")
	}

	items := make([]StorageContentItem, 0, len(data))
	for _, raw := range data {
		itemMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		item := StorageContentItem{
			VolID:   getString(itemMap, "volid"),
			Content: getString(itemMap, "content"),
			Format:  getString(itemMap, "format"),
			Notes:   getString(itemMap, "notes"),
			Parent:  getString(itemMap, "parent"),
			Size:    int64(getFloat(itemMap, "size")),
			Used:    int64(getFloat(itemMap, "used")),
			VMID:    getInt(itemMap, "vmid"),
		}

		if protected, ok := itemMap["protected"].(bool); ok {
			item.Protected = protected
		}

		if ctime, ok := itemMap["ctime"].(float64); ok && ctime > 0 {
			item.CreatedAt = time.Unix(int64(ctime), 0)
		}

		items = append(items, item)
	}

	return items, nil
}
