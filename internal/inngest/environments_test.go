package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testV2Envs = `{"data": [
  {"id": "env-1", "name": "Production", "type": "PRODUCTION", "isArchived": false, "createdAt": "` + testTimeQueued + `"},
  {"id": "env-2", "name": "staging", "type": "BRANCH", "isArchived": false}
], "page": {"hasMore": false}}`

func TestListEnvironments_Paginates(t *testing.T) {
	srv, counter := newV2Server(t, map[string]v2Route{
		"GET " + testPathV2Envs: func(t *testing.T, r *http.Request) string {
			requireQuery(t, r, map[string][]string{"limit": {"100"}})
			if r.URL.Query().Get("cursor") == testCursor2 {
				return testV2Envs
			}
			return `{"data": [{"id": "env-0", "name": "first", "type": "TEST"}], "page": {"cursor": "` + testCursor2 + `", "hasMore": true}}`
		},
	})
	envs, err := newCloudClient(srv).ListEnvironments(context.Background())
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	if len(envs) != 3 || counter.count() != 2 {
		t.Fatalf("got %d envs over %d requests, want 3 over 2", len(envs), counter.count())
	}
	if envs[1].Type != "PRODUCTION" || envs[1].CreatedAt == nil {
		t.Errorf("env = %+v", envs[1])
	}
}

func TestListEnvironments_Dev(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"GET /api/v2/envs": staticRoute(testV2Envs),
	})
	// newV2Server asserts the cloud bearer; dev mode sends none, so use a bare client.
	client := NewClient(ClientOptions{DevServerURL: srv.URL, DevMode: true, SigningKey: testAPIKey})
	envs, err := client.ListEnvironments(context.Background())
	if err != nil || len(envs) != 2 {
		t.Fatalf("ListEnvironments(dev) = (%d, %v)", len(envs), err)
	}
}

func TestGetEnvironment(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{"GET " + testPathV2Envs: staticRoute(testV2Envs)})
	client := newCloudClient(srv)

	byName, err := client.GetEnvironment(context.Background(), "production")
	if err != nil || byName.ID != "env-1" {
		t.Errorf("by name (case-insensitive) = (%+v, %v)", byName, err)
	}
	byID, err := client.GetEnvironment(context.Background(), "env-2")
	if err != nil || byID.Name != "staging" {
		t.Errorf("by id = (%+v, %v)", byID, err)
	}
	if _, err := client.GetEnvironment(context.Background(), "nope"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown env error = %v", err)
	}
}

func TestListEnvironments_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, testUnauthorized)
	}))
	defer srv.Close()
	_, err := newCloudClient(srv).ListEnvironments(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("want auth error, got %v", err)
	}
}
