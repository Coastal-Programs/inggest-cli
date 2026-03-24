package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
)

const (
	testDevServerURL = "http://localhost:8288"
	testAppVersion   = "test"
)

func TestDevCmdHasSubcommands(t *testing.T) {
	cmd := NewDevCmd()

	want := map[string]bool{
		"status":    false,
		"functions": false,
		"runs":      false,
		"send":      false,
		"events":    false,
		"invoke":    false,
	}

	for _, sub := range cmd.Commands() {
		name := sub.Name()
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("dev command missing subcommand %q", name)
		}
	}
}

func TestDevSendRequiresArg(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON
	state.DevServer = testDevServerURL
	state.AppVersion = testAppVersion

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"send"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided to dev send")
	}
}

func TestDevSendInvalidData(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON
	state.DevServer = testDevServerURL
	state.AppVersion = testAppVersion

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"send", "test/event", "--data", "not-json"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON data")
	}
	if !strings.Contains(err.Error(), "invalid --data JSON") {
		t.Errorf("expected error about invalid JSON, got: %v", err)
	}
}

func TestDevInvokeRequiresArg(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON
	state.DevServer = testDevServerURL
	state.AppVersion = testAppVersion

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"invoke"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided to dev invoke")
	}
}

func TestDevInvokeInvalidData(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON
	state.DevServer = testDevServerURL
	state.AppVersion = testAppVersion

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"invoke", "my-func", "--data", "not-json"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON data")
	}
	if !strings.Contains(err.Error(), "invalid --data JSON") {
		t.Errorf("expected error about invalid JSON, got: %v", err)
	}
}

func TestDevRunsInvalidSince(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON
	state.DevServer = testDevServerURL
	state.AppVersion = testAppVersion

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"runs", "--since", "notaduration"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --since duration")
	}
	if !strings.Contains(err.Error(), "invalid --since duration") {
		t.Errorf("expected error about invalid duration, got: %v", err)
	}
}

func TestNewDevClient(t *testing.T) {
	state.DevServer = testDevServerURL
	state.AppVersion = "v1.0.0"

	client := newDevClient()
	if client == nil {
		t.Fatal("expected non-nil client from newDevClient()")
	}
}

// ---------------------------------------------------------------------------
// New integration tests using mock server
// ---------------------------------------------------------------------------

func TestDevStatus_Online(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/dev": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"version":"0.1.0","functions":[{"id":"fn1","name":"test-fn"}]}`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"status"})
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

	if result["status"] != "online" {
		t.Errorf("expected status %q, got %q", "online", result["status"])
	}
	if result["version"] != "0.1.0" {
		t.Errorf("expected version %q, got %q", "0.1.0", result["version"])
	}
	// functions count is returned as float64 from JSON unmarshalling
	if fns, ok := result["functions"].(float64); !ok || fns != 1 {
		t.Errorf("expected functions count 1, got %v", result["functions"])
	}
}

func TestDevStatus_Offline(t *testing.T) {
	srv := newMockServer(t, nil, nil)
	closedURL := srv.URL
	srv.Close() // close immediately so the server is unreachable

	state.DevServer = closedURL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"status"})
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

	if result["status"] != "offline" {
		t.Errorf("expected status %q, got %q", "offline", result["status"])
	}
}

func TestDevFunctions(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": `{"data":{"functions":[{"id":"fn-1","name":"My Function","slug":"my-func","triggers":[{"type":"event","value":"test/event"}],"app":{"name":"test-app"}}]}}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"functions"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON array output: %v\nraw output: %s", err, got)
	}

	if len(result) == 0 {
		t.Fatal("expected at least one function in output")
	}
	if result[0]["name"] != "My Function" {
		t.Errorf("expected first function name %q, got %q", "My Function", result[0]["name"])
	}
}

func TestDevRuns_Success(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"DevRuns": `{"data":{"runs":{"edges":[{"node":{"id":"run-1","status":"COMPLETED","eventName":"test/event","function":{"name":"My Func","slug":"my-func"}}}],"totalCount":1}}}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"runs", "--since", "1h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"run-1"`) {
		t.Errorf("expected output to contain %q, got: %s", "run-1", got)
	}
}

