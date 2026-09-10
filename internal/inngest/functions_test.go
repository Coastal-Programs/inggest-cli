package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// cloudFunctionsRoutes stubs apps + per-app functions for cloud ListFunctions.
func cloudFunctionsRoutes() map[string]v2Route {
	return map[string]v2Route{
		"GET " + testPathV2Apps:                                  twoPageApps,
		"GET /v2/apps/" + testAppID1 + "/functions":              staticRoute(`{"data": [` + testV2Fn1 + `], "page": {"hasMore": false}}`),
		"GET /v2/apps/" + testAppID2 + "/functions":              staticRoute(`{"data": [` + testV2Fn2 + `], "page": {"hasMore": false}}`),
		"GET /v2/apps/" + testAppID1 + "/functions/" + testFnID1: staticRoute(`{"data": ` + testV2Fn1 + `}`),
	}
}

func TestListFunctions_CloudAttachesApp(t *testing.T) {
	srv, _ := newV2Server(t, cloudFunctionsRoutes())
	fns, err := newCloudClient(srv).ListFunctions(context.Background())
	if err != nil {
		t.Fatalf("ListFunctions: %v", err)
	}
	if len(fns) != 2 {
		t.Fatalf("got %d functions, want 2", len(fns))
	}
	fn := fns[0]
	if fn.Slug != testSlugSend || fn.App == nil || fn.App.Name != testMyApp {
		t.Errorf("fn = %+v app = %+v, want full app attached", fn, fn.App)
	}
	if len(fn.Triggers) != 1 || fn.Triggers[0].Condition != "event.data.ok" {
		t.Errorf("triggers = %+v, want condition decoded from \"if\"", fn.Triggers)
	}
	if fn.Configuration == nil || fn.Configuration.Retries.Value != 4 || fn.Configuration.Concurrency[0].Limit.Value != 10 {
		t.Errorf("configuration = %+v", fn.Configuration)
	}
	if fns[1].App.Name != "Other App" {
		t.Errorf("second fn app = %+v", fns[1].App)
	}
}

func TestListAppFunctions_Paginates(t *testing.T) {
	srv, counter := newV2Server(t, map[string]v2Route{
		"GET /v2/apps/" + testAppID1 + "/functions": func(t *testing.T, r *http.Request) string {
			if r.URL.Query().Get("cursor") == testCursor2 {
				return `{"data": [` + testV2Fn2 + `], "page": {"hasMore": false}}`
			}
			return `{"data": [` + testV2Fn1 + `], "page": {"cursor": "` + testCursor2 + `", "hasMore": true}}`
		},
	})
	fns, err := newCloudClient(srv).ListAppFunctions(context.Background(), testAppID1)
	if err != nil || len(fns) != 2 || counter.count() != 2 {
		t.Fatalf("ListAppFunctions = (%d fns, %v) over %d requests", len(fns), err, counter.count())
	}
}

func TestGetFunction(t *testing.T) {
	srv, _ := newV2Server(t, cloudFunctionsRoutes())
	client := newCloudClient(srv)

	bySlug, err := client.GetFunction(context.Background(), testSlugSend)
	if err != nil || bySlug.ID != testFnID1 {
		t.Errorf("by slug = (%+v, %v)", bySlug, err)
	}
	byID, err := client.GetFunction(context.Background(), testFnID2)
	if err != nil || byID.Slug != "other-fn" {
		t.Errorf("by id = (%+v, %v)", byID, err)
	}
	if _, err := client.GetFunction(context.Background(), "nope"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown function error = %v", err)
	}
}

func TestListFunctions_CloudError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, testUnauthorized)
	}))
	defer srv.Close()
	if _, err := newCloudClient(srv).ListFunctions(context.Background()); !IsAuthError(err) {
		t.Fatalf("want auth error, got %v", err)
	}
}

func TestListFunctions_Dev(t *testing.T) {
	srv, _ := newDevGQLServer(t, map[string]string{"DevFunctions": `{"data":{"functions":[` + testV2Fn1 + `]}}`})
	fns, err := newDevClient(srv).ListFunctions(context.Background())
	if err != nil || len(fns) != 1 || fns[0].Triggers[0].Condition != "event.data.ok" {
		t.Fatalf("ListFunctions(dev) = (%+v, %v)", fns, err)
	}
	nilSrv, _ := newDevGQLServer(t, map[string]string{"DevFunctions": `{"data":{"functions":null}}`})
	fns, err = newDevClient(nilSrv).ListFunctions(context.Background())
	if err != nil || fns == nil || len(fns) != 0 {
		t.Errorf("null functions = (%v, %v), want empty slice", fns, err)
	}
}

func TestInvokeFunction_Cloud(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"POST /v2/apps/" + testAppID1 + "/functions/" + testFnID1 + "/invoke": func(t *testing.T, r *http.Request) string {
			body := decodeJSONBody(t, r)
			data, _ := body["data"].(map[string]any)
			if data["hello"] != "world" {
				t.Errorf("data = %v", body["data"])
			}
			if body["idempotencyKey"] != "k1" {
				t.Errorf("idempotencyKey = %v", body["idempotencyKey"])
			}
			return `{"data": {"runId": "` + testRunID1 + `", "queuedAt": "` + testTimeQueued + `"}}`
		},
	})
	res, err := newCloudClient(srv).InvokeFunction(context.Background(), testAppID1, testFnID1, map[string]any{"hello": "world"}, "k1")
	if err != nil || res.RunID != testRunID1 || res.QueuedAt == nil {
		t.Fatalf("InvokeFunction = (%+v, %v)", res, err)
	}
}

