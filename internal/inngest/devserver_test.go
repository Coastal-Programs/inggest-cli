package inngest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsDevServerRunning_Running(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/dev" {
			t.Errorf("expected path /dev, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx := context.Background()
	if !client.IsDevServerRunning(ctx) {
		t.Fatal("expected IsDevServerRunning to return true when server responds 200")
	}
}

func TestIsDevServerRunning_NotRunning(t *testing.T) {
	// Create a server and immediately close it so the port is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: closedURL,
	})

	ctx := context.Background()
	if client.IsDevServerRunning(ctx) {
		t.Fatal("expected IsDevServerRunning to return false when server is not reachable")
	}
}

func TestIsDevServerRunning_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx := context.Background()
	if client.IsDevServerRunning(ctx) {
		t.Fatal("expected IsDevServerRunning to return false when server responds 500")
	}
}

func TestGetDevInfo(t *testing.T) {
	respPayload := DevServerInfo{
		Version: "0.27.0",
		Functions: []Function{
			{
				ID:   "fn1",
				Name: "my-func",
				Slug: "my-func",
			},
		},
		EventKeyHash: "abc123",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/dev" {
			t.Errorf("expected path /dev, got %s", r.URL.Path)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Errorf("expected Accept: application/json, got %s", accept)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(respPayload)
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx := context.Background()
	info, err := client.GetDevInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Version != "0.27.0" {
		t.Errorf("expected version 0.27.0, got %s", info.Version)
	}
	if info.EventKeyHash != "abc123" {
		t.Errorf("expected eventKeyHash abc123, got %s", info.EventKeyHash)
	}
	if len(info.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(info.Functions))
	}
	if info.Functions[0].ID != "fn1" {
		t.Errorf("expected function ID fn1, got %s", info.Functions[0].ID)
	}
	if info.Functions[0].Name != "my-func" {
		t.Errorf("expected function name my-func, got %s", info.Functions[0].Name)
	}
	if info.Functions[0].Slug != "my-func" {
		t.Errorf("expected function slug my-func, got %s", info.Functions[0].Slug)
	}
}

func TestGetDevInfo_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx := context.Background()
	_, err := client.GetDevInfo(ctx)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestSendDevEvent(t *testing.T) {
	eventPayload := map[string]interface{}{
		"name": "test/event.sent",
		"data": map[string]interface{}{
			"userId": "user-123",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/e/test-key" {
			t.Errorf("expected path /e/test-key, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		var received map[string]interface{}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		if received["name"] != "test/event.sent" {
			t.Errorf("expected event name test/event.sent, got %v", received["name"])
		}
		data, ok := received["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected data to be a map, got %T", received["data"])
		}
		if data["userId"] != "user-123" {
			t.Errorf("expected userId user-123, got %v", data["userId"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":    []string{"evt-001", "evt-002"},
			"status": 200,
		})
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		DevMode:      true,
		EventKey:     "test-key",
	})

	ctx := context.Background()
	ids, err := client.SendDevEvent(ctx, eventPayload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
	if ids[0] != "evt-001" {
		t.Errorf("expected first ID evt-001, got %s", ids[0])
	}
	if ids[1] != "evt-002" {
		t.Errorf("expected second ID evt-002, got %s", ids[1])
	}
}

func TestSendDevEvent_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		DevMode:      true,
		EventKey:     "test-key",
	})

	ctx := context.Background()
	_, err := client.SendDevEvent(ctx, map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
}

func TestInvokeDevFunction(t *testing.T) {
	invokeData := map[string]interface{}{
		"event": map[string]interface{}{
			"name": "test/invoke",
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/invoke/my-function" {
			t.Errorf("expected path /invoke/my-function, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		var received map[string]interface{}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		if _, ok := received["event"]; !ok {
			t.Error("expected 'event' key in request body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "run-001",
			"status": 200,
		})
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx := context.Background()
	id, err := client.InvokeDevFunction(ctx, "my-function", invokeData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != "run-001" {
		t.Errorf("expected run ID run-001, got %s", id)
	}
}

func TestInvokeDevFunction_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"function not found"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx := context.Background()
	_, err := client.InvokeDevFunction(ctx, "nonexistent-function", nil)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestGetDevInfo_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx := context.Background()
	_, err := client.GetDevInfo(ctx)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSendDevEvent_EmptyIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":    []string{},
			"status": 200,
		})
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		DevMode:      true,
		EventKey:     "test-key",
	})

	ctx := context.Background()
	ids, err := client.SendDevEvent(ctx, map[string]string{"name": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs, got %d", len(ids))
	}
}

