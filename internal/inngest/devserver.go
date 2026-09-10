package inngest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DevServerInfo is the response from GET /dev.
type DevServerInfo struct {
	Version      string     `json:"version"`
	StartOpts    any        `json:"startOpts,omitempty"`
	Functions    []Function `json:"functions"`
	EventKeyHash string     `json:"eventKeyHash,omitempty"`
}

// GetDevInfo fetches dev server info (GET /dev).
func (c *Client) GetDevInfo(ctx context.Context) (*DevServerInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.devURL("dev"), nil)
	if err != nil {
		return nil, fmt.Errorf("inngest: create dev info request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("inngest: dev info request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("inngest: read dev info response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inngest: dev info returned status %d: %s", resp.StatusCode, truncateBody(string(respBody)))
	}

	var info DevServerInfo
	if err := json.Unmarshal(respBody, &info); err != nil {
		return nil, fmt.Errorf("inngest: unmarshal dev info: %w", err)
	}

	return &info, nil
}

// IsDevServerRunning checks if the dev server is reachable.
func (c *Client) IsDevServerRunning(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.devURL("dev"), nil)
	if err != nil {
		return false
	}

	resp, err := c.do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
