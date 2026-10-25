package nixhash

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUpdateVendorHash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Nix helper requires a Unix shell")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	script, err := filepath.Abs("../../../scripts/update-nix-hash.sh")
	if err != nil {
		t.Fatal(err)
	}
	const oldHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	const newHash = "sha256-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="
	for _, tc := range []struct {
		name       string
		scenario   string
		checkOnly  bool
		failSmoke  bool
		wantError  bool
		wantUpdate bool
		wantCalls  int
	}{
		{name: "current hash", scenario: "success", wantCalls: 1},
		{name: "current hash read only", scenario: "success", checkOnly: true, wantCalls: 1},
		{name: "repair and verify", scenario: "mismatch", wantUpdate: true, wantCalls: 2},
		{name: "fork stale hash", scenario: "mismatch", checkOnly: true, wantError: true, wantCalls: 1},
		{name: "unexpected build failure", scenario: "build-error", wantError: true, wantCalls: 1},
		{name: "unrelated hash mismatch", scenario: "foreign", wantError: true, wantCalls: 1},
		{name: "missing computed hash", scenario: "missing", wantError: true, wantCalls: 1},
		{name: "unchanged hash", scenario: "same", wantError: true, wantCalls: 1},
		{name: "rebuild fails and restores", scenario: "verify-error", wantError: true, wantCalls: 2},
		{name: "smoke fails and restores", scenario: "mismatch", failSmoke: true, wantError: true, wantCalls: 2},
		{name: "current hash smoke fails", scenario: "success", failSmoke: true, wantError: true, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, body string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Join(dir, "package", "bin"), 0o700); err != nil {
				t.Fatal(err)
			}
			original := "{\n  vendorHash = \"" + oldHash + "\";\n}\n"
			write("flake.nix", original, 0o600)
			write("output", "", 0o600)
			write("package/bin/pvetui", "#!"+bash+"\n[[ $1 == --version ]] || exit 7\n[[ $SMOKE_FAIL == false ]] || exit 8\necho 'pvetui version test'\n", 0o700)
			write("nix", "#!"+bash+"\n"+`set -euo pipefail
[[ "$*" == 'build .#default --no-link --print-out-paths' ]] || exit 9
count=0
if [[ -f "$TEST_DIR/calls" ]]; then count=$(<"$TEST_DIR/calls"); fi
count=$((count + 1))
printf '%s' "$count" > "$TEST_DIR/calls"
if [[ "$SCENARIO" == build-error || ("$SCENARIO" == verify-error && "$count" == 2) ]]; then
  echo 'go: incompatible toolchain' >&2
  exit 42
fi
if [[ "$SCENARIO" != success && "$count" == 1 ]]; then
  drv=pvetui-test-go-modules
  if [[ "$SCENARIO" == foreign ]]; then drv=other-source; fi
  echo "error: hash mismatch in fixed-output derivation '/nix/store/test-$drv.drv':" >&2
  if [[ "$SCENARIO" != missing ]]; then
    hash=$NEW_HASH
    if [[ "$SCENARIO" == same ]]; then hash=$OLD_HASH; fi
    printf '         specified: %s\n              got:    %s\n' "$OLD_HASH" "$hash" >&2
  fi
  exit 1
fi
printf '%s/package\n' "$TEST_DIR"
`, 0o700)
			args := []string{script}
			if tc.checkOnly {
				args = append(args, "--check")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// #nosec G204 -- runs the repository helper with test-controlled arguments and a mock Nix executable.
			cmd := exec.CommandContext(ctx, bash, args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(),
				"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"TEST_DIR="+dir, "SCENARIO="+tc.scenario,
				"OLD_HASH="+oldHash, "NEW_HASH="+newHash,
				"SMOKE_FAIL="+strconv.FormatBool(tc.failSmoke),
				"GITHUB_OUTPUT="+filepath.Join(dir, "output"))
			output, runErr := cmd.CombinedOutput()
			if (runErr != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %t\n%s", runErr, tc.wantError, output)
			}
			read := func(name string) string {
				t.Helper()
				// #nosec G304 -- fixture names below are literals within this test's private temporary directory.
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			if calls := read("calls"); calls != strconv.Itoa(tc.wantCalls) {
				t.Errorf("build calls = %s, want %d", calls, tc.wantCalls)
			}
			wantFlake := original
			if tc.wantUpdate {
				wantFlake = strings.ReplaceAll(original, oldHash, newHash)
			}
			if got := read("flake.nix"); got != wantFlake {
				t.Errorf("flake = %q, want %q", got, wantFlake)
			}
			wantOutput := ""
			if !tc.wantError {
				wantOutput = "updated=" + strconv.FormatBool(tc.wantUpdate) + "\n"
			}
			if got := read("output"); got != wantOutput {
				t.Errorf("workflow output = %q, want %q", got, wantOutput)
			}
		})
	}
}
