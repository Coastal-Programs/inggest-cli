package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testAppID1            = "app-1"
	testMyApp             = "My App"
	testAppURL            = "https://example.com/api/inngest"
	testFnID1             = "fn-1"
	testSendEmail         = "Send Email"
	testUnauthorizedResp  = `{"data": null, "errors": [{"message": "unauthorized"}]}`
	testTriggerTypeEvent  = "event"
	testTriggerUserSignup = "user/signup"
)

func TestListApps(t *testing.T) {
	response := `{
		"data": {
			"apps": [
				{
					"id": "app-1",
					"externalID": "ext-1",
					"name": "My App",
					"sdkLanguage": "go",
					"sdkVersion": "0.7.0",
					"framework": "gin",
					"url": "https://example.com/api/inngest",
					"checksum": "abc123",
					"error": "",
					"connected": true,
					"functionCount": 3,
					"autodiscovered": false,
					"method": "http",
					"functions": [
						{"id": "fn-1", "name": "Send Email", "slug": "send-email"},
						{"id": "fn-2", "name": "Process Order", "slug": "process-order"}
					]
				},
				{
					"id": "app-2",
					"externalID": "ext-2",
					"name": "Another App",
					"sdkLanguage": "typescript",
					"sdkVersion": "3.0.0",
					"framework": "next",
					"url": "https://other.example.com/api/inngest",
					"connected": false,
					"functionCount": 1,
					"functions": []
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

	apps, err := client.ListApps(context.Background())
	if err != nil {
		t.Fatalf("ListApps returned error: %v", err)
	}

	if !strings.Contains(captured.Query, "apps") {
		t.Errorf("expected query to contain 'apps', got: %s", captured.Query)
	}

	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// First app.
	if apps[0].ID != testAppID1 {
		t.Errorf("expected first app ID 'app-1', got %q", apps[0].ID)
	}
	if apps[0].ExternalID != "ext-1" {
		t.Errorf("expected ExternalID 'ext-1', got %q", apps[0].ExternalID)
	}
	if apps[0].Name != testMyApp {
		t.Errorf("expected Name 'My App', got %q", apps[0].Name)
	}
	if apps[0].SDKLanguage != "go" {
		t.Errorf("expected SDKLanguage 'go', got %q", apps[0].SDKLanguage)
	}
	if apps[0].SDKVersion != "0.7.0" {
		t.Errorf("expected SDKVersion '0.7.0', got %q", apps[0].SDKVersion)
	}
	if apps[0].Framework != "gin" {
		t.Errorf("expected Framework 'gin', got %q", apps[0].Framework)
	}
	if apps[0].URL != testAppURL {
		t.Errorf("expected URL 'https://example.com/api/inngest', got %q", apps[0].URL)
	}
	if !apps[0].Connected {
		t.Error("expected Connected true, got false")
	}
	if apps[0].FunctionCount != 3 {
		t.Errorf("expected FunctionCount 3, got %d", apps[0].FunctionCount)
	}
	if len(apps[0].Functions) != 2 {
		t.Fatalf("expected 2 functions on first app, got %d", len(apps[0].Functions))
	}
	if apps[0].Functions[0].ID != testFnID1 {
		t.Errorf("expected function ID 'fn-1', got %q", apps[0].Functions[0].ID)
	}
	if apps[0].Functions[0].Name != "Send Email" {
		t.Errorf("expected function Name 'Send Email', got %q", apps[0].Functions[0].Name)
	}
	if apps[0].Functions[1].Slug != "process-order" {
		t.Errorf("expected function Slug 'process-order', got %q", apps[0].Functions[1].Slug)
	}

	// Second app.
	if apps[1].ID != "app-2" {
		t.Errorf("expected second app ID 'app-2', got %q", apps[1].ID)
	}
	if apps[1].Name != "Another App" {
		t.Errorf("expected Name 'Another App', got %q", apps[1].Name)
	}
	if apps[1].Connected {
		t.Error("expected Connected false, got true")
	}
}

func TestListAppsError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "unauthorized"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	apps, err := client.ListApps(context.Background())
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if apps != nil {
		t.Errorf("expected nil apps, got %+v", apps)
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected error to contain 'unauthorized', got: %v", err)
	}
}

func TestListApps_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	apps, err := client.ListApps(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if apps != nil {
		t.Errorf("expected nil apps, got %+v", apps)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got: %v", err)
	}
}

func TestGetApp(t *testing.T) {
	response := `{
		"data": {
			"app": {
				"id": "app-1",
				"externalID": "ext-1",
				"name": "My App",
				"sdkLanguage": "go",
				"sdkVersion": "0.7.0",
				"framework": "gin",
				"url": "https://example.com/api/inngest",
				"checksum": "abc123",
				"error": "",
				"connected": true,
				"functionCount": 2,
				"method": "http",
				"functions": [
					{
						"id": "fn-1",
						"name": "Send Email",
						"slug": "send-email",
						"triggers": [{"type": "event", "value": "user/signup"}]
					},
					{
						"id": "fn-2",
						"name": "Process Order",
						"slug": "process-order",
						"triggers": [{"type": "cron", "value": "0 * * * *"}]
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

	app, err := client.GetApp(context.Background(), testAppID1)
	if err != nil {
		t.Fatalf("GetApp returned error: %v", err)
	}

	// Verify request.
	if !strings.Contains(captured.Query, "GetApp") {
		t.Errorf("expected query to contain 'GetApp', got: %s", captured.Query)
	}
	if captured.Variables == nil {
		t.Fatal("expected variables to be non-nil")
	}
	if id, ok := captured.Variables["id"].(string); !ok || id != "app-1" {
		t.Errorf("expected id variable 'app-1', got %v", captured.Variables["id"])
	}

	// Verify response.
	if app == nil {
		t.Fatal("expected non-nil app")
	}
	if app.ID != testAppID1 {
		t.Errorf("expected ID 'app-1', got %q", app.ID)
	}
	if app.Name != testMyApp {
		t.Errorf("expected Name 'My App', got %q", app.Name)
	}
	if app.SDKLanguage != "go" {
		t.Errorf("expected SDKLanguage 'go', got %q", app.SDKLanguage)
	}
	if !app.Connected {
		t.Error("expected Connected true, got false")
	}
	if app.FunctionCount != 2 {
		t.Errorf("expected FunctionCount 2, got %d", app.FunctionCount)
	}
	if len(app.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(app.Functions))
	}
	if app.Functions[0].Name != testSendEmail {
		t.Errorf("expected function Name 'Send Email', got %q", app.Functions[0].Name)
	}
	if len(app.Functions[0].Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(app.Functions[0].Triggers))
	}
	if app.Functions[0].Triggers[0].Type != testTriggerTypeEvent {
		t.Errorf("expected trigger type 'event', got %q", app.Functions[0].Triggers[0].Type)
	}
	if app.Functions[0].Triggers[0].Value != testTriggerUserSignup {
		t.Errorf("expected trigger value 'user/signup', got %q", app.Functions[0].Triggers[0].Value)
	}
}

func TestGetAppNotFound(t *testing.T) {
	response := `{"data": {"app": null}}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	app, err := client.GetApp(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nil app, got nil")
	}
	if app != nil {
		t.Errorf("expected nil app, got %+v", app)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to contain 'nonexistent', got %q", err.Error())
	}
}

func TestGetApp_GraphQLError(t *testing.T) {
	response := `{"data": null, "errors": [{"message": "internal server error"}]}`

	srv := newTestServer(t, response, nil)
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-key",
		APIBaseURL: srv.URL,
	})

	app, err := client.GetApp(context.Background(), testAppID1)
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if app != nil {
		t.Errorf("expected nil app, got %+v", app)
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error to contain 'internal server error', got: %v", err)
	}
}
