package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
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
		Long:  "Commands for the local Inngest dev server at localhost:8288. No cloud auth required.",
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
	})
}

func newDevStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check if the local dev server is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			if !client.IsDevServerRunning(ctx) {
				return output.Print(map[string]any{
					"status":  "offline",
					"url":     state.DevServer,
					"message": "Dev server is not reachable. Start it with: npx inngest-cli@latest dev",
				}, format)
			}

			info, err := client.GetDevInfo(ctx)
			if err != nil {
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
			ctx := context.Background()

			query := `query {
  functions {
    id
    name
    slug
    config
    triggers {
      type
      value
      condition
    }
    app {
      name
      url
      sdkLanguage
      sdkVersion
      framework
    }
  }
}`

			var result struct {
				Functions []inngest.Function `json:"functions"`
			}
			if err := client.ExecuteGraphQL(ctx, "ListFunctions", query, nil, &result); err != nil {
				return fmt.Errorf("querying functions: %w", err)
			}

			return output.Print(result.Functions, format)
		},
	}
}

func newDevRunsCmd() *cobra.Command {
	var limit int
	var status string
	var since string
	var function string

	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List recent function runs from the dev server",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			duration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since duration %q: %w", since, err)
			}
			fromTime := time.Now().Add(-duration)

			query := `query DevRuns($first: Int!, $filter: RunsFilterV2!) {
  runs(first: $first, orderBy: [{field: QUEUED_AT, direction: DESC}], filter: $filter) {
    edges {
      node {
        id
        status
        queuedAt
        startedAt
        endedAt
        eventName
        function {
          name
          slug
        }
      }
    }
    totalCount
  }
}`

			filter := map[string]any{
				"from": fromTime.Format(time.RFC3339),
			}
			if status != "" {
				filter["status"] = []string{strings.ToUpper(status)}
			}
			if function != "" {
				filter["functionSlug"] = function
			}

			variables := map[string]any{
				"first":  limit,
				"filter": filter,
			}

			var result struct {
				Runs inngest.RunsConnection `json:"runs"`
			}
			if err := client.ExecuteGraphQL(ctx, "DevRuns", query, variables, &result); err != nil {
				return fmt.Errorf("querying runs: %w", err)
			}

			runs := make([]inngest.FunctionRun, len(result.Runs.Edges))
			for i, edge := range result.Runs.Edges {
				runs[i] = edge.Node
			}

			return output.Print(map[string]any{
				"runs":       runs,
				"totalCount": result.Runs.TotalCount,
			}, format)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of runs to return")
	cmd.Flags().StringVar(&status, "status", "", "Filter by run status (e.g. COMPLETED, FAILED)")
	cmd.Flags().StringVar(&since, "since", "1h", "Show runs since this duration ago (e.g. 1h, 30m, 24h)")
	cmd.Flags().StringVar(&function, "function", "", "Filter by function slug")

	return cmd
}

func newDevSendCmd() *cobra.Command {
	var data string

	cmd := &cobra.Command{
		Use:   "send <event-name>",
		Short: "Send an event to the dev server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			eventName := args[0]

			var eventData any
			switch {
			case data != "":
				if err := json.Unmarshal([]byte(data), &eventData); err != nil {
					return fmt.Errorf("invalid --data JSON: %w", err)
				}
			case !isInteractive():
				raw, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if len(raw) > 0 {
					if err := json.Unmarshal(raw, &eventData); err != nil {
						return fmt.Errorf("invalid stdin JSON: %w", err)
					}
				}
			}

			if eventData == nil {
				eventData = map[string]any{}
			}

			event := map[string]any{
				"name": eventName,
				"data": eventData,
				"ts":   time.Now().UnixMilli(),
			}

			ids, err := client.SendDevEvent(ctx, event)
			if err != nil {
				return fmt.Errorf("sending event: %w", err)
			}

			return output.Print(map[string]any{
				"event_name": eventName,
				"event_ids":  ids,
			}, format)
		},
	}

	cmd.Flags().StringVar(&data, "data", "", "Event data as a JSON string")

	return cmd
}

func newDevInvokeCmd() *cobra.Command {
	var data string

	cmd := &cobra.Command{
		Use:   "invoke <function-slug>",
		Short: "Invoke a function on the dev server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			slug := args[0]

			var payload any
			if data != "" {
				if err := json.Unmarshal([]byte(data), &payload); err != nil {
					return fmt.Errorf("invalid --data JSON: %w", err)
				}
			}
			if payload == nil {
				payload = map[string]any{}
			}

			id, err := client.InvokeDevFunction(ctx, slug, payload)
			if err != nil {
				return fmt.Errorf("invoking function: %w", err)
			}

			return output.Print(map[string]any{
				"function_slug": slug,
				"event_id":      id,
			}, format)
		},
	}

	cmd.Flags().StringVar(&data, "data", "", "Event payload as a JSON string")

	return cmd
}

func newDevEventsCmd() *cobra.Command {
	var limit int
	var name string

	cmd := &cobra.Command{
		Use:   "events",
		Short: "List recent events from the dev server",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newDevClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			query := `query {
  events(query: {workspaceId: "local", lastEventId: null}) {
    id
    name
    createdAt
    status
    totalRuns
    raw
  }
}`

			var result struct {
				Events []inngest.Event `json:"events"`
			}
			if err := client.ExecuteGraphQL(ctx, "ListDevEvents", query, nil, &result); err != nil {
				return fmt.Errorf("querying events: %w", err)
			}

			events := result.Events

			// Apply client-side filters.
			if name != "" {
				filtered := make([]inngest.Event, 0)
				for _, e := range events {
					if e.Name == name {
						filtered = append(filtered, e)
					}
				}
				events = filtered
			}

			if limit > 0 && len(events) > limit {
				events = events[:limit]
			}

			return output.Print(events, format)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of events to return")
	cmd.Flags().StringVar(&name, "name", "", "Filter by event name")

	return cmd
}
