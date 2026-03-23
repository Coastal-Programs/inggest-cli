package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListFunctions(t *testing.T) {
	response := `{
		"data": {
			"functions": [
				{
					"id": "fn-1",
					"name": "Send Email",
					"slug": "send-email",
					"appID": "app-1",
					"triggers": [{"type": "event", "value": "user/signup"}]
				},
				{
					"id": "fn-2",
					"name": "Process Order",
					"slug": "process-order",
					"appID": "app-1",
					"triggers": [{"type": "cron", "value": "0 * * * *"}]
				}
			]
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
	if !strings.Contains(captured.Query, "functions") {
		t.Errorf("expected query to contain 'functions', got: %s", captured.Query)
	}
	if captured.Variables != nil {
		t.Errorf("expected nil variables for ListFunctions, got: %v", captured.Variables)
	}

	// Verify the returned functions.
	if len(fns) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(fns))
	}

	// First function.
	if fns[0].ID != "fn-1" {
		t.Errorf("expected first function ID 'fn-1', got %q", fns[0].ID)
	}
	if fns[0].Name != "Send Email" {
		t.Errorf("expected first function Name 'Send Email', got %q", fns[0].Name)
	}
	if fns[0].Slug != "send-email" {
		t.Errorf("expected first function Slug 'send-email', got %q", fns[0].Slug)
	}
	if fns[0].AppID != "app-1" {
		t.Errorf("expected first function AppID 'app-1', got %q", fns[0].AppID)
	}
	if len(fns[0].Triggers) != 1 {
		t.Fatalf("expected 1 trigger on first function, got %d", len(fns[0].Triggers))
	}
	if fns[0].Triggers[0].Type != "event" {
		t.Errorf("expected trigger type 'event', got %q", fns[0].Triggers[0].Type)
	}
	if fns[0].Triggers[0].Value != "user/signup" {
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
	if fns[1].AppID != "app-1" {
		t.Errorf("expected second function AppID 'app-1', got %q", fns[1].AppID)
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
	response := `{"data": {"functions": []}}`

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
			"functionBySlug": {
				"id": "fn-1",
				"name": "Send Email",
				"slug": "send-email",
				"appID": "app-1",
				"url": "https://example.com/api/inngest",
				"config": "{\"id\":\"send-email\"}",
				"concurrency": 5,
				"triggers": [
					{"type": "event", "value": "user/signup", "condition": "event.data.active == true"}
				],
				"configuration": {
					"retries": {"value": 3, "isDefault": false}
				},
				"app": {
					"id": "app-1",
					"name": "My App",
					"sdkLanguage": "go",
					"sdkVersion": "0.7.0",
					"framework": "gin",
					"url": "https://example.com/api/inngest",
					"connected": true
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

	fn, err := client.GetFunction(context.Background(), "send-email")
	if err != nil {
		t.Fatalf("GetFunction returned error: %v", err)
	}

	// Verify the request contained the slug variable.
	if !strings.Contains(captured.Query, "functionBySlug") {
		t.Errorf("expected query to contain 'functionBySlug', got: %s", captured.Query)
	}
	if captured.Variables == nil {
		t.Fatal("expected variables to be non-nil")
	}
	if slug, ok := captured.Variables["slug"]; !ok {
		t.Error("expected 'slug' variable in request")
	} else if slug != "send-email" {
		t.Errorf("expected slug variable 'send-email', got %v", slug)
	}

	// Verify all fields on the returned function.
	if fn == nil {
		t.Fatal("expected non-nil function")
	}
	if fn.ID != "fn-1" {
		t.Errorf("expected ID 'fn-1', got %q", fn.ID)
	}
	if fn.Name != "Send Email" {
		t.Errorf("expected Name 'Send Email', got %q", fn.Name)
	}
	if fn.Slug != "send-email" {
		t.Errorf("expected Slug 'send-email', got %q", fn.Slug)
	}
	if fn.AppID != "app-1" {
		t.Errorf("expected AppID 'app-1', got %q", fn.AppID)
	}
	if fn.URL != "https://example.com/api/inngest" {
		t.Errorf("expected URL 'https://example.com/api/inngest', got %q", fn.URL)
	}
	if fn.Config != `{"id":"send-email"}` {
		t.Errorf("expected Config '{\"id\":\"send-email\"}', got %q", fn.Config)
	}
	if fn.Concurrency != 5 {
		t.Errorf("expected Concurrency 5, got %d", fn.Concurrency)
	}

	// Triggers.
	if len(fn.Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(fn.Triggers))
	}
	trigger := fn.Triggers[0]
	if trigger.Type != "event" {
		t.Errorf("expected trigger Type 'event', got %q", trigger.Type)
	}
	if trigger.Value != "user/signup" {
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
	if fn.App.ID != "app-1" {
		t.Errorf("expected App.ID 'app-1', got %q", fn.App.ID)
	}
	if fn.App.Name != "My App" {
		t.Errorf("expected App.Name 'My App', got %q", fn.App.Name)
	}
	if fn.App.SDKLanguage != "go" {
		t.Errorf("expected App.SDKLanguage 'go', got %q", fn.App.SDKLanguage)
	}
	if fn.App.SDKVersion != "0.7.0" {
		t.Errorf("expected App.SDKVersion '0.7.0', got %q", fn.App.SDKVersion)
	}
	if fn.App.Framework != "gin" {
		t.Errorf("expected App.Framework 'gin', got %q", fn.App.Framework)
	}
	if fn.App.URL != "https://example.com/api/inngest" {
		t.Errorf("expected App.URL 'https://example.com/api/inngest', got %q", fn.App.URL)
	}
	if fn.App.Connected != true {
		t.Errorf("expected App.Connected true, got %v", fn.App.Connected)
	}
}

func TestGetFunction_NotFound(t *testing.T) {
	response := `{"data": {"functionBySlug": null}}`

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
