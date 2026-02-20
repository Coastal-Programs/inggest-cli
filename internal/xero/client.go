package xero

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jakeschepis/zeus-cli/internal/common/config"
)

const (
	baseURL   = "https://api.xero.com/api.xro/2.0"
	userAgent = "xero-cli/1.0 (github.com/jakeschepis/zeus-cli)"
)

// shared transport with connection pooling across all clients
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// Client is an authenticated Xero API client scoped to one tenant.
type Client struct {
	accessToken string
	tenantID    string
	httpClient  *http.Client
}

// New creates a Client using the active tenant from config.
func New(cfg *config.Config) (*Client, error) {
	if !cfg.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated — run: xero auth login")
	}
	return &Client{
		accessToken: cfg.AccessToken,
		tenantID:    cfg.ActiveTenantID,
		httpClient:  &http.Client{Timeout: 30 * time.Second, Transport: sharedTransport},
	}, nil
}

// NewForTenant creates a Client targeting a specific tenant ID.
func NewForTenant(cfg *config.Config, tenantID string) (*Client, error) {
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("not authenticated — run: xero auth login")
	}
	return &Client{
		accessToken: cfg.AccessToken,
		tenantID:    tenantID,
		httpClient:  &http.Client{Timeout: 30 * time.Second, Transport: sharedTransport},
	}, nil
}

// TenantID returns the tenant ID this client is scoped to.
func (c *Client) TenantID() string { return c.tenantID }

func (c *Client) get(path string, dst any) error {
	return c.do("GET", path, nil, dst)
}

func (c *Client) post(path string, body any, dst any) error {
	return c.do("POST", path, body, dst)
}

func (c *Client) put(path string, body any, dst any) error {
	return c.do("PUT", path, body, dst)
}

// do executes an HTTP request with automatic 429 retry-after handling.
func (c *Client) do(method, path string, body any, dst any) error {
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := c.doOnce(method, path, body, dst)
		if err == nil {
			return nil
		}
		// Check if it's a rate limit error worth retrying
		var rle *rateLimitError
		if isRateLimitError(err, &rle) && attempt < maxRetries-1 {
			time.Sleep(rle.retryAfter)
			continue
		}
		return err
	}
	return fmt.Errorf("max retries exceeded")
}

type rateLimitError struct {
	retryAfter time.Duration
	problem    string
}

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("rate limited (%s), retry after %s", e.problem, e.retryAfter)
}

func isRateLimitError(err error, out **rateLimitError) bool {
	if rle, ok := err.(*rateLimitError); ok {
		*out = rle
		return true
	}
	return false
}

func (c *Client) doOnce(method, path string, body any, dst any) error {
	url := baseURL + path

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = strings.NewReader(string(b))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Xero-Tenant-Id", c.tenantID)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	// Handle 429 rate limiting with Retry-After
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := 60 * time.Second // safe default
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil {
				retryAfter = time.Duration(secs) * time.Second
			}
		}
		problem := resp.Header.Get("X-Rate-Limit-Problem")
		if problem == "" {
			problem = "rate-limit"
		}
		return &rateLimitError{retryAfter: retryAfter, problem: problem}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("xero api error %d: %s", resp.StatusCode, truncate(string(respBytes), 300))
	}

	if dst != nil {
		if err := json.Unmarshal(respBytes, dst); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
