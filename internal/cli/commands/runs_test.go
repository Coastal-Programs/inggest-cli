package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

const (
	v2RunJSON = `{"id":"01RUN1","status":"COMPLETED","app":{"id":"app-1","name":"my-app"},
		"function":{"id":"fn-1","name":"My Fn","slug":"my-fn","app":{"id":"app-1"}},
		"trigger":{"eventName":"user.signup","eventIds":["01EV"],"isBatch":false},
		"queuedAt":"2024-01-01T00:00:00Z","startedAt":"2024-01-01T00:00:01Z","endedAt":"2024-01-01T00:00:02Z",
		"durationMs":"1000","output":{"ok":true}}`
	v2RunsPage  = `{"data":[` + v2RunJSON + `],"page":{"cursor":"next-1","hasMore":true,"limit":20}}`
	v2TraceJSON = `{"data":{"runId":"01RUN1","rootSpan":{"id":"s1","name":"my-fn","status":"COMPLETED","stepOp":"RUN","durationMs":"12",
		"children":[{"id":"s2","name":"step-a","status":"COMPLETED","stepOp":"RUN","durationMs":"5"}]}}}`
)

func runRuns(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRunsCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	return out, err
}

func TestRunsCmdHasSubcommands(t *testing.T) {
	cmd := NewRunsCmd()
	for _, name := range []string{"list", "get", "trace", "cancel", "replay", "watch"} {
		if sub, _, err := cmd.Find([]string{name}); err != nil || sub == nil || sub.Name() != name {
			t.Errorf("expected subcommand %q", name)
		}
	}
}

func TestRunsList_QueryAndOutput(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if got := q["status"]; len(got) != 2 || got[0] != "FAILED" || got[1] != "CANCELLED" {
				t.Errorf("status = %v", got)
			}
			if q.Get("limit") != "5" || q.Get("cursor") != "c1" || q.Get("order") != "ASC" || q.Get("timeField") != "startedAt" {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			if q["functionId"][0] != testFnID || q["appId"][0] != "app-1" || q.Get("includeOutput") != queryTrue {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			if q.Get("from") == "" || q.Get("until") == "" {
				t.Errorf("expected from/until, got %s", r.URL.RawQuery)
			}
			jsonOK(v2RunsPage)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runRuns(t, "list", "--status", "failed,cancelled", "--limit", "5", "--after", "c1",
		"--order", "asc", "--time-field", "startedAt", "--function", "fn-1", "--app", "app-1",
		"--since", "2h", "--until", "2024-06-01T00:00:00Z", "--output-data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var page struct {
		Runs []map[string]any `json:"runs"`
		Page map[string]any   `json:"page"`
	}
	if err := json.Unmarshal([]byte(got), &page); err != nil {
		t.Fatalf("parse output: %v\n%s", err, got)
	}
	if len(page.Runs) != 1 || page.Runs[0]["functionID"] != "fn-1" || page.Runs[0]["eventName"] != "user.signup" {
		t.Errorf("runs = %v", page.Runs)
	}
	if page.Page["cursor"] != "next-1" || page.Page["hasMore"] != true {
		t.Errorf("page = %v", page.Page)
	}
}

func TestRunsList_Validation(t *testing.T) {
	setupCloudState(t, "http://127.0.0.1:1")
	for _, tc := range []struct{ args, want string }{
		{"--limit 0", "--limit must be between"},
		{"--limit 101", "--limit must be between"},
		{"--since bogus", "invalid --since"},
		{"--until bogus", "invalid --until"},
	} {
		_, err := runRuns(t, append([]string{"list"}, strings.Fields(tc.args)...)...)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.args, err, tc.want)
		}
	}
}

