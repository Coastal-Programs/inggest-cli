package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
)

func TestAuthCmdHasSubcommands(t *testing.T) {
	cmd := NewAuthCmd()

	want := map[string]bool{
		"login":  false,
		"logout": false,
		"status": false,
	}

	for _, sub := range cmd.Commands() {
		name := sub.Name()
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("auth command missing subcommand %q", name)
		}
	}
}

func TestAuthLoginInvalidKey(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"login", "--signing-key", "bad-key"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid signing key")
	}
	if !strings.Contains(err.Error(), "signkey-") {
		t.Errorf("expected error about signkey- prefix, got: %v", err)
	}
}

func TestAuthLoginNoKey(t *testing.T) {
	state.Config = &config.Config{}
	state.Output = "json"

	// Clear env var so the command can't pick it up from environment.
	t.Setenv("INNGEST_SIGNING_KEY", "")

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"login"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Running in tests is non-interactive (stdin is not a terminal),
	// so the command should error asking for a signing key.
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no signing key is provided in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "signing key required") {
		t.Errorf("expected error about signing key required, got: %v", err)
	}
}

func TestAuthLoginWithSigningKey(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"login", "--signing-key", "signkey-test-abc123"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Config.SigningKey != "signkey-test-abc123" {
		t.Errorf("expected signing key %q, got %q", "signkey-test-abc123", state.Config.SigningKey)
	}

	// Verify config was persisted to disk.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	if !strings.Contains(string(data), "signkey-test-abc123") {
		t.Errorf("saved config does not contain signing key, got: %s", data)
	}
}

func TestAuthLoginWithEventKey(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"login", "--signing-key", "signkey-test-abc123", "--event-key", "evt-key-xyz"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Config.SigningKey != "signkey-test-abc123" {
		t.Errorf("expected signing key %q, got %q", "signkey-test-abc123", state.Config.SigningKey)
	}
	if state.Config.EventKey != "evt-key-xyz" {
		t.Errorf("expected event key %q, got %q", "evt-key-xyz", state.Config.EventKey)
	}
}

func TestAuthLogout(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	state.Config = &config.Config{
		SigningKey: "signkey-test-abc123",
		EventKey:   "evt-key-xyz",
	}
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"logout"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Config.SigningKey != "" {
		t.Errorf("expected signing key to be cleared, got %q", state.Config.SigningKey)
	}
	if state.Config.EventKey != "" {
		t.Errorf("expected event key to be cleared, got %q", state.Config.EventKey)
	}
}

func TestAuthLoginKeyFromEnv(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	t.Setenv("INNGEST_SIGNING_KEY", "signkey-from-env-999")

	state.Config = &config.Config{}
	state.Output = "json"

	cmd := NewAuthCmd()
	// No --signing-key flag; the command should fall back to the env var.
	cmd.SetArgs([]string{"login"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Config.SigningKey != "signkey-from-env-999" {
		t.Errorf("expected signing key %q from env, got %q", "signkey-from-env-999", state.Config.SigningKey)
	}
}

// newAuthCheckMockServer creates a mock HTTP server that handles the AuthCheck GraphQL query.
// If success is true, it returns a valid response; otherwise it returns an error response.
func newAuthCheckMockServer(success bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var req struct {
			OperationName string `json:"operationName"`
		}
		json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		if req.OperationName == "AuthCheck" {
			if success {
				w.Write([]byte(`{"data":{"functions":[{"id":"fn-1","name":"test-fn"}]}}`))
			} else {
				w.Write([]byte(`{"data":null,"errors":[{"message":"unauthorized"}]}`))
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAuthStatus_WithSigningKey(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	srv := newAuthCheckMockServer(true)
	defer srv.Close()

	state.Config = &config.Config{SigningKey: "signkey-test-abc123"}
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if auth, ok := result["authenticated"].(bool); !ok || !auth {
		t.Errorf("expected authenticated=true, got %v", result["authenticated"])
	}
	if v, ok := result["api_validation"].(string); !ok || v != "ok" {
		t.Errorf("expected api_validation=%q, got %v", "ok", result["api_validation"])
	}
}

func TestAuthStatus_NotConfigured(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{}
	state.APIBaseURL = ""
	state.DevServer = ""
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if auth, ok := result["authenticated"].(bool); !ok || auth {
		t.Errorf("expected authenticated=false, got %v", result["authenticated"])
	}
}

func TestAuthStatus_APIValidationFailed(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	srv := newAuthCheckMockServer(false)
	defer srv.Close()

	state.Config = &config.Config{SigningKey: "signkey-test-abc123"}
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if v, ok := result["api_validation"].(string); !ok || v != "failed" {
		t.Errorf("expected api_validation=%q, got %v", "failed", result["api_validation"])
	}
}

func TestAuthStatus_SigningKeyFromEnv(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "signkey-test-env123")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	srv := newAuthCheckMockServer(true)
	defer srv.Close()

	state.Config = &config.Config{} // No signing key in config
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if v, ok := result["signing_key_source"].(string); !ok || v != "env (INNGEST_SIGNING_KEY)" {
		t.Errorf("expected signing_key_source=%q, got %v", "env (INNGEST_SIGNING_KEY)", result["signing_key_source"])
	}
}

func TestAuthStatus_SigningKeyFromBoth(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "signkey-test-envboth")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	srv := newAuthCheckMockServer(true)
	defer srv.Close()

	state.Config = &config.Config{SigningKey: "signkey-test-cfgboth"}
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if v, ok := result["signing_key_source"].(string); !ok || v != "config (env var also set)" {
		t.Errorf("expected signing_key_source=%q, got %v", "config (env var also set)", result["signing_key_source"])
	}
}

func TestValidateSigningKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "empty string", key: "", wantErr: true},
		{name: "cloud format test", key: "signkey-test-abc", wantErr: false},
		{name: "cloud format prod", key: "signkey-prod-xyz", wantErr: false},
		{name: "valid hex even length", key: "abcdef0123456789", wantErr: false},
		{name: "odd length hex", key: "abc", wantErr: true},
		{name: "invalid hex even length", key: "not-hex-string!!", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSigningKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSigningKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestAuthStatus_FallbackFromConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	srv := newAuthCheckMockServer(true)
	defer srv.Close()

	state.Config = &config.Config{
		SigningKey:         "signkey-test-primary",
		SigningKeyFallback: "signkey-test-fallback",
	}
	state.APIBaseURL = srv.URL
	state.DevServer = srv.URL
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if v, ok := result["signing_key_fallback"].(string); !ok || v == "not configured" {
		t.Errorf("expected signing_key_fallback to not be %q, got %v", "not configured", result["signing_key_fallback"])
	}
}

func TestAuthStatus_EventKeyFromEnv(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "evt-key-from-env")

	state.Config = &config.Config{} // No event key in config
	state.APIBaseURL = ""
	state.DevServer = ""
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if v, ok := result["event_key_source"].(string); !ok || v != "env (INNGEST_EVENT_KEY)" {
		t.Errorf("expected event_key_source=%q, got %v", "env (INNGEST_EVENT_KEY)", result["event_key_source"])
	}
}

func TestAuthStatus_CustomAPIURL(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)
	config.ResetForTest()

	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{
		APIBaseURL: "https://custom.inngest.example.com",
	}
	state.APIBaseURL = ""
	state.DevServer = ""
	state.Env = ""
	state.Output = "json"

	cmd := NewAuthCmd()
	cmd.SetArgs([]string{"status"})
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

	if v, ok := result["custom_api_url"].(bool); !ok || !v {
		t.Errorf("expected custom_api_url=true, got %v", result["custom_api_url"])
	}
}