func TestSendDevEvent_EmptyEventKeyFallbackToTest(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":    []string{"evt-1"},
			"status": 200,
		})
	}))
	defer srv.Close()

	// Client with empty EventKey — should fall back to "test".
	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		DevMode:      true,
		EventKey:     "", // empty
	})

	ctx := context.Background()
	ids, err := client.SendDevEvent(ctx, map[string]string{"name": "test/event"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPath != "/e/test" {
		t.Errorf("expected path /e/test, got %s", capturedPath)
	}
	if len(ids) != 1 || ids[0] != "evt-1" {
		t.Errorf("unexpected ids: %v", ids)
	}
}

func TestGetDevInfo_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetDevInfo(ctx)
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestSendDevEvent_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ids":[],"status":200}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		DevMode:      true,
		EventKey:     "test-key",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.SendDevEvent(ctx, map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestInvokeDevFunction_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"run-1","status":200}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.InvokeDevFunction(ctx, "my-func", nil)
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestIsDevServerRunning_UserAgentHeader(t *testing.T) {
	var receivedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		UserAgent:    "my-cli/1.0",
	})

	ctx := context.Background()
	client.IsDevServerRunning(ctx)

	if receivedUA != "my-cli/1.0" {
		t.Errorf("expected User-Agent my-cli/1.0, got %s", receivedUA)
	}
}

func TestGetDevInfo_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"version":"0.1.0"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})
	client.httpClient.Transport = &errBodyTransport{wrapped: srv.Client().Transport}

	_, err := client.GetDevInfo(context.Background())
	if err == nil {
		t.Fatal("expected error when response body read fails, got nil")
	}
	if !strings.Contains(err.Error(), "read dev info response") {
		t.Errorf("expected error to contain 'read dev info response', got: %v", err)
	}
}

func TestSendDevEvent_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ids":["evt-1"],"status":200}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		EventKey:     "test-key",
	})
	client.httpClient.Transport = &errBodyTransport{wrapped: srv.Client().Transport}

	_, err := client.SendDevEvent(context.Background(), map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error when response body read fails, got nil")
	}
	if !strings.Contains(err.Error(), "read send event response") {
		t.Errorf("expected error to contain 'read send event response', got: %v", err)
	}
}

func TestInvokeDevFunction_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"run-1","status":200}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})
	client.httpClient.Transport = &errBodyTransport{wrapped: srv.Client().Transport}

	_, err := client.InvokeDevFunction(context.Background(), "my-func", map[string]string{"key": "val"})
	if err == nil {
		t.Fatal("expected error when response body read fails, got nil")
	}
	if !strings.Contains(err.Error(), "read invoke response") {
		t.Errorf("expected error to contain 'read invoke response', got: %v", err)
	}
}

func TestSendDevEvent_InvalidJSON_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
		EventKey:     "test-key",
	})

	_, err := client.SendDevEvent(context.Background(), map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal send event response") {
		t.Errorf("expected error to contain 'unmarshal send event response', got: %v", err)
	}
}

func TestInvokeDevFunction_InvalidJSON_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		DevServerURL: srv.URL,
	})

	_, err := client.InvokeDevFunction(context.Background(), "my-func", map[string]string{"key": "val"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal invoke response") {
		t.Errorf("expected error to contain 'unmarshal invoke response', got: %v", err)
	}
}

func TestSendDevEvent_MarshalError(t *testing.T) {
	client := NewClient(ClientOptions{
		DevServerURL: "http://localhost:0",
		EventKey:     "test-key",
	})

	// Channels cannot be marshalled to JSON.
	_, err := client.SendDevEvent(context.Background(), make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable event data, got nil")
	}
	if !strings.Contains(err.Error(), "marshal event data") {
		t.Errorf("expected error to contain 'marshal event data', got: %v", err)
	}
}

func TestInvokeDevFunction_MarshalError(t *testing.T) {
	client := NewClient(ClientOptions{
		DevServerURL: "http://localhost:0",
	})

	// Channels cannot be marshalled to JSON.
	_, err := client.InvokeDevFunction(context.Background(), "my-func", make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable invoke data, got nil")
	}
	if !strings.Contains(err.Error(), "marshal invoke data") {
		t.Errorf("expected error to contain 'marshal invoke data', got: %v", err)
	}
}