func TestRunsList_TableAndAuthError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{"/v2/runs": jsonOK(v2RunsPage)})
	defer srv.Close()
	setupCloudState(t, srv.URL)
	state.Output = testOutputTable
	got, err := runRuns(t, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"ID", "STATUS", "FUNCTION", "DURATION", "01RUN1", "My Fn", "1s"} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing %q:\n%s", want, got)
		}
	}

	bad := newMockServer(t, nil, map[string]http.HandlerFunc{"/v2/runs": jsonStatus(http.StatusUnauthorized, v2Unauthorized)})
	defer bad.Close()
	setupCloudState(t, bad.URL)
	if _, err := runRuns(t, "list"); !inngest.IsAuthError(err) {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestRunsGet_WithTrace(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs/01RUN1": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("includeOutput") != queryTrue {
				t.Errorf("expected includeOutput=true, got %s", r.URL.RawQuery)
			}
			jsonOK(`{"data":`+v2RunJSON+`}`)(w, r)
		},
		"/v2/runs/01RUN1/trace": jsonOK(v2TraceJSON),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runRuns(t, "get", "01RUN1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var run map[string]any
	if err := json.Unmarshal([]byte(got), &run); err != nil {
		t.Fatalf("parse: %v\n%s", err, got)
	}
	trace, _ := run["trace"].(map[string]any)
	if trace["id"] != "s1" || len(trace["children"].([]any)) != 1 {
		t.Errorf("trace = %v", run["trace"])
	}
	if out, _ := run["output"].(map[string]any); out["ok"] != true {
		t.Errorf("output = %v", run["output"])
	}

	state.Output = testOutputText
	got, err = runRuns(t, "get", "01RUN1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Run ID:", "01RUN1", "Status:", "COMPLETED", "Function:", "Duration:", "1s", "Output:", "Trace:", "step-a"} {
		if !strings.Contains(got, want) {
			t.Errorf("text output missing %q:\n%s", want, got)
		}
	}
}

func TestRunsGet_TraceMissingAndNoTrace(t *testing.T) {
	traceCalls := 0
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs/01RUN1": jsonOK(`{"data":{"id":"01RUN1","status":"QUEUED"}}`),
		"/v2/runs/01RUN1/trace": func(w http.ResponseWriter, r *http.Request) {
			traceCalls++
			jsonStatus(http.StatusNotFound, `{"errors":[{"code":"not_found","message":"no trace"}]}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runRuns(t, "get", "01RUN1")
	if err != nil || strings.Contains(got, `"trace"`) {
		t.Errorf("404 trace should be tolerated: err=%v out=%s", err, got)
	}
	if _, err := runRuns(t, "get", "01RUN1", "--no-trace"); err != nil {
		t.Errorf("--no-trace: %v", err)
	}
	if traceCalls != 1 {
		t.Errorf("trace calls = %d, want 1 (--no-trace must skip it)", traceCalls)
	}
}

func TestRunsGet_Wait(t *testing.T) {
	calls := 0
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs/01RUN1": func(w http.ResponseWriter, r *http.Request) {
			calls++
			status := "RUNNING"
			if calls > 1 {
				status = "COMPLETED"
			}
			jsonOK(`{"data":{"id":"01RUN1","status":"`+status+`"}}`)(w, r)
		},
		"/v2/runs/01RUN1/trace": jsonStatus(http.StatusNotFound, `{"errors":[]}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runRuns(t, "get", "01RUN1", "--wait")
	if err != nil || !strings.Contains(got, `"COMPLETED"`) || calls < 2 {
		t.Errorf("--wait: err=%v calls=%d out=%s", err, calls, got)
	}
}

func TestWaitForRun_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	client := inngest.NewClient(inngest.ClientOptions{APIBaseURL: "http://127.0.0.1:1", SigningKey: "k"})
	_, err := waitForRun(ctx, client, &inngest.FunctionRun{ID: "x", Status: "RUNNING"})
	if err == nil || !strings.Contains(err.Error(), "waiting for run") {
		t.Errorf("err = %v", err)
	}
}

func TestRunsTrace(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{"/v2/runs/01RUN1/trace": jsonOK(v2TraceJSON)})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runRuns(t, "trace", "01RUN1")
	if err != nil || !strings.Contains(got, `"s2"`) {
		t.Errorf("json trace: err=%v out=%s", err, got)
	}
	state.Output = testOutputText
	got, err = runRuns(t, "trace", "01RUN1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "my-fn") || !strings.Contains(got, "step-a") || !strings.Contains(got, "12ms") {
		t.Errorf("text trace:\n%s", got)
	}

	empty := newMockServer(t, nil, map[string]http.HandlerFunc{"/v2/runs/01RUN1/trace": jsonOK(`{"data":{"runId":"01RUN1"}}`)})
	defer empty.Close()
	setupCloudState(t, empty.URL)
	if _, err := runRuns(t, "trace", "01RUN1"); err == nil || !strings.Contains(err.Error(), "no trace yet") {
		t.Errorf("empty trace err = %v", err)
	}
}

