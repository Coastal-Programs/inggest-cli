package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewEventsCmd returns the "events" command group for cloud events.
func NewEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Send and query events in Inngest Cloud",
		Long:  "Send events and query event history from Inngest Cloud (or dev server with --dev).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newEventsSendCmd())
	cmd.AddCommand(newEventsGetCmd())
	cmd.AddCommand(newEventsListCmd())
	cmd.AddCommand(newEventsTypesCmd())
	return cmd
}

func newCloudClient() *inngest.Client {
	cfg := state.Config
	return inngest.NewClient(inngest.ClientOptions{
		SigningKey:         cfg.GetSigningKey(),
		SigningKeyFallback: cfg.GetSigningKeyFallback(),
		EventKey:           cfg.GetEventKey(),
		Env:                state.Env,
		APIBaseURL:         state.APIBaseURL,
		DevServerURL:       state.DevServer,
		DevMode:            state.DevMode,
		UserAgent:          "inngest-cli/" + state.AppVersion,
	})
}

func newEventsSendCmd() *cobra.Command {
	var data string
	var async bool

	cmd := &cobra.Command{
		Use:   "send <event-name>",
		Short: "Send an event to Inngest Cloud",
		Long:  "Send an event to Inngest Cloud (or dev server with --dev). Reads data from --data flag or stdin.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			eventName := args[0]

			// Require event key unless in dev mode.
			cfg := state.Config
			if cfg.GetEventKey() == "" && !state.DevMode {
				return fmt.Errorf("event key required: use 'inngest auth login --event-key' or set INNGEST_EVENT_KEY")
			}

			// Parse event data from --data flag or stdin.
			var eventData interface{}
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
				eventData = map[string]interface{}{}
			}

			event := map[string]interface{}{
				"name": eventName,
				"data": eventData,
				"ts":   time.Now().UnixMilli(),
			}

			ids, err := client.SendEvent(ctx, event)
			if err != nil {
				return fmt.Errorf("sending event: %w", err)
			}

			return output.Print(map[string]interface{}{
				"event_name": eventName,
				"event_ids":  ids,
				"async":      async,
			}, format)
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Event data as a JSON string")
	cmd.Flags().BoolVar(&async, "async", true, "Don't wait for runs to complete")

	return cmd
}

func newEventsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <event-id>",
		Short: "Get event details and triggered runs",
		Long:  "Fetch event details and the runs it triggered. Uses GraphQL for full details, falls back to REST for runs.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			eventID := args[0]

			// Try GraphQL first for full event details.
			event, err := client.GetEvent(ctx, eventID)
			if err == nil {
				return output.Print(event, format)
			}

			// Fall back to REST for just the runs.
			runs, restErr := client.GetEventRuns(ctx, eventID)
			if restErr != nil {
				return fmt.Errorf("getting event: graphql: %v, rest: %w", err, restErr)
			}

			return output.Print(map[string]interface{}{
				"event_id": eventID,
				"runs":     runs,
			}, format)
		},
	}
}

func newEventsListCmd() *cobra.Command {
	var limit int
	var name string
	var since string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent events from Inngest Cloud",
		Long:  "Query recent events via GraphQL. Requires signing key auth.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			duration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since duration %q: %w", since, err)
			}
			fromTime := time.Now().Add(-duration)

			conn, err := client.ListEvents(ctx, inngest.ListEventsOptions{
				First: limit,
				Name:  name,
				Since: fromTime,
			})
			if err != nil {
				return fmt.Errorf("listing events: %w", err)
			}

			events := make([]inngest.Event, len(conn.Edges))
			for i, edge := range conn.Edges {
				events[i] = edge.Node
			}

			return output.Print(map[string]interface{}{
				"events":     events,
				"totalCount": conn.TotalCount,
			}, format)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of events to return")
	cmd.Flags().StringVar(&name, "name", "", "Filter by event name")
	cmd.Flags().StringVar(&since, "since", "24h", "Show events since this duration ago (e.g. 1h, 30m, 24h)")

	return cmd
}

func newEventsTypesCmd() *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "types",
		Short: "List unique event names seen recently",
		Long:  "Query recent events and extract distinct event names.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			duration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since duration %q: %w", since, err)
			}
			fromTime := time.Now().Add(-duration)

			conn, err := client.ListEvents(ctx, inngest.ListEventsOptions{
				First: 100,
				Since: fromTime,
			})
			if err != nil {
				return fmt.Errorf("listing events: %w", err)
			}

			seen := map[string]bool{}
			var names []string
			for _, edge := range conn.Edges {
				if !seen[edge.Node.Name] {
					seen[edge.Node.Name] = true
					names = append(names, edge.Node.Name)
				}
			}

			return output.Print(names, format)
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Look back this duration for event types (e.g. 1h, 24h, 72h)")

	return cmd
}
