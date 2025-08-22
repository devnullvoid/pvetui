package api

import (
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// ClientOptions holds optional dependencies for the API client.
type ClientOptions struct {
	Logger interfaces.Logger
	Cache  interfaces.Cache
}

// ClientOption is a function that configures ClientOptions.
type ClientOption func(*ClientOptions)

// WithLogger sets a custom logger for the client.
func WithLogger(logger interfaces.Logger) ClientOption {
	return func(opts *ClientOptions) {
		opts.Logger = logger
	}
}

// WithCache sets a custom cache for the client.
func WithCache(cache interfaces.Cache) ClientOption {
	return func(opts *ClientOptions) {
		opts.Cache = cache
	}
}

// defaultOptions returns ClientOptions with sensible defaults.
func defaultOptions() *ClientOptions {
	return &ClientOptions{
		Logger: &interfaces.NoOpLogger{},
		Cache:  &interfaces.NoOpCache{},
	}
}
