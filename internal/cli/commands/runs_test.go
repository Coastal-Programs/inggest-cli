package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

const testRunID1 = "run-1"

func TestRunsCmdHasSubcommands(t *testing.T) {
	cmd := NewRunsCmd()

	want := map[string]bool{
		"list":   false,
		"get":    false,
		"cancel": false,
		"replay": false,
		"watch":  false,
	}

	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("runs command missing subcommand %q", name)
		}
	}
}

func TestRunsFromEdges(t *testing.T) {
	edges := []inngest.RunEdge{
		{
			Node:   inngest.FunctionRun{ID: "run-1", Status: "COMPLETED"},
			Cursor: "c1",
		},
		{
			Node:   inngest.FunctionRun{ID: "run-2", Status: "FAILED"},
			Cursor: "c2",
		},
	}

	runs := runsFromEdges(edges)
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	if runs[0].ID != testRunID1 {
		t.Errorf("expected run ID %q, got %q", testRunID1, runs[0].ID)
	}
	if runs[0].Status != "COMPLETED" {
		t.Errorf("expected status %q, got %q", "COMPLETED", runs[0].Status)
	}
	if runs[1].ID != "run-2" {
		t.Errorf("expected run ID %q, got %q", "run-2", runs[1].ID)
	}
	if runs[1].Status != "FAILED" {
		t.Errorf("expected status %q, got %q", "FAILED", runs[1].Status)
	}
}

func TestRunsFromEdgesEmpty(t *testing.T) {
	runs := runsFromEdges([]inngest.RunEdge{})
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs))
	}
}

func TestPrintRunsTable(t *testing.T) {
	now := time.Now()
	started := now.Add(-5 * time.Second)
	ended := now
	queued := now.Add(-10 * time.Second)

	conn := &inngest.RunsConnection{
		Edges: []inngest.RunEdge{
			{
				Node: inngest.FunctionRun{
					ID:        "run-1",
					Status:    "COMPLETED",
					EventName: "app/user.created",
					Function:  &inngest.Function{Name: "Handle User Created"},
					StartedAt: &started,
					EndedAt:   &ended,
				},
				Cursor: "c1",
			},
			{
				Node: inngest.FunctionRun{
					ID:        "run-2",
					Status:    "RUNNING",
					EventName: "app/order.placed",
					Function:  &inngest.Function{Name: "Process Order"},
					StartedAt: &started,
				},
				Cursor: "c2",
			},
			{
				Node: inngest.FunctionRun{
					ID:        "run-3",
					Status:    "QUEUED",
					EventName: "app/email.send",
					QueuedAt:  &queued,
				},
				Cursor: "c3",
			},
		},
		TotalCount: 3,
	}

	if err := printRunsTable(conn); err != nil {
		t.Fatalf("printRunsTable returned error: %v", err)
	}
}

func TestPrintRunsTableEmpty(t *testing.T) {
	conn := &inngest.RunsConnection{
		Edges: []inngest.RunEdge{},
	}

	if err := printRunsTable(conn); err != nil {
		t.Fatalf("printRunsTable returned error: %v", err)
	}
}

func TestPrintRunDetail(t *testing.T) {
	now := time.Now()
	queued := now.Add(-10 * time.Second)
	started := now.Add(-5 * time.Second)
	ended := now

	run := &inngest.FunctionRun{
		ID:           "run-detail-1",
		Status:       "COMPLETED",
		EventName:    "app/user.created",
		IsBatch:      true,
		CronSchedule: "*/5 * * * *",
		Output:       `{"ok": true}`,
		TraceID:      "trace-abc-123",
		QueuedAt:     &queued,
		StartedAt:    &started,
		EndedAt:      &ended,
		Function: &inngest.Function{
			Name: "Handle User Created",
			Slug: "handle-user-created",
		},
		App: &inngest.App{
			Name:        "My App",
			SDKLanguage: "typescript",
			SDKVersion:  "3.0.0",
		},
		Trace: &inngest.RunTraceSpan{
			Name:     "handle-user-created",
			Status:   "COMPLETED",
			Duration: 5000,
			Children: []inngest.RunTraceSpan{
				{
					Name:     "validate-input",
					Status:   "COMPLETED",
					Duration: 100,
					StepOp:   "run",
				},
				{
					Name:     "send-welcome-email",
					Status:   "COMPLETED",
					Duration: 4800,
					StepOp:   "run",
				},
			},
		},
	}

	if err := printRunDetail(run); err != nil {
		t.Fatalf("printRunDetail returned error: %v", err)
	}
}

