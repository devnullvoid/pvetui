package api

import (
	"fmt"
	"strconv"
	"strings"
)

// VMCreateOptions contains the initial field set for creating a QEMU VM.
type VMCreateOptions struct {
	VMID        int
	Name        string
	MemoryMB    int
	Cores       int
	Sockets     int
	DiskStorage string
	DiskSizeGB  int
	ISOVolume   string
	ImportFrom  string
	Bridge      string
	Start       bool
}

// GetNextID returns the next free VMID, optionally validating a requested one.
func (c *Client) GetNextID(requested int) (int, error) {
	path := "/cluster/nextid"
	if requested > 0 {
		path += "?vmid=" + strconv.Itoa(requested)
	}

	var res map[string]interface{}
	if err := c.Get(path, &res); err != nil {
		return 0, fmt.Errorf("failed to get next VMID: %w", err)
	}

	nextID := getInt(res, "data")
	if nextID <= 0 {
		return 0, fmt.Errorf("invalid nextid response format")
	}

	return nextID, nil
}

// CreateVM creates a QEMU VM and returns the queued task UPID.
func (c *Client) CreateVM(nodeName string, options VMCreateOptions) (string, error) {
	if strings.TrimSpace(nodeName) == "" {
		return "", fmt.Errorf("node name is required")
	}
	if options.VMID <= 0 {
		return "", fmt.Errorf("vmid must be positive")
	}
	if strings.TrimSpace(options.Name) == "" {
		return "", fmt.Errorf("vm name is required")
	}
	if strings.TrimSpace(options.DiskStorage) == "" {
		return "", fmt.Errorf("disk storage is required")
	}
	if strings.TrimSpace(options.ImportFrom) == "" && options.DiskSizeGB <= 0 {
		return "", fmt.Errorf("disk size must be positive")
	}

	if options.MemoryMB <= 0 {
		options.MemoryMB = 2048
	}
	if options.Cores <= 0 {
		options.Cores = 2
	}
	if options.Sockets <= 0 {
		options.Sockets = 1
	}
	if strings.TrimSpace(options.Bridge) == "" {
		options.Bridge = "vmbr0"
	}

	data := map[string]interface{}{
		"vmid":    options.VMID,
		"name":    strings.TrimSpace(options.Name),
		"memory":  strconv.Itoa(options.MemoryMB),
		"cores":   options.Cores,
		"sockets": options.Sockets,
		"ostype":  "l26",
		"net0":    fmt.Sprintf("virtio,bridge=%s", strings.TrimSpace(options.Bridge)),
	}
	if strings.TrimSpace(options.ImportFrom) != "" {
		data["scsi0"] = fmt.Sprintf("%s:0,import-from=%s",
			strings.TrimSpace(options.DiskStorage),
			strings.TrimSpace(options.ImportFrom),
		)
	} else {
		data["scsi0"] = fmt.Sprintf("%s:%d", strings.TrimSpace(options.DiskStorage), options.DiskSizeGB)
	}
	if strings.TrimSpace(options.ISOVolume) != "" {
		data["cdrom"] = strings.TrimSpace(options.ISOVolume)
		data["boot"] = "order=ide2;scsi0;net0"
	} else if strings.TrimSpace(options.ImportFrom) != "" {
		data["boot"] = "order=scsi0;net0"
	}
	if options.Start {
		data["start"] = true
	}

	var res map[string]interface{}
	if err := c.PostWithResponse(fmt.Sprintf("/nodes/%s/qemu", nodeName), data, &res); err != nil {
		return "", fmt.Errorf("failed to create VM on node %s: %w", nodeName, err)
	}
	if errMsg, ok := res["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("vm create failed: %s", errMsg)
	}

	upid, ok := res["data"].(string)
	if !ok || !strings.HasPrefix(upid, "UPID:") {
		return "", fmt.Errorf("failed to get create task ID")
	}

	c.ClearAPICache()

	return upid, nil
}
