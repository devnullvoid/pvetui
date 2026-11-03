package nixhash

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type refreshStep struct {
	ID   string            `yaml:"id"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	Env  map[string]string `yaml:"env"`
}

type refreshJob struct {
	Needs       string            `yaml:"needs"`
	If          string            `yaml:"if"`
	Permissions map[string]string `yaml:"permissions"`
	Outputs     map[string]string `yaml:"outputs"`
	Steps       []refreshStep     `yaml:"steps"`
}

func loadRefreshJobs(t *testing.T) map[string]refreshJob {
	t.Helper()
	data, err := os.ReadFile("../../../.github/workflows/update-nix-hash.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]refreshJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow.Jobs
}

func TestRefreshWorkflowSafety(t *testing.T) {
	jobs := loadRefreshJobs(t)
	refresh := jobs["refresh-ci"]
	if refresh.Needs != "update-nix-hash" || refresh.If != "needs.update-nix-hash.outputs.committed == 'true' && (github.event_name == 'push' || github.event.pull_request.head.repo.full_name == github.repository)" {
		t.Fatal("refresh must require an actual verified commit and exclude fork PRs")
	}
	for name, want := range map[string]string{"contents": "read", "actions": "write", "statuses": "write"} {
		if refresh.Permissions[name] != want {
			t.Errorf("refresh permission %s = %q, want %q", name, refresh.Permissions[name], want)
		}
	}
	if len(refresh.Steps) != 1 || refresh.Steps[0].Uses != "" {
		t.Fatal("refresh must remain an API-only job without a checkout or external action")
	}
	if refresh.Steps[0].Env["VERIFIED_SHA"] != "${{ needs.update-nix-hash.outputs.commit }}" {
		t.Fatal("refresh must use the hash actually committed by the updater")
	}
	update := jobs["update-nix-hash"]
	if update.Outputs["committed"] != "${{ steps.commit.outputs.changes_detected }}" || update.Outputs["commit"] != "${{ steps.commit.outputs.commit_hash }}" {
		t.Fatal("update job must export the auto-commit action's result")
	}
	if !slices.ContainsFunc(update.Steps, func(step refreshStep) bool {
		return step.ID == "commit" && strings.HasPrefix(step.Uses, "stefanzweifel/git-auto-commit-action@")
	}) {
		t.Fatal("commit outputs must come from the auto-commit action")
	}
	data, err := os.ReadFile("../../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var ci struct {
		On map[string]any `yaml:"on"`
	}
	if err := yaml.Unmarshal(data, &ci); err != nil {
		t.Fatal(err)
	}
	if _, ok := ci.On["workflow_dispatch"]; !ok {
		t.Fatal("CI must accept explicit dispatches after bot commits")
	}
}

func TestRefreshVerifiedCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("workflow requires a Unix shell")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	jobs := loadRefreshJobs(t)
	script := jobs["refresh-ci"].Steps[0].Run
	const sha = "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		name      string
		scenario  string
		sha       string
		wantCalls int
		wantError bool
	}{
		{name: "current commit", sha: sha, wantCalls: 4},
		{name: "branch advanced", scenario: "advanced", sha: sha, wantCalls: 1},
		{name: "invalid sha", sha: "not-a-sha", wantError: true},
		{name: "lookup fails", scenario: "lookup-error", sha: sha, wantCalls: 1, wantError: true},
		{name: "status fails", scenario: "status-error", sha: sha, wantCalls: 2, wantError: true},
		{name: "pending status fails", scenario: "pending-error", sha: sha, wantCalls: 3, wantError: true},
		{name: "dispatch fails", scenario: "dispatch-error", sha: sha, wantCalls: 4, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			callsPath := filepath.Join(dir, "calls")
			if err := os.WriteFile(callsPath, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			mock := "#!" + bash + "\n" + `set -euo pipefail
printf '%s\n' "$*" >> "$CALLS_PATH"
case "$*" in
  *'/git/ref/heads/'*)
    [[ "$SCENARIO" != lookup-error ]] || exit 1
    if [[ "$SCENARIO" == advanced ]]; then echo different; else echo "$VERIFIED_SHA"; fi
    ;;
  *'/statuses/'*)
    [[ "$SCENARIO" != status-error ]] || exit 1
    if [[ "$*" == *'state=pending'* && "$SCENARIO" == pending-error ]]; then exit 1; fi
    ;;
  *'/actions/workflows/ci.yml/dispatches'*) [[ "$SCENARIO" != dispatch-error ]] || exit 1 ;;
  *) exit 42 ;;
esac
`
			// #nosec G306 -- executable fixture in a private temporary directory.
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(mock), 0o700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// #nosec G204 -- executes the checked-in workflow with a mock gh and fixed test inputs.
			cmd := exec.CommandContext(ctx, bash, "-c", script)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"CALLS_PATH="+callsPath, "SCENARIO="+tc.scenario, "VERIFIED_SHA="+tc.sha,
				"GH_REPO=owner/repo", "TARGET_REF=dependabot/go_modules/develop/example-1.2.3",
				"RUN_URL=https://github.com/owner/repo/actions/runs/123")
			output, runErr := cmd.CombinedOutput()
			if (runErr != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %t\n%s", runErr, tc.wantError, output)
			}
			// #nosec G304 -- reads only the mock API log in this test's temporary directory.
			calls, err := os.ReadFile(callsPath)
			if err != nil {
				t.Fatal(err)
			}
			if count := strings.Count(string(calls), "\n"); count != tc.wantCalls {
				t.Fatalf("API calls = %d, want %d\n%s", count, tc.wantCalls, calls)
			}
			if tc.wantCalls == 4 {
				for _, want := range []string{"statuses/" + sha, "context=Nix / Verified hash", "context=CI / Final commit", "state=pending", "ref=dependabot/go_modules/develop/example-1.2.3"} {
					if !strings.Contains(string(calls), want) {
						t.Errorf("missing %q in API calls: %s", want, calls)
					}
				}
			}
		})
	}
}
