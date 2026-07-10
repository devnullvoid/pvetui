package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/devnullvoid/pvetui/internal/adapters"
	"github.com/devnullvoid/pvetui/internal/config"
	"github.com/devnullvoid/pvetui/pkg/api"
	"github.com/devnullvoid/pvetui/pkg/api/interfaces"
)

// captureStdout replaces os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	old := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)

	return buf.String()
}

// captureStderr replaces os.Stderr for the duration of fn and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	old := os.Stderr
	os.Stderr = w

	fn()

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)

	return buf.String()
}

func TestPrintJSON(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	got := captureStdout(t, func() {
		if err := printJSON(payload{Name: "test", Value: 42}); err != nil {
			t.Fatalf("printJSON returned error: %v", err)
		}
	})

	var decoded payload
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, got)
	}

	if decoded.Name != "test" || decoded.Value != 42 {
		t.Errorf("unexpected decoded value: %+v", decoded)
	}
}

func TestPrintError(t *testing.T) {
	sentinel := errors.New("something went wrong")

	got := captureStderr(t, func() {
		_ = printError(sentinel)
	})

	var obj map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &obj); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\noutput: %s", err, got)
	}

	if obj["error"] != sentinel.Error() {
		t.Errorf("expected error %q, got %q", sentinel.Error(), obj["error"])
	}
}

