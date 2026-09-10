package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

func TestHealthCmdExists(t *testing.T) {
	cmd := NewHealthCmd()

	if cmd.Use != "health" {
		t.Errorf("expected Use %q, got %q", "health", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestMetricsCmdExists(t *testing.T) {
	cmd := NewMetricsCmd()

	if cmd.Use != "metrics" {
		t.Errorf("expected Use %q, got %q", "metrics", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestMetricsCmdFlags(t *testing.T) {
	cmd := NewMetricsCmd()

	sinceFlag := cmd.Flags().Lookup("since")
	if sinceFlag == nil {
		t.Fatal("expected --since flag to exist")
	}
	if sinceFlag.DefValue != "24h" {
		t.Errorf("expected --since default %q, got %q", "24h", sinceFlag.DefValue)
	}

	fnFlag := cmd.Flags().Lookup("function")
	if fnFlag == nil {
		t.Fatal("expected --function flag to exist")
	}
	if fnFlag.DefValue != "" {
		t.Errorf("expected --function default %q, got %q", "", fnFlag.DefValue)
	}
}

func TestBacklogCmdExists(t *testing.T) {
	cmd := NewBacklogCmd()

	if cmd.Use != "backlog" {
		t.Errorf("expected Use %q, got %q", "backlog", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestMaxPagesConstant(t *testing.T) {
	if maxPages != 50 {
		t.Errorf("expected maxPages == 50, got %d", maxPages)
	}
}

func TestHealth_AllPassed(t *testing.T) {
	srv := newMockServer(t,
		nil, map[string]http.HandlerFunc{
			"/v2/envs":     jsonOK(`{"data":[]}`),
			"/api/v2/envs": jsonOK(`{"data":[]}`),
			"/dev": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"version":"test"}`))
			},
		},
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123", EventKey: "evt-key-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = true
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewHealthCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	healthy, ok := result["healthy"]
	if !ok {
		t.Fatal("expected 'healthy' key in output")
	}
	if healthy != true {
		t.Errorf("expected healthy=true, got %v", healthy)
	}
}

func TestHealth_NoSigningKey(t *testing.T) {
	srv := newMockServer(t,
		nil, map[string]http.HandlerFunc{
			"/v2/envs":     jsonStatus(http.StatusUnauthorized, v2Unauthorized),
			"/api/v2/envs": jsonStatus(http.StatusUnauthorized, v2Unauthorized),
		},
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewHealthCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected health check to return an error")
		}
	})

	if !strings.Contains(got, `"api_key"`) {
		t.Errorf("expected output to contain api_key check, got: %s", got)
	}
	if !strings.Contains(got, `"fail"`) {
		t.Errorf("expected output to contain status 'fail', got: %s", got)
	}
}

func TestMetrics_Success(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"COMPLETED","startedAt":"2099-01-01T00:00:00Z","endedAt":"2099-01-01T00:00:01Z","function":{"id":"fn1","name":"fn1","slug":"fn1"},"trigger":{"eventName":"evt"}},{"id":"r2","status":"COMPLETED","startedAt":"2099-01-01T00:00:00Z","endedAt":"2099-01-01T00:00:02Z","function":{"id":"fn1","name":"fn1","slug":"fn1"},"trigger":{"eventName":"evt"}},{"id":"r3","status":"FAILED","startedAt":"2099-01-01T00:00:00Z","endedAt":"2099-01-01T00:00:01Z","function":{"id":"fn2","name":"fn2","slug":"fn2"},"trigger":{"eventName":"evt"}},{"id":"r4","status":"RUNNING","function":{"id":"fn1","name":"fn1","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "24h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	// JSON numbers are float64
	if total, ok := result["total"].(float64); !ok || total != 4 {
		t.Errorf("expected total=4, got %v", result["total"])
	}
	if completed, ok := result["completed"].(float64); !ok || completed != 2 {
		t.Errorf("expected completed=2, got %v", result["completed"])
	}
	if failed, ok := result["failed"].(float64); !ok || failed != 1 {
		t.Errorf("expected failed=1, got %v", result["failed"])
	}
	if running, ok := result["running"].(float64); !ok || running != 1 {
		t.Errorf("expected running=1, got %v", result["running"])
	}
}

func TestBacklog_WithRuns(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn1","name":"Process Payment","slug":"fn1"},"trigger":{"eventName":"evt"}},{"id":"r2","status":"QUEUED","function":{"id":"fn1","name":"Process Payment","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if _, ok := result["entries"]; !ok {
		t.Errorf("expected 'entries' key in output, got keys: %v", result)
	}
}

func TestBacklog_Empty(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	got = strings.TrimSpace(got)
	if got != "[]" {
		t.Errorf("expected empty array '[]', got: %s", got)
	}
}

func TestMetrics_TextOutput(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"COMPLETED","startedAt":"2099-01-01T00:00:00Z","endedAt":"2099-01-01T00:00:01Z","function":{"id":"fn1","name":"fn1","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputText
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "24h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "Total runs") {
		t.Errorf("expected text output to contain 'Total runs', got: %s", got)
	}
	if !strings.Contains(got, "Success rate") {
		t.Errorf("expected text output to contain 'Success rate', got: %s", got)
	}
}

func TestMetrics_WithFunctionFilter(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"COMPLETED","startedAt":"2099-01-01T00:00:00Z","endedAt":"2099-01-01T00:00:01Z","function":{"id":"fn-123","name":"fn1","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "1h", "--function", "fn-123"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if total, ok := result["total"].(float64); !ok || total != 1 {
		t.Errorf("expected total=1, got %v", result["total"])
	}
}

func TestMetrics_InvalidSince(t *testing.T) {
	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "notaduration"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --since")
	}
	if !strings.Contains(err.Error(), "invalid --since") {
		t.Errorf("expected error about invalid --since, got: %v", err)
	}
}

func TestHealth_TextOutput(t *testing.T) {
	srv := newMockServer(t,
		nil, map[string]http.HandlerFunc{
			"/v2/envs":     jsonOK(`{"data":[]}`),
			"/api/v2/envs": jsonOK(`{"data":[]}`),
			"/dev": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"version":"test"}`))
			},
		},
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123", EventKey: "evt-key-123"}
	state.Output = testOutputText
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = true
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewHealthCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "api_key") {
		t.Errorf("expected text output to contain 'api_key', got: %s", got)
	}
	if !strings.Contains(got, "All checks passed") {
		t.Errorf("expected text output to contain 'All checks passed', got: %s", got)
	}
}

func TestBacklog_TextOutput(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputText
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "No queued or running") {
		t.Errorf("expected text output to contain 'No queued or running', got: %s", got)
	}
}

func TestBacklog_TextWithEntries(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn1","name":"Process Payment","slug":"fn1"},"trigger":{"eventName":"evt"}},{"id":"r2","status":"QUEUED","function":{"id":"fn1","name":"Process Payment","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputText
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "Process Payment") {
		t.Errorf("expected text output to contain 'Process Payment', got: %s", got)
	}
}

func TestBacklog_UnknownFunction(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Runs with no Function should be grouped under "(unknown)".
	if !strings.Contains(got, "(unknown)") {
		t.Errorf("expected output to contain '(unknown)' for runs with nil function, got: %s", got)
	}
}

func TestMetrics_Truncated(t *testing.T) {
	// With the events-based API, ListRuns always returns HasNextPage=false,
	// so truncation is no longer triggered. This test verifies a single page is returned.
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"COMPLETED","startedAt":"2099-01-01T00:00:00Z","endedAt":"2099-01-01T00:00:01Z","function":{"id":"fn1","name":"fn1","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "24h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	// With events-based API, truncation no longer happens since ListRuns returns all runs in one page.
	if total, ok := result["total"].(float64); !ok || total != 1 {
		t.Errorf("expected total=1, got %v", result["total"])
	}
}

func TestMetrics_TextTruncated(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"COMPLETED","startedAt":"2099-01-01T00:00:00Z","endedAt":"2099-01-01T00:00:01Z","function":{"id":"fn1","name":"fn1","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputText
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "24h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// With events-based API, truncation no longer happens. Just verify text output has metrics content.
	if !strings.Contains(got, "Total runs") {
		t.Errorf("expected text output to contain 'Total runs', got: %s", got)
	}
	if !strings.Contains(got, "Duration") {
		t.Errorf("expected text output to contain 'Duration', got: %s", got)
	}
}

func TestBacklog_TableOutput(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn1","name":"Process Payment","slug":"fn1"},"trigger":{"eventName":"evt"}},{"id":"r2","status":"QUEUED","function":{"id":"fn1","name":"Process Payment","slug":"fn1"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputTable
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "Process Payment") {
		t.Errorf("expected table output to contain 'Process Payment', got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// Additional tests for uncovered branches
// ---------------------------------------------------------------------------

// TestHealth_DevServerFail covers metrics.go:97-104 — DevMode=true but dev
// server is not reachable, producing a "fail" result for the dev_server check.
func TestHealth_DevServerFail(t *testing.T) {
	// API mock returns valid functions so the API check passes.
	srv := newMockServer(t,
		nil, map[string]http.HandlerFunc{
			"/v2/envs":     jsonOK(`{"data":[]}`),
			"/api/v2/envs": jsonOK(`{"data":[]}`),
		},
	)
	defer srv.Close()

	// Create and immediately close a second server to get an unreachable URL.
	closedSrv := newMockServer(t, nil, nil)
	closedDevURL := closedSrv.URL
	closedSrv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123", EventKey: "evt-key-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = closedDevURL // unreachable
	state.DevMode = true           // forces dev_server check
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewHealthCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		// Expect error because dev_server is unreachable → allPassed=false
		cmd.Execute()
	})

	if !strings.Contains(got, `"fail"`) {
		t.Errorf("expected JSON output to contain status 'fail', got: %s", got)
	}
	if !strings.Contains(got, `"dev_server"`) {
		t.Errorf("expected JSON output to contain 'dev_server' check, got: %s", got)
	}
	if !strings.Contains(got, "not reachable") {
		t.Errorf("expected dev_server detail to mention 'not reachable', got: %s", got)
	}
}

// TestHealth_TextWithFailAndWarn covers metrics.go:117-128 — text output icon
// branches for "fail", "warn", and "Some checks failed" message.
func TestHealth_TextWithFailAndWarn(t *testing.T) {
	// Use a closed server for the API so the API check also fails.
	closedSrv := newMockServer(t, nil, nil)
	closedURL := closedSrv.URL
	closedSrv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	// No signing key → fail, no event key → warn, API unreachable → fail,
	// DevMode=true but dev server unreachable → fail
	state.Config = &config.Config{}
	state.Output = testOutputText
	state.APIBaseURL = closedURL
	state.DevServer = closedURL
	state.DevMode = true
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewHealthCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		cmd.Execute() // will return error, we check stdout
	})

	// "fail" icon ✗ for signing_key, api, and dev_server
	if !strings.Contains(got, "✗") {
		t.Errorf("expected text output to contain fail icon '✗', got: %s", got)
	}
	// "warn" icon ! for event_key
	if !strings.Contains(got, "!") {
		t.Errorf("expected text output to contain warn icon '!', got: %s", got)
	}
	// "Some checks failed" message (allPassed=false)
	if !strings.Contains(got, "Some checks failed") {
		t.Errorf("expected text output to contain 'Some checks failed', got: %s", got)
	}
}

// TestHealth_TextWithSkip covers metrics.go:121-122 — text output "skip" icon
// when DevMode=false and dev server is not auto-detected.
func TestHealth_TextWithSkip(t *testing.T) {
	closedSrv := newMockServer(t, nil, nil)
	closedURL := closedSrv.URL
	closedSrv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	// signing_key + event_key configured so they pass; API will fail but that's ok.
	state.Config = &config.Config{SigningKey: "signkey-test-123", EventKey: "evt-123"}
	state.Output = testOutputText
	state.APIBaseURL = closedURL
	state.DevServer = closedURL
	state.DevMode = false // not dev mode and server unreachable → "skip"
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewHealthCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		cmd.Execute()
	})

	// "skip" icon "-" for dev_server
	if !strings.Contains(got, "-  dev_server") {
		t.Errorf("expected text output to contain skip icon '-' for dev_server, got: %s", got)
	}
}

// TestMetrics_ListRunsError covers metrics.go:187-189 — ListRuns returning an
// error in the metrics command.
func TestMetrics_ListRunsError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonStatus(http.StatusUnauthorized, v2Unauthorized),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "1h"})
	cmd.SilenceUsage = true
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListRuns fails")
	}
	if !strings.Contains(err.Error(), "querying runs") {
		t.Errorf("expected error about querying runs, got: %v", err)
	}
}

