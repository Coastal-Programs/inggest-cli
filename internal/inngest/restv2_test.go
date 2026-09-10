package inngest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDecodeV2Envelope(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantData string
		wantPage *Page
		wantErr  *APIError // expected *APIError, nil when no API error expected
		wantMsg  string    // substring of a non-API error
	}{
		{
			name:     "200 with data and page",
			status:   http.StatusOK,
			body:     `{"data": [{"id": "r1"}], "page": {"cursor": "abc", "hasMore": true, "limit": 50}, "metadata": {"fetchedAt": "2026-09-01T00:00:00Z"}}`,
			wantData: `[{"id": "r1"}]`,
			wantPage: &Page{Cursor: "abc", HasMore: true, Limit: 50},
		},
		{
			name:     "201 created is success",
			status:   http.StatusCreated,
			body:     `{"data": {"id": "s1"}}`,
			wantData: `{"id": "s1"}`,
		},
		{
			name:   "204 empty body is success",
			status: http.StatusNoContent,
			body:   "",
		},
		{
			name:    "401 with error code and message",
			status:  http.StatusUnauthorized,
			body:    testUnauthorized,
			wantErr: &APIError{StatusCode: http.StatusUnauthorized, Code: "invalid_signing_key", Message: "invalid signing key"},
		},
		{
			name:    "422 multiple errors joined",
			status:  http.StatusUnprocessableEntity,
			body:    `{"errors": [{"code": "invalid_request", "message": "limit too large"}, {"code": "other", "message": "cursor malformed"}]}`,
			wantErr: &APIError{StatusCode: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "limit too large; cursor malformed"},
		},
		{
			name:    "500 empty body uses status text",
			status:  http.StatusInternalServerError,
			body:    "",
			wantErr: &APIError{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error"},
		},
		{
			name:    "502 non-JSON body is truncated into message",
			status:  http.StatusBadGateway,
			body:    "<html>" + strings.Repeat("x", truncateBodyMaxLen*2),
			wantErr: &APIError{StatusCode: http.StatusBadGateway, Message: truncateBody("<html>" + strings.Repeat("x", truncateBodyMaxLen*2))},
		},
		{
			name:    "200 non-JSON body is a decode error",
			status:  http.StatusOK,
			body:    "<html>ok</html>",
			wantMsg: "decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := decodeV2Envelope(tt.status, []byte(tt.body))

			if tt.wantErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("error = %v, want *APIError", err)
				}
				if *apiErr != *tt.wantErr {
					t.Errorf("APIError = %+v, want %+v", *apiErr, *tt.wantErr)
				}
				return
			}
			if tt.wantMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(env.Data) != tt.wantData {
				t.Errorf("data = %s, want %s", env.Data, tt.wantData)
			}
			switch {
			case tt.wantPage == nil && env.Page != nil:
				t.Errorf("page = %+v, want nil", *env.Page)
			case tt.wantPage != nil && env.Page == nil:
				t.Errorf("page = nil, want %+v", *tt.wantPage)
			case tt.wantPage != nil && *env.Page != *tt.wantPage:
				t.Errorf("page = %+v, want %+v", *env.Page, *tt.wantPage)
			}
		})
	}
}

