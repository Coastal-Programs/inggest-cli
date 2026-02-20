package commands

import (
	"github.com/jakeschepis/zeus-cli/internal/common/config"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewConfigCmd returns the config command group.
func NewConfigCmd(format *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get and set CLI configuration values",
		Long: `Manage xero CLI configuration.

Config is stored at ~/.config/xero/config.json (override with XERO_CONFIG env var).

Available keys:
  client_id       Xero OAuth app client ID
  client_secret   Xero OAuth app client secret
  tenant_id       Active Xero organisation (tenant) ID
  tenant_name     Active Xero organisation name`,
	}
	cmd.AddCommand(
		newConfigGetCmd(format),
		newConfigSetCmd(format),
		newConfigShowCmd(format),
	)
	return cmd
}

func newConfigGetCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Example: `  xero config get client_id
  xero config get tenant_name`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			val, err := cfg.Get(args[0])
			if err != nil {
				return err
			}
			return output.Print(map[string]string{args[0]: val}, output.Format(*format))
		},
	}
}

func newConfigSetCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Example: `  xero config set client_id my-client-id
  xero config set client_secret my-secret`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.Set(args[0], args[1]); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			return output.Print(map[string]string{
				"status": "ok",
				"key":    args[0],
			}, output.Format(*format))
		},
	}
}

func newConfigShowCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show all config values (sensitive fields redacted)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return output.Print(cfg.Redacted(), output.Format(*format))
		},
	}
}
