package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/proxmox-tui/pkg/api/testutils"
)

// Test constants for repeated strings.
const (
	testTokenValue = "user@realm!tokenid=secret"
	testEndpoint   = "/access/ticket"
)

func TestAuthToken_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		token    *AuthToken
		expected bool
	}{
		{
			name:     "nil token",
			token:    nil,
			expected: false,
		},
		{
			name: "empty ticket",
			token: &AuthToken{
				Ticket:    "",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "expired token",
			token: &AuthToken{
				Ticket:    "valid-ticket",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "valid token",
			token: &AuthToken{
				Ticket:    "valid-ticket",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.token.IsValid())
		})
	}
}

func TestNewAuthManagerWithPassword(t *testing.T) {
	httpClient := &HTTPClient{baseURL: "https://test.example.com"}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	assert.NotNil(t, authManager)
	assert.Equal(t, httpClient, authManager.httpClient)
	assert.Equal(t, "testuser", authManager.username)
	assert.Equal(t, "testpass", authManager.password)
	assert.Empty(t, authManager.token)
	assert.Equal(t, logger, authManager.logger)
	assert.False(t, authManager.IsTokenAuth())
}

func TestNewAuthManagerWithToken(t *testing.T) {
	httpClient := &HTTPClient{baseURL: "https://test.example.com"}
	logger := testutils.NewTestLogger()
	token := testTokenValue

	authManager := NewAuthManagerWithToken(httpClient, token, logger)

	assert.NotNil(t, authManager)
	assert.Equal(t, httpClient, authManager.httpClient)
	assert.Equal(t, token, authManager.token)
	assert.Empty(t, authManager.username)
	assert.Empty(t, authManager.password)
	assert.Equal(t, logger, authManager.logger)
	assert.True(t, authManager.IsTokenAuth())
}

func TestAuthManager_IsTokenAuth(t *testing.T) {
	httpClient := &HTTPClient{baseURL: "https://test.example.com"}
	logger := testutils.NewTestLogger()

	// Test password auth
	passwordAuth := NewAuthManagerWithPassword(httpClient, "user", "pass", logger)
	assert.False(t, passwordAuth.IsTokenAuth())

	// Test token auth
	tokenAuth := NewAuthManagerWithToken(httpClient, "token", logger)
	assert.True(t, tokenAuth.IsTokenAuth())
}

func TestAuthManager_EnsureAuthenticated_WithToken(t *testing.T) {
	// Create a mock HTTP client
	httpClient := &HTTPClient{
		baseURL: "https://test.example.com",
		client:  &http.Client{},
	}
	logger := testutils.NewTestLogger()
	token := testTokenValue

	authManager := NewAuthManagerWithToken(httpClient, token, logger)

	err := authManager.EnsureAuthenticated()
	assert.NoError(t, err)

	// Verify that the HTTP client has the token set
	assert.Equal(t, token, httpClient.apiToken)
}

func TestAuthManager_GetValidToken_WithAPIToken(t *testing.T) {
	httpClient := &HTTPClient{baseURL: "https://test.example.com"}
	logger := testutils.NewTestLogger()
	apiToken := testTokenValue

	authManager := NewAuthManagerWithToken(httpClient, apiToken, logger)

	token, err := authManager.GetValidToken(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, apiToken, token.Ticket)
	assert.Equal(t, "", token.CSRFToken)
	assert.Equal(t, "api-token", token.Username)
	assert.True(t, token.IsValid())
}

func TestAuthManager_GetValidToken_WithCachedToken(t *testing.T) {
	httpClient := &HTTPClient{baseURL: "https://test.example.com"}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "user", "pass", logger)

	// Set a valid cached token
	cachedToken := &AuthToken{
		Ticket:    "cached-ticket",
		CSRFToken: "cached-csrf",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	authManager.authToken = cachedToken

	token, err := authManager.GetValidToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, cachedToken, token)
}

func TestAuthManager_GetValidToken_WithExpiredToken(t *testing.T) {
	// Create a test server that returns a successful authentication response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testEndpoint && r.Method == http.MethodPost {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"ticket":              "new-ticket",
					"CSRFPreventionToken": "new-csrf",
					"username":            "testuser",
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)

			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	// Set an expired cached token
	expiredToken := &AuthToken{
		Ticket:    "expired-ticket",
		CSRFToken: "expired-csrf",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	authManager.authToken = expiredToken

	token, err := authManager.GetValidToken(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, "new-ticket", token.Ticket)
	assert.Equal(t, "new-csrf", token.CSRFToken)
	assert.Equal(t, "testuser", token.Username)
	assert.True(t, token.IsValid())
}

