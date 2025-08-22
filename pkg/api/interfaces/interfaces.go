// Package interfaces defines the core interfaces used throughout the pvetui application.
//
// This package provides clean abstractions for logging, caching, and configuration
// that enable dependency injection and testability. All interfaces follow Go
// best practices with minimal, focused responsibilities.
//
// The interfaces are designed to be easily mockable for testing and allow
// for different implementations (e.g., file-based vs in-memory caching,
// console vs file logging).
package interfaces

import "time"

// Logger defines the interface for structured logging functionality.
//
// Implementations should support different log levels and be safe for
// concurrent use. The format parameter follows fmt.Printf conventions.
//
// Example usage:
//
//	logger.Debug("Processing request for user: %s", userID)
//	logger.Info("Server started on port %d", port)
//	logger.Error("Failed to connect to database: %v", err)
type Logger interface {
	// Debug logs debug-level messages. These are typically only shown
	// in development or when debug logging is explicitly enabled.
	Debug(format string, args ...interface{})

	// Info logs informational messages about normal application flow.
	Info(format string, args ...interface{})

	// Error logs error messages for exceptional conditions that should
	// be investigated.
	Error(format string, args ...interface{})
}

// Cache defines the interface for key-value caching functionality.
//
// Implementations should be safe for concurrent use and handle TTL
// (time-to-live) expiration automatically. The dest parameter in Get
// should be a pointer to the type you want to unmarshal into.
//
// Example usage:
//
//	// Store data with 1 hour TTL
//	cache.Set("user:123", userData, time.Hour)
//
//	// Retrieve data
//	var user User
//	found, err := cache.Get("user:123", &user)
//	if found && err == nil {
//		// Use user data
//	}
type Cache interface {
	// Get retrieves a value from the cache and unmarshals it into dest.
	// Returns true if the key was found and not expired, false otherwise.
	// dest must be a pointer to the type you want to unmarshal into.
	Get(key string, dest interface{}) (bool, error)

	// Set stores a value in the cache with the specified TTL.
	// If ttl is 0, the item will not expire automatically.
	Set(key string, value interface{}, ttl time.Duration) error

	// Delete removes a specific key from the cache.
	Delete(key string) error

	// Clear removes all items from the cache.
	Clear() error
}

// Config defines the interface for accessing application configuration.
//
// This interface abstracts configuration sources (environment variables,
// files, command-line flags) and provides a clean API for accessing
// Proxmox connection settings and authentication credentials.
//
// Implementations should validate configuration values and provide
// sensible defaults where appropriate.
type Config interface {
	// GetAddr returns the Proxmox server URL (e.g., "https://pve.example.com:8006").
	GetAddr() string

	// GetUser returns the Proxmox username (without realm suffix).
	GetUser() string

	// GetPassword returns the password for password-based authentication.
	// Returns empty string if using token authentication.
	GetPassword() string

	// GetRealm returns the authentication realm (e.g., "pam", "pve").
	GetRealm() string

	// GetTokenID returns the API token ID for token-based authentication.
	// Returns empty string if using password authentication.
	GetTokenID() string

	// GetTokenSecret returns the API token secret for token-based authentication.
	// Returns empty string if using password authentication.
	GetTokenSecret() string

	// GetInsecure returns true if TLS certificate verification should be skipped.
	GetInsecure() bool

	// IsUsingTokenAuth returns true if configured for API token authentication,
	// false if using password authentication.
	IsUsingTokenAuth() bool

	// GetAPIToken returns the complete API token string in Proxmox format:
	// "PVEAPIToken=USER@REALM!TOKENID=SECRET"
	// Returns empty string if using password authentication.
	GetAPIToken() string
}

// NoOpLogger is a logger implementation that discards all log messages.
//
// This is useful for testing scenarios where you want to suppress log output,
// or for production deployments where logging is handled elsewhere.
//
// Example usage:
//
//	logger := &interfaces.NoOpLogger{}
//	client := api.NewClient(config, api.WithLogger(logger))
type NoOpLogger struct{}

// Debug discards the debug message.
func (n *NoOpLogger) Debug(format string, args ...interface{}) {}

// Info discards the info message.
func (n *NoOpLogger) Info(format string, args ...interface{}) {}

// Error discards the error message.
func (n *NoOpLogger) Error(format string, args ...interface{}) {}

// NoOpCache is a cache implementation that doesn't store anything.
//
// This is useful for testing scenarios where you want to disable caching,
// or for deployments where caching is not desired or handled elsewhere.
//
// All Get operations return false (not found), and all Set/Delete/Clear
// operations succeed immediately without doing anything.
//
// Example usage:
//
//	cache := &interfaces.NoOpCache{}
//	client := api.NewClient(config, api.WithCache(cache))
type NoOpCache struct{}

// Get always returns false (not found) and no error.
func (n *NoOpCache) Get(key string, dest interface{}) (bool, error) { return false, nil }

// Set always succeeds immediately without storing anything.
func (n *NoOpCache) Set(key string, value interface{}, ttl time.Duration) error { return nil }

// Delete always succeeds immediately without doing anything.
func (n *NoOpCache) Delete(key string) error { return nil }

// Clear always succeeds immediately without doing anything.
func (n *NoOpCache) Clear() error { return nil }
