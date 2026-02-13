package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDefaultConfigPathForOS_WindowsXDGFallback(t *testing.T) {
	tmp := t.TempDir()
	appData := filepath.Join(tmp, "appdata")
	home := filepath.Join(tmp, "home")

	t.Setenv("APPDATA", appData)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	legacyConfig := filepath.Join(home, ".config", "pvetui", "config.yml")
	if err := os.MkdirAll(filepath.Dir(legacyConfig), 0o755); err != nil {
		t.Fatalf("create legacy config dir: %v", err)
	}
	if err := os.WriteFile(legacyConfig, []byte("profiles: {}\n"), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	got, found := findDefaultConfigPathForOS("windows")
	if !found {
		t.Fatalf("expected config path to be found")
	}
	if got != legacyConfig {
		t.Fatalf("expected legacy config path %q, got %q", legacyConfig, got)
	}
}

func TestFindDefaultConfigPathForOS_WindowsPrefersAppData(t *testing.T) {
	tmp := t.TempDir()
	appData := filepath.Join(tmp, "appdata")
	home := filepath.Join(tmp, "home")

	t.Setenv("APPDATA", appData)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	newConfig := filepath.Join(appData, "pvetui", "config.yml")
	if err := os.MkdirAll(filepath.Dir(newConfig), 0o755); err != nil {
		t.Fatalf("create appdata config dir: %v", err)
	}
	if err := os.WriteFile(newConfig, []byte("profiles: {}\n"), 0o600); err != nil {
		t.Fatalf("write appdata config: %v", err)
	}

	legacyConfig := filepath.Join(home, ".config", "pvetui", "config.yml")
	if err := os.MkdirAll(filepath.Dir(legacyConfig), 0o755); err != nil {
		t.Fatalf("create legacy config dir: %v", err)
	}
	if err := os.WriteFile(legacyConfig, []byte("profiles: {}\n"), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	got, found := findDefaultConfigPathForOS("windows")
	if !found {
		t.Fatalf("expected config path to be found")
	}
	if got != newConfig {
		t.Fatalf("expected appdata config path %q, got %q", newConfig, got)
	}
}

func TestGetCacheDirForOS_WindowsLegacyFallback(t *testing.T) {
	tmp := t.TempDir()
	localAppData := filepath.Join(tmp, "localappdata")
	home := filepath.Join(tmp, "home")

	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")

	legacyCache := filepath.Join(home, ".cache", "pvetui")
	if err := os.MkdirAll(legacyCache, 0o755); err != nil {
		t.Fatalf("create legacy cache dir: %v", err)
	}

	got := getCacheDirForOS("windows")
	if got != legacyCache {
		t.Fatalf("expected legacy cache path %q, got %q", legacyCache, got)
	}
}

func TestGetCacheDirForOS_WindowsPrefersLocalAppData(t *testing.T) {
	tmp := t.TempDir()
	localAppData := filepath.Join(tmp, "localappdata")
	home := filepath.Join(tmp, "home")

	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")

	preferredCache := filepath.Join(localAppData, "pvetui")
	if err := os.MkdirAll(preferredCache, 0o755); err != nil {
		t.Fatalf("create preferred cache dir: %v", err)
	}

	legacyCache := filepath.Join(home, ".cache", "pvetui")
	if err := os.MkdirAll(legacyCache, 0o755); err != nil {
		t.Fatalf("create legacy cache dir: %v", err)
	}

	got := getCacheDirForOS("windows")
	if got != preferredCache {
		t.Fatalf("expected preferred cache path %q, got %q", preferredCache, got)
	}
}
