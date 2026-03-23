package inngest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteGraphQL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"functions": [{"id": "fn-1", "name": "test-fn"}]}}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	type function struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var result struct {
		Functions []function `json:"functions"`
	}

	err := client.ExecuteGraphQL(context.Background(), "", `{ functions { id name } }`, nil, &result)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Functions))
	}
	if result.Functions[0].ID != "fn-1" {
		t.Errorf("expected function ID %q, got %q", "fn-1", result.Functions[0].ID)
	}
	if result.Functions[0].Name != "test-fn" {
		t.Errorf("expected function name %q, got %q", "test-fn", result.Functions[0].Name)
	}
}

func TestExecuteGraphQL_GraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": null, "errors": [{"message": "not authorized"}, {"message": "bad query"}]}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	var result json.RawMessage
	err := client.ExecuteGraphQL(context.Background(), "", `{ secret }`, nil, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "not authorized") {
		t.Errorf("expected error to contain %q, got: %s", "not authorized", errMsg)
	}
	if !strings.Contains(errMsg, "bad query") {
		t.Errorf("expected error to contain %q, got: %s", "bad query", errMsg)
	}
}

func TestExecuteGraphQL_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	var result json.RawMessage
	err := client.ExecuteGraphQL(context.Background(), "", `{ something }`, nil, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "500") {
		t.Errorf("expected error to contain %q, got: %s", "500", errMsg)
	}
}

func TestExecuteGraphQL_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	var result json.RawMessage
	err := client.ExecuteGraphQL(context.Background(), "", `{ something }`, nil, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "unmarshal") {
		t.Errorf("expected error to contain %q, got: %s", "unmarshal", errMsg)
	}
}

func TestExecuteGraphQL_AuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-signing-key" {
			t.Errorf("expected Authorization header %q, got %q", "Bearer test-signing-key", auth)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"errors": [{"message": "unauthorized"}]}`))
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type header %q, got %q", "application/json", contentType)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"ok": true}}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	var result struct {
		OK bool `json:"ok"`
	}
	err := client.ExecuteGraphQL(context.Background(), "", `{ ok }`, nil, &result)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.OK {
		t.Error("expected result.OK to be true")
	}
}

func TestExecuteGraphQL_ContentType(t *testing.T) {
	var receivedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": null}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	err := client.ExecuteGraphQL(context.Background(), "", `{ ping }`, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", receivedContentType)
	}
}

func TestExecuteGraphQL_RequestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/v0/gql" {
			t.Errorf("expected path /v0/gql, got %s", r.URL.Path)
		}

		var reqBody graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if reqBody.Query != "query GetFn($id: ID!) { function(id: $id) { name } }" {
			t.Errorf("unexpected query: %s", reqBody.Query)
		}
		if reqBody.Variables["id"] != "fn-123" {
			t.Errorf("expected variable id=%q, got %v", "fn-123", reqBody.Variables["id"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"function": {"name": "my-func"}}}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	variables := map[string]interface{}{
		"id": "fn-123",
	}

	var result struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}

	err := client.ExecuteGraphQL(
		context.Background(),
		"GetFn",
		"query GetFn($id: ID!) { function(id: $id) { name } }",
		variables,
		&result,
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Function.Name != "my-func" {
		t.Errorf("expected function name %q, got %q", "my-func", result.Function.Name)
	}
}

func TestExecuteGraphQL_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"ok": true}}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var result struct {
		OK bool `json:"ok"`
	}
	err := client.ExecuteGraphQL(ctx, "", `{ ok }`, nil, &result)
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "graphql request") {
		t.Errorf("expected error to contain 'graphql request', got: %v", err)
	}
}

func TestExecuteGraphQL_DataUnmarshalError(t *testing.T) {
	// Return data that is a string instead of an object — this should fail
	// when unmarshaling into a struct result.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "not-an-object"}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	var result struct {
		Functions []struct {
			ID string `json:"id"`
		} `json:"functions"`
	}
	err := client.ExecuteGraphQL(context.Background(), "", `{ functions { id } }`, nil, &result)
	if err == nil {
		t.Fatal("expected error when data doesn't match target type, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal graphql data") {
		t.Errorf("expected error to contain 'unmarshal graphql data', got: %v", err)
	}
}

func TestExecuteGraphQL_NilResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"something": true}}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})

	err := client.ExecuteGraphQL(context.Background(), "", `{ something }`, nil, nil)
	if err != nil {
		t.Fatalf("expected no error when result is nil, got: %v", err)
	}
}

func TestExecuteGraphQL_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"ok": true}}`))
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{
		SigningKey: "test-signing-key",
		APIBaseURL: srv.URL,
	})
	client.httpClient.Transport = &errBodyTransport{wrapped: srv.Client().Transport}

	var result struct {
		OK bool `json:"ok"`
	}
	err := client.ExecuteGraphQL(context.Background(), "", `{ ok }`, nil, &result)
	if err == nil {
		t.Fatal("expected error when response body read fails, got nil")
	}
	if !strings.Contains(err.Error(), "read graphql response") {
		t.Errorf("expected error to contain 'read graphql response', got: %v", err)
	}
}