func TestPrintRunDetailMinimal(t *testing.T) {
	run := &inngest.FunctionRun{
		ID:     "run-minimal-1",
		Status: "QUEUED",
	}

	if err := printRunDetail(run); err != nil {
		t.Fatalf("printRunDetail returned error: %v", err)
	}
}

func TestPrintTraceSpan(t *testing.T) {
	span := &inngest.RunTraceSpan{
		Name:     "step1",
		Status:   "COMPLETED",
		Duration: 150,
		StepOp:   "run",
		Children: []inngest.RunTraceSpan{
			{
				Name:     "child-step",
				Status:   "COMPLETED",
				Duration: 50,
				StepOp:   "sleep",
			},
		},
	}

	// Verify no panic.
	printTraceSpan(span, "  ")
}

// ---------- integration tests using newMockServer ----------

func TestRunsList_Success(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"run-1","status":"COMPLETED","queuedAt":"2024-01-01T00:00:00Z","startedAt":"2024-01-01T00:00:01Z","endedAt":"2024-01-01T00:00:02Z","eventName":"test/event","function":{"name":"My Func","slug":"my-func"}},"cursor":"c1"},{"node":{"id":"run-2","status":"RUNNING","startedAt":"2024-01-01T00:00:01Z","eventName":"other/event","function":{"name":"Other Func","slug":"other-func"}},"cursor":"c2"}],"pageInfo":{"hasNextPage":false,"endCursor":"c2"},"totalCount":2}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"list"})
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

	runs, ok := result["runs"].([]any)
	if !ok {
		t.Fatalf("expected 'runs' to be an array, got %T", result["runs"])
	}

	ids := make(map[string]bool)
	for _, r := range runs {
		rm, _ := r.(map[string]any)
		if id, ok := rm["id"]; ok {
			ids[id.(string)] = true
		}
	}

	if !ids[testRunID1] {
		t.Error("expected output to contain run-1")
	}
	if !ids["run-2"] {
		t.Error("expected output to contain run-2")
	}
}

func TestRunsList_Table(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"run-1","status":"COMPLETED","queuedAt":"2024-01-01T00:00:00Z","startedAt":"2024-01-01T00:00:01Z","endedAt":"2024-01-01T00:00:02Z","eventName":"test/event","function":{"name":"My Func","slug":"my-func"}},"cursor":"c1"},{"node":{"id":"run-2","status":"RUNNING","startedAt":"2024-01-01T00:00:01Z","eventName":"other/event","function":{"name":"Other Func","slug":"other-func"}},"cursor":"c2"}],"pageInfo":{"hasNextPage":false,"endCursor":"c2"},"totalCount":2}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = "table"

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, testRunID1) {
		t.Errorf("expected table output to contain %q, got: %s", testRunID1, got)
	}
	if !strings.Contains(got, "My Func") {
		t.Errorf("expected table output to contain %q, got: %s", "My Func", got)
	}
}

func TestRunsGet_Success(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"GetRun": `{"data":{"run":{"id":"run-1","status":"COMPLETED","queuedAt":"2024-01-01T00:00:00Z","startedAt":"2024-01-01T00:00:01Z","endedAt":"2024-01-01T00:00:02Z","eventName":"test/event","output":"{\"result\":\"ok\"}","traceID":"trace-abc","function":{"name":"My Func","slug":"my-func"},"app":{"name":"My App","sdkLanguage":"go","sdkVersion":"0.1.0"},"trace":{"runID":"run-1","spanID":"span-1","name":"my-func","status":"COMPLETED","durationMS":1000,"childrenSpans":[{"spanID":"span-2","name":"step1","status":"COMPLETED","durationMS":500,"stepOp":"run"}]}}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"get", "run-1"})
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

	if id, ok := result["id"]; !ok || id.(string) != testRunID1 {
		t.Errorf("expected run ID %q, got %v", testRunID1, result["id"])
	}
	if status, ok := result["status"]; !ok || status.(string) != "COMPLETED" {
		t.Errorf("expected status %q, got %v", "COMPLETED", result["status"])
	}
}

func TestRunsCancel_Force(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"CancelRun": `{"data":{"cancelRun":{"id":"run-1","status":"CANCELLED"}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"cancel", "run-1", "--force"})
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

	if id, ok := result["id"]; !ok || id.(string) != testRunID1 {
		t.Errorf("expected id %q, got %v", testRunID1, result["id"])
	}
	if status, ok := result["status"]; !ok || status.(string) != "CANCELLED" {
		t.Errorf("expected status %q, got %v", "CANCELLED", result["status"])
	}
}

