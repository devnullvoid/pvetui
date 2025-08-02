// Package logger provides a comprehensive logging system designed for TUI applications.
// It supports multiple log levels, file-based logging, and configurable output destinations.
// The logger is designed to avoid stdout interference with terminal user interfaces.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/devnullvoid/proxmox-tui/pkg/api/interfaces"
)

// Level represents the logging level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelError
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger implements the interfaces.Logger interface with configurable output and levels.
type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	errorLogger *log.Logger
	level       Level
	output      io.Writer
}

// Config holds configuration for the logger.
type Config struct {
	Level      Level
	Output     io.Writer
	LogToFile  bool
	LogFile    string
	TimeFormat string
}

// NewInternalLogger creates a logger that stores logs in the specified cache directory
// This is designed for TUI applications where stdout logging would interfere with the UI.
func NewInternalLogger(level Level, cacheDir string) (*Logger, error) {
	// Use the provided cache directory for log files
	logsDir := cacheDir
	if logsDir == "" {
		// Fallback to current directory if no cache dir provided
		logsDir = "."
	}

	if err := os.MkdirAll(logsDir, 0o750); err != nil {
		// If we can't create cache directory, fall back to current directory
		logsDir = "."
	}

	logFile := filepath.Join(logsDir, "proxmox-tui.log")
	config := &Config{
		Level:     level,
		LogToFile: true,
		LogFile:   logFile,
	}

	return NewLogger(config)
}

// DefaultConfig returns a default logger configuration.
func DefaultConfig() *Config {
	return &Config{
		Level:      LevelInfo,
		Output:     os.Stdout,
		LogToFile:  false,
		TimeFormat: "2006-01-02 15:04:05",
	}
}

// NewLogger creates a new logger with the given configuration.
func NewLogger(config *Config) (*Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	output := config.Output
	if output == nil {
		output = os.Stdout
	}

	// If logging to file is enabled, set up file output
	if config.LogToFile && config.LogFile != "" {
		// Ensure the directory exists
		dir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open the log file
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		// Use both stdout and file if output is stdout, otherwise just file
		if config.Output == os.Stdout {
			output = io.MultiWriter(os.Stdout, file)
		} else {
			output = file
		}
	}

	// Create individual loggers for each level with appropriate prefixes
	debugLogger := log.New(output, "", 0)
	infoLogger := log.New(output, "", 0)
	errorLogger := log.New(output, "", 0)

	return &Logger{
		debugLogger: debugLogger,
		infoLogger:  infoLogger,
		errorLogger: errorLogger,
		level:       config.Level,
		output:      output,
	}, nil
}

// NewSimpleLogger creates a logger that outputs to stdout with the given level.
func NewSimpleLogger(level Level) *Logger {
	config := &Config{
		Level:  level,
		Output: os.Stdout,
	}
	logger, _ := NewLogger(config) // Safe to ignore error with this config

	return logger
}

// NewFileLogger creates a logger that outputs to a file with the given level.
func NewFileLogger(level Level, logFile string) (*Logger, error) {
	config := &Config{
		Level:     level,
		LogToFile: true,
		LogFile:   logFile,
	}

	return NewLogger(config)
}

// NewDualLogger creates a logger that outputs to both stdout and a file.
func NewDualLogger(level Level, logFile string) (*Logger, error) {
	config := &Config{
		Level:     level,
		Output:    os.Stdout,
		LogToFile: true,
		LogFile:   logFile,
	}

	return NewLogger(config)
}

// formatMessage creates a formatted log message with timestamp and level.
func (l *Logger) formatMessage(level Level, format string, args ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	return fmt.Sprintf("[%s] [%s] %s", timestamp, level.String(), message)
}

// Debug logs a debug message (implements interfaces.Logger).
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		message := l.formatMessage(LevelDebug, format, args...)
		l.debugLogger.Println(message)
	}
}

// Info logs an info message (implements interfaces.Logger).
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		message := l.formatMessage(LevelInfo, format, args...)
		l.infoLogger.Println(message)
	}
}

