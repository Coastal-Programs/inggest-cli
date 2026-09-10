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

// ---------------------------------------------------------------------------
// Error branches and text-detail formatting
// ---------------------------------------------------------------------------

const (
	runsPath      = "/v2/runs/01RUN1"
	runsTracePath = "/v2/runs/01RUN1/trace"
)

func TestRunsCmd_BareHelp(t *testing.T) {
	cmd := NewRunsCmd()
	cmd.SetArgs([]string{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error from bare runs command: %v", err)
	}
	if !strings.Contains(buf.String(), "Available Commands") {
		t.Errorf("expected help text, got:\n%s", buf.String())
	}
}

func TestRunsGet_ErrorPaths(t *testing.T) {
	tests := []struct {
		name   string
		routes map[string]http.HandlerFunc
		args   []string
		want   string
	}{
		{
			name:   "get run fails",
			routes: map[string]http.HandlerFunc{runsPath: jsonStatus(http.StatusUnauthorized, v2Unauthorized)},
			args:   []string{"get", "01RUN1"},
			want:   "getting run:",
		},
		{
			name: "trace fetch fails with non-404",
			routes: map[string]http.HandlerFunc{
				runsPath:      jsonOK(`{"data":{"id":"01RUN1","status":"COMPLETED"}}`),
				runsTracePath: jsonStatus(http.StatusInternalServerError, v2ServerError),
			},
			args: []string{"get", "01RUN1"},
			want: "getting run trace:",
		},
		{
			name: "wait polls and the poll fails",
			routes: map[string]http.HandlerFunc{
				runsPath: func() http.HandlerFunc {
					calls := 0
					return func(w http.ResponseWriter, r *http.Request) {
						calls++
						if calls == 1 {
							jsonOK(`{"data":{"id":"01RUN1","status":"RUNNING"}}`)(w, r)
							return
						}
						jsonStatus(http.StatusInternalServerError, v2ServerError)(w, r)
					}
				}(),
			},
			args: []string{"get", "01RUN1", "--wait"},
			want: "waiting for run:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newMockServer(t, nil, tc.routes)
			defer srv.Close()
			setupCloudState(t, srv.URL)

			_, err := runRuns(t, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestRunsTrace_FetchError(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		runsTracePath: jsonStatus(http.StatusUnauthorized, v2Unauthorized),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	_, err := runRuns(t, "trace", "01RUN1")
	if err == nil || !strings.Contains(err.Error(), "getting run trace:") {
		t.Errorf("err = %v, want containing %q", err, "getting run trace:")
	}
	if !inngest.IsAuthError(err) {
		t.Errorf("expected auth error to be preserved for the exit code, got %v", err)
	}
}

// A run with an app that reports an SDK, plus a trace span timed by
// startedAt/endedAt rather than durationMs.
func TestRunsGet_TextDetailWithAppSDK(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		runsPath: jsonOK(`{"data":{"id":"01RUN1","status":"COMPLETED",
			"app":{"id":"app-1","name":"billing-app","sdkLanguage":"typescript","sdkVersion":"3.16.1"},
			"function":{"id":"fn-1","name":"My Fn","slug":"my-fn"},
			"trigger":{"eventName":"user.signup","cronSchedule":"*/5 * * * *"}}}`),
		runsTracePath: jsonOK(`{"data":{"runId":"01RUN1","rootSpan":{"id":"s1","name":"my-fn","status":"COMPLETED",
			"startedAt":"2024-01-01T00:00:00Z","endedAt":"2024-01-01T00:00:02.500Z"}}}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)
	state.Output = testOutputText

	got, err := runRuns(t, "get", "01RUN1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"App:", "billing-app", "SDK:", "typescript/3.16.1", "Cron:", "*/5 * * * *", "2.5s"} {
		if !strings.Contains(got, want) {
			t.Errorf("text detail missing %q:\n%s", want, got)
		}
	}
}

// A run with no app object, no timestamps, a batch trigger, an unnamed function
// and a plain-string output exercises every fallback in printRunDetail.
func TestRunsGet_TextDetailFallbacks(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		runsPath: jsonOK(`{"data":{"id":"01RUN1","status":"QUEUED",
			"function":{"name":"Unnamed","app":{"id":"app-9"}},
			"trigger":{"eventName":"user.signup","eventIds":["01EV","02EV"],"isBatch":true},
			"output":"plain text result"}}`),
		runsTracePath: jsonStatus(http.StatusNotFound, `{"errors":[{"code":"not_found","message":"no trace"}]}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)
	state.Output = testOutputText

	got, err := runRuns(t, "get", "01RUN1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"App ID:", "app-9", "Batch:", "yes", "Event IDs:", "01EV, 02EV", "Output:", "plain text result", "Unnamed ()"} {
		if !strings.Contains(got, want) {
			t.Errorf("text detail missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"Queued:", "Started:", "Ended:", "Trace:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("text detail should omit %q:\n%s", unwanted, got)
		}
	}
}

func TestRunsCancel_Prompt(t *testing.T) {
	tests := []struct {
		name        string
		stdin       string
		wantCancel  bool
		wantStderr  string
		wantInOut   string
		wantNoOutIn string
	}{
		{name: "y confirms", stdin: "y\n", wantCancel: true, wantStderr: "Cancel run 01RUN1?", wantInOut: `"CANCELLED"`},
		{name: "yes confirms", stdin: "yes\n", wantCancel: true, wantStderr: "Cancel run 01RUN1?", wantInOut: `"CANCELLED"`},
		{name: "n aborts", stdin: "n\n", wantCancel: false, wantStderr: "Aborted.", wantNoOutIn: `"CANCELLED"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cancelled := false
			srv := newMockServer(t, nil, map[string]http.HandlerFunc{
				"/v2/runs/01RUN1/cancel": func(w http.ResponseWriter, r *http.Request) {
					cancelled = true
					jsonOK(`{"data":{"runId":"01RUN1"}}`)(w, r)
				},
			})
			defer srv.Close()
			setupCloudState(t, srv.URL)
			setStdin(t, tc.stdin)

			var got string
			stderr := captureStderr(t, func() {
				var err error
				got, err = runRuns(t, "cancel", "01RUN1")
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			})

			if cancelled != tc.wantCancel {
				t.Errorf("cancel request sent = %v, want %v", cancelled, tc.wantCancel)
			}
			if !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr = %q, want containing %q", stderr, tc.wantStderr)
			}
			if tc.wantInOut != "" && !strings.Contains(got, tc.wantInOut) {
				t.Errorf("stdout = %q, want containing %q", got, tc.wantInOut)
			}
			if tc.wantNoOutIn != "" && strings.Contains(got, tc.wantNoOutIn) {
				t.Errorf("stdout = %q, should not contain %q", got, tc.wantNoOutIn)
			}
		})
	}
}

func TestRunsCancelAndReplay_Errors(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs/01RUN1/cancel": jsonStatus(http.StatusInternalServerError, v2ServerError),
		"/v2/runs/01RUN1/rerun":  jsonStatus(http.StatusInternalServerError, v2ServerError),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	for _, tc := range []struct{ args, want string }{
		{"cancel 01RUN1 --force", "cancelling run:"},
		{"replay 01RUN1", "replaying run:"},
	} {
		_, err := runRuns(t, strings.Fields(tc.args)...)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want containing %q", tc.args, err, tc.want)
		}
	}
}

// A failing poll is reported on stderr and watch keeps going until cancelled.
func TestRunsWatch_PollErrorContinues(t *testing.T) {
	calls := 0
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			calls++
			jsonStatus(http.StatusInternalServerError, v2ServerError)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "50ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	var err error
	stderr := captureStderr(t, func() { err = cmd.ExecuteContext(ctx) })
	if err != nil {
		t.Fatalf("watch should exit cleanly on cancel, got %v", err)
	}
	if calls < 2 {
		t.Errorf("watch should keep polling after an error, got %d calls", calls)
	}
	if !strings.Contains(stderr, "Error polling runs:") {
		t.Errorf("stderr = %q, want containing %q", stderr, "Error polling runs:")
	}
	// "Stopped." is deliberately not asserted here: it races. When the context
	// expires while a poll is in flight, runs.go returns silently via the
	// ctx.Err() check instead of printing it. TestRunsWatch_StoppedOnCancel
	// below covers that line deterministically.
}

// Cancelling between polls prints the "Stopped." banner. The server blocks
// until the test cancels, so the ctx.Done() arm of the select always wins.
func TestRunsWatch_StoppedOnCancel(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": jsonStatus(http.StatusOK, `{"data":[],"metadata":{"fetchedAt":"2024-01-01T00:00:00Z","cachedUntil":null}}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := NewRunsCmd()
	// A long interval guarantees no poll fires before the cancel lands, so the
	// ctx.Done() branch is the one that runs.
	cmd.SetArgs([]string{"watch", "--interval", "1h"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	var err error
	stderr := captureStderr(t, func() {
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = cmd.ExecuteContext(ctx)
		}()
		cancel()
		<-done
	})

	if err != nil {
		t.Fatalf("watch should exit cleanly on cancel, got %v", err)
	}
	if !strings.Contains(stderr, "Stopped.") {
		t.Errorf("stderr = %q, want containing %q", stderr, "Stopped.")
	}
}

// When the context expires mid-request watch exits silently instead of
// reporting the cancellation as a polling error.
func TestRunsWatch_ContextCancelledDuringPoll(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "20ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	var err error
	stderr := captureStderr(t, func() { err = cmd.ExecuteContext(ctx) })
	if err != nil {
		t.Fatalf("watch should exit cleanly when the context is cancelled, got %v", err)
	}
	if strings.Contains(stderr, "Error polling runs:") {
		t.Errorf("cancellation must not be reported as a polling error:\n%s", stderr)
	}
}

// A run queued after the watch started advances the cursor for the next poll.
func TestRunsWatch_AdvancesCursorFromQueuedAt(t *testing.T) {
	queued := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	var froms []string
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			froms = append(froms, r.URL.Query().Get("from"))
			jsonOK(`{"data":[{"id":"01RUN1","status":"RUNNING","function":{"id":"fn-1","name":"My Fn"},
				"trigger":{"eventName":"user.signup"},"queuedAt":"`+queued+`"}],"page":{"hasMore":false}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	cmd := NewRunsCmd()
	cmd.SetArgs([]string{"watch", "--interval", "50ms"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	var err error
	got := captureStdout(t, func() { err = cmd.ExecuteContext(ctx) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(froms) < 2 {
		t.Fatalf("expected at least 2 polls, got %d", len(froms))
	}
	if froms[1] == froms[0] {
		t.Errorf("second poll should start from the newest queuedAt, got %q both times", froms[0])
	}
	if !strings.Contains(got, "01RUN1") || !strings.Contains(got, "My Fn") {
		t.Errorf("watch output = %q", got)
	}
}

func TestRunStartedLabel(t *testing.T) {
	started := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	queued := time.Date(2024, 1, 1, 9, 15, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  inngest.FunctionRun
		want string
	}{
		{"startedAt wins", inngest.FunctionRun{StartedAt: &started, QueuedAt: &queued}, started.Local().Format("15:04:05")},
		{"queuedAt fallback", inngest.FunctionRun{QueuedAt: &queued}, queued.Local().Format("15:04:05")},
		{"neither", inngest.FunctionRun{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := runStartedLabel(tc.run); got != tc.want {
				t.Errorf("runStartedLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunDetailHelpers(t *testing.T) {
	if got := formatLocalTime(nil); got != "" {
		t.Errorf("formatLocalTime(nil) = %q, want empty", got)
	}
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := formatLocalTime(&ts); got != ts.Local().Format(time.RFC3339) {
		t.Errorf("formatLocalTime = %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty(\"\", \"\") = %q, want empty", got)
	}
	if got := firstNonEmpty("", "b"); got != "b" {
		t.Errorf("firstNonEmpty = %q, want %q", got, "b")
	}
	if got := formatJSONInline(json.RawMessage(`"plain text"`)); got != "plain text" {
		t.Errorf("formatJSONInline string = %q", got)
	}
	if got := formatJSONInline(json.RawMessage(`{"ok":true}`)); got != `{"ok":true}` {
		t.Errorf("formatJSONInline object = %q", got)
	}
}
