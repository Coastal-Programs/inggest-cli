package inngest

import "time"

// Function represents an Inngest function (called "Workflow" in the API).
type Function struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Slug          string                 `json:"slug"`
	URL           string                 `json:"url,omitempty"`
	IsPaused      bool                   `json:"isPaused"`
	IsArchived    bool                   `json:"isArchived"`
	Triggers      []FunctionTrigger      `json:"triggers"`
	Configuration *FunctionConfiguration `json:"configuration,omitempty"`
	App           *App                   `json:"app,omitempty"`
}

// FunctionTrigger defines what triggers a function.
type FunctionTrigger struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Condition string `json:"condition,omitempty"`
}

// FunctionConfiguration holds the parsed configuration for a function.
type FunctionConfiguration struct {
	Retries     *RetryConfig        `json:"retries,omitempty"`
	Concurrency []ConcurrencyConfig `json:"concurrency,omitempty"`
	RateLimit   *RateLimitConfig    `json:"rateLimit,omitempty"`
	Debounce    *DebounceConfig     `json:"debounce,omitempty"`
	Throttle    *ThrottleConfig     `json:"throttle,omitempty"`
	EventsBatch *EventsBatchConfig  `json:"eventsBatch,omitempty"`
	Priority    string              `json:"priority,omitempty"`
}

// RetryConfig defines retry behaviour for a function.
type RetryConfig struct {
	Value     int  `json:"value"`
	IsDefault bool `json:"isDefault"`
}

// ConcurrencyLimit wraps a concurrency limit value from the GraphQL API.
type ConcurrencyLimit struct {
	Value int `json:"value"`
}

// ConcurrencyConfig defines concurrency limits for a function.
type ConcurrencyConfig struct {
	Scope string            `json:"scope"`
	Limit *ConcurrencyLimit `json:"limit,omitempty"`
	Key   string            `json:"key,omitempty"`
}

// RateLimitConfig defines rate limiting for a function.
type RateLimitConfig struct {
	Limit  int    `json:"limit"`
	Period string `json:"period"`
	Key    string `json:"key,omitempty"`
}

// DebounceConfig defines debounce settings for a function.
type DebounceConfig struct {
	Period string `json:"period"`
	Key    string `json:"key,omitempty"`
}

// ThrottleConfig defines throttle settings for a function.
type ThrottleConfig struct {
	Burst  int    `json:"burst"`
	Limit  int    `json:"limit"`
	Period string `json:"period"`
	Key    string `json:"key,omitempty"`
}

// EventsBatchConfig defines event batching for a function.
type EventsBatchConfig struct {
	MaxSize int    `json:"maxSize"`
	Timeout string `json:"timeout"`
	Key     string `json:"key,omitempty"`
}

// App represents an Inngest app (nested within workflows/functions).
type App struct {
	ID          string `json:"id"`
	ExternalID  string `json:"externalID"`
	Name        string `json:"name"`
	AppVersion  string `json:"appVersion,omitempty"`
	SDKLanguage string `json:"sdkLanguage,omitempty"`
	SDKVersion  string `json:"sdkVersion,omitempty"`
}

// Environment represents an Inngest environment (workspace) from the envs query.
type Environment struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	Slug                 string     `json:"slug"`
	Type                 string     `json:"type"`
	IsAutoArchiveEnabled bool       `json:"isAutoArchiveEnabled,omitempty"`
	WebhookSigningKey    string     `json:"webhookSigningKey,omitempty"`
	CreatedAt            *time.Time `json:"createdAt,omitempty"`
}

// EnvEdge wraps an Environment with relay-style pagination.
type EnvEdge struct {
	Node Environment `json:"node"`
}

// EnvsConnection is a relay-style paginated list of environments.
type EnvsConnection struct {
	Edges    []EnvEdge `json:"edges"`
	PageInfo PageInfo  `json:"pageInfo"`
}