func TestRunsReplay_Success(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"Rerun": `{"data":{"rerun":"new-run-id"}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"replay", "run-1"})
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

	if v, ok := result["originalRunID"]; !ok || v.(string) != testRunID1 {
		t.Errorf("expected originalRunID %q, got %v", testRunID1, result["originalRunID"])
	}
	if v, ok := result["newRunID"]; !ok || v.(string) != "new-run-id" {
		t.Errorf("expected newRunID %q, got %v", "new-run-id", result["newRunID"])
	}
}

func TestRunsList_WithStatusFilter(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"run-1","status":"COMPLETED","eventName":"test/event","function":{"name":"My Func","slug":"my-func"}},"cursor":"c1"}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"},"totalCount":1}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"list", "--status", "Completed,Failed", "--function", "fn-id-1", "--since", "1h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, testRunID1) {
		t.Errorf("expected output to contain testRunID1, got: %s", got)
	}
}

func TestRunsList_WithUntilFlag(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false},"totalCount":0}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"list", "--until", "1h"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "totalCount") {
		t.Errorf("expected output to contain totalCount, got: %s", got)
	}
}

func TestRunsList_InvalidSince(t *testing.T) {
	setupCloudState(t, "http://localhost:9999")

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"list", "--since", "notaduration"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --since duration")
	}
	if !strings.Contains(err.Error(), "invalid --since duration") {
		t.Errorf("expected error about invalid duration, got: %v", err)
	}
}

func TestRunsList_InvalidUntil(t *testing.T) {
	setupCloudState(t, "http://localhost:9999")

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"list", "--until", "notaduration"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --until duration")
	}
	if !strings.Contains(err.Error(), "invalid --until duration") {
		t.Errorf("expected error about invalid until duration, got: %v", err)
	}
}

func TestRunsWatch_ContextCancel(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"run-1","status":"COMPLETED","queuedAt":"2024-01-01T00:00:00Z","startedAt":"2024-01-01T00:00:01Z","endedAt":"2024-01-01T00:00:02Z","eventName":"test/event","function":{"name":"My Func","slug":"my-func"}},"cursor":"c1"}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"},"totalCount":1}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "10ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	// The watch command uses signal.NotifyContext(context.Background(), os.Interrupt).
	// We send ourselves SIGINT after a short delay.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	// Give the watch command time to do at least one poll, then send SIGINT.
	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch command didn't stop after SIGINT")
	}
}

func TestRunsWatch_WithFilters(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false},"totalCount":0}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "10ms", "--status", "Completed,Failed", "--function", "fn-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch command didn't stop after SIGINT")
	}
}

func TestRunsWatch_ErrorContinues(t *testing.T) {
	// Server closes immediately → ListRuns will fail, but the watch loop should log and continue.
	srv := newMockServer(t, nil, nil)
	closedURL := srv.URL
	srv.Close()

	setupCloudState(t, closedURL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "10ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	// Let it poll and hit errors for a bit, then stop.
	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch command didn't stop after SIGINT")
	}
}

func TestRunsCancel_NonForce_Yes(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"CancelRun": `{"data":{"cancelRun":{"id":"run-1","status":"CANCELLED"}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	// Pipe "y\n" to stdin to simulate confirmation.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte("y\n"))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"cancel", "run-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, got)
	}

	if result["id"] != testRunID1 {
		t.Errorf("expected id %q, got %v", testRunID1, result["id"])
	}
	if result["status"] != "CANCELLED" {
		t.Errorf("expected status %q, got %v", "CANCELLED", result["status"])
	}
}

func TestRunsCancel_NonForce_No(t *testing.T) {
	setupCloudState(t, "http://localhost:9999")

	// Pipe "n\n" to stdin to decline.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte("n\n"))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"cancel", "run-1"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	// Should succeed (return nil) — cancellation was declined.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunsReplay_Text(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"Rerun": `{"data":{"rerun":"new-run-id"}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputText

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"replay", "run-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "new-run-id") {
		t.Errorf("expected output to contain new-run-id, got: %s", got)
	}
}

