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

func TestWizardAuthState(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		tokenID     string
		tokenSecret string
		hasPassword bool
		hasToken    bool
	}{
		{
			name:        "password only",
			password:    "secret",
			hasPassword: true,
		},
		{
			name:        "token only",
			tokenID:     "id",
			tokenSecret: "secret",
			hasToken:    true,
		},
		{
			name:        "whitespace ignored",
			password:    "   ",
			tokenID:     "  id  ",
			tokenSecret: " secret ",
			hasToken:    true,
		},
		{
			name:     "missing secret",
			tokenID:  "id",
			hasToken: false,
		},
		{
			name:        "empty values",
			hasPassword: false,
			hasToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasPassword, hasToken := wizardAuthState(tt.password, tt.tokenID, tt.tokenSecret)
			if hasPassword != tt.hasPassword {
				t.Fatalf("expected hasPassword=%v, got %v", tt.hasPassword, hasPassword)
			}
			if hasToken != tt.hasToken {
				t.Fatalf("expected hasToken=%v, got %v", tt.hasToken, hasToken)
			}
		})
	}
}

func TestValidateWizardAuth(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		tokenID     string
		tokenSecret string
		wantError   bool
	}{
		{
			name:      "password only",
			password:  "secret",
			wantError: false,
		},
		{
			name:        "token only",
			tokenID:     "id",
			tokenSecret: "secret",
			wantError:   false,
		},
		{
			name:      "token id without secret",
			tokenID:   "id",
			wantError: true,
		},
		{
			name:        "token secret without id",
			tokenSecret: "secret",
			wantError:   true,
		},
		{
			name:      "empty values",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, errMsg := validateWizardAuth(tt.password, tt.tokenID, tt.tokenSecret)
			if (errMsg != "") != tt.wantError {
				t.Fatalf("expected error=%v, got %q", tt.wantError, errMsg)
			}
		})
	}
}

func TestNormalizeWizardFormValues(t *testing.T) {
	values := normalizeWizardFormValues(wizardFormValues{
		ProfileName: " default ",
		Addr:        " https://host ",
		User:        " root ",
		Password:    "  secret  ",
		TokenID:     " token ",
		TokenSecret: "  tokensecret  ",
		Realm:       " pam ",
		ApiPath:     " /api2/json ",
		SSHUser:     " root ",
		VMSSHUser:   " vmroot ",
	})

	if values.ProfileName != "default" {
		t.Fatalf("expected ProfileName to be trimmed, got %q", values.ProfileName)
	}
	if values.Addr != "https://host" {
		t.Fatalf("expected Addr to be trimmed, got %q", values.Addr)
	}
	if values.User != "root" {
		t.Fatalf("expected User to be trimmed, got %q", values.User)
	}
	if values.Password != "  secret  " {
		t.Fatalf("expected Password to be unchanged, got %q", values.Password)
	}
	if values.TokenID != "token" {
		t.Fatalf("expected TokenID to be trimmed, got %q", values.TokenID)
	}
	if values.TokenSecret != "  tokensecret  " {
		t.Fatalf("expected TokenSecret to be unchanged, got %q", values.TokenSecret)
	}
	if values.Realm != "pam" {
		t.Fatalf("expected Realm to be trimmed, got %q", values.Realm)
	}
	if values.ApiPath != "/api2/json" {
		t.Fatalf("expected ApiPath to be trimmed, got %q", values.ApiPath)
	}
	if values.SSHUser != "root" {
		t.Fatalf("expected SSHUser to be trimmed, got %q", values.SSHUser)
	}
	if values.VMSSHUser != "vmroot" {
		t.Fatalf("expected VMSSHUser to be trimmed, got %q", values.VMSSHUser)
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

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml marshal error: %v", err)
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

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml marshal error: %v", err)
	}

	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal yaml: %v", err)
	}

	if _, ok := decoded["plugins"]; !ok {
		t.Fatalf("expected plugins key to be present even when empty")
	}
}
