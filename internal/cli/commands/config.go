package commands

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

const (
	configKeyEventKey     = "event_key"
	configKeyActiveEnv    = "active_env"
	configKeyAPIBaseURL   = "api_base_url"
	configKeyDevServerURL = "dev_server_url"
	configKeySigningKey   = "signing_key"

	sourceConfig        = "config"
	sourceDefault       = "default"
	sourceEnvSigningKey = "env (INNGEST_SIGNING_KEY)"
	sourceEnvEventKey   = "env (INNGEST_EVENT_KEY)"
)

var validConfigKeys = []string{
	configKeySigningKey,
	configKeyEventKey,
	configKeyActiveEnv,
	configKeyAPIBaseURL,
	configKeyDevServerURL,
}

func isValidKey(key string) bool {
	return slices.Contains(validConfigKeys, key)
}

func isSecretKey(key string) bool {
	return key == configKeySigningKey || key == configKeyEventKey
}

func getConfigValue(cfg *config.Config, key string) string {
	switch key {
	case configKeySigningKey:
		return cfg.GetSigningKey()
	case configKeyEventKey:
		return cfg.GetEventKey()
	case configKeyActiveEnv:
		return cfg.GetActiveEnv()
	case configKeyAPIBaseURL:
		return cfg.GetAPIBaseURL()
	case configKeyDevServerURL:
		return cfg.GetDevServerURL()
	default:
		return ""
	}
}

func configSource(cfg *config.Config, key string) string {
	switch key {
	case configKeySigningKey:
		if cfg.SigningKey != "" {
			return sourceConfig
		}
		if os.Getenv("INNGEST_SIGNING_KEY") != "" {
			return sourceEnvSigningKey
		}
		return sourceDefault
	case configKeyEventKey:
		if cfg.EventKey != "" {
			return sourceConfig
		}
		if os.Getenv("INNGEST_EVENT_KEY") != "" {
			return sourceEnvEventKey
		}
		return sourceDefault
	case configKeyActiveEnv:
		if cfg.ActiveEnv != "" {
			return sourceConfig
		}
		return sourceDefault
	case configKeyAPIBaseURL:
		if cfg.APIBaseURL != "" {
			return sourceConfig
		}
		return sourceDefault
	case configKeyDevServerURL:
		if cfg.DevServerURL != "" {
			return sourceConfig
		}
		return sourceDefault
	default:
		return "unknown"
	}
}

// NewConfigCmd returns the "config" command group.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  "Show, get, and set Inngest CLI configuration values.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigPathCmd())
	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show all configuration values",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := state.Config

			type configEntry struct {
				Key    string `json:"key"`
				Value  string `json:"value"`
				Source string `json:"source"`
			}

			entries := make([]configEntry, 0, len(validConfigKeys))
			for _, key := range validConfigKeys {
				val := getConfigValue(cfg, key)
				if isSecretKey(key) && val != "" {
					val = config.Redact(val)
				}
				entries = append(entries, configEntry{
					Key:    key,
					Value:  val,
					Source: configSource(cfg, key),
				})
			}
			return output.Print(entries, output.Format(state.Output))
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	var raw bool

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single configuration value",
		Long:  "Valid keys: " + strings.Join(validConfigKeys, ", "),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if !isValidKey(key) {
				return fmt.Errorf("unrecognized config key %q; valid keys: %s", key, strings.Join(validConfigKeys, ", "))
			}
			val := getConfigValue(state.Config, key)
			if !raw && isSecretKey(key) && val != "" {
				val = config.Redact(val)
			}
			return output.Print(map[string]string{
				"key":   key,
				"value": val,
			}, output.Format(state.Output))
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Show unredacted secret values")
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  "Valid keys: " + strings.Join(validConfigKeys, ", "),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			if !isValidKey(key) {
				return fmt.Errorf("unrecognized config key %q; valid keys: %s", key, strings.Join(validConfigKeys, ", "))
			}

			// Validate signing key format
			if key == configKeySigningKey {
				if err := validateSigningKey(value); err != nil {
					return fmt.Errorf("invalid signing key format: %w", err)
				}
			}

			cfg := state.Config
			switch key {
			case configKeySigningKey:
				cfg.SigningKey = value
			case configKeyEventKey:
				cfg.EventKey = value
			case configKeyActiveEnv:
				cfg.ActiveEnv = value
			case configKeyAPIBaseURL:
				cfg.APIBaseURL = value
			case configKeyDevServerURL:
				cfg.DevServerURL = value
			}

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			displayValue := value
			if isSecretKey(key) {
				displayValue = config.Redact(value)
			}
			return output.Print(map[string]string{
				"status": "ok",
				"key":    key,
				"value":  displayValue,
			}, output.Format(state.Output))
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			return output.Print(map[string]string{
				"path": config.DefaultConfigPath(),
			}, output.Format(state.Output))
		},
	}
}
