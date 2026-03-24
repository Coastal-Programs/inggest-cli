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
	Name        string
	RecentCount int // Number of recent event instances per type to fetch.
}

// SendEvent sends an event via the Event API (POST https://inn.gs/e/{eventKey}).
// Returns the event IDs.
func (c *Client) SendEvent(ctx context.Context, event any) ([]string, error) {
	if c.eventKey == "" {
		return nil, fmt.Errorf("inngest: event key is required to send events — set INNGEST_EVENT_KEY or use 'inngest auth login --event-key'")
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

	// The event ingestion endpoint (inn.gs/e/{eventKey}) authenticates via
	// the event key in the URL path — not via a signing key Bearer token.
	// Use doEvent instead of do to avoid injecting signing key auth headers
	// which would cause 401 errors on the event ingestion endpoint.
	resp, err := c.doEvent(req)
	if err != nil {
		return nil, fmt.Errorf("inngest: send event: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("inngest: read send event response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inngest: send event returned status %d: %s", resp.StatusCode, truncateBody(string(respBody)))
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

// ListEvents queries event types via the GraphQL `events` query.
// The API returns event types (not individual instances). Use the `recent`
// field on each type to access actual event instances.
func (c *Client) ListEvents(ctx context.Context, opts ListEventsOptions) (*EventTypesResult, error) {
	recentCount := opts.RecentCount
	if recentCount <= 0 {
		recentCount = 5
	}

	query := `query ListEvents($name: String, $recentCount: Int!) {
  events(query: {name: $name}) {
    data {
      name
      description
      firstSeen
      usage { total }
      workflows {
        id
        name
        slug
        triggers { type value condition }
        app { id name externalID }
      }
      recent(count: $recentCount) {
        id
        occurredAt
        receivedAt
        name
        event
        version
        functionRuns {
          id
          status
          startedAt
          endedAt
          output
          function { id name slug }
        }
      }
    }
    page {
      page
      perPage
      totalItems
      totalPages
    }
  }
}`

	variables := map[string]any{
		"recentCount": recentCount,
	}
	if opts.Name != "" {
		variables["name"] = opts.Name
	}

	var result struct {
		Events EventTypesResult `json:"events"`
	}

	if err := c.ExecuteGraphQL(ctx, "ListEvents", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: list events: %w", err)
	}

	return &result.Events, nil
}

// GetEvent finds a single event instance by ID. It queries all event types
// and searches their recent instances for a matching ID.
func (c *Client) GetEvent(ctx context.Context, eventID string) (*ArchivedEvent, error) {
	query := `query GetEvent($recentCount: Int!) {
  events(query: {}) {
    data {
      name
      recent(count: $recentCount) {
        id
        occurredAt
        receivedAt
        name
        event
        version
        functionRuns {
          id
          status
          startedAt
          endedAt
          output
          function { id name slug }
        }
      }
    }
  }
}`

	variables := map[string]any{
		"recentCount": 20,
	}

	var result struct {
		Events struct {
			Data []struct {
				Recent []ArchivedEvent `json:"recent"`
			} `json:"data"`
		} `json:"events"`
	}

	if err := c.ExecuteGraphQL(ctx, "GetEvent", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: get event: %w", err)
	}

	for _, evtType := range result.Events.Data {
		for i := range evtType.Recent {
			if evtType.Recent[i].ID == eventID {
				return &evtType.Recent[i], nil
			}
		}
	}

	return nil, fmt.Errorf("inngest: event %s not found", eventID)
}
