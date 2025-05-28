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
			if resType, exists := vm["type"].(string); exists && (resType == "qemu" || resType == "lxc") {
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