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

func TestSendEvent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/e/test-event-key") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if body["name"] != "test/event.sent" {
			t.Errorf("unexpected event name: %v", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":    []string{"ev-1"},
			"status": 200,
		})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		EventKey:     "test-event-key",
		DevMode:      true,
		DevServerURL: srv.URL,
	})

	event := map[string]interface{}{
		"name": "test/event.sent",
		"data": map[string]string{"foo": "bar"},
	}

	ids, err := c.SendEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("SendEvent returned error: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 ID, got %d", len(ids))
	}
	if ids[0] != "ev-1" {
		t.Errorf("expected id ev-1, got %s", ids[0])
	}
}

func TestSendEvent_MultipleIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":    []string{"ev-1", "ev-2", "ev-3"},
			"status": 200,
		})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		EventKey:     "test-key",
		DevMode:      true,
		DevServerURL: srv.URL,
	})

	ids, err := c.SendEvent(context.Background(), []map[string]interface{}{
		{"name": "test/a", "data": map[string]string{}},
		{"name": "test/b", "data": map[string]string{}},
		{"name": "test/c", "data": map[string]string{}},
	})
	if err != nil {
		t.Fatalf("SendEvent returned error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	expected := []string{"ev-1", "ev-2", "ev-3"}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("ids[%d] = %s, want %s", i, id, expected[i])
		}
	}
}

func TestSendEvent_NoEventKey(t *testing.T) {
	c := NewClient(ClientOptions{
		DevMode: true,
	})

	_, err := c.SendEvent(context.Background(), map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error when event key is empty, got nil")
	}
	if !strings.Contains(err.Error(), "event key is required") {
		t.Errorf("expected error to mention 'event key is required', got: %v", err)
	}
}

func TestSendEvent_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		EventKey:     "test-key",
		DevMode:      true,
		DevServerURL: srv.URL,
	})

	_, err := c.SendEvent(context.Background(), map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status code 500, got: %v", err)
	}
}