func TestV2Do(t *testing.T) {
	t.Run("GET decodes data and page and sends headers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requireRequest(t, r, http.MethodGet, testPathV2Runs, testBearerAPIKey)
			requireQuery(t, r, map[string][]string{"limit": {"5"}})
			if got := r.Header.Get("Accept"); got != testApplicationJSON {
				t.Errorf("Accept = %q, want %q", got, testApplicationJSON)
			}
			if got := r.Header.Get("Content-Type"); got != "" {
				t.Errorf("Content-Type = %q, want empty on GET", got)
			}
			writeJSON(w, http.StatusOK, `{"data": [{"id": "a"}, {"id": "b"}], "page": {"cursor": "next", "hasMore": true, "limit": 5}}`)
		}))
		defer srv.Close()

		var data []struct {
			ID string `json:"id"`
		}
		page, err := newCloudClient(srv).v2Get(t.Context(), "runs", url.Values{"limit": {"5"}}, &data)
		if err != nil {
			t.Fatalf("v2Get: %v", err)
		}
		if len(data) != 2 || data[0].ID != "a" || data[1].ID != "b" {
			t.Errorf("data = %+v, want ids a,b", data)
		}
		if page == nil || *page != (Page{Cursor: "next", HasMore: true, Limit: 5}) {
			t.Errorf("page = %+v, want cursor next/hasMore/limit 5", page)
		}
	})

	t.Run("POST sends JSON body with content type", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requireRequest(t, r, http.MethodPost, "/v2/things", testBearerAPIKey)
			if got := r.Header.Get("Content-Type"); got != testApplicationJSON {
				t.Errorf("Content-Type = %q, want %q", got, testApplicationJSON)
			}
			body := decodeJSONBody(t, r)
			if body["name"] != "x" {
				t.Errorf("body = %v, want name x", body)
			}
			writeJSON(w, http.StatusCreated, `{"data": {"ok": true}}`)
		}))
		defer srv.Close()

		var out struct {
			OK bool `json:"ok"`
		}
		if err := newCloudClient(srv).v2Post(t.Context(), "things", map[string]string{"name": "x"}, &out); err != nil {
			t.Fatalf("v2Post: %v", err)
		}
		if !out.OK {
			t.Error("out.OK = false, want true")
		}
	})

	t.Run("null data with non-nil out leaves out untouched", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, `{"data": null, "page": {"hasMore": false}}`)
		}))
		defer srv.Close()

		out := map[string]any{"keep": true}
		page, err := newCloudClient(srv).v2Get(t.Context(), "things", nil, &out)
		if err != nil {
			t.Fatalf("v2Get: %v", err)
		}
		if out["keep"] != true {
			t.Errorf("out = %v, want untouched", out)
		}
		if page == nil || page.HasMore {
			t.Errorf("page = %+v, want hasMore false", page)
		}
	})

	t.Run("nil out discards data", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, `{"data": {"anything": [1, 2, 3]}}`)
		}))
		defer srv.Close()

		if _, err := newCloudClient(srv).v2Get(t.Context(), "things", nil, nil); err != nil {
			t.Fatalf("v2Get: %v", err)
		}
	})

	t.Run("401 returns APIError from errors array", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusUnauthorized, testUnauthorized)
		}))
		defer srv.Close()

		var out any
		_, err := newCloudClient(srv).v2Get(t.Context(), "runs", nil, &out)
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("error = %v, want *APIError", err)
		}
		if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Code != "invalid_signing_key" || apiErr.Message != "invalid signing key" {
			t.Errorf("APIError = %+v", *apiErr)
		}
		if !IsAuthError(err) {
			t.Error("IsAuthError = false, want true")
		}
		if !strings.Contains(err.Error(), "GET runs") {
			t.Errorf("error = %v, want method and path in message", err)
		}
	})

	t.Run("data that does not match out is a decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, `{"data": "not-an-object"}`)
		}))
		defer srv.Close()

		var out struct {
			ID string `json:"id"`
		}
		_, err := newCloudClient(srv).v2Get(t.Context(), "runs", nil, &out)
		if err == nil || !strings.Contains(err.Error(), "decode GET runs data") {
			t.Fatalf("error = %v, want decode error", err)
		}
	})

	t.Run("unmarshalable body is an encode error without a request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Error("server should not be called")
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		err := newCloudClient(srv).v2Post(t.Context(), "things", map[string]any{"ch": make(chan int)}, nil)
		if err == nil || !strings.Contains(err.Error(), "encode POST things body") {
			t.Fatalf("error = %v, want encode error", err)
		}
	})

	t.Run("cancelled context fails the request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, `{"data": null}`)
		}))
		defer srv.Close()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := newCloudClient(srv).v2Get(ctx, "runs", nil, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})

	t.Run("read body error is reported", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, `{"data": null}`)
		}))
		defer srv.Close()

		c := newCloudClient(srv)
		c.httpClient.Transport = &errBodyTransport{wrapped: srv.Client().Transport}
		_, err := c.v2Get(t.Context(), "runs", nil, nil)
		if err == nil || !strings.Contains(err.Error(), "read GET runs response") {
			t.Fatalf("error = %v, want read error", err)
		}
	})

	t.Run("invalid base URL is a request creation error", func(t *testing.T) {
		c := NewClient(ClientOptions{APIBaseURL: "http://invalid\x00host"})
		_, err := c.v2Get(t.Context(), "runs", nil, nil)
		if err == nil || !strings.Contains(err.Error(), "create GET runs request") {
			t.Fatalf("error = %v, want create request error", err)
		}
	})
}

