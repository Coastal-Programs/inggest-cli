package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDevInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRequest(t, r, http.MethodGet, "/dev", "")
		writeJSON(w, http.StatusOK, `{"version": "1.17.0", "functions": [{"id": "f", "slug": "f"}], "handlers": []}`)
	}))
	defer srv.Close()
	client := newDevClient(srv)

	info, err := client.GetDevInfo(context.Background())
	if err != nil || info.Version != "1.17.0" || len(info.Functions) != 1 {
		t.Fatalf("GetDevInfo = (%+v, %v)", info, err)
	}
	if !client.IsDevServerRunning(context.Background()) {
		t.Error("IsDevServerRunning = false for a healthy server")
	}
}

func TestGetDevInfo_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"server error", http.StatusInternalServerError, `oops`},
		{"invalid json", http.StatusOK, `<html>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, tt.status, tt.body)
			}))
			defer srv.Close()
			if _, err := newDevClient(srv).GetDevInfo(context.Background()); err == nil {
				t.Error("expected error")
			}
		})
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	down.Close()
	if newDevClient(down).IsDevServerRunning(context.Background()) {
		t.Error("IsDevServerRunning = true for a closed server")
	}
}

// TestGetDevInfo_RequestConstructionError covers the branch where the dev URL
// is unusable, so the failure happens before any connection is attempted.
func TestGetDevInfo_RequestConstructionError(t *testing.T) {
	client := NewClient(ClientOptions{DevServerURL: testInvalidURL, DevMode: true})

	_, err := client.GetDevInfo(context.Background())
	requireErrContains(t, err, "create dev info request")

	if client.IsDevServerRunning(context.Background()) {
		t.Error("IsDevServerRunning = true for an unusable dev URL")
	}
}

// TestGetDevInfo_TransportError covers the unreachable-dev-server branch.
func TestGetDevInfo_TransportError(t *testing.T) {
	srv := newClosedServer(t)

	_, err := newDevClient(srv).GetDevInfo(context.Background())
	requireErrContains(t, err, "dev info request")
}

// TestGetDevInfo_ReadBodyError covers the branch where the response headers
// arrive but the body cannot be read.
func TestGetDevInfo_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()

	_, err := newDevClient(srv).GetDevInfo(context.Background())
	requireErrContains(t, err, "read dev info response")
}
