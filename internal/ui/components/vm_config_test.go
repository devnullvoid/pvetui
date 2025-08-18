package components

import (
	"strings"
	"testing"
)

func TestHostnameValidation(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		expected bool
	}{
		// Valid hostnames
		{"simple lowercase", "test", true},
		{"with numbers", "test123", true},
		{"with hyphens", "test-vm", true},
		{"with multiple hyphens", "test-vm-01", true},
		{"single letter", "a", true},
		{"single digit", "1", true},
		{"mixed case", "TestVM", true},
		{"max length", "a" + strings.Repeat("b", 62), true}, // 63 characters

		// Invalid hostnames
		{"empty string", "", false},
		{"starts with hyphen", "-test", false},
		{"ends with hyphen", "test-", false},
		{"starts and ends with hyphen", "-test-", false},
		{"contains underscore", "test_vm", false},
		{"contains space", "test vm", false},
		{"contains special chars", "test@vm", false},
		{"too long", "a" + strings.Repeat("b", 63), false}, // 64 characters
		{"only hyphens", "---", false},
		{"single hyphen", "-", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidHostname(tt.hostname)
			if result != tt.expected {
				t.Errorf("isValidHostname(%q) = %v, want %v", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestHostnameCharValidation(t *testing.T) {
	tests := []struct {
		name     string
		char     rune
		expected bool
	}{
		// Valid characters
		{"lowercase a", 'a', true},
		{"lowercase z", 'z', true},
		{"uppercase A", 'A', true},
		{"uppercase Z", 'Z', true},
		{"digit 0", '0', true},
		{"digit 9", '9', true},
		{"hyphen", '-', true},

		// Invalid characters
		{"underscore", '_', false},
		{"space", ' ', false},
		{"at symbol", '@', false},
		{"period", '.', false},
		{"exclamation", '!', false},
		{"hash", '#', false},
		{"dollar", '$', false},
		{"percent", '%', false},
		{"ampersand", '&', false},
		{"asterisk", '*', false},
		{"plus", '+', false},
		{"equals", '=', false},
		{"question", '?', false},
		{"pipe", '|', false},
		{"backslash", '\\', false},
		{"forward slash", '/', false},
		{"backtick", '`', false},
		{"tilde", '~', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidHostnameChar(tt.char)
			if result != tt.expected {
				t.Errorf("isValidHostnameChar(%q) = %v, want %v", string(tt.char), result, tt.expected)
			}
		})
	}
}
