package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
)

func TestNewRootCmd_HasAllSubcommands(t *testing.T) {
	cmd := newRootCmd()

	expected := []string{
		"auth", "version", "config", "dev", "events",
		"functions", "runs", "env", "health", "metrics", "backlog",
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

	expectedFlags := []string{"output", "env", "api-url", "dev", "dev-url"}
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

	if err := Execute("v1.2.3"); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if state.AppVersion != "v1.2.3" {
		t.Errorf("expected AppVersion %q, got %q", "v1.2.3", state.AppVersion)
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
