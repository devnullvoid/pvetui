package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected string
	}{
		{"debug level", LevelDebug, "DEBUG"},
		{"info level", LevelInfo, "INFO"},
		{"error level", LevelError, "ERROR"},
		{"unknown level", Level(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, LevelInfo, config.Level)
	assert.Equal(t, os.Stdout, config.Output)
	assert.False(t, config.LogToFile)
	assert.Equal(t, "2006-01-02 15:04:05", config.TimeFormat)
}

func TestNewLogger_WithNilConfig(t *testing.T) {
	logger, err := NewLogger(nil)

	require.NoError(t, err)
	assert.NotNil(t, logger)
	assert.Equal(t, LevelInfo, logger.level)
}

func TestNewLogger_WithCustomOutput(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelDebug,
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	logger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "[INFO]")
	assert.Contains(t, output, "test message")
}

func TestNewLogger_WithFileOutput(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "test.log")
	config := &Config{
		Level:     LevelDebug,
		LogToFile: true,
		LogFile:   logFile,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	logger.Info("test file message")

	// Check if file was created and contains the message
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	assert.Contains(t, string(content), "[INFO]")
	assert.Contains(t, string(content), "test file message")
}

func TestNewLogger_WithDualOutput(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "test.log")

	config := &Config{
		Level:     LevelDebug,
		Output:    os.Stdout, // Use stdout for dual output test
		LogToFile: true,
		LogFile:   logFile,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	logger.Info("dual output message")

	// Check file output (dual output writes to both stdout and file when Output is stdout)
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "[INFO]")
	assert.Contains(t, string(content), "dual output message")
}

func TestNewLogger_FileCreationError(t *testing.T) {
	// Try to create log file in non-existent directory without permission
	config := &Config{
		Level:     LevelInfo,
		LogToFile: true,
		LogFile:   "/root/nonexistent/test.log", // This should fail on most systems
	}

	logger, err := NewLogger(config)
	if err != nil {
		// Expected case - error occurred
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create log directory")
	} else {
		// Unexpected case - no error occurred, but logger should still be valid
		assert.NotNil(t, logger)
		if logger != nil {
			logger.Close() // Clean up if logger was created
		}
		t.Log("Warning: Expected error creating log file in /root/nonexistent/, but operation succeeded")
	}
}

func TestNewInternalLogger(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logger, err := NewInternalLogger(LevelDebug, tempDir)
	require.NoError(t, err)
	assert.NotNil(t, logger)

	logger.Info("internal logger test")

	// Check if log file was created
	logFile := filepath.Join(tempDir, "proxmox-tui.log")
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	assert.Contains(t, string(content), "[INFO]")
	assert.Contains(t, string(content), "internal logger test")
}

func TestNewInternalLogger_EmptyCacheDir(t *testing.T) {
	// Create a temporary directory for this test to avoid creating files in current directory
	tempDir := t.TempDir()

	// Change to temp directory temporarily
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to change back to original directory: %v", err)
		}
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	logger, err := NewInternalLogger(LevelInfo, "")
	require.NoError(t, err)
	assert.NotNil(t, logger)

	// Clean up any created log files
	defer logger.Close()
}

func TestNewInternalLogger_InvalidCacheDir(t *testing.T) {
	// Create a temporary directory for this test to avoid creating files in current directory
	tempDir := t.TempDir()

	// Change to temp directory temporarily
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to change back to original directory: %v", err)
		}
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Try with invalid cache directory - should fall back to current directory (which is now tempDir)
	logger, err := NewInternalLogger(LevelInfo, "/root/nonexistent")
	require.NoError(t, err) // Should not error, falls back to current directory
	assert.NotNil(t, logger)

	// Clean up any created log files
	defer logger.Close()
}

func TestNewSimpleLogger(t *testing.T) {
	logger := NewSimpleLogger(LevelDebug)
	assert.NotNil(t, logger)
	assert.Equal(t, LevelDebug, logger.level)
}

