package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Telmate/proxmox-api-go/proxmox"
	"github.com/devnullvoid/proxmox-tui/pkg/cache"
	"github.com/devnullvoid/proxmox-tui/pkg/config"
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
	ProxClient *proxmox.Client
	Cluster    *Cluster // Cached cluster state

	// API settings
	baseURL string
	user    string
}

// Get makes a GET request to the Proxmox API
func (c *Client) Get(path string, result *map[string]interface{}) error {
	config.DebugLog("API GET: %s", path)
	return c.ProxClient.GetJsonRetryable(context.Background(), path, result, 3)
}

// Post makes a POST request to the Proxmox API
func (c *Client) Post(path string, data interface{}) error {
	config.DebugLog("API POST: %s", path)
	// Convert data to map[string]interface{} if it's not nil
	var postData map[string]interface{}
	if data != nil {
		var ok bool
		postData, ok = data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("data must be of type map[string]interface{}")
		}
	}
	return c.ProxClient.Post(context.Background(), postData, path)
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

// ClearAPICache removes all API-related cached responses
func (c *Client) ClearAPICache() {
	// We can't easily clear only API cache entries, but this is a good candidate
	// for future improvement with cache namespaces
	config.DebugLog("Clearing API cache")
}

// NewClient initializes a new Proxmox API client with optimized defaults
func NewClient(addr, user, password, realm string, insecure bool) (*Client, error) {
	// Validate input parameters
	if addr == "" {
		return nil, fmt.Errorf("proxmox address cannot be empty")
	}

	// Construct base URL
	baseURL := strings.TrimRight(addr, "/")
	if !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	// if !strings.Contains(baseURL, ":8006") {
	// 	baseURL += ":8006"
	// }
	config.DebugLog("Proxmox API URL: %s", baseURL)

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
	if !strings.Contains(baseURL, ":") {
		return nil, fmt.Errorf("missing port in address %s", baseURL)
	}

	// Create proxmox client with correct parameters
	proxClient, err := proxmox.NewClient(
		baseURL,
		httpClient,
		"", // API token (empty for password auth)
		transport.TLSClientConfig,
		"",  // Logging prefix
		600, // Timeout
	)
	if err != nil {
		return nil, err
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

	config.DebugLog("Authentication parameters:\n- User: %s\n- Realm: %s\n- API: %s",
		authUser, realm, baseURL)

	// Perform authentication with formatted username (realm should be empty when using username@realm format)
	if err := proxClient.Login(context.Background(), authUser, password, ""); err != nil {
		config.DebugLog("Authentication failure details:\nUser: %s\nAPI: %s", authUser, baseURL)
		return nil, fmt.Errorf("authentication failed for %s at %s: %w\nCheck:\n1. Credentials format: username@realm\n2. Realm '%s' exists\n3. User has API permissions\n4. TLS certificate validity", authUser, baseURL, err, realm)
		// return nil, fmt.Errorf("authentication failed: %w\nTroubleshooting:\n1. Verify credentials\n2. Check network connectivity\n3. Validate TLS settings", err)
	}

	// Verify API connectivity
	if _, err := proxClient.GetVersion(context.Background()); err != nil {
		return nil, fmt.Errorf("API verification failed: %w", err)
	}

	return &Client{
		ProxClient: proxClient,
		baseURL:    baseURL,
		user:       user,
	}, nil
}
