package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
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

// Without an event key the event goes through the REST API with the signing key.
func TestEventsSend_RESTWithoutEventKey(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/events": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "test/event" || body["id"] != "idem-1" {
				t.Errorf("unexpected body: %v", body)
			}
			if r.Header.Get("Authorization") == "" {
				t.Error("expected Authorization header on REST send")
			}
			jsonOK(`{"data":{"eventId":"evt-rest-1"}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)
	t.Setenv("INNGEST_EVENT_KEY", "")

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"send", "test/event", "--data", "{}", "--id", "idem-1"})
	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(got, `"evt-rest-1"`) {
		t.Errorf("expected REST event id in output, got: %s", got)
	}
}

func TestEventsSendNoArgError(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON

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
	state.Output = testOutputJSON

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

func TestEventsListFlags(t *testing.T) {
	cmd := NewEventsCmd()
	listCmd, _, _ := cmd.Find([]string{"list"})
	if listCmd == nil {
		t.Fatal("expected list subcommand")
	}
	for _, name := range []string{"limit", "since", "after"} {
		if f := listCmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on list command", name)
		}
	}
	if f := listCmd.Flags().Lookup("name"); f == nil {
		t.Error("expected --name flag on list command")
	}
}

func TestNewCloudClient(t *testing.T) {
	state.Config = &config.Config{
		SigningKey: "signkey-test-123",
		EventKey:   "evt-key",
	}
	state.Env = "production"
	state.APIBaseURL = "https://api.inngest.com"
	state.DevServer = testDevServerURL
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
	state.Output = testOutputJSON
	state.DevMode = true
	state.DevServer = srv.URL
	state.APIBaseURL = srv.URL
	state.AppVersion = testAppVersion

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
	state.Output = testOutputJSON
	state.DevMode = true
	state.DevServer = srv.URL
	state.APIBaseURL = srv.URL
	state.AppVersion = testAppVersion

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
	state.Output = testOutputJSON
	state.DevMode = true
	state.DevServer = testDevServerURL
	state.APIBaseURL = testDevServerURL
	state.AppVersion = testAppVersion

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
	if !strings.Contains(err.Error(), "invalid JSON input") {
		t.Errorf("expected error about invalid JSON input, got: %v", err)
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
	state.Output = testOutputJSON
	state.DevMode = true
	state.DevServer = srv.URL
	state.APIBaseURL = srv.URL
	state.AppVersion = testAppVersion

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
	state.Output = testOutputJSON
	state.DevMode = true
	state.DevServer = testDevServerURL
	state.APIBaseURL = testDevServerURL
	state.AppVersion = testAppVersion

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"send", "test/event", "--data", "not-json"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON data")
	}
	if !strings.Contains(err.Error(), "invalid JSON input") {
		t.Errorf("expected error about invalid JSON, got: %v", err)
	}
}

const v1EventJSON = `{"data":{"internal_id":"evt-1","name":"test/event","data":{"userId":"1"},"ts":1704067200000,"received_at":"2024-01-01T00:00:00Z"}}`

func TestEventsGet_Success(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v1/events/evt-1":      jsonOK(v1EventJSON),
		"/v2/events/evt-1/runs": jsonOK(`{"data":[{"id":"run-1","status":"COMPLETED","function":{"id":"fn-1","name":"My Func","slug":"my-func"},"trigger":{"eventName":"test/event"}}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"get", "evt-1"})
	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse output: %v\n%s", err, got)
	}
	event, _ := result["event"].(map[string]any)
	if event["internal_id"] != "evt-1" || event["name"] != "test/event" {
		t.Errorf("unexpected event: %v", event)
	}
	runs, _ := result["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %v", result["runs"])
	}
	if run := runs[0].(map[string]any); run["id"] != "run-1" || run["functionID"] != testFnID {
		t.Errorf("unexpected run: %v", run)
	}
}

func TestEventsGet_EventNotFound(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v1/events/evt-1": jsonStatus(http.StatusNotFound, `{"error":"Not Found","status":404}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"get", "evt-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "getting event") {
		t.Fatalf("expected getting event error, got: %v", err)
	}
}

func TestEventsGet_RunsError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v1/events/evt-1":      jsonOK(v1EventJSON),
		"/v2/events/evt-1/runs": jsonStatus(http.StatusUnauthorized, v2Unauthorized),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"get", "evt-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if !inngest.IsAuthError(err) {
		t.Fatalf("expected auth error from runs lookup, got: %v", err)
	}
}

func TestEventsList_Success(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v1/events": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "20" {
				t.Errorf("expected default limit=20, got %q", r.URL.RawQuery)
			}
			jsonOK(`{"data":[{"internal_id":"evt-1","name":"test/event","data":{}},{"internal_id":"evt-2","name":"other/event","data":{}}]}`)(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = testAppVersion

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
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/insights/events/schemas": jsonOK(`{"data":[{"name":"test/event","schema":{"type":"object"}},{"name":"other/event"}],"page":{"hasMore":false}}`),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = testAppVersion

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
}

func TestEventsSend_StdinReadError(t *testing.T) {
	t.Setenv("INNGEST_EVENT_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY", "")

	state.Config = &config.Config{EventKey: "test-key"}
	state.Output = testOutputJSON
	state.DevMode = true
	state.DevServer = testDevServerURL
	state.APIBaseURL = testDevServerURL
	state.AppVersion = testAppVersion

	oldStdin := os.Stdin
	r, _, _ := os.Pipe()
	r.Close() // Close immediately to trigger read error
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	cmd := NewEventsCmd()
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

func TestEventsSend_SendError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/e/*": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`server error`))
		},
	})
	defer srv.Close()

	t.Setenv("INNGEST_EVENT_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY", "")

	state.Config = &config.Config{EventKey: "test-key"}
	state.Output = testOutputJSON
	state.DevMode = true
	state.DevServer = srv.URL
	state.APIBaseURL = srv.URL
	state.AppVersion = testAppVersion

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"send", "test/event", "--data", `{"key":"val"}`})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when SendEvent fails")
	}
	if !strings.Contains(err.Error(), "sending event") {
		t.Errorf("expected error about sending event, got: %v", err)
	}
}

func TestEventsList_Error(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v1/events":                  jsonStatus(http.StatusUnauthorized, `{"error":"Unauthorized","status":401}`),
		"/v2/insights/events/schemas": jsonStatus(http.StatusUnauthorized, v2Unauthorized),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = testAppVersion

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListEvents fails")
	}
	if !strings.Contains(err.Error(), "listing events") {
		t.Errorf("expected error about listing events, got: %v", err)
	}
}

func TestEventsTypes_ListError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v1/events":                  jsonStatus(http.StatusUnauthorized, `{"error":"Unauthorized","status":401}`),
		"/v2/insights/events/schemas": jsonStatus(http.StatusUnauthorized, v2Unauthorized),
	})
	defer srv.Close()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = testAppVersion

	cmd := NewEventsCmd()
	cmd.SetArgs([]string{"types"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when the schemas API fails")
	}
	if !strings.Contains(err.Error(), "listing event types") || !inngest.IsAuthError(err) {
		t.Errorf("expected auth error about listing event types, got: %v", err)
	}
}
