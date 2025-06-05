package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devnullvoid/proxmox-tui/pkg/api/interfaces"
)

// Cache TTLs for different types of data
const (
	ClusterDataTTL  = 1 * time.Hour
	NodeDataTTL     = 1 * time.Hour
	VMDataTTL       = 1 * time.Hour
	ResourceDataTTL = 1 * time.Hour
)

// Client is a Proxmox API client with dependency injection for logging and caching
type Client struct {
	httpClient  *HTTPClient
	authManager *AuthManager
	Cluster     *Cluster // Cached cluster state

	// Dependencies
	logger interfaces.Logger
	cache  interfaces.Cache

	// API settings
	baseURL string
	user    string
}

// Get makes a GET request to the Proxmox API with retry logic
func (c *Client) Get(path string, result *map[string]interface{}) error {
	c.logger.Debug("API GET: %s", path)
	return c.httpClient.GetWithRetry(context.Background(), path, result, 3)
}

// GetNoRetry makes a GET request to the Proxmox API without retry logic
func (c *Client) GetNoRetry(path string, result *map[string]interface{}) error {
	c.logger.Debug("API GET (no retry): %s", path)
	return c.httpClient.Get(context.Background(), path, result)
}

// Post makes a POST request to the Proxmox API
func (c *Client) Post(path string, data interface{}) error {
	c.logger.Debug("API POST: %s", path)
	// Convert data to map[string]interface{} if it's not nil
	var postData interface{}
	if data != nil {
		var ok bool
		postData, ok = data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("data must be of type map[string]interface{}")
		}
	}
	return c.httpClient.Post(context.Background(), path, postData, nil)
}

// PostWithResponse makes a POST request to the Proxmox API and returns the response
func (c *Client) PostWithResponse(path string, data interface{}, result *map[string]interface{}) error {
	c.logger.Debug("API POST with response: %s", path)
	// Convert data to map[string]interface{} if it's not nil
	var postData interface{}
	if data != nil {
		var ok bool
		postData, ok = data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("data must be of type map[string]interface{}")
		}
	}
	return c.httpClient.Post(context.Background(), path, postData, result)
}

// IsUsingTokenAuth returns true if the client is using API token authentication
func (c *Client) IsUsingTokenAuth() bool {
	// Check if the auth manager is using token authentication
	// Token auth users have a '!' in their username (e.g., "user@realm!tokenid")
	return c.authManager != nil && c.authManager.IsTokenAuth()
}

// GetWithCache makes a GET request to the Proxmox API with caching
func (c *Client) GetWithCache(path string, result *map[string]interface{}, ttl time.Duration) error {
	// Generate cache key based on API path
	cacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, path)
	cacheKey = strings.ReplaceAll(cacheKey, "/", "_")

	// Try to get from cache first
	var cachedData map[string]interface{}
	found, err := c.cache.Get(cacheKey, &cachedData)
	if err != nil {
		c.logger.Debug("Cache error for %s: %v", path, err)
	} else if found {
		c.logger.Debug("Cache hit for: %s", path)
		if result != nil {
			// Copy the cached data to the result
			*result = make(map[string]interface{}, len(cachedData))
			for k, v := range cachedData {
				(*result)[k] = v
			}
			return nil
		}
	}

	// Cache miss or error, make the API call
	c.logger.Debug("Cache miss for: %s", path)
	err = c.Get(path, result)
	if err != nil {
		return err
	}

	// Cache the result
	if result != nil && *result != nil {
		if err := c.cache.Set(cacheKey, *result, ttl); err != nil {
			c.logger.Debug("Failed to cache API result for %s: %v", path, err)
		} else {
			c.logger.Debug("Cached API result for %s with TTL %v", path, ttl)
		}
	}

	return nil
}

// GetWithRetry makes a GET request with retry logic
func (c *Client) GetWithRetry(path string, result *map[string]interface{}, maxRetries int) error {
	c.logger.Debug("API GET with retry: %s", path)
	return c.httpClient.GetWithRetry(context.Background(), path, result, maxRetries)
}