// Event represents an Inngest event.
type Event struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Raw        string        `json:"raw,omitempty"`
	CreatedAt  *time.Time    `json:"createdAt,omitempty"`
	ReceivedAt *time.Time    `json:"receivedAt,omitempty"`
	Status     string        `json:"status,omitempty"`
	TotalRuns  int           `json:"totalRuns"`
	Runs       []FunctionRun `json:"runs,omitempty"`
}

// FunctionRun represents a function execution.
type FunctionRun struct {
	ID           string        `json:"id"`
	FunctionID   string        `json:"functionID"`
	AppID        string        `json:"appID,omitempty"`
	Status       string        `json:"status"`
	EventName    string        `json:"eventName,omitempty"`
	QueuedAt     *time.Time    `json:"queuedAt,omitempty"`
	StartedAt    *time.Time    `json:"startedAt,omitempty"`
	EndedAt      *time.Time    `json:"endedAt,omitempty"`
	Output       string        `json:"output,omitempty"`
	TraceID      string        `json:"traceID,omitempty"`
	IsBatch      bool          `json:"isBatch"`
	CronSchedule string        `json:"cronSchedule,omitempty"`
	Function     *Function     `json:"function,omitempty"`
	App          *App          `json:"app,omitempty"`
	Trace        *RunTraceSpan `json:"trace,omitempty"`
}

// RunTraceSpan represents a trace span for a run.
type RunTraceSpan struct {
	RunID     string         `json:"runID"`
	SpanID    string         `json:"spanID"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	StartedAt *time.Time     `json:"startedAt,omitempty"`
	EndedAt   *time.Time     `json:"endedAt,omitempty"`
	Duration  int            `json:"durationMS"`
	StepOp    string         `json:"stepOp,omitempty"`
	Children  []RunTraceSpan `json:"childrenSpans,omitempty"`
}

// StreamItem represents an item in the event stream.
type StreamItem struct {
	ID        string        `json:"id"`
	Trigger   string        `json:"trigger"`
	Type      string        `json:"type"`
	CreatedAt time.Time     `json:"createdAt"`
	Runs      []FunctionRun `json:"runs,omitempty"`
	InBatch   bool          `json:"inBatch"`
}

// PageInfo for cursor-based pagination.
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor,omitempty"`
	EndCursor       string `json:"endCursor,omitempty"`
}

// RunsConnection is a paginated list of function runs.
type RunsConnection struct {
	Edges      []RunEdge `json:"edges"`
	PageInfo   PageInfo  `json:"pageInfo"`
	TotalCount int       `json:"totalCount"`
}

// RunEdge wraps a FunctionRun with a cursor.
type RunEdge struct {
	Node   FunctionRun `json:"node"`
	Cursor string      `json:"cursor"`
}

// EventType represents an event type from the Inngest Cloud API.
// The API returns event types (not instances) via the `events` query.
type EventType struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	FirstSeen   *time.Time      `json:"firstSeen,omitempty"`
	Usage       *EventTypeUsage `json:"usage,omitempty"`
	Workflows   []Function      `json:"workflows,omitempty"`
	Recent      []ArchivedEvent `json:"recent,omitempty"`
}

// EventTypeUsage holds usage statistics for an event type.
type EventTypeUsage struct {
	Total int `json:"total"`
}

// ArchivedEvent represents an individual event instance from `recent`.
type ArchivedEvent struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	OccurredAt   *time.Time    `json:"occurredAt,omitempty"`
	ReceivedAt   *time.Time    `json:"receivedAt,omitempty"`
	Event        string        `json:"event,omitempty"` // JSON string of the event payload
	Version      string        `json:"version,omitempty"`
	FunctionRuns []FunctionRun `json:"functionRuns,omitempty"`
}

// EventTypesResult is the paginated result from the `events` query.
type EventTypesResult struct {
	Data []EventType `json:"data"`
	Page PageResults `json:"page"`
}

// PageResults holds page-based pagination info.
type PageResults struct {
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}
