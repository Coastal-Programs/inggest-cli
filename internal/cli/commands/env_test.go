package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

// ---------------------------------------------------------------------------
// Shared REST v2 fixtures and helpers (used by env/functions/events/apps/auth tests)
// ---------------------------------------------------------------------------

const (
	envProduction = "production"
	envStaging    = "staging"

	// envListV2JSON is the GET /v2/envs envelope.
	envListV2JSON = `{"data":[{"id":"env-1","name":"production","type":"PRODUCTION","isArchived":false,"createdAt":"2024-01-01T00:00:00Z"},{"id":"env-2","name":"staging","type":"BRANCH","isArchived":false}],"page":{"cursor":"","hasMore":false,"limit":100}}`

	// v2AuthErrorJSON is the body the v2 API returns with a 401.
	v2AuthErrorJSON = `{"errors":[{"code":"authorization_header_missing","message":"authorization header missing or invalid"}]}`
)

// v2Reply returns a handler that writes a canned JSON body with the given status.
func v2Reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// runCapture executes cmd with args and returns what it printed to stdout.
// The test fails if the command returns an error.
func runCapture(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	var execErr error
	got := captureStdout(t, func() {
		execErr = cmd.Execute()
	})
	if execErr != nil {
		t.Fatalf("unexpected error running %v: %v\noutput: %s", args, execErr, got)
	}
	return got
}

// runExpectErr executes cmd with args and returns the error, failing if there is none.
func runExpectErr(t *testing.T, cmd *cobra.Command, args ...string) error {
	t.Helper()
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	var execErr error
	got := captureStdout(t, func() {
		execErr = cmd.Execute()
	})
	if execErr == nil {
		t.Fatalf("expected error running %v, got none\noutput: %s", args, got)
	}
	return execErr
}

// ---------------------------------------------------------------------------
// Command wiring
// ---------------------------------------------------------------------------

func TestEnvCmdHasSubcommands(t *testing.T) {
	cmd := NewEnvCmd()

	want := map[string]bool{"list": false, "use": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("env command missing subcommand %q", name)
		}
	}
}

func TestEnvCmd_BareHelp(t *testing.T) {
	cmd := NewEnvCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error from bare env command: %v", err)
	}
	if !strings.Contains(buf.String(), "list") {
		t.Errorf("expected help output to mention subcommands, got: %s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// env use
// ---------------------------------------------------------------------------

func TestEnvUseUpdatesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "cli.json")
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	state.Config = &config.Config{}
	state.Output = testOutputJSON
	state.Env = ""

	got := runCapture(t, NewEnvCmd(), "use", envStaging)

	var result map[string]string
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, got)
	}
	if result["status"] != "ok" || result["active_env"] != envStaging {
		t.Errorf("unexpected output: %v", result)
	}

	if state.Config.ActiveEnv != envStaging {
		t.Errorf("expected ActiveEnv %q, got %q", envStaging, state.Config.ActiveEnv)
	}
	if state.Env != envStaging {
		t.Errorf("expected state.Env %q, got %q", envStaging, state.Env)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	if !strings.Contains(string(data), envStaging) {
		t.Errorf("saved config does not contain %q, got: %s", envStaging, data)
	}

	// The persisted file must round-trip through the config loader.
	config.ResetForTest()
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reloading config: %v", err)
	}
	if loaded.ActiveEnv != envStaging {
		t.Errorf("expected reloaded ActiveEnv %q, got %q", envStaging, loaded.ActiveEnv)
	}
}

func TestEnvUseRequiresArg(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"use"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when no argument is provided to env use")
	}
}