// Version gets the Proxmox API version
func (c *Client) Version(ctx context.Context) (float64, error) {
	var result map[string]interface{}
	err := c.httpClient.Get(ctx, "/version", &result)
	if err != nil {
		return 0, fmt.Errorf("failed to get version: %w", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid version response format")
	}

	version, ok := data["version"].(string)
	if !ok {
		return 0, fmt.Errorf("version not found in response")
	}

	// Parse version string (e.g., "7.4" -> 7.4)
	var versionFloat float64
	if _, err := fmt.Sscanf(version, "%f", &versionFloat); err != nil {
		return 0, fmt.Errorf("failed to parse version: %w", err)
	}

	return versionFloat, nil
}

// GetVmList gets a list of VMs
func (c *Client) GetVmList(ctx context.Context) ([]map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.httpClient.Get(ctx, "/cluster/resources", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM list: %w", err)
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid VM list response format")
	}

	var vms []map[string]interface{}
	for _, item := range data {
		if vm, ok := item.(map[string]interface{}); ok {
			// Filter for VMs and containers only
			if resType, exists := vm["type"].(string); exists && (resType == VMTypeQemu || resType == VMTypeLXC) {
				vms = append(vms, vm)
			}
		}
	}

	return vms, nil
}

// ClearAPICache removes all API-related cached responses
func (c *Client) ClearAPICache() {
	if err := c.cache.Clear(); err != nil {
		c.logger.Debug("Failed to clear API cache: %v", err)
	} else {
		c.logger.Debug("API cache cleared successfully")
	}
}

// GetFreshClusterStatus retrieves cluster status bypassing cache completely
func (c *Client) GetFreshClusterStatus() (*Cluster, error) {
	// Clear the cache first to ensure fresh data
	c.ClearAPICache()

	// Now get fresh data
	return c.GetClusterStatus()
}

// RefreshNodeData refreshes data for a specific node by clearing its cache entries and fetching fresh data
func (c *Client) RefreshNodeData(nodeName string) (*Node, error) {
	// Clear cache entries for this specific node
	nodeStatusPath := fmt.Sprintf("/nodes/%s/status", nodeName)
	nodeVersionPath := fmt.Sprintf("/nodes/%s/version", nodeName)
	nodeConfigPath := fmt.Sprintf("/nodes/%s/config", nodeName)

	// Generate cache keys and delete them
	statusCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, nodeStatusPath)
	statusCacheKey = strings.ReplaceAll(statusCacheKey, "/", "_")

	versionCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, nodeVersionPath)
	versionCacheKey = strings.ReplaceAll(versionCacheKey, "/", "_")

	configCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, nodeConfigPath)
	configCacheKey = strings.ReplaceAll(configCacheKey, "/", "_")

	// Delete cache entries (ignore errors as they might not exist)
	_ = c.cache.Delete(statusCacheKey)
	_ = c.cache.Delete(versionCacheKey)
	_ = c.cache.Delete(configCacheKey)

	c.logger.Debug("Cleared cache for node %s", nodeName)

	// Get the current node to preserve certain data like VMs and online status
	var originalNode *Node
	if c.Cluster != nil {
		for _, node := range c.Cluster.Nodes {
			if node != nil && node.Name == nodeName {
				originalNode = node
				break
			}
		}
	}

	// Fetch fresh node data
	freshNode, err := c.GetNodeStatus(nodeName)
	if err != nil {
		// If we can't reach the node, it's likely offline
		if originalNode != nil {
			originalNode.Online = false
		}
		return nil, fmt.Errorf("failed to refresh node %s: %w", nodeName, err)
	}

	// If we successfully got node status, the node is online
	freshNode.Online = true

	// Preserve important data from original node if it exists
	if originalNode != nil {
		// Preserve IP address (comes from cluster status, not node status)
		if originalNode.IP != "" {
			freshNode.IP = originalNode.IP
		}

		// Preserve VMs list
		if originalNode.VMs != nil {
			freshNode.VMs = originalNode.VMs
		}

		// Preserve storage info
		if originalNode.Storage != nil {
			freshNode.Storage = originalNode.Storage
		}
	}

	return freshNode, nil
}

