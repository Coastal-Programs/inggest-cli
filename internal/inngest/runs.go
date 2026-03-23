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
	After       string   // cursor for pagination
	Status      []string // COMPLETED, FAILED, RUNNING, CANCELLED, QUEUED
	FunctionIDs []string
	AppIDs      []string
	From        time.Time
	Until       *time.Time
	Query       string // CEL query string
}

// ListRuns queries runs via GraphQL.
func (c *Client) ListRuns(ctx context.Context, opts ListRunsOptions) (*RunsConnection, error) {
	query := `query ListRuns($first: Int!, $after: String, $orderBy: [RunsV2OrderBy!]!, $filter: RunsFilterV2!) {
  runs(first: $first, after: $after, orderBy: $orderBy, filter: $filter) {
    edges {
      node {
        id
        status
        queuedAt
        startedAt
        endedAt
        eventName
        isBatch
        cronSchedule
        function {
          name
          slug
        }
        app {
          name
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

	first := opts.First
	if first <= 0 {
		first = 20
	}

	filter := map[string]interface{}{}
	if !opts.From.IsZero() {
		filter["from"] = opts.From.Format(time.RFC3339)
	}
	if opts.Until != nil {
		filter["until"] = opts.Until.Format(time.RFC3339)
	}
	if len(opts.Status) > 0 {
		filter["status"] = opts.Status
	}
	if len(opts.FunctionIDs) > 0 {
		filter["functionIDs"] = opts.FunctionIDs
	}
	if len(opts.AppIDs) > 0 {
		filter["appIDs"] = opts.AppIDs
	}
	if opts.Query != "" {
		filter["query"] = opts.Query
	}

	variables := map[string]interface{}{
		"first":   first,
		"orderBy": []map[string]string{{"field": "QUEUED_AT", "direction": "DESC"}},
		"filter":  filter,
	}
	if opts.After != "" {
		variables["after"] = opts.After
	}

	var result struct {
		Runs struct {
			Edges []struct {
				Node   runNode `json:"node"`
				Cursor string  `json:"cursor"`
			} `json:"edges"`
			PageInfo   PageInfo `json:"pageInfo"`
			TotalCount int      `json:"totalCount"`
		} `json:"runs"`
	}

	if err := c.ExecuteGraphQL(ctx, "ListRuns", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: list runs: %w", err)
	}

	conn := &RunsConnection{
		PageInfo:   result.Runs.PageInfo,
		TotalCount: result.Runs.TotalCount,
	}

	conn.Edges = make([]RunEdge, len(result.Runs.Edges))
	for i, edge := range result.Runs.Edges {
		conn.Edges[i] = RunEdge{
			Node:   nodeToFunctionRun(edge.Node),
			Cursor: edge.Cursor,
		}
	}

	return conn, nil
}

// GetRun gets a single run by ID via GraphQL.
func (c *Client) GetRun(ctx context.Context, runID string) (*FunctionRun, error) {
	query := `query GetRun($runID: String!) {
  run(runID: $runID) {
    id
    status
    queuedAt
    startedAt
    endedAt
    eventName
    isBatch
    cronSchedule
    output
    traceID
    function {
      name
      slug
      config
    }
    app {
      name
      sdkLanguage
      sdkVersion
    }
    trace {
      runID
      spanID
      name
      status
      startedAt
      endedAt
      durationMS
      stepOp
      childrenSpans {
        spanID
        name
        status
        startedAt
        endedAt
        durationMS
        stepOp
      }
    }
  }
}`

	variables := map[string]interface{}{
		"runID": runID,
	}

	var result struct {
		Run *runDetailNode `json:"run"`
	}

	if err := c.ExecuteGraphQL(ctx, "GetRun", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: get run: %w", err)
	}

	if result.Run == nil {
		return nil, fmt.Errorf("inngest: run %s not found", runID)
	}

	r := result.Run
	run := FunctionRun{
		ID:           r.ID,
		Status:       r.Status,
		QueuedAt:     r.QueuedAt,
		StartedAt:    r.StartedAt,
		EndedAt:      r.EndedAt,
		EventName:    r.EventName,
		IsBatch:      r.IsBatch,
		CronSchedule: r.CronSchedule,
		Output:       r.Output,
		TraceID:      r.TraceID,
	}

	if r.Function != nil {
		run.Function = &Function{
			Name:   r.Function.Name,
			Slug:   r.Function.Slug,
			Config: r.Function.Config,
		}
	}
	if r.App != nil {
		run.App = &App{
			Name:        r.App.Name,
			SDKLanguage: r.App.SDKLanguage,
			SDKVersion:  r.App.SDKVersion,
		}
	}

	if r.Trace != nil {
		run.Trace = convertTrace(r.Trace)
	}

	return &run, nil
}

// CancelRun cancels a running function via GraphQL mutation.
func (c *Client) CancelRun(ctx context.Context, runID string) (*FunctionRun, error) {
	query := `mutation CancelRun($runID: ULID!) {
  cancelRun(runID: $runID) {
    id
    status
  }
}`

	variables := map[string]interface{}{
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

	variables := map[string]interface{}{
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

// runNode is the GraphQL shape for run list queries.
type runNode struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	QueuedAt     *time.Time `json:"queuedAt,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	EndedAt      *time.Time `json:"endedAt,omitempty"`
	EventName    string     `json:"eventName,omitempty"`
	IsBatch      bool       `json:"isBatch"`
	CronSchedule string     `json:"cronSchedule,omitempty"`
	Function     *struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"function,omitempty"`
	App *struct {
		Name string `json:"name"`
	} `json:"app,omitempty"`
}

// runDetailFunction is the GraphQL shape for function in detail queries.
type runDetailFunction struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Config string `json:"config,omitempty"`
}

// runDetailApp is the GraphQL shape for app in detail queries.
type runDetailApp struct {
	Name        string `json:"name"`
	SDKLanguage string `json:"sdkLanguage,omitempty"`
	SDKVersion  string `json:"sdkVersion,omitempty"`
}

// runDetailNode has explicit fields (no embedding) to avoid shadowing issues.
type runDetailNode struct {
	ID           string             `json:"id"`
	Status       string             `json:"status"`
	QueuedAt     *time.Time         `json:"queuedAt,omitempty"`
	StartedAt    *time.Time         `json:"startedAt,omitempty"`
	EndedAt      *time.Time         `json:"endedAt,omitempty"`
	EventName    string             `json:"eventName,omitempty"`
	IsBatch      bool               `json:"isBatch"`
	CronSchedule string             `json:"cronSchedule,omitempty"`
	Output       string             `json:"output,omitempty"`
	TraceID      string             `json:"traceID,omitempty"`
	Function     *runDetailFunction `json:"function,omitempty"`
	App          *runDetailApp      `json:"app,omitempty"`
	Trace        *traceNode         `json:"trace,omitempty"`
}

// traceNode is the GraphQL shape for trace spans.
type traceNode struct {
	RunID     string      `json:"runID,omitempty"`
	SpanID    string      `json:"spanID"`
	Name      string      `json:"name"`
	Status    string      `json:"status"`
	StartedAt *time.Time  `json:"startedAt,omitempty"`
	EndedAt   *time.Time  `json:"endedAt,omitempty"`
	Duration  int         `json:"durationMS"`
	StepOp    string      `json:"stepOp,omitempty"`
	Children  []traceNode `json:"childrenSpans,omitempty"`
}

func nodeToFunctionRun(n runNode) FunctionRun {
	run := FunctionRun{
		ID:           n.ID,
		Status:       n.Status,
		QueuedAt:     n.QueuedAt,
		StartedAt:    n.StartedAt,
		EndedAt:      n.EndedAt,
		EventName:    n.EventName,
		IsBatch:      n.IsBatch,
		CronSchedule: n.CronSchedule,
	}
	if n.Function != nil {
		run.Function = &Function{
			Name: n.Function.Name,
			Slug: n.Function.Slug,
		}
	}
	if n.App != nil {
		run.App = &App{Name: n.App.Name}
	}
	return run
}

func convertTrace(n *traceNode) *RunTraceSpan {
	if n == nil {
		return nil
	}
	span := &RunTraceSpan{
		RunID:     n.RunID,
		SpanID:    n.SpanID,
		Name:      n.Name,
		Status:    n.Status,
		StartedAt: n.StartedAt,
		EndedAt:   n.EndedAt,
		Duration:  n.Duration,
		StepOp:    n.StepOp,
	}
	for _, child := range n.Children {
		c := child
		span.Children = append(span.Children, *convertTrace(&c))
	}
	return span
}

// StatusToUpper normalises status strings to uppercase for the API.
func StatusToUpper(statuses []string) []string {
	out := make([]string, len(statuses))
	for i, s := range statuses {
		out[i] = strings.ToUpper(s)
	}
	return out
}
