package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
)

// newMockServer creates a test server that handles GraphQL and REST endpoints.
// gqlResponses maps operationName to JSON response string.
// restHandlers maps URL path to handler function.
func newMockServer(t *testing.T, gqlResponses map[string]string, restHandlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GraphQL endpoint
		if (r.URL.Path == "/gql" || r.URL.Path == "/v0/gql") && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var req struct {
				OperationName string `json:"operationName"`
			}
			json.Unmarshal(body, &req)

			if resp, ok := gqlResponses[req.OperationName]; ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(resp))
				return
			}
			// Unnamed queries (operationName empty) — try "" key
			if resp, ok := gqlResponses[""]; ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(resp))
				return
			}
			t.Logf("unhandled GraphQL operation: %q", req.OperationName)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// REST/other endpoints — exact match first
		if handler, ok := restHandlers[r.URL.Path]; ok {
			handler(w, r)
			return
		}
		// Try prefix match for parameterized paths (path ends with *)
		for path, handler := range restHandlers {
			if len(path) > 0 && path[len(path)-1] == '*' {
				prefix := path[:len(path)-1]
				if strings.HasPrefix(r.URL.Path, prefix) {
					handler(w, r)
					return
				}
			}
		}

		t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
}

// setupCloudState configures state globals for cloud tests pointing at the given server URL.
func setupCloudState(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("INNGEST_SIGNING_KEY", "")
	t.Setenv("INNGEST_SIGNING_KEY_FALLBACK", "")
	t.Setenv("INNGEST_EVENT_KEY", "")

	state.Config = &config.Config{SigningKey: "signkey-test-123"}
	state.Output = testOutputJSON
	state.APIBaseURL = srvURL
	state.DevServer = srvURL
	state.DevMode = false
	state.Env = ""
	state.AppVersion = testAppVersion
}

const (
	testOutputTable = "table"
	testOutputText  = "text"
	testFnID        = "fn-1"
	queryTrue       = "true"
)

// jsonOK returns a handler that serves body as application/json with status 200.
func jsonOK(body string) http.HandlerFunc {
	return jsonStatus(http.StatusOK, body)
}

// jsonStatus returns a handler that serves body as application/json with status.
func jsonStatus(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// v2Unauthorized is the REST v2 body for a rejected credential.
const v2Unauthorized = `{"errors":[{"code":"authorization_header_missing","message":"authorization header missing or invalid"}]}`

// v2ServerError is the REST v2 body for a 5xx failure.
const v2ServerError = `{"errors":[{"code":"internal_error","message":"something went wrong"}]}`

// captureStderr redirects os.Stderr to a pipe, runs fn, then returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("reading pipe: %v", err)
	}
	return buf.String()
}

// setStdin replaces os.Stdin with a pipe pre-loaded with input for the test's duration.
func setStdin(t *testing.T, input string) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("writing stdin: %v", err)
	}
	w.Close()

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})
}

// setStdinClosed points os.Stdin at a closed pipe so reads fail.
func setStdinClosed(t *testing.T) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	w.Close()
	r.Close()

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
}
