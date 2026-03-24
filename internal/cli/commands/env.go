package commands

import (
	"context"
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
		Short: "Manage environments (apps/deployments)",
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
	Name      string
	SDK       string
	Framework string
	URL       string
	Connected string
	Functions int
	Active    string
}

func newEnvListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environments (apps)",
		Long:  "List all apps registered with Inngest Cloud or the local dev server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			apps, err := client.ListApps(ctx)
			if err != nil {
				return fmt.Errorf("listing environments: %w", err)
			}

			if format == output.FormatTable {
				return printEnvTable(apps)
			}

			return output.Print(apps, format)
		},
	}
}

func printEnvTable(apps []inngest.App) error {
	activeEnv := state.Env
	rows := make([]envRow, len(apps))
	for i, app := range apps {
		sdk := app.SDKLanguage
		if app.SDKVersion != "" {
			sdk += "/" + app.SDKVersion
		}
		connected := "no"
		if app.Connected {
			connected = "yes"
		}
		active := ""
		if strings.EqualFold(app.Name, activeEnv) {
			active = "◀"
		}
		rows[i] = envRow{
			Name:      app.Name,
			SDK:       sdk,
			Framework: app.Framework,
			URL:       app.URL,
			Connected: connected,
			Functions: app.FunctionCount,
			Active:    active,
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
		Short: "Get detailed environment (app) info",
		Long:  "Fetch full environment details including connected status and all functions.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()
			nameOrID := args[0]

			// Try to find by name first by listing all apps.
			apps, err := client.ListApps(ctx)
			if err != nil {
				return fmt.Errorf("listing environments: %w", err)
			}

			var app *inngest.App
			for i := range apps {
				if strings.EqualFold(apps[i].Name, nameOrID) || apps[i].ID == nameOrID {
					app = &apps[i]
					break
				}
			}

			// If not found by name, try by ID via GetApp.
			if app == nil {
				app, err = client.GetApp(ctx, nameOrID)
				if err != nil {
					return fmt.Errorf("environment %q not found", nameOrID)
				}
			} else {
				// Fetch full details with functions/triggers via GetApp.
				detailed, err := client.GetApp(ctx, app.ID)
				if err == nil {
					app = detailed
				}
			}

			if format == output.FormatText {
				return printEnvDetail(app)
			}

			return output.Print(app, format)
		},
	}
}

func printEnvDetail(app *inngest.App) error {
	fmt.Printf("Name:          %s\n", app.Name)
	fmt.Printf("ID:            %s\n", app.ID)
	fmt.Printf("External ID:   %s\n", app.ExternalID)
	fmt.Printf("SDK:           %s/%s\n", app.SDKLanguage, app.SDKVersion)
	if app.Framework != "" {
		fmt.Printf("Framework:     %s\n", app.Framework)
	}
	if app.URL != "" {
		fmt.Printf("URL:           %s\n", app.URL)
	}
	if app.Method != "" {
		fmt.Printf("Method:        %s\n", app.Method)
	}
	fmt.Printf("Connected:     %v\n", app.Connected)
	fmt.Printf("Functions:     %d\n", app.FunctionCount)
	if app.Checksum != "" {
		fmt.Printf("Checksum:      %s\n", app.Checksum)
	}
	if app.Error != "" {
		fmt.Printf("Error:         %s\n", app.Error)
	}

	if len(app.Functions) > 0 {
		fmt.Printf("\nFunctions:\n")
		for _, fn := range app.Functions {
			fmt.Printf("  - %s (%s)\n", fn.Name, fn.Slug)
			for _, t := range fn.Triggers {
				fmt.Printf("      trigger: %s:%s\n", t.Type, t.Value)
			}
		}
	}

	return nil
}
