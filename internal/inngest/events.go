package inngest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ListEventsOptions configures the ListEvents query.
type ListEventsOptions struct {
	First int
	Name  string
	Since time.Time
}

// SendEvent sends an event via the Event API (POST https://inn.gs/e/{eventKey}).
// Returns the event IDs.
func (c *Client) SendEvent(ctx context.Context, event interface{}) ([]string, error) {
	if c.eventKey == "" {
		return nil, fmt.Errorf("inngest: event key is required to send events")
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

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("inngest: send event: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("inngest: read send event response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inngest: send event returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		IDs    []string `json:"ids"`
		Status int      `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("inngest: unmarshal send event response: %w", err)
	}

	return result.IDs, nil
}

// GetEventRuns fetches runs triggered by an event (GET /v1/events/{eventId}/runs).
func (c *Client) GetEventRuns(ctx context.Context, eventID string) ([]FunctionRun, error) {
	var result struct {
		Data []struct {
			RunID      string     `json:"run_id"`
			Status     string     `json:"status"`
			FunctionID string     `json:"function_id"`
			StartedAt  *time.Time `json:"started_at,omitempty"`
			EndedAt    *time.Time `json:"ended_at,omitempty"`
			Output     string     `json:"output,omitempty"`
		} `json:"data"`
	}

	if err := c.GetREST(ctx, fmt.Sprintf("events/%s/runs", eventID), &result); err != nil {
		return nil, fmt.Errorf("inngest: get event runs: %w", err)
	}

	runs := make([]FunctionRun, len(result.Data))
	for i, r := range result.Data {
		runs[i] = FunctionRun{
			ID:         r.RunID,
			FunctionID: r.FunctionID,
			Status:     r.Status,
			StartedAt:  r.StartedAt,
			EndedAt:    r.EndedAt,
			Output:     r.Output,
		}
	}

	return runs, nil
}

// ListEvents queries events via GraphQL.
func (c *Client) ListEvents(ctx context.Context, opts ListEventsOptions) (*EventsConnection, error) {
	query := `query ListEvents($first: Int!, $filter: EventsFilter!) {
  eventsV2(first: $first, filter: $filter) {
    edges {
      node {
        id
        name
        occurredAt
        receivedAt
        raw
        runs {
          id
          status
          function {
            name
          }
        }
      }
      cursor
    }
    pageInfo {
      hasNextPage
      endCursor
    }
    totalCount
  }
}`

	filter := map[string]interface{}{}
	if !opts.Since.IsZero() {
		filter["from"] = opts.Since.Format(time.RFC3339)
	}
	if opts.Name != "" {
		filter["name"] = opts.Name
	}

	first := opts.First
	if first <= 0 {
		first = 20
	}

	variables := map[string]interface{}{
		"first":  first,
		"filter": filter,
	}

	var result struct {
		EventsV2 struct {
			Edges []struct {
				Node   eventV2Node `json:"node"`
				Cursor string      `json:"cursor"`
			} `json:"edges"`
			PageInfo   PageInfo `json:"pageInfo"`
			TotalCount int      `json:"totalCount"`
		} `json:"eventsV2"`
	}

	if err := c.ExecuteGraphQL(ctx, "ListEvents", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: list events: %w", err)
	}

	conn := &EventsConnection{
		PageInfo:   result.EventsV2.PageInfo,
		TotalCount: result.EventsV2.TotalCount,
	}

	conn.Edges = make([]EventEdge, len(result.EventsV2.Edges))
	for i, edge := range result.EventsV2.Edges {
		runs := make([]FunctionRun, len(edge.Node.Runs))
		for j, r := range edge.Node.Runs {
			runs[j] = FunctionRun{
				ID:     r.ID,
				Status: r.Status,
			}
			if r.Function != nil {
				runs[j].Function = &Function{Name: r.Function.Name}
			}
		}

		conn.Edges[i] = EventEdge{
			Node: Event{
				ID:         edge.Node.ID,
				Name:       edge.Node.Name,
				Raw:        edge.Node.Raw,
				ReceivedAt: edge.Node.ReceivedAt,
				Runs:       runs,
			},
			Cursor: edge.Cursor,
		}
		if edge.Node.OccurredAt != nil {
			conn.Edges[i].Node.CreatedAt = edge.Node.OccurredAt
		}
	}

	return conn, nil
}

// GetEvent gets a single event by ID via GraphQL.
func (c *Client) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	query := `query GetEvent($eventId: ID!) {
  event(query: {eventId: $eventId}) {
    id
    name
    occurredAt
    receivedAt
    raw
    runs {
      id
      status
      function {
        name
      }
    }
  }
}`

	variables := map[string]interface{}{
		"eventId": eventID,
	}

	var result struct {
		Event *eventV2Node `json:"event"`
	}

	if err := c.ExecuteGraphQL(ctx, "GetEvent", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: get event: %w", err)
	}

	if result.Event == nil {
		return nil, fmt.Errorf("inngest: event %s not found", eventID)
	}

	runs := make([]FunctionRun, len(result.Event.Runs))
	for i, r := range result.Event.Runs {
		runs[i] = FunctionRun{
			ID:     r.ID,
			Status: r.Status,
		}
		if r.Function != nil {
			runs[i].Function = &Function{Name: r.Function.Name}
		}
	}

	event := &Event{
		ID:         result.Event.ID,
		Name:       result.Event.Name,
		Raw:        result.Event.Raw,
		ReceivedAt: result.Event.ReceivedAt,
		Runs:       runs,
		TotalRuns:  len(runs),
	}
	if result.Event.OccurredAt != nil {
		event.CreatedAt = result.Event.OccurredAt
	}

	return event, nil
}

// eventV2Node is the GraphQL shape for event queries.
type eventV2Node struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	OccurredAt *time.Time `json:"occurredAt,omitempty"`
	ReceivedAt *time.Time `json:"receivedAt,omitempty"`
	Raw        string     `json:"raw,omitempty"`
	Runs       []eventRun `json:"runs,omitempty"`
}

// eventRun is the GraphQL shape for runs within an event query.
type eventRun struct {
	ID       string        `json:"id"`
	Status   string        `json:"status"`
	Function *eventRunFunc `json:"function,omitempty"`
}

// eventRunFunc is the function info within an event run.
type eventRunFunc struct {
	Name string `json:"name"`
}
