package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

const (
	testProcessPayment = "Process Payment"
	testSlug           = "test"
)

func TestFunctionsCmdHasSubcommands(t *testing.T) {
	cmd := NewFunctionsCmd()

	// Verify alias.
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "fn" {
		t.Errorf("expected alias %q, got %v", "fn", cmd.Aliases)
	}

	want := map[string]bool{
		"list":   false,
		"get":    false,
		"config": false,
	}

	for _, sub := range cmd.Commands() {
		name := sub.Name()
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("functions command missing subcommand %q", name)
		}
	}
}

func TestPrintFunctionsTable(t *testing.T) {
	functions := []inngest.Function{
		{
			Name: "Process Payment",
			Slug: "process-payment",
			Triggers: []inngest.FunctionTrigger{
				{Type: "event", Value: "payment/created"},
				{Type: "cron", Value: "0 * * * *"},
			},
			App: &inngest.App{
				Name:       "billing-app",
				ExternalID: "billing-app",
			},
		},
		{
			Name: "Send Email",
			Slug: "send-email",
			Triggers: []inngest.FunctionTrigger{
				{Type: "event", Value: "user/signup"},
			},
			App: &inngest.App{
				Name:       "notifications",
				ExternalID: "notifications",
			},
		},
	}

	if err := printFunctionsTable(functions); err != nil {
		t.Fatalf("printFunctionsTable returned error: %v", err)
	}
}

func TestPrintFunctionsTableEmpty(t *testing.T) {
	if err := printFunctionsTable([]inngest.Function{}); err != nil {
		t.Fatalf("printFunctionsTable with empty slice returned error: %v", err)
	}
}

func TestPrintFunctionDetail(t *testing.T) {
	fn := &inngest.Function{
		ID:         "fn-123",
		Name:       "Process Payment",
		Slug:       "process-payment",
		URL:        "https://example.com/api/inngest",
		IsPaused:   false,
		IsArchived: false,
		Triggers: []inngest.FunctionTrigger{
			{Type: "event", Value: "payment/created", Condition: "event.data.amount > 100"},
			{Type: "cron", Value: "0 * * * *"},
		},
		App: &inngest.App{
			ID:         "app-456",
			Name:       "billing-app",
			ExternalID: "billing-app",
			AppVersion: "2.0.0",
		},
		Configuration: &inngest.FunctionConfiguration{
			Retries: &inngest.RetryConfig{Value: 3, IsDefault: false},
			Concurrency: []inngest.ConcurrencyConfig{
				{Scope: "function", Limit: &inngest.ConcurrencyLimit{Value: 10}, Key: "event.data.userID"},
			},
			RateLimit:   &inngest.RateLimitConfig{Limit: 100, Period: "1m", Key: "event.data.userID"},
			Debounce:    &inngest.DebounceConfig{Period: "5s", Key: "event.data.id"},
			Throttle:    &inngest.ThrottleConfig{Limit: 50, Burst: 10, Period: "1h", Key: "event.data.team"},
			EventsBatch: &inngest.EventsBatchConfig{MaxSize: 100, Timeout: "30s", Key: "event.data.batch"},
			Priority:    "event.data.priority",
		},
	}

	if err := printFunctionDetail(fn); err != nil {
		t.Fatalf("printFunctionDetail returned error: %v", err)
	}
}

func TestPrintFunctionDetailMinimal(t *testing.T) {
	fn := &inngest.Function{
		Name: "Minimal Function",
		Slug: "minimal-fn",
	}

	if err := printFunctionDetail(fn); err != nil {
		t.Fatalf("printFunctionDetail with minimal data returned error: %v", err)
	}
}

func TestBuildConfigOutput(t *testing.T) {
	fn := &inngest.Function{
		Slug: testSlug,
		Name: "Test",
		Configuration: &inngest.FunctionConfiguration{
			Retries: &inngest.RetryConfig{Value: 3, IsDefault: false},
		},
	}

	result := buildConfigOutput(fn)

	for _, key := range []string{"slug", "name", "configuration"} {
		if _, ok := result[key]; !ok {
			t.Errorf("buildConfigOutput missing key %q", key)
		}
	}

	if result["slug"] != testSlug {
		t.Errorf("expected slug %q, got %v", testSlug, result["slug"])
	}
	if result["name"] != "Test" {
		t.Errorf("expected name %q, got %v", "Test", result["name"])
	}
}

func TestBuildConfigOutputNoConfig(t *testing.T) {
	fn := &inngest.Function{
		Slug: "bare",
		Name: "Bare",
	}

	result := buildConfigOutput(fn)

	if _, ok := result["slug"]; !ok {
		t.Error("buildConfigOutput missing key \"slug\"")
	}
	if _, ok := result["name"]; !ok {
		t.Error("buildConfigOutput missing key \"name\"")
	}
	if _, ok := result["configuration"]; ok {
		t.Error("buildConfigOutput should not have \"configuration\" key when Configuration is nil")
	}
}