// TestBacklog_ListRunsError covers metrics.go:330-332 — ListRuns returning an
// error in the backlog command.
func TestBacklog_ListRunsError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonStatus(http.StatusUnauthorized, v2Unauthorized),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	cmd.SilenceUsage = true
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListRuns fails")
	}
	if !strings.Contains(err.Error(), "querying") {
		t.Errorf("expected error about querying runs, got: %v", err)
	}
}

// TestBacklog_TruncatedJSON covers metrics.go:341-347 (QUEUED pagination limit),
// metrics.go:385-387 (sort comparison with 2+ entries), and metrics.go:407-410
// (JSON truncation metadata).
func TestBacklog_TruncatedJSON(t *testing.T) {
	// hasNextPage:true forces pagination loop to hit maxPages limit.
	// Two different functions so sort.Slice comparison executes.
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn-a","name":"Func A","slug":"fn-a"},"trigger":{"eventName":"evt"}},{"id":"r2","status":"QUEUED","function":{"id":"fn-b","name":"Func B","slug":"fn-b"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	// With events-based API, truncation no longer happens. Verify entries are present.
	if _, ok := result["entries"]; !ok {
		t.Error("expected 'entries' key in output")
	}
	// Both function names should appear.
	if !strings.Contains(got, "Func A") {
		t.Errorf("expected output to contain 'Func A', got: %s", got)
	}
	if !strings.Contains(got, "Func B") {
		t.Errorf("expected output to contain 'Func B', got: %s", got)
	}
}

