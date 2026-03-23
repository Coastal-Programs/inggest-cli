package inngest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListRuns(t *testing.T) {
	response := `{
		"data": {
			"runs": {
				"edges": [
					{
						"node": {
							"id": "run-1",
							"status": "COMPLETED",
							"eventName": "test/event",
							"isBatch": false,
							"function": {
								"name": "My Func",
								"slug": "my-func"
							}
						},
						"cursor": "c1"
					}
				],
				"pageInfo": {
					"hasNextPage": false
				},
				"totalCount": 1
			}
		}
	}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	conn, err := client.ListRuns(context.Background(), ListRunsOptions{
		First: 10,
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	// Verify the returned connection.
	if conn.TotalCount != 1 {
		t.Errorf("expected TotalCount 1, got %d", conn.TotalCount)
	}
	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(conn.Edges))
	}

	edge := conn.Edges[0]
	if edge.Node.ID != "run-1" {
		t.Errorf("expected run ID 'run-1', got %q", edge.Node.ID)
	}
	if edge.Node.Status != "COMPLETED" {
		t.Errorf("expected status 'COMPLETED', got %q", edge.Node.Status)
	}
	if edge.Node.EventName != "test/event" {
		t.Errorf("expected eventName 'test/event', got %q", edge.Node.EventName)
	}
	if edge.Cursor != "c1" {
		t.Errorf("expected cursor 'c1', got %q", edge.Cursor)
	}
	if edge.Node.Function == nil {
		t.Fatal("expected Function to be non-nil")
	}
	if edge.Node.Function.Name != "My Func" {
		t.Errorf("expected function name 'My Func', got %q", edge.Node.Function.Name)
	}
	if edge.Node.Function.Slug != "my-func" {
		t.Errorf("expected function slug 'my-func', got %q", edge.Node.Function.Slug)
	}
	if conn.PageInfo.HasNextPage {
		t.Errorf("expected hasNextPage false")
	}

	// Verify the query contains the expected operation name.
	if !strings.Contains(captured.Query, "ListRuns") {
		t.Errorf("expected query to contain 'ListRuns', got %q", captured.Query)
	}
}

func TestGetRun(t *testing.T) {
	response := `{
		"data": {
			"run": {
				"id": "run-1",
				"status": "RUNNING",
				"eventName": "test/event",
				"isBatch": false,
				"output": "{\"result\":true}",
				"traceID": "trace-1",
				"function": {
					"name": "My Func",
					"slug": "my-func",
					"config": "{}"
				},
				"app": {
					"name": "my-app",
					"sdkLanguage": "go",
					"sdkVersion": "1.0"
				}
			}
		}
	}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	run, err := client.GetRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}

	// Verify scalar fields.
	if run.ID != "run-1" {
		t.Errorf("expected ID 'run-1', got %q", run.ID)
	}
	if run.Status != "RUNNING" {
		t.Errorf("expected status 'RUNNING', got %q", run.Status)
	}
	if run.EventName != "test/event" {
		t.Errorf("expected eventName 'test/event', got %q", run.EventName)
	}
	if run.Output != `{"result":true}` {
		t.Errorf("expected output '{\"result\":true}', got %q", run.Output)
	}
	if run.TraceID != "trace-1" {
		t.Errorf("expected traceID 'trace-1', got %q", run.TraceID)
	}

	// Verify Function.
	if run.Function == nil {
		t.Fatal("expected Function to be non-nil")
	}
	if run.Function.Name != "My Func" {
		t.Errorf("expected function name 'My Func', got %q", run.Function.Name)
	}
	if run.Function.Slug != "my-func" {
		t.Errorf("expected function slug 'my-func', got %q", run.Function.Slug)
	}
	if run.Function.Config != "{}" {
		t.Errorf("expected function config '{}', got %q", run.Function.Config)
	}

	// Verify App.
	if run.App == nil {
		t.Fatal("expected App to be non-nil")
	}
	if run.App.Name != "my-app" {
		t.Errorf("expected app name 'my-app', got %q", run.App.Name)
	}
	if run.App.SDKLanguage != "go" {
		t.Errorf("expected sdkLanguage 'go', got %q", run.App.SDKLanguage)
	}
	if run.App.SDKVersion != "1.0" {
		t.Errorf("expected sdkVersion '1.0', got %q", run.App.SDKVersion)
	}

	// Verify the captured request contains expected variable.
	if captured.Variables == nil {
		t.Fatal("expected variables to be non-nil")
	}
	if runID, ok := captured.Variables["runID"].(string); !ok || runID != "run-1" {
		t.Errorf("expected runID variable 'run-1', got %v", captured.Variables["runID"])
	}
}

