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

// StorageDownloadURLOptions contains parameters for downloading storage content from a URL.
type StorageDownloadURLOptions struct {
	URL                string
	Content            string
	Filename           string
	Checksum           string
	ChecksumAlgorithm  string
	Compression        string
	VerifyCertificates bool
}

// StorageOCIPullOptions contains parameters for pulling an OCI image into storage.
type StorageOCIPullOptions struct {
	Reference string
	Filename  string
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

// DownloadStorageContentFromURL queues a storage download task from a URL and returns the task UPID.
func (c *Client) DownloadStorageContentFromURL(nodeName, storageName string, options StorageDownloadURLOptions) (string, error) {
	if strings.TrimSpace(nodeName) == "" {
		return "", fmt.Errorf("node name is required")
	}
	if strings.TrimSpace(storageName) == "" {
		return "", fmt.Errorf("storage name is required")
	}
	if strings.TrimSpace(options.URL) == "" {
		return "", fmt.Errorf("url is required")
	}
	if strings.TrimSpace(options.Content) == "" {
		return "", fmt.Errorf("content type is required")
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/download-url", nodeName, storageName)
	data := map[string]interface{}{
		"url":                 strings.TrimSpace(options.URL),
		"content":             strings.TrimSpace(options.Content),
		"verify-certificates": options.VerifyCertificates,
	}
	if strings.TrimSpace(options.Filename) != "" {
		data["filename"] = strings.TrimSpace(options.Filename)
	}
	if strings.TrimSpace(options.Checksum) != "" {
		data["checksum"] = strings.TrimSpace(options.Checksum)
	}
	if strings.TrimSpace(options.ChecksumAlgorithm) != "" {
		data["checksum-algorithm"] = strings.TrimSpace(options.ChecksumAlgorithm)
	}
	if strings.TrimSpace(options.Compression) != "" {
		data["compression"] = strings.TrimSpace(options.Compression)
	}

	var res map[string]interface{}
	if err := c.PostWithResponse(path, data, &res); err != nil {
		return "", fmt.Errorf("failed to download storage content from URL on %s/%s: %w", nodeName, storageName, err)
	}

	if errMsg, ok := res["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("storage download failed: %s", errMsg)
	}

	upid, ok := res["data"].(string)
	if !ok || !strings.HasPrefix(upid, "UPID:") {
		return "", fmt.Errorf("failed to get download task ID")
	}

	c.ClearAPICache()

	return upid, nil
}

// PullStorageOCIImage queues an OCI registry pull task and returns the task UPID.
func (c *Client) PullStorageOCIImage(nodeName, storageName string, options StorageOCIPullOptions) (string, error) {
	if strings.TrimSpace(nodeName) == "" {
		return "", fmt.Errorf("node name is required")
	}
	if strings.TrimSpace(storageName) == "" {
		return "", fmt.Errorf("storage name is required")
	}
	if strings.TrimSpace(options.Reference) == "" {
		return "", fmt.Errorf("reference is required")
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/oci-registry-pull", nodeName, storageName)
	data := map[string]interface{}{
		"reference": strings.TrimSpace(options.Reference),
	}
	if strings.TrimSpace(options.Filename) != "" {
		data["filename"] = strings.TrimSpace(options.Filename)
	}

	var res map[string]interface{}
	if err := c.PostWithResponse(path, data, &res); err != nil {
		return "", fmt.Errorf("failed to pull OCI image on %s/%s: %w", nodeName, storageName, err)
	}

	if errMsg, ok := res["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("OCI pull failed: %s", errMsg)
	}

	upid, ok := res["data"].(string)
	if !ok || !strings.HasPrefix(upid, "UPID:") {
		return "", fmt.Errorf("failed to get OCI pull task ID")
	}

	c.ClearAPICache()

	return upid, nil
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
