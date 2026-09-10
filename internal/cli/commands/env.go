package commands

import (
	"fmt"
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
	Type   string
	ID     string
	Active string
}

func newEnvListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environments",
		Long:  "List the environments of the account that owns the configured key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			envs, err := client.ListEnvironments(cmd.Context())
			if err != nil {
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
	rows := make([]envRow, len(envs))
	for i, env := range envs {
		active := ""
		if strings.EqualFold(env.Name, state.Env) {
			active = "◀"
		}
		rows[i] = envRow{
			Name:   env.Name,
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
		Long:  "Fetch environment details by name or ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			env, err := client.GetEnvironment(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("getting environment: %w", err)
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
	fmt.Printf("Type:          %s\n", env.Type)
	if env.CreatedAt != nil {
		fmt.Printf("Created:       %s\n", env.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Archived:      %v\n", env.IsArchived)
	return nil
}
