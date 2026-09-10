package inngest

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Dev server GraphQL (POST {devServerURL}/v0/gql). The local dev server does not
// implement the REST v2 runs/functions endpoints, so these queries alias the
// OSS GraphQL schema into the same Go types the v2 API decodes into.

const devRunFields = `
        id
        functionID
        appID
        status
        eventName
        eventIDs: triggerIDs
        queuedAt
        startedAt
        endedAt
        isBatch
        cronSchedule
        function { id name slug }
        app { id name externalID }
`

const devTraceFields = `
          id: spanID
          name
          status
          stepId: stepID
          stepOp
          attempts
          queuedAt
          startedAt
          endedAt
          durationMs: duration
`

// devTraceQuery nests children four levels deep; GraphQL cannot recurse.
var devTraceQuery = "trace {" + devTraceFields +
	"children: childrenSpans {" + devTraceFields +
	"children: childrenSpans {" + devTraceFields +
	"children: childrenSpans {" + devTraceFields +
	"} } } }"

// devFunctionFields mirrors the REST v2 function shape.
const devFunctionFields = `
      id name slug url
      triggers { type value if: condition }
      configuration {
        retries { value isDefault }
        concurrency { scope limit { value } key }
        rateLimit { limit period key }
        debounce { period key }
        throttle { burst limit period key }
        eventsBatch { maxSize timeout key }
        priority
      }
      app { id name externalID appVersion sdkLanguage sdkVersion framework url }
`