func TestEnvUse_SaveError(t *testing.T) {
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", "/dev/null/impossible/cli.json")

	state.Config = &config.Config{}
	state.Output = testOutputJSON

	err := runExpectErr(t, NewEnvCmd(), "use", envStaging)
	if !strings.Contains(err.Error(), "saving config") {
		t.Errorf("expected error about saving config, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Printers
// ---------------------------------------------------------------------------

func TestPrintEnvTable(t *testing.T) {
	state.Env = "Production" // active-env match is case-insensitive
	state.Output = testOutputTable

	envs := []inngest.Environment{
		{ID: "env-1", Name: envProduction, Type: "PRODUCTION"},
		{ID: "env-2", Name: envStaging, Type: "BRANCH"},
	}

	got := captureStdout(t, func() {
		if err := printEnvTable(envs); err != nil {
			t.Fatalf("printEnvTable returned error: %v", err)
		}
	})

	for _, col := range []string{"NAME", "TYPE", "ID", "ACTIVE"} {
		if !strings.Contains(got, col) {
			t.Errorf("expected table header to contain %q, got:\n%s", col, got)
		}
	}
	for line := range strings.SplitSeq(got, "\n") {
		switch {
		case strings.Contains(line, envProduction):
			if !strings.Contains(line, "◀") {
				t.Errorf("expected active marker on production row, got: %q", line)
			}
		case strings.Contains(line, envStaging):
			if strings.Contains(line, "◀") {
				t.Errorf("did not expect active marker on staging row, got: %q", line)
			}
		}
	}
}

func TestPrintEnvDetail(t *testing.T) {
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	env := &inngest.Environment{
		ID:         "env-123",
		Name:       envProduction,
		Type:       "PRODUCTION",
		IsArchived: true,
		CreatedAt:  &created,
	}

	got := captureStdout(t, func() {
		if err := printEnvDetail(env); err != nil {
			t.Fatalf("printEnvDetail returned error: %v", err)
		}
	})

	for _, want := range []string{
		"Name:          production",
		"ID:            env-123",
		"Type:          PRODUCTION",
		"Created:       2024-01-02 03:04:05",
		"Archived:      true",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestPrintEnvDetail_NoCreatedAt(t *testing.T) {
	env := &inngest.Environment{ID: "env-1", Name: envStaging, Type: "BRANCH"}

	got := captureStdout(t, func() {
		if err := printEnvDetail(env); err != nil {
			t.Fatalf("printEnvDetail returned error: %v", err)
		}
	})

	if strings.Contains(got, "Created:") {
		t.Errorf("did not expect Created line without CreatedAt, got:\n%s", got)
	}
	if !strings.Contains(got, "Archived:      false") {
		t.Errorf("expected Archived false line, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// env list (GET /v2/envs)
// ---------------------------------------------------------------------------

func TestEnvList_JSON(t *testing.T) {
	var gotQuery, gotAuth, gotMethod string
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			gotAuth = r.Header.Get("Authorization")
			gotMethod = r.Method
			v2Reply(http.StatusOK, envListV2JSON)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got := runCapture(t, NewEnvCmd(), "list")

	var envs []inngest.Environment
	if err := json.Unmarshal([]byte(got), &envs); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, got)
	}
	if len(envs) != 2 {
		t.Fatalf("expected 2 environments, got %d: %s", len(envs), got)
	}
	if envs[0].ID != "env-1" || envs[0].Name != envProduction || envs[0].Type != "PRODUCTION" {
		t.Errorf("unexpected first env: %+v", envs[0])
	}
	if envs[0].CreatedAt == nil || envs[0].CreatedAt.Year() != 2024 {
		t.Errorf("expected createdAt to be decoded, got %v", envs[0].CreatedAt)
	}
	if envs[1].ID != "env-2" || envs[1].Type != "BRANCH" {
		t.Errorf("unexpected second env: %+v", envs[1])
	}

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotQuery != "limit=100" {
		t.Errorf("expected query limit=100, got %q", gotQuery)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("expected Bearer authorization header, got %q", gotAuth)
	}
}

func TestEnvList_Table(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusOK, envListV2JSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)
	state.Output = testOutputTable
	state.Env = "PRODUCTION"

	got := runCapture(t, NewEnvCmd(), "list")

	for _, want := range []string{"NAME", "TYPE", "ID", "ACTIVE", envProduction, envStaging, "env-1", "env-2", "◀"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected table output to contain %q, got:\n%s", want, got)
		}
	}
	for line := range strings.SplitSeq(got, "\n") {
		if strings.Contains(line, envStaging) && strings.Contains(line, "◀") {
			t.Errorf("staging must not be marked active, got: %q", line)
		}
	}
}

func TestEnvList_Pagination(t *testing.T) {
	var calls int
	var cursors []string
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": func(w http.ResponseWriter, r *http.Request) {
			calls++
			cursors = append(cursors, r.URL.Query().Get("cursor"))
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("cursor") == "" {
				_, _ = w.Write([]byte(`{"data":[{"id":"env-1","name":"production","type":"PRODUCTION"}],"page":{"cursor":"c2","hasMore":true,"limit":100}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"env-2","name":"staging","type":"BRANCH"},{"id":"env-3","name":"qa","type":"TEST"}],"page":{"cursor":"","hasMore":false,"limit":100}}`))
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got := runCapture(t, NewEnvCmd(), "list")

	var envs []inngest.Environment
	if err := json.Unmarshal([]byte(got), &envs); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, got)
	}
	if len(envs) != 3 {
		t.Errorf("expected 3 environments across pages, got %d", len(envs))
	}
	if calls != 2 {
		t.Errorf("expected 2 page requests, got %d", calls)
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "c2" {
		t.Errorf("expected cursors [\"\" \"c2\"], got %q", cursors)
	}
}

func TestEnvList_AuthError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusUnauthorized, v2AuthErrorJSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	err := runExpectErr(t, NewEnvCmd(), "list")
	if !strings.Contains(err.Error(), "listing environments") {
		t.Errorf("expected error about listing environments, got: %v", err)
	}
	if !inngest.IsAuthError(err) {
		t.Errorf("expected an auth (401) API error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "authorization header missing") {
		t.Errorf("expected server message to surface, got: %v", err)
	}
}

func TestEnvList_ServerError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusInternalServerError, `{"errors":[{"code":"internal","message":"boom"}]}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	err := runExpectErr(t, NewEnvCmd(), "list")
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected server error message to surface, got: %v", err)
	}
	if inngest.IsAuthError(err) {
		t.Errorf("500 must not be reported as an auth error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// env get
// ---------------------------------------------------------------------------

func TestEnvGet_ByNameCaseInsensitive(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusOK, envListV2JSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got := runCapture(t, NewEnvCmd(), "get", "PRODUCTION")

	var env inngest.Environment
	if err := json.Unmarshal([]byte(got), &env); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, got)
	}
	if env.ID != "env-1" || env.Name != envProduction {
		t.Errorf("expected production env, got %+v", env)
	}
}

func TestEnvGet_ByID(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusOK, envListV2JSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got := runCapture(t, NewEnvCmd(), "get", "env-2")

	var env inngest.Environment
	if err := json.Unmarshal([]byte(got), &env); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, got)
	}
	if env.ID != "env-2" || env.Name != envStaging || env.Type != "BRANCH" {
		t.Errorf("expected staging env, got %+v", env)
	}
}

func TestEnvGet_Text(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusOK, envListV2JSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)
	state.Output = testOutputText

	got := runCapture(t, NewEnvCmd(), "get", envProduction)

	for _, want := range []string{
		"Name:          production",
		"ID:            env-1",
		"Type:          PRODUCTION",
		"Created:       2024-01-01 00:00:00",
		"Archived:      false",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected text output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestEnvGet_NotFound(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusOK, envListV2JSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	err := runExpectErr(t, NewEnvCmd(), "get", "nope")
	if !strings.Contains(err.Error(), "getting environment") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

func TestEnvGet_AuthError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/envs": v2Reply(http.StatusUnauthorized, v2AuthErrorJSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	err := runExpectErr(t, NewEnvCmd(), "get", envProduction)
	if !inngest.IsAuthError(err) {
		t.Errorf("expected auth error to propagate, got: %v", err)
	}
}

func TestEnvGet_RequiresArg(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = testOutputJSON

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"get"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when no argument is provided to env get")
	}
}
