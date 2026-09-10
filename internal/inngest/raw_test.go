package inngest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRawRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRequest(t, r, http.MethodPost, "/v2/runs/x/cancel", testBearerAPIKey)
		if r.URL.RawQuery != "limit=5" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		if ct := r.Header.Get("Content-Type"); ct != testApplicationJSON {
			t.Errorf("Content-Type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"a":1}` {
			t.Errorf("body = %s", body)
		}
		writeJSON(w, http.StatusAccepted, `{"data": {"ok": true}}`)
	}))
	defer srv.Close()

	// Missing leading slash is tolerated.
	resp, err := newCloudClient(srv).RawRequest(context.Background(), "post", "v2/runs/x/cancel?limit=5", []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("RawRequest: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted || string(resp.Body) != `{"data": {"ok": true}}` {
		t.Errorf("resp = %d %s", resp.StatusCode, resp.Body)
	}
}

func TestRawRequest_ReturnsErrorStatusWithoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" {
			t.Error("GET without body must not set Content-Type")
		}
		writeJSON(w, http.StatusNotFound, testNotFoundResp)
	}))
	defer srv.Close()
	resp, err := newCloudClient(srv).RawRequest(context.Background(), http.MethodGet, "/v2/nope", nil)
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("RawRequest = (%+v, %v), want 404 without error", resp, err)
	}
}

func TestRawRequest_RejectsAbsoluteURLs(t *testing.T) {
	client := NewClient(ClientOptions{APIBaseURL: "https://api.example.com", SigningKey: testAPIKey})
	for _, path := range []string{"https://evil.example/v2/runs", "//evil.example/v2/runs", "/v2/../../https://x"} {
		_, err := client.RawRequest(context.Background(), http.MethodGet, path, nil)
		if err == nil || !strings.Contains(err.Error(), "relative to the API base URL") {
			t.Errorf("path %q: err = %v, want rejection", path, err)
		}
	}
}

func TestRawRequest_DevModeUsesDevServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRequest(t, r, http.MethodGet, "/api/v2/health", "")
		writeJSON(w, http.StatusOK, `{"data": {"status": "ok"}}`)
	}))
	defer srv.Close()
	resp, err := newDevClient(srv).RawRequest(context.Background(), http.MethodGet, "/api/v2/health", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("RawRequest(dev) = (%+v, %v)", resp, err)
	}
}

// TestRawRequest_RejectsMalformedPaths covers the url.ParseRequestURI guard:
// a path that is relative but still unparseable is refused before any request
// is made, so the credential never leaves the process.
func TestRawRequest_RejectsMalformedPaths(t *testing.T) {
	srv := newUnusedServer(t)
	client := newCloudClient(srv)

	for _, path := range []string{"/v2/runs%zz", "/v2/runs\x7f"} {
		_, err := client.RawRequest(context.Background(), http.MethodGet, path, nil)
		requireErrContains(t, err, "invalid path")
	}
}

// TestRawRequest_TransportError covers the branch where the request is well
// formed but the API is unreachable.
func TestRawRequest_TransportError(t *testing.T) {
	srv := newClosedServer(t)

	_, err := newCloudClient(srv).RawRequest(context.Background(), http.MethodGet, "/v2/runs", nil)
	requireErrContains(t, err, "GET /v2/runs")
}

// TestRawRequest_ReadBodyError covers the branch where response headers arrive
// but the body cannot be fully read.
func TestRawRequest_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()

	_, err := newCloudClient(srv).RawRequest(context.Background(), http.MethodGet, "/v2/runs", nil)
	requireErrContains(t, err, "read GET /v2/runs response")
}

// TestRawRequest_MethodConstructionError covers the branch where the method
// itself is not a valid HTTP token, so the request cannot be constructed.
func TestRawRequest_MethodConstructionError(t *testing.T) {
	srv := newUnusedServer(t)

	_, err := newCloudClient(srv).RawRequest(context.Background(), "BAD METHOD", "/v2/runs", nil)
	requireErrContains(t, err, "create BAD METHOD /v2/runs request")
}
