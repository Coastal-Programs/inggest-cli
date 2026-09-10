package inngest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// REST API v2 client. Reference: https://api-docs.inngest.com (OpenAPI spec at
// https://api-docs.inngest.com/api-specs/v2.json). Every response is wrapped in
// {"data": ..., "page": {...}, "metadata": {...}} and errors in
// {"errors": [{"code": "...", "message": "..."}]}.

// maxResponseBytes caps how much of a response body is read into memory.
// Matches the official Inngest CLI (inngest/inngest cmd/apiv2cli).
const maxResponseBytes = 25 << 20

// APIError is a structured error returned by the Inngest REST API.
type APIError struct {
	StatusCode int
	Code       string // machine-readable code, e.g. "invalid_signing_key", "not_found"
	Message    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("inngest: API error %d (%s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("inngest: API error %d: %s", e.StatusCode, e.Message)
}

// IsAuthError reports whether err is a 401/403 response from the API.
func IsAuthError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden)
}

// IsNotFound reports whether err is a 404 response from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// Page is the cursor-pagination envelope on v2 list responses.
type Page struct {
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"hasMore"`
	Limit   int    `json:"limit,omitempty"`
}

// Millis is a millisecond duration that decodes from either a JSON number or a
// numeric string (protobuf JSON renders int64 as a string).
type Millis int64

func (m *Millis) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*m = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid millisecond value %s: %w", string(b), err)
	}
	*m = Millis(n)
	return nil
}

type v2Envelope struct {
	Data   json.RawMessage `json:"data"`
	Page   *Page           `json:"page"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// v2URL returns the REST v2 URL for path (relative to /v2) with optional query.
// Callers must url.PathEscape any user-supplied path segments.
func (c *Client) v2URL(path string, q url.Values) string {
	base := c.apiBaseURL + "/v2/"
	if c.devMode {
		base = c.devServerURL + "/api/v2/"
	}
	u := base + strings.TrimLeft(path, "/")
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// v2Get performs GET /v2/{path} and decodes the "data" field into out.
func (c *Client) v2Get(ctx context.Context, path string, q url.Values, out any) (*Page, error) {
	return c.v2Do(ctx, http.MethodGet, path, q, nil, out)
}

// v2Post performs POST /v2/{path} with a JSON body and decodes "data" into out.
func (c *Client) v2Post(ctx context.Context, path string, body, out any) error {
	_, err := c.v2Do(ctx, http.MethodPost, path, nil, body, out)
	return err
}

// v2Do is the shared request/decode path for the v2 API. A nil out discards the
// data field; a nil body sends no body.
func (c *Client) v2Do(ctx context.Context, method, path string, q url.Values, body, out any) (*Page, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("inngest: encode %s %s body: %w", method, path, err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.v2URL(path, q), reader)
	if err != nil {
		return nil, fmt.Errorf("inngest: create %s %s request: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
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

	env, err := decodeV2Envelope(resp.StatusCode, raw)
	if err != nil {
		return nil, fmt.Errorf("inngest: %s %s: %w", method, path, err)
	}
	if out != nil && len(env.Data) > 0 && !bytes.Equal(env.Data, []byte("null")) {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return nil, fmt.Errorf("inngest: decode %s %s data: %w", method, path, err)
		}
	}
	return env.Page, nil
}

// decodeV2Envelope parses a v2 response body and converts non-2xx statuses into
// an *APIError carrying the server's error code and message.
func decodeV2Envelope(status int, raw []byte) (*v2Envelope, error) {
	var env v2Envelope
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			if status >= http.StatusMultipleChoices {
				return nil, &APIError{StatusCode: status, Message: truncateBody(string(raw))}
			}
			return nil, fmt.Errorf("decode response: %w", err)
		}
	}
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return &env, nil
	}

	apiErr := &APIError{StatusCode: status, Message: http.StatusText(status)}
	if len(env.Errors) > 0 {
		apiErr.Code = env.Errors[0].Code
		msgs := make([]string, 0, len(env.Errors))
		for _, e := range env.Errors {
			msgs = append(msgs, e.Message)
		}
		apiErr.Message = strings.Join(msgs, "; ")
	}
	return nil, apiErr
}
