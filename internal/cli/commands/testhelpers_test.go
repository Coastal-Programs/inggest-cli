package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newMockServer creates a test server that handles GraphQL and REST endpoints.
// gqlResponses maps operationName to JSON response string.
// restHandlers maps URL path to handler function.
func newMockServer(t *testing.T, gqlResponses map[string]string, restHandlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GraphQL endpoint
		if r.URL.Path == "/v0/gql" && r.Method == http.MethodPost {
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
