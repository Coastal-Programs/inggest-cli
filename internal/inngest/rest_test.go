package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetREST_NilResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "some value"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "test-key",
	})

	// Passing nil result should succeed without error — the body is read but not unmarshaled.
	err := client.GetREST(context.Background(), "test/path", nil)
	if err != nil {
		t.Fatalf("expected no error with nil result, got: %v", err)
	}
}

func TestGetREST_UnmarshalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "test-key",
	})

	var result map[string]any
	err := client.GetREST(context.Background(), "test/path", &result)
	if err == nil {
		t.Fatal("expected error for invalid JSON body, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected error to contain 'unmarshal', got: %v", err)
	}
}

func TestGetREST_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "some value"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		APIBaseURL: srv.URL,
		SigningKey: "test-key",
	})
	client.httpClient.Transport = &errBodyTransport{wrapped: srv.Client().Transport}

	var result map[string]any
	err := client.GetREST(context.Background(), "test/path", &result)
	if err == nil {
		t.Fatal("expected error when response body read fails, got nil")
	}
	if !strings.Contains(err.Error(), "read GET response") {
		t.Errorf("expected error to contain 'read GET response', got: %v", err)
	}
}

func TestGetREST_NewRequestError(t *testing.T) {
	client := NewClient(ClientOptions{APIBaseURL: "http://invalid\x00host"})
	err := client.GetREST(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "create GET request") {
		t.Errorf("expected 'create GET request' error, got: %v", err)
	}
}
