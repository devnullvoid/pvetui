package winget

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type manifest struct {
	Identifier     string   `yaml:"PackageIdentifier"`
	Version        string   `yaml:"PackageVersion"`
	ManifestType   string   `yaml:"ManifestType"`
	Schema         string   `yaml:"ManifestVersion"`
	DefaultLocale  string   `yaml:"DefaultLocale"`
	PackageLocale  string   `yaml:"PackageLocale"`
	Publisher      string   `yaml:"Publisher"`
	License        string   `yaml:"License"`
	LicenseURL     string   `yaml:"LicenseUrl"`
	ReleaseURL     string   `yaml:"ReleaseNotesUrl"`
	InstallerType  string   `yaml:"InstallerType"`
	NestedType     string   `yaml:"NestedInstallerType"`
	Scope          string   `yaml:"Scope"`
	Upgrade        string   `yaml:"UpgradeBehavior"`
	Commands       []string `yaml:"Commands"`
	ReleaseDate    string   `yaml:"ReleaseDate"`
	InstallerFiles []struct {
		Path  string `yaml:"RelativeFilePath"`
		Alias string `yaml:"PortableCommandAlias"`
	} `yaml:"NestedInstallerFiles"`
	Installers []struct {
		Architecture string `yaml:"Architecture"`
		Scope        string `yaml:"Scope"`
		URL          string `yaml:"InstallerUrl"`
		SHA256       string `yaml:"InstallerSha256"`
	} `yaml:"Installers"`
}

func TestManifestSets(t *testing.T) {
	const root = "../../../packaging/winget/manifests/d/devnullvoid/pvetui"
	versions, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("no WinGet manifests found")
	}
	for _, version := range versions {
		t.Run(version.Name(), func(t *testing.T) {
			if !version.IsDir() {
				t.Fatal("package root must contain only version directories")
			}
			dir := filepath.Join(root, version.Name())
			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 3 {
				t.Fatalf("expected three manifest files, got %d", len(files))
			}
			for _, tc := range []struct{ suffix, kind string }{
				{suffix: ".yaml", kind: "version"},
				{suffix: ".installer.yaml", kind: "installer"},
				{suffix: ".locale.en-US.yaml", kind: "defaultLocale"},
			} {
				t.Run(tc.kind, func(t *testing.T) {
					// #nosec G304 -- reads repository manifest fixtures beneath the fixed package root.
					data, err := os.ReadFile(filepath.Join(dir, "devnullvoid.pvetui"+tc.suffix))
					if err != nil {
						t.Fatal(err)
					}
					var m manifest
					if err := yaml.Unmarshal(data, &m); err != nil {
						t.Fatal(err)
					}
					if m.Identifier != "devnullvoid.pvetui" || m.Version != version.Name() || m.ManifestType != tc.kind || m.Schema != "1.28.0" {
						t.Fatal("manifest identity, version, kind, or schema does not match the directory")
					}
					switch tc.kind {
					case "version":
						if m.DefaultLocale != "en-US" {
							t.Fatal("default locale must match the en-US manifest")
						}
					case "defaultLocale":
						if m.PackageLocale != "en-US" || m.Publisher == "" || m.License != "MIT" {
							t.Fatal("missing locale, publisher, or MIT license")
						}
						base := "https://github.com/devnullvoid/pvetui/"
						if m.LicenseURL != base+"blob/v"+m.Version+"/LICENSE" || m.ReleaseURL != base+"releases/tag/v"+m.Version {
							t.Fatal("license and release notes must be version-pinned")
						}
					case "installer":
						checkInstaller(t, m)
					}
				})
			}
		})
	}
}

func checkInstaller(t *testing.T, m manifest) {
	t.Helper()
	if m.InstallerType != "zip" || m.NestedType != "portable" || m.Scope != "" || m.Upgrade != "uninstallPrevious" {
		t.Fatal("installer must remain a portable ZIP with no manifest Scope and uninstallPrevious upgrades")
	}
	if len(m.InstallerFiles) != 1 || m.InstallerFiles[0].Path != "pvetui.exe" || m.InstallerFiles[0].Alias != "pvetui" || !slices.Contains(m.Commands, "pvetui") {
		t.Fatal("installer must expose the root executable as pvetui")
	}
	if _, err := time.Parse(time.DateOnly, m.ReleaseDate); err != nil {
		t.Fatalf("invalid release date: %v", err)
	}
	if len(m.Installers) != 1 || m.Installers[0].Architecture != "x64" {
		t.Fatal("initial candidate supports only x64; test additional architectures before adding them")
	}
	installer := m.Installers[0]
	if installer.Scope != "" {
		t.Fatal("portable installer scope must be selected with --scope, not declared in the manifest")
	}
	wantURL := "https://github.com/devnullvoid/pvetui/releases/download/v" + m.Version + "/pvetui_" + m.Version + "_windows_amd64.zip"
	if installer.URL != wantURL {
		t.Fatalf("installer URL = %q, want %q", installer.URL, wantURL)
	}
	if !regexp.MustCompile(`^[A-Fa-f0-9]{64}$`).MatchString(installer.SHA256) {
		t.Fatal("installer must include a full SHA-256 checksum")
	}
}
