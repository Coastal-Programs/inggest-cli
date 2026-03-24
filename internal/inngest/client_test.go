package inngest

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testSigningKey = "signkey-test-abc123def456"

// ---------------------------------------------------------------------------
// TestNewClient
// ---------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	t.Run("default URLs when none specified", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		if c.apiBaseURL != defaultAPIBaseURL {
			t.Errorf("apiBaseURL = %q, want %q", c.apiBaseURL, defaultAPIBaseURL)
		}
		if c.devServerURL != defaultDevServerURL {
			t.Errorf("devServerURL = %q, want %q", c.devServerURL, defaultDevServerURL)
		}
	})

	t.Run("custom URLs", func(t *testing.T) {
		c := NewClient(ClientOptions{
			APIBaseURL:   "https://custom-api.example.com",
			DevServerURL: "http://custom-dev:9999",
		})
		if c.apiBaseURL != "https://custom-api.example.com" {
			t.Errorf("apiBaseURL = %q, want %q", c.apiBaseURL, "https://custom-api.example.com")
		}
		if c.devServerURL != "http://custom-dev:9999" {
			t.Errorf("devServerURL = %q, want %q", c.devServerURL, "http://custom-dev:9999")
		}
	})

	t.Run("trailing slash trimmed", func(t *testing.T) {
		c := NewClient(ClientOptions{
			APIBaseURL:   "https://api.example.com///",
			DevServerURL: "http://localhost:8288/",
		})
		if c.apiBaseURL != "https://api.example.com" {
			t.Errorf("apiBaseURL = %q, want trailing slashes trimmed", c.apiBaseURL)
		}
		if c.devServerURL != "http://localhost:8288" {
			t.Errorf("devServerURL = %q, want trailing slashes trimmed", c.devServerURL)
		}
	})

	t.Run("default user agent", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		if c.userAgent != "inngest-cli/dev" {
			t.Errorf("userAgent = %q, want %q", c.userAgent, "inngest-cli/dev")
		}
	})

	t.Run("custom user agent", func(t *testing.T) {
		c := NewClient(ClientOptions{UserAgent: "my-app/1.0"})
		if c.userAgent != "my-app/1.0" {
			t.Errorf("userAgent = %q, want %q", c.userAgent, "my-app/1.0")
		}
	})

	t.Run("fields propagated", func(t *testing.T) {
		c := NewClient(ClientOptions{
			SigningKey: "sk-test-key",
			EventKey:   "evt-key",
			Env:        "production",
			DevMode:    true,
		})
		if c.signingKey != "sk-test-key" {
			t.Errorf("signingKey = %q, want %q", c.signingKey, "sk-test-key")
		}
		if c.eventKey != "evt-key" {
			t.Errorf("eventKey = %q, want %q", c.eventKey, "evt-key")
		}
		if c.env != "production" {
			t.Errorf("env = %q, want %q", c.env, "production")
		}
		if !c.devMode {
			t.Error("devMode = false, want true")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGraphqlURL
// ---------------------------------------------------------------------------

func TestGraphqlURL(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		want := defaultAPIBaseURL + "/gql"
		if got := c.graphqlURL(); got != want {
			t.Errorf("graphqlURL() = %q, want %q", got, want)
		}
	})

	t.Run("custom base URL", func(t *testing.T) {
		c := NewClient(ClientOptions{APIBaseURL: "https://custom.api.io"})
		want := "https://custom.api.io/gql"
		if got := c.graphqlURL(); got != want {
			t.Errorf("graphqlURL() = %q, want %q", got, want)
		}
	})

	t.Run("dev mode uses dev server URL", func(t *testing.T) {
		c := NewClient(ClientOptions{DevMode: true})
		want := defaultDevServerURL + "/v0/gql"
		if got := c.graphqlURL(); got != want {
			t.Errorf("graphqlURL() = %q, want %q", got, want)
		}
	})

	t.Run("dev mode with custom dev server URL", func(t *testing.T) {
		c := NewClient(ClientOptions{
			DevMode:      true,
			DevServerURL: "http://mydev:1234",
		})
		want := "http://mydev:1234/v0/gql"
		if got := c.graphqlURL(); got != want {
			t.Errorf("graphqlURL() = %q, want %q", got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRestURL
// ---------------------------------------------------------------------------

func TestRestURL(t *testing.T) {
	t.Run("default base URL", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		want := defaultAPIBaseURL + "/v1/apps"
		if got := c.restURL("apps"); got != want {
			t.Errorf("restURL(\"apps\") = %q, want %q", got, want)
		}
	})

	t.Run("leading slash stripped", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		want := defaultAPIBaseURL + "/v1/apps/123"
		if got := c.restURL("/apps/123"); got != want {
			t.Errorf("restURL(\"/apps/123\") = %q, want %q", got, want)
		}
	})

	t.Run("custom base URL", func(t *testing.T) {
		c := NewClient(ClientOptions{APIBaseURL: "https://custom.api.io/"})
		want := "https://custom.api.io/v1/functions"
		if got := c.restURL("functions"); got != want {
			t.Errorf("restURL(\"functions\") = %q, want %q", got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// TestEventURL
// ---------------------------------------------------------------------------

func TestEventURL(t *testing.T) {
	t.Run("default with event key", func(t *testing.T) {
		c := NewClient(ClientOptions{EventKey: "my-event-key"})
		want := defaultEventHost + "/e/my-event-key"
		if got := c.eventURL(); got != want {
			t.Errorf("eventURL() = %q, want %q", got, want)
		}
	})

	t.Run("empty event key", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		want := defaultEventHost + "/e/"
		if got := c.eventURL(); got != want {
			t.Errorf("eventURL() = %q, want %q", got, want)
		}
	})

	t.Run("dev mode uses dev server URL", func(t *testing.T) {
		c := NewClient(ClientOptions{
			DevMode:  true,
			EventKey: "test-key",
		})
		want := defaultDevServerURL + "/e/test-key"
		if got := c.eventURL(); got != want {
			t.Errorf("eventURL() = %q, want %q", got, want)
		}
	})

	t.Run("dev mode with custom dev server URL", func(t *testing.T) {
		c := NewClient(ClientOptions{
			DevMode:      true,
			DevServerURL: "http://dev:5555/",
			EventKey:     "key-abc",
		})
		want := "http://dev:5555/e/key-abc"
		if got := c.eventURL(); got != want {
			t.Errorf("eventURL() = %q, want %q", got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDevURL
// ---------------------------------------------------------------------------

func TestDevURL(t *testing.T) {
	t.Run("default dev server URL", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		want := defaultDevServerURL + "/status"
		if got := c.devURL("status"); got != want {
			t.Errorf("devURL(\"status\") = %q, want %q", got, want)
		}
	})

	t.Run("leading slash stripped", func(t *testing.T) {
		c := NewClient(ClientOptions{})
		want := defaultDevServerURL + "/some/path"
		if got := c.devURL("/some/path"); got != want {
			t.Errorf("devURL(\"/some/path\") = %q, want %q", got, want)
		}
	})

	t.Run("custom dev server URL", func(t *testing.T) {
		c := NewClient(ClientOptions{DevServerURL: "http://mydev:7777/"})
		want := "http://mydev:7777/health"
		if got := c.devURL("health"); got != want {
			t.Errorf("devURL(\"health\") = %q, want %q", got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDoRetry429WithRetryAfter
// ---------------------------------------------------------------------------

func TestDoRetry429WithRetryAfter(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{APIBaseURL: srv.URL})
	// Override httpClient to use the test server's client.
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	got := atomic.LoadInt64(&calls)
	if got != 2 {
		t.Errorf("server received %d requests, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// TestDoRetry429Exhausted
// ---------------------------------------------------------------------------

func TestDoRetry429Exhausted(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{APIBaseURL: srv.URL})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	// The last attempt's response is returned — should be 429.
	if resp == nil {
		t.Fatal("expected non-nil response after retry exhaustion")
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}

	got := atomic.LoadInt64(&calls)
	if got != int64(maxRetries) {
		t.Errorf("server received %d requests, want %d", got, maxRetries)
	}
}

// ---------------------------------------------------------------------------
// TestDoNoRetryOn200
// ---------------------------------------------------------------------------

func TestDoNoRetryOn200(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{APIBaseURL: srv.URL})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	got := atomic.LoadInt64(&calls)
	if got != 1 {
		t.Errorf("server received %d requests, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// TestDoNoRetryOn500
// ---------------------------------------------------------------------------

func TestDoNoRetryOn500(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{APIBaseURL: srv.URL})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	got := atomic.LoadInt64(&calls)
	if got != 1 {
		t.Errorf("server received %d requests, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// TestDoFallbackKeyOn401
// ---------------------------------------------------------------------------

func TestDoFallbackKeyOn401(t *testing.T) {
	t.Run("retries with fallback key on 401", func(t *testing.T) {
		var calls int64
		var lastAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt64(&calls, 1)
			lastAuth = r.Header.Get("Authorization")
			if n == 1 {
				// First call with primary key — reject
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("unauthorized"))
				return
			}
			// Second call with fallback key — accept
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer srv.Close()

		c := NewClient(ClientOptions{
			SigningKey:         "sk-primary",
			SigningKeyFallback: "sk-fallback",
		})
		c.httpClient = srv.Client()

		req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}

		resp, err := c.do(req)
		if err != nil {
			t.Fatalf("do() error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		got := atomic.LoadInt64(&calls)
		if got != 2 {
			t.Errorf("server received %d requests, want 2", got)
		}

		if lastAuth != "Bearer sk-fallback" {
			t.Errorf("last Authorization = %q, want %q", lastAuth, "Bearer sk-fallback")
		}
	})

	t.Run("no fallback retry when no fallback key set", func(t *testing.T) {
		var calls int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(&calls, 1)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized"))
		}))
		defer srv.Close()

		c := NewClient(ClientOptions{
			SigningKey: "sk-primary",
		})
		c.httpClient = srv.Client()

		req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}

		resp, err := c.do(req)
		if err != nil {
			t.Fatalf("do() error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}

		got := atomic.LoadInt64(&calls)
		if got != 1 {
			t.Errorf("server received %d requests, want 1 (no fallback retry)", got)
		}
	})

	t.Run("fallback with POST body is replayed correctly", func(t *testing.T) {
		var calls int64
		var lastBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt64(&calls, 1)
			body, _ := io.ReadAll(r.Body)
			lastBody = string(body)
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer srv.Close()

		c := NewClient(ClientOptions{
			SigningKey:         "sk-primary",
			SigningKeyFallback: "sk-fallback",
		})
		c.httpClient = srv.Client()

		body := `{"query":"{ functions { id } }"}`
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/test", strings.NewReader(body))
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}

		resp, err := c.do(req)
		if err != nil {
			t.Fatalf("do() error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		if lastBody != body {
			t.Errorf("body on retry = %q, want %q", lastBody, body)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDoRetry429PostBodyReplay
// ---------------------------------------------------------------------------

func TestDoRetry429PostBodyReplay(t *testing.T) {
	var calls int64
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{APIBaseURL: srv.URL})
	c.httpClient = srv.Client()

	payload := `{"event":"test/sent","data":{"user_id":"abc123"}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/e/test-key", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	got := atomic.LoadInt64(&calls)
	if got != 2 {
		t.Errorf("server received %d requests, want 2", got)
	}

	if len(bodies) != 2 {
		t.Fatalf("captured %d bodies, want 2", len(bodies))
	}
	if bodies[0] != payload {
		t.Errorf("first request body = %q, want %q", bodies[0], payload)
	}
	if bodies[1] != payload {
		t.Errorf("second request body = %q, want %q (body not replayed)", bodies[1], payload)
	}
}

// ---------------------------------------------------------------------------
// TestDoSetsHeaders
// ---------------------------------------------------------------------------

func TestDoSetsHeaders(t *testing.T) {
	t.Run("user agent and authorization set", func(t *testing.T) {
		var gotUA, gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUA = r.Header.Get("User-Agent")
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		c := NewClient(ClientOptions{
			SigningKey: "sk-test-123",
			UserAgent:  "test-agent/2.0",
		})
		c.httpClient = srv.Client()

		req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}

		resp, err := c.do(req)
		if err != nil {
			t.Fatalf("do() error: %v", err)
		}
		resp.Body.Close()

		if gotUA != "test-agent/2.0" {
			t.Errorf("User-Agent = %q, want %q", gotUA, "test-agent/2.0")
		}
		if gotAuth != "Bearer sk-test-123" {
			t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-test-123")
		}
	})

	t.Run("no authorization when signing key empty", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		c := NewClient(ClientOptions{})
		c.httpClient = srv.Client()

		req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}

		resp, err := c.do(req)
		if err != nil {
			t.Fatalf("do() error: %v", err)
		}
		resp.Body.Close()

		if gotAuth != "" {
			t.Errorf("Authorization = %q, want empty (no signing key)", gotAuth)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDoSetsEnvHeader
// ---------------------------------------------------------------------------

func TestDoSetsEnvHeader(t *testing.T) {
	var gotEnv string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEnv = r.Header.Get("X-Inngest-Env")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{Env: "production", DevMode: false})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	resp.Body.Close()

	if gotEnv != "production" {
		t.Errorf("X-Inngest-Env = %q, want %q", gotEnv, "production")
	}
}

// ---------------------------------------------------------------------------
// TestDoSuppressesEnvHeaderInDevMode
// ---------------------------------------------------------------------------

func TestDoSuppressesEnvHeaderInDevMode(t *testing.T) {
	var gotEnv string
	var envPresent bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEnv = r.Header.Get("X-Inngest-Env")
		_, envPresent = r.Header["X-Inngest-Env"]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{Env: "production", DevMode: true})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	resp.Body.Close()

	if envPresent {
		t.Errorf("X-Inngest-Env header should not be set in dev mode, got %q", gotEnv)
	}
}

// ---------------------------------------------------------------------------
// TestDoReadBodyError
// ---------------------------------------------------------------------------

// errReader is a reader that always returns an error.
type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

func TestDoReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Body = io.NopCloser(errReader{})

	resp, err := c.do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "reading request body") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "reading request body")
	}
}

// ---------------------------------------------------------------------------
// TestDoFallbackRetryError
// ---------------------------------------------------------------------------

func TestDoFallbackRetryError(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			// First call returns 401 to trigger fallback.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Should not reach here — server will be closed.
		w.WriteHeader(http.StatusOK)
	}))

	c := NewClient(ClientOptions{
		SigningKey:         "sk-primary",
		SigningKeyFallback: "sk-fallback",
	})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	// Close the server after the first request so the fallback retry fails.
	// We need a custom approach: close the server right after the first 401.
	// Actually, we make the first call, it gets 401, then we close.
	// But do() does this internally. So we close the server in the handler
	// after the first response, but that's racy. Instead, use a transport hook.

	// Simpler approach: use a custom RoundTripper that fails on the second call.
	origTransport := c.httpClient.Transport
	callCount := int64(0)
	c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n == 1 {
			return origTransport.RoundTrip(r)
		}
		return nil, fmt.Errorf("connection refused")
	})

	resp, err := c.do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	srv.Close()

	if err == nil {
		t.Fatal("expected error from fallback retry, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "connection refused")
	}
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// ---------------------------------------------------------------------------
// TestDoWithRetry_TransportError
// ---------------------------------------------------------------------------

func TestDoWithRetry_TransportError(t *testing.T) {
	c := NewClient(ClientOptions{})
	c.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		}),
	}

	req, err := http.NewRequest(http.MethodGet, "http://localhost:1/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "request failed")
	}
}

// ---------------------------------------------------------------------------
// TestDoWithRetry_RetryAfterHTTPDate
// ---------------------------------------------------------------------------

func TestDoWithRetry_RetryAfterHTTPDate(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			// Use an HTTP-date a tiny bit in the future.
			futureDate := time.Now().Add(1 * time.Second).UTC().Format(http.TimeFormat)
			w.Header().Set("Retry-After", futureDate)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Errorf("server received %d requests, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// TestDoWithRetry_RetryAfterPastDate
// ---------------------------------------------------------------------------

func TestDoWithRetry_RetryAfterPastDate(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			// Use an HTTP-date in the past.
			pastDate := time.Now().Add(-10 * time.Minute).UTC().Format(http.TimeFormat)
			w.Header().Set("Retry-After", pastDate)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Errorf("server received %d requests, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// TestDoWithRetry_EmptyRetryAfter
// ---------------------------------------------------------------------------

func TestDoWithRetry_EmptyRetryAfter(t *testing.T) {
	// Empty Retry-After means the default backoff is used (attempt+1 seconds).
	// To avoid slow tests, we test this with a short timeout — the first
	// attempt's default wait is 1s, so we allow up to 3s total.
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			// Return 429 with no Retry-After header at all.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	start := time.Now()
	resp, err := c.do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	// Default backoff for attempt 0 is 1s. Verify we actually waited.
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, expected at least ~1s default backoff", elapsed)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Errorf("server received %d requests, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// TestDoWithRetry_UnparseableRetryAfter
// ---------------------------------------------------------------------------

func TestDoWithRetry_UnparseableRetryAfter(t *testing.T) {
	// Unparseable Retry-After falls through to default backoff.
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "invalid-stuff")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	start := time.Now()
	resp, err := c.do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	// Default backoff for attempt 0 is 1s.
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, expected at least ~1s default backoff", elapsed)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Errorf("server received %d requests, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// TestRestURL_DevMode
// ---------------------------------------------------------------------------

func TestRestURL_DevMode(t *testing.T) {
	c := NewClient(ClientOptions{DevMode: true})
	want := defaultDevServerURL + "/v1/apps"
	if got := c.restURL("apps"); got != want {
		t.Errorf("restURL(\"apps\") = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// TestRestURL_DevModeCustomURL
// ---------------------------------------------------------------------------

func TestRestURL_DevModeCustomURL(t *testing.T) {
	c := NewClient(ClientOptions{DevMode: true, DevServerURL: "http://mydev:9000/"})
	want := "http://mydev:9000/v1/functions"
	if got := c.restURL("functions"); got != want {
		t.Errorf("restURL(\"functions\") = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// TestHashSigningKey
// ---------------------------------------------------------------------------

func TestHashSigningKey(t *testing.T) {
	// Test vector derived from inngest/inngest pkg/authn/signing_key_strategy.go:
	// normalizeKey strips "signkey-test-" prefix, then hex-decode + SHA-256 + hex-encode.
	t.Run("cloud key with prefix", func(t *testing.T) {
		// "abc123def456" hex-decoded = 6 bytes, SHA-256 of those bytes
		key := testSigningKey
		got, err := HashSigningKey(key)
		if err != nil {
			t.Fatalf("HashSigningKey(%q) error: %v", key, err)
		}
		if got == "" {
			t.Fatal("expected non-empty hash")
		}
		if got == "abc123def456" {
			t.Error("hash should not equal the raw key material")
		}
		if len(got) != 64 {
			t.Errorf("expected 64-char hex SHA-256 hash, got %d chars: %s", len(got), got)
		}
	})

	t.Run("prod key prefix stripped", func(t *testing.T) {
		key := "signkey-prod-abc123def456"
		got, err := HashSigningKey(key)
		if err != nil {
			t.Fatalf("HashSigningKey(%q) error: %v", key, err)
		}
		// Should produce same hash as test prefix since the hex payload is the same
		key2 := testSigningKey
		got2, _ := HashSigningKey(key2)
		if got != got2 {
			t.Errorf("same hex payload with different prefixes should hash the same: %q != %q", got, got2)
		}
	})

	t.Run("raw hex key without prefix", func(t *testing.T) {
		key := "abc123def456"
		got, err := HashSigningKey(key)
		if err != nil {
			t.Fatalf("HashSigningKey(%q) error: %v", key, err)
		}
		if len(got) != 64 {
			t.Errorf("expected 64-char hash, got %d", len(got))
		}
	})

	t.Run("invalid hex returns error", func(t *testing.T) {
		_, err := HashSigningKey("not-hex-at-all")
		if err == nil {
			t.Fatal("expected error for non-hex key")
		}
	})

	t.Run("odd-length hex returns error", func(t *testing.T) {
		_, err := HashSigningKey("abc")
		if err == nil {
			t.Fatal("expected error for odd-length hex")
		}
	})

	t.Run("matches inngest server implementation", func(t *testing.T) {
		// Verify against the Go server's HashedSigningKey from
		// inngest/inngest pkg/authn/signing_key_strategy.go:
		// HashedSigningKey("abc123def456") should produce the same result.
		key := testSigningKey
		got, err := HashSigningKey(key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Manually compute: hex.Decode("abc123def456") -> sha256 -> hex.Encode
		raw := []byte{0xab, 0xc1, 0x23, 0xde, 0xf4, 0x56}
		sum := sha256Sum(raw)
		want := fmt.Sprintf("%x", sum)
		if got != want {
			t.Errorf("HashSigningKey(%q) = %q, want %q", key, got, want)
		}
	})
}

func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// ---------------------------------------------------------------------------
// TestDoHashedSigningKey — covers the success branch of HashSigningKey in do()
// ---------------------------------------------------------------------------

func TestDoHashedSigningKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use a valid signkey- prefixed key with valid hex after the prefix.
	c := NewClient(ClientOptions{
		SigningKey: "signkey-test-abcdef0123456789abcdef0123456789",
	})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	resp.Body.Close()

	// The key should be hashed, not sent raw.
	if gotAuth == "Bearer signkey-test-abcdef0123456789abcdef0123456789" {
		t.Error("Authorization should be hashed, not raw")
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("Authorization = %q, want Bearer prefix", gotAuth)
	}
}

func TestDoHashedFallbackKey(t *testing.T) {
	var calls int64
	var lastAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		lastAuth = r.Header.Get("Authorization")
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(ClientOptions{
		SigningKey:         "signkey-test-abcdef0123456789abcdef0123456789",
		SigningKeyFallback: "signkey-prod-1234567890abcdef1234567890abcdef",
	})
	c.httpClient = srv.Client()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	// Fallback key should also be hashed.
	if lastAuth == "Bearer signkey-prod-1234567890abcdef1234567890abcdef" {
		t.Error("Fallback authorization should be hashed, not raw")
	}
}
