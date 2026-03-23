package cli

import (
	"os"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/commands"
	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	// Flag values
	outputFormat string
	flagEnv      string
	flagAPIURL   string
	flagDev      bool
	flagDevURL   string
)

// Execute runs the root command.
func Execute(version string) error {
	state.AppVersion = version
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		output.PrintError(err.Error(), nil)
		return err
	}
	return nil
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inngest",
		Short: "CLI for Inngest — monitor, debug, and manage functions",
		Long: `inngest is a command-line interface for Inngest.

Monitor, debug, and manage your Inngest functions from the terminal.
Works with both Inngest Cloud and local dev server.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			state.Config = cfg

			// Resolve env: flag > config > default "production"
			state.Env = "production"
			if cfg.ActiveEnv != "" {
				state.Env = cfg.ActiveEnv
			}
			if flagEnv != "" {
				state.Env = flagEnv
			}

			// Resolve API base URL: flag > config > default
			state.APIBaseURL = cfg.GetAPIBaseURL()
			if flagAPIURL != "" {
				state.APIBaseURL = flagAPIURL
			}

			// Resolve dev server URL: flag > config > default
			state.DevServer = cfg.GetDevServerURL()
			if flagDevURL != "" {
				state.DevServer = flagDevURL
			}

			state.DevMode = flagDev
			state.Output = outputFormat

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json, text, table")
	cmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "", "Target environment (production, staging, branch name)")
	cmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "Override API base URL (for self-hosted Inngest)")
	cmd.PersistentFlags().BoolVar(&flagDev, "dev", false, "Target local dev server instead of Inngest Cloud")
	cmd.PersistentFlags().StringVar(&flagDevURL, "dev-url", "", "Override dev server URL")

	cmd.SetErr(os.Stderr)
	cmd.SetOut(os.Stdout)

	// Register command groups
	cmd.AddCommand(commands.NewAuthCmd())
	cmd.AddCommand(commands.NewVersionCmd())
	cmd.AddCommand(commands.NewConfigCmd())
	cmd.AddCommand(commands.NewDevCmd())
	cmd.AddCommand(commands.NewEventsCmd())
	cmd.AddCommand(commands.NewFunctionsCmd())
	cmd.AddCommand(commands.NewRunsCmd())
	cmd.AddCommand(commands.NewEnvCmd())
	cmd.AddCommand(commands.NewHealthCmd())
	cmd.AddCommand(commands.NewMetricsCmd())
	cmd.AddCommand(commands.NewBacklogCmd())

	return cmd
}
