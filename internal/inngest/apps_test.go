package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testV2App1 = `{"id": "` + testAppID1 + `", "name": "` + testMyApp + `", "externalID": "my-app", "sdkLanguage": "js", "sdkVersion": "3.0.0", "method": "SERVE", "functionCount": 1}`
	testV2App2 = `{"id": "` + testAppID2 + `", "name": "Other App", "method": "CONNECT"}`
	testV2Fn1  = `{"id": "` + testFnID1 + `", "name": "` + testSendEmail + `", "slug": "` + testSlugSend + `", "isPaused": false,
		"triggers": [{"type": "EVENT", "value": "` + testEventName + `", "if": "event.data.ok"}],
		"configuration": {"retries": {"value": 4, "isDefault": true}, "concurrency": [{"scope": "FUNCTION", "limit": {"value": 10}, "key": "event.data.id"}]},
		"app": {"id": "` + testAppID1 + `"}}`
	testV2Fn2 = `{"id": "` + testFnID2 + `", "name": "Other Fn", "slug": "other-fn", "app": {"id": "` + testAppID2 + `"}}`
)

// twoPageApps serves apps across two cursor pages.
func twoPageApps(t *testing.T, r *http.Request) string {
	requireQuery(t, r, map[string][]string{"limit": {"100"}})
	if r.URL.Query().Get("cursor") == testCursor2 {
		return `{"data": [` + testV2App2 + `], "page": {"hasMore": false}}`
	}
	requireNoQuery(t, r, "cursor")
	return `{"data": [` + testV2App1 + `], "page": {"cursor": "` + testCursor2 + `", "hasMore": true}}`
}

func TestListApps_PaginatesAndArchived(t *testing.T) {
	srv, counter := newV2Server(t, map[string]v2Route{
		"GET " + testPathV2Apps: func(t *testing.T, r *http.Request) string {
			requireNoQuery(t, r, "archived")
			return twoPageApps(t, r)
		},
	})
	apps, err := newCloudClient(srv).ListApps(context.Background(), false)
	if err != nil {
		t.Fatalf("ListApps: %v", err)
	}
	if len(apps) != 2 || apps[0].ID != testAppID1 || apps[1].ID != testAppID2 {
		t.Fatalf("apps = %+v", apps)
	}
	if apps[0].SDKLanguage != "js" || apps[0].FunctionCount != 1 || apps[0].Method != "SERVE" {
		t.Errorf("app fields = %+v", apps[0])
	}
	if counter.count() != 2 {
		t.Errorf("requests = %d, want 2 pages", counter.count())
	}

	archSrv, _ := newV2Server(t, map[string]v2Route{
		"GET " + testPathV2Apps: func(t *testing.T, r *http.Request) string {
			requireQuery(t, r, map[string][]string{"archived": {"true"}})
			return testEmptyListResp
		},
	})
	archived, err := newCloudClient(archSrv).ListApps(context.Background(), true)
	if err != nil || len(archived) != 0 || archived == nil {
		t.Errorf("ListApps(archived) = (%v, %v), want empty non-nil", archived, err)
	}
}

func TestListApps_Dev(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{"DevApps": `{"data":{"apps":[` + testV2App1 + `]}}`})
	apps, err := newDevClient(srv).ListApps(context.Background(), false)
	if err != nil || len(apps) != 1 || apps[0].Name != testMyApp {
		t.Fatalf("ListApps(dev) = (%+v, %v)", apps, err)
	}
	nilSrv, _ := newDevGQLServer(t, map[string]string{"DevApps": `{"data":{"apps":null}}`})
	apps, err = newDevClient(nilSrv).ListApps(context.Background(), false)
	if err != nil || apps == nil || len(apps) != 0 {
		t.Errorf("null apps = (%v, %v), want empty slice", apps, err)
	}
}

func TestGetApp_And_SyncApp(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"GET /v2/apps/" + testAppID1: staticRoute(`{"data": ` + testV2App1 + `}`),
		"POST /v2/apps/" + testAppID1 + "/syncs": func(t *testing.T, r *http.Request) string {
			if body := decodeJSONBody(t, r); body["url"] != testAppURL {
				t.Errorf("sync body = %v", body)
			}
			return `{"data": {"id": "sync-1", "appId": "` + testAppID1 + `", "status": "PENDING"}}`
		},
	})
	client := newCloudClient(srv)

	app, err := client.GetApp(context.Background(), testAppID1)
	if err != nil || app.Name != testMyApp {
		t.Fatalf("GetApp = (%+v, %v)", app, err)
	}
	sync, err := client.SyncApp(context.Background(), testAppID1, testAppURL)
	if err != nil || sync.ID != "sync-1" || sync.Status != "PENDING" {
		t.Fatalf("SyncApp = (%+v, %v)", sync, err)
	}
}

func TestGetApp_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, testNotFoundResp)
	}))
	defer srv.Close()
	if _, err := newCloudClient(srv).GetApp(context.Background(), "nope"); !IsNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}
}