// TestBacklog_TruncatedText covers metrics.go:397-400 — truncated text output
// for the backlog command.
func TestBacklog_TruncatedText(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn-a","name":"Func A","slug":"fn-a"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputText
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// With events-based API, truncation no longer happens. Just verify function name is present.
	if !strings.Contains(got, "Func A") {
		t.Errorf("expected text output to contain 'Func A', got: %s", got)
	}
}

// TestMetrics_NoDurations covers metrics.go:236-238 — the percentile function
// returns 0 when there are no duration samples (runs without startedAt/endedAt).
func TestMetrics_NoDurations(t *testing.T) {
	// Runs without startedAt/endedAt → no duration samples.
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn1","name":"fn1","slug":"fn1"}},{"id":"r2","status":"QUEUED","function":{"id":"fn1","name":"fn1","slug":"fn1"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "1h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	// With no durations, the result should not have p50/p90/p99 keys.
	if _, ok := result["durationSamples"]; ok {
		t.Errorf("expected no 'durationSamples' key when no timing data, got: %v", result["durationSamples"])
	}
	if total, ok := result["total"].(float64); !ok || total != 2 {
		t.Errorf("expected total=2, got %v", result["total"])
	}
}

// TestBacklog_TableWithEntries covers metrics.go:414-416 — table format output
// for backlog when there are entries (non-empty, non-JSON, non-text path).
// The existing TestBacklog_TableOutput test may hit FormatTable but the sort
// comparison at line 385-387 was uncovered due to only 1 unique function.
// This test uses 2 functions so the comparator runs.
func TestBacklog_TableWithEntries(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn-a","name":"Func A","slug":"fn-a"},"trigger":{"eventName":"evt"}},{"id":"r2","status":"RUNNING","function":{"id":"fn-b","name":"Func B","slug":"fn-b"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputTable
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = testAppVersion
	state.Env = ""

	cmd := NewBacklogCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "Func A") {
		t.Errorf("expected table output to contain 'Func A', got: %s", got)
	}
	if !strings.Contains(got, "Func B") {
		t.Errorf("expected table output to contain 'Func B', got: %s", got)
	}
}

