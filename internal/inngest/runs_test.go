package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// eventsResponse is a helper that wraps function runs in the events-based
// GraphQL response structure used by the new ListRuns query.
func eventsResponse(functionRuns string) string {
	return `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"occurredAt": "2024-01-01T00:00:00Z",
								"receivedAt": "2024-01-01T00:00:00Z",
								"name": "test/event",
								"functionRuns": [` + functionRuns + `]
							}
						]
					}
				],
				"page": {"page": 1, "totalPages": 1}
			}
		}
	}`
}

func TestListRuns(t *testing.T) {
	response := eventsResponse(`{
		"id": "run-1",
		"status": "COMPLETED",
		"startedAt": "2024-01-01T00:00:01Z",
		"endedAt": "2024-01-01T00:00:02Z",
		"output": "{}",
		"function": {"id": "fn-1", "name": "My Func", "slug": "my-func"}
	}`)

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

	// TotalCount is set to the number of filtered results.
	if conn.TotalCount != 1 {
		t.Errorf("expected TotalCount 1, got %d", conn.TotalCount)
	}
	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(conn.Edges))
	}

	edge := conn.Edges[0]
	if edge.Node.ID != testRunID1Events {
		t.Errorf("expected run ID 'run-1', got %q", edge.Node.ID)
	}
	if edge.Node.Status != testStatusCompleted {
		t.Errorf("expected status 'COMPLETED', got %q", edge.Node.Status)
	}
	if edge.Node.EventName != "test/event" {
		t.Errorf("expected eventName 'test/event', got %q", edge.Node.EventName)
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
	response := eventsResponse(`{
		"id": "run-1",
		"status": "RUNNING",
		"startedAt": "2024-01-01T00:00:01Z",
		"output": "{\"result\":true}",
		"function": {"id": "fn-1", "name": "My Func", "slug": "my-func"}
	}`)

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	run, err := client.GetRun(context.Background(), testRunID1Events)
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}

	// Verify scalar fields.
	if run.ID != testRunID1Events {
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
}

func TestGetRun_NotFound(t *testing.T) {
	response := testEmptyEventsResp

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.GetRun(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing run, got nil")
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

	run, err := client.CancelRun(context.Background(), "env-uuid-123", testRunID1Events)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}

	if run.ID != testRunID1Events {
		t.Errorf("expected ID 'run-1', got %q", run.ID)
	}
	if run.Status != "CANCELLED" {
		t.Errorf("expected status 'CANCELLED', got %q", run.Status)
	}

	// Verify the query is a mutation with envID.
	if !strings.Contains(captured.Query, "mutation") {
		t.Errorf("expected query to contain 'mutation', got %q", captured.Query)
	}
	if !strings.Contains(captured.Query, "cancelRun") {
		t.Errorf("expected query to contain 'cancelRun', got %q", captured.Query)
	}
	if !strings.Contains(captured.Query, "$envID: UUID!") {
		t.Errorf("expected query to contain '$envID: UUID!', got %q", captured.Query)
	}

	// Verify the envID variable was sent.
	if envID, ok := captured.Variables["envID"].(string); !ok || envID != "env-uuid-123" {
		t.Errorf("expected envID variable 'env-uuid-123', got %v", captured.Variables["envID"])
	}
}

func TestCancelRun_NoEnvID(t *testing.T) {
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

	run, err := client.CancelRun(context.Background(), "", testRunID1Events)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}

	if run.ID != testRunID1Events {
		t.Errorf("expected ID 'run-1', got %q", run.ID)
	}
	if run.Status != "CANCELLED" {
		t.Errorf("expected status 'CANCELLED', got %q", run.Status)
	}

	// Verify the query does NOT contain $envID when envID is empty.
	if strings.Contains(captured.Query, "$envID") {
		t.Errorf("expected query to NOT contain '$envID' when envID is empty, got %q", captured.Query)
	}
	if strings.Contains(captured.Query, "envID:") {
		t.Errorf("expected query to NOT contain 'envID:' when envID is empty, got %q", captured.Query)
	}

	// Verify the envID variable was NOT sent.
	if _, ok := captured.Variables["envID"]; ok {
		t.Errorf("expected no envID variable, but got %v", captured.Variables["envID"])
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

	newRunID, err := client.RerunRun(context.Background(), testRunID1Events)
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
	if runID, ok := captured.Variables["runID"].(string); !ok || runID != testRunID1Events {
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

func TestListRuns_StatusFilter(t *testing.T) {
	// Response has two runs: one COMPLETED, one FAILED.
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"occurredAt": "2024-01-01T00:00:00Z",
								"receivedAt": "2024-01-01T00:00:00Z",
								"name": "test/event",
								"functionRuns": [
									{
										"id": "run-1",
										"status": "COMPLETED",
										"startedAt": "2024-01-01T00:00:01Z",
										"endedAt": "2024-01-01T00:00:02Z",
										"output": "{}",
										"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
									},
									{
										"id": "run-2",
										"status": "FAILED",
										"startedAt": "2024-01-01T00:00:01Z",
										"endedAt": "2024-01-01T00:00:02Z",
										"output": "{}",
										"function": {"id": "fn-2", "name": "Fn2", "slug": "fn-2"}
									}
								]
							}
						]
					}
				],
				"page": {"page": 1, "totalPages": 1}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	conn, err := client.ListRuns(context.Background(), ListRunsOptions{
		First:  10,
		Status: []string{"COMPLETED"},
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	// Only the COMPLETED run should be returned (client-side filtering).
	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(conn.Edges))
	}
	if conn.Edges[0].Node.ID != "run-1" {
		t.Errorf("expected run-1, got %q", conn.Edges[0].Node.ID)
	}
	if conn.Edges[0].Node.Status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %q", conn.Edges[0].Node.Status)
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

	_, err := client.CancelRun(context.Background(), "env-uuid-123", testRunID1Events)
	if err == nil {
		t.Fatal("expected error for nil cancelRun result, got nil")
	}
	if !strings.Contains(err.Error(), "no result") {
		t.Errorf("expected error to contain 'no result', got %q", err.Error())
	}
}

