package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	// "github.com/devnullvoid/proxmox-tui/pkg/config"
)

// Ensure config package is properly imported

// CPUInfo contains detailed CPU information from Proxmox node status
type CPUInfo struct {
	Cores   int    `json:"cores"`
	Cpus    int    `json:"cpus"`
	Model   string `json:"model"`
	Sockets int    `json:"sockets"`
}

// Node represents a Proxmox cluster node
type Node struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	IP            string   `json:"ip"`
	CPUCount      float64  `json:"cpus"`
	CPUUsage      float64  `json:"cpu"`
	MemoryTotal   float64  `json:"memory_total"`
	MemoryUsed    float64  `json:"memory_used"`
	TotalStorage  int64    `json:"rootfs_total"`
	UsedStorage   int64    `json:"rootfs_used"`
	Uptime        int64    `json:"uptime"`
	Version       string   `json:"pveversion"`
	KernelVersion string   `json:"kversion"`
	Online        bool     `json:"-"`
	CGroupMode    int      `json:"cgroup_mode,omitempty"`
	Level         string   `json:"level,omitempty"`
	Storage       *Storage `json:"storage,omitempty"`
	VMs           []*VM    `json:"vms,omitempty"`
	CPUInfo       *CPUInfo `json:"cpuinfo,omitempty"`
	LoadAvg       []string `json:"loadavg,omitempty"`

	// For metrics tracking and concurrency
	// mu                sync.RWMutex `json:"-"`
	lastMetricsUpdate time.Time `json:"-"`
	// metricsTTL        time.Duration `json:"-"`
	// lastCPUUsage      float64       `json:"-"`
	// lastMemoryUsage   float64       `json:"-"`
	// lastLoadAvg       []string      `json:"-"`
}

// ListNodes retrieves nodes from cached cluster data
func (c *Client) ListNodes() ([]Node, error) {
	if c.Cluster == nil {
		if _, err := c.GetClusterStatus(); err != nil {
			return nil, err
		}
	}

	nodes := make([]Node, 0, len(c.Cluster.Nodes))
	for _, clusterNode := range c.Cluster.Nodes {
		if clusterNode != nil {
			nodes = append(nodes, *clusterNode)
		}
	}
	return nodes, nil
}

// GetNodeStatus retrieves real-time status for a specific node
func (c *Client) GetNodeStatus(nodeName string) (*Node, error) {
	var res map[string]interface{}

	if err := c.GetWithCache(fmt.Sprintf("/nodes/%s/status", nodeName), &res, NodeDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get status for node %s: %w", nodeName, err)
	}

	// config.DebugLog("[DEBUG] Raw node status response: %+v", res) // Log raw API response

	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid status response format for node %s", nodeName)
	}

	// config.DebugLog("[DEBUG] Parsed node status data: %+v", data) // Log parsed data structure

	node := &Node{
		Name:          nodeName,
		Online:        strings.EqualFold(getString(data, "status"), "online"),
		CPUUsage:      getFloat(data, "cpu"),
		KernelVersion: getString(data, "kversion"),
		Version:       getString(data, "pveversion"),
	}

	// Get CPU count from cpuinfo
	if cpuinfo, ok := data["cpuinfo"].(map[string]interface{}); ok {
		node.CPUCount = getFloat(cpuinfo, "cpus")
	}

	// Get memory stats
	if memory, ok := data["memory"].(map[string]interface{}); ok {
		node.MemoryTotal = getFloat(memory, "total") / 1073741824
		node.MemoryUsed = getFloat(memory, "used") / 1073741824
	}

	// Get storage stats
	if rootfs, ok := data["rootfs"].(map[string]interface{}); ok {
		node.TotalStorage = int64(getFloat(rootfs, "total"))
		node.UsedStorage = int64(getFloat(rootfs, "used"))
	}

	// Get uptime
	node.Uptime = int64(getFloat(data, "uptime"))

	// Parse CPU info with safe type conversion
	if cpuinfoData, ok := data["cpuinfo"].(map[string]interface{}); ok {
		cpuInfo := &CPUInfo{}
		if cores, ok := cpuinfoData["cores"].(float64); ok {
			cpuInfo.Cores = int(cores)
		}
		if cpus, ok := cpuinfoData["cpus"].(float64); ok {
			cpuInfo.Cpus = int(cpus)
		}
		if model, ok := cpuinfoData["model"].(string); ok {
			cpuInfo.Model = model
		}
		if sockets, ok := cpuinfoData["sockets"].(float64); ok {
			cpuInfo.Sockets = int(sockets)
		}
		node.CPUInfo = cpuInfo
	}

	// Parse load averages with type conversion
	if loadavg, ok := data["loadavg"].([]interface{}); ok {
		node.LoadAvg = make([]string, 0, len(loadavg))
		for _, val := range loadavg {
			// Convert numeric values to strings if needed
			switch v := val.(type) {
			case string:
				node.LoadAvg = append(node.LoadAvg, v)
			case float64:
				node.LoadAvg = append(node.LoadAvg, fmt.Sprintf("%.2f", v))
			default:
				node.LoadAvg = append(node.LoadAvg, fmt.Sprintf("%v", v))
			}
		}
	}

	// Fallback to version endpoint if pveversion not in status
	if node.Version == "" {
		var versionRes map[string]interface{}

		if err := c.GetWithCache(fmt.Sprintf("/nodes/%s/version", nodeName), &versionRes, NodeDataTTL); err == nil {
			if versionData, ok := versionRes["data"].(map[string]interface{}); ok {
				node.Version = getString(versionData, "version")
			}
		}
	}

	return node, nil
}

