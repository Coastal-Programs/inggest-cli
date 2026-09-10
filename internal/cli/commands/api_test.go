package commands

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
)

func runAPI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewAPICmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return out.String(), err
}

func TestAPI_GetPrettyPrints(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.RawQuery != "limit=1" {
				t.Errorf("request = %s %s", r.Method, r.URL.String())
			}
			if r.Header.Get("Authorization") == "" {
				t.Error("expected credential on passthrough request")
			}
			jsonOK(`{"data":[{"id":"r1"}]}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runAPI(t, "/v2/runs?limit=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "\n  \"data\"") {
		t.Errorf("expected indented JSON, got:\n%s", got)
	}

	got, err = runAPI(t, "/v2/runs?limit=1", "--raw")
	if err != nil || got != `{"data":[{"id":"r1"}]}`+"\n" {
		t.Errorf("--raw output = %q (%v)", got, err)
	}
}

func TestAPI_BodyImpliesPost(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/events": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || string(body) != `{"name":"x"}` || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("request = %s %s body=%s", r.Method, r.Header.Get("Content-Type"), body)
			}
			jsonOK(`{"data":{"eventId":"01EV"}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	if _, err := runAPI(t, "/v2/events", "--body", `{"name":"x"}`); err != nil {
		t.Errorf("--body: %v", err)
	}

	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte(`{"name":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runAPI(t, "/v2/events", "-X", "POST", "--body-file", file); err != nil {
		t.Errorf("--body-file: %v", err)
	}
	if _, err := runAPI(t, "/v2/events", "--body-file", filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("expected error for missing body file")
	}
}

func TestAPI_ErrorStatusExitsNonZero(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/runs":  jsonStatus(http.StatusUnauthorized, v2Unauthorized),
		"/v1/oops":  jsonStatus(http.StatusBadRequest, `{"error":"bad request","status":400}`),
		"/v2/plain": jsonStatus(http.StatusBadGateway, `upstream down`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runAPI(t, "/v2/runs")
	if !inngest.IsAuthError(err) || !strings.Contains(err.Error(), "authorization_header_missing") {
		t.Errorf("401: err = %v", err)
	}
	if !strings.Contains(got, "authorization header missing") {
		t.Errorf("body should still be printed on error, got %q", got)
	}
	if _, err := runAPI(t, "/v1/oops"); err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("v1 error: %v", err)
	}
	if _, err := runAPI(t, "/v2/plain"); err == nil || !strings.Contains(err.Error(), "502") {
		t.Errorf("non-JSON error: %v", err)
	}
}

func TestAPI_RejectsAbsoluteURL(t *testing.T) {
	setupCloudState(t, "http://127.0.0.1:1")
	if _, err := runAPI(t, "https://evil.example/v2/runs"); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Errorf("absolute URL: %v", err)
	}
}

// TestReadBodyInput covers every input source of readBodyInput, including the
// stdin and file error branches.
func TestReadBodyInput(t *testing.T) {
	t.Run("inline body wins", func(t *testing.T) {
		got, err := readBodyInput(`{"a":1}`, "ignored.json")
		if err != nil {
			t.Fatalf("readBodyInput returned error: %v", err)
		}
		if string(got) != `{"a":1}` {
			t.Errorf("got %q, want the inline body", got)
		}
	})

	t.Run("dash reads stdin", func(t *testing.T) {
		setStdin(t, `{"from":"stdin"}`)
		got, err := readBodyInput("", "-")
		if err != nil {
			t.Fatalf("readBodyInput returned error: %v", err)
		}
		if string(got) != `{"from":"stdin"}` {
			t.Errorf("got %q, want the stdin body", got)
		}
	})

	t.Run("stdin read error", func(t *testing.T) {
		setStdinClosed(t)
		_, err := readBodyInput("", "-")
		if err == nil || !strings.Contains(err.Error(), "reading stdin") {
			t.Errorf("err = %v, want a stdin read error", err)
		}
	})

	t.Run("reads a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "body.json")
		if err := os.WriteFile(path, []byte(`{"from":"file"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readBodyInput("", path)
		if err != nil {
			t.Fatalf("readBodyInput returned error: %v", err)
		}
		if string(got) != `{"from":"file"}` {
			t.Errorf("got %q, want the file body", got)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := readBodyInput("", filepath.Join(t.TempDir(), "nope.json"))
		if err == nil || !strings.Contains(err.Error(), "reading ") {
			t.Errorf("err = %v, want a file read error", err)
		}
	})
}
