package inngest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DevServerInfo is the response from GET /dev.
type DevServerInfo struct {
	Version      string      `json:"version"`
	StartOpts    interface{} `json:"startOpts,omitempty"`
	Functions    []Function  `json:"functions"`
	EventKeyHash string      `json:"eventKeyHash,omitempty"`
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
		return nil, fmt.Errorf("inngest: dev info returned status %d: %s", resp.StatusCode, string(respBody))
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
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// SendDevEvent sends an event to the dev server (POST /e/{key}).
// Returns the list of event IDs created.
func (c *Client) SendDevEvent(ctx context.Context, eventData interface{}) ([]string, error) {
	body, err := json.Marshal(eventData)
	if err != nil {
		return nil, fmt.Errorf("inngest: marshal event data: %w", err)
	}

	// Dev server accepts any event key; use the configured key or fall back to "test".
	eventKey := c.eventKey
	if eventKey == "" {
		eventKey = "test"
	}
	url := c.devServerURL + "/e/" + eventKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("inngest: create send event request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("inngest: send event request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("inngest: read send event response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inngest: send event returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		IDs    []string `json:"ids"`
		Status int      `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("inngest: unmarshal send event response: %w", err)
	}

	return result.IDs, nil
}

// InvokeDevFunction invokes a function on the dev server (POST /invoke/{slug}).
// Returns the run ID.
func (c *Client) InvokeDevFunction(ctx context.Context, slug string, data interface{}) (string, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("inngest: marshal invoke data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.devURL("invoke/"+slug), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("inngest: create invoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("inngest: invoke request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("inngest: read invoke response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("inngest: invoke returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("inngest: unmarshal invoke response: %w", err)
	}

	return result.ID, nil
}
