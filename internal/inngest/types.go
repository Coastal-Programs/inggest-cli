package inngest

import "time"

// Function represents an Inngest function.
type Function struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Slug          string                 `json:"slug"`
	AppID         string                 `json:"appID"`
	URL           string                 `json:"url"`
	Config        string                 `json:"config"`
	Concurrency   int                    `json:"concurrency"`
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

// App represents an Inngest app.
type App struct {
	ID             string     `json:"id"`
	ExternalID     string     `json:"externalID"`
	Name           string     `json:"name"`
	SDKLanguage    string     `json:"sdkLanguage"`
	SDKVersion     string     `json:"sdkVersion"`
	Framework      string     `json:"framework,omitempty"`
	URL            string     `json:"url,omitempty"`
	Checksum       string     `json:"checksum,omitempty"`
	Error          string     `json:"error,omitempty"`
	Connected      bool       `json:"connected"`
	FunctionCount  int        `json:"functionCount"`
	Autodiscovered bool       `json:"autodiscovered,omitempty"`
	Method         string     `json:"method,omitempty"`
	Functions      []Function `json:"functions,omitempty"`
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

// EventsConnection is a paginated list of events.
type EventsConnection struct {
	Edges      []EventEdge `json:"edges"`
	PageInfo   PageInfo    `json:"pageInfo"`
	TotalCount int         `json:"totalCount"`
}

// EventEdge wraps an Event with a cursor.
type EventEdge struct {
	Node   Event  `json:"node"`
	Cursor string `json:"cursor"`
}
