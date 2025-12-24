package bootstrap

import (
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
