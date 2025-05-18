package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Telmate/proxmox-api-go/proxmox"
	"github.com/devnullvoid/proxmox-tui/pkg/config"
)

// cachedResponse holds the cached API response and its expiration time
type cachedResponse struct {
	data      interface{}
	expiresAt time.Time
}

// Client is a Proxmox API client with caching capabilities
type Client struct {
	ProxClient *proxmox.Client
	Cluster    *Cluster // Cached cluster state

	// Cache-related fields
	cache    map[string]cachedResponse
	cacheMu  sync.RWMutex
	cacheTTL time.Duration
}

// Get makes a GET request to the Proxmox API
func (c *Client) Get(path string, result *map[string]interface{}) error {
	config.DebugLog("API GET: %s", path)
	return c.ProxClient.GetJsonRetryable(context.Background(), path, result, 3)
}

// GetWithCache makes a GET request to the Proxmox API with caching
func (c *Client) GetWithCache(path string, result *map[string]interface{}) error {
	// If cache is not initialized, initialize it
	if c.cache == nil {
		c.cacheMu.Lock()
		if c.cache == nil {
			c.cache = make(map[string]cachedResponse)
			// Default cache TTL of 30 seconds
			if c.cacheTTL == 0 {
				c.cacheTTL = 30 * time.Second
			}
		}
		c.cacheMu.Unlock()
	}

	// Check cache first
	c.cacheMu.RLock()
	cached, exists := c.cache[path]
	c.cacheMu.RUnlock()

	now := time.Now()
	if exists && now.Before(cached.expiresAt) {
		// Cache hit and not expired
		config.DebugLog("Cache hit for: %s", path)
		if result != nil {
			// Deep copy the cached data to the result
			cachedMap, ok := cached.data.(map[string]interface{})
			if ok {
				*result = make(map[string]interface{}, len(cachedMap))
				for k, v := range cachedMap {
					(*result)[k] = v
				}
				return nil
			}
		}
	}

	// Cache miss or expired, make the API call
	config.DebugLog("Cache miss for: %s", path)
	err := c.Get(path, result)
	if err != nil {
		return err
	}

	// Cache the result
	c.cacheMu.Lock()
	c.cache[path] = cachedResponse{
		data:      *result,
		expiresAt: now.Add(c.cacheTTL),
	}
	c.cacheMu.Unlock()

	return nil
}

// SetCacheTTL sets the cache time-to-live duration
func (c *Client) SetCacheTTL(ttl time.Duration) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cacheTTL = ttl
}

// ClearCache removes all cached responses
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache = make(map[string]cachedResponse)
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

	return &Client{ProxClient: proxClient}, nil
}

// TODO: add methods: StartVM, StopVM, etc.
