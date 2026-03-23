package commands

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
)

func TestEventsCmdHasSubcommands(t *testing.T) {
	cmd := NewEventsCmd()

	want := map[string]bool{
		"send":  false,
		"get":   false,
		"list":  false,
		"types": false,
	}

	for _, sub := range cmd.Commands() {
		name := sub.Name()
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("events command missing subcommand %q", name)
		}
	}
}

func TestEventsSendRequiresEventKey(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = "json"
	state.DevMode = false
	t.Setenv("INNGEST_EVENT_KEY", "")

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"send", "test/event", "--data", "{}"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no event key is set")
	}
	if !strings.Contains(err.Error(), "event key required") {
		t.Errorf("expected error about event key required, got: %v", err)
	}
}

func TestEventsSendNoArgError(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"send"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided to events send")
	}
}

func TestEventsGetNoArgError(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"get"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided to events get")
	}
}

func TestEventsListInvalidSince(t *testing.T) {
	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"list", "--since", "notaduration"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --since duration")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected error about invalid duration, got: %v", err)
	}
}

func TestNewCloudClient(t *testing.T) {
	state.Config = &config.Config{
		SigningKey: "signkey-test-123",
		EventKey:   "evt-key",
	}
	state.Env = "production"
	state.APIBaseURL = "https://api.inngest.com"
	state.DevServer = "http://localhost:8288"
	state.DevMode = false
	state.AppVersion = "v1.0.0"

	client := newCloudClient()
	if client == nil {
		t.Fatal("expected non-nil client from newCloudClient()")
	}
}

func TestEventsSend_Success(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ids":["evt-id-1"],"status":200}`))
		},
	})
	defer srv.Close()

	t.Setenv("INNGEST_EVENT_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY", "")

	state.Config = &config.Config{EventKey: "test-key"}
	state.Output = "json"
	state.DevMode = true
	state.DevServer = srv.URL
	state.APIBaseURL = srv.URL
	state.AppVersion = "test"

	cmd := NewEventsCmd()
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
		t.Errorf("expected output to contain \"event_ids\", got: %s", got)
	}
}

func TestEventsSend_Stdin(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ids":["evt-stdin-1"],"status":200}`))
		},
	})
	defer srv.Close()

	t.Setenv("INNGEST_EVENT_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY", "")

	state.Config = &config.Config{EventKey: "test-key"}
	state.Output = "json"
	state.DevMode = true
	state.DevServer = srv.URL
	state.APIBaseURL = srv.URL
	state.AppVersion = "test"

	// Feed JSON via stdin pipe.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(`{"from":"stdin"}`))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewEventsCmd()
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

func TestEventsSend_StdinInvalidJSON(t *testing.T) {
	t.Setenv("INNGEST_EVENT_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY", "")

	state.Config = &config.Config{EventKey: "test-key"}
	state.Output = "json"
	state.DevMode = true
	state.DevServer = "http://localhost:8288"
	state.APIBaseURL = "http://localhost:8288"
	state.AppVersion = "test"

	// Feed invalid JSON via stdin pipe.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(`not-json`))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewEventsCmd()
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

func TestEventsSend_StdinEmpty(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ids":["evt-empty-1"],"status":200}`))
		},
	})
	defer srv.Close()

	t.Setenv("INNGEST_EVENT_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY", "")

	state.Config = &config.Config{EventKey: "test-key"}
	state.Output = "json"
	state.DevMode = true
	state.DevServer = srv.URL
	state.APIBaseURL = srv.URL
	state.AppVersion = "test"

	// Feed empty stdin.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"send", "test/event"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "evt-empty-1") {
		t.Errorf("expected output to contain %q, got: %s", "evt-empty-1", got)
	}
}

func TestEventsCmd_BareHelp(t *testing.T) {
	cmd := NewEventsCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error from bare events command: %v", err)
	}
}

func TestEventsSend_InvalidData(t *testing.T) {
	t.Setenv("INNGEST_EVENT_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY", "")

	state.Config = &config.Config{EventKey: "test-key"}
	state.Output = "json"
	state.DevMode = true
	state.DevServer = "http://localhost:8288"
	state.APIBaseURL = "http://localhost:8288"
	state.AppVersion = "test"

	cmd := NewEventsCmd()
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

func TestEventsGet_GraphQLSuccess(t *testing.T) {
	gqlResponses := map[string]string{
		"GetEvent": `{"data":{"event":{"id":"evt-1","name":"test/event","raw":"{\"name\":\"test/event\"}","runs":[{"id":"run-1","status":"COMPLETED","function":{"name":"My Func"}}]}}}`,
	}
	srv := newMockServer(t, gqlResponses, nil)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = "test"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"get", "evt-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "evt-1") {
		t.Errorf("expected output to contain \"evt-1\", got: %s", got)
	}
}

func TestEventsGet_FallbackToREST(t *testing.T) {
	gqlResponses := map[string]string{
		"GetEvent": `{"data":null,"errors":[{"message":"not found"}]}`,
	}
	restHandlers := map[string]http.HandlerFunc{
		"/v1/*": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[{"run_id":"run-1","status":"COMPLETED","function_id":"fn-1"}]}`))
		},
	}
	srv := newMockServer(t, gqlResponses, restHandlers)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = "test"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"get", "evt-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "run-1") {
		t.Errorf("expected output to contain \"run-1\", got: %s", got)
	}
}

func TestEventsGet_BothFail(t *testing.T) {
	gqlResponses := map[string]string{
		"GetEvent": `{"data":null,"errors":[{"message":"not found"}]}`,
	}
	// No REST handlers — will 404.
	srv := newMockServer(t, gqlResponses, nil)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = "test"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"get", "evt-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both GraphQL and REST fail")
	}
}

func TestEventsList_Success(t *testing.T) {
	gqlResponses := map[string]string{
		"ListEvents": `{"data":{"eventsV2":{"edges":[{"node":{"id":"evt-1","name":"test/event","raw":"{}"},"cursor":"c1"},{"node":{"id":"evt-2","name":"other/event","raw":"{}"},"cursor":"c2"}],"pageInfo":{"hasNextPage":false},"totalCount":2}}}`,
	}
	srv := newMockServer(t, gqlResponses, nil)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = "test"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "evt-1") {
		t.Errorf("expected output to contain \"evt-1\", got: %s", got)
	}
	if !strings.Contains(got, "evt-2") {
		t.Errorf("expected output to contain \"evt-2\", got: %s", got)
	}
}

func TestEventsTypes_Success(t *testing.T) {
	gqlResponses := map[string]string{
		"ListEvents": `{"data":{"eventsV2":{"edges":[{"node":{"id":"e1","name":"test/event"},"cursor":"c1"},{"node":{"id":"e2","name":"other/event"},"cursor":"c2"},{"node":{"id":"e3","name":"test/event"},"cursor":"c3"}],"pageInfo":{"hasNextPage":false},"totalCount":3}}}`,
	}
	srv := newMockServer(t, gqlResponses, nil)
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = "test"

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"types"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "test/event") {
		t.Errorf("expected output to contain \"test/event\", got: %s", got)
	}
	if !strings.Contains(got, "other/event") {
		t.Errorf("expected output to contain \"other/event\", got: %s", got)
	}
	// Verify deduplication: count occurrences of event names in JSON output.
	// The JSON array should have exactly 2 entries.
	count := strings.Count(got, "test/event")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of \"test/event\" (deduped), got %d in: %s", count, got)
	}
}