func TestDevSend_Success(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ids":["evt-id-1"],"status":200}`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"send", "test/event", "--data", `{"key":"val"}`})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"event_ids"`) {
		t.Errorf("expected output to contain event_ids, got: %s", got)
	}
	if !strings.Contains(got, `"evt-id-1"`) {
		t.Errorf("expected output to contain %q, got: %s", "evt-id-1", got)
	}
}

func TestDevInvoke_Success(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/invoke/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"run-id-1","status":200}`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"invoke", "my-func", "--data", `{"key":"val"}`})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"event_id"`) {
		t.Errorf("expected output to contain event_id, got: %s", got)
	}
	if !strings.Contains(got, `"run-id-1"`) {
		t.Errorf("expected output to contain %q, got: %s", "run-id-1", got)
	}
}

func TestDevInvoke_NoData(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/invoke/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"run-id-1","status":200}`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"invoke", "my-func"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"event_id"`) {
		t.Errorf("expected output to contain event_id, got: %s", got)
	}
	if !strings.Contains(got, `"run-id-1"`) {
		t.Errorf("expected output to contain %q, got: %s", "run-id-1", got)
	}
}

func TestDevSend_Stdin(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ids":["evt-stdin-1"],"status":200}`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	// Feed JSON via stdin pipe.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(`{"user":"test"}`))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewDevCmd()
	// No --data flag; the command should read from stdin.
	cmd.SetArgs([]string{"send", "test/stdin-event"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "evt-stdin-1") {
		t.Errorf("expected output to contain %q, got: %s", "evt-stdin-1", got)
	}
}

func TestDevSend_StdinInvalidJSON(t *testing.T) {
	state.DevServer = testDevServerURL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	// Feed invalid JSON via stdin pipe.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(`not-json`))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"send", "test/event"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid stdin JSON")
	}
	if !strings.Contains(err.Error(), "invalid stdin JSON") {
		t.Errorf("expected error about invalid stdin JSON, got: %v", err)
	}
}

func TestDevSend_StdinEmpty(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ids":["evt-empty-1"],"status":200}`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	// Feed empty stdin.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Close() // close immediately — empty stdin
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"send", "test/event"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Should succeed with empty data.
	if !strings.Contains(got, "evt-empty-1") {
		t.Errorf("expected output to contain %q, got: %s", "evt-empty-1", got)
	}
}

func TestDevCmd_BareHelp(t *testing.T) {
	cmd := NewDevCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error from bare dev command: %v", err)
	}
}

func TestDevEvents_Success(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListDevEvents": `{"data":{"events":[{"id":"evt-1","name":"test/event","createdAt":"2024-01-01T00:00:00Z","status":"received","totalRuns":1},{"id":"evt-2","name":"other/event","createdAt":"2024-01-01T00:00:00Z","status":"received","totalRuns":0}]}}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"events"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"evt-1"`) {
		t.Errorf("expected output to contain %q, got: %s", "evt-1", got)
	}
	if !strings.Contains(got, `"evt-2"`) {
		t.Errorf("expected output to contain %q, got: %s", "evt-2", got)
	}
}

func TestDevEvents_NameFilter(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListDevEvents": `{"data":{"events":[{"id":"evt-1","name":"test/event","createdAt":"2024-01-01T00:00:00Z","status":"received","totalRuns":1},{"id":"evt-2","name":"other/event","createdAt":"2024-01-01T00:00:00Z","status":"received","totalRuns":0}]}}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"events", "--name", "test/event"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"evt-1"`) {
		t.Errorf("expected output to contain %q, got: %s", "evt-1", got)
	}
	if strings.Contains(got, `"evt-2"`) {
		t.Errorf("expected output NOT to contain %q, got: %s", "evt-2", got)
	}
}

