package inngest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// ListAppFunctions lists the functions of one app (GET /v2/apps/{appId}/functions).
func (c *Client) ListAppFunctions(ctx context.Context, appID string) ([]Function, error) {
	q := url.Values{"limit": {strconv.Itoa(v2ListPageSize)}}
	functions := []Function{}
	for range maxListPages {
		var data []Function
		page, err := c.v2Get(ctx, "apps/"+url.PathEscape(appID)+"/functions", q, &data)
		if err != nil {
			return nil, fmt.Errorf("inngest: list functions for app %s: %w", appID, err)
		}
		functions = append(functions, data...)
		if page == nil || !page.HasMore || page.Cursor == "" {
			return functions, nil
		}
		q.Set("cursor", page.Cursor)
	}
	return functions, nil
}

// ListFunctions lists every function across all active apps in the environment.
// The v2 API scopes functions to apps, so this is one request per app.
func (c *Client) ListFunctions(ctx context.Context) ([]Function, error) {
	if c.devMode {
		return c.devListFunctions(ctx)
	}

	apps, err := c.ListApps(ctx, false)
	if err != nil {
		return nil, err
	}

	functions := []Function{}
	for i := range apps {
		fns, err := c.ListAppFunctions(ctx, apps[i].ID)
		if err != nil {
			return nil, err
		}
		for j := range fns {
			// The v2 function payload only carries an app ID; attach the full app.
			app := apps[i]
			fns[j].App = &app
		}
		functions = append(functions, fns...)
	}
	return functions, nil
}

// GetFunction finds a function by slug or ID across all apps.
func (c *Client) GetFunction(ctx context.Context, slugOrID string) (*Function, error) {
	functions, err := c.ListFunctions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range functions {
		if functions[i].Slug == slugOrID || functions[i].ID == slugOrID {
			return &functions[i], nil
		}
	}
	return nil, fmt.Errorf("inngest: function %q not found", slugOrID)
}

// InvokeResult is the response of a function invocation.
type InvokeResult struct {
	RunID       string     `json:"runId"`
	QueuedAt    *time.Time `json:"queuedAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// InvokeFunction invokes a function with the given event data
// (POST /v2/apps/{appId}/functions/{functionId}/invoke). idempotencyKey is optional.
// In dev mode the function is invoked by slug via the dev server and appID is ignored.
func (c *Client) InvokeFunction(ctx context.Context, appID, functionID string, data any, idempotencyKey string) (*InvokeResult, error) {
	if data == nil {
		data = map[string]any{}
	}
	if c.devMode {
		runID, err := c.InvokeDevFunction(ctx, functionID, data)
		if err != nil {
			return nil, err
		}
		return &InvokeResult{RunID: runID}, nil
	}

	body := map[string]any{"data": data}
	if idempotencyKey != "" {
		body["idempotencyKey"] = idempotencyKey
	}

	var result InvokeResult
	path := "apps/" + url.PathEscape(appID) + "/functions/" + url.PathEscape(functionID) + "/invoke"
	if err := c.v2Post(ctx, path, body, &result); err != nil {
		return nil, fmt.Errorf("inngest: invoke function %s: %w", functionID, err)
	}
	return &result, nil
}