func TestAuthManager_authenticate_Success(t *testing.T) {
	// Create a test server that returns a successful authentication response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/access/ticket" && r.Method == http.MethodPost {
			// Verify the request
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			assert.Equal(t, "proxmox-tui", r.Header.Get("User-Agent"))

			// Parse form data
			err := r.ParseForm()
			require.NoError(t, err)
			assert.Equal(t, "testuser", r.Form.Get("username"))
			assert.Equal(t, "testpass", r.Form.Get("password"))

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"ticket":              "auth-ticket",
					"CSRFPreventionToken": "auth-csrf",
					"username":            "testuser",
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)

			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	token, err := authManager.authenticate(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, "auth-ticket", token.Ticket)
	assert.Equal(t, "auth-csrf", token.CSRFToken)
	assert.Equal(t, "testuser", token.Username)
	assert.True(t, token.IsValid())

	// Verify the token is cached
	assert.Equal(t, token, authManager.authToken)
}

func TestAuthManager_authenticate_HTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testEndpoint {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Authentication failed"))

			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "wrongpass", logger)

	token, err := authManager.authenticate(context.Background())
	assert.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "authentication failed with status 401")
}

func TestAuthManager_authenticate_InvalidJSON(t *testing.T) {
	// Create a test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testEndpoint {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json"))

			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	token, err := authManager.authenticate(context.Background())
	assert.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "failed to parse authentication response")
}

func TestAuthManager_authenticate_NoTicket(t *testing.T) {
	// Create a test server that returns response without ticket
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testEndpoint {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"username": "testuser",
					// Missing ticket
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)

			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	token, err := authManager.authenticate(context.Background())
	assert.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "authentication failed: no ticket received")
}

func TestAuthManager_authenticate_NetworkError(t *testing.T) {
	// Use an invalid URL to simulate network error
	httpClient := &HTTPClient{
		baseURL: "http://invalid-host-that-does-not-exist.local",
		client:  &http.Client{Timeout: 1 * time.Second},
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	token, err := authManager.authenticate(context.Background())
	assert.Error(t, err)
	assert.Nil(t, token)
	// Depending on network environment, the exact error may vary
	assert.NotEmpty(t, err.Error())
}

func TestAuthManager_ClearToken(t *testing.T) {
	httpClient := &HTTPClient{baseURL: "https://test.example.com"}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "user", "pass", logger)

	// Set a token
	authManager.authToken = &AuthToken{
		Ticket:    "test-ticket",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	// Clear the token
	authManager.ClearToken()

	assert.Nil(t, authManager.authToken)
}

func TestAuthManager_ConcurrentAccess(t *testing.T) {
	// Create a test server that simulates slow authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testEndpoint {
			// Add a small delay to test concurrent access
			time.Sleep(10 * time.Millisecond)

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"ticket":              "concurrent-ticket",
					"CSRFPreventionToken": "concurrent-csrf",
					"username":            "testuser",
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)

			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	// Start multiple goroutines trying to authenticate concurrently
	const numGoroutines = 5
	results := make(chan *AuthToken, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			token, err := authManager.GetValidToken(context.Background())
			if err != nil {
				errors <- err
			} else {
				results <- token
			}
		}()
	}

	// Collect results
	var tokens []*AuthToken

	for i := 0; i < numGoroutines; i++ {
		select {
		case token := <-results:
			tokens = append(tokens, token)
		case err := <-errors:
			t.Errorf("Unexpected error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	}

	// All tokens should be the same (cached)
	assert.Len(t, tokens, numGoroutines)

	for i := 1; i < len(tokens); i++ {
		assert.Equal(t, tokens[0], tokens[i])
	}
}

func TestAuthManager_ContextCancellation(t *testing.T) {
	// Create a test server that delays response to test context cancellation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait longer than the context timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	// Create a context that will be canceled
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	token, err := authManager.GetValidToken(ctx)
	assert.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestAuthManager_EnsureAuthenticated_WithPassword(t *testing.T) {
	// Create a test server for authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testEndpoint {
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"ticket":              "ensure-auth-ticket",
					"CSRFPreventionToken": "ensure-auth-csrf",
					"username":            "testuser",
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)

			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	httpClient := &HTTPClient{
		baseURL: server.URL,
		client:  server.Client(),
	}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "testuser", "testpass", logger)

	err := authManager.EnsureAuthenticated()
	assert.NoError(t, err)

	// Verify that authentication was performed
	assert.NotNil(t, authManager.authToken)
	assert.Equal(t, "ensure-auth-ticket", authManager.authToken.Ticket)
}

// Benchmark tests.
func BenchmarkAuthToken_IsValid(b *testing.B) {
	token := &AuthToken{
		Ticket:    "benchmark-ticket",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = token.IsValid()
	}
}

func BenchmarkAuthManager_GetValidToken_Cached(b *testing.B) {
	httpClient := &HTTPClient{baseURL: "https://test.example.com"}
	logger := testutils.NewTestLogger()

	authManager := NewAuthManagerWithPassword(httpClient, "user", "pass", logger)
	authManager.authToken = &AuthToken{
		Ticket:    "cached-ticket",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = authManager.GetValidToken(ctx)
	}
}
