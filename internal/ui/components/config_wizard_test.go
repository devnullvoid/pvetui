package components

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/devnullvoid/pvetui/internal/config"
)

func TestConfigWizardValidation(t *testing.T) {
	cases := []struct {
		name      string
		cfg       config.Config
		wantError bool
		wantMsg   string
	}{
		{
			name:      "valid password auth",
			cfg:       config.Config{Addr: "https://host", User: "root", Password: "pw"},
			wantError: false,
		},
		{
			name:      "valid token auth",
			cfg:       config.Config{Addr: "https://host", User: "root", TokenID: "id", TokenSecret: "secret"},
			wantError: false,
		},
		{
			name:      "both password and token",
			cfg:       config.Config{Addr: "https://host", User: "root", Password: "pw", TokenID: "id", TokenSecret: "secret"},
			wantError: true,
			wantMsg:   "either password authentication or token authentication, not both",
		},
		{
			name:      "neither password nor token",
			cfg:       config.Config{Addr: "https://host", User: "root"},
			wantError: true,
			wantMsg:   "must provide either a password or a token",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hasPassword := tc.cfg.Password != ""
			hasToken := tc.cfg.TokenID != "" && tc.cfg.TokenSecret != ""

			var gotError bool

			var gotMsg string

			if hasPassword && hasToken {
				gotError = true
				gotMsg = "either password authentication or token authentication, not both"
			} else if !hasPassword && !hasToken {
				gotError = true
				gotMsg = "must provide either a password or a token"
			}

			if gotError != tc.wantError {
				t.Errorf("expected error=%v, got %v", tc.wantError, gotError)
			}

			if tc.wantError && tc.wantMsg != "" && gotMsg != tc.wantMsg {
				t.Errorf("expected msg %q, got %q", tc.wantMsg, gotMsg)
			}
		})
	}
}

func TestFindSOPSRule(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	_ = os.Mkdir(subdir, 0o755)
	// No .sops.yaml
	if findSOPSRule(subdir) {
		t.Error("expected false when no .sops.yaml present")
	}
	// Add .sops.yaml in parent
	f, err := os.Create(filepath.Join(dir, ".sops.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	_ = f.Close()

	if !findSOPSRule(subdir) {
		t.Error("expected true when .sops.yaml present in parent")
	}
}

func TestConfigToYAML_PreservesPluginsSection(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Plugins.Enabled = []string{"community-scripts"}

	data, err := configToYAML(cfg)
	if err != nil {
		t.Fatalf("configToYAML error: %v", err)
	}

	var decoded struct {
		Plugins struct {
			Enabled []string `yaml:"enabled"`
		} `yaml:"plugins"`
	}

	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal yaml: %v", err)
	}

	if len(decoded.Plugins.Enabled) != 1 || decoded.Plugins.Enabled[0] != "community-scripts" {
		t.Fatalf("expected enabled plugin to be preserved, got %v", decoded.Plugins.Enabled)
	}
}

func TestConfigToYAML_RetainsPluginsKeyWhenEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Plugins.Enabled = nil

	data, err := configToYAML(cfg)
	if err != nil {
		t.Fatalf("configToYAML error: %v", err)
	}

	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal yaml: %v", err)
	}

	if _, ok := decoded["plugins"]; !ok {
		t.Fatalf("expected plugins key to be present even when empty")
	}
}
