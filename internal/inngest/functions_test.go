package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testSendEmailSlug   = "send-email"
	testExternalIDMyApp = "my-app"
)

func TestListFunctions(t *testing.T) {
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"workflows": [
							{
								"id": "fn-1",
								"name": "Send Email",
								"slug": "send-email",
								"isPaused": false,
								"isArchived": false,
								"triggers": [{"type": "event", "value": "user/signup"}],
								"app": {"id": "app-1", "name": "My App", "externalID": "my-app"}
							},
							{
								"id": "fn-2",
								"name": "Process Order",
								"slug": "process-order",
								"isPaused": false,
								"isArchived": false,
								"triggers": [{"type": "cron", "value": "0 * * * *"}],
								"app": {"id": "app-1", "name": "My App", "externalID": "my-app"}
							}
						]
					},
					{
						"workflows": [
							{
								"id": "fn-1",
								"name": "Send Email",
								"slug": "send-email",
								"isPaused": false,
								"isArchived": false,
								"triggers": [{"type": "event", "value": "user/signup"}],
								"app": {"id": "app-1", "name": "My App", "externalID": "my-app"}
							}
						]
					}
				],
				"page": {"page": 1, "totalPages": 1}
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

	fns, err := client.ListFunctions(context.Background())
	if err != nil {
		t.Fatalf("ListFunctions returned error: %v", err)
	}

	// Verify the request contained the expected query.
	if !strings.Contains(captured.Query, "events") {
		t.Errorf("expected query to contain 'events', got: %s", captured.Query)
	}
	if !strings.Contains(captured.Query, "workflows") {
		t.Errorf("expected query to contain 'workflows', got: %s", captured.Query)
	}

	// Verify deduplication: fn-1 appears in two event types but should be returned once.
	if len(fns) != 2 {
		t.Fatalf("expected 2 functions (deduplicated), got %d", len(fns))
	}

	// First function.
	if fns[0].ID != testFnID1 {
		t.Errorf("expected first function ID 'fn-1', got %q", fns[0].ID)
	}
	if fns[0].Name != testSendEmail {
		t.Errorf("expected first function Name 'Send Email', got %q", fns[0].Name)
	}
	if fns[0].Slug != "send-email" {
		t.Errorf("expected first function Slug 'send-email', got %q", fns[0].Slug)
	}
	if fns[0].App == nil || fns[0].App.ID != testAppID1 {
		t.Errorf("expected first function App.ID 'app-1', got %v", fns[0].App)
	}
	if len(fns[0].Triggers) != 1 {
		t.Fatalf("expected 1 trigger on first function, got %d", len(fns[0].Triggers))
	}
	if fns[0].Triggers[0].Type != "event" {
		t.Errorf("expected trigger type 'event', got %q", fns[0].Triggers[0].Type)
	}
	if fns[0].Triggers[0].Value != testTriggerUserSignup {
		t.Errorf("expected trigger value 'user/signup', got %q", fns[0].Triggers[0].Value)
	}

	// Second function.
	if fns[1].ID != "fn-2" {
		t.Errorf("expected second function ID 'fn-2', got %q", fns[1].ID)
	}
	if fns[1].Name != "Process Order" {
		t.Errorf("expected second function Name 'Process Order', got %q", fns[1].Name)
	}
	if fns[1].Slug != "process-order" {
		t.Errorf("expected second function Slug 'process-order', got %q", fns[1].Slug)
	}
	if len(fns[1].Triggers) != 1 {
		t.Fatalf("expected 1 trigger on second function, got %d", len(fns[1].Triggers))
	}
	if fns[1].Triggers[0].Type != "cron" {
		t.Errorf("expected trigger type 'cron', got %q", fns[1].Triggers[0].Type)
	}
	if fns[1].Triggers[0].Value != "0 * * * *" {
		t.Errorf("expected trigger value '0 * * * *', got %q", fns[1].Triggers[0].Value)
	}
}

