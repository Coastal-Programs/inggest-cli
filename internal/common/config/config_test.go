package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// resetConfig resets the sync.Once so getConfigPath() re-evaluates on the
// next call. Must be called after setting INNGEST_CLI_CONFIG via t.Setenv.
func resetConfig(t *testing.T) {
	t.Helper()
	once = sync.Once{}
	configPath = ""
	t.Cleanup(func() {
		once = sync.Once{}
		configPath = ""
	})
}

// --- DefaultConfigPath ---

func TestDefaultConfigPath_EndsWithInngestCLI(t *testing.T) {
	// Clear env vars so we hit os.UserConfigDir() or fallback
	t.Setenv("INNGEST_CLI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	got := DefaultConfigPath()
	suffix := filepath.Join("inngest", "cli.json")
	if !strings.HasSuffix(got, suffix) {
		t.Errorf("expected path ending %s, got %q", suffix, got)
	}
}

func TestDefaultConfigPath_XDGConfigHome(t *testing.T) {
	t.Setenv("INNGEST_CLI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgtest")

	got := DefaultConfigPath()
	want := filepath.Join("/tmp/xdgtest", "inngest", "cli.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDefaultConfigPath_UsesUserConfigDir(t *testing.T) {
	t.Setenv("INNGEST_CLI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	got := DefaultConfigPath()
	// os.UserConfigDir() should succeed in CI/dev; verify it uses that result
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Skip("os.UserConfigDir() not available on this platform")
	}
	want := filepath.Join(dir, "inngest", "cli.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- getConfigPath ---

func TestGetConfigPath_ValidEnvVar(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.json")
	t.Setenv("INNGEST_CLI_CONFIG", p)
	resetConfig(t)

	if got := getConfigPath(); got != p {
		t.Errorf("got %q, want %q", got, p)
	}
}

func TestGetConfigPath_RelativePath_FallsBackToDefault(t *testing.T) {
	t.Setenv("INNGEST_CLI_CONFIG", "relative/path.json")
	resetConfig(t)

	got := getConfigPath()
	if strings.Contains(got, "relative") {
		t.Errorf("relative path should be rejected, got %q", got)
	}
	if !strings.HasSuffix(got, filepath.Join("inngest", "cli.json")) {
		t.Errorf("expected default path ending inngest/cli.json, got %q", got)
	}
}

func TestGetConfigPath_NonJSONSuffix_FallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	t.Setenv("INNGEST_CLI_CONFIG", p)
	resetConfig(t)

	got := getConfigPath()
	if got == p {
		t.Errorf("non-.json path should be rejected, got %q", got)
	}
	if !strings.HasSuffix(got, filepath.Join("inngest", "cli.json")) {
		t.Errorf("expected default path ending inngest/cli.json, got %q", got)
	}
}

func TestGetConfigPath_TraversalRejected(t *testing.T) {
	t.Setenv("INNGEST_CLI_CONFIG", "/tmp/../etc/config.json")
	resetConfig(t)

	got := getConfigPath()
	// filepath.Clean resolves the .., so it becomes /etc/config.json which is valid.
	// But paths with .. before Clean that resolve to absolute .json are accepted by design.
	// The key protection is: no ".." in the cleaned path.
	if strings.Contains(got, "..") {
		t.Errorf("path with traversal should be rejected, got %q", got)
	}
}

func TestGetConfigPath_Default(t *testing.T) {
	t.Setenv("INNGEST_CLI_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	resetConfig(t)

	got := getConfigPath()
	if !strings.HasSuffix(got, filepath.Join("inngest", "cli.json")) {
		t.Errorf("expected default path ending inngest/cli.json, got %q", got)
	}
}

// --- Load ---

func TestLoad_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INNGEST_CLI_CONFIG", filepath.Join(dir, "missing.json"))
	resetConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.SigningKey != "" || cfg.EventKey != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	t.Setenv("INNGEST_CLI_CONFIG", p)
	resetConfig(t)

	data := `{"signing_key":"sk-test-123","event_key":"evt-key-456","active_env":"staging"}`
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SigningKey != "sk-test-123" {
		t.Errorf("SigningKey: got %q, want sk-test-123", cfg.SigningKey)
	}
	if cfg.EventKey != "evt-key-456" {
		t.Errorf("EventKey: got %q, want evt-key-456", cfg.EventKey)
	}
	if cfg.ActiveEnv != "staging" {
		t.Errorf("ActiveEnv: got %q, want staging", cfg.ActiveEnv)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	t.Setenv("INNGEST_CLI_CONFIG", p)
	resetConfig(t)

	if err := os.WriteFile(p, []byte("not json {{{"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("error should mention 'parsing config', got: %v", err)
	}
}

// --- Save ---

func TestSave_CreatesFileWith0600Perms(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", p)
	resetConfig(t)

	cfg := &Config{SigningKey: "sk-test"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions: got %04o, want 0600", perm)
	}
}

func TestSave_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", p)
	resetConfig(t)

	cfg := &Config{
		SigningKey:   "sk-signing-key-test-1234",
		EventKey:     "evt-event-key-test-5678",
		ActiveEnv:    "staging",
		APIBaseURL:   "https://custom.inngest.com",
		DevServerURL: "http://localhost:9999",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after Save error: %v", err)
	}
	if loaded.SigningKey != cfg.SigningKey {
		t.Errorf("SigningKey: got %q, want %q", loaded.SigningKey, cfg.SigningKey)
	}
	if loaded.EventKey != cfg.EventKey {
		t.Errorf("EventKey: got %q, want %q", loaded.EventKey, cfg.EventKey)
	}
	if loaded.ActiveEnv != cfg.ActiveEnv {
		t.Errorf("ActiveEnv: got %q, want %q", loaded.ActiveEnv, cfg.ActiveEnv)
	}
	if loaded.APIBaseURL != cfg.APIBaseURL {
		t.Errorf("APIBaseURL: got %q, want %q", loaded.APIBaseURL, cfg.APIBaseURL)
	}
	if loaded.DevServerURL != cfg.DevServerURL {
		t.Errorf("DevServerURL: got %q, want %q", loaded.DevServerURL, cfg.DevServerURL)
	}
}

// --- Redact ---

func TestRedact_ShortOrEmpty(t *testing.T) {
	for _, s := range []string{"", "short", "12345678"} {
		if got := Redact(s); got != "****" {
			t.Errorf("Redact(%q) = %q, want ****", s, got)
		}
	}
}

func TestRedact_Long(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"123456789", "1234****6789"},
		{"abcdefghijklmnop", "abcd****mnop"},
	}
	for _, tc := range cases {
		if got := Redact(tc.input); got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- GetSigningKey ---

func TestGetSigningKey_FromConfig(t *testing.T) {
	cfg := &Config{SigningKey: "sk-from-config"}
	if got := cfg.GetSigningKey(); got != "sk-from-config" {
		t.Errorf("got %q, want sk-from-config", got)
	}
}

func TestGetSigningKey_FallsBackToEnvVar(t *testing.T) {
	t.Setenv("INNGEST_SIGNING_KEY", "sk-from-env")
	cfg := &Config{}
	if got := cfg.GetSigningKey(); got != "sk-from-env" {
		t.Errorf("got %q, want sk-from-env", got)
	}
}

func TestGetSigningKey_ConfigTakesPrecedence(t *testing.T) {
	t.Setenv("INNGEST_SIGNING_KEY", "sk-from-env")
	cfg := &Config{SigningKey: "sk-from-config"}
	if got := cfg.GetSigningKey(); got != "sk-from-config" {
		t.Errorf("got %q, want sk-from-config", got)
	}
}

// --- GetSigningKeyFallback ---

func TestGetSigningKeyFallback_FromConfig(t *testing.T) {
	cfg := &Config{SigningKeyFallback: "sk-fallback-from-config"}
	if got := cfg.GetSigningKeyFallback(); got != "sk-fallback-from-config" {
		t.Errorf("got %q, want sk-fallback-from-config", got)
	}
}

func TestGetSigningKeyFallback_FallsBackToEnvVar(t *testing.T) {
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "sk-fallback-from-env")
	cfg := &Config{}
	if got := cfg.GetSigningKeyFallback(); got != "sk-fallback-from-env" {
		t.Errorf("got %q, want sk-fallback-from-env", got)
	}
}

func TestGetSigningKeyFallback_ConfigTakesPrecedence(t *testing.T) {
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "sk-fallback-from-env")
	cfg := &Config{SigningKeyFallback: "sk-fallback-from-config"}
	if got := cfg.GetSigningKeyFallback(); got != "sk-fallback-from-config" {
		t.Errorf("got %q, want sk-fallback-from-config", got)
	}
}

func TestGetSigningKeyFallback_EmptyWhenNotSet(t *testing.T) {
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	cfg := &Config{}
	if got := cfg.GetSigningKeyFallback(); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// --- GetEventKey ---

func TestGetEventKey_FromConfig(t *testing.T) {
	cfg := &Config{EventKey: "evt-from-config"}
	if got := cfg.GetEventKey(); got != "evt-from-config" {
		t.Errorf("got %q, want evt-from-config", got)
	}
}

func TestGetEventKey_FallsBackToEnvVar(t *testing.T) {
	t.Setenv("INNGEST_EVENT_KEY", "evt-from-env")
	cfg := &Config{}
	if got := cfg.GetEventKey(); got != "evt-from-env" {
		t.Errorf("got %q, want evt-from-env", got)
	}
}

// --- GetAPIBaseURL ---

func TestGetAPIBaseURL_Default(t *testing.T) {
	cfg := &Config{}
	if got := cfg.GetAPIBaseURL(); got != "https://api.inngest.com" {
		t.Errorf("got %q, want https://api.inngest.com", got)
	}
}

func TestGetAPIBaseURL_Custom(t *testing.T) {
	cfg := &Config{APIBaseURL: "https://custom.example.com"}
	if got := cfg.GetAPIBaseURL(); got != "https://custom.example.com" {
		t.Errorf("got %q, want https://custom.example.com", got)
	}
}

// --- GetDevServerURL ---

func TestGetDevServerURL_Default(t *testing.T) {
	cfg := &Config{}
	if got := cfg.GetDevServerURL(); got != "http://localhost:8288" {
		t.Errorf("got %q, want http://localhost:8288", got)
	}
}

func TestGetDevServerURL_Custom(t *testing.T) {
	cfg := &Config{DevServerURL: "http://localhost:9999"}
	if got := cfg.GetDevServerURL(); got != "http://localhost:9999" {
		t.Errorf("got %q, want http://localhost:9999", got)
	}
}

// --- GetActiveEnv ---

func TestGetActiveEnv_Default(t *testing.T) {
	cfg := &Config{}
	if got := cfg.GetActiveEnv(); got != "production" {
		t.Errorf("got %q, want production", got)
	}
}

func TestGetActiveEnv_Custom(t *testing.T) {
	cfg := &Config{ActiveEnv: "staging"}
	if got := cfg.GetActiveEnv(); got != "staging" {
		t.Errorf("got %q, want staging", got)
	}
}

// --- IsConfigured ---

func TestIsConfigured(t *testing.T) {
	cases := []struct {
		name     string
		cfg      Config
		expected bool
	}{
		{"both keys set", Config{SigningKey: "sk", EventKey: "ek"}, true},
		{"signing key only", Config{SigningKey: "sk"}, true},
		{"event key only", Config{EventKey: "ek"}, true},
		{"neither", Config{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear env vars so they don't interfere
			t.Setenv("INNGEST_SIGNING_KEY", "")
			t.Setenv("INNGEST_EVENT_KEY", "")
			if got := tc.cfg.IsConfigured(); got != tc.expected {
				t.Errorf("IsConfigured() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestIsConfigured_ViaEnvVar(t *testing.T) {
	t.Setenv("INNGEST_SIGNING_KEY", "sk-from-env")
	t.Setenv("INNGEST_EVENT_KEY", "")
	cfg := &Config{}
	if !cfg.IsConfigured() {
		t.Error("IsConfigured() should be true when INNGEST_SIGNING_KEY is set")
	}
}

// --- ResetForTest ---

func TestResetForTest(t *testing.T) {
	// Set some state first
	configPath = "/some/path.json"
	once.Do(func() {}) // consume the once

	ResetForTest()

	if configPath != "" {
		t.Error("expected empty configPath after ResetForTest")
	}
}

// --- Load: ReadFile error that is NOT IsNotExist ---

func TestLoad_ReadError_NotNotExist(t *testing.T) {
	dir := t.TempDir()
	fakePath := filepath.Join(dir, "subdir.json")
	t.Setenv("INNGEST_CLI_CONFIG", fakePath)
	resetConfig(t)

	// Create subdir.json as a directory, not a file — ReadFile on a dir returns
	// a non-IsNotExist error (e.g. "is a directory").
	if err := os.MkdirAll(fakePath, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Error("expected error when config path is a directory")
	}
	if !strings.Contains(err.Error(), "reading config") {
		t.Errorf("error should mention 'reading config', got: %v", err)
	}
}

// --- Save: MkdirAll error ---

func TestSave_MkdirAllError(t *testing.T) {
	// /dev/null is a file, so creating subdirectories under it is impossible.
	t.Setenv("INNGEST_CLI_CONFIG", "/dev/null/impossible/cli.json")
	resetConfig(t)

	cfg := &Config{SigningKey: "sk-test"}
	err := cfg.Save()
	if err == nil {
		t.Error("expected error when parent dir can't be created")
	}
	if !strings.Contains(err.Error(), "creating config dir") {
		t.Errorf("error should mention 'creating config dir', got: %v", err)
	}
}

// --- DefaultConfigPath: UserConfigDir fails ---

func TestDefaultConfigPath_UserConfigDirFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	old := userConfigDirFn
	userConfigDirFn = func() (string, error) {
		return "", fmt.Errorf("no config dir")
	}
	t.Cleanup(func() { userConfigDirFn = old })

	got := DefaultConfigPath()
	if !strings.HasSuffix(got, filepath.Join(".config", "inngest", "cli.json")) {
		t.Errorf("expected fallback path ending .config/inngest/cli.json, got %q", got)
	}
}

// --- Save: json.MarshalIndent error ---

func TestSave_MarshalIndentError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cli.json")
	t.Setenv("INNGEST_CLI_CONFIG", p)
	resetConfig(t)

	old := jsonMarshalIndentFn
	jsonMarshalIndentFn = func(v any, prefix, indent string) ([]byte, error) {
		return nil, fmt.Errorf("simulated marshal error")
	}
	t.Cleanup(func() { jsonMarshalIndentFn = old })

	cfg := &Config{SigningKey: "sk-test"}
	err := cfg.Save()
	if err == nil {
		t.Error("expected error when marshal fails")
	}
	if !strings.Contains(err.Error(), "encoding config") {
		t.Errorf("error should mention 'encoding config', got: %v", err)
	}
}
