package vnc

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestOpenBrowserXdgOpenDetection(t *testing.T) {
	// Only run this test on Linux
	if runtime.GOOS != "linux" {
		t.Skip("This test is only relevant on Linux")
	}

	// Test that openBrowser returns an error when xdg-open is not found
	// We can simulate this by temporarily renaming xdg-open if it exists
	_, err := exec.LookPath("xdg-open")
	if err != nil {
		// xdg-open doesn't exist, so our function should return an error
		err := openBrowser("http://example.com")
		if err == nil {
			t.Error("Expected error when xdg-open is not found, but got nil")
		}
		if !strings.Contains(err.Error(), "xdg-open not found") {
			t.Errorf("Expected error to contain 'xdg-open not found', but got: %v", err)
		}
		return
	}

	// xdg-open exists, so our function should work (or at least not return the specific error)
	err = openBrowser("http://example.com")
	if err != nil && strings.Contains(err.Error(), "xdg-open not found") {
		t.Errorf("Unexpected 'xdg-open not found' error when xdg-open exists: %v", err)
	}
}

func TestCreateShortenedVNCURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard VNC URL",
			input:    "http://localhost:45167/vnc.html?autoconnect=true&reconnect=true&password=%5D%3E%283LV%2C.&path=vnc-proxy&resize=scale",
			expected: "http://localhost:45167/vnc-forward",
		},
		{
			name:     "Different port",
			input:    "http://localhost:12345/vnc.html?autoconnect=true",
			expected: "http://localhost:12345/vnc-forward",
		},
		{
			name:     "No localhost in URL",
			input:    "https://example.com/vnc.html",
			expected: "https://example.com/vnc.html", // Should fallback to original
		},
		{
			name:     "No path in URL",
			input:    "http://localhost:8080",
			expected: "http://localhost:8080", // Should fallback to original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createShortenedVNCURL(tt.input)
			if result != tt.expected {
				t.Errorf("createShortenedVNCURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
