package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// NewEnvCmd returns the "env" command group.
func NewEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments (workspaces)",
		Long:  "List, inspect, and switch between Inngest environments.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newEnvListCmd())
	cmd.AddCommand(newEnvUseCmd())
	cmd.AddCommand(newEnvGetCmd())
	return cmd
}

// envRow is used for table output of env list.
type envRow struct {
	Name   string
	Slug   string
	Type   string
	ID     string
	Active string
}

func newEnvListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environments",
		Long:  "List all environments registered with Inngest Cloud.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			envs, err := client.ListEnvironments(ctx)
			if err != nil {
				if errors.Is(err, inngest.ErrAccountAuthRequired) || strings.Contains(strings.ToLower(err.Error()), "authenticat") {
					return printCurrentEnvFallback(format, err)
				}
				return fmt.Errorf("listing environments: %w", err)
			}

			if format == output.FormatTable {
				return printEnvTable(envs)
			}

			return output.Print(envs, format)
		},
	}
}

func printEnvTable(envs []inngest.Environment) error {
	activeEnv := state.Env
	rows := make([]envRow, len(envs))
	for i, env := range envs {
		active := ""
		if strings.EqualFold(env.Name, activeEnv) || strings.EqualFold(env.Slug, activeEnv) {
			active = "◀"
		}
		rows[i] = envRow{
			Name:   env.Name,
			Slug:   env.Slug,
			Type:   env.Type,
			ID:     env.ID,
			Active: active,
		}
	}
	return output.Print(rows, output.FormatTable)
}

func newEnvUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active environment",
		Long:  "Set the active environment in config. Subsequent commands will target this environment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := state.Config
			cfg.ActiveEnv = name
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			state.Env = name
			return output.Print(map[string]string{
				"status":     "ok",
				"active_env": name,
			}, output.Format(state.Output))
		},
	}
}

func newEnvGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name-or-id>",
		Short: "Get environment details",
		Long:  "Fetch environment details by name, slug, or ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()
			nameOrID := args[0]

			env, err := client.GetEnvironment(ctx, nameOrID)
			if err != nil {
				if errors.Is(err, inngest.ErrAccountAuthRequired) || strings.Contains(strings.ToLower(err.Error()), "authenticat") {
					return printCurrentEnvFallback(format, err)
				}
				return fmt.Errorf("environment %q not found: %w", nameOrID, err)
			}

			if format == output.FormatText {
				return printEnvDetail(env)
			}

			return output.Print(env, format)
		},
	}
}

func printEnvDetail(env *inngest.Environment) error {
	fmt.Printf("Name:          %s\n", env.Name)
	fmt.Printf("ID:            %s\n", env.ID)
	fmt.Printf("Slug:          %s\n", env.Slug)
	fmt.Printf("Type:          %s\n", env.Type)
	if env.CreatedAt != nil {
		fmt.Printf("Created:       %s\n", env.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Auto-Archive:  %v\n", env.IsAutoArchiveEnabled)
	return nil
}

// printCurrentEnvFallback shows the current environment from config when the
// API requires account-level auth that we don't have. It prints a warning to
// stderr, then outputs the locally-known environment info to stdout.
func printCurrentEnvFallback(format output.Format, authErr error) error {
	activeEnv := state.Config.GetActiveEnv()

	// Print warning to stderr so structured output on stdout stays clean.
	fmt.Fprintln(os.Stderr, "Warning: "+authErr.Error())
	fmt.Fprintln(os.Stderr, "Showing current environment from local config instead.")
	fmt.Fprintln(os.Stderr)

	info := map[string]string{
		"active_env": activeEnv,
		"source":     "local_config",
		"hint":       "Visit https://app.inngest.com/env to manage all environments",
	}
	return output.Print(info, format)
}
