package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devnullvoid/pvetui/internal/config"
)

func TestResolveConfigPathForWizardFallback(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", baseDir)

	want := config.GetDefaultConfigPath()
	got := ResolveConfigPathForWizard("")

	if got != want {
		t.Fatalf("expected fallback path %q, got %q", want, got)
	}
}

func TestResolveConfigPathForWizardFlag(t *testing.T) {
	flagPath := filepath.Join(t.TempDir(), "custom.yml")
	got := ResolveConfigPathForWizard(flagPath)

	if got != flagPath {
		t.Fatalf("expected flag path %q, got %q", flagPath, got)
	}
}

func TestResolveConfigPathForWizardExistingDefault(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", baseDir)

	defaultPath := config.GetDefaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(defaultPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got := ResolveConfigPathForWizard("")
	if got != defaultPath {
		t.Fatalf("expected existing path %q, got %q", defaultPath, got)
	}
}
