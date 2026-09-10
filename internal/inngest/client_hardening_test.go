package inngest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBearerToken(t *testing.T) {
	hexKey := "signkey-prod-abcdef0123456789"
	hashed, err := HashSigningKey(hexKey)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"api key sent raw", testAPIKey, testAPIKey},
		{"hex signing key hashed", hexKey, hashed},
		{"non-hex signing key falls back to raw", "signkey-test-nothex!", "signkey-test-nothex!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bearerToken(tt.key); got != tt.want {
				t.Errorf("bearerToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGuardPlaintextAuth(t *testing.T) {
	tests := []struct {
		url    string
		wantOK bool
	}{
		{"https://api.inngest.com/v2/runs", true},
		{"http://localhost:8288/v0/gql", true},
		{"http://app.localhost/x", true},
		{"http://127.0.0.1:8288/x", true},
		{"http://[::1]:8288/x", true},
		{"http://api.example.com/v2/runs", false},
		{"http://10.0.0.5/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = guardPlaintextAuth(req)
			if tt.wantOK && err != nil {
				t.Errorf("guardPlaintextAuth = %v, want nil", err)
			}
			if !tt.wantOK && !errors.Is(err, ErrPlaintextCredentials) {
				t.Errorf("guardPlaintextAuth = %v, want ErrPlaintextCredentials", err)
			}
		})
	}
}

func TestDo_RefusesCredentialOverPlaintext(t *testing.T) {
	c := NewClient(ClientOptions{SigningKey: testAPIKey, APIBaseURL: "http://api.example.com"})
	_, err := c.v2Get(context.Background(), "runs", nil, nil)
	if !errors.Is(err, ErrPlaintextCredentials) {
		t.Fatalf("err = %v, want ErrPlaintextCredentials", err)
	}

	// Without a credential the request may proceed (it fails on DNS, not the guard).
	anon := NewClient(ClientOptions{APIBaseURL: "http://api.invalid", Timeout: time.Second})
	if _, err := anon.v2Get(context.Background(), "runs", nil, nil); errors.Is(err, ErrPlaintextCredentials) {
		t.Error("guard must not trigger when no credential is attached")
	}
}

func TestDoWithRetry_CancelledDuringBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := newCloudClient(srv).v2Get(ctx, "runs", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "cancelled while rate limited") {
		t.Fatalf("err = %v, want cancellation during backoff", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("backoff ignored cancellation: took %s", elapsed)
	}
}

func TestNewClient_Timeout(t *testing.T) {
	if got := NewClient(ClientOptions{}).httpClient.Timeout; got != 30*time.Second {
		t.Errorf("default timeout = %s, want 30s", got)
	}
	if got := NewClient(ClientOptions{Timeout: 5 * time.Second}).httpClient.Timeout; got != 5*time.Second {
		t.Errorf("custom timeout = %s, want 5s", got)
	}
	if got := NewClient(ClientOptions{Timeout: -1}).httpClient.Timeout; got != 30*time.Second {
		t.Errorf("negative timeout = %s, want default", got)
	}
}

func TestEventURL_DevFallbackKey(t *testing.T) {
	c := NewClient(ClientOptions{DevMode: true, DevServerURL: "http://localhost:8288"})
	if got := c.eventURL(); got != "http://localhost:8288/e/test" {
		t.Errorf("eventURL() = %s, want placeholder key in dev mode", got)
	}
}

func TestGetREST_APIErrorWithoutJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer srv.Close()
	var out any
	err := newCloudClient(srv).GetREST(context.Background(), "events", &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadGateway || !strings.Contains(apiErr.Message, "bad gateway") {
		t.Fatalf("err = %v, want APIError 502 with body", err)
	}
}
