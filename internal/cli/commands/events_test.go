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
	state.Output = testOutputJSON
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
	if f := listCmd.Flags().Lookup("recent"); f == nil {
		t.Error("expected --recent flag on list command")
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
	if !strings.Contains(err.Error(), "invalid --data JSON") {
		t.Errorf("expected error about invalid JSON, got: %v", err)
	}
}

func TestEventsGet_GraphQLSuccess(t *testing.T) {
	gqlResponses := map[string]string{
		"GetEvent": `{"data":{"events":{"data":[{"name":"test/event","recent":[{"id":"evt-1","name":"test/event","event":"{\"name\":\"test/event\"}","functionRuns":[{"id":"run-1","status":"COMPLETED","function":{"id":"fn-1","name":"My Func","slug":"my-func"}}]}]}]}}}`,
	}
	srv := newMockServer(t, gqlResponses, nil)
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
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = testAppVersion

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
	state.Output = testOutputJSON
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = testAppVersion

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
		"ListEvents": `{"data":{"events":{"data":[{"name":"test/event","recent":[{"id":"evt-1","name":"test/event","event":"{}"}]},{"name":"other/event","recent":[{"id":"evt-2","name":"other/event","event":"{}"}]}],"page":{"page":1,"perPage":20,"totalItems":2,"totalPages":1}}}}`,
	}
	srv := newMockServer(t, gqlResponses, nil)
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
	gqlResponses := map[string]string{
		"ListEvents": `{"data":{"events":{"data":[{"name":"test/event"},{"name":"other/event"}],"page":{"page":1,"perPage":20,"totalItems":2,"totalPages":1}}}}`,
	}
	srv := newMockServer(t, gqlResponses, nil)
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
	srv := newMockServer(t, map[string]string{
		"ListEvents": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
	}, nil)
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
	srv := newMockServer(t, map[string]string{
		"ListEvents": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
	}, nil)
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
		t.Fatal("expected error when ListEvents fails")
	}
	if !strings.Contains(err.Error(), "listing events") {
		t.Errorf("expected error about listing events, got: %v", err)
	}
}
