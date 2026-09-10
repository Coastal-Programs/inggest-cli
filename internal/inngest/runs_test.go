package inngest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testV2Run = `{
  "id": "` + testRunID1 + `",
  "status": "COMPLETED",
  "app": {"id": "` + testAppID1 + `", "name": "` + testMyApp + `"},
  "function": {"id": "` + testFnID1 + `", "name": "` + testSendEmail + `", "slug": "` + testSlugSend + `", "app": {"id": "` + testAppID1 + `"}},
  "trigger": {"eventName": "` + testEventName + `", "eventIds": ["` + testEventID1 + `"], "cronSchedule": "", "isBatch": false},
  "queuedAt": "` + testTimeQueued + `",
  "startedAt": "` + testTimeStart + `",
  "endedAt": "` + testTimeEnd + `",
  "durationMs": "1000",
  "output": {"ok": true}
}`

func TestListRunsOptions_Query(t *testing.T) {
	until := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	opts := ListRunsOptions{
		First:         50,
		After:         testCursor2,
		Status:        []string{"failed", "Cancelled"},
		FunctionIDs:   []string{testFnID1, testFnID2},
		AppIDs:        []string{testAppID1},
		From:          time.Date(2026, 9, 1, 0, 0, 0, 0, time.FixedZone("plus8", 8*3600)),
		Until:         &until,
		TimeField:     "startedAt",
		Order:         "asc",
		IncludeOutput: true,
	}
	q := opts.query()
	want := map[string][]string{
		"limit":         {"50"},
		"cursor":        {testCursor2},
		"status":        {"FAILED", "CANCELLED"},
		"functionId":    {testFnID1, testFnID2},
		"appId":         {testAppID1},
		"from":          {"2026-08-31T16:00:00Z"},
		"until":         {"2026-09-02T00:00:00Z"},
		"timeField":     {"startedAt"},
		"order":         {"ASC"},
		"includeOutput": {"true"},
	}
	for key, vals := range want {
		got := q[key]
		if len(got) != len(vals) {
			t.Errorf("%s = %v, want %v", key, got, vals)
			continue
		}
		for i := range vals {
			if got[i] != vals[i] {
				t.Errorf("%s[%d] = %q, want %q", key, i, got[i], vals[i])
			}
		}
	}
	if empty := (ListRunsOptions{}).query(); len(empty) != 0 {
		t.Errorf("zero options produced query %v", empty)
	}
}

func TestListRuns_Cloud(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"GET " + testPathV2Runs: func(t *testing.T, r *http.Request) string {
			requireQuery(t, r, map[string][]string{"limit": {"100"}, "status": {"FAILED"}})
			return `{"data": [` + testV2Run + `], "page": {"cursor": "` + testCursor2 + `", "hasMore": true, "limit": 100}}`
		},
	})

	page, err := newCloudClient(srv).ListRuns(context.Background(), ListRunsOptions{First: 500, Status: []string{"failed"}})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if !page.Page.HasMore || page.Page.Cursor != testCursor2 {
		t.Errorf("page = %+v, want hasMore with cursor %s", page.Page, testCursor2)
	}
	if len(page.Runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(page.Runs))
	}
	run := page.Runs[0]
	checks := map[string][2]string{
		"ID":         {run.ID, testRunID1},
		"Status":     {run.Status, testStatusDone},
		"FunctionID": {run.FunctionID, testFnID1},
		"AppID":      {run.AppID, testAppID1},
		"EventName":  {run.EventName, testEventName},
		"App.Name":   {run.App.Name, testMyApp},
		"Fn.Slug":    {run.Function.Slug, testSlugSend},
		"Output":     {string(run.Output), `{"ok": true}`},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if run.DurationMs != 1000 {
		t.Errorf("DurationMs = %d, want 1000", run.DurationMs)
	}
	if len(run.EventIDs) != 1 || run.EventIDs[0] != testEventID1 {
		t.Errorf("EventIDs = %v", run.EventIDs)
	}
	if run.StartedAt == nil || run.EndedAt.Sub(*run.StartedAt) != time.Second {
		t.Errorf("timestamps not decoded: started=%v ended=%v", run.StartedAt, run.EndedAt)
	}
}

func TestListRuns_CloudAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, testUnauthorized)
	}))
	defer srv.Close()

	_, err := newCloudClient(srv).ListRuns(context.Background(), ListRunsOptions{})
	if !IsAuthError(err) {
		t.Fatalf("want auth error, got %v", err)
	}
}

