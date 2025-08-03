# Testing Guide for Proxmox TUI

This document provides a comprehensive guide to testing in the Proxmox TUI project. We use Go's built-in testing framework along with the testify library for assertions and mocking.

## Table of Contents

- [Overview](#overview)
- [Running Tests](#running-tests)
- [Test Structure](#test-structure)
- [Writing Tests](#writing-tests)
- [Test Utilities](#test-utilities)
- [Coverage](#coverage)
- [Best Practices](#best-practices)
- [Continuous Integration](#continuous-integration)

## Overview

Our testing strategy follows Go best practices and includes:

- **Unit Tests**: Test individual functions and methods in isolation
- **Integration Tests**: Test interactions between components
- **Table-Driven Tests**: Comprehensive test cases using Go's table-driven pattern
- **Mocking**: Mock external dependencies using testify/mock
- **Test Coverage**: Measure and maintain good test coverage

### Testing Libraries

- **Go Testing**: Built-in `testing` package
- **Testify**: Assertions, mocks, and test suites (`github.com/stretchr/testify`)
  - `assert`: Rich assertions
  - `require`: Assertions that stop test execution on failure
  - `mock`: Mocking framework
  - `suite`: Test suites for setup/teardown

## Running Tests

### Basic Test Commands

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests for a specific package
go test ./internal/config

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -run TestConfig_Validate ./internal/config

# Run tests in parallel
go test -parallel 4 ./...
```

### Coverage Commands

```bash
# Generate coverage report
make test-coverage

# View coverage in browser (after running test-coverage)
open coverage.html

# Get coverage percentage only
go test -cover ./...
```

## Test Structure

### File Organization

Tests are organized alongside the code they test:

```
internal/
├── config/
│   ├── config.go
│   └── config_test.go
├── cache/
│   ├── cache.go
│   └── cache_test.go
└── adapters/
    ├── adapters.go
    └── adapters_test.go

pkg/
├── api/
│   ├── utils.go
│   ├── utils_test.go
│   └── testutils/
│       └── mocks.go
```

### Test File Naming

- Test files end with `_test.go`
- Test functions start with `Test`
- Benchmark functions start with `Benchmark`
- Example functions start with `Example`

## Writing Tests

### Basic Test Structure

```go
func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"

    // Act
    result := FunctionToTest(input)

    // Assert
    assert.Equal(t, expected, result)
}
```

### Table-Driven Tests

Use table-driven tests for comprehensive coverage:

```go
func TestConfig_Validate(t *testing.T) {
    tests := []struct {
        name        string
        config      *Config
        expectError bool
        errorMsg    string
    }{
        {
            name: "valid config",
            config: &Config{
                Addr: "https://example.com",
                User: "user",
                Password: "pass",
            },
            expectError: false,
        },
        {
            name: "missing address",
            config: &Config{
                User: "user",
                Password: "pass",
            },
            expectError: true,
            errorMsg: "address required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()

            if tt.expectError {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Testing with Mocks

Use mocks to isolate units under test:

```go
func TestClientWithMocks(t *testing.T) {
    // Create mocks
    mockLogger := &testutils.MockLogger{}
    mockCache := &testutils.MockCache{}
    mockConfig := &testutils.MockConfig{}

    // Set expectations
    mockConfig.On("GetAddr").Return("https://test.com")
    mockCache.On("Get", "key", mock.Anything).Return(false, nil)

    // Test your code
    // ... test implementation

    // Verify expectations
    mockConfig.AssertExpectations(t)
    mockCache.AssertExpectations(t)
}
```

### Testing with Temporary Files/Directories

Use `t.TempDir()` for tests that need file system operations:

```go
func TestFileOperations(t *testing.T) {
    tempDir := t.TempDir() // Automatically cleaned up

    filePath := filepath.Join(tempDir, "test.txt")
    err := os.WriteFile(filePath, []byte("test"), 0644)
    require.NoError(t, err)

    // Test your file operations
}
```

### Testing Error Cases

Always test both success and error paths:

```go
func TestParseVMID(t *testing.T) {
    tests := []struct {
        name        string
        input       interface{}
        expected    int
        expectError bool
    }{
        {
            name:        "valid integer",
            input:       123,
            expected:    123,
            expectError: false,
        },
        {
            name:        "invalid string",
            input:       "not-a-number",
            expected:    0,
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := ParseVMID(tt.input)

            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

## Test Utilities

### Available Test Utilities

Located in `pkg/api/testutils/`:

#### MockLogger
```go
mockLogger := &testutils.MockLogger{}
mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
```

#### MockCache
```go
mockCache := &testutils.MockCache{}
mockCache.On("Get", "key", mock.Anything).Return(true, nil)
mockCache.On("Set", "key", "value", time.Hour).Return(nil)
```

#### MockConfig
```go
mockConfig := &testutils.MockConfig{}
mockConfig.On("GetAddr").Return("https://test.com")
```

#### TestConfig
```go
// Simple test config with sensible defaults
config := testutils.NewTestConfig()

// Test config with token auth
tokenConfig := testutils.NewTestConfigWithToken()
```

#### TestLogger
```go
logger := testutils.NewTestLogger()
// Use logger in tests
logger.Debug("test message")

// Check logged messages
assert.Contains(t, logger.DebugMessages[0], "test message")
```

#### InMemoryCache
```go
cache := testutils.NewInMemoryCache()
cache.Set("key", "value", time.Hour)
```

### Creating New Test Utilities

When creating new test utilities:

1. Place them in appropriate `testutils` packages
2. Follow naming conventions (`Mock*`, `Test*`, `New*`)
3. Implement relevant interfaces
4. Provide sensible defaults
5. Document usage with examples

## Coverage

### Current Coverage

- **Config Package**: ~66% coverage
- **Cache Package**: ~32% coverage
- **Adapters Package**: ~95% coverage
- **API Utils**: ~4% coverage (mostly utility functions)

### Coverage Goals

- Aim for **80%+ coverage** on core business logic
- **100% coverage** on critical paths (authentication, configuration)
- Focus on **meaningful coverage** over percentage

### Improving Coverage

1. Identify uncovered lines: `go tool cover -html=coverage.out`
2. Add tests for uncovered branches
3. Test error paths and edge cases
4. Add integration tests for complex workflows

## Best Practices

### Test Organization

1. **One test file per source file**: `config.go` → `config_test.go`
2. **Group related tests**: Use subtests with `t.Run()`
3. **Clear test names**: Describe what is being tested
4. **Arrange-Act-Assert**: Structure tests clearly

### Test Data

1. **Use table-driven tests** for multiple scenarios
2. **Test edge cases**: empty strings, nil values, boundary conditions
3. **Test error conditions**: Invalid inputs, network failures, etc.
4. **Use realistic test data**: Representative of actual usage

### Assertions

1. **Use appropriate assertions**:
   - `assert.Equal()` for value comparison
   - `assert.NoError()` / `assert.Error()` for error checking
   - `assert.Contains()` for substring/element checking
   - `require.*()` when test should stop on failure

2. **Provide meaningful messages**:
   ```go
   assert.Equal(t, expected, actual, "Config validation should pass for valid input")
   ```

### Mocking

1. **Mock external dependencies**: HTTP clients, databases, file systems
2. **Don't mock value objects**: Simple structs, data containers
3. **Verify mock expectations**: Use `AssertExpectations()`
4. **Reset mocks between tests**: Avoid test pollution

### Performance

1. **Use `t.Parallel()`** for independent tests
2. **Avoid expensive operations** in test setup
3. **Cache test fixtures** when appropriate
4. **Use benchmarks** for performance-critical code

### Cleanup

1. **Use `t.TempDir()`** for temporary files
2. **Defer cleanup operations**
3. **Reset global state** between tests
4. **Close resources** properly

## Continuous Integration

### GitHub Actions

Tests run automatically on:
- Pull requests
- Pushes to main/develop branches
- Scheduled runs (nightly)

### Local Pre-commit

Run tests before committing:

```bash
# Quick test run
make test

# Full test with coverage
make test-coverage

# Lint and format
make lint
make format
```

### Test Requirements

- All tests must pass
- Coverage should not decrease
- New features require tests
- Bug fixes should include regression tests

## Examples

### Testing Configuration Loading

```go
func TestConfig_LoadFromFile(t *testing.T) {
    tempDir := t.TempDir()
    configFile := filepath.Join(tempDir, "config.yml")

    configContent := `
addr: "https://test.example.com:8006"
user: "testuser"
password: "testpass"
`

    err := os.WriteFile(configFile, []byte(configContent), 0644)
    require.NoError(t, err)

    config := &Config{}
    err = config.MergeWithFile(configFile)

    assert.NoError(t, err)
    assert.Equal(t, "https://test.example.com:8006", config.Addr)
    assert.Equal(t, "testuser", config.User)
    assert.Equal(t, "testpass", config.Password)
}
```

### Testing Cache Operations

```go
func TestCache_SetAndGet(t *testing.T) {
    cache := NewInMemoryCache()

    key := "test-key"
    value := "test-value"

    // Test Set
    err := cache.Set(key, value, time.Hour)
    assert.NoError(t, err)

    // Test Get
    var result string
    found, err := cache.Get(key, &result)
    assert.NoError(t, err)
    assert.True(t, found)
    assert.Equal(t, value, result)
}
```

### Testing with Environment Variables

```go
func TestConfig_FromEnvironment(t *testing.T) {
    // Save original environment
    originalAddr := os.Getenv("PROXMOX_ADDR")
    defer os.Setenv("PROXMOX_ADDR", originalAddr)

    // Set test environment
    os.Setenv("PROXMOX_ADDR", "https://test.com")

    config := NewConfig()
    assert.Equal(t, "https://test.com", config.Addr)
}
```

## Troubleshooting

### Common Issues

1. **Tests fail in CI but pass locally**
   - Check for race conditions
   - Verify environment differences
   - Look for hardcoded paths/values

2. **Flaky tests**
   - Add proper synchronization
   - Increase timeouts for timing-sensitive tests
   - Use deterministic test data

3. **Low coverage**
   - Check for untested error paths
   - Add tests for edge cases
   - Test private functions through public interfaces

4. **Slow tests**
   - Use `t.Parallel()` for independent tests
   - Mock expensive operations
   - Optimize test setup/teardown

### Getting Help

- Check existing tests for patterns
- Review Go testing documentation
- Ask team members for guidance
- Use `go test -v` for detailed output

---

This testing guide should help you understand and contribute to the test suite. Remember: good tests are an investment in code quality and developer productivity!
