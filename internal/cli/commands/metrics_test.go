package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
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
		map[string]string{
			"ListFunctions": `{"data":{"functions":[{"id":"fn-1","name":"test"}]}}`,
		},
		map[string]http.HandlerFunc{
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
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = true
	state.AppVersion = "test"
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

	var result map[string]interface{}
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
		map[string]string{
			"ListFunctions": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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

	if !strings.Contains(got, `"signing_key"`) {
		t.Errorf("expected output to contain signing_key check, got: %s", got)
	}
	if !strings.Contains(got, `"fail"`) {
		t.Errorf("expected output to contain status 'fail', got: %s", got)
	}
}

func TestMetrics_Success(t *testing.T) {
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"COMPLETED","startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:01Z","function":{"name":"fn1","slug":"fn1"}},"cursor":"c1"},{"node":{"id":"r2","status":"COMPLETED","startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:02Z","function":{"name":"fn1","slug":"fn1"}},"cursor":"c2"},{"node":{"id":"r3","status":"FAILED","startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:01Z","function":{"name":"fn2","slug":"fn2"}},"cursor":"c3"},{"node":{"id":"r4","status":"RUNNING","function":{"name":"fn1","slug":"fn1"}},"cursor":"c4"}],"pageInfo":{"hasNextPage":false},"totalCount":4}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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

	var result map[string]interface{}
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
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"RUNNING","function":{"name":"Process Payment","slug":"fn1"}},"cursor":"c1"},{"node":{"id":"r2","status":"QUEUED","function":{"name":"Process Payment","slug":"fn1"}},"cursor":"c2"}],"pageInfo":{"hasNextPage":false},"totalCount":2}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if _, ok := result["entries"]; !ok {
		t.Errorf("expected 'entries' key in output, got keys: %v", result)
	}
}

func TestBacklog_Empty(t *testing.T) {
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false},"totalCount":0}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"COMPLETED","startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:01Z","function":{"name":"fn1","slug":"fn1"}},"cursor":"c1"}],"pageInfo":{"hasNextPage":false},"totalCount":1}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "text"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"COMPLETED","startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:01Z","function":{"name":"fn1","slug":"fn1"}},"cursor":"c1"}],"pageInfo":{"hasNextPage":false},"totalCount":1}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if total, ok := result["total"].(float64); !ok || total != 1 {
		t.Errorf("expected total=1, got %v", result["total"])
	}
}

func TestMetrics_InvalidSince(t *testing.T) {
	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"

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
		map[string]string{
			"ListFunctions": `{"data":{"functions":[{"id":"fn-1","name":"test"}]}}`,
		},
		map[string]http.HandlerFunc{
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
	state.Output = "text"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = true
	state.AppVersion = "test"
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

	if !strings.Contains(got, "signing_key") {
		t.Errorf("expected text output to contain 'signing_key', got: %s", got)
	}
	if !strings.Contains(got, "All checks passed") {
		t.Errorf("expected text output to contain 'All checks passed', got: %s", got)
	}
}

func TestBacklog_TextOutput(t *testing.T) {
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false},"totalCount":0}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "text"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"RUNNING","function":{"name":"Process Payment","slug":"fn1"}},"cursor":"c1"},{"node":{"id":"r2","status":"QUEUED","function":{"name":"Process Payment","slug":"fn1"}},"cursor":"c2"}],"pageInfo":{"hasNextPage":false},"totalCount":2}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "text"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"RUNNING"},"cursor":"c1"}],"pageInfo":{"hasNextPage":false},"totalCount":1}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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
	// Simulate pagination that returns hasNextPage=true for maxPages pages.
	// The mock will always say hasNextPage=true so the metrics loop hits the truncation limit.
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"COMPLETED","startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:01Z","function":{"name":"fn1","slug":"fn1"}},"cursor":"c1"}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"},"totalCount":100}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if truncated, ok := result["truncated"].(bool); !ok || !truncated {
		t.Errorf("expected truncated=true, got %v", result["truncated"])
	}
}

func TestMetrics_TextTruncated(t *testing.T) {
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"COMPLETED","startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:01Z","function":{"name":"fn1","slug":"fn1"}},"cursor":"c1"}],"pageInfo":{"hasNextPage":true,"endCursor":"c1"},"totalCount":100}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "text"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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

	if !strings.Contains(got, "truncated") {
		t.Errorf("expected text output to contain 'truncated', got: %s", got)
	}
	if !strings.Contains(got, "Duration") {
		t.Errorf("expected text output to contain 'Duration', got: %s", got)
	}
}

func TestBacklog_TableOutput(t *testing.T) {
	srv := newMockServer(t,
		map[string]string{
			"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"r1","status":"RUNNING","function":{"name":"Process Payment","slug":"fn1"}},"cursor":"c1"},{"node":{"id":"r2","status":"QUEUED","function":{"name":"Process Payment","slug":"fn1"}},"cursor":"c2"}],"pageInfo":{"hasNextPage":false},"totalCount":2}}}`,
		},
		nil,
	)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "table"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.AppVersion = "test"
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
