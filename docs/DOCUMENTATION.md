# Documentation Guide for Proxmox TUI

This document provides a comprehensive guide to the documentation system implemented throughout the Proxmox TUI codebase. All packages and public functions now include comprehensive GoDoc documentation following Go best practices.

## Table of Contents

- [Overview](#overview)
- [Documentation Standards](#documentation-standards)
- [Viewing Documentation](#viewing-documentation)
- [Package Documentation](#package-documentation)
- [API Reference](#api-reference)
- [Examples](#examples)
- [Contributing to Documentation](#contributing-to-documentation)

## Overview

The Proxmox TUI project follows Go's official documentation conventions using GoDoc. Every package, type, function, and method includes comprehensive documentation that explains:

- **Purpose and functionality**
- **Parameters and return values**
- **Usage examples**
- **Error conditions**
- **Thread safety considerations**
- **Best practices**

## Documentation Standards

### Package-Level Documentation

Every package includes comprehensive package-level documentation that explains:

- Purpose and scope of the package
- Key features and capabilities
- Usage patterns and examples
- Integration with other packages
- Thread safety guarantees

### Function Documentation

All public functions include documentation covering:

- Clear description of functionality
- Parameter descriptions with types and constraints
- Return value explanations
- Usage examples with code samples
- Error conditions and handling
- Performance considerations where relevant

### Type Documentation

All public types include:

- Purpose and use cases
- Field descriptions with constraints
- Usage examples
- Relationship to other types
- Implementation notes

## Viewing Documentation

### Command Line Documentation

Use Go's built-in documentation tools to view documentation:

```bash
# View package documentation
go doc pkg/api
go doc internal/config
go doc pkg/api/interfaces

# View specific function documentation
go doc pkg/api FormatBytes
go doc pkg/api AuthManager.GetValidToken

# View detailed documentation for all exported items
go doc -all pkg/api

# View documentation with source code
go doc -src pkg/api FormatBytes
```

### Web Documentation

Generate and serve HTML documentation locally:

```bash
# Install godoc if not already installed
go install golang.org/x/tools/cmd/godoc@latest

# Serve documentation on http://localhost:6060
godoc -http=:6060

# Then navigate to:
# http://localhost:6060/pkg/github.com/devnullvoid/peevetui/
```

### IDE Integration

Most Go IDEs (VS Code, GoLand, Vim with plugins) automatically display GoDoc documentation:

- Hover over functions to see documentation
- Use "Go to Definition" to see full documentation
- Auto-completion includes documentation snippets

## Package Documentation

### Core Packages

#### `pkg/api` - Main API Client
Comprehensive client library for Proxmox Virtual Environment API with:
- Clean Architecture with dependency injection
- Authentication support (password + API tokens)
- Built-in caching and logging
- Thread-safe operations
- Robust error handling and retry logic

#### `pkg/api/interfaces` - Core Interfaces
Clean abstractions for logging, caching, and configuration:
- Logger interface for structured logging
- Cache interface for key-value caching
- Config interface for configuration access
- NoOp implementations for testing

#### `internal/config` - Configuration Management
Multi-source configuration with XDG compliance:
- Environment variables
- Command-line flags
- YAML configuration files
- XDG Base Directory Specification support

#### `internal/adapters` - Adapter Pattern
Bridge implementations connecting internal components:
- ConfigAdapter for API client integration
- LoggerAdapter for logging abstraction
- CacheAdapter for caching abstraction

#### `internal/logger` - Logging System
Comprehensive logging with TUI-friendly design:
- Multiple log levels (Debug, Info, Error)
- File-based logging (avoids stdout interference)
- Configurable output destinations
- Thread-safe operations

### Testing Packages

#### `pkg/api/testutils` - Testing Utilities
Comprehensive testing support with:
- Mock implementations using testify/mock
- Test data generators
- Helper functions for common patterns
- In-memory implementations

## API Reference

### Authentication

```go
// Password-based authentication
authManager := api.NewAuthManagerWithPassword(httpClient, "root", "password", logger)

// API token authentication
authManager := api.NewAuthManagerWithToken(httpClient, "PVEAPIToken=...", logger)

// Ensure authentication
err := authManager.EnsureAuthenticated()
```

### Client Creation

```go
// Basic client
client, err := api.NewClient(config)

// Client with custom logger and cache
client, err := api.NewClient(config,
    api.WithLogger(logger),
    api.WithCache(cache))
```

### Configuration

```go
// Load configuration from multiple sources
config := config.NewConfig()
config.ParseFlags()
err := config.MergeWithFile("config.yml")
config.SetDefaults()
err = config.Validate()
```

### Utility Functions

```go
// Format bytes for human display
formatted := api.FormatBytes(1073741824) // "1.0 GB"

// Format uptime duration
uptime := api.FormatUptime(3661) // "1 hour, 1 minute, 1 second"

// Safe type conversion
vmid, err := api.ParseVMID(input)
str := api.SafeStringValue(value)
num := api.SafeFloatValue(value)
bool := api.SafeBoolValue(value)
```

## Examples

### Complete Client Setup

```go
package main

import (
    "context"
    "log"

    "github.com/devnullvoid/peevetui/internal/config"
    "github.com/devnullvoid/peevetui/internal/logger"
    "github.com/devnullvoid/peevetui/internal/adapters"
    "github.com/devnullvoid/peevetui/pkg/api"
)

func main() {
    // Load configuration
    cfg := config.NewConfig()
    cfg.ParseFlags()
    cfg.SetDefaults()
    if err := cfg.Validate(); err != nil {
        log.Fatal("Invalid configuration:", err)
    }

    // Create logger
    loggerInstance, err := logger.NewInternalLogger(logger.LevelInfo, cfg.CacheDir)
    if err != nil {
        log.Fatal("Failed to create logger:", err)
    }
    defer loggerInstance.Close()

    // Create adapters
    configAdapter := adapters.NewConfigAdapter(cfg)
    loggerAdapter := adapters.NewLoggerAdapter(cfg)

    // Create API client
    // Note: This will attempt initial authentication and may fail if credentials are invalid
    client, err := api.NewClient(configAdapter,
        api.WithLogger(loggerAdapter))
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }

    // Use the client
    ctx := context.Background()
    vms, err := client.GetVmList(ctx)
    if err != nil {
        log.Fatal("Failed to get VMs:", err)
    }

    log.Printf("Found %d VMs", len(vms))
}
```

### Robust Client Setup with Error Handling

For production use, you should include proper error handling:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/devnullvoid/peevetui/internal/config"
    "github.com/devnullvoid/peevetui/internal/logger"
    "github.com/devnullvoid/peevetui/internal/adapters"
    "github.com/devnullvoid/peevetui/pkg/api"
)

func main() {
    // Load configuration with error handling
    cfg := config.NewConfig()
    cfg.ParseFlags()
    cfg.SetDefaults()

    if err := cfg.Validate(); err != nil {
        log.Printf("Configuration validation failed: %v", err)
        log.Println("Please check your configuration and try again.")
        return
    }

    // Create logger with error handling
    loggerInstance, err := logger.NewInternalLogger(logger.LevelInfo, cfg.CacheDir)
    if err != nil {
        log.Printf("Failed to create logger: %v", err)
        log.Println("Falling back to simple logging...")
        loggerInstance = logger.NewSimpleLogger(logger.LevelInfo)
    }
    defer loggerInstance.Close()

    // Create adapters
    configAdapter := adapters.NewConfigAdapter(cfg)
    loggerAdapter := adapters.NewLoggerAdapter(cfg)

    // Create API client with error handling
    client, err := api.NewClient(configAdapter,
        api.WithLogger(loggerAdapter))
    if err != nil {
        log.Printf("Failed to create client: %v", err)
        log.Println("Please check your Proxmox server address and credentials.")
        return
    }

    // Use the client with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    vms, err := client.GetVmList(ctx)
    if err != nil {
        log.Printf("Failed to get VMs: %v", err)
        log.Println("This could be due to network issues or insufficient permissions.")
        return
    }

    log.Printf("Successfully retrieved %d VMs", len(vms))

         // Example: List VM details
     for _, vm := range vms {
         vmid := api.SafeStringValue(vm["vmid"])
         name := api.SafeStringValue(vm["name"])
         status := api.SafeStringValue(vm["status"])
         log.Printf("VM %s: %s (Status: %s)", vmid, name, status)
     }
}
```

### Testing with Mocks

```go
package main

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"

    "github.com/devnullvoid/peevetui/pkg/api"
    "github.com/devnullvoid/peevetui/pkg/api/testutils"
)

func TestAPIClient(t *testing.T) {
    // Create mocks
    mockLogger := &testutils.MockLogger{}
    mockCache := &testutils.MockCache{}
    config := testutils.NewTestConfig()

    // Set expectations
    mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
    mockCache.On("Get", "test-key", mock.Anything).Return(false, nil)

    // Create client with mocks
    client, err := api.NewClient(config,
        api.WithLogger(mockLogger),
        api.WithCache(mockCache))
    assert.NoError(t, err)

    // Test client operations
    // ... your test code here

    // Verify expectations
    mockLogger.AssertExpectations(t)
    mockCache.AssertExpectations(t)
}
```

## Contributing to Documentation

### Writing Documentation

When contributing code, follow these documentation guidelines:

1. **Package Documentation**: Every package should have comprehensive package-level documentation explaining its purpose, key features, and usage patterns.

2. **Function Documentation**: All public functions must include:
   - Clear description of what the function does
   - Parameter descriptions with types and constraints
   - Return value explanations
   - Usage examples
   - Error conditions

3. **Type Documentation**: All public types should include:
   - Purpose and use cases
   - Field descriptions
   - Usage examples
   - Relationships to other types

4. **Examples**: Include practical examples showing real-world usage patterns.

### Documentation Format

Follow Go's documentation conventions:

```go
// PackageName provides functionality for specific purpose.
//
// Longer description explaining the package's role, key features,
// and how it fits into the larger system.
//
// Example usage:
//
//	// Code example showing basic usage
//	client := NewClient(config)
//	result, err := client.DoSomething()
package packagename

// FunctionName performs a specific operation with the given parameters.
//
// Detailed description of what the function does, including any
// important behavior, side effects, or constraints.
//
// Parameters:
//   - param1: Description of first parameter
//   - param2: Description of second parameter
//
// Returns the result or an error if the operation fails.
//
// Example usage:
//
//	result, err := FunctionName("value1", 42)
//	if err != nil {
//		log.Fatal("Operation failed:", err)
//	}
func FunctionName(param1 string, param2 int) (string, error) {
    // Implementation
}
```

### Documentation Review

When reviewing code:

1. Ensure all public APIs have comprehensive documentation
2. Verify examples compile and work correctly
3. Check that documentation explains the "why" not just the "what"
4. Confirm error conditions are documented
5. Validate that examples follow best practices

### Updating Documentation

When modifying existing code:

1. Update documentation to reflect changes
2. Add new examples if functionality has expanded
3. Update package-level documentation if scope has changed
4. Ensure backward compatibility notes are included

## Best Practices

### Writing Effective Documentation

1. **Start with the user's perspective**: What does someone need to know to use this effectively?

2. **Include examples**: Code examples are often more valuable than lengthy explanations.

3. **Document error conditions**: Explain when and why functions might fail.

4. **Explain relationships**: How does this component interact with others?

5. **Keep it current**: Update documentation when code changes.

### Documentation Maintenance

1. **Regular reviews**: Periodically review documentation for accuracy and completeness.

2. **User feedback**: Pay attention to questions that indicate unclear documentation.

3. **Consistency**: Maintain consistent terminology and formatting across packages.

4. **Testing**: Ensure code examples in documentation actually work.

## Tools and Resources

### Go Documentation Tools

- `go doc`: Command-line documentation viewer
- `godoc`: HTML documentation server
- `go help doc`: Help for documentation tools

### External Tools

- [pkg.go.dev](https://pkg.go.dev): Online Go package documentation
- [GoDoc.org](https://godoc.org): Legacy online documentation
- IDE plugins: Most Go IDEs provide integrated documentation viewing

### Style Guides

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Doc Comments](https://tip.golang.org/doc/comment)

---

This documentation system ensures that the Proxmox TUI codebase is well-documented, maintainable, and accessible to both contributors and users. The comprehensive GoDoc comments provide clear guidance for using the API and understanding the system architecture.
