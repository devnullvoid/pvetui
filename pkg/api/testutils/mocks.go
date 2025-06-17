package testutils

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockLogger is a mock implementation of the Logger interface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Info(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Error(format string, args ...interface{}) {
	m.Called(format, args)
}

// MockCache is a mock implementation of the Cache interface
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(key string, dest interface{}) (bool, error) {
	args := m.Called(key, dest)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) Set(key string, value interface{}, ttl time.Duration) error {
	args := m.Called(key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Delete(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *MockCache) Clear() error {
	args := m.Called()
	return args.Error(0)
}

// MockConfig is a mock implementation of the Config interface
type MockConfig struct {
	mock.Mock
}

func (m *MockConfig) GetAddr() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConfig) GetUser() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConfig) GetPassword() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConfig) GetRealm() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConfig) GetTokenID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConfig) GetTokenSecret() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConfig) GetInsecure() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockConfig) IsUsingTokenAuth() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockConfig) GetAPIToken() string {
	args := m.Called()
	return args.String(0)
}

// TestConfig is a simple test implementation of the Config interface
type TestConfig struct {
	Addr        string
	User        string
	Password    string
	Realm       string
	TokenID     string
	TokenSecret string
	Insecure    bool
}

func (c *TestConfig) GetAddr() string        { return c.Addr }
func (c *TestConfig) GetUser() string        { return c.User }
func (c *TestConfig) GetPassword() string    { return c.Password }
func (c *TestConfig) GetRealm() string       { return c.Realm }
func (c *TestConfig) GetTokenID() string     { return c.TokenID }
func (c *TestConfig) GetTokenSecret() string { return c.TokenSecret }
func (c *TestConfig) GetInsecure() bool      { return c.Insecure }

func (c *TestConfig) IsUsingTokenAuth() bool {
	return c.TokenID != "" && c.TokenSecret != ""
}

func (c *TestConfig) GetAPIToken() string {
	if c.IsUsingTokenAuth() {
		return "PVEAPIToken=" + c.User + "@" + c.Realm + "!" + c.TokenID + "=" + c.TokenSecret
	}
	return ""
}

// NewTestConfig creates a test configuration with sensible defaults
func NewTestConfig() *TestConfig {
	return &TestConfig{
		Addr:     "https://test.example.com:8006",
		User:     "testuser",
		Password: "testpass",
		Realm:    "pam",
		Insecure: true,
	}
}

// NewTestConfigWithToken creates a test configuration using token authentication
func NewTestConfigWithToken() *TestConfig {
	return &TestConfig{
		Addr:        "https://test.example.com:8006",
		User:        "testuser",
		Realm:       "pam",
		TokenID:     "testtoken",
		TokenSecret: "testsecret",
		Insecure:    true,
	}
}

// TestLogger is a simple test logger that captures log messages
type TestLogger struct {
	DebugMessages []string
	InfoMessages  []string
	ErrorMessages []string
}

func (l *TestLogger) Debug(format string, args ...interface{}) {
	l.DebugMessages = append(l.DebugMessages, fmt.Sprintf(format, args...))
}

func (l *TestLogger) Info(format string, args ...interface{}) {
	l.InfoMessages = append(l.InfoMessages, fmt.Sprintf(format, args...))
}

func (l *TestLogger) Error(format string, args ...interface{}) {
	l.ErrorMessages = append(l.ErrorMessages, fmt.Sprintf(format, args...))
}

func (l *TestLogger) Reset() {
	l.DebugMessages = nil
	l.InfoMessages = nil
	l.ErrorMessages = nil
}

// NewTestLogger creates a new test logger
func NewTestLogger() *TestLogger {
	return &TestLogger{
		DebugMessages: make([]string, 0),
		InfoMessages:  make([]string, 0),
		ErrorMessages: make([]string, 0),
	}
}

// InMemoryCache is a simple in-memory cache for testing
type InMemoryCache struct {
	data map[string]interface{}
}

func (c *InMemoryCache) Get(key string, dest interface{}) (bool, error) {
	value, exists := c.data[key]
	if !exists {
		return false, nil
	}

	// Simple type assertion for testing
	switch d := dest.(type) {
	case *string:
		if s, ok := value.(string); ok {
			*d = s
		}
	case *map[string]interface{}:
		if m, ok := value.(map[string]interface{}); ok {
			*d = m
		}
	case *interface{}:
		*d = value
	}

	return true, nil
}

func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	c.data[key] = value
	return nil
}

func (c *InMemoryCache) Delete(key string) error {
	delete(c.data, key)
	return nil
}

func (c *InMemoryCache) Clear() error {
	c.data = make(map[string]interface{})
	return nil
}

// NewInMemoryCache creates a new in-memory cache for testing
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		data: make(map[string]interface{}),
	}
}

// AssertLogContains checks if a log message contains the expected text
func AssertLogContains(t *testing.T, logger *TestLogger, level string, expectedText string) {
	var messages []string
	switch level {
	case "debug":
		messages = logger.DebugMessages
	case "info":
		messages = logger.InfoMessages
	case "error":
		messages = logger.ErrorMessages
	default:
		t.Fatalf("Unknown log level: %s", level)
	}

	for _, msg := range messages {
		if strings.Contains(msg, expectedText) {
			return
		}
	}

	t.Errorf("Expected %s log to contain '%s', but it was not found. Messages: %v", level, expectedText, messages)
}