// ---------------------------------------------------------------------------
// Integration tests using mock GraphQL server
// ---------------------------------------------------------------------------

// Both ListFunctions and GetFunction use the "ListFunctions" operation name,
// since GetFunction calls ListFunctions internally.
const listFunctionsResponse = `{"data":{"events":{"data":[{"workflows":[{"id":"fn-1","name":"Process Payment","slug":"process-payment","isPaused":false,"isArchived":false,"triggers":[{"type":"event","value":"payment/created"}],"app":{"id":"app-1","name":"billing-app","externalID":"billing-app"}},{"id":"fn-2","name":"Send Email","slug":"send-email","isPaused":false,"isArchived":false,"triggers":[{"type":"event","value":"user/signup"}],"app":{"id":"app-1","name":"billing-app","externalID":"billing-app"}}]}],"page":{"page":1,"totalPages":1}}}}`

const getFunctionResponse = `{"data":{"events":{"data":[{"workflows":[{"id":"fn-1","name":"Process Payment","slug":"process-payment","url":"https://example.com/api/inngest","isPaused":false,"isArchived":false,"triggers":[{"type":"event","value":"payment/created"}],"configuration":{"retries":{"value":3,"isDefault":false}},"app":{"id":"app-1","name":"billing-app","externalID":"billing-app","appVersion":"1.0.0"}}]}],"page":{"page":1,"totalPages":1}}}}`

const getFunctionWithConfigResponse = `{"data":{"events":{"data":[{"workflows":[{"id":"fn-1","name":"Process Payment","slug":"process-payment","url":"https://example.com/api/inngest","isPaused":false,"isArchived":false,"triggers":[{"type":"event","value":"payment/created"}],"configuration":{"retries":{"value":3,"isDefault":false}},"app":{"id":"app-1","name":"billing-app","externalID":"billing-app","appVersion":"1.0.0"}}]}],"page":{"page":1,"totalPages":1}}}}`

// setupFunctionsTestState configures global state for cloud-mode tests.
func setupFunctionsTestState(t *testing.T, srvURL string) {
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
	state.AppVersion = testAppVersion
}

func TestFunctionsList_Success(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": listFunctionsResponse,
	}, nil)
	defer srv.Close()

	setupFunctionsTestState(t, srv.URL)

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var functions []inngest.Function
	if err := json.Unmarshal([]byte(got), &functions); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if len(functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functions))
	}
	if functions[0].Name != testProcessPayment {
		t.Errorf("expected first function name %q, got %q", testProcessPayment, functions[0].Name)
	}
	if functions[1].Name != "Send Email" {
		t.Errorf("expected second function name %q, got %q", "Send Email", functions[1].Name)
	}
}

func TestFunctionsList_AppFilter(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": listFunctionsResponse,
	}, nil)
	defer srv.Close()

	// Both functions belong to "billing-app", so --app billing-app should return 2.
	t.Run("matching app", func(t *testing.T) {
		setupFunctionsTestState(t, srv.URL)

		cmd := NewFunctionsCmd()
		cmd.SetArgs([]string{"list", "--app", "billing-app"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		got := captureStdout(t, func() {
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		var functions []inngest.Function
		if err := json.Unmarshal([]byte(got), &functions); err != nil {
			t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
		}

		if len(functions) != 2 {
			t.Fatalf("expected 2 functions for billing-app, got %d", len(functions))
		}
	})

	// --app nonexistent should return 0 functions (empty array).
	t.Run("nonexistent app", func(t *testing.T) {
		setupFunctionsTestState(t, srv.URL)

		cmd := NewFunctionsCmd()
		cmd.SetArgs([]string{"list", "--app", "nonexistent"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		got := captureStdout(t, func() {
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		// JSON output for an empty slice can be "[]" or "null".
		got = strings.TrimSpace(got)
		if got != "[]" && got != "null" {
			// Try parsing to be sure.
			var functions []inngest.Function
			if err := json.Unmarshal([]byte(got), &functions); err != nil {
				t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
			}
			if len(functions) != 0 {
				t.Fatalf("expected 0 functions for nonexistent app, got %d", len(functions))
			}
		}
	})
}

func TestFunctionsList_Table(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": listFunctionsResponse,
	}, nil)
	defer srv.Close()

	setupFunctionsTestState(t, srv.URL)
	state.Output = testOutputTable

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "Process Payment") {
		t.Errorf("expected table output to contain %q, got: %s", "Process Payment", got)
	}
	if !strings.Contains(got, "Send Email") {
		t.Errorf("expected table output to contain %q, got: %s", "Send Email", got)
	}
}

func TestFunctionsGet_Success(t *testing.T) {
	// GetFunction calls ListFunctions internally, so use same operation name.
	srv := newMockServer(t, map[string]string{
		"ListFunctions": getFunctionResponse,
	}, nil)
	defer srv.Close()

	setupFunctionsTestState(t, srv.URL)

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"get", "process-payment"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var fn inngest.Function
	if err := json.Unmarshal([]byte(got), &fn); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if fn.Name != testProcessPayment {
		t.Errorf("expected function name %q, got %q", testProcessPayment, fn.Name)
	}
	if fn.Slug != "process-payment" {
		t.Errorf("expected function slug %q, got %q", "process-payment", fn.Slug)
	}
}

func TestFunctionsConfig_JSON(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": getFunctionWithConfigResponse,
	}, nil)
	defer srv.Close()

	setupFunctionsTestState(t, srv.URL)

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"config", "process-payment"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	for _, key := range []string{"slug", "name", "configuration"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected key %q in config JSON output", key)
		}
	}

	if result["slug"] != "process-payment" {
		t.Errorf("expected slug %q, got %v", "process-payment", result["slug"])
	}
	if result["name"] != testProcessPayment {
		t.Errorf("expected name %q, got %v", testProcessPayment, result["name"])
	}
}

func TestFunctionsConfig_Text(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": getFunctionWithConfigResponse,
	}, nil)
	defer srv.Close()

	setupFunctionsTestState(t, srv.URL)
	state.Output = testOutputText

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"config", "process-payment"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "Configuration for") {
		t.Errorf("expected output to contain %q, got: %s", "Configuration for", got)
	}
	if !strings.Contains(got, "Retries") {
		t.Errorf("expected output to contain %q, got: %s", "Retries", got)
	}
}