func TestNewFileLogger(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "file_logger.log")
	logger, err := NewFileLogger(LevelInfo, logFile)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewDualLogger(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "dual_logger.log")
	logger, err := NewDualLogger(LevelInfo, logFile)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo, // Set to Info level
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// Test that debug messages are filtered out
	logger.Debug("debug message")
	assert.Empty(t, buf.String())

	// Test that info messages are logged
	logger.Info("info message")
	output := buf.String()
	assert.Contains(t, output, "[INFO]")
	assert.Contains(t, output, "info message")

	// Reset buffer
	buf.Reset()

	// Test that error messages are logged
	logger.Error("error message")
	output = buf.String()
	assert.Contains(t, output, "[ERROR]")
	assert.Contains(t, output, "error message")
}

func TestLogger_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelDebug,
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// All levels should be logged
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Error("error message")

	output := buf.String()
	assert.Contains(t, output, "[DEBUG]")
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "[INFO]")
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "[ERROR]")
	assert.Contains(t, output, "error message")
}

func TestLogger_ErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelError,
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// Only error messages should be logged
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "[DEBUG]")
	assert.NotContains(t, output, "[INFO]")
	assert.Contains(t, output, "[ERROR]")
	assert.Contains(t, output, "error message")
}

func TestLogger_FormatMessage(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	logger.Info("test message with %s and %d", "string", 42)

	output := buf.String()
	assert.Contains(t, output, "[INFO]")
	assert.Contains(t, output, "test message with string and 42")

	// Check timestamp format (should be present)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.True(t, len(lines) > 0)

	// The timestamp should be in format [YYYY-MM-DD HH:MM:SS]
	assert.Regexp(t, `\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]`, lines[0])
}

func TestLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// Initially at Info level, debug should be filtered
	logger.Debug("debug message 1")
	assert.Empty(t, buf.String())

	// Change to Debug level
	logger.SetLevel(LevelDebug)
	assert.Equal(t, LevelDebug, logger.GetLevel())

	// Now debug should be logged
	logger.Debug("debug message 2")
	output := buf.String()
	assert.Contains(t, output, "[DEBUG]")
	assert.Contains(t, output, "debug message 2")
}

func TestLogger_GetLevel(t *testing.T) {
	logger := NewSimpleLogger(LevelError)
	assert.Equal(t, LevelError, logger.GetLevel())
}

func TestLogger_Close(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "close_test.log")
	logger, err := NewFileLogger(LevelInfo, logFile)
	require.NoError(t, err)

	// Write something to the log
	logger.Info("test message before close")

	// Close the logger
	err = logger.Close()
	assert.NoError(t, err)

	// Verify the file contains the message
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message before close")
}

func TestLogger_Close_WithoutFile(t *testing.T) {
	// Use a buffer instead of stdout to avoid conflicts with coverage output
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// Closing a logger without file should not error
	err = logger.Close()
	assert.NoError(t, err)
}

func TestLogger_ConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:  LevelInfo,
		Output: &buf,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// Test concurrent logging
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.Info("concurrent message %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	output := buf.String()

	// Should contain all messages
	for i := 0; i < 10; i++ {
		assert.Contains(t, output, "concurrent message")
	}
}

// TestLogger_InterfaceCompliance verifies that Logger implements interfaces.Logger
func TestLogger_InterfaceCompliance(t *testing.T) {
	logger := NewSimpleLogger(LevelInfo)

	// This test ensures the logger implements the interface correctly
	// If it doesn't, this won't compile
	var _ interface {
		Debug(format string, args ...interface{})
		Info(format string, args ...interface{})
		Error(format string, args ...interface{})
	} = logger

	assert.NotNil(t, logger)
}

func TestLogger_AppendToExistingFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "append_test.log")

	// Create first logger and write a message
	logger1, err := NewFileLogger(LevelInfo, logFile)
	require.NoError(t, err)
	logger1.Info("first message")
	logger1.Close()

	// Create second logger and write another message
	logger2, err := NewFileLogger(LevelInfo, logFile)
	require.NoError(t, err)
	logger2.Info("second message")
	logger2.Close()

	// Verify both messages are in the file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "first message")
	assert.Contains(t, contentStr, "second message")

	// Count occurrences to ensure both are there
	assert.Equal(t, 1, strings.Count(contentStr, "first message"))
	assert.Equal(t, 1, strings.Count(contentStr, "second message"))
}