func TestMillis_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Millis
		wantErr bool
	}{
		{name: "numeric string", input: `"12"`, want: 12},
		{name: "bare number", input: `12`, want: 12},
		{name: "large int64 string", input: `"9007199254740993"`, want: 9007199254740993},
		{name: "null", input: `null`, want: 0},
		{name: "empty string", input: `""`, want: 0},
		{name: "non-numeric string", input: `"abc"`, wantErr: true},
		{name: "float", input: `1.5`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				D Millis `json:"d"`
			}
			err := json.Unmarshal([]byte(`{"d": `+tt.input+`}`), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = %d, want error", tt.input, got.D)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s): %v", tt.input, err)
			}
			if got.D != tt.want {
				t.Errorf("Millis = %d, want %d", got.D, tt.want)
			}
		})
	}
}

func TestV2URL(t *testing.T) {
	tests := []struct {
		name string
		opts ClientOptions
		path string
		q    url.Values
		want string
	}{
		{
			name: "cloud without query",
			opts: ClientOptions{APIBaseURL: "https://api.example.com"},
			path: "runs",
			want: "https://api.example.com/v2/runs",
		},
		{
			name: "cloud with query",
			opts: ClientOptions{APIBaseURL: "https://api.example.com"},
			path: "runs",
			q:    url.Values{"limit": {"10"}, "cursor": {"a b"}},
			want: "https://api.example.com/v2/runs?cursor=a+b&limit=10",
		},
		{
			name: "cloud strips leading slash and trailing base slash",
			opts: ClientOptions{APIBaseURL: "https://api.example.com/"},
			path: "/runs/abc/trace",
			want: "https://api.example.com/v2/runs/abc/trace",
		},
		{
			name: "dev mode uses dev server api prefix",
			opts: ClientOptions{DevMode: true, DevServerURL: "http://localhost:9999"},
			path: "envs",
			want: "http://localhost:9999/api/v2/envs",
		},
		{
			name: "dev mode with query",
			opts: ClientOptions{DevMode: true, DevServerURL: "http://localhost:9999/"},
			path: "events/evt/runs",
			q:    url.Values{"limit": {"100"}},
			want: "http://localhost:9999/api/v2/events/evt/runs?limit=100",
		},
		{
			name: "empty query is not appended",
			opts: ClientOptions{APIBaseURL: "https://api.example.com"},
			path: "apps",
			q:    url.Values{},
			want: "https://api.example.com/v2/apps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewClient(tt.opts).v2URL(tt.path, tt.q); got != tt.want {
				t.Errorf("v2URL(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestAPIError(t *testing.T) {
	t.Run("Error includes code when present", func(t *testing.T) {
		err := &APIError{StatusCode: 404, Code: "not_found", Message: "run missing"}
		if got := err.Error(); got != "inngest: API error 404 (not_found): run missing" {
			t.Errorf("Error() = %q", got)
		}
	})

	t.Run("Error omits code when empty", func(t *testing.T) {
		err := &APIError{StatusCode: 502, Message: "Bad Gateway"}
		if got := err.Error(); got != "inngest: API error 502: Bad Gateway" {
			t.Errorf("Error() = %q", got)
		}
	})

	tests := []struct {
		name         string
		err          error
		wantAuth     bool
		wantNotFound bool
	}{
		{name: "401", err: &APIError{StatusCode: http.StatusUnauthorized}, wantAuth: true},
		{name: "403", err: &APIError{StatusCode: http.StatusForbidden}, wantAuth: true},
		{name: "404", err: &APIError{StatusCode: http.StatusNotFound}, wantNotFound: true},
		{name: "wrapped 401", err: fmt.Errorf("outer: %w", &APIError{StatusCode: http.StatusUnauthorized}), wantAuth: true},
		{name: "wrapped 404", err: fmt.Errorf("outer: %w", &APIError{StatusCode: http.StatusNotFound}), wantNotFound: true},
		{name: "500", err: &APIError{StatusCode: http.StatusInternalServerError}},
		{name: "plain error", err: errors.New("boom")},
		{name: "nil", err: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthError(tt.err); got != tt.wantAuth {
				t.Errorf("IsAuthError = %v, want %v", got, tt.wantAuth)
			}
			if got := IsNotFound(tt.err); got != tt.wantNotFound {
				t.Errorf("IsNotFound = %v, want %v", got, tt.wantNotFound)
			}
		})
	}
}
