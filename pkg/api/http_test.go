package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/proxmox-tui/pkg/api/testutils"
)

func TestNewHTTPClient(t *testing.T) {
	httpClient := &http.Client{}
	baseURL := "https://test.example.com"
	logger := testutils.NewTestLogger()

	client := NewHTTPClient(httpClient, baseURL, logger)

	assert.NotNil(t, client)
	assert.Equal(t, httpClient, client.client)
	assert.Equal(t, baseURL, client.baseURL)
	assert.Equal(t, logger, client.logger)
	assert.Nil(t, client.authManager)
	assert.Empty(t, client.apiToken)
}

func TestHTTPClient_SetAuthManager(t *testing.T) {
	client := NewHTTPClient(&http.Client{}, "https://test.example.com", testutils.NewTestLogger())
	authManager := &AuthManager{}

	client.SetAuthManager(authManager)

	assert.Equal(t, authManager, client.authManager)
}

func TestHTTPClient_SetAPIToken(t *testing.T) {
	client := NewHTTPClient(&http.Client{}, "https://test.example.com", testutils.NewTestLogger())
	token := "user@realm!tokenid=secret"

	client.SetAPIToken(token)

	assert.Equal(t, token, client.apiToken)
}

func TestHTTPClient_Get_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test/path", r.URL.Path)
		assert.Equal(t, "proxmox-tui", r.Header.Get("User-Agent"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"test": "value",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test/path", &result)

	require.NoError(t, err)
	assert.NotNil(t, result)

	data, ok := result["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "value", data["test"])
}

func TestHTTPClient_Get_WithAPIToken(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "user@realm!tokenid=secret", r.Header.Get("Authorization"))

		response := map[string]interface{}{"success": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())
	client.SetAPIToken("user@realm!tokenid=secret")

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
}

func TestHTTPClient_Get_WithTicketAuth(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "PVEAuthCookie=test-ticket", r.Header.Get("Cookie"))

		response := map[string]interface{}{"success": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	// Create mock auth manager
	authManager := NewAuthManagerWithPassword(client, "user", "pass", testutils.NewTestLogger())
	authManager.authToken = &AuthToken{
		Ticket:    "test-ticket",
		CSRFToken: "test-csrf",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	client.SetAuthManager(authManager)

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
}

func TestHTTPClient_Post_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var requestData map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&requestData)
		require.NoError(t, err)
		assert.Equal(t, "test-value", requestData["key"])

		response := map[string]interface{}{"created": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	postData := map[string]interface{}{"key": "test-value"}
	var result map[string]interface{}
	err := client.Post(context.Background(), "/test", postData, &result)

	require.NoError(t, err)
	assert.True(t, result["created"].(bool))
}

func TestHTTPClient_Post_WithCSRFToken(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "PVEAuthCookie=test-ticket", r.Header.Get("Cookie"))
		assert.Equal(t, "test-csrf", r.Header.Get("CSRFPreventionToken"))

		response := map[string]interface{}{"success": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	// Create mock auth manager with CSRF token
	authManager := NewAuthManagerWithPassword(client, "user", "pass", testutils.NewTestLogger())
	authManager.authToken = &AuthToken{
		Ticket:    "test-ticket",
		CSRFToken: "test-csrf",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	client.SetAuthManager(authManager)

	postData := map[string]interface{}{"test": "data"}
	var result map[string]interface{}
	err := client.Post(context.Background(), "/test", postData, &result)

	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
}

func TestHTTPClient_Put_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		response := map[string]interface{}{"updated": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	putData := map[string]interface{}{"key": "updated-value"}
	var result map[string]interface{}
	err := client.Put(context.Background(), "/test", putData, &result)

	require.NoError(t, err)
	assert.True(t, result["updated"].(bool))
}

func TestHTTPClient_Delete_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)

		response := map[string]interface{}{"deleted": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	var result map[string]interface{}
	err := client.Delete(context.Background(), "/test", &result)

	require.NoError(t, err)
	assert.True(t, result["deleted"].(bool))
}

func TestHTTPClient_PathHandling(t *testing.T) {
	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
	}{
		{"path with leading slash", "/api/test", "/api/test"},
		{"path without leading slash", "api/test", "/api/test"},
		{"root path", "/", "/"},
		{"empty path", "", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.expectedPath, r.URL.Path)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			}))
			defer server.Close()

			client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

			var result map[string]interface{}
			err := client.Get(context.Background(), tt.inputPath, &result)
			require.NoError(t, err)
		})
	}
}