func TestRunsCancel(t *testing.T) {
	cancelled := false
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs/01RUN1/cancel": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s", r.Method)
			}
			cancelled = true
			jsonOK(`{"data":{"runId":"01RUN1"}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	orig := isInteractiveFn
	isInteractiveFn = func() bool { return false }
	t.Cleanup(func() { isInteractiveFn = orig })

	got, err := runRuns(t, "cancel", "01RUN1", "--force")
	if err != nil || !cancelled || !strings.Contains(got, `"CANCELLED"`) {
		t.Errorf("cancel --force: err=%v cancelled=%v out=%s", err, cancelled, got)
	}
}

func TestRunsReplay(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs/01RUN1/rerun": jsonOK(`{"data":{"runId":"01NEW"}}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runRuns(t, "replay", "01RUN1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("parse: %v\n%s", err, got)
	}
	if result["originalRunID"] != "01RUN1" || result["newRunID"] != "01NEW" {
		t.Errorf("result = %v", result)
	}
}

func TestRunsWatch_PrintsEachRunOnceUntilCancelled(t *testing.T) {
	calls := 0
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Query().Get("order") != "ASC" {
				t.Errorf("watch must poll ascending, got %s", r.URL.RawQuery)
			}
			jsonOK(`{"data":[`+v2RunJSON+`],"page":{"hasMore":false}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "100ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	var err error
	got := captureStdout(t, func() { err = cmd.ExecuteContext(ctx) })
	if err != nil {
		t.Fatalf("watch should exit cleanly on cancel, got %v", err)
	}
	if calls < 2 {
		t.Errorf("expected repeated polling, got %d calls", calls)
	}
	if n := strings.Count(got, "01RUN1"); n != 1 {
		t.Errorf("run printed %d times, want exactly once:\n%s", n, got)
	}
}

func TestRunHelpers(t *testing.T) {
	if got := splitCSV(" a, ,b,"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("splitCSV = %v", got)
	}
	if got := splitCSV(""); got != nil {
		t.Errorf("splitCSV(\"\") = %v, want nil", got)
	}

	if ts, err := parseSince("since", "2024-01-02T03:04:05Z"); err != nil || ts.Year() != 2024 {
		t.Errorf("parseSince RFC3339 = (%v, %v)", ts, err)
	}
	if ts, err := parseSince("since", "1h"); err != nil || time.Since(ts) < 59*time.Minute {
		t.Errorf("parseSince duration = (%v, %v)", ts, err)
	}
	if _, err := parseSince("until", "nope"); err == nil || !strings.Contains(err.Error(), "--until") {
		t.Errorf("parseSince invalid = %v", err)
	}

	start := time.Now().Add(-3 * time.Second)
	end := start.Add(1500 * time.Millisecond)
	cases := []struct {
		name string
		run  inngest.FunctionRun
		want string
	}{
		{"durationMs wins", inngest.FunctionRun{DurationMs: 2500, StartedAt: &start, EndedAt: &end}, "2.5s"},
		{"ended-started", inngest.FunctionRun{StartedAt: &start, EndedAt: &end}, "1.5s"},
		{"none", inngest.FunctionRun{}, ""},
	}
	for _, tc := range cases {
		if got := runDuration(tc.run); got != tc.want {
			t.Errorf("%s: runDuration = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := runDuration(inngest.FunctionRun{StartedAt: &start}); !strings.HasSuffix(got, "…") {
		t.Errorf("running duration = %q, want trailing ellipsis", got)
	}
	if got := runFunctionName(inngest.FunctionRun{FunctionID: "fn-1"}); got != "fn-1" {
		t.Errorf("runFunctionName fallback = %q", got)
	}
	if got := runFunctionName(inngest.FunctionRun{FunctionID: "fn-1", Function: &inngest.Function{Name: "N"}}); got != "N" {
		t.Errorf("runFunctionName = %q", got)
	}
}
