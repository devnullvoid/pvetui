package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "megabytes",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "terabytes",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TB",
		},
		{
			name:     "fractional gigabytes",
			bytes:    1536 * 1024 * 1024, // 1.5 GB
			expected: "1.5 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int64
		expected string
	}{
		{
			name:     "zero seconds",
			seconds:  0,
			expected: "0s",
		},
		{
			name:     "seconds only",
			seconds:  45,
			expected: "45s",
		},
		{
			name:     "minutes and seconds",
			seconds:  125, // 2m 5s
			expected: "2m 5s",
		},
		{
			name:     "hours, minutes, and seconds",
			seconds:  3665, // 1h 1m 5s
			expected: "1h 1m 5s",
		},
		{
			name:     "days, hours, minutes, and seconds",
			seconds:  90061, // 1d 1h 1m 1s
			expected: "1d 1h 1m 1s",
		},
		{
			name:     "exact minutes",
			seconds:  120, // 2m 0s
			expected: "2m",
		},
		{
			name:     "exact hours",
			seconds:  3600, // 1h 0m 0s
			expected: "1h",
		},
		{
			name:     "exact days",
			seconds:  86400, // 1d 0h 0m 0s
			expected: "1d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUptime(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

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
			name:        "valid float64",
			input:       float64(456),
			expected:    456,
			expectError: false,
		},
		{
			name:        "valid string number",
			input:       "789",
			expected:    789,
			expectError: false,
		},
		{
			name:        "invalid string",
			input:       "not-a-number",
			expected:    0,
			expectError: true,
		},
		{
			name:        "nil input",
			input:       nil,
			expected:    0,
			expectError: true,
		},
		{
			name:        "negative number",
			input:       -123,
			expected:    -123,
			expectError: false,
		},
		{
			name:        "zero",
			input:       0,
			expected:    0,
			expectError: false,
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

func TestSafeStringValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string input",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "integer input",
			input:    123,
			expected: "123",
		},
		{
			name:     "float input",
			input:    123.45,
			expected: "123.45",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeStringValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeFloatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{
			name:     "float64 input",
			input:    123.45,
			expected: 123.45,
		},
		{
			name:     "integer input",
			input:    123,
			expected: 123.0,
		},
		{
			name:     "string number",
			input:    "123.45",
			expected: 123.45,
		},
		{
			name:     "string integer",
			input:    "123",
			expected: 123.0,
		},
		{
			name:     "invalid string",
			input:    "not-a-number",
			expected: 0.0,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: 0.0,
		},
		{
			name:     "boolean input",
			input:    true,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeFloatValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeBoolValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{
			name:     "boolean true",
			input:    true,
			expected: true,
		},
		{
			name:     "boolean false",
			input:    false,
			expected: false,
		},
		{
			name:     "string true",
			input:    "true",
			expected: true,
		},
		{
			name:     "string false",
			input:    "false",
			expected: false,
		},
		{
			name:     "string 1",
			input:    "1",
			expected: true,
		},
		{
			name:     "string 0",
			input:    "0",
			expected: false,
		},
		{
			name:     "integer 1",
			input:    1,
			expected: true,
		},
		{
			name:     "integer 0",
			input:    0,
			expected: false,
		},
		{
			name:     "float 1.0",
			input:    1.0,
			expected: true,
		},
		{
			name:     "float 0.0",
			input:    0.0,
			expected: false,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: false,
		},
		{
			name:     "invalid string",
			input:    "invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeBoolValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
