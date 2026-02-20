package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)


// Tenant represents a connected Xero organisation.
type Tenant struct {
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
}

// Config holds all Xero CLI settings and OAuth tokens.
type Config struct {
	// OAuth app credentials
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`

	// OAuth tokens (populated after login)
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	TokenExpiry   int64  `json:"token_expiry"`   // Unix timestamp
	InstanceToken string `json:"instance_token"` // Long-lived proxy authorisation token

	// All connected orgs
	Tenants []Tenant `json:"tenants"`

	// Active org (default for commands)
	ActiveTenantID string `json:"active_tenant_id"`
}

var (
	configPath string
	once       sync.Once
)

func getConfigPath() string {
	once.Do(func() {
		if p := os.Getenv("XERO_CONFIG"); p != "" {
			// Validate: must be an absolute path ending in .json, no traversal
			clean := filepath.Clean(p)
			if filepath.IsAbs(clean) && strings.HasSuffix(clean, ".json") && !strings.Contains(clean, "..") {
				configPath = clean
				return
			}
			// Fall through to default if env var is invalid
		}
		home, err := os.UserHomeDir()
		if err != nil {
			configPath = "config.json"
			return
		}
		configPath = filepath.Join(home, ".config", "xero", "config.json")
	})
	return configPath
}

// Load reads the config file. Returns empty Config if file doesn't exist.
func Load() (*Config, error) {
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to disk with 0600 permissions.
func (c *Config) Save() error {
	path := getConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// ActiveTenant returns the currently active Tenant.
func (c *Config) ActiveTenant() (*Tenant, error) {
	if c.ActiveTenantID == "" {
		return nil, fmt.Errorf("no active org — run: xero auth login")
	}
	for _, t := range c.Tenants {
		if t.TenantID == c.ActiveTenantID {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("active tenant not found — run: xero auth login")
}

// ResolveTenant finds a tenant by ID or name (case-insensitive partial match).
// If orgQuery is empty, returns the active tenant.
func (c *Config) ResolveTenant(orgQuery string) (*Tenant, error) {
	if orgQuery == "" {
		return c.ActiveTenant()
	}
	q := strings.ToLower(orgQuery)
	for _, t := range c.Tenants {
		if t.TenantID == orgQuery || strings.Contains(strings.ToLower(t.TenantName), q) {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("org not found: %q — run 'xero orgs list' to see available orgs", orgQuery)
}

// Set updates a named key.
func (c *Config) Set(key, value string) error {
	switch key {
	case "client_id", "client-id":
		c.ClientID = value
	case "client_secret", "client-secret":
		c.ClientSecret = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

// Get returns a named key's value.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "client_id", "client-id":
		return c.ClientID, nil
	case "client_secret", "client-secret":
		return redact(c.ClientSecret), nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

// Redacted returns a copy of Config with sensitive fields masked.
func (c *Config) Redacted() map[string]any {
	return map[string]any{
		"client_id":        c.ClientID,
		"client_secret":    redact(c.ClientSecret),
		"access_token":     redact(c.AccessToken),
		"refresh_token":    redact(c.RefreshToken),
		"active_tenant_id": c.ActiveTenantID,
		"tenants":          c.Tenants,
	}
}

// IsAuthenticated reports whether a valid access token exists.
func (c *Config) IsAuthenticated() bool {
	return c.AccessToken != "" && c.ActiveTenantID != ""
}

func redact(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
