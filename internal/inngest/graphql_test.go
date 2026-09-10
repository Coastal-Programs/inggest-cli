package inngest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	if result.Functions[0].ID != testFnID1 {
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
		if contentType != testApplicationJSON {
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

	if receivedContentType != testApplicationJSON {
		t.Errorf("expected Content-Type %q, got %q", "application/json", receivedContentType)
	}
}

func TestExecuteGraphQL_RequestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/gql" {
			t.Errorf("expected path /gql, got %s", r.URL.Path)
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

	variables := map[string]any{
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
	if result.Function.Name != testMyFunc {
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

func TestExecuteGraphQL_MarshalError(t *testing.T) {
	client := NewClient(ClientOptions{})
	vars := map[string]any{"bad": make(chan int)}
	err := client.ExecuteGraphQL(context.Background(), "Test", "query {}", vars, nil)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal graphql request") {
		t.Errorf("expected 'marshal graphql request' error, got: %v", err)
	}
}

func TestExecuteGraphQL_NewRequestError(t *testing.T) {
	client := NewClient(ClientOptions{APIBaseURL: "http://invalid\x00host"})
	err := client.ExecuteGraphQL(context.Background(), "Test", "query {}", nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "create graphql request") {
		t.Errorf("expected 'create graphql request' error, got: %v", err)
	}
}

// TestDevGraphQLOperations_TransportErrors covers the error branch of every
// dev-mode GraphQL wrapper: when the dev server is unreachable, each call must
// fail with its own contextual message rather than a bare transport error.
func TestDevGraphQLOperations_TransportErrors(t *testing.T) {
	srv := newClosedServer(t)
	client := newDevClient(srv)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want string
	}{
		{
			name: "cancel run",
			call: func() error { _, err := client.CancelRun(ctx, testRunID1); return err },
			want: "cancel run " + testRunID1,
		},
		{
			name: "rerun",
			call: func() error { _, err := client.RerunRun(ctx, testRunID1); return err },
			want: "rerun " + testRunID1,
		},
		{
			name: "list functions",
			call: func() error { _, err := client.ListFunctions(ctx); return err },
			want: "list functions",
		},
		{
			name: "list apps",
			call: func() error { _, err := client.ListApps(ctx, false); return err },
			want: "list apps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrContains(t, tt.call(), tt.want)
		})
	}
}

// TestInvokeDevFunction_Errors covers the failure branches of the dev invoke
// flow: a rejected mutation, and a context cancelled while polling for the run
// the invocation should have produced.
func TestInvokeDevFunction_Errors(t *testing.T) {
	fnResp := `{"data":{"functions":[{"id":"` + testFnID1 + `","slug":"` + testSlugSend + `"}]}}`

	t.Run("dev server declines the invocation", func(t *testing.T) {
		srv, _ := newDevGQLServer(t, map[string]string{
			"DevFunctions": fnResp,
			"DevInvoke":    `{"data":{"invokeFunction":false}}`,
		})

		_, err := newDevClient(srv).InvokeDevFunction(context.Background(), testSlugSend, nil)
		requireErrContains(t, err, "did not accept the invocation")
	})

	t.Run("context cancelled while polling", func(t *testing.T) {
		srv, rec := newDevGQLServer(t, map[string]string{
			"DevFunctions": fnResp,
			"DevInvoke":    `{"data":{"invokeFunction":true}}`,
			// No run ever appears, so the caller keeps polling until the
			// context is cancelled.
			"DevRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false}}}}`,
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			_, err := newDevClient(srv).InvokeDevFunction(ctx, testSlugSend, nil)
			errCh <- err
		}()

		// Cancel once the first poll has been served so the ctx.Done() arm of
		// the poll loop is the branch that ends it.
		waitFor(t, func() bool { return rec.countOp("DevRuns") > 0 })
		cancel()

		select {
		case err := <-errCh:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("err = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("InvokeDevFunction did not return after cancellation")
		}
	})
}

// TestDevGraphQLReads_TransportErrors covers the error branch of the dev-mode
// read operations, each of which wraps the failure with its own context.
func TestDevGraphQLReads_TransportErrors(t *testing.T) {
	srv := newClosedServer(t)
	client := newDevClient(srv)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want string
	}{
		{
			name: "get run",
			call: func() error { _, err := client.GetRun(ctx, testRunID1); return err },
			want: "get run " + testRunID1,
		},
		{
			name: "get event runs",
			call: func() error { _, err := client.GetEventRuns(ctx, testEventID1); return err },
			want: "get event runs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrContains(t, tt.call(), tt.want)
		})
	}
}

// TestDevCancelRun_EchoesRunIDWhenServerOmitsIt covers the fallback branch:
// the dev server acknowledges without echoing an ID, so the requested run ID
// is returned instead of an empty string.
func TestDevCancelRun_EchoesRunIDWhenServerOmitsIt(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{
		"DevCancelRun": `{"data":{"cancelRun":{"id":""}}}`,
	})

	got, err := newDevClient(srv).CancelRun(context.Background(), testRunID1)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if got != testRunID1 {
		t.Errorf("CancelRun = %q, want the requested ID %q", got, testRunID1)
	}
}

// TestInvokeDevFunction_FunctionLookupError covers the branch where the
// function cannot be resolved, so no invocation is attempted.
func TestInvokeDevFunction_FunctionLookupError(t *testing.T) {
	srv := newClosedServer(t)

	_, err := newDevClient(srv).InvokeDevFunction(context.Background(), testSlugSend, nil)
	requireErrContains(t, err, "list functions")
}

