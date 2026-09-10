package inngest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// maxListPages bounds cursor-following loops so a misbehaving API cannot hang the CLI.
const maxListPages = 50

// v2 list endpoints accept limit ≤ 100.
const v2ListPageSize = 100

// ListApps lists apps in the current environment (GET /v2/apps).
// Pass archived=true to list archived apps instead of active ones.
func (c *Client) ListApps(ctx context.Context, archived bool) ([]App, error) {
	if c.devMode {
		return c.devListApps(ctx)
	}

	q := url.Values{"limit": {strconv.Itoa(v2ListPageSize)}}
	if archived {
		q.Set("archived", "true")
	}

	apps := []App{}
	for range maxListPages {
		var data []App
		page, err := c.v2Get(ctx, "apps", q, &data)
		if err != nil {
			return nil, fmt.Errorf("inngest: list apps: %w", err)
		}
		apps = append(apps, data...)
		if page == nil || !page.HasMore || page.Cursor == "" {
			return apps, nil
		}
		q.Set("cursor", page.Cursor)
	}
	return apps, nil
}

// GetApp fetches a single app by ID (GET /v2/apps/{appId}).
func (c *Client) GetApp(ctx context.Context, appID string) (*App, error) {
	var app App
	if _, err := c.v2Get(ctx, "apps/"+url.PathEscape(appID), nil, &app); err != nil {
		return nil, fmt.Errorf("inngest: get app %s: %w", appID, err)
	}
	return &app, nil
}

// SyncResult is the outcome of an app sync request.
type SyncResult struct {
	ID     string `json:"id"`
	AppID  string `json:"appId"`
	Status string `json:"status"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// SyncApp triggers a sync of the app at serveURL (POST /v2/apps/{appId}/syncs).
func (c *Client) SyncApp(ctx context.Context, appID, serveURL string) (*SyncResult, error) {
	var result SyncResult
	body := map[string]any{"url": serveURL}
	if err := c.v2Post(ctx, "apps/"+url.PathEscape(appID)+"/syncs", body, &result); err != nil {
		return nil, fmt.Errorf("inngest: sync app %s: %w", appID, err)
	}
	return &result, nil
}
