package inngest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RawResponse is the undecoded result of a passthrough API call.
type RawResponse struct {
	StatusCode int
	Body       []byte
}

// RawRequest performs an authenticated request against an arbitrary API path
// (e.g. "/v2/runs", "/v1/events?limit=5") and returns the raw response. path must
// be a path relative to the API base URL; absolute URLs are rejected so the
// credential can never be sent to another host.
func (c *Client) RawRequest(ctx context.Context, method, path string, body []byte) (*RawResponse, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.HasPrefix(path, "//") || strings.Contains(path, "://") {
		return nil, fmt.Errorf("inngest: path must be relative to the API base URL, got %q", path)
	}
	if _, err := url.ParseRequestURI(path); err != nil {
		return nil, fmt.Errorf("inngest: invalid path %q: %w", path, err)
	}

	base := c.apiBaseURL
	if c.devMode {
		base = c.devServerURL
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), base+path, reader)
	if err != nil {
		return nil, fmt.Errorf("inngest: create %s %s request: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("inngest: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("inngest: read %s %s response: %w", method, path, err)
	}
	return &RawResponse{StatusCode: resp.StatusCode, Body: raw}, nil
}
