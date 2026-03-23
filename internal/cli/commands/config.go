package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
	"github.com/spf13/cobra"
)

var validConfigKeys = []string{
	"signing_key",
	"event_key",
	"active_env",
	"api_base_url",
	"dev_server_url",
}

func isValidKey(key string) bool {
	for _, k := range validConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

func isSecretKey(key string) bool {
	return key == "signing_key" || key == "event_key"
}

func getConfigValue(cfg *config.Config, key string) string {
	switch key {
	case "signing_key":
		return cfg.GetSigningKey()
	case "event_key":
		return cfg.GetEventKey()
	case "active_env":
		return cfg.GetActiveEnv()
	case "api_base_url":
		return cfg.GetAPIBaseURL()
	case "dev_server_url":
		return cfg.GetDevServerURL()
	default:
		return ""
	}
}

func configSource(cfg *config.Config, key string) string {
	switch key {
	case "signing_key":
		if cfg.SigningKey != "" {
			return "config"
		}
		if os.Getenv("INNGEST_SIGNING_KEY") != "" {
			return "env (INNGEST_SIGNING_KEY)"
		}
		return "default"
	case "event_key":
		if cfg.EventKey != "" {
			return "config"
		}
		if os.Getenv("INNGEST_EVENT_KEY") != "" {
			return "env (INNGEST_EVENT_KEY)"
		}
		return "default"
	case "active_env":
		if cfg.ActiveEnv != "" {
			return "config"
		}
		return "default"
	case "api_base_url":
		if cfg.APIBaseURL != "" {
			return "config"
		}
		return "default"
	case "dev_server_url":
		if cfg.DevServerURL != "" {
			return "config"
		}
		return "default"
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

			var entries []configEntry
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
			if key == "signing_key" {
				if err := validateSigningKey(value); err != nil {
					return fmt.Errorf("invalid signing key format: %w", err)
				}
			}

			cfg := state.Config
			switch key {
			case "signing_key":
				cfg.SigningKey = value
			case "event_key":
				cfg.EventKey = value
			case "active_env":
				cfg.ActiveEnv = value
			case "api_base_url":
				cfg.APIBaseURL = value
			case "dev_server_url":
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