// TestComputeMetrics_Truncated verifies the truncation flags are recorded when
// the caller reports that results were cut short.
func TestComputeMetrics_Truncated(t *testing.T) {
	started := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ended := started.Add(2 * time.Second)
	runs := []inngest.FunctionRun{
		{ID: "r1", Status: "COMPLETED", StartedAt: &started, EndedAt: &ended},
		{ID: "r2", Status: "FAILED"},
	}

	result := computeMetrics(runs, "24h", true)

	if v, ok := result["truncated"].(bool); !ok || !v {
		t.Errorf("expected truncated=true, got %v", result["truncated"])
	}
	if v, ok := result["truncatedAt"].(int); !ok || v != 2 {
		t.Errorf("expected truncatedAt=2, got %v", result["truncatedAt"])
	}
}

// TestComputeMetrics_NotTruncated verifies the flags are absent otherwise.
func TestComputeMetrics_NotTruncated(t *testing.T) {
	result := computeMetrics([]inngest.FunctionRun{{ID: "r1", Status: "COMPLETED"}}, "24h", false)

	if _, ok := result["truncated"]; ok {
		t.Error("expected no truncated key when not truncated")
	}
	if _, ok := result["truncatedAt"]; ok {
		t.Error("expected no truncatedAt key when not truncated")
	}
}

