package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDefaultConfigPathForOS_WindowsPrefersAppData(t *testing.T) {
	appData := t.TempDir()
	xdgBase := t.TempDir()

	t.Setenv("APPDATA", appData)
	t.Setenv("XDG_CONFIG_HOME", xdgBase)

	appDataPath := filepath.Join(appData, "pvetui", "config.yml")
	if err := os.MkdirAll(filepath.Dir(appDataPath), 0o750); err != nil {
		t.Fatalf("mkdir appdata: %v", err)
	}
	if err := os.WriteFile(appDataPath, []byte("appdata"), 0o600); err != nil {
		t.Fatalf("write appdata config: %v", err)
	}

	xdgPath := filepath.Join(xdgBase, "pvetui", "config.yml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o750); err != nil {
		t.Fatalf("mkdir xdg: %v", err)
	}
	if err := os.WriteFile(xdgPath, []byte("xdg"), 0o600); err != nil {
		t.Fatalf("write xdg config: %v", err)
	}

	got, ok := findDefaultConfigPathForOS("windows")
	if !ok {
		t.Fatalf("expected config path to be found")
	}
	if got != appDataPath {
		t.Fatalf("expected APPDATA path %q, got %q", appDataPath, got)
	}
}

func TestFindDefaultConfigPathForOS_WindowsFallbacksToXDG(t *testing.T) {
	xdgBase := t.TempDir()

	t.Setenv("APPDATA", "")
	t.Setenv("XDG_CONFIG_HOME", xdgBase)

	xdgPath := filepath.Join(xdgBase, "pvetui", "config.yml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o750); err != nil {
		t.Fatalf("mkdir xdg: %v", err)
	}
	if err := os.WriteFile(xdgPath, []byte("xdg"), 0o600); err != nil {
		t.Fatalf("write xdg config: %v", err)
	}

	got, ok := findDefaultConfigPathForOS("windows")
	if !ok {
		t.Fatalf("expected config path to be found")
	}
	if got != xdgPath {
		t.Fatalf("expected XDG path %q, got %q", xdgPath, got)
	}
}
