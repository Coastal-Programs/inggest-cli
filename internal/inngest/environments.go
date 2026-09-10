package inngest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ListEnvironments lists the account's environments (GET /v2/envs).
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error) {
	q := url.Values{"limit": {strconv.Itoa(v2ListPageSize)}}
	envs := []Environment{}
	for range maxListPages {
		var data []Environment
		page, err := c.v2Get(ctx, "envs", q, &data)
		if err != nil {
			return nil, fmt.Errorf("inngest: list environments: %w", err)
		}
		envs = append(envs, data...)
		if page == nil || !page.HasMore || page.Cursor == "" {
			return envs, nil
		}
		q.Set("cursor", page.Cursor)
	}
	return envs, nil
}

// GetEnvironment fetches a single environment by ID or name (case-insensitive).
func (c *Client) GetEnvironment(ctx context.Context, nameOrID string) (*Environment, error) {
	envs, err := c.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	for i := range envs {
		if envs[i].ID == nameOrID || strings.EqualFold(envs[i].Name, nameOrID) {
			return &envs[i], nil
		}
	}
	return nil, fmt.Errorf("inngest: environment %q not found", nameOrID)
}
