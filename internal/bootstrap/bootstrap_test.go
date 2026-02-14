package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
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

func TestFormatProfileAndGroupList(t *testing.T) {
	cfg := &config.Config{
		DefaultProfile: "dev",
		Profiles: map[string]config.ProfileConfig{
			"dev": {
				Addr:     "https://dev.example",
				User:     "root@pam",
				Password: "secret",
				Realm:    "pam",
				Groups:   []string{"lab"},
			},
			"prod": {
				Addr:        "https://prod.example",
				User:        "root@pam",
				TokenID:     "abc",
				TokenSecret: "xyz",
				Realm:       "pam",
				Groups:      []string{"critical", "lab"},
			},
		},
	}

	output := formatProfileAndGroupList(cfg, "lab")

	if !strings.Contains(output, "Available connection profiles:") {
		t.Fatalf("expected profiles header in output: %q", output)
	}
	if !strings.Contains(output, "dev") || !strings.Contains(output, "prod") {
		t.Fatalf("expected profile names in output: %q", output)
	}
	if !strings.Contains(output, "auth: password") || !strings.Contains(output, "auth: api-token") {
		t.Fatalf("expected authentication details in output: %q", output)
	}
	if !strings.Contains(output, "* lab (2 profiles)") {
		t.Fatalf("expected selected group marker in output: %q", output)
	}
	if !strings.Contains(output, "Default profile: dev") {
		t.Fatalf("expected default profile in output: %q", output)
	}
}

func TestFormatProfileAndGroupListNoProfiles(t *testing.T) {
	cfg := &config.Config{}
	output := formatProfileAndGroupList(cfg, "")

	if !strings.Contains(output, "(none configured)") {
		t.Fatalf("expected no profiles/groups message in output: %q", output)
	}
	if !strings.Contains(output, "Selected by current flags/config: -") {
		t.Fatalf("expected empty selected value in output: %q", output)
	}
}