// RefreshVMData refreshes data for a specific VM by clearing its cache entries and fetching fresh data
func (c *Client) RefreshVMData(vm *VM) (*VM, error) {
	// Clear cache entries for this specific VM
	statusPath := fmt.Sprintf("/nodes/%s/%s/%d/status/current", vm.Node, vm.Type, vm.ID)
	configPath := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.Node, vm.Type, vm.ID)

	// Generate cache keys and delete them
	statusCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, statusPath)
	statusCacheKey = strings.ReplaceAll(statusCacheKey, "/", "_")

	configCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, configPath)
	configCacheKey = strings.ReplaceAll(configCacheKey, "/", "_")

	// Delete cache entries (ignore errors as they might not exist)
	_ = c.cache.Delete(statusCacheKey)
	_ = c.cache.Delete(configCacheKey)

	// Also clear guest agent related cache entries if it's a QEMU VM
	if vm.Type == VMTypeQemu {
		agentNetPath := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", vm.Node, vm.ID)
		agentFsPath := fmt.Sprintf("/nodes/%s/qemu/%d/agent/get-fsinfo", vm.Node, vm.ID)

		agentNetCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, agentNetPath)
		agentNetCacheKey = strings.ReplaceAll(agentNetCacheKey, "/", "_")

		agentFsCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, agentFsPath)
		agentFsCacheKey = strings.ReplaceAll(agentFsCacheKey, "/", "_")

		_ = c.cache.Delete(agentNetCacheKey)
		_ = c.cache.Delete(agentFsCacheKey)
	} else if vm.Type == VMTypeLXC {
		// Clear LXC interfaces cache
		lxcInterfacesPath := fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", vm.Node, vm.ID)
		lxcInterfacesCacheKey := fmt.Sprintf("proxmox_api_%s_%s", c.baseURL, lxcInterfacesPath)
		lxcInterfacesCacheKey = strings.ReplaceAll(lxcInterfacesCacheKey, "/", "_")

		_ = c.cache.Delete(lxcInterfacesCacheKey)
	}

	c.logger.Debug("Cleared cache for VM %s (%d) on node %s", vm.Name, vm.ID, vm.Node)

	// Fetch fresh VM data using GetDetailedVmInfo for basic information
	freshVM, err := c.GetDetailedVmInfo(vm.Node, vm.Type, vm.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM details: %w", err)
	}

	// Now enrich the VM with guest agent data just like the full refresh does
	// This is what was missing - we need to call GetVmStatus to get the enriched data
	if freshVM.Status == VMStatusRunning {
		// Store the current disk values from GetDetailedVmInfo to preserve them
		diskUsage := freshVM.Disk
		maxDiskUsage := freshVM.MaxDisk

		// Enrich with guest agent data (network interfaces, filesystems, etc.)
		if err := c.GetVmStatus(freshVM); err != nil {
			c.logger.Debug("Failed to enrich VM %s with guest agent data: %v", freshVM.Name, err)
			// Don't return error, just log it - basic VM data is still valid
		}

		// Restore disk usage values from GetDetailedVmInfo if they got overwritten or are zero
		if freshVM.Disk == 0 && diskUsage > 0 {
			freshVM.Disk = diskUsage
		}

		if freshVM.MaxDisk == 0 && maxDiskUsage > 0 {
			freshVM.MaxDisk = maxDiskUsage
		}
	}

	return freshVM, nil
}

// NewClient creates a new Proxmox API client with dependency injection
func NewClient(config interfaces.Config, options ...ClientOption) (*Client, error) {
	// Apply options
	opts := defaultOptions()
	for _, option := range options {
		option(opts)
	}

	// Validate input parameters
	if config.GetAddr() == "" {
		return nil, fmt.Errorf("proxmox address cannot be empty")
	}

	// Construct base URL - remove any API path suffix
	baseURL := strings.TrimRight(config.GetAddr(), "/")
	if !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	// Remove /api2/json suffix if present to get the server base URL
	serverBaseURL := strings.TrimSuffix(baseURL, "/api2/json")

	opts.Logger.Debug("Proxmox server URL: %s", serverBaseURL)
	opts.Logger.Debug("Proxmox API base URL: %s", serverBaseURL+"/api2/json")

	// Configure TLS
	tlsConfig := &tls.Config{InsecureSkipVerify: config.GetInsecure()}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	// Create HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Validate port presence
	if !strings.Contains(serverBaseURL, ":") {
		return nil, fmt.Errorf("missing port in address %s", serverBaseURL)
	}

	// Format credentials with realm
	userWithRealm := fmt.Sprintf("%s@%s", config.GetUser(), config.GetRealm())

	// Create HTTP client wrapper
	httpClientWrapper := NewHTTPClient(httpClient, serverBaseURL+"/api2/json", opts.Logger)

	// Create auth manager
	var authManager *AuthManager
	if config.IsUsingTokenAuth() {
		authManager = NewAuthManagerWithToken(httpClientWrapper, config.GetAPIToken(), opts.Logger)
	} else {
		authManager = NewAuthManagerWithPassword(httpClientWrapper, userWithRealm, config.GetPassword(), opts.Logger)
	}

	// Create client
	client := &Client{
		httpClient:  httpClientWrapper,
		authManager: authManager,
		logger:      opts.Logger,
		cache:       opts.Cache,
		baseURL:     serverBaseURL,
		user:        config.GetUser(),
	}

	// Set auth manager in HTTP client
	httpClientWrapper.SetAuthManager(authManager)

	// Test authentication
	if err := authManager.EnsureAuthenticated(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	opts.Logger.Debug("Proxmox API client initialized successfully")
	return client, nil
}
