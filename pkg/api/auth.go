// Package api provides a comprehensive client library for the Proxmox Virtual Environment API.
//
// This package implements a clean, testable client for interacting with Proxmox VE
// clusters, including authentication, resource management, and monitoring capabilities.
// It supports both password-based and API token authentication methods.
//
// Key Features:
//   - Clean Architecture with dependency injection
//   - Comprehensive authentication support (password + API tokens)
//   - Built-in caching with configurable TTL
//   - Structured logging with configurable levels
//   - Robust error handling and retry logic
//   - Full support for VMs, containers, nodes, and cluster operations
//   - Thread-safe operations with proper concurrency handling
//
// Basic Usage:
//
//	// Create configuration
//	config := &Config{
//		Addr:     "https://pve.example.com:8006",
//		User:     "root",
//		Password: "password",
//		Realm:    "pam",
//	}
//
//	// Create client with optional logger and cache
//	client, err := NewClient(config, WithLogger(logger), WithCache(cache))
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Use the client
//	vms, err := client.GetVmList(context.Background())
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Authentication:
//
// The package supports two authentication methods:
//
// 1. Password Authentication (username/password):
//   - Automatically handles ticket-based authentication
//   - Manages CSRF tokens for write operations
//   - Handles token refresh and expiration
//
// 2. API Token Authentication:
//   - Uses Proxmox API tokens for stateless authentication
//   - No session management required
//   - Recommended for automated/service accounts
//
// Thread Safety:
//
// All client operations are thread-safe and can be used concurrently
// from multiple goroutines. Internal state is protected with appropriate
// synchronization primitives.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/devnullvoid/proxmox-tui/pkg/api/interfaces"
)

// AuthToken represents a Proxmox authentication token containing session information.
//
// This structure holds the authentication ticket, CSRF prevention token, and
// expiration information returned by the Proxmox API during password-based
// authentication. The token is used for subsequent API requests until it expires.
//
// For API token authentication, this structure is used as a container but
// the actual API token string is used directly in Authorization headers.
type AuthToken struct {
	Ticket    string    `json:"ticket"`     // Authentication ticket for session-based auth
	CSRFToken string    `json:"csrf_token"` // CSRF prevention token for write operations
	Username  string    `json:"username"`   // Authenticated username
	ExpiresAt time.Time `json:"expires_at"` // Token expiration time
}

// IsValid checks if the authentication token is still valid and not expired.
//
// Returns true if the token exists, has a non-empty ticket, and the current
// time is before the expiration time. This method is safe to call on nil tokens.
//
// Example usage:
//
//	if token != nil && token.IsValid() {
//		// Use existing token
//	} else {
//		// Need to re-authenticate
//	}
func (t *AuthToken) IsValid() bool {
	return t != nil && t.Ticket != "" && time.Now().Before(t.ExpiresAt)
}

// AuthManager handles Proxmox API authentication with support for both
// password-based and API token authentication methods.
//
// The manager automatically handles:
//   - Token caching and refresh for password authentication
//   - CSRF token management for write operations
//   - Thread-safe access to authentication state
//   - Automatic re-authentication when tokens expire
//
// For password authentication, the manager maintains an internal authentication
// token that is refreshed as needed. For API token authentication, the manager
// simply configures the HTTP client with the provided token.
//
// All methods are safe for concurrent use from multiple goroutines.
type AuthManager struct {
	httpClient *HTTPClient       // HTTP client for making authentication requests
	username   string            // Username for password authentication
	password   string            // Password for password authentication
	token      string            // API token for token authentication
	authToken  *AuthToken        // Cached authentication token
	logger     interfaces.Logger // Logger for debugging and monitoring
	mu         sync.RWMutex      // Mutex for thread-safe access
}

// NewAuthManagerWithPassword creates a new authentication manager for password-based authentication.
//
// This method sets up the manager to use username/password authentication with
// automatic ticket management. The manager will authenticate with Proxmox and
// cache the resulting authentication ticket for subsequent requests.
//
// Parameters:
//   - httpClient: HTTP client configured for the Proxmox server
//   - username: Proxmox username (without realm, e.g., "root")
//   - password: User password
//   - logger: Logger for debugging and error reporting
//
// Example usage:
//
//	authManager := NewAuthManagerWithPassword(httpClient, "root", "password", logger)
//	err := authManager.EnsureAuthenticated()
//	if err != nil {
//		log.Fatal("Authentication failed:", err)
//	}
func NewAuthManagerWithPassword(httpClient *HTTPClient, username, password string, logger interfaces.Logger) *AuthManager {
	return &AuthManager{
		httpClient: httpClient,
		username:   username,
		password:   password,
		logger:     logger,
	}
}

// NewAuthManagerWithToken creates a new authentication manager for API token authentication.
//
// This method sets up the manager to use Proxmox API tokens for stateless
// authentication. API tokens don't require session management and are
// recommended for automated systems and service accounts.
//
// Parameters:
//   - httpClient: HTTP client configured for the Proxmox server
//   - token: Complete API token string in Proxmox format (PVEAPIToken=USER@REALM!TOKENID=SECRET)
//   - logger: Logger for debugging and error reporting
//
// Example usage:
//
//	token := "PVEAPIToken=root@pam!mytoken=12345678-1234-1234-1234-123456789abc"
//	authManager := NewAuthManagerWithToken(httpClient, token, logger)
//	err := authManager.EnsureAuthenticated()
//	if err != nil {
//		log.Fatal("Authentication failed:", err)
//	}
func NewAuthManagerWithToken(httpClient *HTTPClient, token string, logger interfaces.Logger) *AuthManager {
	return &AuthManager{
		httpClient: httpClient,
		token:      token,
		logger:     logger,
	}
}

