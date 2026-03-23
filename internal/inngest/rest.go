package inngest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetREST performs a GET request to the REST v1 API and unmarshals the response.
func (c *Client) GetREST(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.restURL(path), nil)
	if err != nil {
		return fmt.Errorf("inngest: create GET request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("inngest: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("inngest: read GET response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("inngest: GET %s returned status %d: %s", path, resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("inngest: unmarshal GET response: %w", err)
		}
	}

	return nil
}