func TestGetRun_NotFound(t *testing.T) {
	response := `{"data": {"run": null}}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.GetRun(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nil run, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got %q", err.Error())
	}
}

func TestCancelRun(t *testing.T) {
	response := `{
		"data": {
			"cancelRun": {
				"id": "run-1",
				"status": "CANCELLED"
			}
		}
	}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	run, err := client.CancelRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}

	if run.ID != "run-1" {
		t.Errorf("expected ID 'run-1', got %q", run.ID)
	}
	if run.Status != "CANCELLED" {
		t.Errorf("expected status 'CANCELLED', got %q", run.Status)
	}

	// Verify the query is a mutation.
	if !strings.Contains(captured.Query, "mutation") {
		t.Errorf("expected query to contain 'mutation', got %q", captured.Query)
	}
	if !strings.Contains(captured.Query, "cancelRun") {
		t.Errorf("expected query to contain 'cancelRun', got %q", captured.Query)
	}
}

func TestRerunRun(t *testing.T) {
	response := `{
		"data": {
			"rerun": "run-2"
		}
	}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	newRunID, err := client.RerunRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("RerunRun returned error: %v", err)
	}

	if newRunID != "run-2" {
		t.Errorf("expected new run ID 'run-2', got %q", newRunID)
	}

	// Verify the query is a mutation.
	if !strings.Contains(captured.Query, "mutation") {
		t.Errorf("expected query to contain 'mutation', got %q", captured.Query)
	}
	if !strings.Contains(captured.Query, "rerun") {
		t.Errorf("expected query to contain 'rerun', got %q", captured.Query)
	}
	// Verify the runID variable was sent.
	if runID, ok := captured.Variables["runID"].(string); !ok || runID != "run-1" {
		t.Errorf("expected runID variable 'run-1', got %v", captured.Variables["runID"])
	}
}

func TestListRuns_GraphQLError(t *testing.T) {
	response := `{
		"data": null,
		"errors": [{"message": "unauthorized"}]
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListRuns(context.Background(), ListRunsOptions{First: 10})
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected error to contain 'unauthorized', got %q", err.Error())
	}
}

func TestListRuns_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListRuns(context.Background(), ListRunsOptions{First: 10})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got %q", err.Error())
	}
}