// GetNodeConfig retrieves configuration for a given node with caching
func (c *Client) GetNodeConfig(nodeName string) (map[string]interface{}, error) {
	var res map[string]interface{}
	if err := c.GetWithCache(fmt.Sprintf("/nodes/%s/config", nodeName), &res, NodeDataTTL); err != nil {
		return nil, fmt.Errorf("failed to get node config: %w", err)
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format for node config")
	}
	return data, nil
}

// GetNodeVNCShell creates a VNC shell connection for a node and returns connection details
func (c *Client) GetNodeVNCShell(nodeName string) (*VNCProxyResponse, error) {
	// Node VNC shells don't work with API token authentication
	if c.IsUsingTokenAuth() {
		return nil, fmt.Errorf("node VNC shells are not supported with API token authentication, please use password authentication")
	}

	var res map[string]interface{}
	path := fmt.Sprintf("/nodes/%s/vncshell", nodeName)
	
	// POST request with websocket=1 parameter for noVNC compatibility
	data := map[string]interface{}{
		"websocket": 1,
	}
	
	if err := c.PostWithResponse(path, data, &res); err != nil {
		return nil, fmt.Errorf("failed to create VNC shell: %w", err)
	}

	responseData, ok := res["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected VNC shell response format")
	}

	response := &VNCProxyResponse{}
	
	if ticket, ok := responseData["ticket"].(string); ok {
		response.Ticket = ticket
	}
	
	if port, ok := responseData["port"].(string); ok {
		response.Port = port
	} else if portFloat, ok := responseData["port"].(float64); ok {
		response.Port = fmt.Sprintf("%.0f", portFloat)
	}
	
	if user, ok := responseData["user"].(string); ok {
		response.User = user
	}
	
	if cert, ok := responseData["cert"].(string); ok {
		response.Cert = cert
	}

	return response, nil
}

// GenerateNodeVNCURL creates a noVNC shell URL for the given node
func (c *Client) GenerateNodeVNCURL(nodeName string) (string, error) {
	// Get VNC shell proxy details
	proxy, err := c.GetNodeVNCShell(nodeName)
	if err != nil {
		return "", err
	}

	// Extract server details from base URL
	serverURL := strings.TrimSuffix(c.baseURL, "/api2/json")
	
	// URL encode the VNC ticket (critical for avoiding 401 errors)
	encodedTicket := url.QueryEscape(proxy.Ticket)
	
	// Build the noVNC shell URL using the working format from the forum post
	// Format: https://server:8006/?console=shell&novnc=1&node=nodename&resize=off&cmd=&vncticket=encoded_ticket
	vncURL := fmt.Sprintf("%s/?console=shell&novnc=1&node=%s&resize=off&cmd=&vncticket=%s",
		serverURL, nodeName, encodedTicket)

	return vncURL, nil
}
