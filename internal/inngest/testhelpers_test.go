package inngest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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

// graphqlBody is the request payload POSTed by ExecuteGraphQL, used for
// capturing and asserting the request in tests.
type graphqlBody struct {
	OperationName string                 `json:"operationName,omitempty"`
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// newTestServer creates an httptest.Server that handles POST /gql (cloud) and
// returns the given JSON response body. It verifies method, path, content-type
// and authorization headers. If captured is non-nil the decoded request body is
// stored there for assertions.
func newTestServer(t *testing.T, responseBody string, captured *graphqlRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("unexpected Authorization header: %s", auth)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		if captured != nil {
			if err := json.Unmarshal(body, captured); err != nil {
				t.Fatalf("failed to unmarshal request body: %v", err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	}))
}
