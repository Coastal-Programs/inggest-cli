package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

func TestNewRootCmd_HasAllSubcommands(t *testing.T) {
	cmd := newRootCmd()

	expected := []string{
		"auth", "version", "config", "dev", "events",
		"functions", "runs", "env", "health", "metrics", "backlog",
		"apps", "api",
	}

	subs := cmd.Commands()
	got := make(map[string]bool, len(subs))
	for _, c := range subs {
		got[c.Name()] = true
	}

	for _, name := range expected {
		if !got[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestNewRootCmd_GlobalFlags(t *testing.T) {
	cmd := newRootCmd()

	expectedFlags := []string{"output", "env", "api-url", "dev", "dev-url", "timeout"}
	for _, name := range expectedFlags {
		if cmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing persistent flag %q", name)
		}
	}
}

func TestNewRootCmd_DefaultOutput(t *testing.T) {
	cmd := newRootCmd()

	f := cmd.PersistentFlags().Lookup("output")
	if f == nil {
		t.Fatal("output flag not found")
	}
	if f.DefValue != "json" {
		t.Errorf("expected default output %q, got %q", "json", f.DefValue)
	}
}

func TestNewRootCmd_HelpOutput(t *testing.T) {
	// Running with no args should show help and return no error.
	// We need a valid config so PersistentPreRunE succeeds.
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cli.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	cmd := newRootCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestExecute_SetsVersion(t *testing.T) {
	// Provide a valid config so PersistentPreRunE won't fail.
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cli.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	// Reset before test
	state.AppVersion = ""

	if code := Execute("v1.2.3"); code != ExitOK {
		t.Fatalf("Execute returned exit code %d", code)
	}
	if state.AppVersion != "v1.2.3" {
		t.Errorf("expected AppVersion %q, got %q", "v1.2.3", state.AppVersion)
	}
}

func TestExitCodeFor(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want int
	}{
		{"generic error", context.Background(), errors.New("boom"), ExitError},
		{"auth error", context.Background(), &inngest.APIError{StatusCode: http.StatusUnauthorized}, ExitAuth},
		{"wrapped auth error", context.Background(), errors.Join(errors.New("listing runs"), &inngest.APIError{StatusCode: http.StatusForbidden}), ExitAuth},
		{"interrupted", cancelled, errors.New("request cancelled"), ExitCancel},
		{"context.Canceled", context.Background(), context.Canceled, ExitCancel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFor(tt.ctx, tt.err); got != tt.want {
				t.Errorf("exitCodeFor() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestExecute_BadCredentialExitsAuth guards the contract that a rejected key
// never yields exit 0 (agents and CI branch on this).
func TestExecute_BadCredentialExitsAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"code":"authorization_header_missing","message":"authorization header missing or invalid"}]}`))
	}))
	defer srv.Close()

	cfgPath := filepath.Join(t.TempDir(), "cli.json")
	if err := os.WriteFile(cfgPath, []byte(`{"signing_key":"signkey-test-abcd"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_API_KEY", "")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"env", "list", "--api-url", srv.URL})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected an error for a rejected credential")
	}
	if got := exitCodeFor(context.Background(), err); got != ExitAuth {
		t.Errorf("exit code = %d, want %d (err: %v)", got, ExitAuth, err)
	}
}

func TestNewRootCmd_PersistentPreRunE(t *testing.T) {
	// Write a config file with known values.
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cli.json")

	cfg := config.Config{
		ActiveEnv:    "staging",
		APIBaseURL:   "https://custom-api.example.com",
		DevServerURL: "http://localhost:9999",
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	// Reset state
	state.Env = ""
	state.APIBaseURL = ""
	state.DevServer = ""

	cmd := newRootCmd()
	// Execute with no args triggers PersistentPreRunE via help.
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Config values should have been resolved into state.
	if state.Env != "staging" {
		t.Errorf("expected Env %q, got %q", "staging", state.Env)
	}
	if state.APIBaseURL != "https://custom-api.example.com" {
		t.Errorf("expected APIBaseURL %q, got %q", "https://custom-api.example.com", state.APIBaseURL)
	}
	if state.DevServer != "http://localhost:9999" {
		t.Errorf("expected DevServer %q, got %q", "http://localhost:9999", state.DevServer)
	}
}

// TestNewRootCmd_FlagsOverrideConfig covers the flag-precedence branches in
// PersistentPreRunE: --env, --api-url and --dev-url must each beat the value
// resolved from the config file.
func TestNewRootCmd_FlagsOverrideConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "cli.json")
	cfg := config.Config{
		ActiveEnv:    "staging",
		APIBaseURL:   "https://config-api.example.com",
		DevServerURL: "http://localhost:9999",
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	state.Env = ""
	state.APIBaseURL = ""
	state.DevServer = ""

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"--env", "flag-env",
		"--api-url", "https://flag-api.example.com",
		"--dev-url", "http://localhost:1234",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if state.Env != "flag-env" {
		t.Errorf("--env should win over config: got %q, want %q", state.Env, "flag-env")
	}
	if state.APIBaseURL != "https://flag-api.example.com" {
		t.Errorf("--api-url should win over config: got %q", state.APIBaseURL)
	}
	if state.DevServer != "http://localhost:1234" {
		t.Errorf("--dev-url should win over config: got %q", state.DevServer)
	}
}

// TestNewRootCmd_PersistentPreRunE_ConfigLoadError covers the branch where
// config.Load() fails: a malformed config file must abort the command rather
// than run it against half-resolved state.
func TestNewRootCmd_PersistentPreRunE_ConfigLoadError(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "cli.json")
	if err := os.WriteFile(cfgPath, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when the config file is malformed")
	}
}

// TestExecute_ErrorPathReturnsExitCode covers the failure tail of Execute:
// the error is reported and mapped to a non-zero exit code.
func TestExecute_ErrorPathReturnsExitCode(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "cli.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config.ResetForTest()
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	// Execute reads the command line from os.Args.
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"inngest", "no-such-command"}

	// Silence the error report so the suite output stays clean.
	origStderr := os.Stderr
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = devNull.Close()
	})

	code := Execute("v0.0.0-test")
	if code == ExitOK {
		t.Fatal("expected a non-zero exit code for an unknown command")
	}
}
