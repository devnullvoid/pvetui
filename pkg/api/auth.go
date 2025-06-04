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

// AuthToken represents a Proxmox authentication token
type AuthToken struct {
	Ticket    string    `json:"ticket"`
	CSRFToken string    `json:"csrf_token"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsValid checks if the token is still valid (not expired)
func (t *AuthToken) IsValid() bool {
	return t != nil && t.Ticket != "" && time.Now().Before(t.ExpiresAt)
}

// AuthManager handles Proxmox API authentication with dependency injection
type AuthManager struct {
	httpClient *HTTPClient
	username   string
	password   string
	token      string // For API token authentication
	authToken  *AuthToken
	logger     interfaces.Logger
	mu         sync.RWMutex
}

// NewAuthManagerWithPassword creates a new authentication manager for password auth
func NewAuthManagerWithPassword(httpClient *HTTPClient, username, password string, logger interfaces.Logger) *AuthManager {
	return &AuthManager{
		httpClient: httpClient,
		username:   username,
		password:   password,
		logger:     logger,
	}
}

// NewAuthManagerWithToken creates a new authentication manager for token auth
func NewAuthManagerWithToken(httpClient *HTTPClient, token string, logger interfaces.Logger) *AuthManager {
	return &AuthManager{
		httpClient: httpClient,
		token:      token,
		logger:     logger,
	}
}

// EnsureAuthenticated ensures the client is authenticated
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

// GetValidToken returns a valid authentication token, refreshing if necessary
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

// authenticate performs the authentication flow with Proxmox API
func (am *AuthManager) authenticate(ctx context.Context) (*AuthToken, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Double-check after acquiring write lock
	if am.authToken != nil && am.authToken.IsValid() {
		return am.authToken, nil
	}

	am.logger.Debug("Authenticating with Proxmox API: %s", am.username)

	// Prepare authentication request
	authURL := "/access/ticket"
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

// ClearToken clears the cached authentication token
func (am *AuthManager) ClearToken() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.authToken = nil
	am.logger.Debug("Authentication token cleared")
}

// IsTokenAuth returns true if using API token authentication
func (am *AuthManager) IsTokenAuth() bool {
	return am.token != ""
}