func TestPrintTable(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"pve01", "online"},
		{"pve02", "offline"},
	}

	got := captureStdout(t, func() {
		printTable(headers, rows)
	})

	for _, want := range []string{"NAME", "STATUS", "pve01", "online", "pve02", "offline"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestApplyConfiguredOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Config
		setFlag    string
		want       string
		wantChange bool
	}{
		{
			name: "uses config default when flag omitted",
			cfg: &config.Config{
				CLI: config.CLIConfig{DefaultOutput: outputTable},
			},
			want: outputTable,
		},
		{
			name: "keeps explicit flag over config default",
			cfg: &config.Config{
				CLI: config.CLIConfig{DefaultOutput: outputTable},
			},
			setFlag:    outputJSON,
			want:       outputJSON,
			wantChange: true,
		},
		{
			name: "keeps built-in default when config empty",
			cfg:  &config.Config{},
			want: outputJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().StringP("output", "o", outputJSON, "Output format")
			if tt.setFlag != "" {
				if err := cmd.Flags().Set("output", tt.setFlag); err != nil {
					t.Fatalf("set output flag: %v", err)
				}
			}

			applyConfiguredOutputFormat(cmd, tt.cfg)

			if got := getOutputFormat(cmd); got != tt.want {
				t.Fatalf("getOutputFormat() = %q, want %q", got, tt.want)
			}
			if got := cmd.Flag("output").Changed; got != tt.wantChange {
				t.Fatalf("flag Changed = %v, want %v", got, tt.wantChange)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}

	for _, tc := range cases {
		got := formatBytes(tc.input)
		if got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		input int64
		want  string
	}{
		{0, "0s"},
		{45, "45s"},
		{90, "1m30s"},
		{3661, "1h1m"},
		{90061, "1d1h1m"},
	}

	for _, tc := range cases {
		got := formatUptime(tc.input)
		if got != tc.want {
			t.Errorf("formatUptime(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// newTestClient creates an api.Client pointing at the given httptest server URL.
// The server must respond to POST /api2/json/access/ticket with a valid auth response.
func newTestClient(t *testing.T, serverURL string) *api.Client {
	t.Helper()

	cfg := &config.Config{
		Addr:     serverURL,
		User:     "user",
		Password: "pass",
		Realm:    "pam",
		Insecure: true,
	}

	client, err := api.NewClient(
		adapters.NewConfigAdapter(cfg),
		api.WithCache(&interfaces.NoOpCache{}),
	)
	if err != nil {
		t.Fatalf("api.NewClient: %v", err)
	}

	return client
}

// authResponse is the minimal ticket response the API client needs.
const authResponse = `{"data":{"ticket":"t","CSRFPreventionToken":"c","username":"user@pam"}}`

// taskStatusResponse returns a /nodes/{node}/tasks/{upid}/status JSON body.
func taskStatusResponse(exitStatus string) string {
	return `{"data":{"status":"stopped","exitstatus":"` + exitStatus + `","upid":"UPID:pve:test","node":"pve","starttime":1700000000,"endtime":1700000060}}`
}

func TestWaitForTaskSuccess(t *testing.T) {
	const upid = "UPID:pve:00001234:00000000:test:qmcreate:100:user@pam:"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api2/json/access/ticket":
			_, _ = w.Write([]byte(authResponse))
		case r.URL.Path == "/api2/json/cluster/tasks":
			// Return the task as already complete with OK status.
			body := `{"data":[{"upid":"` + upid + `","status":"OK","endtime":1700000060,"starttime":1700000000,"node":"pve","type":"qmcreate"}]}`
			_, _ = w.Write([]byte(body))
		case strings.Contains(r.URL.Path, "/tasks/") && strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(taskStatusResponse("OK")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)

	exitStatus, err := waitForTask(context.Background(), client, "pve", upid, "test-op")
	if err != nil {
		t.Fatalf("waitForTask returned unexpected error: %v", err)
	}

	if exitStatus != "OK" {
		t.Errorf("exitStatus = %q, want %q", exitStatus, "OK")
	}
}

func TestWaitForTaskFailure(t *testing.T) {
	const upid = "UPID:pve:00001234:00000000:test:qmcreate:100:user@pam:"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api2/json/access/ticket":
			_, _ = w.Write([]byte(authResponse))
		case r.URL.Path == "/api2/json/cluster/tasks":
			// Task finished with an error status.
			body := `{"data":[{"upid":"` + upid + `","status":"disk full","endtime":1700000060,"starttime":1700000000,"node":"pve","type":"qmcreate"}]}`
			_, _ = w.Write([]byte(body))
		case strings.Contains(r.URL.Path, "/tasks/") && strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(taskStatusResponse("disk full")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)

	exitStatus, err := waitForTask(context.Background(), client, "pve", upid, "test-op")
	if err == nil {
		t.Fatal("waitForTask returned nil error, expected non-nil for failed task")
	}

	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error message %q does not mention failure reason", err.Error())
	}

	if exitStatus != "ERROR" {
		t.Errorf("exitStatus = %q, want %q", exitStatus, "ERROR")
	}
}

func TestWaitForTaskCancelled(t *testing.T) {
	const upid = "UPID:pve:00001234:00000000:test:qmcreate:100:user@pam:"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api2/json/access/ticket" {
			_, _ = w.Write([]byte(authResponse))
			return
		}

		// Never complete the task — return an empty task list so
		// WaitForTaskCompletion keeps polling.
		if r.URL.Path == "/api2/json/cluster/tasks" {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before waitForTask is called

	_, err := waitForTask(ctx, client, "pve", upid, "test-op")
	if err == nil {
		t.Fatal("waitForTask returned nil error, expected cancellation error")
	}

	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error %q does not mention cancellation", err.Error())
	}
}

func TestInferGuestTypeFromVolID(t *testing.T) {
	cases := []struct {
		volid   string
		want    string
		wantErr bool
	}{
		{"local:backup/vzdump-qemu-100-2024_01_01-00_00_00.tar.zst", "qemu", false},
		{"local:backup/vzdump-lxc-101-2024_01_01-00_00_00.tar.zst", "lxc", false},
		{"VZDUMP-QEMU-100.tar.zst", "qemu", false}, // case-insensitive
		{"local:iso/debian-12.iso", "", true},
		{"", "", true},
	}

	for _, tc := range cases {
		got, err := inferGuestTypeFromVolID(tc.volid)
		if tc.wantErr {
			if err == nil {
				t.Errorf("inferGuestTypeFromVolID(%q) returned no error, want error", tc.volid)
			}
		} else {
			if err != nil {
				t.Errorf("inferGuestTypeFromVolID(%q) returned error: %v", tc.volid, err)
			}

			if got != tc.want {
				t.Errorf("inferGuestTypeFromVolID(%q) = %q, want %q", tc.volid, got, tc.want)
			}
		}
	}
}

func TestInferContentTypeFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://example.com/debian-12.iso", "iso"},
		{"https://example.com/debian-12.iso?foo=bar", "iso"},
		{"https://example.com/debian-12-standard_12.7-1_amd64.tar.zst", "vztmpl"},
		{"https://example.com/disk.img", "import"},
		{"https://example.com/ubuntu.qcow2", ""},
		{"https://example.com/file.TAR.GZ", "vztmpl"}, // case-insensitive
	}

	for _, tc := range cases {
		got := inferContentTypeFromURL(tc.url)
		if got != tc.want {
			t.Errorf("inferContentTypeFromURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
