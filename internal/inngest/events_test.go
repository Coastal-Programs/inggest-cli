package inngest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testV1Event = `{"internal_id": "` + testEventID1 + `", "id": "evt-ext", "name": "` + testEventName + `", "data": {"userId": "1"}, "ts": 1756720800000, "received_at": "` + testTimeQueued + `"}`

func TestSendEvent_RESTWithoutEventKey(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"POST " + testPathV2Events: func(t *testing.T, r *http.Request) string {
			body := decodeJSONBody(t, r)
			if body["name"] != testEventName || body["id"] != "idem-1" || body["ts"] != float64(1756720800000) {
				t.Errorf("body = %v", body)
			}
			if _, ok := body["user"]; ok {
				t.Error("user should be omitted when nil")
			}
			return `{"data": {"eventId": "` + testEventID1 + `"}}`
		},
	})
	ids, err := newCloudClient(srv).SendEvent(context.Background(), EventInput{
		Name: testEventName, Data: map[string]any{"userId": "1"}, ID: "idem-1", TS: 1756720800000,
	})
	if err != nil || len(ids) != 1 || ids[0] != testEventID1 {
		t.Fatalf("SendEvent = (%v, %v)", ids, err)
	}
}

func TestSendEvent_EventAPI(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/e/evtkey-123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if body := decodeJSONBody(t, r); body["name"] != testEventName {
			t.Errorf("body = %v", body)
		}
		writeJSON(w, http.StatusOK, `{"ids": ["`+testEventID1+`"], "status": 200}`)
	}))
	defer srv.Close()

	client := NewClient(ClientOptions{DevServerURL: srv.URL, DevMode: true, EventKey: "evtkey-123", SigningKey: testAPIKey})
	ids, err := client.SendEvent(context.Background(), EventInput{Name: testEventName, Data: map[string]any{}})
	if err != nil || len(ids) != 1 {
		t.Fatalf("SendEvent = (%v, %v)", ids, err)
	}
	if gotAuth != "" {
		t.Error("event API request must not carry the signing key")
	}
}

func TestSendEvent_EventAPIErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"non-2xx", http.StatusBadRequest, `{"error": "bad"}`, "status 400"},
		{"invalid json", http.StatusOK, `not json`, "unmarshal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, tt.status, tt.body)
			}))
			defer srv.Close()
			// Dev mode without an event key falls back to the "test" key.
			_, err := newDevClient(srv).SendEvent(context.Background(), EventInput{Name: testEventName})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestGetEventRuns_Cloud(t *testing.T) {
	srv, counter := newV2Server(t, map[string]v2Route{
		"GET /v2/events/" + testEventID1 + "/runs": func(t *testing.T, r *http.Request) string {
			requireQuery(t, r, map[string][]string{"includeOutput": {"true"}, "limit": {"100"}})
			if r.URL.Query().Get("cursor") == testCursor2 {
				return `{"data": [], "page": {"hasMore": false}}`
			}
			return `{"data": [` + testV2Run + `], "page": {"cursor": "` + testCursor2 + `", "hasMore": true}}`
		},
	})
	runs, err := newCloudClient(srv).GetEventRuns(context.Background(), testEventID1)
	if err != nil || len(runs) != 1 || runs[0].FunctionID != testFnID1 || counter.count() != 2 {
		t.Fatalf("GetEventRuns = (%+v, %v) over %d requests", runs, err, counter.count())
	}
}

func TestGetEventRuns_Dev(t *testing.T) {
	srv, rec := newDevGQLServer(t, map[string]string{
		"DevEventRuns": `{"data":{"eventV2":{"runs":[{"id":"` + testRunID1 + `","status":"COMPLETED"}]}}}`,
	})
	runs, err := newDevClient(srv).GetEventRuns(context.Background(), testEventID1)
	if err != nil || len(runs) != 1 || runs[0].ID != testRunID1 {
		t.Fatalf("GetEventRuns(dev) = (%+v, %v)", runs, err)
	}
	if rec.last(t).Variables["id"] != testEventID1 {
		t.Errorf("id variable = %v", rec.last(t).Variables["id"])
	}

	missing, _ := newDevGQLServer(t, map[string]string{"DevEventRuns": `{"data":{"eventV2":null}}`})
	if _, err := newDevClient(missing).GetEventRuns(context.Background(), "x"); !IsNotFound(err) {
		t.Errorf("missing event err = %v", err)
	}
	empty, _ := newDevGQLServer(t, map[string]string{"DevEventRuns": `{"data":{"eventV2":{"runs":null}}}`})
	runs, err = newDevClient(empty).GetEventRuns(context.Background(), "x")
	if err != nil || runs == nil || len(runs) != 0 {
		t.Errorf("null runs = (%v, %v), want empty slice", runs, err)
	}
}

