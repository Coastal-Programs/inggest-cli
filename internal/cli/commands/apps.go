package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// NewAppsCmd returns the "apps" command group.
func NewAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "List, inspect, and sync apps (SDK deployments)",
		Long:  "Apps are synced SDK deployments; each owns a set of functions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAppsListCmd())
	cmd.AddCommand(newAppsGetCmd())
	cmd.AddCommand(newAppsSyncCmd())
	return cmd
}

type appRow struct {
	Name      string
	ID        string
	Method    string
	Functions int
	SDK       string
}

func printAppsTable(apps []inngest.App) error {
	rows := make([]appRow, len(apps))
	for i, app := range apps {
		sdk := app.SDKLanguage
		if app.SDKVersion != "" {
			sdk += "/" + app.SDKVersion
		}
		rows[i] = appRow{Name: app.Name, ID: app.ID, Method: app.Method, Functions: app.FunctionCount, SDK: sdk}
	}
	return output.Print(rows, output.FormatTable)
}

func newAppsListCmd() *cobra.Command {
	var archived bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List apps in the current environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			apps, err := client.ListApps(cmd.Context(), archived)
			if err != nil {
				return fmt.Errorf("listing apps: %w", err)
			}
			if format == output.FormatTable {
				return printAppsTable(apps)
			}
			return output.Print(apps, format)
		},
	}

	cmd.Flags().BoolVar(&archived, "archived", false, "List archived apps instead of active ones")

	return cmd
}

func newAppsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <app-id>",
		Short: "Get app details including its latest sync",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			app, err := client.GetApp(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("getting app: %w", err)
			}
			return output.Print(app, output.Format(state.Output))
		},
	}
}

func newAppsSyncCmd() *cobra.Command {
	var serveURL string

	cmd := &cobra.Command{
		Use:   "sync <app-id>",
		Short: "Sync an app from its serve URL",
		Long:  "Ask Inngest to re-fetch the app's functions from the given SDK serve endpoint (e.g. after a deploy).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			result, err := client.SyncApp(cmd.Context(), args[0], serveURL)
			if err != nil {
				return fmt.Errorf("syncing app: %w", err)
			}
			return output.Print(result, output.Format(state.Output))
		},
	}

	cmd.Flags().StringVar(&serveURL, "url", "", "SDK serve URL, e.g. https://example.com/api/inngest (required)")
	_ = cmd.MarkFlagRequired("url")

	return cmd
}
