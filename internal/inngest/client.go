package inngest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var signingKeyPrefixRegexp = regexp.MustCompile(`^signkey-\w+-`)

const (
	defaultAPIBaseURL   = "https://api.inngest.com"
	defaultDevServerURL = "http://localhost:8288"
	defaultEventHost    = "https://inn.gs"
	maxRetries          = 3
)

// Client is the Inngest API client.
type Client struct {
	signingKey         string
	signingKeyFallback string
	eventKey           string
	env                string
	apiBaseURL         string
	devServerURL       string
	devMode            bool
	httpClient         *http.Client
	userAgent          string
}

// ClientOptions configures a new Client.
type ClientOptions struct {
	SigningKey         string
	SigningKeyFallback string
	EventKey           string
	Env                string
	APIBaseURL         string // default: https://api.inngest.com
	DevServerURL       string // default: http://localhost:8288
	DevMode            bool
	UserAgent          string
}

// NewClient creates a new Inngest API client.
func NewClient(opts ClientOptions) *Client {
	apiBase := opts.APIBaseURL
	if apiBase == "" {
		apiBase = defaultAPIBaseURL
	}
	apiBase = strings.TrimRight(apiBase, "/")

	devURL := opts.DevServerURL
	if devURL == "" {
		devURL = defaultDevServerURL
	}
	devURL = strings.TrimRight(devURL, "/")

	ua := opts.UserAgent
	if ua == "" {
		ua = "inngest-cli/dev"
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &Client{
		signingKey:         opts.SigningKey,
		signingKeyFallback: opts.SigningKeyFallback,
		eventKey:           opts.EventKey,
		env:                opts.Env,
		apiBaseURL:         apiBase,
		devServerURL:       devURL,
		devMode:            opts.DevMode,
		userAgent:          ua,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// do executes an HTTP request with auth headers, retry logic for 429s,
// and signing key fallback on 401 Unauthorized.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	// Buffer the request body so we can replay it on fallback retry.
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	req.Header.Set("User-Agent", c.userAgent)
	if c.signingKey != "" {
		hashed, err := HashSigningKey(c.signingKey)
		if err != nil {
			// Fall back to raw key if hashing fails (e.g. non-hex self-hosted key).
			req.Header.Set("Authorization", "Bearer "+c.signingKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+hashed)
		}
	}
	if c.env != "" && !c.devMode {
		req.Header.Set("X-Inngest-Env", c.env)
	}

	resp, err := c.doWithRetry(req, bodyBytes)
	if err != nil {
		return nil, err
	}

	// If auth failed and we have a fallback key, retry with it.
	if resp.StatusCode == http.StatusUnauthorized && c.signingKeyFallback != "" {
		_ = resp.Body.Close()
		hashed, err := HashSigningKey(c.signingKeyFallback)
		if err != nil {
			req.Header.Set("Authorization", "Bearer "+c.signingKeyFallback)
		} else {
			req.Header.Set("Authorization", "Bearer "+hashed)
		}
		return c.doWithRetry(req, bodyBytes)
	}

	return resp, nil
}

// doWithRetry executes the request with retry logic for 429 Too Many Requests.
// bodyBytes is the buffered request body (nil for GET requests with no body)
// used to reset req.Body before each retry so the body is not empty/EOF.
func (c *Client) doWithRetry(req *http.Request, bodyBytes []byte) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := range maxRetries {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		resp, err = c.httpClient.Do(req) //nolint:gosec // URL constructed from configured API base, not untrusted input
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// Rate limited — parse Retry-After and wait.
		retryAfter := resp.Header.Get("Retry-After")
		_ = resp.Body.Close()

		wait := time.Duration(attempt+1) * time.Second // default backoff
		if retryAfter != "" {
			if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
				wait = time.Duration(seconds) * time.Second
			} else if t, parseErr := http.ParseTime(retryAfter); parseErr == nil {
				wait = time.Until(t)
				if wait < 0 {
					wait = time.Second
				}
			}
		}

		time.Sleep(wait)
	}

	return resp, nil
}

// graphqlURL returns the GraphQL endpoint URL.
func (c *Client) graphqlURL() string {
	if c.devMode {
		return c.devServerURL + "/v0/gql"
	}
	return c.apiBaseURL + "/gql"
}

// restURL returns the REST v1 API URL for the given path.
func (c *Client) restURL(path string) string {
	base := c.apiBaseURL
	if c.devMode {
		base = c.devServerURL
	}
	return base + "/v1/" + strings.TrimLeft(path, "/")
}

// eventURL returns the event ingestion URL.
func (c *Client) eventURL() string {
	if c.devMode {
		return c.devServerURL + "/e/" + c.eventKey
	}
	return defaultEventHost + "/e/" + c.eventKey
}

// devURL returns a dev server URL for the given path.
func (c *Client) devURL(path string) string {
	return c.devServerURL + "/" + strings.TrimLeft(path, "/")
}

// HashSigningKey hashes a signing key to match the Inngest SDK convention.
// The raw key is never sent over the wire — instead:
//  1. Strip the "signkey-{env}-" prefix
//  2. Hex-decode the remaining string
//  3. SHA-256 hash those bytes
//  4. Hex-encode the hash
//
// This matches inngest/inngest pkg/authn/signing_key_strategy.go HashedSigningKey
// and inngest/inngest-js helpers/strings.ts hashSigningKey.
func HashSigningKey(key string) (string, error) {
	normalized := signingKeyPrefixRegexp.ReplaceAllString(key, "")
	raw, err := hex.DecodeString(normalized)
	if err != nil {
		return "", fmt.Errorf("signing key is not valid hex: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