func TestListEvents(t *testing.T) {
	before := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRequest(t, r, http.MethodGet, "/v1/events", testBearerAPIKey)
		requireQuery(t, r, map[string][]string{
			"name": {testEventName}, "limit": {"5"}, "cursor": {"c1"},
			"received_after": {"2026-09-01T00:00:00Z"}, "received_before": {"2026-09-02T00:00:00Z"},
		})
		writeJSON(w, http.StatusOK, `{"data": [`+testV1Event+`]}`)
	}))
	defer srv.Close()

	events, err := newCloudClient(srv).ListEvents(context.Background(), ListEventsOptions{
		Name: testEventName, Limit: 5, Cursor: "c1",
		ReceivedAfter: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), ReceivedBefore: &before,
	})
	if err != nil || len(events) != 1 {
		t.Fatalf("ListEvents = (%v, %v)", events, err)
	}
	e := events[0]
	if e.InternalID != testEventID1 || e.Name != testEventName || e.ReceivedAt == nil || string(e.Data) != `{"userId": "1"}` {
		t.Errorf("event = %+v", e)
	}
}

func TestListEvents_EmptyAndError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireNoQuery(t, r, "name", "limit", "cursor", "received_after", "received_before")
		writeJSON(w, http.StatusOK, `{"data": null}`)
	}))
	defer srv.Close()
	events, err := newCloudClient(srv).ListEvents(context.Background(), ListEventsOptions{})
	if err != nil || events == nil || len(events) != 0 {
		t.Errorf("null data = (%v, %v), want empty slice", events, err)
	}

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, `{"error": "Unauthorized", "status": 401}`)
	}))
	defer authSrv.Close()
	_, err = newCloudClient(authSrv).ListEvents(context.Background(), ListEventsOptions{})
	if !IsAuthError(err) || !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("v1 401 → %v, want APIError with server message", err)
	}
}

func TestGetEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/v1/events/" + testEventID1:
			writeJSON(w, http.StatusOK, `{"data": `+testV1Event+`}`)
		case "/v1/events/a%2Fb":
			writeJSON(w, http.StatusOK, `{"data": null}`)
		default:
			t.Errorf("unexpected path %s", r.URL.EscapedPath())
		}
	}))
	defer srv.Close()
	client := newCloudClient(srv)

	event, err := client.GetEvent(context.Background(), testEventID1)
	if err != nil || event.Name != testEventName {
		t.Fatalf("GetEvent = (%+v, %v)", event, err)
	}
	if _, err := client.GetEvent(context.Background(), "a/b"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("null data err = %v", err)
	}
}

func TestEvent_UnmarshalReceivedAtVariants(t *testing.T) {
	var snake, camel Event
	if err := snake.UnmarshalJSON([]byte(`{"name":"a","received_at":"` + testTimeQueued + `"}`)); err != nil || snake.ReceivedAt == nil {
		t.Errorf("snake_case: %v %v", snake.ReceivedAt, err)
	}
	if err := camel.UnmarshalJSON([]byte(`{"name":"a","receivedAt":"` + testTimeQueued + `"}`)); err != nil || camel.ReceivedAt == nil {
		t.Errorf("camelCase: %v %v", camel.ReceivedAt, err)
	}
	if err := camel.UnmarshalJSON([]byte(`{"name": 1}`)); err == nil {
		t.Error("expected type error")
	}
}

func TestListEventSchemas_Cloud(t *testing.T) {
	srv, _ := newV2Server(t, map[string]v2Route{
		"GET /v2/insights/events/schemas": func(t *testing.T, r *http.Request) string {
			requireQuery(t, r, map[string][]string{"limit": {"100"}})
			return `{"data": [{"name": "` + testEventName + `", "schema": {"type": "object"}}, {"name": "b"}], "page": {"hasMore": false}}`
		},
	})
	schemas, err := newCloudClient(srv).ListEventSchemas(context.Background())
	if err != nil || len(schemas) != 2 || schemas[0].Name != testEventName || string(schemas[0].Schema) != `{"type": "object"}` {
		t.Fatalf("ListEventSchemas = (%+v, %v)", schemas, err)
	}
}

func TestListEventSchemas_DevDerivesNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRequest(t, r, http.MethodGet, "/v1/events", "")
		requireQuery(t, r, map[string][]string{"limit": {"500"}})
		writeJSON(w, http.StatusOK, `{"data": [{"internal_id":"1","name":"zeta"},{"internal_id":"2","name":"alpha"},{"internal_id":"3","name":"zeta"}]}`)
	}))
	defer srv.Close()
	schemas, err := newDevClient(srv).ListEventSchemas(context.Background())
	if err != nil || len(schemas) != 2 || schemas[0].Name != "alpha" || schemas[1].Name != "zeta" {
		t.Fatalf("dev schemas = (%+v, %v), want sorted distinct names", schemas, err)
	}
}