func TestFunctionsCmd_BareHelp(t *testing.T) {
	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error from bare functions command: %v", err)
	}
}

func TestFunctionsGet_TextOutput(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": getFunctionResponse,
	}, nil)
	defer srv.Close()

	setupFunctionsTestState(t, srv.URL)
	state.Output = testOutputText

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"get", "process-payment"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "Process Payment") {
		t.Errorf("expected text output to contain %q, got: %s", "Process Payment", got)
	}
	if !strings.Contains(got, "Name:") {
		t.Errorf("expected text output to contain %q, got: %s", "Name:", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for uncovered error and branch paths
// ---------------------------------------------------------------------------

func TestFunctionsList_Error(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": `{"data":null,"errors":[{"message":"unauthorized"}]}`,
	}, nil)
	defer srv.Close()
	setupFunctionsTestState(t, srv.URL)

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListFunctions fails")
	}
	if !strings.Contains(err.Error(), "listing functions") {
		t.Errorf("expected error about listing functions, got: %v", err)
	}
}

func TestFunctionsGet_Error(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": `{"data":null,"errors":[{"message":"not found"}]}`,
	}, nil)
	defer srv.Close()
	setupFunctionsTestState(t, srv.URL)

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"get", "nonexistent-func"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GetFunction fails")
	}
	if !strings.Contains(err.Error(), "getting function") {
		t.Errorf("expected error about getting function, got: %v", err)
	}
}

func TestPrintConfiguration_RetriesDefault(t *testing.T) {
	cfg := &inngest.FunctionConfiguration{
		Retries: &inngest.RetryConfig{Value: 4, IsDefault: true},
	}

	got := captureStdout(t, func() {
		printConfiguration(cfg)
	})

	if !strings.Contains(got, "(default)") {
		t.Errorf("expected output to contain '(default)', got: %s", got)
	}
	if !strings.Contains(got, "Retries") {
		t.Errorf("expected output to contain 'Retries', got: %s", got)
	}
}

func TestFunctionsConfig_Error(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListFunctions": `{"data":null,"errors":[{"message":"not found"}]}`,
	}, nil)
	defer srv.Close()
	setupFunctionsTestState(t, srv.URL)

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"config", "nonexistent-func"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GetFunction fails for config")
	}
	if !strings.Contains(err.Error(), "getting function config") {
		t.Errorf("expected error about getting function config, got: %v", err)
	}
}

func TestFunctionsConfig_TextNoConfig(t *testing.T) {
	// Return a function without configuration to test the "No configuration" branch.
	resp := `{"data":{"events":{"data":[{"workflows":[{"id":"fn-1","name":"Test","slug":"test-fn","isPaused":false,"isArchived":false,"triggers":[{"type":"event","value":"test/event"}],"app":{"id":"app-1","name":"test-app","externalID":"test-app"}}]}],"page":{"page":1,"totalPages":1}}}}`
	srv := newMockServer(t, map[string]string{
		"ListFunctions": resp,
	}, nil)
	defer srv.Close()
	setupFunctionsTestState(t, srv.URL)
	state.Output = testOutputText

	cmd := NewFunctionsCmd()
	cmd.SetArgs([]string{"config", "test-fn"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "No configuration") {
		t.Errorf("expected output to contain 'No configuration', got: %s", got)
	}
}
