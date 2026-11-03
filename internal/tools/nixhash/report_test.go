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

func dispatchedReportScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Needs       []string          `yaml:"needs"`
			If          string            `yaml:"if"`
			Permissions map[string]string `yaml:"permissions"`
			Steps       []refreshStep     `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["report-dispatched-ci"]
	if job.If != "always() && github.event_name == 'workflow_dispatch'" {
		t.Fatal("report must run after failures, only for dispatched CI")
	}
	if !slices.Equal(job.Needs, []string{"lint", "test", "build"}) {
		t.Fatal("report must wait for lint, tests, and every platform build")
	}
	if job.Permissions["statuses"] != "write" || job.Permissions["contents"] != "read" || len(job.Permissions) != 2 {
		t.Fatal("report must only request read-only contents and status publishing")
	}
	if len(job.Steps) != 1 || job.Steps[0].Uses != "" || job.Steps[0].Env["NEEDS_RESULTS"] != "${{ toJSON(needs) }}" {
		t.Fatal("report must remain API-only, using actual prerequisite job results")
	}
	return job.Steps[0].Run
}

func TestDispatchedCIReport(t *testing.T) {
	script := dispatchedReportScript(t)
	if runtime.GOOS == "windows" {
		t.Skip("workflow requires a Unix shell")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is unavailable")
	}
	for _, tc := range []struct {
		name      string
		results   string
		state     string
		apiFail   bool
		wantError bool
	}{
		{name: "success", results: `{"lint":{"result":"success"},"test":{"result":"success"},"build":{"result":"success"}}`, state: "success"},
		{name: "lint failure", results: `{"lint":{"result":"failure"},"test":{"result":"success"},"build":{"result":"success"}}`, state: "failure"},
		{name: "test failure", results: `{"lint":{"result":"success"},"test":{"result":"failure"},"build":{"result":"skipped"}}`, state: "failure"},
		{name: "build failure", results: `{"lint":{"result":"success"},"test":{"result":"success"},"build":{"result":"failure"}}`, state: "failure"},
		{name: "cancelled build", results: `{"lint":{"result":"success"},"test":{"result":"success"},"build":{"result":"cancelled"}}`, state: "failure"},
		{name: "missing results", results: `{}`, state: "failure"},
		{name: "invalid results", results: `invalid`, state: "failure"},
		{name: "API failure", results: `{}`, state: "failure", apiFail: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			callsPath := filepath.Join(dir, "calls")
			mock := "#!" + bash + "\n" + `set -euo pipefail
printf '%s\n' "$*" > "$CALLS_PATH"
[[ "$*" == *"state=$EXPECTED_STATE"* ]] || exit 42
[[ "$API_FAIL" != true ]] || exit 1
`
			// #nosec G306 -- executable mock in a private temporary directory.
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(mock), 0o700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// #nosec G204 -- checked-in workflow script, mocked gh, and fixed test inputs.
			cmd := exec.CommandContext(ctx, bash, "-c", script)
			apiFail := "false"
			if tc.apiFail {
				apiFail = "true"
			}
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"CALLS_PATH="+callsPath, "EXPECTED_STATE="+tc.state, "API_FAIL="+apiFail,
				"GH_REPO=owner/repo", "GITHUB_SHA=0123456789abcdef0123456789abcdef01234567",
				"NEEDS_RESULTS="+tc.results, "RUN_URL=https://github.com/owner/repo/actions/runs/456")
			output, runErr := cmd.CombinedOutput()
			if (runErr != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %t\n%s", runErr, tc.wantError, output)
			}
			// #nosec G304 -- reads only this test's private API-call fixture.
			calls, err := os.ReadFile(callsPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"statuses/0123456789abcdef0123456789abcdef01234567", "context=CI / Final commit", "target_url=https://github.com/owner/repo/actions/runs/456"} {
				if !strings.Contains(string(calls), want) {
					t.Errorf("missing %q in API call: %s", want, calls)
				}
			}
		})
	}
}
