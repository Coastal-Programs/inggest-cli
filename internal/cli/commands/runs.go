package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// NewRunsCmd returns the "runs" command group.
func NewRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List, inspect, cancel, and replay function runs",
		Long:  "Query and manage function runs in Inngest Cloud or dev server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newRunsListCmd())
	cmd.AddCommand(newRunsGetCmd())
	cmd.AddCommand(newRunsTraceCmd())
	cmd.AddCommand(newRunsCancelCmd())
	cmd.AddCommand(newRunsReplayCmd())
	cmd.AddCommand(newRunsWatchCmd())
	return cmd
}

// splitCSV splits a comma-separated flag value, dropping empty items.
func splitCSV(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseSince turns a duration like "24h" or an RFC3339 timestamp into a time.
func parseSince(flag, value string) (time.Time, error) {
	if d, err := time.ParseDuration(value); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --%s %q: use a duration (1h, 30m) or an RFC3339 timestamp", flag, value)
}

func newRunsListCmd() *cobra.Command {
	var (
		limit     int
		status    string
		function  string
		app       string
		since     string
		until     string
		after     string
		order     string
		timeField string
		withOut   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent function runs",
		Long: `List function runs with status, function name, and timing.

Filters are applied server-side. Paginate with --after using page.cursor from the previous response.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			if limit < 1 || limit > inngest.MaxRunsPageSize {
				return fmt.Errorf("--limit must be between 1 and %d", inngest.MaxRunsPageSize)
			}

			opts := inngest.ListRunsOptions{
				First:         limit,
				After:         after,
				Status:        splitCSV(status),
				FunctionIDs:   splitCSV(function),
				AppIDs:        splitCSV(app),
				Order:         order,
				TimeField:     timeField,
				IncludeOutput: withOut,
			}
			if since != "" {
				from, err := parseSince("since", since)
				if err != nil {
					return err
				}
				opts.From = from
			}
			if until != "" {
				t, err := parseSince("until", until)
				if err != nil {
					return err
				}
				opts.Until = &t
			}

			page, err := client.ListRuns(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("listing runs: %w", err)
			}

			if format == output.FormatTable {
				return printRunsTable(page.Runs)
			}
			return output.Print(page, format)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Runs per page (1–100)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (comma-separated: QUEUED,RUNNING,COMPLETED,FAILED,CANCELLED)")
	cmd.Flags().StringVar(&function, "function", "", "Filter by function ID (comma-separated)")
	cmd.Flags().StringVar(&app, "app", "", "Filter by app ID (comma-separated)")
	cmd.Flags().StringVar(&since, "since", "24h", "Start of time range: duration ago (1h, 30m) or RFC3339 timestamp")
	cmd.Flags().StringVar(&until, "until", "", "End of time range: duration ago or RFC3339 timestamp")
	cmd.Flags().StringVar(&after, "after", "", "Pagination cursor (page.cursor from the previous response)")
	cmd.Flags().StringVar(&order, "order", "DESC", "Sort direction: ASC or DESC")
	cmd.Flags().StringVar(&timeField, "time-field", "queuedAt", "Timestamp used for filtering and ordering: queuedAt, startedAt, endedAt")
	cmd.Flags().BoolVar(&withOut, "output-data", false, "Include each run's output (larger responses)")

	return cmd
}

// runRow is used for table output.
type runRow struct {
	ID       string
	Status   string
	Function string
	Event    string
	Started  string
	Duration string
}

func runDuration(run inngest.FunctionRun) string {
	switch {
	case run.DurationMs > 0:
		return (time.Duration(run.DurationMs) * time.Millisecond).String()
	case run.StartedAt != nil && run.EndedAt != nil:
		return run.EndedAt.Sub(*run.StartedAt).Round(time.Millisecond).String()
	case run.StartedAt != nil:
		return time.Since(*run.StartedAt).Round(time.Second).String() + "…"
	}
	return ""
}

func runStartedLabel(run inngest.FunctionRun) string {
	switch {
	case run.StartedAt != nil:
		return run.StartedAt.Local().Format("15:04:05")
	case run.QueuedAt != nil:
		return run.QueuedAt.Local().Format("15:04:05")
	}
	return ""
}

func runFunctionName(run inngest.FunctionRun) string {
	if run.Function != nil && run.Function.Name != "" {
		return run.Function.Name
	}
	return run.FunctionID
}

func printRunsTable(runs []inngest.FunctionRun) error {
	rows := make([]runRow, len(runs))
	for i, run := range runs {
		rows[i] = runRow{
			ID:       run.ID,
			Status:   run.Status,
			Function: runFunctionName(run),
			Event:    run.EventName,
			Started:  runStartedLabel(run),
			Duration: runDuration(run),
		}
	}
	return output.Print(rows, output.FormatTable)
}

func newRunsGetCmd() *cobra.Command {
	var (
		wait    bool
		noTrace bool
	)

	cmd := &cobra.Command{
		Use:   "get <run-id>",
		Short: "Get full run details including trace",
		Long:  "Fetch run metadata, output, and the step-by-step trace with timing. Use --wait to block until the run finishes.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := cmd.Context()

			run, err := client.GetRun(ctx, args[0])
			if err != nil {
				return fmt.Errorf("getting run: %w", err)
			}
			if wait {
				run, err = waitForRun(ctx, client, run)
				if err != nil {
					return err
				}
			}

			if !noTrace && run.Trace == nil {
				trace, err := client.GetRunTrace(ctx, run.ID)
				switch {
				case err == nil:
					run.Trace = trace
				case inngest.IsNotFound(err):
					// Queued runs have no trace yet.
				default:
					return fmt.Errorf("getting run trace: %w", err)
				}
			}

			if format == output.FormatText {
				return printRunDetail(run)
			}
			return output.Print(run, format)
		},
	}

	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until the run reaches a terminal status")
	cmd.Flags().BoolVar(&noTrace, "no-trace", false, "Skip fetching the trace")

	return cmd
}

// waitForRun polls until the run is COMPLETED, FAILED or CANCELLED.
func waitForRun(ctx context.Context, client *inngest.Client, run *inngest.FunctionRun) (*inngest.FunctionRun, error) {
	const pollInterval = 2 * time.Second
	for !inngest.IsTerminalRunStatus(run.Status) {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for run %s: %w", run.ID, ctx.Err())
		case <-time.After(pollInterval):
		}
		next, err := client.GetRun(ctx, run.ID)
		if err != nil {
			return nil, fmt.Errorf("waiting for run: %w", err)
		}
		run = next
	}
	return run, nil
}

func newRunsTraceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trace <run-id>",
		Short: "Show the step-by-step trace of a run",
		Long:  "Fetch the trace tree for a run: each step with its operation, status, timing, and (in JSON) input/output.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			trace, err := client.GetRunTrace(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("getting run trace: %w", err)
			}
			if trace == nil {
				return fmt.Errorf("run %s has no trace yet", args[0])
			}

			if format == output.FormatJSON {
				return output.Print(trace, format)
			}
			printTraceSpan(trace, "")
			return nil
		},
	}
}

// detailField is one "Label: value" line of a text detail view; empty values are skipped.
type detailField struct {
	label string
	value string
}

func printDetailFields(fields []detailField) {
	for _, f := range fields {
		if f.value != "" {
			fmt.Printf("%-12s %s\n", f.label+":", f.value)
		}
	}
}

func formatLocalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Local().Format(time.RFC3339)
}

func printRunDetail(run *inngest.FunctionRun) error {
	fields := []detailField{
		{"Run ID", run.ID},
		{"Status", run.Status},
	}
	if run.Function != nil {
		fields = append(fields, detailField{"Function", fmt.Sprintf("%s (%s)", run.Function.Name, firstNonEmpty(run.Function.Slug, run.Function.ID))})
	}
	fields = append(fields,
		detailField{"Event", run.EventName},
		detailField{"Event IDs", strings.Join(run.EventIDs, ", ")},
	)
	if run.App != nil && run.App.Name != "" {
		fields = append(fields, detailField{"App", run.App.Name})
		if run.App.SDKLanguage != "" {
			fields = append(fields, detailField{"SDK", run.App.SDKLanguage + "/" + run.App.SDKVersion})
		}
	} else {
		fields = append(fields, detailField{"App ID", run.AppID})
	}
	batch := ""
	if run.IsBatch {
		batch = "yes"
	}
	fields = append(fields,
		detailField{"Queued", formatLocalTime(run.QueuedAt)},
		detailField{"Started", formatLocalTime(run.StartedAt)},
		detailField{"Ended", formatLocalTime(run.EndedAt)},
		detailField{"Duration", runDuration(*run)},
		detailField{"Batch", batch},
		detailField{"Cron", run.CronSchedule},
	)
	printDetailFields(fields)

	if len(run.Output) > 0 {
		fmt.Printf("\nOutput:\n  %s\n", formatJSONInline(run.Output))
	}
	if run.Trace != nil {
		fmt.Printf("\nTrace:\n")
		printTraceSpan(run.Trace, "  ")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// formatJSONInline compacts JSON for single-line display; non-JSON is returned as-is.
func formatJSONInline(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func printTraceSpan(span *inngest.RunTraceSpan, indent string) {
	label := span.Name
	if span.StepOp != "" {
		label = fmt.Sprintf("%s [%s]", span.Name, span.StepOp)
	}
	dur := ""
	switch {
	case span.DurationMs > 0:
		dur = fmt.Sprintf("%dms", span.DurationMs)
	case span.StartedAt != nil && span.EndedAt != nil:
		dur = span.EndedAt.Sub(*span.StartedAt).Round(time.Millisecond).String()
	}
	fmt.Printf("%s%-40s %-12s %s\n", indent, label, span.Status, dur)

	for i := range span.Children {
		printTraceSpan(&span.Children[i], indent+"  ")
	}
}

func newRunsCancelCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a running function",
		Long:  "Cancel an in-progress function run. No further steps execute after cancellation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			runID := args[0]

			if !force {
				fmt.Fprintf(os.Stderr, "Cancel run %s? [y/N] ", runID)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			id, err := client.CancelRun(cmd.Context(), runID)
			if err != nil {
				return fmt.Errorf("cancelling run: %w", err)
			}

			return output.Print(map[string]any{
				"id":     id,
				"status": "CANCELLED",
			}, format)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func newRunsReplayCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "replay <run-id>",
		Aliases: []string{"rerun"},
		Short:   "Replay a function run",
		Long:    "Re-execute a function run with its original trigger. Returns the new run ID.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			newRunID, err := client.RerunRun(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("replaying run: %w", err)
			}

			return output.Print(map[string]any{
				"originalRunID": args[0],
				"newRunID":      newRunID,
			}, format)
		},
	}
}

func newRunsWatchCmd() *cobra.Command {
	var (
		function string
		status   string
		interval time.Duration
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for new runs in real-time",
		Long:  "Poll for new function runs and display them as they appear. Runs until Ctrl+C.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			ctx := cmd.Context()

			opts := inngest.ListRunsOptions{
				First:       inngest.MaxRunsPageSize,
				Order:       "ASC",
				From:        time.Now(),
				Status:      splitCSV(status),
				FunctionIDs: splitCSV(function),
			}

			fmt.Fprintln(os.Stderr, "Watching for new runs… (Ctrl+C to stop)")

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			// Each poll asks for runs queued at/after the newest run seen so far and
			// prints each (run, status) pair once, so status transitions show up too.
			seen := map[string]struct{}{}

			for {
				select {
				case <-ctx.Done():
					fmt.Fprintln(os.Stderr, "\nStopped.")
					return nil
				case <-ticker.C:
					page, err := client.ListRuns(ctx, opts)
					if err != nil {
						if ctx.Err() != nil {
							return nil
						}
						fmt.Fprintf(os.Stderr, "Error polling runs: %v\n", err)
						continue
					}

					current := make(map[string]struct{}, len(page.Runs))
					for _, run := range page.Runs {
						key := run.ID + "|" + run.Status
						current[key] = struct{}{}
						if _, ok := seen[key]; ok {
							continue
						}
						fmt.Printf("[%s] %-12s %-40s %-30s %s\n",
							runStartedLabel(run), run.Status, runFunctionName(run), run.EventName, run.ID)
						if run.QueuedAt != nil && run.QueuedAt.After(opts.From) {
							opts.From = *run.QueuedAt
						}
					}
					seen = current
				}
			}
		},
	}

	cmd.Flags().StringVar(&function, "function", "", "Filter by function ID (comma-separated)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (comma-separated)")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Poll interval")

	return cmd
}
