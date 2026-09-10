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
