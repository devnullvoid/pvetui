package releaseconfig

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type releaseConfig struct {
	Casks    []map[string]any `yaml:"homebrew_casks"`
	Archives []map[string]any `yaml:"archives"`
}

func loadReleaseConfig(t *testing.T) releaseConfig {
	t.Helper()
	data, err := os.ReadFile("../../../.goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var config releaseConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Casks) != 1 || len(config.Archives) != 1 {
		t.Fatal("expected one source cask and archive configuration")
	}
	return config
}

func TestHomebrewCaskConfiguration(t *testing.T) {
	config := loadReleaseConfig(t)
	data, err := yaml.Marshal(config.Casks[0])
	if err != nil {
		t.Fatal(err)
	}
	var cask struct {
		CustomBlock string `yaml:"custom_block"`
		Hooks       struct {
			Post struct {
				Install      string `yaml:"install"`
				InstallSteps string `yaml:"install_steps"`
			} `yaml:"post"`
		} `yaml:"hooks"`
	}
	if err := yaml.Unmarshal(data, &cask); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cask.CustomBlock, "rubocop:disable") || cask.Hooks.Post.Install != "" {
		t.Fatal("cask must not emit obsolete cop suppressions or legacy postflight hooks")
	}
	for _, want := range []string{"on_macos do", `run "/usr/bin/xattr"`, `"{{ .StagedPath }}/pvetui"`} {
		if !strings.Contains(cask.Hooks.Post.InstallSteps, want) {
			t.Errorf("install_steps is missing %q", want)
		}
	}
}

func TestGeneratedHomebrewCask(t *testing.T) {
	if os.Getenv("PVETUI_GORELEASER_TEST") != "true" {
		t.Skip("set PVETUI_GORELEASER_TEST=true to generate a snapshot fixture with GoReleaser")
	}
	if runtime.GOOS == "windows" {
		t.Skip("GoReleaser snapshot fixture requires a Unix host")
	}
	config := loadReleaseConfig(t)
	repository, ok := config.Casks[0]["repository"].(map[string]any)
	if !ok {
		t.Fatal("missing cask repository configuration")
	}
	delete(repository, "token")
	config.Archives[0]["files"] = []string{"LICENSE"}
	fixture := map[string]any{
		"version": 2, "project_name": "pvetui",
		"release":  map[string]any{"github": map[string]any{"owner": "devnullvoid", "name": "pvetui"}},
		"snapshot": map[string]any{"version_template": "1.4.4"},
		"builds": []map[string]any{{
			"id": "pvetui", "main": "./main.go", "env": []string{"CGO_ENABLED=0"},
			"goos": []string{"linux", "darwin"}, "goarch": []string{"amd64", "arm64"},
		}},
		"archives": config.Archives, "homebrew_casks": config.Casks,
	}
	data, err := yaml.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{
		".goreleaser.yaml": string(data),
		"go.mod":           "module example.invalid/pvetui-release-fixture\n\ngo 1.26.0\n",
		"main.go":          "package main\n\nfunc main() {}\n",
		"LICENSE":          "Snapshot fixture only; not a redistributable pvetui build.\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	run := func(name string, args ...string) {
		t.Helper()
		// #nosec G204 -- fixed local tools and test-controlled fixture arguments; snapshot mode never publishes.
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		// Exclude publishing credentials and user Git/Go configuration from the fixture.
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.TempDir(),
			"GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off",
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s failed: %v\n%s", name, err, output)
		}
	}
	run("git", "init", "-q")
	run("git", "remote", "add", "origin", "https://github.com/devnullvoid/pvetui.git")
	run("git", "add", ".")
	run("git", "-c", "core.hooksPath=", "-c", "commit.gpgsign=false", "-c", "user.name=Release Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	run("git", "-c", "tag.gpgsign=false", "tag", "v1.4.4")
	run("goreleaser", "release", "--snapshot", "--skip=publish,sign,announce,changelog", "--parallelism=2", "--timeout=5m")
	root, err := os.OpenRoot(filepath.Join(dir, "dist"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Errorf("close snapshot output root: %v", err)
		}
	}()
	var generated []byte
	if err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() == "pvetui.rb" {
			generated, err = root.ReadFile(path)
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	for _, want := range []string{"postflight_steps do", "on_macos do", `run "/usr/bin/xattr"`, `"{{staged_path}}/pvetui"`, "zap :not_required", "on_linux do", "on_arm do", "on_intel do"} {
		if !strings.Contains(text, want) {
			t.Errorf("generated cask is missing %q", want)
		}
	}
	for _, obsolete := range []string{"rubocop:disable", "postflight do", "system_command", "No zap stanza required"} {
		if strings.Contains(text, obsolete) {
			t.Errorf("generated cask includes obsolete syntax %q", obsolete)
		}
	}
	for _, target := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"} {
		want := "https://github.com/devnullvoid/pvetui/releases/download/v#{version}/pvetui_#{version}_" + target + ".tar.gz"
		if !strings.Contains(text, want) {
			t.Errorf("generated cask is missing version-specific URL %q", want)
		}
	}
	t.Logf("Generated snapshot fixture (synthetic binaries/checksums, never published):\n%s", text)
}