func TestRunsCmd_BareHelp(t *testing.T) {
	// Calling the parent command with no subcommand should print help (not error).
	cmd := NewRunsCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error from bare runs command: %v", err)
	}
}

func TestRunsGet_Text(t *testing.T) {
	srv := newMockServer(t, map[string]string{
		"GetRun": `{"data":{"run":{"id":"run-1","status":"COMPLETED","queuedAt":"2024-01-01T00:00:00Z","startedAt":"2024-01-01T00:00:01Z","endedAt":"2024-01-01T00:00:02Z","eventName":"test/event","output":"{\"result\":\"ok\"}","traceID":"trace-abc","function":{"name":"My Func","slug":"my-func"},"app":{"name":"My App","sdkLanguage":"go","sdkVersion":"0.1.0"},"trace":{"runID":"run-1","spanID":"span-1","name":"my-func","status":"COMPLETED","durationMS":1000,"childrenSpans":[{"spanID":"span-2","name":"step1","status":"COMPLETED","durationMS":500,"stepOp":"run"}]}}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)
	state.Output = testOutputText

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"get", "run-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	got := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, testRunID1) {
		t.Errorf("expected text output to contain testRunID1, got: %s", got)
	}
	if !strings.Contains(got, "COMPLETED") {
		t.Errorf("expected text output to contain COMPLETED, got: %s", got)
	}
}

// ---------- error-path tests ----------

func TestRunsList_ListRunsError(t *testing.T) {
	// No "ListRuns" key → mock returns 400 → client returns error.
	srv := newMockServer(t, map[string]string{}, nil)
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when ListRuns fails")
	}
	if !strings.Contains(err.Error(), "listing runs") {
		t.Errorf("expected error about listing runs, got: %v", err)
	}
}

func TestRunsGet_Error(t *testing.T) {
	// No "GetRun" key → mock returns 400 → client returns error.
	srv := newMockServer(t, map[string]string{}, nil)
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"get", "run-nonexistent"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when GetRun fails")
	}
	if !strings.Contains(err.Error(), "getting run") {
		t.Errorf("expected error about getting run, got: %v", err)
	}
}

func TestRunsCancel_Error(t *testing.T) {
	// No "CancelRun" key → mock returns 400 → client returns error.
	srv := newMockServer(t, map[string]string{}, nil)
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"cancel", "run-nonexistent", "--force"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when CancelRun fails")
	}
	if !strings.Contains(err.Error(), "cancelling run") {
		t.Errorf("expected error about cancelling run, got: %v", err)
	}
}

func TestRunsReplay_Error(t *testing.T) {
	// No "Rerun" key → mock returns 400 → client returns error.
	srv := newMockServer(t, map[string]string{}, nil)
	defer srv.Close()
	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"replay", "run-nonexistent"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when RerunRun fails")
	}
	if !strings.Contains(err.Error(), "replaying run") {
		t.Errorf("expected error about replaying run, got: %v", err)
	}
}

func TestRunsWatch_CtxDonePath(t *testing.T) {
	// Use a very long interval so the ticker never fires before SIGINT.
	// This ensures the select picks ctx.Done() instead of ticker.C.
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false},"totalCount":0}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "1h"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	// Give the goroutine time to start and enter the select, then SIGINT.
	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch command didn't stop after SIGINT")
	}
}

func TestRunsWatch_QueuedAtFallback(t *testing.T) {
	// Return runs with queuedAt set but startedAt absent (null) to cover the else-if branch.
	srv := newMockServer(t, map[string]string{
		"ListRuns": `{"data":{"runs":{"edges":[{"node":{"id":"run-q1","status":"QUEUED","queuedAt":"2024-01-01T12:00:00Z","eventName":"test/event","function":{"name":"My Func","slug":"my-func"}},"cursor":"c1"}],"pageInfo":{"hasNextPage":false,"endCursor":"c1"},"totalCount":1}}}`,
	}, nil)
	defer srv.Close()

	setupCloudState(t, srv.URL)

	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "10ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	// Give the watch command time to do at least one poll, then send SIGINT.
	time.Sleep(50 * time.Millisecond)
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch command didn't stop after SIGINT")
	}
}
