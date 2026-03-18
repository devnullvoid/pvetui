package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
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
