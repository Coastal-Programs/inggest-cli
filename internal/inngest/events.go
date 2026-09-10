package inngest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// ListEventsOptions configures the events list query (GET /v1/events).
type ListEventsOptions struct {
	Name           string
	Limit          int
	Cursor         string     // internal ID of the last event from the previous page
	ReceivedAfter  time.Time  // zero = unbounded
	ReceivedBefore *time.Time // nil = unbounded
}

// EventInput is the body accepted by the v2 send endpoint (one event per call).
type EventInput struct {
	Name string `json:"name"`
	Data any    `json:"data"`
	ID   string `json:"id,omitempty"`
	TS   int64  `json:"ts,omitempty"` // Unix milliseconds
	User any    `json:"user,omitempty"`
}

// SendEvent sends an event and returns its ID(s). With an event key configured
// (or in dev mode) it uses the Event API (POST https://inn.gs/e/{eventKey});
// otherwise it falls back to the REST API (POST /v2/events), which is
// authenticated by the signing/API key and intended for testing and debugging.
func (c *Client) SendEvent(ctx context.Context, event EventInput) ([]string, error) {
	if c.eventKey == "" && !c.devMode {
		return c.sendEventREST(ctx, event)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("inngest: marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.eventURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("inngest: create send event request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// The Event API authenticates via the event key in the URL path, not a
	// signing key Bearer token; doEvent skips the Authorization header.
	resp, err := c.doEvent(req)
	if err != nil {
		return nil, fmt.Errorf("inngest: send event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("inngest: read send event response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inngest: send event returned status %d: %s", resp.StatusCode, truncateBody(string(respBody)))
	}

	var result struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("inngest: unmarshal send event response: %w", err)
	}
	return result.IDs, nil
}

func (c *Client) sendEventREST(ctx context.Context, event EventInput) ([]string, error) {
	var data struct {
		EventID string `json:"eventId"`
	}
	if err := c.v2Post(ctx, "events", event, &data); err != nil {
		return nil, fmt.Errorf("inngest: send event: %w", err)
	}
	return []string{data.EventID}, nil
}

// GetEventRuns fetches the runs triggered by an event (GET /v2/events/{eventId}/runs).
func (c *Client) GetEventRuns(ctx context.Context, eventID string) ([]FunctionRun, error) {
	if c.devMode {
		return c.devEventRuns(ctx, eventID)
	}
	q := url.Values{"includeOutput": {"true"}, "limit": {strconv.Itoa(v2ListPageSize)}}
	runs := []FunctionRun{}
	for range maxListPages {
		var data []v2Run
		page, err := c.v2Get(ctx, "events/"+url.PathEscape(eventID)+"/runs", q, &data)
		if err != nil {
			return nil, fmt.Errorf("inngest: get event runs: %w", err)
		}
		for _, r := range data {
			runs = append(runs, r.toFunctionRun())
		}
		if page == nil || !page.HasMore || page.Cursor == "" {
			return runs, nil
		}
		q.Set("cursor", page.Cursor)
	}
	return runs, nil
}

// ListEvents lists recent event instances (GET /v1/events), newest first.
func (c *Client) ListEvents(ctx context.Context, opts ListEventsOptions) ([]Event, error) {
	q := url.Values{}
	if opts.Name != "" {
		q.Set("name", opts.Name)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	if !opts.ReceivedAfter.IsZero() {
		q.Set("received_after", opts.ReceivedAfter.UTC().Format(time.RFC3339))
	}
	if opts.ReceivedBefore != nil {
		q.Set("received_before", opts.ReceivedBefore.UTC().Format(time.RFC3339))
	}

	var result struct {
		Data []Event `json:"data"`
	}
	if err := c.GetREST(ctx, "events?"+q.Encode(), &result); err != nil {
		return nil, fmt.Errorf("inngest: list events: %w", err)
	}
	if result.Data == nil {
		result.Data = []Event{}
	}
	return result.Data, nil
}

// GetEvent fetches a single event by its internal ID (GET /v1/events/{internalID}).
func (c *Client) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	var result struct {
		Data *Event `json:"data"`
	}
	if err := c.GetREST(ctx, "events/"+url.PathEscape(eventID), &result); err != nil {
		return nil, fmt.Errorf("inngest: get event: %w", err)
	}
	if result.Data == nil {
		return nil, fmt.Errorf("inngest: event %s not found", eventID)
	}
	return result.Data, nil
}

// ListEventSchemas lists the event types seen in the environment with the inferred
// shape of their data (GET /v2/insights/events/schemas). The dev server has no
// schema endpoint, so there the names are derived from recent events.
func (c *Client) ListEventSchemas(ctx context.Context) ([]EventSchema, error) {
	if c.devMode {
		return c.devEventTypes(ctx)
	}
	q := url.Values{"limit": {strconv.Itoa(v2ListPageSize)}}
	schemas := []EventSchema{}
	for range maxListPages {
		var data []EventSchema
		page, err := c.v2Get(ctx, "insights/events/schemas", q, &data)
		if err != nil {
			return nil, fmt.Errorf("inngest: list event schemas: %w", err)
		}
		schemas = append(schemas, data...)
		if page == nil || !page.HasMore || page.Cursor == "" {
			return schemas, nil
		}
		q.Set("cursor", page.Cursor)
	}
	return schemas, nil
}

// devEventTypeSample is how many recent dev-server events are scanned for names.
const devEventTypeSample = 500

// devEventTypes derives distinct event names from recent dev-server events.
func (c *Client) devEventTypes(ctx context.Context) ([]EventSchema, error) {
	events, err := c.ListEvents(ctx, ListEventsOptions{Limit: devEventTypeSample})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	schemas := []EventSchema{}
	for _, e := range events {
		if !seen[e.Name] {
			seen[e.Name] = true
			schemas = append(schemas, EventSchema{Name: e.Name})
		}
	}
	sort.Slice(schemas, func(i, j int) bool { return schemas[i].Name < schemas[j].Name })
	return schemas, nil
}