func TestListRuns_Empty(t *testing.T) {
	response := testEmptyEventsResp

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
	// The From filter is applied client-side. Provide two runs: one before
	// the threshold and one after.
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"occurredAt": "2025-05-01T00:00:00Z",
								"receivedAt": "2025-05-01T00:00:00Z",
								"name": "test/event",
								"functionRuns": [
									{
										"id": "run-early",
										"status": "COMPLETED",
										"startedAt": "2025-05-01T00:00:00Z",
										"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
									},
									{
										"id": "run-late",
										"status": "COMPLETED",
										"startedAt": "2025-07-01T00:00:00Z",
										"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
									}
								]
							}
						]
					}
				],
				"page": {"page": 1, "totalPages": 1}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	from := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	conn, err := client.ListRuns(context.Background(), ListRunsOptions{
		First: 10,
		From:  from,
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	// Only run-late should pass the From filter.
	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge after From filter, got %d", len(conn.Edges))
	}
	if conn.Edges[0].Node.ID != "run-late" {
		t.Errorf("expected run-late, got %q", conn.Edges[0].Node.ID)
	}
}

func TestListRuns_UntilFilter(t *testing.T) {
	// The Until filter is applied client-side. Provide two runs: one before
	// the threshold and one after.
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"occurredAt": "2025-05-01T00:00:00Z",
								"receivedAt": "2025-05-01T00:00:00Z",
								"name": "test/event",
								"functionRuns": [
									{
										"id": "run-early",
										"status": "COMPLETED",
										"startedAt": "2025-05-01T00:00:00Z",
										"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
									},
									{
										"id": "run-late",
										"status": "COMPLETED",
										"startedAt": "2025-07-01T00:00:00Z",
										"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
									}
								]
							}
						]
					}
				],
				"page": {"page": 1, "totalPages": 1}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	until := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	conn, err := client.ListRuns(context.Background(), ListRunsOptions{
		First: 10,
		Until: &until,
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	// Only run-early should pass the Until filter.
	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge after Until filter, got %d", len(conn.Edges))
	}
	if conn.Edges[0].Node.ID != "run-early" {
		t.Errorf("expected run-early, got %q", conn.Edges[0].Node.ID)
	}
}

func TestListRuns_FunctionIDsFilter(t *testing.T) {
	// The FunctionIDs filter is applied client-side.
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"occurredAt": "2025-05-01T00:00:00Z",
								"receivedAt": "2025-05-01T00:00:00Z",
								"name": "test/event",
								"functionRuns": [
									{
										"id": "run-1",
										"status": "COMPLETED",
										"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
									},
									{
										"id": "run-2",
										"status": "COMPLETED",
										"function": {"id": "fn-2", "name": "Fn2", "slug": "fn-2"}
									}
								]
							}
						]
					}
				],
				"page": {"page": 1, "totalPages": 1}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	conn, err := client.ListRuns(context.Background(), ListRunsOptions{
		First:       10,
		FunctionIDs: []string{"fn-1"},
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge after FunctionIDs filter, got %d", len(conn.Edges))
	}
	if conn.Edges[0].Node.ID != "run-1" {
		t.Errorf("expected run-1, got %q", conn.Edges[0].Node.ID)
	}
}

func TestListRuns_AppIDsFilter(t *testing.T) {
	// AppIDs filter is in ListRunsOptions but the events-based response
	// doesn't include appID on function runs. Verify that when AppIDs
	// is set the query still executes without error (filtering is a no-op
	// because runs lack Function.ID matching appIDs).
	response := eventsResponse(`{
		"id": "run-1",
		"status": "COMPLETED",
		"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
	}`)

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	conn, err := client.ListRuns(context.Background(), ListRunsOptions{
		First:  10,
		AppIDs: []string{"app-1"},
	})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	// AppIDs filter doesn't match Function.ID or Slug, so run passes through.
	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(conn.Edges))
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

	_, err := client.GetRun(context.Background(), testRunID1Events)
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected error to contain 'permission denied', got %q", err.Error())
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

	_, err := client.CancelRun(context.Background(), "env-uuid-123", testRunID1Events)
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

	_, err := client.RerunRun(context.Background(), testRunID1Events)
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "rerun failed") {
		t.Errorf("expected error to contain 'rerun failed', got %q", err.Error())
	}
}

func TestGetRun_PostMethod(t *testing.T) {
	var method string

	// GetRun calls ListRuns under the hood, so serve the events-based response.
	response := eventsResponse(`{
		"id": "run-1",
		"status": "COMPLETED",
		"function": {"id": "fn-1", "name": "Fn1", "slug": "fn-1"}
	}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.GetRun(context.Background(), testRunID1Events)
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("expected POST method, got %q", method)
	}
}