func TestSendEvent_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		EventKey:     "test-key",
		DevMode:      true,
		DevServerURL: srv.URL,
	})

	_, err := c.SendEvent(context.Background(), map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error on invalid JSON response, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestSendEvent_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ids": []string{"ev-1"}, "status": 200})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		EventKey:     "test-key",
		DevMode:      true,
		DevServerURL: srv.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.SendEvent(ctx, map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestSendEvent_UnmarshalableEvent(t *testing.T) {
	c := NewClient(ClientOptions{
		EventKey:     "test-key",
		DevMode:      true,
		DevServerURL: "http://localhost:0",
	})

	// Channels cannot be marshalled to JSON.
	_, err := c.SendEvent(context.Background(), make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable event, got nil")
	}
	if !strings.Contains(err.Error(), "marshal") {
		t.Errorf("expected marshal error, got: %v", err)
	}
}

func TestListEvents(t *testing.T) {
	response := `{
		"data": {
			"eventsV2": {
				"edges": [
					{
						"node": {
							"id": "evt-1",
							"name": "user/signup",
							"occurredAt": "2025-01-01T00:00:00Z",
							"receivedAt": "2025-01-01T00:00:01Z",
							"raw": "{\"name\":\"user/signup\",\"data\":{}}",
							"runs": [
								{
									"id": "run-1",
									"status": "COMPLETED",
									"function": {"name": "Send Email"}
								}
							]
						},
						"cursor": "c1"
					},
					{
						"node": {
							"id": "evt-2",
							"name": "order/created",
							"raw": "{\"name\":\"order/created\"}",
							"runs": []
						},
						"cursor": "c2"
					}
				],
				"pageInfo": {
					"hasNextPage": true,
					"endCursor": "c2"
				},
				"totalCount": 5
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

	conn, err := client.ListEvents(context.Background(), ListEventsOptions{First: 2})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	if !strings.Contains(captured.Query, "ListEvents") {
		t.Errorf("expected query to contain 'ListEvents', got: %s", captured.Query)
	}

	if conn.TotalCount != 5 {
		t.Errorf("expected TotalCount 5, got %d", conn.TotalCount)
	}
	if !conn.PageInfo.HasNextPage {
		t.Error("expected HasNextPage true")
	}
	if conn.PageInfo.EndCursor != "c2" {
		t.Errorf("expected EndCursor 'c2', got %q", conn.PageInfo.EndCursor)
	}
	if len(conn.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(conn.Edges))
	}

	// First event.
	evt := conn.Edges[0].Node
	if evt.ID != "evt-1" {
		t.Errorf("expected ID 'evt-1', got %q", evt.ID)
	}
	if evt.Name != "user/signup" {
		t.Errorf("expected Name 'user/signup', got %q", evt.Name)
	}
	if evt.CreatedAt == nil {
		t.Error("expected CreatedAt to be non-nil")
	}
	if evt.ReceivedAt == nil {
		t.Error("expected ReceivedAt to be non-nil")
	}
	if len(evt.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(evt.Runs))
	}
	if evt.Runs[0].ID != "run-1" {
		t.Errorf("expected run ID 'run-1', got %q", evt.Runs[0].ID)
	}
	if evt.Runs[0].Status != "COMPLETED" {
		t.Errorf("expected run status 'COMPLETED', got %q", evt.Runs[0].Status)
	}
	if evt.Runs[0].Function == nil {
		t.Fatal("expected run function to be non-nil")
	}
	if evt.Runs[0].Function.Name != "Send Email" {
		t.Errorf("expected function name 'Send Email', got %q", evt.Runs[0].Function.Name)
	}
	if conn.Edges[0].Cursor != "c1" {
		t.Errorf("expected cursor 'c1', got %q", conn.Edges[0].Cursor)
	}

	// Second event.
	if conn.Edges[1].Node.ID != "evt-2" {
		t.Errorf("expected ID 'evt-2', got %q", conn.Edges[1].Node.ID)
	}
	if len(conn.Edges[1].Node.Runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(conn.Edges[1].Node.Runs))
	}
}

func TestListEventsError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "unauthorized"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	conn, err := client.ListEvents(context.Background(), ListEventsOptions{First: 10})
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if conn != nil {
		t.Errorf("expected nil connection, got %+v", conn)
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected error to contain 'unauthorized', got: %v", err)
	}
}

func TestGetEvent(t *testing.T) {
	response := `{
		"data": {
			"event": {
				"id": "evt-1",
				"name": "user/signup",
				"occurredAt": "2025-01-01T00:00:00Z",
				"receivedAt": "2025-01-01T00:00:01Z",
				"raw": "{\"name\":\"user/signup\",\"data\":{\"email\":\"test@example.com\"}}",
				"runs": [
					{
						"id": "run-1",
						"status": "COMPLETED",
						"function": {"name": "Send Welcome Email"}
					},
					{
						"id": "run-2",
						"status": "FAILED",
						"function": {"name": "Create Profile"}
					}
				]
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

	event, err := client.GetEvent(context.Background(), "evt-1")
	if err != nil {
		t.Fatalf("GetEvent returned error: %v", err)
	}

	// Verify request.
	if !strings.Contains(captured.Query, "GetEvent") {
		t.Errorf("expected query to contain 'GetEvent', got: %s", captured.Query)
	}
	if captured.Variables == nil {
		t.Fatal("expected variables to be non-nil")
	}
	if eventID, ok := captured.Variables["eventId"].(string); !ok || eventID != "evt-1" {
		t.Errorf("expected eventId variable 'evt-1', got %v", captured.Variables["eventId"])
	}

	// Verify response.
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.ID != "evt-1" {
		t.Errorf("expected ID 'evt-1', got %q", event.ID)
	}
	if event.Name != "user/signup" {
		t.Errorf("expected Name 'user/signup', got %q", event.Name)
	}
	if event.CreatedAt == nil {
		t.Error("expected CreatedAt to be non-nil (from occurredAt)")
	}
	if event.ReceivedAt == nil {
		t.Error("expected ReceivedAt to be non-nil")
	}
	if event.TotalRuns != 2 {
		t.Errorf("expected TotalRuns 2, got %d", event.TotalRuns)
	}
	if len(event.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(event.Runs))
	}

	if event.Runs[0].ID != "run-1" {
		t.Errorf("expected run[0] ID 'run-1', got %q", event.Runs[0].ID)
	}
	if event.Runs[0].Status != "COMPLETED" {
		t.Errorf("expected run[0] status 'COMPLETED', got %q", event.Runs[0].Status)
	}
	if event.Runs[0].Function == nil || event.Runs[0].Function.Name != "Send Welcome Email" {
		t.Errorf("expected run[0] function name 'Send Welcome Email'")
	}

	if event.Runs[1].ID != "run-2" {
		t.Errorf("expected run[1] ID 'run-2', got %q", event.Runs[1].ID)
	}
	if event.Runs[1].Status != "FAILED" {
		t.Errorf("expected run[1] status 'FAILED', got %q", event.Runs[1].Status)
	}
}

func TestGetEventNotFound(t *testing.T) {
	response := `{"data": {"event": null}}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	event, err := client.GetEvent(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nil event, got nil")
	}
	if event != nil {
		t.Errorf("expected nil event, got %+v", event)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got %q", err.Error())
	}
}

func TestGetEventRuns_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/events/evt-123/runs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Errorf("expected Accept application/json, got %s", accept)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"run_id":      "run-1",
					"status":      "COMPLETED",
					"function_id": "fn-1",
					"output":      `{"result": true}`,
				},
				{
					"run_id":      "run-2",
					"status":      "FAILED",
					"function_id": "fn-2",
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "test-key",
	})

	runs, err := c.GetEventRuns(context.Background(), "evt-123")
	if err != nil {
		t.Fatalf("GetEventRuns returned error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	if runs[0].ID != "run-1" {
		t.Errorf("runs[0].ID = %s, want run-1", runs[0].ID)
	}
	if runs[0].Status != "COMPLETED" {
		t.Errorf("runs[0].Status = %s, want COMPLETED", runs[0].Status)
	}
	if runs[0].FunctionID != "fn-1" {
		t.Errorf("runs[0].FunctionID = %s, want fn-1", runs[0].FunctionID)
	}
	if runs[0].Output != `{"result": true}` {
		t.Errorf("runs[0].Output = %s, want {\"result\": true}", runs[0].Output)
	}

	if runs[1].ID != "run-2" {
		t.Errorf("runs[1].ID = %s, want run-2", runs[1].ID)
	}
	if runs[1].Status != "FAILED" {
		t.Errorf("runs[1].Status = %s, want FAILED", runs[1].Status)
	}
	if runs[1].FunctionID != "fn-2" {
		t.Errorf("runs[1].FunctionID = %s, want fn-2", runs[1].FunctionID)
	}
}

func TestGetEventRuns_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{},
		})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "test-key",
	})

	runs, err := c.GetEventRuns(context.Background(), "evt-empty")
	if err != nil {
		t.Fatalf("GetEventRuns returned error: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

func TestGetEventRuns_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "test-key",
	})

	_, err := c.GetEventRuns(context.Background(), "evt-fail")
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention 500, got: %v", err)
	}
}

func TestGetEventRuns_AuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer signkey-123" {
			t.Errorf("expected Authorization 'Bearer signkey-123', got '%s'", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{},
		})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "signkey-123",
	})

	_, err := c.GetEventRuns(context.Background(), "evt-auth")
	if err != nil {
		t.Fatalf("GetEventRuns returned error: %v", err)
	}
}

func TestSendEvent_DevModeURL(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":    []string{"ev-1"},
			"status": 200,
		})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		EventKey:     "my-key",
		DevMode:      true,
		DevServerURL: srv.URL,
	})

	_, err := c.SendEvent(context.Background(), map[string]string{"name": "test"})
	if err != nil {
		t.Fatalf("SendEvent returned error: %v", err)
	}
	if capturedPath != "/e/my-key" {
		t.Errorf("expected path /e/my-key, got %s", capturedPath)
	}
}

func TestListEvents_SinceFilter(t *testing.T) {
	response := `{"data": {"eventsV2": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	since := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	_, err := client.ListEvents(context.Background(), ListEventsOptions{
		First: 10,
		Since: since,
	})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	// Verify filter contains "from" key.
	filterRaw, ok := captured.Variables["filter"]
	if !ok {
		t.Fatal("expected filter in variables")
	}
	filterBytes, _ := json.Marshal(filterRaw)
	var filter map[string]interface{}
	if err := json.Unmarshal(filterBytes, &filter); err != nil {
		t.Fatalf("failed to unmarshal filter: %v", err)
	}
	from, ok := filter["from"]
	if !ok {
		t.Fatal("expected 'from' key in filter")
	}
	if from != since.Format(time.RFC3339) {
		t.Errorf("expected from=%q, got %q", since.Format(time.RFC3339), from)
	}
}

func TestListEvents_NameFilter(t *testing.T) {
	response := `{"data": {"eventsV2": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListEvents(context.Background(), ListEventsOptions{
		First: 10,
		Name:  "user/signup",
	})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
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
	name, ok := filter["name"]
	if !ok {
		t.Fatal("expected 'name' key in filter")
	}
	if name != "user/signup" {
		t.Errorf("expected name=%q, got %q", "user/signup", name)
	}
}

func TestListEvents_FirstDefaultsTo20(t *testing.T) {
	response := `{"data": {"eventsV2": {"edges": [], "pageInfo": {"hasNextPage": false}, "totalCount": 0}}}`

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListEvents(context.Background(), ListEventsOptions{
		First: 0, // should default to 20
	})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	first, ok := captured.Variables["first"].(float64)
	if !ok {
		t.Fatalf("expected first to be a number, got %T", captured.Variables["first"])
	}
	if int(first) != 20 {
		t.Errorf("expected first=20, got %d", int(first))
	}
}

func TestListEvents_RunWithNilFunction(t *testing.T) {
	response := `{
		"data": {
			"eventsV2": {
				"edges": [
					{
						"node": {
							"id": "evt-1",
							"name": "test/event",
							"runs": [
								{
									"id": "run-1",
									"status": "RUNNING"
								}
							]
						},
						"cursor": "c1"
					}
				],
				"pageInfo": {"hasNextPage": false},
				"totalCount": 1
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	conn, err := client.ListEvents(context.Background(), ListEventsOptions{First: 10})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	if len(conn.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(conn.Edges))
	}
	run := conn.Edges[0].Node.Runs[0]
	if run.Function != nil {
		t.Errorf("expected nil Function for run without function field, got %+v", run.Function)
	}
	if run.ID != "run-1" {
		t.Errorf("expected run ID 'run-1', got %q", run.ID)
	}
}

func TestGetEvent_NilOccurredAt(t *testing.T) {
	response := `{
		"data": {
			"event": {
				"id": "evt-1",
				"name": "test/event",
				"receivedAt": "2025-01-01T00:00:01Z",
				"raw": "{}",
				"runs": []
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	event, err := client.GetEvent(context.Background(), "evt-1")
	if err != nil {
		t.Fatalf("GetEvent returned error: %v", err)
	}

	if event.CreatedAt != nil {
		t.Errorf("expected nil CreatedAt when occurredAt is missing, got %v", event.CreatedAt)
	}
	if event.ReceivedAt == nil {
		t.Error("expected non-nil ReceivedAt")
	}
}

func TestGetEvent_RunWithNilFunction(t *testing.T) {
	response := `{
		"data": {
			"event": {
				"id": "evt-1",
				"name": "test/event",
				"runs": [
					{
						"id": "run-1",
						"status": "RUNNING"
					}
				]
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	event, err := client.GetEvent(context.Background(), "evt-1")
	if err != nil {
		t.Fatalf("GetEvent returned error: %v", err)
	}

	if len(event.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(event.Runs))
	}
	if event.Runs[0].Function != nil {
		t.Errorf("expected nil Function, got %+v", event.Runs[0].Function)
	}
}

func TestGetEvent_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "internal error"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.GetEvent(context.Background(), "evt-1")
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Errorf("expected error to contain 'internal error', got %q", err.Error())
	}
}

func TestGetEventRuns_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{},
		})
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "test-key",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetEventRuns(ctx, "evt-cancelled")
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestSendEvent_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ids":["ev-1"],"status":200}`))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		EventKey:     "test-key",
		DevMode:      true,
		DevServerURL: srv.URL,
	})
	c.httpClient.Transport = &errBodyTransport{wrapped: srv.Client().Transport}

	_, err := c.SendEvent(context.Background(), map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error when response body read fails, got nil")
	}
	if !strings.Contains(err.Error(), "read send event response") {
		t.Errorf("expected error to contain 'read send event response', got: %v", err)
	}
}

func TestSendEvent_NewRequestError(t *testing.T) {
	client := NewClient(ClientOptions{
		EventKey:     "k",
		DevMode:      true,
		DevServerURL: "http://invalid\x00host",
	})
	_, err := client.SendEvent(context.Background(), map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "create send event request") {
		t.Errorf("expected 'create send event request' error, got: %v", err)
	}
}