func TestListFunctions_Empty(t *testing.T) {
	response := `{"data": {"events": {"data": [], "page": {"page": 1, "totalPages": 0}}}}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	fns, err := client.ListFunctions(context.Background())
	if err != nil {
		t.Fatalf("ListFunctions returned error: %v", err)
	}
	if fns == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(fns) != 0 {
		t.Errorf("expected 0 functions, got %d", len(fns))
	}
}

func TestGetFunction(t *testing.T) {
	response := `{
		"data": {
			"events": {
				"data": [
					{
						"workflows": [
							{
								"id": "fn-1",
								"name": "Send Email",
								"slug": "send-email",
								"url": "https://example.com/api/inngest",
								"isPaused": false,
								"isArchived": false,
								"triggers": [
									{"type": "event", "value": "user/signup", "condition": "event.data.active == true"}
								],
								"configuration": {
									"retries": {"value": 3, "isDefault": false}
								},
								"app": {
									"id": "app-1",
									"name": "My App",
									"externalID": "my-app",
									"appVersion": "1.0.0"
								}
							}
						]
					}
				],
				"page": {"page": 1, "totalPages": 1}
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

	fn, err := client.GetFunction(context.Background(), "send-email")
	if err != nil {
		t.Fatalf("GetFunction returned error: %v", err)
	}

	// Verify the request uses the events/workflows pattern.
	if !strings.Contains(captured.Query, "events") {
		t.Errorf("expected query to contain 'events', got: %s", captured.Query)
	}

	// Verify all fields on the returned function.
	if fn == nil {
		t.Fatal("expected non-nil function")
	}
	if fn.ID != testFnID1 {
		t.Errorf("expected ID 'fn-1', got %q", fn.ID)
	}
	if fn.Name != testSendEmail {
		t.Errorf("expected Name 'Send Email', got %q", fn.Name)
	}
	if fn.Slug != testSendEmailSlug {
		t.Errorf("expected Slug 'send-email', got %q", fn.Slug)
	}
	if fn.URL != testAppURL {
		t.Errorf("expected URL 'https://example.com/api/inngest', got %q", fn.URL)
	}
	if fn.IsPaused {
		t.Errorf("expected IsPaused false, got true")
	}
	if fn.IsArchived {
		t.Errorf("expected IsArchived false, got true")
	}

	// Triggers.
	if len(fn.Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(fn.Triggers))
	}
	trigger := fn.Triggers[0]
	if trigger.Type != "event" {
		t.Errorf("expected trigger Type 'event', got %q", trigger.Type)
	}
	if trigger.Value != testTriggerUserSignup {
		t.Errorf("expected trigger Value 'user/signup', got %q", trigger.Value)
	}
	if trigger.Condition != "event.data.active == true" {
		t.Errorf("expected trigger Condition 'event.data.active == true', got %q", trigger.Condition)
	}

	// Configuration.
	if fn.Configuration == nil {
		t.Fatal("expected non-nil Configuration")
	}
	if fn.Configuration.Retries == nil {
		t.Fatal("expected non-nil Retries in Configuration")
	}
	if fn.Configuration.Retries.Value != 3 {
		t.Errorf("expected Retries.Value 3, got %d", fn.Configuration.Retries.Value)
	}
	if fn.Configuration.Retries.IsDefault != false {
		t.Errorf("expected Retries.IsDefault false, got %v", fn.Configuration.Retries.IsDefault)
	}

	// App.
	if fn.App == nil {
		t.Fatal("expected non-nil App")
	}
	if fn.App.ID != testAppID1 {
		t.Errorf("expected App.ID 'app-1', got %q", fn.App.ID)
	}
	if fn.App.Name != testMyApp {
		t.Errorf("expected App.Name 'My App', got %q", fn.App.Name)
	}
	if fn.App.ExternalID != testExternalIDMyApp {
		t.Errorf("expected App.ExternalID %q, got %q", testExternalIDMyApp, fn.App.ExternalID)
	}
	if fn.App.AppVersion != "1.0.0" {
		t.Errorf("expected App.AppVersion '1.0.0', got %q", fn.App.AppVersion)
	}
}

func TestGetFunction_NotFound(t *testing.T) {
	response := `{"data": {"events": {"data": [], "page": {"page": 1, "totalPages": 0}}}}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	fn, err := client.GetFunction(context.Background(), "nonexistent-fn")
	if err == nil {
		t.Fatal("expected error for not-found function, got nil")
	}
	if fn != nil {
		t.Errorf("expected nil function, got %+v", fn)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent-fn") {
		t.Errorf("expected error to contain slug 'nonexistent-fn', got: %v", err)
	}
}

func TestListFunctions_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "unauthorized"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	fns, err := client.ListFunctions(context.Background())
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if fns != nil {
		t.Errorf("expected nil functions, got %+v", fns)
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected error to contain 'unauthorized', got: %v", err)
	}
}

func TestGetFunction_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "internal server error"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	fn, err := client.GetFunction(context.Background(), "send-email")
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if fn != nil {
		t.Errorf("expected nil function, got %+v", fn)
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error to contain 'internal server error', got: %v", err)
	}
}

func TestListFunctions_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	fns, err := client.ListFunctions(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if fns != nil {
		t.Errorf("expected nil functions, got %+v", fns)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got: %v", err)
	}
}

func TestGetFunction_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	fn, err := client.GetFunction(context.Background(), "send-email")
	if err == nil {
		t.Fatal("expected error for HTTP 502, got nil")
	}
	if fn != nil {
		t.Errorf("expected nil function, got %+v", fn)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected error to contain '502', got: %v", err)
	}
}
