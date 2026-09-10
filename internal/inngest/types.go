package inngest

import (
	"encoding/json"
	"time"
)

// Shapes follow the Inngest REST API v2 (https://api-docs.inngest.com/api-specs/v2.json).
// The dev server's GraphQL API is aliased into the same fields so both backends
// decode into one set of types.

// Function represents an Inngest function.
type Function struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Slug          string                 `json:"slug"`
	URL           string                 `json:"url,omitempty"`
	IsPaused      bool                   `json:"isPaused"`
	IsArchived    bool                   `json:"isArchived"`
	Triggers      []FunctionTrigger      `json:"triggers,omitempty"`
	Configuration *FunctionConfiguration `json:"configuration,omitempty"`
	App           *App                   `json:"app,omitempty"`
}

// FunctionTrigger defines what triggers a function: type is EVENT or CRON.
type FunctionTrigger struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Condition string `json:"if,omitempty"`
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

// ConcurrencyLimit wraps a concurrency limit value.
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

// App represents an Inngest app (a synced SDK deployment).
type App struct {
	ID            string     `json:"id"`
	ExternalID    string     `json:"externalID,omitempty"`
	Name          string     `json:"name"`
	AppVersion    string     `json:"appVersion,omitempty"`
	SDKLanguage   string     `json:"sdkLanguage,omitempty"`
	SDKVersion    string     `json:"sdkVersion,omitempty"`
	Framework     string     `json:"framework,omitempty"`
	URL           string     `json:"url,omitempty"`
	Method        string     `json:"method,omitempty"` // SERVE, CONNECT, API
	IsArchived    bool       `json:"isArchived"`
	FunctionCount int        `json:"functionCount,omitempty"`
	CreatedAt     *time.Time `json:"createdAt,omitempty"`
	ArchivedAt    *time.Time `json:"archivedAt,omitempty"`
	LatestSync    *AppSync   `json:"latestSync,omitempty"`
}

// AppSync describes the most recent sync of an app.
type AppSync struct {
	Status      string     `json:"status,omitempty"`
	URL         string     `json:"url,omitempty"`
	Error       string     `json:"error,omitempty"`
	AppVersion  string     `json:"appVersion,omitempty"`
	Framework   string     `json:"framework,omitempty"`
	SDKLanguage string     `json:"sdkLanguage,omitempty"`
	SDKVersion  string     `json:"sdkVersion,omitempty"`
	SyncedAt    *time.Time `json:"syncedAt,omitempty"`
}

// Environment represents an Inngest environment: type is PRODUCTION, TEST or BRANCH.
type Environment struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	IsArchived bool       `json:"isArchived"`
	CreatedAt  *time.Time `json:"createdAt,omitempty"`
}

// Event is an event instance as returned by the REST v1 events API.
type Event struct {
	InternalID string          `json:"internal_id"`
	ID         string          `json:"id,omitempty"`
	Name       string          `json:"name"`
	Data       json.RawMessage `json:"data,omitempty"`
	User       json.RawMessage `json:"user,omitempty"`
	Timestamp  int64           `json:"ts,omitempty"`
	Version    string          `json:"v,omitempty"`
	Source     string          `json:"source,omitempty"`
	ReceivedAt *time.Time      `json:"received_at,omitempty"`
}

// UnmarshalJSON accepts both "received_at" (what the server sends) and
// "receivedAt" (what the OpenAPI spec documents).
func (e *Event) UnmarshalJSON(b []byte) error {
	type plain Event
	aux := struct {
		*plain
		ReceivedAtCamel *time.Time `json:"receivedAt"`
	}{plain: (*plain)(e)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if e.ReceivedAt == nil {
		e.ReceivedAt = aux.ReceivedAtCamel
	}
	return nil
}

// EventSchema describes the shape of one event type's data.
type EventSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

// FunctionRun represents a function execution. Status is one of
// QUEUED, RUNNING, COMPLETED, FAILED, CANCELLED.
type FunctionRun struct {
	ID           string          `json:"id"`
	FunctionID   string          `json:"functionID"`
	AppID        string          `json:"appID,omitempty"`
	Status       string          `json:"status"`
	EventName    string          `json:"eventName,omitempty"`
	EventIDs     []string        `json:"eventIDs,omitempty"`
	QueuedAt     *time.Time      `json:"queuedAt,omitempty"`
	StartedAt    *time.Time      `json:"startedAt,omitempty"`
	EndedAt      *time.Time      `json:"endedAt,omitempty"`
	DurationMs   Millis          `json:"durationMs,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
	IsBatch      bool            `json:"isBatch"`
	BatchID      string          `json:"batchID,omitempty"`
	CronSchedule string          `json:"cronSchedule,omitempty"`
	Function     *Function       `json:"function,omitempty"`
	App          *App            `json:"app,omitempty"`
	Trace        *RunTraceSpan   `json:"trace,omitempty"`
}

// RunTraceSpan is one node of a run's trace tree. StepOp is one of
// RUN, SLEEP, WAIT_FOR_EVENT, INVOKE, SEND_EVENT, AI_GATEWAY, WAIT_FOR_SIGNAL.
type RunTraceSpan struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	StepID     string          `json:"stepId,omitempty"`
	StepOp     string          `json:"stepOp,omitempty"`
	Attempts   int             `json:"attempts,omitempty"`
	QueuedAt   *time.Time      `json:"queuedAt,omitempty"`
	StartedAt  *time.Time      `json:"startedAt,omitempty"`
	EndedAt    *time.Time      `json:"endedAt,omitempty"`
	DurationMs Millis          `json:"durationMs,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	Children   []RunTraceSpan  `json:"children,omitempty"`
}

// RunsPage is one page of function runs plus the cursor for the next page.
type RunsPage struct {
	Runs []FunctionRun `json:"runs"`
	Page Page          `json:"page"`
}
