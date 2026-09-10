package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// NewEventsCmd returns the "events" command group for cloud events.
func NewEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Send and query events",
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
		SigningKey:         cfg.GetAPICredential(),
		SigningKeyFallback: cfg.GetSigningKeyFallback(),
		EventKey:           cfg.GetEventKey(),
		Env:                state.Env,
		APIBaseURL:         state.APIBaseURL,
		DevServerURL:       state.DevServer,
		DevMode:            state.DevMode,
		UserAgent:          "inngest-cli/" + state.AppVersion,
		Timeout:            state.Timeout,
	})
}

// readJSONInput parses JSON from --data, a file (--data-file, "-" for stdin),
// or piped stdin. Returns nil when nothing was provided.
func readJSONInput(flagData, flagFile string) (any, error) {
	var raw []byte
	switch {
	case flagData != "":
		raw = []byte(flagData)
	case flagFile == "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		raw = b
	case flagFile != "":
		b, err := os.ReadFile(flagFile)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", flagFile, err)
		}
		raw = b
	case !isInteractive():
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		raw = b
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %w", err)
	}
	return v, nil
}

func newEventsSendCmd() *cobra.Command {
	var (
		data     string
		dataFile string
		eventID  string
	)

	cmd := &cobra.Command{
		Use:   "send <event-name>",
		Short: "Send an event",
		Long: `Send an event to Inngest Cloud (or the dev server with --dev).

Event data comes from --data, --data-file (use "-" for stdin), or piped stdin.
With an event key configured the Event API is used; otherwise the event is sent
through the REST API with your signing/API key (intended for testing and debugging).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
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
				ID:   eventID,
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

	cmd.Flags().StringVarP(&data, "data", "d", "", "Event data as a JSON string")
	cmd.Flags().StringVar(&dataFile, "data-file", "", `Read event data from a file ("-" for stdin)`)
	cmd.Flags().StringVar(&eventID, "id", "", "Event ID for idempotent delivery")

	return cmd
}

func newEventsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <event-id>",
		Short: "Get an event and the runs it triggered",
		Long:  "Fetch an event by its internal ID together with the function runs it triggered.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := cmd.Context()

			event, err := client.GetEvent(ctx, args[0])
			if err != nil {
				return fmt.Errorf("getting event: %w", err)
			}
			runs, err := client.GetEventRuns(ctx, args[0])
			if err != nil {
				return fmt.Errorf("getting event runs: %w", err)
			}

			return output.Print(map[string]any{
				"event": event,
				"runs":  runs,
			}, format)
		},
	}
}

func newEventsListCmd() *cobra.Command {
	var (
		name   string
		limit  int
		since  string
		cursor string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent events",
		Long:  "List recent events, newest first. Paginate by passing the last event's internal_id as --after.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			opts := inngest.ListEventsOptions{Name: name, Limit: limit, Cursor: cursor}
			if since != "" {
				d, err := time.ParseDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since duration %q: %w", since, err)
				}
				opts.ReceivedAfter = time.Now().Add(-d)
			}

			events, err := client.ListEvents(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("listing events: %w", err)
			}

			if format == output.FormatTable {
				return printEventsTable(events)
			}
			return output.Print(events, format)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Filter by event name")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of events to return")
	cmd.Flags().StringVar(&since, "since", "", "Only events received within this duration (e.g. 1h, 30m)")
	cmd.Flags().StringVar(&cursor, "after", "", "Pagination cursor: internal_id of the last event from the previous page")

	return cmd
}

type eventRow struct {
	InternalID string
	Name       string
	Received   string
}

func printEventsTable(events []inngest.Event) error {
	rows := make([]eventRow, len(events))
	for i, e := range events {
		received := ""
		if e.ReceivedAt != nil {
			received = e.ReceivedAt.Local().Format("2006-01-02 15:04:05")
		}
		rows[i] = eventRow{InternalID: e.InternalID, Name: e.Name, Received: received}
	}
	return output.Print(rows, output.FormatTable)
}

func newEventsTypesCmd() *cobra.Command {
	var withSchema bool

	cmd := &cobra.Command{
		Use:   "types",
		Short: "List event types seen in this environment",
		Long:  "List the event types Inngest has observed, optionally with the inferred JSON schema of their data.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			schemas, err := client.ListEventSchemas(cmd.Context())
			if err != nil {
				return fmt.Errorf("listing event types: %w", err)
			}
			if withSchema {
				return output.Print(schemas, format)
			}

			names := make([]string, len(schemas))
			for i, s := range schemas {
				names[i] = s.Name
			}
			return output.Print(names, format)
		},
	}

	cmd.Flags().BoolVar(&withSchema, "schema", false, "Include the inferred data schema of each event type")

	return cmd
}
