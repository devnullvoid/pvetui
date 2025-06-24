package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devnullvoid/proxmox-tui/pkg/api/interfaces"
)

// HTTPClient wraps http.Client with Proxmox-specific functionality and dependency injection
type HTTPClient struct {
	client      *http.Client
	authManager *AuthManager
	baseURL     string
	apiToken    string // For API token authentication
	logger      interfaces.Logger
}

// NewHTTPClient creates a new Proxmox HTTP client with dependency injection
func NewHTTPClient(httpClient *http.Client, baseURL string, logger interfaces.Logger) *HTTPClient {
	return &HTTPClient{
		client:  httpClient,
		baseURL: baseURL,
		logger:  logger,
	}
}

// SetAuthManager sets the auth manager for the HTTP client
func (hc *HTTPClient) SetAuthManager(authManager *AuthManager) {
	hc.authManager = authManager
}

// SetAPIToken sets the API token for authentication
func (hc *HTTPClient) SetAPIToken(token string) {
	hc.apiToken = token
}

// Get performs a GET request to the Proxmox API
func (hc *HTTPClient) Get(ctx context.Context, path string, result *map[string]interface{}) error {
	return hc.doRequest(ctx, "GET", path, nil, result)
}

// Post performs a POST request to the Proxmox API
func (hc *HTTPClient) Post(ctx context.Context, path string, data interface{}, result *map[string]interface{}) error {
	return hc.doRequest(ctx, "POST", path, data, result)
}

// Put performs a PUT request to the Proxmox API
func (hc *HTTPClient) Put(ctx context.Context, path string, data interface{}, result *map[string]interface{}) error {
	return hc.doRequest(ctx, "PUT", path, data, result)
}

// Delete performs a DELETE request to the Proxmox API
func (hc *HTTPClient) Delete(ctx context.Context, path string, result *map[string]interface{}) error {
	return hc.doRequest(ctx, "DELETE", path, nil, result)
}

// GetWithRetry performs a GET request with retry logic
func (hc *HTTPClient) GetWithRetry(ctx context.Context, path string, result *map[string]interface{}, maxRetries int) error {
	return hc.doRequestWithRetry(ctx, "GET", path, nil, result, maxRetries)
}

// doRequest performs an HTTP request with proper authentication
func (hc *HTTPClient) doRequest(ctx context.Context, method, path string, data interface{}, result *map[string]interface{}) error {
	return hc.doRequestWithRetry(ctx, method, path, data, result, 1)
}

// doRequestWithRetry performs an HTTP request with retry logic
func (hc *HTTPClient) doRequestWithRetry(ctx context.Context, method, path string, data interface{}, result *map[string]interface{}, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			// Exponential backoff
			backoff := time.Duration(attempt-1) * 500 * time.Millisecond
			hc.logger.Debug("Retrying request after %v (attempt %d/%d)", backoff, attempt, maxRetries)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := hc.executeRequest(ctx, method, path, data, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !hc.shouldRetry(err, attempt, maxRetries) {
			break
		}

		hc.logger.Debug("Request failed, will retry: %v", err)
	}

	return fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}

// executeRequest performs a single HTTP request
func (hc *HTTPClient) executeRequest(ctx context.Context, method, path string, data interface{}, result *map[string]interface{}) error {
	// Construct full URL
	fullURL := hc.baseURL + path
	if !strings.HasPrefix(path, "/") {
		fullURL = hc.baseURL + "/" + path
	}

	// Prepare request body
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal request data: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "proxmox-tui")
	req.Header.Set("Accept", "application/json")

	// Handle authentication
	if hc.apiToken != "" {
		// Use API token authentication
		req.Header.Set("Authorization", hc.apiToken)
		hc.logger.Debug("Using API token authentication")
	} else if hc.authManager != nil {
		// Use ticket-based authentication
		token, authErr := hc.authManager.GetValidToken(ctx)
		if authErr != nil {
			return fmt.Errorf("authentication failed: %w", authErr)
		}

		// Set authentication cookie
		req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", token.Ticket))

		// Set CSRF token for write operations
		if method == HTTPMethodPOST || method == HTTPMethodPUT || method == HTTPMethodDELETE {
			if token.CSRFToken != "" {
				req.Header.Set("CSRFPreventionToken", token.CSRFToken)
			}
		}
		hc.logger.Debug("Using ticket-based authentication")
	}

	// Set content type for write operations
	if (method == HTTPMethodPOST || method == HTTPMethodPUT || method == HTTPMethodDELETE) && data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	hc.logger.Debug("API %s: %s", method, path)

	// Execute request
	resp, err := hc.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		if hc.apiToken != "" {
			return fmt.Errorf("API token authentication failed: %s", resp.Status)
		} else if hc.authManager != nil {
			hc.logger.Debug("Authentication token expired, clearing cache")
			hc.authManager.ClearToken()
			return fmt.Errorf("authentication failed: %s", resp.Status)
		}
	}

	// Check for other HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse JSON response if result is provided
	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response JSON: %w", err)
		}
	}

	return nil
}

// shouldRetry determines if a request should be retried
func (hc *HTTPClient) shouldRetry(err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}

	// Retry on network errors, timeouts, and 5xx server errors
	if strings.Contains(err.Error(), "connection") ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "status 5") {
		return true
	}

	return false
}
