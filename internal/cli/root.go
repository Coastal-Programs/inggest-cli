package cli

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/commands"
	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/common/config"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// Exit codes, modelled on gh: scripts and agents can branch on them.
const (
	ExitOK     = 0
	ExitError  = 1
	ExitCancel = 2 // interrupted (Ctrl+C / SIGTERM)
	ExitAuth   = 4 // the API rejected the credential
)

var (
	// Flag values
	outputFormat string
	flagEnv      string
	flagAPIURL   string
	flagDev      bool
	flagDevURL   string
	flagTimeout  time.Duration
)

// Execute runs the root command and returns the process exit code.
// Ctrl+C / SIGTERM cancel the command context so in-flight requests abort.
func Execute(version string) int {
	state.AppVersion = version
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := newRootCmd().ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	output.PrintError(err.Error(), nil)
	return exitCodeFor(ctx, err)
}

func exitCodeFor(ctx context.Context, err error) int {
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		return ExitCancel
	case inngest.IsAuthError(err):
		return ExitAuth
	default:
		return ExitError
	}
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
			state.Timeout = flagTimeout

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json, text, table")
	cmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "", "Target environment (production, staging, branch name)")
	cmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "Override API base URL (for self-hosted Inngest)")
	cmd.PersistentFlags().BoolVar(&flagDev, "dev", false, "Target local dev server instead of Inngest Cloud")
	cmd.PersistentFlags().StringVar(&flagDevURL, "dev-url", "", "Override dev server URL")
	cmd.PersistentFlags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "Per-request timeout (e.g. 10s, 2m)")

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
	cmd.AddCommand(commands.NewAppsCmd())
	cmd.AddCommand(commands.NewAPICmd())

	return cmd
}
