package inngest

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ListRunsOptions configures the runs list query.
type ListRunsOptions struct {
	First       int
	After       string   // unused with events-based approach, kept for interface compat
	Status      []string // COMPLETED, FAILED, RUNNING, CANCELLED, QUEUED
	FunctionIDs []string
	AppIDs      []string
	From        time.Time
	Until       *time.Time
}

// listRunsQuery fetches runs via events.recent.functionRuns since the
// Inngest Cloud API has no root `runs` query.
const listRunsQuery = `query ListRuns {
  events(query: {}) {
    data {
      name
      recent(count: 50) {
        id
        occurredAt
        receivedAt
        name
        functionRuns {
          id
          status
          startedAt
          endedAt
          output
          function {
            id
            name
            slug
          }
        }
      }
    }
    page {
      page
      totalPages
    }
  }
}`

// ListRuns queries runs via the events GraphQL query.
// Since there is no root `runs` query on the Inngest Cloud API, runs are
// fetched through events(query:{}).data.recent.functionRuns and then
// flattened, deduplicated, and filtered client-side.
func (c *Client) ListRuns(ctx context.Context, opts ListRunsOptions) (*RunsConnection, error) {
	var result struct {
		Events struct {
			Data []struct {
				Recent []struct {
					ID           string     `json:"id"`
					OccurredAt   *time.Time `json:"occurredAt"`
					ReceivedAt   *time.Time `json:"receivedAt"`
					Name         string     `json:"name"`
					FunctionRuns []struct {
						ID        string     `json:"id"`
						Status    string     `json:"status"`
						StartedAt *time.Time `json:"startedAt"`
						EndedAt   *time.Time `json:"endedAt"`
						Output    string     `json:"output"`
						Function  *struct {
							ID   string `json:"id"`
							Name string `json:"name"`
							Slug string `json:"slug"`
						} `json:"function"`
					} `json:"functionRuns"`
				} `json:"recent"`
			} `json:"data"`
			Page PageResults `json:"page"`
		} `json:"events"`
	}

	if err := c.ExecuteGraphQL(ctx, "ListRuns", listRunsQuery, nil, &result); err != nil {
		return nil, fmt.Errorf("inngest: list runs: %w", err)
	}

	// Flatten all functionRuns from all event instances across all event types.
	seen := map[string]bool{}
	var runs []FunctionRun

	for _, eventType := range result.Events.Data {
		for _, event := range eventType.Recent {
			for _, fr := range event.FunctionRuns {
				if seen[fr.ID] {
					continue
				}
				seen[fr.ID] = true

				run := FunctionRun{
					ID:        fr.ID,
					Status:    fr.Status,
					StartedAt: fr.StartedAt,
					EndedAt:   fr.EndedAt,
					Output:    fr.Output,
					EventName: event.Name,
				}
				if fr.Function != nil {
					run.Function = &Function{
						ID:   fr.Function.ID,
						Name: fr.Function.Name,
						Slug: fr.Function.Slug,
					}
				}

				// Apply filters client-side.
				if !matchesRunFilters(run, opts) {
					continue
				}

				runs = append(runs, run)
			}
		}
	}

	// Apply limit.
	limit := opts.First
	if limit <= 0 {
		limit = 20
	}
	if len(runs) > limit {
		runs = runs[:limit]
	}

	conn := &RunsConnection{
		TotalCount: len(runs),
		PageInfo: PageInfo{
			HasNextPage: false,
		},
	}
	conn.Edges = make([]RunEdge, len(runs))
	for i, run := range runs {
		conn.Edges[i] = RunEdge{Node: run}
	}

	return conn, nil
}

// matchesRunFilters checks if a run matches the filter options.
func matchesRunFilters(run FunctionRun, opts ListRunsOptions) bool {
	if !matchesStatusFilter(run.Status, opts.Status) {
		return false
	}
	if !matchesTimeFilter(run.StartedAt, opts.From, opts.Until) {
		return false
	}
	if !matchesFunctionFilter(run.Function, opts.FunctionIDs) {
		return false
	}
	return true
}

func matchesStatusFilter(status string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, s := range allowed {
		if strings.EqualFold(status, s) {
			return true
		}
	}
	return false
}

func matchesTimeFilter(startedAt *time.Time, from time.Time, until *time.Time) bool {
	if !from.IsZero() && startedAt != nil && startedAt.Before(from) {
		return false
	}
	if until != nil && startedAt != nil && startedAt.After(*until) {
		return false
	}
	return true
}

func matchesFunctionFilter(fn *Function, functionIDs []string) bool {
	if len(functionIDs) == 0 || fn == nil {
		return true
	}
	for _, fid := range functionIDs {
		if fn.ID == fid || fn.Slug == fid {
			return true
		}
	}
	return false
}

// GetRun gets a single run by ID. Since there is no root `run` query on the
// Inngest Cloud API, this searches through recent events' function runs.
func (c *Client) GetRun(ctx context.Context, runID string) (*FunctionRun, error) {
	conn, err := c.ListRuns(ctx, ListRunsOptions{First: 500})
	if err != nil {
		return nil, fmt.Errorf("inngest: get run: %w", err)
	}

	for _, edge := range conn.Edges {
		if edge.Node.ID == runID {
			run := edge.Node
			return &run, nil
		}
	}

	return nil, fmt.Errorf("inngest: run %s not found (only recent runs are searchable)", runID)
}

// CancelRun cancels a running function via GraphQL mutation.
// The Inngest Cloud API requires envID for cancelRun. Pass envID obtained from
// the --env-id flag or the INNGEST_ENV_ID environment variable.
func (c *Client) CancelRun(ctx context.Context, envID, runID string) (*FunctionRun, error) {
	if envID == "" {
		return nil, fmt.Errorf("inngest: cancel run requires environment ID; set --env-id flag or INNGEST_ENV_ID env var")
	}

	query := `mutation CancelRun($envID: UUID!, $runID: ULID!) {
  cancelRun(envID: $envID, runID: $runID) {
    id
    status
  }
}`

	variables := map[string]any{
		"envID": envID,
		"runID": runID,
	}

	var result struct {
		CancelRun *struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"cancelRun"`
	}

	if err := c.ExecuteGraphQL(ctx, "CancelRun", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: cancel run: %w", err)
	}

	if result.CancelRun == nil {
		return nil, fmt.Errorf("inngest: cancel run %s returned no result", runID)
	}

	return &FunctionRun{
		ID:     result.CancelRun.ID,
		Status: result.CancelRun.Status,
	}, nil
}

// RerunRun replays a run via GraphQL mutation.
func (c *Client) RerunRun(ctx context.Context, runID string) (string, error) {
	query := `mutation Rerun($runID: ULID!) {
  rerun(runID: $runID)
}`

	variables := map[string]any{
		"runID": runID,
	}

	var result struct {
		Rerun string `json:"rerun"`
	}

	if err := c.ExecuteGraphQL(ctx, "Rerun", query, variables, &result); err != nil {
		return "", fmt.Errorf("inngest: rerun: %w", err)
	}

	return result.Rerun, nil
}

// StatusToUpper normalises status strings to uppercase for the API.
func StatusToUpper(statuses []string) []string {
	out := make([]string, len(statuses))
	for i, s := range statuses {
		out[i] = strings.ToUpper(s)
	}
	return out
}
