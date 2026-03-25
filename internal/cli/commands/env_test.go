package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

const (
	testOutputTable = "table"
	testOutputText  = "text"
	testProduction  = "production"
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
	state.Output = testOutputJSON

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"use", "staging"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Config.ActiveEnv != testStaging {
		t.Errorf("expected ActiveEnv %q, got %q", testStaging, state.Config.ActiveEnv)
	}
	if state.Env != testStaging {
		t.Errorf("expected state.Env %q, got %q", testStaging, state.Env)
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
	state.Output = testOutputJSON

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
	state.Env = testProduction
	state.Output = testOutputTable

	envs := []inngest.Environment{
		{
			ID:   "env-1",
			Name: "Production",
			Slug: "production",
			Type: "production",
		},
		{
			ID:   "env-2",
			Name: "Staging",
			Slug: "staging",
			Type: "branch",
		},
	}

	err := printEnvTable(envs)
	if err != nil {
		t.Fatalf("printEnvTable returned error: %v", err)
	}
}

func TestPrintEnvDetail(t *testing.T) {
	env := &inngest.Environment{
		ID:                   "env-123",
		Name:                 "Production",
		Slug:                 "production",
		Type:                 "production",
		IsAutoArchiveEnabled: true,
	}

	err := printEnvDetail(env)
	if err != nil {
		t.Fatalf("printEnvDetail returned error: %v", err)
	}
}

const listEnvsResponse = `{"data":{"envs":{"edges":[{"node":{"id":"env-1","name":"Production","slug":"production","type":"production"}},{"node":{"id":"env-2","name":"Staging","slug":"staging","type":"branch"}}],"pageInfo":{"hasNextPage":false}}}}`

func TestEnvList_JSON(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": listEnvsResponse,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputJSON

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

	var envs []json.RawMessage
	if err := json.Unmarshal([]byte(out), &envs); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, out)
	}

	if len(envs) != 2 {
		t.Errorf("expected 2 envs, got %d", len(envs))
	}
}

func TestEnvList_Table(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": listEnvsResponse,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputTable

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

	if !strings.Contains(out, "Production") {
		t.Errorf("expected table output to contain %q, got: %s", "Production", out)
	}
	if !strings.Contains(out, "Staging") {
		t.Errorf("expected table output to contain %q, got: %s", "Staging", out)
	}
}

func TestEnvGet_ByName(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": listEnvsResponse,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputJSON

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"get", "Production"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	if !strings.Contains(out, "Production") {
		t.Errorf("expected output to contain %q, got: %s", "Production", out)
	}
}

func TestEnvGet_NotFound(t *testing.T) {
	emptyEnvs := `{"data":{"envs":{"edges":[],"pageInfo":{"hasNextPage":false}}}}`
	srv := newMockServer(t, map[string]string{
		"ListEnvs": emptyEnvs,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputJSON

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"get", "nonexistent"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when env is not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain %q, got: %v", "not found", err)
	}
}

func TestEnvGet_TextOutput(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": listEnvsResponse,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputText

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"get", "Production"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	if !strings.Contains(out, "Production") {
		t.Errorf("expected text output to contain %q, got: %s", "Production", out)
	}
	if !strings.Contains(out, "Name:") {
		t.Errorf("expected text output to contain %q, got: %s", "Name:", out)
	}
}

func TestEnvGet_ByID(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": listEnvsResponse,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputJSON

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"get", "env-1"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	if !strings.Contains(out, "Production") {
		t.Errorf("expected output to contain %q, got: %s", "Production", out)
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

func TestEnvList_AuthError_Fallback(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Config.ActiveEnv = "staging"
	state.Output = testOutputJSON

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
		t.Fatalf("expected fallback (no error), got: %v", cmdErr)
	}

	if !strings.Contains(out, "staging") {
		t.Errorf("expected fallback output to contain active env %q, got: %s", "staging", out)
	}
	if !strings.Contains(out, "local_config") {
		t.Errorf("expected fallback output to contain %q, got: %s", "local_config", out)
	}
	if !strings.Contains(out, "https://app.inngest.com/env") {
		t.Errorf("expected fallback output to contain dashboard link, got: %s", out)
	}
}

func TestEnvList_NonAuthError(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": `{"data":null,"errors":[{"message":"internal server error"}]}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListEnvironments fails with non-auth error")
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
	state.Output = testOutputJSON

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

func TestEnvGet_AuthError_Fallback(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Config.ActiveEnv = "production"
	state.Output = testOutputJSON

	var cmdErr error
	out := captureStdout(t, func() {
		cmd := NewEnvCmd()
		cmd.SetArgs([]string{"get", "production"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmdErr = cmd.Execute()
	})

	if cmdErr != nil {
		t.Fatalf("expected fallback (no error), got: %v", cmdErr)
	}

	if !strings.Contains(out, "production") {
		t.Errorf("expected fallback output to contain %q, got: %s", "production", out)
	}
	if !strings.Contains(out, "local_config") {
		t.Errorf("expected fallback output to contain %q, got: %s", "local_config", out)
	}
}

func TestEnvGet_NonAuthError(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListEnvs": `{"data":null,"errors":[{"message":"internal server error"}]}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewEnvCmd()
	cmd.SetArgs([]string{"get", "production"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GetEnvironment fails with non-auth error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error about not found, got: %v", err)
	}
}

func TestPrintEnvDetail_Simple(t *testing.T) {
	env := &inngest.Environment{
		ID:   "env-123",
		Name: "Production",
		Slug: "production",
		Type: "production",
	}

	got := captureStdout(t, func() {
		if err := printEnvDetail(env); err != nil {
			t.Fatalf("printEnvDetail returned error: %v", err)
		}
	})

	if !strings.Contains(got, "Production") {
		t.Errorf("expected output to contain 'Production', got: %s", got)
	}
	if !strings.Contains(got, "env-123") {
		t.Errorf("expected output to contain 'env-123', got: %s", got)
	}
}