func TestDevEvents_LimitFilter(t *testing.T) {
	// Build 5 events for the mock response.
	events := make([]map[string]any, 5)
	for i := range 5 {
		events[i] = map[string]any{
			"id":        fmt.Sprintf("evt-%d", i+1),
			"name":      fmt.Sprintf("event/%d", i+1),
			"createdAt": "2024-01-01T00:00:00Z",
			"status":    "received",
			"totalRuns": 0,
		}
	}
	eventsJSON, _ := json.Marshal(events)
	gqlResp := fmt.Sprintf(`{"data":{"events":%s}}`, string(eventsJSON))

	srv := newMockServer(t, map[string]string{
		"ListDevEvents": gqlResp,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"events", "--limit", "2"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON array output: %v\nraw output: %s", err, got)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 events after --limit 2, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// Tests for uncovered error / branch paths
// ---------------------------------------------------------------------------

// dev.go:63 – GetDevInfo error path in newDevStatusCmd.
// IsDevServerRunning and GetDevInfo both hit GET /dev. The first call (IsDevServerRunning)
// must return 200 so the server is considered online. The second call (GetDevInfo)
// must return invalid JSON so parsing fails.
func TestDevStatus_GetDevInfoError(t *testing.T) {
	var callCount int
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/dev": func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				// IsDevServerRunning: just needs 200
				w.WriteHeader(http.StatusOK)
				return
			}
			// GetDevInfo: return invalid JSON to trigger unmarshal error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not-json`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"status"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GetDevInfo fails")
	}
	if !strings.Contains(err.Error(), "fetching dev server info") {
		t.Errorf("expected error about fetching dev server info, got: %v", err)
	}
}

// dev.go:110 – GraphQL error in newDevFunctionsCmd.
func TestDevFunctions_GraphQLError(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": `{"data":null,"errors":[{"message":"something went wrong"}]}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"functions"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GraphQL returns errors for ListFunctions")
	}
	if !strings.Contains(err.Error(), "querying functions") {
		t.Errorf("expected error about querying functions, got: %v", err)
	}
}

// dev.go:162-167 – status and function filter branches in newDevRunsCmd.
func TestDevRuns_StatusAndFunctionFilters(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"DevRuns": `{"data":{"runs":{"edges":[{"node":{"id":"run-filtered","status":"FAILED","eventName":"test/event","function":{"name":"My Func","slug":"my-func"}}}],"totalCount":1}}}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"runs", "--since", "1h", "--status", "failed", "--function", "my-func"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"run-filtered"`) {
		t.Errorf("expected output to contain %q, got: %s", "run-filtered", got)
	}
}

// dev.go:177 – GraphQL error in newDevRunsCmd.
func TestDevRuns_GraphQLError(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"DevRuns": `{"data":null,"errors":[{"message":"runs query failed"}]}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"runs", "--since", "1h"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GraphQL returns errors for DevRuns")
	}
	if !strings.Contains(err.Error(), "querying runs") {
		t.Errorf("expected error about querying runs, got: %v", err)
	}
}

// dev.go:223 – stdin read error in newDevSendCmd.
func TestDevSend_StdinReadError(t *testing.T) {
	state.DevServer = testDevServerURL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	// Replace stdin with a closed file descriptor to trigger io.ReadAll error.
	oldStdin := os.Stdin
	r, _, _ := os.Pipe()
	r.Close() // Close immediately so io.ReadAll returns an error
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"send", "test/event"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when stdin read fails")
	}
	if !strings.Contains(err.Error(), "reading stdin") {
		t.Errorf("expected error about reading stdin, got: %v", err)
	}
}

// dev.go:244 – SendDevEvent error path in newDevSendCmd.
func TestDevSend_SendDevEventError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`internal server error`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"send", "test/event", "--data", `{"key":"val"}`})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when SendDevEvent fails")
	}
	if !strings.Contains(err.Error(), "sending event") {
		t.Errorf("expected error about sending event, got: %v", err)
	}
}

// dev.go:285 – InvokeDevFunction error path in newDevInvokeCmd.
func TestDevInvoke_InvokeDevFunctionError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/invoke/*": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`internal server error`))
		},
	})
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"invoke", "my-func", "--data", `{"key":"val"}`})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when InvokeDevFunction fails")
	}
	if !strings.Contains(err.Error(), "invoking function") {
		t.Errorf("expected error about invoking function, got: %v", err)
	}
}

// dev.go:327 – GraphQL error in newDevEventsCmd.
func TestDevEvents_GraphQLError(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListDevEvents": `{"data":null,"errors":[{"message":"events query failed"}]}`,
	}, nil)
	defer srv.Close()

	state.DevServer = srv.URL
	state.Output = testOutputJSON
	state.AppVersion = testAppVersion
	state.Config = &config.Config{}

	cmd := NewDevCmd()
	cmd.SetArgs([]string{"events"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GraphQL returns errors for ListDevEvents")
	}
	if !strings.Contains(err.Error(), "querying events") {
		t.Errorf("expected error about querying events, got: %v", err)
	}
}