func (c *Client) devListRuns(ctx context.Context, opts ListRunsOptions) (*RunsPage, error) {
	query := `query DevRuns($first: Int!, $after: String, $orderBy: [RunsV2OrderBy!]!, $filter: RunsFilterV2!) {
  runs(first: $first, after: $after, orderBy: $orderBy, filter: $filter) {
    edges { node {` + devRunFields + `} }
    pageInfo { hasNextPage endCursor }
  }
}`

	first := opts.First
	if first <= 0 {
		first = 20
	}
	variables := devRunsVariables(opts, first)

	var result struct {
		Runs struct {
			Edges []struct {
				Node FunctionRun `json:"node"`
			} `json:"edges"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"runs"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevRuns", query, variables, &result); err != nil {
		return nil, fmt.Errorf("inngest: list runs: %w", err)
	}

	page := &RunsPage{
		Runs: make([]FunctionRun, 0, len(result.Runs.Edges)),
		Page: Page{HasMore: result.Runs.PageInfo.HasNextPage, Cursor: result.Runs.PageInfo.EndCursor, Limit: first},
	}
	for _, edge := range result.Runs.Edges {
		// simplification: dev server v1.17 ignores filter.status, so filter here too;
		// a page may come back short. Drop once the dev server honours the filter.
		if len(opts.Status) > 0 && !containsFold(opts.Status, edge.Node.Status) {
			continue
		}
		page.Runs = append(page.Runs, edge.Node)
	}
	return page, nil
}

// devRunsVariables maps ListRunsOptions onto the dev server's RunsFilterV2 /
// RunsV2OrderBy inputs.
func devRunsVariables(opts ListRunsOptions, first int) map[string]any {
	from := opts.From
	if from.IsZero() {
		from = time.Unix(0, 0)
	}
	filter := map[string]any{"from": from.UTC().Format(time.RFC3339Nano)}
	if opts.Until != nil {
		filter["until"] = opts.Until.UTC().Format(time.RFC3339Nano)
	}
	if len(opts.Status) > 0 {
		statuses := make([]string, len(opts.Status))
		for i, s := range opts.Status {
			statuses[i] = strings.ToUpper(s)
		}
		filter["status"] = statuses
	}
	if len(opts.FunctionIDs) > 0 {
		filter["functionIDs"] = opts.FunctionIDs
	}
	if len(opts.AppIDs) > 0 {
		filter["appIDs"] = opts.AppIDs
	}

	field := "QUEUED_AT"
	switch strings.ToLower(opts.TimeField) {
	case "startedat", "started_at":
		field = "STARTED_AT"
	case "endedat", "ended_at":
		field = "ENDED_AT"
	}
	filter["timeField"] = field
	direction := "DESC"
	if strings.EqualFold(opts.Order, "ASC") {
		direction = "ASC"
	}

	variables := map[string]any{
		"first":   first,
		"orderBy": []map[string]string{{"field": field, "direction": direction}},
		"filter":  filter,
	}
	if opts.After != "" {
		variables["after"] = opts.After
	}
	return variables
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

func (c *Client) devGetRun(ctx context.Context, runID string) (*FunctionRun, error) {
	query := `query DevRun($runID: String!) {
  run(runID: $runID) {` + devRunFields + `
        output
  }
}`

	var result struct {
		Run *FunctionRun `json:"run"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevRun", query, map[string]any{"runID": runID}, &result); err != nil {
		return nil, fmt.Errorf("inngest: get run %s: %w", runID, err)
	}
	if result.Run == nil {
		return nil, &APIError{StatusCode: 404, Code: "not_found", Message: "run " + runID + " not found"}
	}
	result.Run.Output = normalizeJSON(result.Run.Output)
	return result.Run, nil
}

// devGetRunTrace fetches the trace tree separately from the run: the dev
// server rejects the trace field until the run's first span exists, which
// must not make the run itself unreadable.
func (c *Client) devGetRunTrace(ctx context.Context, runID string) (*RunTraceSpan, error) {
	query := `query DevRunTrace($runID: String!) {
  run(runID: $runID) { id ` + devTraceQuery + ` }
}`
	var result struct {
		Run *struct {
			Trace *RunTraceSpan `json:"trace"`
		} `json:"run"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevRunTrace", query, map[string]any{"runID": runID}, &result); err != nil {
		// simplification: the dev server reports a not-yet-started run as a
		// GraphQL error rather than a null trace; treat that as "no trace yet".
		if strings.Contains(err.Error(), "no function run span found") {
			return nil, &APIError{StatusCode: 404, Code: "not_found", Message: "run " + runID + " has no trace yet"}
		}
		return nil, fmt.Errorf("inngest: get run trace %s: %w", runID, err)
	}
	if result.Run == nil {
		return nil, &APIError{StatusCode: 404, Code: "not_found", Message: "run " + runID + " not found"}
	}
	return result.Run.Trace, nil
}

func (c *Client) devCancelRun(ctx context.Context, runID string) (string, error) {
	mutation := `mutation DevCancelRun($runID: ULID!) {
  cancelRun(runID: $runID) { id }
}`
	var result struct {
		CancelRun struct {
			ID string `json:"id"`
		} `json:"cancelRun"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevCancelRun", mutation, map[string]any{"runID": runID}, &result); err != nil {
		return "", fmt.Errorf("inngest: cancel run %s: %w", runID, err)
	}
	if result.CancelRun.ID == "" {
		return runID, nil
	}
	return result.CancelRun.ID, nil
}

func (c *Client) devRerunRun(ctx context.Context, runID string) (string, error) {
	mutation := `mutation DevRerun($runID: ULID!) {
  rerun(runID: $runID)
}`
	var result struct {
		Rerun string `json:"rerun"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevRerun", mutation, map[string]any{"runID": runID}, &result); err != nil {
		return "", fmt.Errorf("inngest: rerun %s: %w", runID, err)
	}
	return result.Rerun, nil
}

func (c *Client) devListFunctions(ctx context.Context) ([]Function, error) {
	query := `query DevFunctions {
  functions {` + devFunctionFields + `}
}`
	var result struct {
		Functions []Function `json:"functions"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevFunctions", query, nil, &result); err != nil {
		return nil, fmt.Errorf("inngest: list functions: %w", err)
	}
	if result.Functions == nil {
		result.Functions = []Function{}
	}
	return result.Functions, nil
}

func (c *Client) devListApps(ctx context.Context) ([]App, error) {
	query := `query DevApps {
  apps { id name externalID appVersion sdkLanguage sdkVersion framework url method functionCount }
}`
	var result struct {
		Apps []App `json:"apps"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevApps", query, nil, &result); err != nil {
		return nil, fmt.Errorf("inngest: list apps: %w", err)
	}
	if result.Apps == nil {
		result.Apps = []App{}
	}
	return result.Apps, nil
}

// devInvokeSettleTimeout bounds how long InvokeDevFunction waits for the run
// created by an invocation to become visible.
const devInvokeSettleTimeout = 5 * time.Second

// InvokeDevFunction invokes a function on the dev server via its invokeFunction
// mutation (the same path the dev UI uses) and returns the resulting run ID.
// The mutation itself only acknowledges, so the run is located by polling the
// newest run of that function queued since the invocation; an empty ID with a
// nil error means the run was not visible within devInvokeSettleTimeout.
func (c *Client) InvokeDevFunction(ctx context.Context, slug string, data any) (string, error) {
	fn, err := c.devFunctionBySlug(ctx, slug)
	if err != nil {
		return "", err
	}

	since := time.Now().Add(-time.Second)
	mutation := `mutation DevInvoke($slug: String!, $data: Map) {
  invokeFunction(functionSlug: $slug, data: $data)
}`
	var result struct {
		Invoked *bool `json:"invokeFunction"`
	}
	vars := map[string]any{"slug": slug, "data": data}
	if err := c.ExecuteGraphQL(ctx, "DevInvoke", mutation, vars, &result); err != nil {
		return "", fmt.Errorf("inngest: invoke function %s: %w", slug, err)
	}
	if result.Invoked == nil || !*result.Invoked {
		return "", fmt.Errorf("inngest: invoke function %s: dev server did not accept the invocation", slug)
	}

	deadline := time.Now().Add(devInvokeSettleTimeout)
	for {
		page, err := c.devListRuns(ctx, ListRunsOptions{First: 1, From: since, FunctionIDs: []string{fn.ID}})
		if err != nil {
			return "", err
		}
		if len(page.Runs) > 0 {
			return page.Runs[0].ID, nil
		}
		if time.Now().After(deadline) {
			return "", nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// devFunctionBySlug resolves a function slug or ID to its dev-server record.
func (c *Client) devFunctionBySlug(ctx context.Context, slugOrID string) (*Function, error) {
	functions, err := c.devListFunctions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range functions {
		if functions[i].Slug == slugOrID || functions[i].ID == slugOrID {
			return &functions[i], nil
		}
	}
	return nil, fmt.Errorf("inngest: function %q not found on dev server", slugOrID)
}

// devEventRuns lists the runs triggered by an event (eventV2.runs).
func (c *Client) devEventRuns(ctx context.Context, eventID string) ([]FunctionRun, error) {
	query := `query DevEventRuns($id: ULID!) {
  eventV2(id: $id) { runs {` + devRunFields + `} }
}`
	var result struct {
		Event *struct {
			Runs []FunctionRun `json:"runs"`
		} `json:"eventV2"`
	}
	if err := c.ExecuteGraphQL(ctx, "DevEventRuns", query, map[string]any{"id": eventID}, &result); err != nil {
		return nil, fmt.Errorf("inngest: get event runs: %w", err)
	}
	if result.Event == nil {
		return nil, &APIError{StatusCode: 404, Code: "not_found", Message: "event " + eventID + " not found"}
	}
	if result.Event.Runs == nil {
		return []FunctionRun{}, nil
	}
	return result.Event.Runs, nil
}