// Error logs an error message (implements interfaces.Logger).
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= LevelError {
		message := l.formatMessage(LevelError, format, args...)
		l.errorLogger.Println(message)
	}
}

// SetLevel changes the logging level.
func (l *Logger) SetLevel(level Level) {
	l.level = level
}

// GetLevel returns the current logging level.
func (l *Logger) GetLevel() Level {
	return l.level
}

// Close closes any file handles if the logger is writing to a file.
func (l *Logger) Close() error {
	// If output is a file, close it
	if closer, ok := l.output.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

// Verify that Logger implements the interfaces.Logger interface.
var _ interfaces.Logger = (*Logger)(nil)

// Global logger system for unified logging across all packages.
var (
	globalLogger     interfaces.Logger
	globalLoggerOnce sync.Once
	globalCacheDir   string
	globalDebugFlag  bool
)

// InitGlobalLogger initializes the global logger with the specified cache directory
// This should be called early in application initialization.
func InitGlobalLogger(level Level, cacheDir string) error {
	var err error

	globalLoggerOnce.Do(func() {
		globalCacheDir = cacheDir

		// Try to create internal logger with file output
		globalLogger, err = NewInternalLogger(level, cacheDir)
		if err != nil {
			// Fallback to simple logger if file logging fails
			globalLogger = NewSimpleLogger(level)
		}
	})

	return err
}

// InitGlobalLoggerWithValidation initializes the global logger with cache directory validation
// This is a convenience function that validates the cache directory before initializing.
func InitGlobalLoggerWithValidation(level Level, cacheDir string) error {
	// Validate cache directory if provided
	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o750); err != nil {
			// If we can't create the directory, fall back to simple logger
			globalLoggerOnce.Do(func() {
				globalLogger = NewSimpleLogger(level)
			})
			return fmt.Errorf("failed to create cache directory: %w", err)
		}

		// Test write access by creating a temporary file
		testFile := filepath.Join(cacheDir, ".write_test")
		if file, err := os.Create(testFile); err == nil {
			_ = file.Close()
			_ = os.Remove(testFile) // Clean up test file
		} else {
			// If we can't write to the directory, fall back to simple logger
			globalLoggerOnce.Do(func() {
				globalLogger = NewSimpleLogger(level)
			})
			return fmt.Errorf("cache directory not writable: %w", err)
		}
	}

	return InitGlobalLogger(level, cacheDir)
}

// GetGlobalLogger returns the global logger instance
// If not initialized, it creates a simple logger with Info level.
func GetGlobalLogger() interfaces.Logger {
	if globalLogger == nil {
		// Create a fallback logger if global logger wasn't initialized
		globalLogger = NewSimpleLogger(LevelInfo)
	}

	return globalLogger
}

// SetDebugEnabled sets the global debug flag for the logger package.
// This should be called during application initialization to enable debug logging.
func SetDebugEnabled(enabled bool) {
	globalDebugFlag = enabled
}

// GetPackageLogger returns a logger for a specific package using the global cache directory
// This ensures all packages log to the same unified log file.
func GetPackageLogger(packageName string) interfaces.Logger {
	level := LevelInfo
	if globalDebugFlag {
		level = LevelDebug
	}

	// Use global cache directory if available
	cacheDir := globalCacheDir
	if cacheDir == "" {
		cacheDir = "."
	}

	logger, err := NewInternalLogger(level, cacheDir)
	if err != nil {
		// Fallback to simple logger if file logging fails
		return NewSimpleLogger(level)
	}

	return logger
}

// GetPackageLoggerConcrete returns a concrete Logger instance for packages that need the specific type
// This ensures all packages log to the same unified log file while maintaining type compatibility.
func GetPackageLoggerConcrete(packageName string) *Logger {
	level := LevelInfo
	if globalDebugFlag {
		level = LevelDebug
	}

	// Use global cache directory if available
	cacheDir := globalCacheDir
	if cacheDir == "" {
		cacheDir = "."
	}

	logger, err := NewInternalLogger(level, cacheDir)
	if err != nil {
		// Fallback to simple logger if file logging fails
		return NewSimpleLogger(level)
	}

	return logger
}
