package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config holds all Inngest CLI settings and credentials.
type Config struct {
	// SigningKey is used for REST API auth (Bearer token)
	SigningKey string `json:"signing_key,omitempty"`
	// SigningKeyFallback is used when the primary signing key fails (e.g. during key rotation)
	SigningKeyFallback string `json:"signing_key_fallback,omitempty"`
	// EventKey is used for sending events via inn.gs/e/{key}
	EventKey string `json:"event_key,omitempty"`
	// ActiveEnv is the currently selected environment (e.g. "production", "staging")
	ActiveEnv string `json:"active_env,omitempty"`
	// APIBaseURL overrides the default API URL (for self-hosted Inngest)
	APIBaseURL string `json:"api_base_url,omitempty"`
	// DevServerURL is the dev server URL (default http://localhost:8288)
	DevServerURL string `json:"dev_server_url,omitempty"`
}

var (
	configPath string
	once       sync.Once
)

// ResetForTest resets the cached config path so tests can set INNGEST_CLI_CONFIG.
// Only call from tests.
func ResetForTest() {
	once = sync.Once{}
	configPath = ""
}

// DefaultConfigPath returns the config file path using cross-platform resolution:
//  1. XDG_CONFIG_HOME env var → $XDG_CONFIG_HOME/inngest/cli.json (freedesktop basedir spec)
//  2. os.UserConfigDir() → {result}/inngest/cli.json (~/Library/Application Support on macOS, %AppData% on Windows, ~/.config on Linux)
//  3. Fallback: ~/.config/inngest/cli.json (only if os.UserConfigDir fails)
//
// Note: INNGEST_CLI_CONFIG env var override is handled by getConfigPath().
func DefaultConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "inngest", "cli.json")
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "inngest", "cli.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "inngest", "cli.json")
}

func getConfigPath() string {
	once.Do(func() {
		if p := os.Getenv("INNGEST_CLI_CONFIG"); p != "" {
			// Validate: must be an absolute path ending in .json, no traversal
			clean := filepath.Clean(p)
			if filepath.IsAbs(clean) && strings.HasSuffix(clean, ".json") && !strings.Contains(clean, "..") {
				configPath = clean
				return
			}
			// Fall through to default if env var is invalid
		}
		configPath = DefaultConfigPath()
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

// GetSigningKey returns the signing key from config or INNGEST_SIGNING_KEY env var.
func (c *Config) GetSigningKey() string {
	if c.SigningKey != "" {
		return c.SigningKey
	}
	return os.Getenv("INNGEST_SIGNING_KEY")
}

// GetSigningKeyFallback returns the fallback signing key from config or INNGEST_SIGNING_KEY_FALLBACK env var.
func (c *Config) GetSigningKeyFallback() string {
	if c.SigningKeyFallback != "" {
		return c.SigningKeyFallback
	}
	return os.Getenv("INNGEST_SIGNING_KEY_FALLBACK")
}

// GetEventKey returns the event key from config or INNGEST_EVENT_KEY env var.
func (c *Config) GetEventKey() string {
	if c.EventKey != "" {
		return c.EventKey
	}
	return os.Getenv("INNGEST_EVENT_KEY")
}

// GetAPIBaseURL returns the API base URL, defaulting to https://api.inngest.com.
func (c *Config) GetAPIBaseURL() string {
	if c.APIBaseURL != "" {
		return c.APIBaseURL
	}
	return "https://api.inngest.com"
}

// GetDevServerURL returns the dev server URL, defaulting to http://localhost:8288.
func (c *Config) GetDevServerURL() string {
	if c.DevServerURL != "" {
		return c.DevServerURL
	}
	return "http://localhost:8288"
}

// GetActiveEnv returns the active environment, defaulting to "production".
func (c *Config) GetActiveEnv() string {
	if c.ActiveEnv != "" {
		return c.ActiveEnv
	}
	return "production"
}

// IsConfigured reports whether a signing key or event key is available.
func (c *Config) IsConfigured() bool {
	return c.GetSigningKey() != "" || c.GetEventKey() != ""
}

// Redact masks a secret string, showing first 4 + **** + last 4 characters.
func Redact(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
