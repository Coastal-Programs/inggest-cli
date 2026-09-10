package inngest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MaxRunsPageSize is the largest page the runs API accepts.
const MaxRunsPageSize = 100

// ListRunsOptions configures the runs list query (GET /v2/runs).
type ListRunsOptions struct {
	First         int        // page size, 1–100 (default 20)
	After         string     // pagination cursor from a previous page
	Status        []string   // QUEUED, RUNNING, COMPLETED, FAILED, CANCELLED
	FunctionIDs   []string   // function UUIDs (cloud) or slugs (dev server)
	AppIDs        []string   // app UUIDs
	From          time.Time  // inclusive start of the time range (zero = unbounded)
	Until         *time.Time // inclusive end of the time range
	TimeField     string     // queuedAt (default), startedAt or endedAt
	Order         string     // DESC (default) or ASC
	IncludeOutput bool
}

func (o ListRunsOptions) query() url.Values {
	q := url.Values{}
	if o.First > 0 {
		q.Set("limit", strconv.Itoa(o.First))
	}
	if o.After != "" {
		q.Set("cursor", o.After)
	}
	for _, s := range o.Status {
		q.Add("status", strings.ToUpper(s))
	}
	for _, id := range o.FunctionIDs {
		q.Add("functionId", id)
	}
	for _, id := range o.AppIDs {
		q.Add("appId", id)
	}
	if !o.From.IsZero() {
		q.Set("from", o.From.UTC().Format(time.RFC3339Nano))
	}
	if o.Until != nil {
		q.Set("until", o.Until.UTC().Format(time.RFC3339Nano))
	}
	if o.TimeField != "" {
		q.Set("timeField", o.TimeField)
	}
	if o.Order != "" {
		q.Set("order", strings.ToUpper(o.Order))
	}
	if o.IncludeOutput {
		q.Set("includeOutput", "true")
	}
	return q
}

// v2Run is the wire shape of a run in the v2 API; it is flattened into FunctionRun.
type v2Run struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	App      *App   `json:"app"`
	Function *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
		App  *App   `json:"app"`
	} `json:"function"`
	Trigger *struct {
		EventName    string   `json:"eventName"`
		EventIDs     []string `json:"eventIds"`
		CronSchedule string   `json:"cronSchedule"`
		IsBatch      bool     `json:"isBatch"`
		BatchID      string   `json:"batchId"`
	} `json:"trigger"`
	QueuedAt   *time.Time      `json:"queuedAt"`
	StartedAt  *time.Time      `json:"startedAt"`
	EndedAt    *time.Time      `json:"endedAt"`
	DurationMs Millis          `json:"durationMs"`
	Output     json.RawMessage `json:"output"`
}

func (r v2Run) toFunctionRun() FunctionRun {
	run := FunctionRun{
		ID:         r.ID,
		Status:     r.Status,
		QueuedAt:   r.QueuedAt,
		StartedAt:  r.StartedAt,
		EndedAt:    r.EndedAt,
		DurationMs: r.DurationMs,
		Output:     normalizeJSON(r.Output),
	}
	if r.App != nil {
		run.AppID = r.App.ID
		run.App = r.App
	}
	if r.Function != nil {
		run.FunctionID = r.Function.ID
		run.Function = &Function{ID: r.Function.ID, Name: r.Function.Name, Slug: r.Function.Slug}
		if run.AppID == "" && r.Function.App != nil {
			run.AppID = r.Function.App.ID
		}
	}
	if r.Trigger != nil {
		run.EventName = r.Trigger.EventName
		run.EventIDs = r.Trigger.EventIDs
		run.CronSchedule = r.Trigger.CronSchedule
		run.IsBatch = r.Trigger.IsBatch
		run.BatchID = r.Trigger.BatchID
	}
	return run
}

// normalizeJSON drops JSON null and unwraps a JSON string that itself holds JSON
// (the dev server returns step output as an encoded string).
func normalizeJSON(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && json.Valid([]byte(s)) {
			return json.RawMessage(s)
		}
	}
	return raw
}

// ListRuns lists function runs newest-first (or per opts.Order).
func (c *Client) ListRuns(ctx context.Context, opts ListRunsOptions) (*RunsPage, error) {
	if c.devMode {
		return c.devListRuns(ctx, opts)
	}
	if opts.First > MaxRunsPageSize {
		opts.First = MaxRunsPageSize
	}

	var data []v2Run
	page, err := c.v2Get(ctx, "runs", opts.query(), &data)
	if err != nil {
		return nil, fmt.Errorf("inngest: list runs: %w", err)
	}

	result := &RunsPage{Runs: make([]FunctionRun, len(data))}
	for i, r := range data {
		result.Runs[i] = r.toFunctionRun()
	}
	if page != nil {
		result.Page = *page
	}
	return result, nil
}

// GetRun fetches a single run by ID, including its output.
func (c *Client) GetRun(ctx context.Context, runID string) (*FunctionRun, error) {
	if c.devMode {
		return c.devGetRun(ctx, runID)
	}

	var data v2Run
	q := url.Values{"includeOutput": {"true"}}
	if _, err := c.v2Get(ctx, "runs/"+url.PathEscape(runID), q, &data); err != nil {
		return nil, fmt.Errorf("inngest: get run %s: %w", runID, err)
	}
	run := data.toFunctionRun()
	return &run, nil
}

// GetRunTrace fetches the step-by-step trace tree for a run.
func (c *Client) GetRunTrace(ctx context.Context, runID string) (*RunTraceSpan, error) {
	if c.devMode {
		return c.devGetRunTrace(ctx, runID)
	}

	var data struct {
		RunID    string        `json:"runId"`
		RootSpan *RunTraceSpan `json:"rootSpan"`
	}
	if _, err := c.v2Get(ctx, "runs/"+url.PathEscape(runID)+"/trace", nil, &data); err != nil {
		return nil, fmt.Errorf("inngest: get run trace %s: %w", runID, err)
	}
	return data.RootSpan, nil
}

// CancelRun cancels an in-progress run. Returns the cancelled run ID.
func (c *Client) CancelRun(ctx context.Context, runID string) (string, error) {
	if c.devMode {
		return c.devCancelRun(ctx, runID)
	}

	var data struct {
		RunID string `json:"runId"`
	}
	if err := c.v2Post(ctx, "runs/"+url.PathEscape(runID)+"/cancel", map[string]any{}, &data); err != nil {
		return "", fmt.Errorf("inngest: cancel run %s: %w", runID, err)
	}
	if data.RunID == "" {
		data.RunID = runID
	}
	return data.RunID, nil
}

// RerunRun replays a run with its original trigger. Returns the new run ID.
func (c *Client) RerunRun(ctx context.Context, runID string) (string, error) {
	if c.devMode {
		return c.devRerunRun(ctx, runID)
	}

	var data struct {
		RunID string `json:"runId"`
	}
	if err := c.v2Post(ctx, "runs/"+url.PathEscape(runID)+"/rerun", map[string]any{}, &data); err != nil {
		return "", fmt.Errorf("inngest: rerun %s: %w", runID, err)
	}
	return data.RunID, nil
}

// IsTerminalRunStatus reports whether a run status is final.
func IsTerminalRunStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "COMPLETED", "FAILED", "CANCELLED":
		return true
	}
	return false
}