// TestPrintMetricsText_TruncatedNote covers the truncation note branch.
func TestPrintMetricsText_TruncatedNote(t *testing.T) {
	started := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ended := started.Add(time.Second)
	runs := []inngest.FunctionRun{
		{ID: "r1", Status: "COMPLETED", StartedAt: &started, EndedAt: &ended},
	}

	out := captureStdout(t, func() {
		printMetricsText(computeMetrics(runs, "24h", true))
	})

	if !strings.Contains(out, "results truncated at 1 runs") {
		t.Errorf("expected truncation note in output, got:\n%s", out)
	}
	// Duration percentiles should also render for the sampled run.
	if !strings.Contains(out, "P50:") {
		t.Errorf("expected duration percentiles in output, got:\n%s", out)
	}
}

// TestPrintMetricsText_NoTruncationNote verifies the note is omitted otherwise.
func TestPrintMetricsText_NoTruncationNote(t *testing.T) {
	out := captureStdout(t, func() {
		printMetricsText(computeMetrics([]inngest.FunctionRun{{ID: "r1", Status: "COMPLETED"}}, "1h", false))
	})

	if strings.Contains(out, "truncated") {
		t.Errorf("expected no truncation note, got:\n%s", out)
	}
}

// TestMetrics_FollowsCursorAcrossPages proves paginateRuns really follows
// page.cursor: two pages of runs must both be counted and the second request
// must carry the cursor from the first response.
func TestMetrics_FollowsCursorAcrossPages(t *testing.T) {
	var cursors []string
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			cursors = append(cursors, r.URL.Query().Get("cursor"))
			if r.URL.Query().Get("limit") != "100" {
				t.Errorf("expected limit=100 for aggregation, got %q", r.URL.RawQuery)
			}
			if len(cursors) == 1 {
				jsonOK(`{"data":[{"id":"r1","status":"COMPLETED","durationMs":"10"}],"page":{"cursor":"page-2","hasMore":true}}`)(w, r)
				return
			}
			jsonOK(`{"data":[{"id":"r2","status":"FAILED","durationMs":"20"}],"page":{"hasMore":false}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "1h"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "page-2" {
		t.Fatalf("cursors sent = %q, want [\"\" \"page-2\"]", cursors)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("parse: %v\n%s", err, got)
	}
	if result["total"] != float64(2) || result["completed"] != float64(1) || result["failed"] != float64(1) {
		t.Errorf("expected both pages aggregated, got %v", result)
	}
	if _, ok := result["truncated"]; ok {
		t.Error("two pages must not be reported as truncated")
	}
}

// TestMetrics_TruncatesAtMaxPages proves the safety cap: a server that always
// reports more pages stops after maxPages requests, warns on stderr, and the
// JSON output flags the result as truncated.
func TestMetrics_TruncatesAtMaxPages(t *testing.T) {
	requests := 0
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			requests++
			jsonOK(`{"data":[{"id":"r","status":"COMPLETED"}],"page":{"cursor":"again","hasMore":true}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewMetricsCmd()
	cmd.SetArgs([]string{"--since", "1h"})
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if requests != maxPages {
		t.Errorf("requests = %d, want exactly maxPages (%d)", requests, maxPages)
	}
	if !strings.Contains(stderr.String(), "pagination limit reached") {
		t.Errorf("expected truncation warning on stderr, got %q", stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("parse: %v\n%s", err, got)
	}
	if result["truncated"] != true || result["truncatedAt"] != float64(maxPages) {
		t.Errorf("expected truncated=true truncatedAt=%d, got %v", maxPages, result)
	}
}

// TestBacklog_PaginationTruncation covers the truncation branches of the
// backlog command. The server always reports another page, so paginateRuns
// stops at its maxPages cap and flags the result as truncated.
func TestBacklog_PaginationTruncation(t *testing.T) {
	alwaysMore := jsonOK(`{"data":[{"id":"r1","status":"RUNNING","function":{"id":"fn-a","name":"Func A","slug":"fn-a"},"trigger":{"eventName":"evt"}}],"page":{"hasMore":true,"cursor":"next"}}`)

	t.Run("text output warns", func(t *testing.T) {
		srv := newMockServer(t, nil, map[string]http.HandlerFunc{"/v2/runs": alwaysMore})
		defer srv.Close()
		setupCloudState(t, srv.URL)
		state.Output = testOutputText

		var buf bytes.Buffer
		cmd := NewBacklogCmd()
		cmd.SetArgs([]string{})
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		got := captureStdout(t, func() {
			if err := cmd.Execute(); err != nil {
				t.Fatalf("backlog returned error: %v", err)
			}
		})

		if !strings.Contains(got, "results truncated at") {
			t.Errorf("expected a truncation note in text output, got:\n%s", got)
		}
		if !strings.Contains(buf.String(), "pagination limit reached") {
			t.Errorf("expected a pagination warning on stderr, got:\n%s", buf.String())
		}
	})

	t.Run("json output carries the flags", func(t *testing.T) {
		srv := newMockServer(t, nil, map[string]http.HandlerFunc{"/v2/runs": alwaysMore})
		defer srv.Close()
		setupCloudState(t, srv.URL)
		state.Output = testOutputJSON

		cmd := NewBacklogCmd()
		cmd.SetArgs([]string{})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		got := captureStdout(t, func() {
			if err := cmd.Execute(); err != nil {
				t.Fatalf("backlog returned error: %v", err)
			}
		})

		var result map[string]any
		if err := json.Unmarshal([]byte(got), &result); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, got)
		}
		if result["truncated"] != true {
			t.Errorf("truncated = %v, want true", result["truncated"])
		}
		if _, ok := result["truncatedAt"]; !ok {
			t.Error("expected a truncatedAt field in JSON output")
		}
	})
}
