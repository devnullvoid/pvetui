package nixhash

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorkflowCoverage(t *testing.T) {
	for _, name := range []string{"ci.yml", "update-nix-hash.yml"} {
		t.Run(name, func(t *testing.T) {
			// #nosec G304 -- names are fixed repository workflow fixtures, not external input.
			data, err := os.ReadFile("../../../.github/workflows/" + name)
			if err != nil {
				t.Fatal(err)
			}
			var workflow struct {
				On map[string]struct {
					Branches []string `yaml:"branches"`
					Paths    []string `yaml:"paths"`
				} `yaml:"on"`
			}
			if err := yaml.Unmarshal(data, &workflow); err != nil {
				t.Fatal(err)
			}
			for _, event := range []string{"pull_request", "push"} {
				for _, branch := range []string{"master", "develop"} {
					if !slices.Contains(workflow.On[event].Branches, branch) {
						t.Errorf("%s must cover %s on %s", name, event, branch)
					}
				}
				if name == "update-nix-hash.yml" {
					for _, path := range []string{"go.mod", "go.sum", ".go-version", "flake.nix", "flake.lock", "scripts/update-nix-hash.sh", "internal/tools/nixhash/**", ".github/workflows/update-nix-hash.yml"} {
						if !slices.Contains(workflow.On[event].Paths, path) {
							t.Errorf("%s must validate changes to %s", event, path)
						}
					}
				}
			}
		})
	}
}

func TestGoToolchainAlignment(t *testing.T) {
	version, err := os.ReadFile("../../../.go-version")
	if err != nil {
		t.Fatal(err)
	}
	flake, err := os.ReadFile("../../../flake.nix")
	if err != nil {
		t.Fatal(err)
	}
	want := "go = pkgs.go_" + strings.ReplaceAll(strings.TrimSpace(string(version)), ".", "_") + ";"
	if !strings.Contains(string(flake), want) {
		t.Errorf("flake.nix must select the toolchain matching .go-version: %s", want)
	}
}
