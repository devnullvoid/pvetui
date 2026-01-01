package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetAgeDirOverride(t *testing.T) {
	dir := t.TempDir()
	SetAgeDirOverride(dir)
	t.Cleanup(func() { SetAgeDirOverride("") })

	encrypted, err := EncryptField("secret")
	if err != nil {
		t.Fatalf("EncryptField error: %v", err)
	}
	if encrypted == "" {
		t.Fatalf("expected encrypted value")
	}

	if _, err := os.Stat(filepath.Join(dir, ".age-identity")); err != nil {
		t.Fatalf("expected .age-identity in override dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".age-recipient")); err != nil {
		t.Fatalf("expected .age-recipient in override dir: %v", err)
	}
}
