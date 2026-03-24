package inngest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testRunID1Events    = "run-1"
	testStatusCompleted = "COMPLETED"
	testRunID2          = "run-2"
	testEmptyEventsResp = `{"data": {"events": {"data": [], "page": {"page": 1, "perPage": 20, "totalItems": 0, "totalPages": 0}}}}`
)

func TestSendEvent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/e/test-event-key") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != testApplicationJSON {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if body["name"] != "test/event.sent" {
			t.Errorf("unexpected event name: %v", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
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

	event := map[string]any{
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
		json.NewEncoder(w).Encode(map[string]any{
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

	ids, err := c.SendEvent(context.Background(), []map[string]any{
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
		json.NewEncoder(w).Encode(map[string]any{"ids": []string{"ev-1"}, "status": 200})
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
			"events": {
				"data": [
					{
						"name": "user/signup",
						"description": "User signed up",
						"firstSeen": "2025-01-01T00:00:00Z",
						"usage": {"total": 42},
						"workflows": [
							{
								"id": "fn-1",
								"name": "Send Email",
								"slug": "send-email",
								"triggers": [{"type": "event", "value": "user/signup"}],
								"app": {"id": "app-1", "name": "My App", "externalID": "my-app"}
							}
						],
						"recent": [
							{
								"id": "evt-1",
								"occurredAt": "2025-01-01T00:00:00Z",
								"receivedAt": "2025-01-01T00:00:01Z",
								"name": "user/signup",
								"event": "{\"name\":\"user/signup\",\"data\":{}}",
								"functionRuns": [
									{
										"id": "run-1",
										"status": "COMPLETED",
										"function": {"id": "fn-1", "name": "Send Email", "slug": "send-email"}
									}
								]
							}
						]
					},
					{
						"name": "order/created",
						"usage": {"total": 10},
						"recent": []
					}
				],
				"page": {
					"page": 1,
					"perPage": 20,
					"totalItems": 2,
					"totalPages": 1
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

	result, err := client.ListEvents(context.Background(), ListEventsOptions{RecentCount: 5})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	if !strings.Contains(captured.Query, "ListEvents") {
		t.Errorf("expected query to contain 'ListEvents', got: %s", captured.Query)
	}

	if result.Page.TotalItems != 2 {
		t.Errorf("expected TotalItems 2, got %d", result.Page.TotalItems)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 event types, got %d", len(result.Data))
	}

	// First event type.
	et := result.Data[0]
	if et.Name != testTriggerUserSignup {
		t.Errorf("expected Name 'user/signup', got %q", et.Name)
	}
	if et.Description != "User signed up" {
		t.Errorf("expected Description 'User signed up', got %q", et.Description)
	}
	if et.Usage == nil || et.Usage.Total != 42 {
		t.Errorf("expected Usage.Total 42, got %v", et.Usage)
	}
	if len(et.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(et.Workflows))
	}
	if et.Workflows[0].Name != testSendEmail {
		t.Errorf("expected workflow name 'Send Email', got %q", et.Workflows[0].Name)
	}
	if len(et.Recent) != 1 {
		t.Fatalf("expected 1 recent event, got %d", len(et.Recent))
	}
	recent := et.Recent[0]
	if recent.ID != testEvtID1 {
		t.Errorf("expected recent ID 'evt-1', got %q", recent.ID)
	}
	if recent.OccurredAt == nil {
		t.Error("expected OccurredAt to be non-nil")
	}
	if len(recent.FunctionRuns) != 1 {
		t.Fatalf("expected 1 function run, got %d", len(recent.FunctionRuns))
	}
	if recent.FunctionRuns[0].ID != testRunID1Events {
		t.Errorf("expected run ID 'run-1', got %q", recent.FunctionRuns[0].ID)
	}
	if recent.FunctionRuns[0].Status != testStatusCompleted {
		t.Errorf("expected run status 'COMPLETED', got %q", recent.FunctionRuns[0].Status)
	}

	// Second event type.
	if result.Data[1].Name != "order/created" {
		t.Errorf("expected Name 'order/created', got %q", result.Data[1].Name)
	}
	if len(result.Data[1].Recent) != 0 {
		t.Errorf("expected 0 recent events, got %d", len(result.Data[1].Recent))
	}
}

func TestListEventsError(t *testing.T) {
	response := testUnauthorizedResp

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	result, err := client.ListEvents(context.Background(), ListEventsOptions{})
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected error to contain 'unauthorized', got: %v", err)
	}
}

func TestGetEvent(t *testing.T) {
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "user/signup",
						"recent": [
							{
								"id": "evt-1",
								"occurredAt": "2025-01-01T00:00:00Z",
								"receivedAt": "2025-01-01T00:00:01Z",
								"name": "user/signup",
								"event": "{\"name\":\"user/signup\",\"data\":{\"email\":\"test@example.com\"}}",
								"functionRuns": [
									{
										"id": "run-1",
										"status": "COMPLETED",
										"function": {"id": "fn-1", "name": "Send Welcome Email", "slug": "send-welcome-email"}
									},
									{
										"id": "run-2",
										"status": "FAILED",
										"function": {"id": "fn-2", "name": "Create Profile", "slug": "create-profile"}
									}
								]
							}
						]
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

	// Verify response.
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.ID != testEvtID1 {
		t.Errorf("expected ID 'evt-1', got %q", event.ID)
	}
	if event.Name != testTriggerUserSignup {
		t.Errorf("expected Name 'user/signup', got %q", event.Name)
	}
	if event.OccurredAt == nil {
		t.Error("expected OccurredAt to be non-nil")
	}
	if event.ReceivedAt == nil {
		t.Error("expected ReceivedAt to be non-nil")
	}
	if len(event.FunctionRuns) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(event.FunctionRuns))
	}

	if event.FunctionRuns[0].ID != testRunID1Events {
		t.Errorf("expected run[0] ID 'run-1', got %q", event.FunctionRuns[0].ID)
	}
	if event.FunctionRuns[0].Status != testStatusCompleted {
		t.Errorf("expected run[0] status 'COMPLETED', got %q", event.FunctionRuns[0].Status)
	}
	if event.FunctionRuns[0].Function == nil || event.FunctionRuns[0].Function.Name != "Send Welcome Email" {
		t.Errorf("expected run[0] function name 'Send Welcome Email'")
	}

	if event.FunctionRuns[1].ID != testRunID2 {
		t.Errorf("expected run[1] ID 'run-2', got %q", event.FunctionRuns[1].ID)
	}
	if event.FunctionRuns[1].Status != "FAILED" {
		t.Errorf("expected run[1] status 'FAILED', got %q", event.FunctionRuns[1].Status)
	}
}

func TestGetEventNotFound(t *testing.T) {
	response := `{"data": {"events": {"data": []}}}`

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
		if accept := r.Header.Get("Accept"); accept != testApplicationJSON {
			t.Errorf("expected Accept application/json, got %s", accept)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
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

	if runs[0].ID != testRunID1Events {
		t.Errorf("runs[0].ID = %s, want run-1", runs[0].ID)
	}
	if runs[0].Status != testStatusCompleted {
		t.Errorf("runs[0].Status = %s, want COMPLETED", runs[0].Status)
	}
	if runs[0].FunctionID != testFnID1 {
		t.Errorf("runs[0].FunctionID = %s, want fn-1", runs[0].FunctionID)
	}
	if runs[0].Output != `{"result": true}` {
		t.Errorf("runs[0].Output = %s, want {\"result\": true}", runs[0].Output)
	}

	if runs[1].ID != testRunID2 {
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
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
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
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
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
		json.NewEncoder(w).Encode(map[string]any{
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

func TestListEvents_NameFilter(t *testing.T) {
	response := testEmptyEventsResp

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListEvents(context.Background(), ListEventsOptions{
		Name: "user/signup",
	})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	name, ok := captured.Variables["name"]
	if !ok {
		t.Fatal("expected 'name' key in variables")
	}
	if name != "user/signup" {
		t.Errorf("expected name=%q, got %q", "user/signup", name)
	}
}

func TestListEvents_RecentCountDefaultsTo5(t *testing.T) {
	response := testEmptyEventsResp

	var captured graphqlRequest
	srv := newTestServer(t, response, &captured)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	_, err := client.ListEvents(context.Background(), ListEventsOptions{
		RecentCount: 0, // should default to 5
	})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	rc, ok := captured.Variables["recentCount"].(float64)
	if !ok {
		t.Fatalf("expected recentCount to be a number, got %T", captured.Variables["recentCount"])
	}
	if int(rc) != 5 {
		t.Errorf("expected recentCount=5, got %d", int(rc))
	}
}

func TestListEvents_RecentRunWithNilFunction(t *testing.T) {
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"name": "test/event",
								"functionRuns": [
									{
										"id": "run-1",
										"status": "RUNNING"
									}
								]
							}
						]
					}
				],
				"page": {"page": 1, "perPage": 20, "totalItems": 1, "totalPages": 1}
			}
		}
	}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	result, err := client.ListEvents(context.Background(), ListEventsOptions{})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	if len(result.Data) != 1 {
		t.Fatalf("expected 1 event type, got %d", len(result.Data))
	}
	run := result.Data[0].Recent[0].FunctionRuns[0]
	if run.Function != nil {
		t.Errorf("expected nil Function for run without function field, got %+v", run.Function)
	}
	if run.ID != testRunID1Events {
		t.Errorf("expected run ID 'run-1', got %q", run.ID)
	}
}

func TestGetEvent_NilOccurredAt(t *testing.T) {
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"name": "test/event",
								"receivedAt": "2025-01-01T00:00:01Z",
								"event": "{}",
								"functionRuns": []
							}
						]
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

	if event.OccurredAt != nil {
		t.Errorf("expected nil OccurredAt when missing, got %v", event.OccurredAt)
	}
	if event.ReceivedAt == nil {
		t.Error("expected non-nil ReceivedAt")
	}
}

func TestGetEvent_RunWithNilFunction(t *testing.T) {
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"name": "test/event",
						"recent": [
							{
								"id": "evt-1",
								"name": "test/event",
								"functionRuns": [
									{
										"id": "run-1",
										"status": "RUNNING"
									}
								]
							}
						]
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

	if len(event.FunctionRuns) != 1 {
		t.Fatalf("expected 1 run, got %d", len(event.FunctionRuns))
	}
	if event.FunctionRuns[0].Function != nil {
		t.Errorf("expected nil Function, got %+v", event.FunctionRuns[0].Function)
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
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
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
