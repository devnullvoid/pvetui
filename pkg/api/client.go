package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devnullvoid/proxmox-tui/internal/cache"
	"github.com/devnullvoid/proxmox-tui/internal/config"
)

// Cache TTLs for different types of data
const (
	ClusterDataTTL  = 1 * time.Hour
	NodeDataTTL     = 1 * time.Hour
	VMDataTTL       = 1 * time.Hour
	ResourceDataTTL = 1 * time.Hour
)

// Client is a Proxmox API client with caching capabilities
type Client struct {
	httpClient  *HTTPClient
	authManager *AuthManager
	Cluster     *Cluster // Cached cluster state

	// API settings
	baseURL string
	user    string
}

// Get makes a GET request to the Proxmox API with retry logic
func (c *Client) Get(path string, result *map[string]interface{}) error {
	config.DebugLog("API GET: %s", path)
	return c.httpClient.GetWithRetry(context.Background(), path, result, 3)
}

// GetNoRetry makes a GET request to the Proxmox API without retry logic
func (c *Client) GetNoRetry(path string, result *map[string]interface{}) error {
	config.DebugLog("API GET (no retry): %s", path)
	return c.httpClient.Get(context.Background(), path, result)
}

// Post makes a POST request to the Proxmox API
func (c *Client) Post(path string, data interface{}) error {
	config.DebugLog("API POST: %s", path)
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

	// Get the global cache
	globalCache := cache.GetGlobalCache()

	// Try to get from cache first
	var cachedData map[string]interface{}
	found, err := globalCache.Get(cacheKey, &cachedData)
	if err != nil {
		config.DebugLog("Cache error for %s: %v", path, err)
	} else if found {
		config.DebugLog("Cache hit for: %s", path)
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
	config.DebugLog("Cache miss for: %s", path)
	err = c.Get(path, result)
	if err != nil {
		return err
	}

	// Cache the result
	if result != nil && *result != nil {
		if err := globalCache.Set(cacheKey, *result, ttl); err != nil {
			config.DebugLog("Failed to cache API result for %s: %v", path, err)
		} else {
			config.DebugLog("Cached API result for %s with TTL %v", path, ttl)
		}
	}

	return nil
}

// GetWithRetry makes a GET request with retry logic (replaces GetJsonRetryable)
func (c *Client) GetWithRetry(path string, result *map[string]interface{}, maxRetries int) error {
	config.DebugLog("API GET with retry: %s", path)
	return c.httpClient.GetWithRetry(context.Background(), path, result, maxRetries)
}

// Version gets the Proxmox API version (replaces ProxClient.GetVersion)
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

// GetVmList gets a list of VMs (replaces ProxClient.GetVmList)
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
	globalCache := cache.GetGlobalCache()
	if err := globalCache.Clear(); err != nil {
		config.DebugLog("Failed to clear API cache: %v", err)
	} else {
		config.DebugLog("API cache cleared successfully")
	}
}

// GetFreshClusterStatus retrieves cluster status bypassing cache completely
func (c *Client) GetFreshClusterStatus() (*Cluster, error) {
	// Clear the cache first to ensure fresh data
	c.ClearAPICache()

	// Now get fresh data
	return c.GetClusterStatus()
}

// NewClient initializes a new Proxmox API client with optimized defaults
func NewClient(addr, user, password, realm string, insecure bool) (*Client, error) {
	// Validate input parameters
	if addr == "" {
		return nil, fmt.Errorf("proxmox address cannot be empty")
	}

	// Construct base URL - remove any API path suffix
	baseURL := strings.TrimRight(addr, "/")
	if !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	// Remove /api2/json suffix if present to get the server base URL
	serverBaseURL := strings.TrimSuffix(baseURL, "/api2/json")

	config.DebugLog("Proxmox server URL: %s", serverBaseURL)
	config.DebugLog("Proxmox API base URL: %s", serverBaseURL+"/api2/json")

	// Configure TLS
	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}
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
	if realm == "" {
		realm = "pam" // Default to pam authentication realm
	}

	// Construct proper proxmox username format
	authUser := user
	if !strings.Contains(authUser, "@") {
		authUser = fmt.Sprintf("%s@%s", user, realm)
	}

	config.DebugLog("Authentication parameters:\n- User: %s\n- Realm: %s\n- Server: %s",
		authUser, realm, serverBaseURL)

	// Create authentication manager with server base URL (no /api2/json)
	authManager := NewAuthManager(serverBaseURL, authUser, password, httpClient)

	// Create HTTP client wrapper with full API base URL
	apiBaseURL := serverBaseURL + "/api2/json"
	httpClientWrapper := NewHTTPClient(httpClient, authManager, apiBaseURL, "")

	// Create the main client
	client := &Client{
		httpClient:  httpClientWrapper,
		authManager: authManager,
		baseURL:     apiBaseURL,
		user:        user,
	}

	// Test authentication by getting API version
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := client.Version(ctx)
	if err != nil {
		config.DebugLog("Authentication failure details:\nUser: %s\nServer: %s", authUser, serverBaseURL)
		return nil, fmt.Errorf("authentication failed for %s at %s: %w\nCheck:\n1. Credentials format: username@realm\n2. Realm '%s' exists\n3. User has API permissions\n4. TLS certificate validity", authUser, serverBaseURL, err, realm)
	}

	config.DebugLog("Successfully authenticated with Proxmox API version %.2f", version)

	return client, nil
}