// EnsureAuthenticated ensures the client is properly authenticated and ready for API calls.
//
// For API token authentication, this method configures the HTTP client with the token.
// For password authentication, this method performs the initial authentication if needed.
//
// This method should be called before making any API requests to ensure proper
// authentication state. It's safe to call multiple times.
//
// Returns an error if authentication fails or if the configuration is invalid.
func (am *AuthManager) EnsureAuthenticated() error {
	if am.token != "" {
		// Using API token, no need to authenticate
		am.httpClient.SetAPIToken(am.token)
		return nil
	}

	// Using password authentication, need to get a ticket
	_, err := am.GetValidToken(context.Background())
	return err
}

// GetValidToken returns a valid authentication token, refreshing if necessary.
//
// For API token authentication, this method returns a dummy AuthToken containing
// the API token string. For password authentication, this method returns the
// cached token if valid, or performs re-authentication if the token is expired
// or missing.
//
// This method is thread-safe and handles concurrent access properly. Multiple
// goroutines can call this method simultaneously without issues.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//
// Returns the authentication token or an error if authentication fails.
//
// Example usage:
//
//	token, err := authManager.GetValidToken(ctx)
//	if err != nil {
//		return fmt.Errorf("authentication failed: %w", err)
//	}
//	// Use token for API requests
func (am *AuthManager) GetValidToken(ctx context.Context) (*AuthToken, error) {
	if am.token != "" {
		// Using API token authentication, return a dummy token
		return &AuthToken{
			Ticket:    am.token,
			CSRFToken: "",
			Username:  "api-token",
			ExpiresAt: time.Now().Add(24 * time.Hour), // API tokens don't expire
		}, nil
	}

	am.mu.RLock()
	if am.authToken != nil && am.authToken.IsValid() {
		token := am.authToken
		am.mu.RUnlock()
		return token, nil
	}
	am.mu.RUnlock()

	// Token is invalid or missing, need to authenticate
	return am.authenticate(ctx)
}

// authenticate performs the authentication flow with Proxmox API using username/password.
//
// This method handles the complete authentication process:
//   - Sends POST request to /access/ticket endpoint
//   - Validates the response and extracts authentication data
//   - Creates and caches the AuthToken with proper expiration
//   - Handles concurrent authentication attempts safely
//
// The method uses form-encoded data as required by the Proxmox API and
// automatically sets appropriate headers and user agent.
//
// This is an internal method and should not be called directly. Use
// GetValidToken() or EnsureAuthenticated() instead.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//
// Returns the new authentication token or an error if authentication fails.
func (am *AuthManager) authenticate(ctx context.Context) (*AuthToken, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Double-check after acquiring write lock
	if am.authToken != nil && am.authToken.IsValid() {
		return am.authToken, nil
	}

	am.logger.Debug("Authenticating with Proxmox API: %s", am.username)

	// Prepare authentication request
	authURL := EndpointAccessTicket
	am.logger.Debug("Authentication URL: %s", authURL)

	// Create form data
	formData := url.Values{}
	formData.Set("username", am.username)
	formData.Set("password", am.password)
	am.logger.Debug("Form data: username=%s, password=<hidden>", am.username)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", am.httpClient.baseURL+authURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create authentication request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "proxmox-tui")

	am.logger.Debug("Sending authentication request to: %s", am.httpClient.baseURL+authURL)

	// Execute request
	resp, err := am.httpClient.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	am.logger.Debug("Authentication response status: %d %s", resp.StatusCode, resp.Status)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read response body for better error details
		body, _ := io.ReadAll(resp.Body)
		am.logger.Debug("Authentication failed response body: %s", string(body))
		return nil, fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Parse response
	var authResponse struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
			Username            string `json:"username"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		return nil, fmt.Errorf("failed to parse authentication response: %w", err)
	}

	// Validate response
	if authResponse.Data.Ticket == "" {
		return nil, fmt.Errorf("authentication failed: no ticket received")
	}

	// Create token with 2-hour expiration (Proxmox default)
	token := &AuthToken{
		Ticket:    authResponse.Data.Ticket,
		CSRFToken: authResponse.Data.CSRFPreventionToken,
		Username:  authResponse.Data.Username,
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}

	am.authToken = token
	am.logger.Debug("Authentication successful for user: %s", token.Username)

	return token, nil
}

// ClearToken clears the cached authentication token, forcing re-authentication on next use.
//
// This method is useful when you know the current token is invalid (e.g., after
// receiving 401 responses) or when you want to force a fresh authentication.
// The method is thread-safe and can be called from multiple goroutines.
//
// After calling this method, the next API request will trigger re-authentication.
//
// Example usage:
//
//	// Clear token after receiving 401 error
//	if isUnauthorizedError(err) {
//		authManager.ClearToken()
//		// Next request will re-authenticate
//	}
func (am *AuthManager) ClearToken() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.authToken = nil
	am.logger.Debug("Authentication token cleared")
}

// IsTokenAuth returns true if the authentication manager is configured for API token authentication.
//
// This method helps distinguish between password-based and token-based authentication
// modes, which can be useful for conditional logic or debugging.
//
// Returns true for API token authentication, false for password authentication.
//
// Example usage:
//
//	if authManager.IsTokenAuth() {
//		log.Println("Using API token authentication")
//	} else {
//		log.Println("Using password authentication")
//	}
func (am *AuthManager) IsTokenAuth() bool {
	return am.token != ""
}
