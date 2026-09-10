package inngest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Shared test constants used across multiple test files in this package.
const (
	// testAPIKey is a raw API key (no "signkey-" prefix); it is sent as-is so
	// handlers can assert the exact Authorization header.
	testAPIKey          = "test-api-key"
	testBearerAPIKey    = "Bearer " + testAPIKey
	testApplicationJSON = "application/json"
	testMyFunc          = "my-func"

	testAppID1     = "app-1"
	testAppID2     = "app-2"
	testMyApp      = "My App"
	testAppURL     = "https://example.com/api/inngest"
	testFnID1      = "fn-1"
	testFnID2      = "fn-2"
	testSendEmail  = "Send Email"
	testSlugSend   = "send-email"
	testRunID1     = "01HRUN0000000000000000001"
	testRunID2     = "01HRUN0000000000000000002"
	testEventID1   = "01HEVT0000000000000000001"
	testCursor2    = "cursor-page-2"
	testStatusDone = "COMPLETED"
	testStatusFail = "FAILED"
	testEventName  = "user/signup"
	testTimeQueued = "2026-09-01T10:00:00Z"
	testTimeStart  = "2026-09-01T10:00:01Z"
	testTimeEnd    = "2026-09-01T10:00:02Z"

	testPathV2Runs   = "/v2/runs"
	testPathV2Apps   = "/v2/apps"
	testPathV2Envs   = "/v2/envs"
	testPathV2Events = "/v2/events"
	testPathDevGQL   = "/v0/gql"

	testEmptyListResp  = `{"data": [], "page": {"hasMore": false}}`
	testUnauthorized   = `{"errors": [{"code": "invalid_signing_key", "message": "invalid signing key"}]}`
	testNotFoundResp   = `{"errors": [{"code": "not_found", "message": "resource not found"}]}`
	testCancelledRunID = "cancelled-run"
)

// errBodyReader is an io.ReadCloser whose Read always returns an error.
// Used to simulate io.ReadAll failures on response bodies.
type errBodyReader struct{}

func (errBodyReader) Read(p []byte) (int, error) { return 0, fmt.Errorf("simulated read error") }
func (errBodyReader) Close() error               { return nil }

// errBodyTransport wraps a real http.RoundTripper. It makes the real HTTP
// request but replaces the response body with an errBodyReader so that any
// subsequent io.ReadAll on the body fails.
type errBodyTransport struct {
	wrapped http.RoundTripper
}

func (t *errBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.wrapped.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	resp.Body = errBodyReader{}
	return resp, nil
}

// newCloudClient returns a cloud-mode client pointed at srv, authenticated with
// a raw API key so handlers can assert "Bearer test-api-key".
func newCloudClient(srv *httptest.Server) *Client {
	return NewClient(ClientOptions{APIBaseURL: srv.URL, SigningKey: testAPIKey})
}

// newDevClient returns a dev-mode client pointed at srv.
func newDevClient(srv *httptest.Server) *Client {
	return NewClient(ClientOptions{DevServerURL: srv.URL, DevMode: true})
}

// writeJSON writes body as an application/json response with the given status.
func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", testApplicationJSON)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// requireRequest asserts the HTTP method, path and Authorization header of r.
// wantAuth "" asserts that no Authorization header was sent.
func requireRequest(t *testing.T, r *http.Request, method, path, wantAuth string) {
	t.Helper()
	if r.Method != method {
		t.Errorf("method = %s, want %s", r.Method, method)
	}
	if r.URL.Path != path {
		t.Errorf("path = %s, want %s", r.URL.Path, path)
	}
	if got := r.Header.Get("Authorization"); got != wantAuth {
		t.Errorf("Authorization = %q, want %q", got, wantAuth)
	}
}

// requireQuery asserts that each key in want has exactly the listed values.
func requireQuery(t *testing.T, r *http.Request, want map[string][]string) {
	t.Helper()
	q := r.URL.Query()
	for key, vals := range want {
		got := q[key]
		if len(got) != len(vals) {
			t.Errorf("query %s = %v, want %v", key, got, vals)
			continue
		}
		for i := range vals {
			if got[i] != vals[i] {
				t.Errorf("query %s[%d] = %q, want %q", key, i, got[i], vals[i])
			}
		}
	}
}

// requireNoQuery asserts that none of the keys are present in the query string.
func requireNoQuery(t *testing.T, r *http.Request, keys ...string) {
	t.Helper()
	q := r.URL.Query()
	for _, key := range keys {
		if _, ok := q[key]; ok {
			t.Errorf("query %s = %v, want absent", key, q[key])
		}
	}
}

// decodeJSONBody decodes the request body into a generic map.
func decodeJSONBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode request body %q: %v", raw, err)
	}
	return body
}

// gqlRecorder stores the GraphQL requests a dev-server stub received.
type gqlRecorder struct {
	mu    sync.Mutex
	calls []graphqlRequest
}

func (r *gqlRecorder) add(req graphqlRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, req)
}

// last returns the most recent request, failing the test if none were made.
func (r *gqlRecorder) last(t *testing.T) graphqlRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		t.Fatal("no GraphQL requests were made")
	}
	return r.calls[len(r.calls)-1]
}

// newDevGQLServer stubs the dev server GraphQL endpoint (POST /v0/gql). Each
// request is dispatched on its operationName to the matching response body;
// unknown operations fail the test. Every request is recorded.
func newDevGQLServer(t *testing.T, responses map[string]string) (*httptest.Server, *gqlRecorder) {
	t.Helper()
	rec := &gqlRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRequest(t, r, http.MethodPost, testPathDevGQL, "")
		if ct := r.Header.Get("Content-Type"); ct != testApplicationJSON {
			t.Errorf("Content-Type = %q, want %q", ct, testApplicationJSON)
		}
		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode graphql request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		rec.add(req)
		body, ok := responses[req.OperationName]
		if !ok {
			t.Errorf("unexpected GraphQL operation %q", req.OperationName)
			writeJSON(w, http.StatusBadRequest, `{"errors":[{"message":"unexpected operation"}]}`)
			return
		}
		writeJSON(w, http.StatusOK, body)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// v2Route is one stubbed v2 endpoint: the handler receives the request and
// returns the JSON body to send with status 200.
type v2Route func(t *testing.T, r *http.Request) string

// newV2Server stubs a set of v2 endpoints keyed by "METHOD /path". Unmatched
// requests fail the test with a 404. It records how many requests were served.
func newV2Server(t *testing.T, routes map[string]v2Route) (*httptest.Server, *requestCounter) {
	t.Helper()
	counter := &requestCounter{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.inc()
		if got := r.Header.Get("Authorization"); got != testBearerAPIKey {
			t.Errorf("Authorization = %q, want %q", got, testBearerAPIKey)
		}
		route, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			writeJSON(w, http.StatusNotFound, testNotFoundResp)
			return
		}
		writeJSON(w, http.StatusOK, route(t, r))
	}))
	t.Cleanup(srv.Close)
	return srv, counter
}

// requestCounter counts requests served by a stub server.
type requestCounter struct {
	mu sync.Mutex
	n  int
}

func (c *requestCounter) inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *requestCounter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// staticRoute returns a v2Route that always responds with body.
func staticRoute(body string) v2Route {
	return func(*testing.T, *http.Request) string { return body }
}