func TestListRuns_RequestBody(t *testing.T) {
	var captured graphqlRequest
	srv := newTestServer(t, `{"data": {"runs": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListRuns(context.Background(), ListRunsOptions{
		First:  5,
		After:  "cursor-abc",
		Status: []string{"COMPLETED"},
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	// Verify the query string contains expected fragments.
	if !strings.Contains(captured.Query, "ListRuns") {
		t.Errorf("expected query to contain 'ListRuns', got %q", captured.Query)
	}

	// Verify variables.
	vars := captured.Variables
	if vars == nil {
		t.Fatal("expected variables to be non-nil")
	}

	// "first" is sent as a number — json decodes it as float64.
	if first, ok := vars["first"].(float64); !ok || int(first) != 5 {
		t.Errorf("expected first=5, got %v", vars["first"])
	}
	if after, ok := vars["after"].(string); !ok || after != "cursor-abc" {
		t.Errorf("expected after='cursor-abc', got %v", vars["after"])
	}

	// Verify filter contains status.
	filterRaw, ok := vars["filter"]
	if !ok {
		t.Fatal("expected filter in variables")
	}
	filterBytes, _ := json.Marshal(filterRaw)
	var filter map[string]interface{}
	if err := json.Unmarshal(filterBytes, &filter); err != nil {
		t.Fatalf("failed to unmarshal filter: %v", err)
	}
	statusRaw, ok := filter["status"]
	if !ok {
		t.Fatal("expected status in filter")
	}
	statusArr, ok := statusRaw.([]interface{})
	if !ok {
		t.Fatalf("expected status to be an array, got %T", statusRaw)
	}
	if len(statusArr) != 1 || statusArr[0] != "COMPLETED" {
		t.Errorf("expected status ['COMPLETED'], got %v", statusArr)
	}
}

func TestCancelRun_NilResult(t *testing.T) {
	response := `{"data": {"cancelRun": null}}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.CancelRun(context.Background(), "run-1")
	if err == nil {
		t.Fatal("expected error for nil cancelRun result, got nil")
	}
	if !strings.Contains(err.Error(), "no result") {
		t.Errorf("expected error to contain 'no result', got %q", err.Error())
	}
}

func TestListRuns_EmptyEdges(t *testing.T) {
	response := `{
		"data": {
			"runs": {
				"edges": [],
				"pageInfo": {"hasNextPage": false},
				"totalCount": 0
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	conn, err := client.ListRuns(context.Background(), ListRunsOptions{})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(conn.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(conn.Edges))
	}
	if conn.TotalCount != 0 {
		t.Errorf("expected totalCount 0, got %d", conn.TotalCount)
	}
}

func TestConvertTrace(t *testing.T) {
	started := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ended := time.Date(2025, 1, 1, 0, 0, 1, 0, time.UTC)
	childStarted := time.Date(2025, 1, 1, 0, 0, 0, 500000000, time.UTC)
	childEnded := time.Date(2025, 1, 1, 0, 0, 0, 800000000, time.UTC)

	node := &traceNode{
		RunID:     "run-1",
		SpanID:    "span-root",
		Name:      "my-function",
		Status:    "COMPLETED",
		StartedAt: &started,
		EndedAt:   &ended,
		Duration:  1000,
		StepOp:    "",
		Children: []traceNode{
			{
				SpanID:    "span-child-1",
				Name:      "step.run",
				Status:    "COMPLETED",
				StartedAt: &childStarted,
				EndedAt:   &childEnded,
				Duration:  300,
				StepOp:    "StepRun",
				Children:  nil,
			},
			{
				SpanID:   "span-child-2",
				Name:     "step.sleep",
				Status:   "COMPLETED",
				Duration: 500,
				StepOp:   "Sleep",
			},
		},
	}

	span := convertTrace(node)
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	if span.RunID != "run-1" {
		t.Errorf("expected RunID 'run-1', got %q", span.RunID)
	}
	if span.SpanID != "span-root" {
		t.Errorf("expected SpanID 'span-root', got %q", span.SpanID)
	}
	if span.Name != "my-function" {
		t.Errorf("expected Name 'my-function', got %q", span.Name)
	}
	if span.Status != "COMPLETED" {
		t.Errorf("expected Status 'COMPLETED', got %q", span.Status)
	}
	if span.Duration != 1000 {
		t.Errorf("expected Duration 1000, got %d", span.Duration)
	}
	if span.StartedAt == nil || !span.StartedAt.Equal(started) {
		t.Errorf("unexpected StartedAt: %v", span.StartedAt)
	}
	if span.EndedAt == nil || !span.EndedAt.Equal(ended) {
		t.Errorf("unexpected EndedAt: %v", span.EndedAt)
	}

	// Children.
	if len(span.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(span.Children))
	}
	child1 := span.Children[0]
	if child1.SpanID != "span-child-1" {
		t.Errorf("expected child SpanID 'span-child-1', got %q", child1.SpanID)
	}
	if child1.Name != "step.run" {
		t.Errorf("expected child Name 'step.run', got %q", child1.Name)
	}
	if child1.StepOp != "StepRun" {
		t.Errorf("expected child StepOp 'StepRun', got %q", child1.StepOp)
	}
	if child1.Duration != 300 {
		t.Errorf("expected child Duration 300, got %d", child1.Duration)
	}

	child2 := span.Children[1]
	if child2.SpanID != "span-child-2" {
		t.Errorf("expected child SpanID 'span-child-2', got %q", child2.SpanID)
	}
	if child2.StepOp != "Sleep" {
		t.Errorf("expected child StepOp 'Sleep', got %q", child2.StepOp)
	}
}

func TestConvertTrace_Nil(t *testing.T) {
	span := convertTrace(nil)
	if span != nil {
		t.Errorf("expected nil span for nil input, got %+v", span)
	}
}

func TestStatusToUpper(t *testing.T) {
	input := []string{"running", "Failed", "COMPLETED"}
	result := StatusToUpper(input)

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	expected := []string{"RUNNING", "FAILED", "COMPLETED"}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestStatusToUpperEmpty(t *testing.T) {
	result := StatusToUpper([]string{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestListRuns_FromFilter(t *testing.T) {
	response := `{"data": {"runs": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	from := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	_, err := client.ListRuns(context.Background(), ListRunsOptions{
		First: 10,
		From:  from,
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	filterRaw, ok := captured.Variables["filter"]
	if !ok {
		t.Fatal("expected filter in variables")
	}
	filterBytes, _ := json.Marshal(filterRaw)
	var filter map[string]interface{}
	if err := json.Unmarshal(filterBytes, &filter); err != nil {
		t.Fatalf("failed to unmarshal filter: %v", err)
	}
	fromVal, ok := filter["from"]
	if !ok {
		t.Fatal("expected 'from' key in filter")
	}
	if fromVal != from.Format(time.RFC3339) {
		t.Errorf("expected from=%q, got %q", from.Format(time.RFC3339), fromVal)
	}
}

func TestListRuns_UntilFilter(t *testing.T) {
	response := `{"data": {"runs": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	until := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	_, err := client.ListRuns(context.Background(), ListRunsOptions{
		First: 10,
		Until: &until,
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	filterRaw, ok := captured.Variables["filter"]
	if !ok {
		t.Fatal("expected filter in variables")
	}
	filterBytes, _ := json.Marshal(filterRaw)
	var filter map[string]interface{}
	if err := json.Unmarshal(filterBytes, &filter); err != nil {
		t.Fatalf("failed to unmarshal filter: %v", err)
	}
	untilVal, ok := filter["until"]
	if !ok {
		t.Fatal("expected 'until' key in filter")
	}
	if untilVal != until.Format(time.RFC3339) {
		t.Errorf("expected until=%q, got %q", until.Format(time.RFC3339), untilVal)
	}
}

func TestListRuns_FunctionIDsFilter(t *testing.T) {
	response := `{"data": {"runs": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListRuns(context.Background(), ListRunsOptions{
		First:       10,
		FunctionIDs: []string{"fn-1"},
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	filterRaw, ok := captured.Variables["filter"]
	if !ok {
		t.Fatal("expected filter in variables")
	}
	filterBytes, _ := json.Marshal(filterRaw)
	var filter map[string]interface{}
	if err := json.Unmarshal(filterBytes, &filter); err != nil {
		t.Fatalf("failed to unmarshal filter: %v", err)
	}
	fnIDs, ok := filter["functionIDs"]
	if !ok {
		t.Fatal("expected 'functionIDs' key in filter")
	}
	fnArr, ok := fnIDs.([]interface{})
	if !ok {
		t.Fatalf("expected functionIDs to be an array, got %T", fnIDs)
	}
	if len(fnArr) != 1 || fnArr[0] != "fn-1" {
		t.Errorf("expected functionIDs ['fn-1'], got %v", fnArr)
	}
}

func TestListRuns_AppIDsFilter(t *testing.T) {
	response := `{"data": {"runs": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListRuns(context.Background(), ListRunsOptions{
		First:  10,
		AppIDs: []string{"app-1"},
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	filterRaw, ok := captured.Variables["filter"]
	if !ok {
		t.Fatal("expected filter in variables")
	}
	filterBytes, _ := json.Marshal(filterRaw)
	var filter map[string]interface{}
	if err := json.Unmarshal(filterBytes, &filter); err != nil {
		t.Fatalf("failed to unmarshal filter: %v", err)
	}
	appIDs, ok := filter["appIDs"]
	if !ok {
		t.Fatal("expected 'appIDs' key in filter")
	}
	appArr, ok := appIDs.([]interface{})
	if !ok {
		t.Fatalf("expected appIDs to be an array, got %T", appIDs)
	}
	if len(appArr) != 1 || appArr[0] != "app-1" {
		t.Errorf("expected appIDs ['app-1'], got %v", appArr)
	}
}

func TestListRuns_QueryFilter(t *testing.T) {
	response := `{"data": {"runs": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListRuns(context.Background(), ListRunsOptions{
		First: 10,
		Query: "cel-query",
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	filterRaw, ok := captured.Variables["filter"]
	if !ok {
		t.Fatal("expected filter in variables")
	}
	filterBytes, _ := json.Marshal(filterRaw)
	var filter map[string]interface{}
	if err := json.Unmarshal(filterBytes, &filter); err != nil {
		t.Fatalf("failed to unmarshal filter: %v", err)
	}
	query, ok := filter["query"]
	if !ok {
		t.Fatal("expected 'query' key in filter")
	}
	if query != "cel-query" {
		t.Errorf("expected query=%q, got %q", "cel-query", query)
	}
}

func TestGetRun_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "permission denied"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.GetRun(context.Background(), "run-1")
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected error to contain 'permission denied', got %q", err.Error())
	}
}

func TestGetRun_NilFunctionNilAppWithTrace(t *testing.T) {
	response := `{
		"data": {
			"run": {
				"id": "run-1",
				"status": "COMPLETED",
				"isBatch": false,
				"eventName": "test/event",
				"output": "{}",
				"traceID": "trace-1",
				"trace": {
					"runID": "run-1",
					"spanID": "span-1",
					"name": "root",
					"status": "COMPLETED",
					"durationMS": 500,
					"childrenSpans": [
						{
							"spanID": "span-2",
							"name": "step.run",
							"status": "COMPLETED",
							"durationMS": 200,
							"stepOp": "StepRun"
						}
					]
				}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	run, err := client.GetRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}

	if run.Function != nil {
		t.Errorf("expected nil Function, got %+v", run.Function)
	}
	if run.App != nil {
		t.Errorf("expected nil App, got %+v", run.App)
	}
	if run.Trace == nil {
		t.Fatal("expected non-nil Trace")
	}
	if run.Trace.SpanID != "span-1" {
		t.Errorf("expected trace SpanID 'span-1', got %q", run.Trace.SpanID)
	}
	if len(run.Trace.Children) != 1 {
		t.Fatalf("expected 1 child span, got %d", len(run.Trace.Children))
	}
	if run.Trace.Children[0].StepOp != "StepRun" {
		t.Errorf("expected child StepOp 'StepRun', got %q", run.Trace.Children[0].StepOp)
	}
}

func TestNodeToFunctionRun_NilFunctionAppPresent(t *testing.T) {
	node := runNode{
		ID:     "run-1",
		Status: "COMPLETED",
		App: &struct {
			Name string `json:"name"`
		}{Name: "my-app"},
	}

	run := nodeToFunctionRun(node)
	if run.Function != nil {
		t.Errorf("expected nil Function, got %+v", run.Function)
	}
	if run.App == nil {
		t.Fatal("expected non-nil App")
	}
	if run.App.Name != "my-app" {
		t.Errorf("expected app name 'my-app', got %q", run.App.Name)
	}
}

func TestCancelRun_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "run not cancellable"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.CancelRun(context.Background(), "run-1")
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "run not cancellable") {
		t.Errorf("expected error to contain 'run not cancellable', got %q", err.Error())
	}
}

func TestRerunRun_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "rerun failed"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.RerunRun(context.Background(), "run-1")
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "rerun failed") {
		t.Errorf("expected error to contain 'rerun failed', got %q", err.Error())
	}
}

func TestGetRun_PostMethod(t *testing.T) {
	var method string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": {"run": {"id": "run-1", "status": "COMPLETED", "isBatch": false}}}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.GetRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("expected POST method, got %q", method)
	}
}
