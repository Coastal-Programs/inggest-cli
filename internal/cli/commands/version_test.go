package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
)

// captureStdout redirects os.Stdout to a pipe, runs fn, then returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("reading pipe: %v", err)
	}
	return buf.String()
}

func TestVersionCmdOutput(t *testing.T) {
	state.AppVersion = "v1.2.3"
	state.Output = "json"

	cmd := NewVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if v, ok := result["version"]; !ok || v != "v1.2.3" {
		t.Errorf("expected version %q, got %q", "v1.2.3", v)
	}
	if v, ok := result["os"]; !ok || v != runtime.GOOS {
		t.Errorf("expected os %q, got %q", runtime.GOOS, v)
	}
	if v, ok := result["arch"]; !ok || v != runtime.GOARCH {
		t.Errorf("expected arch %q, got %q", runtime.GOARCH, v)
	}
}

func TestVersionCmdTextOutput(t *testing.T) {
	state.AppVersion = "v1.2.3"
	state.Output = "text"

	cmd := NewVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "v1.2.3") {
		t.Errorf("expected output to contain %q, got: %s", "v1.2.3", got)
	}
}