func TestInvokeFunction_CloudDefaults(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"POST /v2/apps/" + testAppID1 + "/functions/" + testFnID1 + "/invoke": func(t *testing.T, r *http.Request) string {
			body := decodeJSONBody(t, r)
			if data, ok := body["data"].(map[string]any); !ok || len(data) != 0 {
				t.Errorf("nil data should be sent as {}, got %v", body["data"])
			}
			if _, ok := body["idempotencyKey"]; ok {
				t.Error("idempotencyKey should be omitted when empty")
			}
			return `{"data": {"runId": "` + testRunID1 + `"}}`
		},
	})
	if _, err := newCloudClient(srv).InvokeFunction(context.Background(), testAppID1, testFnID1, nil, ""); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeFunction_Dev(t *testing.T) {
	srv, rec := newDevGQLServer(t, map[string]string{
		"DevFunctions": `{"data":{"functions":[{"id":"` + testFnID1 + `","slug":"` + testSlugSend + `"}]}}`,
		"DevInvoke":    `{"data":{"invokeFunction":true}}`,
		"DevRuns":      `{"data":{"runs":{"edges":[{"node":{"id":"` + testRunID1 + `","status":"QUEUED"}}],"pageInfo":{}}}}`,
	})
	res, err := newDevClient(srv).InvokeFunction(context.Background(), "", testSlugSend, map[string]any{"a": 1}, "")
	if err != nil || res.RunID != testRunID1 {
		t.Fatalf("InvokeFunction(dev) = (%+v, %v)", res, err)
	}
	// The final call is the DevRuns poll scoped to the resolved function UUID.
	filter := rec.last(t).Variables["filter"].(map[string]any)
	if ids := filter["functionIDs"].([]any); len(ids) != 1 || ids[0] != testFnID1 {
		t.Errorf("poll functionIDs = %v", ids)
	}
}

func TestInvokeFunction_DevErrors(t *testing.T) {
	unknown, _ := newDevGQLServer(t, map[string]string{"DevFunctions": `{"data":{"functions":[]}}`})
	if _, err := newDevClient(unknown).InvokeFunction(context.Background(), "", "nope", nil, ""); err == nil || !strings.Contains(err.Error(), "not found on dev server") {
		t.Errorf("unknown slug error = %v", err)
	}

	rejected, _ := newDevGQLServer(t, map[string]string{
		"DevFunctions": `{"data":{"functions":[{"id":"` + testFnID1 + `","slug":"` + testSlugSend + `"}]}}`,
		"DevInvoke":    `{"data":{"invokeFunction":false}}`,
	})
	if _, err := newDevClient(rejected).InvokeFunction(context.Background(), "", testSlugSend, nil, ""); err == nil || !strings.Contains(err.Error(), "did not accept") {
		t.Errorf("rejected invoke error = %v", err)
	}
}

// TestListAppFunctions_MaxPagesGuard proves the per-app cursor loop is bounded.
func TestListAppFunctions_MaxPagesGuard(t *testing.T) {
	srv, counter := newV2Server(t, map[string]v2Route{
		"GET /v2/apps/" + testAppID1 + "/functions": alwaysMoreRoute(testV2Fn1),
	})

	fns, err := newCloudClient(srv).ListAppFunctions(context.Background(), testAppID1)
	if err != nil {
		t.Fatalf("ListAppFunctions returned error: %v", err)
	}

	if got := counter.count(); got != maxListPages {
		t.Errorf("request count = %d, want the maxListPages cap of %d", got, maxListPages)
	}
	if len(fns) != maxListPages {
		t.Errorf("collected %d functions, want %d (one per page)", len(fns), maxListPages)
	}
}

// TestListAppFunctions_Error covers the per-app request failure branch, which
// must name the app whose listing failed.
func TestListAppFunctions_Error(t *testing.T) {
	srv := newErrorServer(t, http.StatusInternalServerError, testServerErrorResp)

	_, err := newCloudClient(srv).ListAppFunctions(context.Background(), testAppID1)
	requireErrContains(t, err, "list functions for app "+testAppID1)
}

// TestListFunctions_AppListingError covers the branch where the initial app
// listing fails, so no per-app request is ever made.
func TestListFunctions_AppListingError(t *testing.T) {
	srv := newErrorServer(t, http.StatusInternalServerError, testServerErrorResp)

	_, err := newCloudClient(srv).ListFunctions(context.Background())
	requireErrContains(t, err, "list apps")
}

// TestGetFunction_ListError covers GetFunction's propagation of a failed
// underlying listing.
func TestGetFunction_ListError(t *testing.T) {
	srv := newErrorServer(t, http.StatusInternalServerError, testServerErrorResp)

	_, err := newCloudClient(srv).GetFunction(context.Background(), "my-func")
	requireErrContains(t, err, "list apps")
}

// TestInvokeFunction_Error covers the failed-invoke branch.
func TestInvokeFunction_Error(t *testing.T) {
	srv := newErrorServer(t, http.StatusInternalServerError, testServerErrorResp)

	_, err := newCloudClient(srv).InvokeFunction(context.Background(), testAppID1, testFnID1, nil, "")
	requireErrContains(t, err, "invoke function "+testFnID1)
}

// TestListFunctions_PerAppError covers the branch where the app listing
// succeeds but a per-app function listing fails, so the whole call fails
// rather than silently returning the functions of the apps that did work.
func TestListFunctions_PerAppError(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"GET " + testPathV2Apps: staticRoute(`{"data": [` + testV2App1 + `], "page": {"hasMore": false}}`),
		// Malformed JSON: the per-app request is answered, but decoding fails.
		"GET /v2/apps/" + testAppID1 + "/functions": staticRoute(`{"data": [`),
	})

	_, err := newCloudClient(srv).ListFunctions(context.Background())
	requireErrContains(t, err, "list functions for app "+testAppID1)
}
