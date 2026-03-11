package api

import (
	"fmt"
	"net/url"
	"strconv"
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

// DeleteStorageContent deletes a storage content item and returns the queued task UPID.
func (c *Client) DeleteStorageContent(nodeName, storageName, volID string) (string, error) {
	if strings.TrimSpace(nodeName) == "" {
		return "", fmt.Errorf("node name is required")
	}
	if strings.TrimSpace(storageName) == "" {
		return "", fmt.Errorf("storage name is required")
	}
	if strings.TrimSpace(volID) == "" {
		return "", fmt.Errorf("volume ID is required")
	}

	path := fmt.Sprintf(
		"/nodes/%s/storage/%s/content/%s",
		nodeName,
		storageName,
		url.PathEscape(strings.TrimSpace(volID)),
	)

	var res map[string]interface{}
	if err := c.DeleteWithResponse(path, &res); err != nil {
		return "", fmt.Errorf("failed to delete storage content %s on %s/%s: %w", volID, nodeName, storageName, err)
	}

	if errMsg, ok := res["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("storage content delete failed: %s", errMsg)
	}

	upid, ok := res["data"].(string)
	if !ok || !strings.HasPrefix(upid, "UPID:") {
		return "", fmt.Errorf("failed to get delete task ID")
	}

	c.ClearAPICache()

	return upid, nil
}

// RestoreGuestFromBackup restores a VM or container from a backup volume and returns the queued task UPID.
func (c *Client) RestoreGuestFromBackup(nodeName, guestType string, vmid int, volID string, force bool) (string, error) {
	if strings.TrimSpace(nodeName) == "" {
		return "", fmt.Errorf("node name is required")
	}
	if vmid <= 0 {
		return "", fmt.Errorf("vmid must be positive")
	}
	if strings.TrimSpace(volID) == "" {
		return "", fmt.Errorf("volume ID is required")
	}

	guestType = strings.TrimSpace(strings.ToLower(guestType))
	path := fmt.Sprintf("/nodes/%s/%s", nodeName, guestType)
	data := map[string]interface{}{
		"vmid": strconv.Itoa(vmid),
	}

	switch guestType {
	case VMTypeQemu:
		data["archive"] = volID
	case VMTypeLXC:
		data["ostemplate"] = volID
		data["restore"] = "1"
	default:
		return "", fmt.Errorf("unsupported guest type: %s", guestType)
	}

	if force {
		data["force"] = "1"
	}

	var res map[string]interface{}
	if err := c.PostWithResponse(path, data, &res); err != nil {
		return "", fmt.Errorf("failed to restore backup %s to %s %d on node %s: %w", volID, guestType, vmid, nodeName, err)
	}

	if errMsg, ok := res["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("restore failed: %s", errMsg)
	}

	upid, ok := res["data"].(string)
	if !ok || !strings.HasPrefix(upid, "UPID:") {
		return "", fmt.Errorf("failed to get restore task ID")
	}

	c.ClearAPICache()

	return upid, nil
}
