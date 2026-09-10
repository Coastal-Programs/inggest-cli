package commands

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// NewDevCmd returns the "dev" command group for interacting with the local dev server.
func NewDevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Interact with the local Inngest dev server",
		Long: `Commands for the local Inngest dev server at localhost:8288. No cloud auth required.

Every cloud command also works against the dev server with the global --dev flag
(e.g. "inngest runs list --dev"); this group is a shorthand for the common ones.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newDevStatusCmd())
	cmd.AddCommand(newDevFunctionsCmd())
	cmd.AddCommand(newDevRunsCmd())
	cmd.AddCommand(newDevSendCmd())
	cmd.AddCommand(newDevInvokeCmd())
	cmd.AddCommand(newDevEventsCmd())
	return cmd
}

func newDevClient() *inngest.Client {
	return inngest.NewClient(inngest.ClientOptions{
		DevServerURL: state.DevServer,
		DevMode:      true,
		UserAgent:    "inngest-cli/" + state.AppVersion,
		Timeout:      state.Timeout,
	})
}

func newDevStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check if the local dev server is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)

			info, err := client.GetDevInfo(cmd.Context())
			var netErr *url.Error
			switch {
			case errors.As(err, &netErr):
				// Transport failure: nothing is listening.
				return output.Print(map[string]any{
					"status":  "offline",
					"url":     state.DevServer,
					"message": "Dev server is not reachable. Start it with: npx inngest-cli@latest dev",
				}, format)
			case err != nil:
				// Something answered but not like a dev server (wrong port, bad JSON).
				return fmt.Errorf("fetching dev server info: %w", err)
			}

			return output.Print(map[string]any{
				"status":    "online",
				"url":       state.DevServer,
				"version":   info.Version,
				"functions": len(info.Functions),
			}, format)
		},
	}
}

func newDevFunctionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "functions",
		Short: "List functions registered with the dev server",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)

			functions, err := client.ListFunctions(cmd.Context())
			if err != nil {
				return fmt.Errorf("querying functions: %w", err)
			}
			if format == output.FormatTable {
				return printFunctionsTable(functions)
			}
			return output.Print(functions, format)
		},
	}
}

func newDevRunsCmd() *cobra.Command {
	var (
		limit    int
		status   string
		since    string
		function string
	)

	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List recent function runs from the dev server",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)

			from, err := parseSince("since", since)
			if err != nil {
				return err
			}

			page, err := client.ListRuns(cmd.Context(), inngest.ListRunsOptions{
				First:       limit,
				From:        from,
				Status:      splitCSV(status),
				FunctionIDs: splitCSV(function),
			})
			if err != nil {
				return fmt.Errorf("querying runs: %w", err)
			}

			if format == output.FormatTable {
				return printRunsTable(page.Runs)
			}
			return output.Print(page, format)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of runs to return")
	cmd.Flags().StringVar(&status, "status", "", "Filter by run status (comma-separated, e.g. COMPLETED,FAILED)")
	cmd.Flags().StringVar(&since, "since", "1h", "Show runs since this duration ago (e.g. 1h, 30m, 24h)")
	cmd.Flags().StringVar(&function, "function", "", "Filter by function ID (comma-separated)")

	return cmd
}

func newDevSendCmd() *cobra.Command {
	var (
		data     string
		dataFile string
	)

	cmd := &cobra.Command{
		Use:   "send <event-name>",
		Short: "Send an event to the dev server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)

			eventData, err := readJSONInput(data, dataFile)
			if err != nil {
				return err
			}
			if eventData == nil {
				eventData = map[string]any{}
			}

			ids, err := client.SendEvent(cmd.Context(), inngest.EventInput{
				Name: args[0],
				Data: eventData,
				TS:   time.Now().UnixMilli(),
			})
			if err != nil {
				return fmt.Errorf("sending event: %w", err)
			}

			return output.Print(map[string]any{
				"event_name": args[0],
				"event_ids":  ids,
			}, format)
		},
	}

	cmd.Flags().StringVar(&data, "data", "", "Event data as a JSON string")
	cmd.Flags().StringVar(&dataFile, "data-file", "", `Read event data from a file ("-" for stdin)`)

	return cmd
}

func newDevInvokeCmd() *cobra.Command {
	var (
		data     string
		dataFile string
	)

	cmd := &cobra.Command{
		Use:   "invoke <function-slug>",
		Short: "Invoke a function on the dev server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)

			payload, err := readJSONInput(data, dataFile)
			if err != nil {
				return err
			}
			if payload == nil {
				payload = map[string]any{}
			}

			runID, err := client.InvokeDevFunction(cmd.Context(), args[0], payload)
			if err != nil {
				return fmt.Errorf("invoking function: %w", err)
			}

			return output.Print(map[string]any{
				"function_slug": args[0],
				"run_id":        runID,
			}, format)
		},
	}

	cmd.Flags().StringVar(&data, "data", "", "Event payload as a JSON string")
	cmd.Flags().StringVar(&dataFile, "data-file", "", `Read event payload from a file ("-" for stdin)`)

	return cmd
}

func newDevEventsCmd() *cobra.Command {
	var (
		limit int
		name  string
		since string
	)

	cmd := &cobra.Command{
		Use:   "events",
		Short: "List recent events from the dev server",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)

			opts := inngest.ListEventsOptions{Name: name, Limit: limit}
			if since != "" {
				from, err := parseSince("since", since)
				if err != nil {
					return err
				}
				opts.ReceivedAfter = from
			}

			events, err := client.ListEvents(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}
			if format == output.FormatTable {
				return printEventsTable(events)
			}
			return output.Print(events, format)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of events to return")
	cmd.Flags().StringVar(&name, "name", "", "Filter by event name")
	cmd.Flags().StringVar(&since, "since", "", "Only events received within this duration (e.g. 1h)")

	return cmd
}