// NewClientFromConfig initializes a new Proxmox API client from a config object
// Supports both password and API token authentication
func NewClientFromConfig(cfg interface{}) (*Client, error) {
	// Use reflection or type assertion to get config values
	// For now, let's assume we have a method to get the values we need
	var addr, user, password, realm, tokenID, tokenSecret string
	var insecure bool

	// Type assertion to get config values - this assumes the config has these methods
	if configObj, ok := cfg.(interface {
		GetAddr() string
		GetUser() string
		GetPassword() string
		GetRealm() string
		GetTokenID() string
		GetTokenSecret() string
		GetInsecure() bool
		IsUsingTokenAuth() bool
		GetAPIToken() string
	}); ok {
		addr = configObj.GetAddr()
		user = configObj.GetUser()
		password = configObj.GetPassword()
		realm = configObj.GetRealm()
		tokenID = configObj.GetTokenID()
		tokenSecret = configObj.GetTokenSecret()
		insecure = configObj.GetInsecure()
	} else {
		return nil, fmt.Errorf("invalid config object type")
	}

	// Validate input parameters
	if addr == "" {
		return nil, fmt.Errorf("proxmox address cannot be empty")
	}
	if user == "" {
		return nil, fmt.Errorf("proxmox username cannot be empty")
	}

	// Check authentication method
	hasPassword := password != ""
	hasToken := tokenID != "" && tokenSecret != ""

	if !hasPassword && !hasToken {
		return nil, fmt.Errorf("authentication required: provide either password or API token")
	}
	if hasPassword && hasToken {
		return nil, fmt.Errorf("conflicting authentication methods: provide either password or API token, not both")
	}

	// Construct base URL - remove any API path suffix
	baseURL := strings.TrimRight(addr, "/")
	if !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	// Remove /api2/json suffix if present to get the server base URL
	serverBaseURL := strings.TrimSuffix(baseURL, "/api2/json")

	config.DebugLog("Proxmox server URL: %s", serverBaseURL)
	config.DebugLog("Proxmox API base URL: %s", serverBaseURL+"/api2/json")

	// Configure TLS
	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}
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
	if realm == "" {
		realm = "pam" // Default to pam authentication realm
	}

	var authManager *AuthManager
	var apiToken string

	if hasToken {
		// API Token authentication
		apiToken = fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s", user, realm, tokenID, tokenSecret)
		config.DebugLog("Authentication method: API Token\n- User: %s@%s\n- Token ID: %s\n- Server: %s",
			user, realm, tokenID, serverBaseURL)
	} else {
		// Password authentication
		authUser := user
		if !strings.Contains(authUser, "@") {
			authUser = fmt.Sprintf("%s@%s", user, realm)
		}
		config.DebugLog("Authentication method: Password\n- User: %s\n- Realm: %s\n- Server: %s",
			authUser, realm, serverBaseURL)

		// Create authentication manager with server base URL (no /api2/json)
		authManager = NewAuthManager(serverBaseURL, authUser, password, httpClient)
	}

	// Create HTTP client wrapper with full API base URL
	apiBaseURL := serverBaseURL + "/api2/json"
	httpClientWrapper := NewHTTPClient(httpClient, authManager, apiBaseURL, apiToken)

	// Create the main client
	client := &Client{
		httpClient:  httpClientWrapper,
		authManager: authManager,
		baseURL:     apiBaseURL,
		user:        user,
	}

	// Test authentication by getting API version
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := client.Version(ctx)
	if err != nil {
		if hasToken {
			config.DebugLog("API token authentication failure:\nUser: %s@%s\nToken ID: %s\nServer: %s", user, realm, tokenID, serverBaseURL)
			return nil, fmt.Errorf("API token authentication failed for %s@%s at %s: %w\nCheck:\n1. Token ID '%s' exists\n2. Token secret is correct\n3. Token has required permissions\n4. Token is not expired", user, realm, serverBaseURL, err, tokenID)
		} else {
			config.DebugLog("Password authentication failure:\nUser: %s@%s\nServer: %s", user, realm, serverBaseURL)
			return nil, fmt.Errorf("password authentication failed for %s@%s at %s: %w\nCheck:\n1. Credentials format: username@realm\n2. Realm '%s' exists\n3. User has API permissions\n4. TLS certificate validity", user, realm, serverBaseURL, err, realm)
		}
	}

	if hasToken {
		config.DebugLog("Successfully authenticated with API token. Proxmox API version %.2f", version)
	} else {
		config.DebugLog("Successfully authenticated with password. Proxmox API version %.2f", version)
	}

	return client, nil
}