func TestV2Run_ToFunctionRun_Fallbacks(t *testing.T) {
	var r v2Run
	raw := `{"id":"x","status":"RUNNING","function":{"id":"f","app":{"id":"from-fn"}},"output":null}`
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	run := r.toFunctionRun()
	if run.AppID != "from-fn" {
		t.Errorf("AppID = %q, want app id from function.app", run.AppID)
	}
	if run.Output != nil {
		t.Errorf("Output = %q, want nil for null", run.Output)
	}
	if run.App != nil || run.EventName != "" {
		t.Errorf("unexpected app/trigger data: %+v", run)
	}
}

func TestNormalizeJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"null", "null", ""},
		{"object", `{"a":1}`, `{"a":1}`},
		{"encoded json string", `"{\"a\":1}"`, `{"a":1}`},
		{"plain string stays", `"hello"`, `"hello"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(normalizeJSON(json.RawMessage(tt.in))); got != tt.want {
				t.Errorf("normalizeJSON(%s) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestGetRun_Cloud(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"GET /v2/runs/" + testRunID1: func(t *testing.T, r *http.Request) string {
			requireQuery(t, r, map[string][]string{"includeOutput": {"true"}})
			return `{"data": ` + testV2Run + `}`
		},
	})
	run, err := newCloudClient(srv).GetRun(context.Background(), testRunID1)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.ID != testRunID1 || run.Function.Name != testSendEmail {
		t.Errorf("run = %+v", run)
	}
}

func TestGetRun_CloudNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, testNotFoundResp)
	}))
	defer srv.Close()
	if _, err := newCloudClient(srv).GetRun(context.Background(), "missing"); !IsNotFound(err) {
		t.Fatalf("want not-found error, got %v", err)
	}
}

func TestGetRun_PathEscapesID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/v2/runs/a%2Fb" {
			t.Errorf("escaped path = %s", r.URL.EscapedPath())
		}
		writeJSON(w, http.StatusOK, `{"data": {"id": "a/b", "status": "QUEUED"}}`)
	}))
	defer srv.Close()
	if _, err := newCloudClient(srv).GetRun(context.Background(), "a/b"); err != nil {
		t.Fatal(err)
	}
}

func TestGetRunTrace_Cloud(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"GET /v2/runs/" + testRunID1 + "/trace": staticRoute(`{"data": {"runId": "` + testRunID1 + `", "rootSpan": {
			"id": "s1", "name": "fn", "status": "COMPLETED", "stepOp": "RUN", "durationMs": "12",
			"children": [{"id": "s2", "name": "step", "status": "COMPLETED", "stepOp": "SLEEP", "durationMs": 5}]}}}`),
	})
	trace, err := newCloudClient(srv).GetRunTrace(context.Background(), testRunID1)
	if err != nil {
		t.Fatalf("GetRunTrace: %v", err)
	}
	if trace.ID != "s1" || trace.DurationMs != 12 || len(trace.Children) != 1 {
		t.Fatalf("trace = %+v", trace)
	}
	if child := trace.Children[0]; child.StepOp != "SLEEP" || child.DurationMs != 5 {
		t.Errorf("child = %+v", child)
	}
}

func TestCancelAndRerun_Cloud(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"POST /v2/runs/" + testRunID1 + "/cancel": func(t *testing.T, r *http.Request) string {
			if body := decodeJSONBody(t, r); len(body) != 0 {
				t.Errorf("cancel body = %v, want {}", body)
			}
			return `{"data": {"runId": ""}}`
		},
		"POST /v2/runs/" + testRunID1 + "/rerun": staticRoute(`{"data": {"runId": "` + testRunID2 + `"}}`),
	})
	client := newCloudClient(srv)

	id, err := client.CancelRun(context.Background(), testRunID1)
	if err != nil || id != testRunID1 {
		t.Errorf("CancelRun = (%q, %v), want input id echoed", id, err)
	}
	newID, err := client.RerunRun(context.Background(), testRunID1)
	if err != nil || newID != testRunID2 {
		t.Errorf("RerunRun = (%q, %v), want %s", newID, err, testRunID2)
	}
}

func TestIsTerminalRunStatus(t *testing.T) {
	for status, want := range map[string]bool{
		"COMPLETED": true, "failed": true, "Cancelled": true,
		"RUNNING": false, "QUEUED": false, "": false,
	} {
		if got := IsTerminalRunStatus(status); got != want {
			t.Errorf("IsTerminalRunStatus(%q) = %v, want %v", status, got, want)
		}
	}
}

// --- dev server (GraphQL) paths ---

const testDevRunsResp = `{"data":{"runs":{"edges":[
  {"node":{"id":"` + testRunID1 + `","functionID":"` + testFnID1 + `","status":"COMPLETED","eventName":"` + testEventName + `","eventIDs":["` + testEventID1 + `"],"queuedAt":"` + testTimeQueued + `","function":{"id":"` + testFnID1 + `","name":"` + testSendEmail + `","slug":"` + testSlugSend + `"},"app":{"id":"` + testAppID1 + `","name":"` + testMyApp + `"}}},
  {"node":{"id":"` + testRunID2 + `","functionID":"` + testFnID1 + `","status":"FAILED"}}
],"pageInfo":{"hasNextPage":true,"endCursor":"` + testCursor2 + `"}}}}`

func TestListRuns_Dev(t *testing.T) {
	srv, rec := newDevGQLServer(t, map[string]string{"DevRuns": testDevRunsResp})
	until := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	page, err := newDevClient(srv).ListRuns(context.Background(), ListRunsOptions{
		Status: []string{"failed"}, FunctionIDs: []string{testFnID1}, AppIDs: []string{testAppID1},
		Until: &until, TimeField: "endedAt", Order: "asc", After: testCursor2,
	})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	// Client-side status filter drops the COMPLETED run (dev server ignores filter.status).
	if len(page.Runs) != 1 || page.Runs[0].ID != testRunID2 {
		t.Fatalf("runs = %+v, want only the FAILED run", page.Runs)
	}
	if !page.Page.HasMore || page.Page.Cursor != testCursor2 || page.Page.Limit != 20 {
		t.Errorf("page = %+v", page.Page)
	}

	vars := rec.last(t).Variables
	if vars["first"] != float64(20) || vars["after"] != testCursor2 {
		t.Errorf("first/after = %v/%v", vars["first"], vars["after"])
	}
	const wantField = "ENDED_AT"
	orderBy := vars["orderBy"].([]any)[0].(map[string]any)
	if orderBy["field"] != wantField || orderBy["direction"] != "ASC" {
		t.Errorf("orderBy = %v", orderBy)
	}
	filter := vars["filter"].(map[string]any)
	if filter["from"] != "1970-01-01T00:00:00Z" || filter["until"] != "2026-09-02T00:00:00Z" || filter["timeField"] != wantField {
		t.Errorf("filter times = %v", filter)
	}
	if got := filter["status"].([]any); len(got) != 1 || got[0] != "FAILED" {
		t.Errorf("filter.status = %v", got)
	}
	if got := filter["functionIDs"].([]any); len(got) != 1 || got[0] != testFnID1 {
		t.Errorf("filter.functionIDs = %v", got)
	}
	if got := filter["appIDs"].([]any); len(got) != 1 || got[0] != testAppID1 {
		t.Errorf("filter.appIDs = %v", got)
	}
}

func TestListRuns_DevDefaultsAndError(t *testing.T) {
	srv, rec := newDevGQLServer(t, map[string]string{"DevRuns": testDevRunsResp})
	page, err := newDevClient(srv).ListRuns(context.Background(), ListRunsOptions{})
	if err != nil || len(page.Runs) != 2 {
		t.Fatalf("ListRuns = (%d runs, %v)", len(page.Runs), err)
	}
	vars := rec.last(t).Variables
	if _, ok := vars["after"]; ok {
		t.Error("after should be omitted when empty")
	}
	if orderBy := vars["orderBy"].([]any)[0].(map[string]any); orderBy["field"] != "QUEUED_AT" || orderBy["direction"] != "DESC" {
		t.Errorf("default orderBy = %v", orderBy)
	}

	errSrv, _ := newDevGQLServer(t, map[string]string{"DevRuns": `{"errors":[{"message":"boom"}]}`})
	if _, err := newDevClient(errSrv).ListRuns(context.Background(), ListRunsOptions{}); err == nil {
		t.Error("expected GraphQL error to propagate")
	}
}

func TestGetRun_Dev(t *testing.T) {
	srv, rec := newDevGQLServer(t, map[string]string{
		"DevRun": `{"data":{"run":{"id":"` + testRunID1 + `","status":"COMPLETED","output":"{\"ok\":true}"}}}`,
		"DevRunTrace": `{"data":{"run":{"id":"` + testRunID1 + `",
			"trace":{"id":"s1","name":"fn","status":"COMPLETED","durationMs":12,"children":[{"id":"s2","name":"step","status":"COMPLETED"}]}}}}`,
	})
	client := newDevClient(srv)

	run, err := client.GetRun(context.Background(), testRunID1)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if rec.last(t).Variables["runID"] != testRunID1 {
		t.Errorf("runID variable = %v", rec.last(t).Variables["runID"])
	}
	if string(run.Output) != `{"ok":true}` {
		t.Errorf("Output = %s, want unwrapped JSON", run.Output)
	}
	if run.Trace != nil {
		t.Error("GetRun must not depend on the trace query in dev mode")
	}

	trace, err := client.GetRunTrace(context.Background(), testRunID1)
	if err != nil || trace.ID != "s1" || len(trace.Children) != 1 {
		t.Errorf("GetRunTrace = (%+v, %v)", trace, err)
	}
}

func TestGetRunTrace_DevNotStarted(t *testing.T) {
	// The dev server reports a run with no spans yet as a GraphQL error.
	srv, _ := newDevGQLServer(t, map[string]string{
		"DevRunTrace": `{"data":null,"errors":[{"message":"no function run span found"}]}`,
	})
	if _, err := newDevClient(srv).GetRunTrace(context.Background(), testRunID1); !IsNotFound(err) {
		t.Fatalf("want not-found for a run without spans, got %v", err)
	}

	missing, _ := newDevGQLServer(t, map[string]string{"DevRunTrace": `{"data":{"run":null}}`})
	if _, err := newDevClient(missing).GetRunTrace(context.Background(), "x"); !IsNotFound(err) {
		t.Errorf("missing run = %v", err)
	}
	other, _ := newDevGQLServer(t, map[string]string{"DevRunTrace": `{"data":null,"errors":[{"message":"boom"}]}`})
	if _, err := newDevClient(other).GetRunTrace(context.Background(), "x"); err == nil || IsNotFound(err) {
		t.Errorf("unrelated GraphQL error must propagate, got %v", err)
	}
}

func TestGetRun_DevNotFound(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{"DevRun": `{"data":{"run":null}}`})
	if _, err := newDevClient(srv).GetRun(context.Background(), "missing"); !IsNotFound(err) {
		t.Fatalf("want not-found, got %v", err)
	}
}

func TestCancelAndRerun_Dev(t *testing.T) {
	srv, rec := newDevGQLServer(t, map[string]string{
		"DevCancelRun": `{"data":{"cancelRun":{"id":""}}}`,
		"DevRerun":     `{"data":{"rerun":"` + testRunID2 + `"}}`,
	})
	client := newDevClient(srv)

	id, err := client.CancelRun(context.Background(), testRunID1)
	if err != nil || id != testRunID1 {
		t.Errorf("CancelRun = (%q, %v)", id, err)
	}
	if rec.last(t).Variables["runID"] != testRunID1 {
		t.Errorf("cancel runID = %v", rec.last(t).Variables["runID"])
	}
	newID, err := client.RerunRun(context.Background(), testRunID1)
	if err != nil || newID != testRunID2 {
		t.Errorf("RerunRun = (%q, %v)", newID, err)
	}
}

// TestCloudRunOperations_Errors covers the failure branch of each cloud run
// operation: every wrapper must name the run it was acting on.
func TestCloudRunOperations_Errors(t *testing.T) {
	srv := newErrorServer(t, http.StatusInternalServerError, testServerErrorResp)
	client := newCloudClient(srv)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want string
	}{
		{
			name: "get run trace",
			call: func() error { _, err := client.GetRunTrace(ctx, testRunID1); return err },
			want: "get run trace " + testRunID1,
		},
		{
			name: "cancel run",
			call: func() error { _, err := client.CancelRun(ctx, testRunID1); return err },
			want: "cancel run " + testRunID1,
		},
		{
			name: "rerun",
			call: func() error { _, err := client.RerunRun(ctx, testRunID1); return err },
			want: "rerun " + testRunID1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrContains(t, tt.call(), tt.want)
		})
	}
}
