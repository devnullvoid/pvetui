package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateDefaultConfigFileAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	gotPath, err := CreateDefaultConfigFileAt(path)
	if err != nil {
		t.Fatalf("CreateDefaultConfigFileAt error: %v", err)
	}
	if gotPath != path {
		t.Fatalf("expected path %q, got %q", path, gotPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "profiles:") {
		t.Fatalf("expected template content to include profiles, got %q", string(data))
	}

	// Second call should be a no-op and keep same path.
	gotPath, err = CreateDefaultConfigFileAt(path)
	if err != nil {
		t.Fatalf("CreateDefaultConfigFileAt second call error: %v", err)
	}
	if gotPath != path {
		t.Fatalf("expected path %q on second call, got %q", path, gotPath)
	}
}

func TestCreateDefaultConfigFileAt_EmptyPath(t *testing.T) {
	if _, err := CreateDefaultConfigFileAt(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}
