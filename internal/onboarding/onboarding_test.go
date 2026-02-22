package onboarding

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOnboardingTarget(t *testing.T) {
	t.Run("empty path uses default", func(t *testing.T) {
		target, err := resolveOnboardingTarget("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if target.exists {
			t.Fatalf("expected default target to be marked as not existing")
		}
		if target.path == "" {
			t.Fatalf("expected default target path to be set")
		}
	})

	t.Run("existing config path", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		target, err := resolveOnboardingTarget(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !target.exists {
			t.Fatalf("expected target to exist")
		}
		if target.path != path {
			t.Fatalf("expected target path %q, got %q", path, target.path)
		}
	})

	t.Run("missing config path", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "missing.yml")

		target, err := resolveOnboardingTarget(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if target.exists {
			t.Fatalf("expected missing target to be marked as not existing")
		}
		if target.path != path {
			t.Fatalf("expected target path %q, got %q", path, target.path)
		}
	})
}

func TestIsKeyBindingValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "keybinding validation error", err: errors.New("key binding global_menu uses reserved key Escape"), want: true},
		{name: "other validation error", err: errors.New("address is required"), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isKeyBindingValidationError(tc.err)
			if got != tc.want {
				t.Fatalf("isKeyBindingValidationError() = %v, want %v", got, tc.want)
			}
		})
	}
}
