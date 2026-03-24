package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

func TestEnvCmdHasSubcommands(t *testing.T) {
	cmd := NewEnvCmd()

	want := map[string]bool{
		"list": false,
		"use":  false,
		"get":  false,
	}

	for _, sub := range cmd.Commands() {
		name := sub.Name()
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("env command missing subcommand %q", name)
		}
	}
}

func TestEnvUseUpdatesConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"use", "staging"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Config.ActiveEnv != "staging" {
		t.Errorf("expected ActiveEnv %q, got %q", "staging", state.Config.ActiveEnv)
	}
	if state.Env != "staging" {
		t.Errorf("expected state.Env %q, got %q", "staging", state.Env)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	if !strings.Contains(string(data), "staging") {
		t.Errorf("saved config does not contain %q, got: %s", "staging", data)
	}
}

func TestEnvUseRequiresArg(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"use"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no argument is provided to env use")
	}
}

func TestPrintEnvTable(t *testing.T) {
	state.Env = "my-app"
	state.Output = "table"

	apps := []inngest.App{
		{
			ID:            "app-1",
			Name:          "my-app",
			SDKLanguage:   "go",
			SDKVersion:    "0.1.0",
			Framework:     "stdlib",
			URL:           "https://example.com/api/inngest",
			Connected:     true,
			FunctionCount: 3,
		},
		{
			ID:            "app-2",
			Name:          "other-app",
			SDKLanguage:   "typescript",
			SDKVersion:    "1.0.0",
			Framework:     "next",
			URL:           "https://other.com/api/inngest",
			Connected:     false,
			FunctionCount: 1,
		},
	}

	err := printEnvTable(apps)
	if err != nil {
		t.Fatalf("printEnvTable returned error: %v", err)
	}
}

func TestPrintEnvDetail(t *testing.T) {
	app := &inngest.App{
		ID:            "app-123",
		ExternalID:    "ext-456",
		Name:          "my-app",
		SDKLanguage:   "go",
		SDKVersion:    "0.1.0",
		Framework:     "stdlib",
		URL:           "https://example.com/api/inngest",
		Method:        "POST",
		Connected:     true,
		FunctionCount: 2,
		Checksum:      "abc123",
		Functions: []inngest.Function{
			{
				Name: "process-order",
				Slug: "my-app-process-order",
				Triggers: []inngest.FunctionTrigger{
					{Type: "event", Value: "order/created"},
				},
			},
			{
				Name: "send-email",
				Slug: "my-app-send-email",
				Triggers: []inngest.FunctionTrigger{
					{Type: "event", Value: "email/requested"},
				},
			},
		},
	}

	err := printEnvDetail(app)
	if err != nil {
		t.Fatalf("printEnvDetail returned error: %v", err)
	}
}

// setupCloudState configures state globals for cloud tests pointing at the given server URL.
func setupCloudState(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = "json"
	state.APIBaseURL = srvURL
	state.DevServer = srvURL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = "test"
}

const listAppsResponse = `{"data":{"apps":[{"id":"app-1","name":"my-app","sdkLanguage":"go","sdkVersion":"0.1.0","framework":"stdlib","url":"https://example.com","connected":true,"functionCount":3},{"id":"app-2","name":"other-app","sdkLanguage":"typescript","sdkVersion":"1.0.0","connected":false,"functionCount":1}]}}`

const getAppResponse = `{"data":{"app":{"id":"app-1","externalID":"ext-1","name":"my-app","sdkLanguage":"go","sdkVersion":"0.1.0","framework":"stdlib","url":"https://example.com","connected":true,"functionCount":3,"functions":[{"id":"fn-1","name":"process","slug":"my-app-process","triggers":[{"type":"event","value":"order/created"}]}]}}}`

func TestEnvList_JSON(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListApps": listAppsResponse,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = "json"

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"list"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	var apps []json.RawMessage
	if err := json.Unmarshal([]byte(out), &apps); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, out)
	}

	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestEnvList_Table(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListApps": listAppsResponse,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = "table"

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"list"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	if !strings.Contains(out, "my-app") {
		t.Errorf("expected table output to contain %q, got: %s", "my-app", out)
	}
	if !strings.Contains(out, "other-app") {
		t.Errorf("expected table output to contain %q, got: %s", "other-app", out)
	}
}

func TestEnvGet_ByName(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListApps": listAppsResponse,
		"GetApp":   getAppResponse,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = "json"

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"get", "my-app"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	if !strings.Contains(out, "my-app") {
		t.Errorf("expected output to contain %q, got: %s", "my-app", out)
	}
}

func TestEnvGet_NotFound(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListApps": `{"data":{"apps":[]}}`,
		"GetApp":   `{"data":null,"errors":[{"message":"not found"}]}`,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = "json"

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"get", "nonexistent"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when app is not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain %q, got: %v", "not found", err)
	}
}

func TestEnvGet_TextOutput(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListApps": listAppsResponse,
		"GetApp":   getAppResponse,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = "text"

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"get", "my-app"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	if !strings.Contains(out, "my-app") {
		t.Errorf("expected text output to contain %q, got: %s", "my-app", out)
	}
	if !strings.Contains(out, "Name:") {
		t.Errorf("expected text output to contain %q, got: %s", "Name:", out)
	}
}

func TestEnvGet_ByID(t *testing.T) {
	// List doesn't find by name, so fallback to GetApp by ID
	srv := newMockServer(t, map[string]string{
		"ListApps": `{"data":{"apps":[]}}`,
		"GetApp":   getAppResponse,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = "json"

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"get", "app-1"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	if !strings.Contains(out, "my-app") {
		t.Errorf("expected output to contain %q, got: %s", "my-app", out)
	}
}

func TestEnvCmd_BareHelp(t *testing.T) {
	cmd := NewEnvCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error from bare env command: %v", err)
	}
}

func TestEnvList_Error(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListApps": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListApps fails")
	}
	if !strings.Contains(err.Error(), "listing environments") {
		t.Errorf("expected error about listing environments, got: %v", err)
	}
}

func TestEnvUse_SaveError(t *testing.T) {
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", "/dev/null/impossible/cli.json")
	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"use", "staging"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config save fails")
	}
	if !strings.Contains(err.Error(), "saving config") {
		t.Errorf("expected error about saving config, got: %v", err)
	}
}

func TestEnvGet_ListAppsError(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListApps": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
	}, map[string]http.HandlerFunc{})
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"get", "my-app"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListApps fails")
	}
	if !strings.Contains(err.Error(), "listing environments") {
		t.Errorf("expected error about listing environments, got: %v", err)
	}
}

func TestPrintEnvDetail_WithError(t *testing.T) {
	app := &inngest.App{
		ID:            "app-123",
		ExternalID:    "ext-456",
		Name:          "my-app",
		SDKLanguage:   "go",
		SDKVersion:    "0.1.0",
		Connected:     false,
		FunctionCount: 0,
		Error:         "connection refused",
	}

	got := captureStdout(t, func() {
		if err := printEnvDetail(app); err != nil {
			t.Fatalf("printEnvDetail returned error: %v", err)
		}
	})

	if !strings.Contains(got, "Error:") {
		t.Errorf("expected output to contain 'Error:', got: %s", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Errorf("expected output to contain 'connection refused', got: %s", got)
	}
}
