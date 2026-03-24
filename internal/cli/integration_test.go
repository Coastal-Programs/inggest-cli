//go:build integration

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath is the path to the compiled inngest binary, set in TestMain.
var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all integration tests.
	tmpDir, err := os.MkdirTemp("", "inngest-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	binaryPath = filepath.Join(tmpDir, "inngest")

	cmd := exec.Command("go", "build", "-o", binaryPath, "github.com/Coastal-Programs/inggest-cli/cmd/inngest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build binary: %v\n", err)
		os.Exit(1)
	}

	exitCode := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(exitCode)
}

// tempConfigEnv creates a temp config file and returns its absolute path.
// The caller should use t.Setenv("INNGEST_CLI_CONFIG", path) with the result.
func tempConfigEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cli.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

// runBinary executes the inngest binary with the given args and config path.
// Returns combined stdout, combined stderr, and exit code.
func runBinary(t *testing.T, configPath string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"INNGEST_CLI_CONFIG="+configPath,
		// Clear signing/event keys to avoid interference from the host env.
		"INNGEST_SIGNING_KEY=",
		"INNGEST_EVENT_KEY=",
		"INNGEST_SIGNING_KEY_FALLBACK=",
	)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("unexpected error running binary: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func TestBinary_Version(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfgPath := tempConfigEnv(t)
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	stdout, _, exitCode := runBinary(t, cfgPath, "version")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, stdout)
	}

	for _, key := range []string{"version", "os", "arch"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected key %q in version output, got: %v", key, result)
		}
	}
}

func TestBinary_Help(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfgPath := tempConfigEnv(t)
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	stdout, _, exitCode := runBinary(t, cfgPath, "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.ToLower(stdout)
	if !strings.Contains(output, "inngest") {
		t.Errorf("expected help output to contain 'inngest', got:\n%s", stdout)
	}
	if !strings.Contains(output, "monitor") {
		t.Errorf("expected help output to contain 'monitor', got:\n%s", stdout)
	}
}

func TestBinary_AuthLoginInvalidKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfgPath := tempConfigEnv(t)
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	_, stderr, exitCode := runBinary(t, cfgPath, "auth", "login", "--signing-key", "bad-key")
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(stderr, "signkey-") {
		t.Errorf("expected stderr to mention 'signkey-', got:\n%s", stderr)
	}
}

func TestBinary_ConfigPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfgPath := tempConfigEnv(t)
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	stdout, _, exitCode := runBinary(t, cfgPath, "config", "path")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, stdout)
	}

	if _, ok := result["path"]; !ok {
		t.Errorf("expected key 'path' in config path output, got: %v", result)
	}
}

func TestBinary_DevStatusOffline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfgPath := tempConfigEnv(t)
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	// Point dev server to a port that's (almost certainly) not running anything.
	stdout, _, exitCode := runBinary(t, cfgPath, "dev", "status", "--dev-url", "http://127.0.0.1:19876")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (dev status should succeed even when offline)", exitCode)
	}

	output := strings.ToLower(stdout)
	if !strings.Contains(output, "offline") {
		t.Errorf("expected output to contain 'offline', got:\n%s", stdout)
	}
}

func TestBinary_UnknownCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfgPath := tempConfigEnv(t)
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	_, _, exitCode := runBinary(t, cfgPath, "nonexistent")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for unknown command, got 0")
	}
}

func TestBinary_OutputFormats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfgPath := tempConfigEnv(t)
	t.Setenv("INNGEST_CLI_CONFIG", cfgPath)

	// Test text format
	stdout, _, exitCode := runBinary(t, cfgPath, "version", "-o", "text")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for text format, got %d", exitCode)
	}
	if !strings.Contains(stdout, "version:") {
		t.Errorf("expected text output to contain 'version:', got:\n%s", stdout)
	}

	// Test table format
	_, _, exitCode = runBinary(t, cfgPath, "version", "-o", "table")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for table format, got %d", exitCode)
	}
}
