package interfaces

import "time"

// Logger defines the interface for logging functionality
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// Cache defines the interface for caching functionality
type Cache interface {
	Get(key string, dest interface{}) (bool, error)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	Clear() error
}

// Config defines the interface for configuration data needed by the API client
type Config interface {
	GetAddr() string
	GetUser() string
	GetPassword() string
	GetRealm() string
	GetTokenID() string
	GetTokenSecret() string
	GetInsecure() bool
	IsUsingTokenAuth() bool
	GetAPIToken() string
}

// NoOpLogger is a logger that does nothing (useful for testing or when logging is not needed)
type NoOpLogger struct{}

func (n *NoOpLogger) Debug(format string, args ...interface{}) {}
func (n *NoOpLogger) Info(format string, args ...interface{})  {}
func (n *NoOpLogger) Error(format string, args ...interface{}) {}

// NoOpCache is a cache that does nothing (useful for testing or when caching is not needed)
type NoOpCache struct{}

func (n *NoOpCache) Get(key string, dest interface{}) (bool, error)             { return false, nil }
func (n *NoOpCache) Set(key string, value interface{}, ttl time.Duration) error { return nil }
func (n *NoOpCache) Delete(key string) error                                    { return nil }
func (n *NoOpCache) Clear() error                                               { return nil }
