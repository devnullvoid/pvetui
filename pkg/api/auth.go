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

	"github.com/devnullvoid/proxmox-tui/pkg/config"
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

// AuthManager handles Proxmox API authentication
type AuthManager struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	token      *AuthToken
	mu         sync.RWMutex
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(baseURL, username, password string, httpClient *http.Client) *AuthManager {
	return &AuthManager{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		httpClient: httpClient,
	}
}

// GetValidToken returns a valid authentication token, refreshing if necessary
func (am *AuthManager) GetValidToken(ctx context.Context) (*AuthToken, error) {
	am.mu.RLock()
	if am.token != nil && am.token.IsValid() {
		token := am.token
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
	if am.token != nil && am.token.IsValid() {
		return am.token, nil
	}

	config.DebugLog("Authenticating with Proxmox API: %s", am.username)

	// Prepare authentication request
	authURL := am.baseURL + "/api2/json/access/ticket"
	config.DebugLog("Authentication URL: %s", authURL)

	// Create form data
	formData := url.Values{}
	formData.Set("username", am.username)
	formData.Set("password", am.password)
	config.DebugLog("Form data: username=%s, password=<hidden>", am.username)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create authentication request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "proxmox-tui")

	config.DebugLog("Sending authentication request to: %s", authURL)

	// Execute request
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	config.DebugLog("Authentication response status: %d %s", resp.StatusCode, resp.Status)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read response body for better error details
		body, _ := io.ReadAll(resp.Body)
		config.DebugLog("Authentication failed response body: %s", string(body))
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

	am.token = token
	config.DebugLog("Authentication successful for user: %s", token.Username)

	return token, nil
}

// ClearToken clears the cached authentication token
func (am *AuthManager) ClearToken() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.token = nil
	config.DebugLog("Authentication token cleared")
}