func TestHTTPClient_HTTPErrors(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedError string
	}{
		{"bad request", http.StatusBadRequest, "API request failed with status 400"},
		{"not found", http.StatusNotFound, "API request failed with status 404"},
		{"internal server error", http.StatusInternalServerError, "API request failed with status 500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("Error response"))
			}))
			defer server.Close()

			client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

			var result map[string]interface{}
			err := client.Get(context.Background(), "/test", &result)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestHTTPClient_UnauthorizedWithAPIToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())
	client.SetAPIToken("invalid-token")

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "API token authentication failed")
}

func TestHTTPClient_UnauthorizedWithTicketAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	// Create mock auth manager
	authManager := NewAuthManagerWithPassword(client, "user", "pass", testutils.NewTestLogger())
	authManager.authToken = &AuthToken{
		Ticket:    "expired-ticket",
		CSRFToken: "expired-csrf",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	client.SetAuthManager(authManager)

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")

	// Verify that the token was cleared
	assert.Nil(t, authManager.authToken)
}

func TestHTTPClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response JSON")
}

func TestHTTPClient_NetworkError(t *testing.T) {
	// Use invalid URL to simulate network error
	client := NewHTTPClient(&http.Client{Timeout: 1 * time.Second}, "http://invalid-host.local", testutils.NewTestLogger())

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestHTTPClient_ContextCancellation(t *testing.T) {
	// Create server that delays response to test context cancellation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait longer than the context timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var result map[string]interface{}
	err := client.Get(ctx, "/test", &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestHTTPClient_GetWithRetry_Success(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			// Fail first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Succeed on third attempt
		response := map[string]interface{}{"success": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	var result map[string]interface{}
	err := client.GetWithRetry(context.Background(), "/test", &result, 3)

	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, 3, attemptCount)
}

func TestHTTPClient_GetWithRetry_MaxRetriesExceeded(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server error"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	var result map[string]interface{}
	err := client.GetWithRetry(context.Background(), "/test", &result, 2)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed after 2 attempts")
	assert.Equal(t, 2, attemptCount)
}

func TestHTTPClient_shouldRetry(t *testing.T) {
	client := NewHTTPClient(&http.Client{}, "https://test.example.com", testutils.NewTestLogger())

	tests := []struct {
		name        string
		err         error
		attempt     int
		maxRetries  int
		shouldRetry bool
	}{
		{
			name:        "connection error should retry",
			err:         fmt.Errorf("connection refused"),
			attempt:     1,
			maxRetries:  3,
			shouldRetry: true,
		},
		{
			name:        "timeout error should retry",
			err:         fmt.Errorf("timeout exceeded"),
			attempt:     1,
			maxRetries:  3,
			shouldRetry: true,
		},
		{
			name:        "5xx error should retry",
			err:         fmt.Errorf("API request failed with status 500"),
			attempt:     1,
			maxRetries:  3,
			shouldRetry: true,
		},
		{
			name:        "4xx error should not retry",
			err:         fmt.Errorf("API request failed with status 400"),
			attempt:     1,
			maxRetries:  3,
			shouldRetry: false,
		},
		{
			name:        "max retries reached should not retry",
			err:         fmt.Errorf("connection refused"),
			attempt:     3,
			maxRetries:  3,
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.shouldRetry(tt.err, tt.attempt, tt.maxRetries)
			assert.Equal(t, tt.shouldRetry, result)
		})
	}
}

func TestHTTPClient_AuthenticationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	// Create auth manager that will fail authentication
	authManager := NewAuthManagerWithPassword(
		NewHTTPClient(&http.Client{}, "http://invalid-auth-server.local", testutils.NewTestLogger()),
		"user", "pass", testutils.NewTestLogger(),
	)
	client.SetAuthManager(authManager)

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestHTTPClient_MarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	// Use data that cannot be marshaled to JSON (channel)
	invalidData := map[string]interface{}{
		"channel": make(chan int),
	}

	var result map[string]interface{}
	err := client.Post(context.Background(), "/test", invalidData, &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal request data")
}

func TestHTTPClient_NilResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"success": true}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	// Test with nil result - should not error
	err := client.Get(context.Background(), "/test", nil)
	require.NoError(t, err)
}

func TestHTTPClient_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty response body
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())

	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response JSON")
}

// Benchmark tests
func BenchmarkHTTPClient_Get(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"data": "benchmark"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL, testutils.NewTestLogger())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		_ = client.Get(ctx, "/benchmark", &result)
	}
}

func BenchmarkHTTPClient_shouldRetry(b *testing.B) {
	client := NewHTTPClient(&http.Client{}, "https://test.example.com", testutils.NewTestLogger())
	err := fmt.Errorf("connection refused")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.shouldRetry(err, 1, 3)
	}
}