// TestInvokeDevFunction_MutationError covers the branch where the invoke
// mutation itself fails.
func TestInvokeDevFunction_MutationError(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{
		"DevFunctions": `{"data":{"functions":[{"id":"` + testFnID1 + `","slug":"` + testSlugSend + `"}]}}`,
		"DevInvoke":    `{"errors":[{"message":"boom"}]}`,
	})

	_, err := newDevClient(srv).InvokeDevFunction(context.Background(), testSlugSend, nil)
	requireErrContains(t, err, "invoke function "+testSlugSend)
}

// TestInvokeDevFunction_ReturnsRunID covers the success path where the polled
// run appears immediately.
func TestInvokeDevFunction_ReturnsRunID(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{
		"DevFunctions": `{"data":{"functions":[{"id":"` + testFnID1 + `","slug":"` + testSlugSend + `"}]}}`,
		"DevInvoke":    `{"data":{"invokeFunction":true}}`,
		"DevRuns":      `{"data":{"runs":{"edges":[{"node":{"id":"` + testRunID1 + `","status":"RUNNING"}}],"pageInfo":{"hasNextPage":false}}}}`,
	})

	got, err := newDevClient(srv).InvokeDevFunction(context.Background(), testSlugSend, nil)
	if err != nil {
		t.Fatalf("InvokeDevFunction returned error: %v", err)
	}
	if got != testRunID1 {
		t.Errorf("InvokeDevFunction = %q, want %q", got, testRunID1)
	}
}

// TestDevListRuns_TimeFieldMapping covers the TimeField switch, which maps the
// CLI's --time-field values onto the dev server's enum.
func TestDevListRuns_TimeFieldMapping(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"startedat", "STARTED_AT"},
		{"started_at", "STARTED_AT"},
		{"endedat", "ENDED_AT"},
		{"ended_at", "ENDED_AT"},
		{"", "QUEUED_AT"},
	} {
		t.Run(tt.in, func(t *testing.T) {
			srv, rec := newDevGQLServer(t, map[string]string{
				"DevRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false}}}}`,
			})

			if _, err := newDevClient(srv).ListRuns(context.Background(), ListRunsOptions{First: 1, TimeField: tt.in}); err != nil {
				t.Fatalf("ListRuns returned error: %v", err)
			}

			vars := rec.last(t).Variables
			if got := fmt.Sprint(vars["orderBy"]); !strings.Contains(got, tt.want) {
				t.Errorf("orderBy = %v, want it to use %q", got, tt.want)
			}
		})
	}
}

// TestDevCancelRun_ReturnsServerID covers the branch where the dev server
// echoes the cancelled run ID, which is returned in preference to the input.
func TestDevCancelRun_ReturnsServerID(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{
		"DevCancelRun": `{"data":{"cancelRun":{"id":"` + testCancelledRunID + `"}}}`,
	})

	got, err := newDevClient(srv).CancelRun(context.Background(), testRunID1)
	if err != nil {
		t.Fatalf("CancelRun returned error: %v", err)
	}
	if got != testCancelledRunID {
		t.Errorf("CancelRun = %q, want the server's ID %q", got, testCancelledRunID)
	}
}

// TestInvokeDevFunction_PollErrorPropagates covers the branch where a poll for
// the newly-invoked run fails.
func TestInvokeDevFunction_PollErrorPropagates(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{
		"DevFunctions": `{"data":{"functions":[{"id":"` + testFnID1 + `","slug":"` + testSlugSend + `"}]}}`,
		"DevInvoke":    `{"data":{"invokeFunction":true}}`,
		"DevRuns":      `{"errors":[{"message":"poll exploded"}]}`,
	})

	_, err := newDevClient(srv).InvokeDevFunction(context.Background(), testSlugSend, nil)
	requireErrContains(t, err, "poll exploded")
}

// TestInvokeDevFunction_SettleTimeout covers the poll loop's exhaustion path:
// the invocation is accepted but the run never becomes visible, so the call
// returns an empty ID with a nil error once devInvokeSettleTimeout elapses.
// This also exercises the 250ms inter-poll wait.
//
// The wait is real, so the test runs in parallel to overlap with the rest of
// the package rather than adding its full duration to the suite.
func TestInvokeDevFunction_SettleTimeout(t *testing.T) {
	t.Parallel()

	srv, rec := newDevGQLServer(t, map[string]string{
		"DevFunctions": `{"data":{"functions":[{"id":"` + testFnID1 + `","slug":"` + testSlugSend + `"}]}}`,
		"DevInvoke":    `{"data":{"invokeFunction":true}}`,
		// The run never appears, so the loop polls until the deadline passes.
		"DevRuns": `{"data":{"runs":{"edges":[],"pageInfo":{"hasNextPage":false}}}}`,
	})

	runID, err := newDevClient(srv).InvokeDevFunction(context.Background(), testSlugSend, nil)
	if err != nil {
		t.Fatalf("InvokeDevFunction returned error: %v", err)
	}
	if runID != "" {
		t.Errorf("InvokeDevFunction = %q, want an empty ID when the run never settles", runID)
	}
	// Proves the 250ms wait arm ran: a single poll could not span the timeout.
	if got := rec.countOp("DevRuns"); got < 2 {
		t.Errorf("DevRuns poll count = %d, want at least 2", got)
	}
}
