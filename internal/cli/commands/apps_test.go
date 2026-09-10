package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
)

const v2AppsJSON = `{"data":[{"id":"app-1","name":"my-app","method":"SERVE","functionCount":3,"sdkLanguage":"js","sdkVersion":"3.0.0"}],"page":{"hasMore":false}}`

func runApps(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewAppsCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	return out, err
}

func TestAppsList(t *testing.T) {
	var sawArchived bool
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/apps": func(w http.ResponseWriter, r *http.Request) {
			sawArchived = r.URL.Query().Get("archived") == queryTrue
			jsonOK(v2AppsJSON)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runApps(t, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var apps []map[string]any
	if err := json.Unmarshal([]byte(got), &apps); err != nil || len(apps) != 1 || apps[0]["id"] != "app-1" {
		t.Errorf("apps = %v (%v)", apps, err)
	}
	if sawArchived {
		t.Error("archived should not be sent by default")
	}

	if _, err := runApps(t, "list", "--archived"); err != nil || !sawArchived {
		t.Errorf("--archived: err=%v sent=%v", err, sawArchived)
	}

	state.Output = testOutputTable
	got, err = runApps(t, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"NAME", "METHOD", "FUNCTIONS", "SDK", "my-app", "SERVE", "js/3.0.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing %q:\n%s", want, got)
		}
	}
}

func TestAppsGetAndSync(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/apps/app-1": jsonOK(`{"data":{"id":"app-1","name":"my-app","latestSync":{"status":"SUCCESS","url":"https://x/api/inngest"}}}`),
		"/v2/apps/app-1/syncs": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if r.Method != http.MethodPost || body["url"] != "https://x/api/inngest" {
				t.Errorf("sync request = %s %v", r.Method, body)
			}
			jsonOK(`{"data":{"id":"sync-1","appId":"app-1","status":"PENDING"}}`)(w, r)
		},
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	got, err := runApps(t, "get", "app-1")
	if err != nil || !strings.Contains(got, `"SUCCESS"`) {
		t.Errorf("get: err=%v out=%s", err, got)
	}
	got, err = runApps(t, "sync", "app-1", "--url", "https://x/api/inngest")
	if err != nil || !strings.Contains(got, `"sync-1"`) {
		t.Errorf("sync: err=%v out=%s", err, got)
	}
	if _, err := runApps(t, "sync", "app-1"); err == nil || !strings.Contains(err.Error(), "url") {
		t.Errorf("sync without --url should fail, got %v", err)
	}
}

func TestAppsErrors(t *testing.T) {
	srv := newMockServer(t, nil, map[string]http.HandlerFunc{
		"/v2/apps":       jsonStatus(http.StatusUnauthorized, v2Unauthorized),
		"/v2/apps/app-1": jsonStatus(http.StatusNotFound, `{"errors":[{"code":"not_found","message":"app not found"}]}`),
	})
	defer srv.Close()
	setupCloudState(t, srv.URL)

	if _, err := runApps(t, "list"); err == nil || !strings.Contains(err.Error(), "listing apps") {
		t.Errorf("list err = %v", err)
	}
	if _, err := runApps(t, "get", "app-1"); err == nil || !strings.Contains(err.Error(), "app not found") {
		t.Errorf("get err = %v", err)
	}
}
