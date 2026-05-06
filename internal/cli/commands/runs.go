package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
	cmd.AddCommand(newRunsCancelCmd())
	cmd.AddCommand(newRunsReplayCmd())
	cmd.AddCommand(newRunsWatchCmd())
	return cmd
}

func newRunsListCmd() *cobra.Command {
	var (
		limit    int
		status   string
		function string
		since    string
		until    string
		after    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent function runs",
		Long:  "List recent function runs with status, function name, and timing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			duration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since duration %q: %w", since, err)
			}
			fromTime := time.Now().Add(-duration)

			opts := inngest.ListRunsOptions{
				First: limit,
				After: after,
				From:  fromTime,
			}

			if status != "" {
				opts.Status = inngest.StatusToUpper(strings.Split(status, ","))
			}
			if function != "" {
				opts.FunctionIDs = []string{function}
			}
			if until != "" {
				d, err := time.ParseDuration(until)
				if err != nil {
					return fmt.Errorf("invalid --until duration %q: %w", until, err)
				}
				t := time.Now().Add(-d)
				opts.Until = &t
			}

			conn, err := client.ListRuns(ctx, opts)
			if err != nil {
				return fmt.Errorf("listing runs: %w", err)
			}

			if format == output.FormatTable {
				return printRunsTable(conn)
			}

			return output.Print(map[string]any{
				"runs":       runsFromEdges(conn.Edges),
				"totalCount": conn.TotalCount,
				"pageInfo":   conn.PageInfo,
			}, format)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of runs to return")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (comma-separated: RUNNING,COMPLETED,FAILED,CANCELLED,QUEUED)")
	cmd.Flags().StringVar(&function, "function", "", "Filter by function ID")
	cmd.Flags().StringVar(&since, "since", "24h", "Show runs since this duration ago (e.g. 1h, 30m, 24h)")
	cmd.Flags().StringVar(&until, "until", "", "Show runs until this duration ago (e.g. 1h)")
	cmd.Flags().StringVar(&after, "after", "", "Pagination cursor (from previous response)")

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

func printRunsTable(conn *inngest.RunsConnection) error {
	rows := make([]runRow, len(conn.Edges))
	for i, edge := range conn.Edges {
		run := edge.Node
		fnName := ""
		if run.Function != nil {
			fnName = run.Function.Name
		}
		started := ""
		if run.StartedAt != nil {
			started = run.StartedAt.Local().Format("15:04:05")
		} else if run.QueuedAt != nil {
			started = run.QueuedAt.Local().Format("15:04:05")
		}
		dur := ""
		if run.StartedAt != nil && run.EndedAt != nil {
			dur = run.EndedAt.Sub(*run.StartedAt).Round(time.Millisecond).String()
		} else if run.StartedAt != nil {
			dur = time.Since(*run.StartedAt).Round(time.Second).String() + "…"
		}
		rows[i] = runRow{
			ID:       run.ID,
			Status:   run.Status,
			Function: fnName,
			Event:    run.EventName,
			Started:  started,
			Duration: dur,
		}
	}
	return output.Print(rows, output.FormatTable)
}

func runsFromEdges(edges []inngest.RunEdge) []inngest.FunctionRun {
	runs := make([]inngest.FunctionRun, len(edges))
	for i, edge := range edges {
		runs[i] = edge.Node
	}
	return runs
}

func newRunsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <run-id>",
		Short: "Get full run details including trace",
		Long:  "Fetch run metadata, step-by-step trace with timing, and output.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			run, err := client.GetRun(ctx, args[0])
			if err != nil {
				return fmt.Errorf("getting run: %w", err)
			}

			if format == output.FormatText {
				return printRunDetail(run)
			}

			return output.Print(run, format)
		},
	}
}

func printRunDetail(run *inngest.FunctionRun) error {
	fmt.Printf("Run ID:      %s\n", run.ID)
	fmt.Printf("Status:      %s\n", run.Status)
	if run.Function != nil {
		fmt.Printf("Function:    %s (%s)\n", run.Function.Name, run.Function.Slug)
	}
	fmt.Printf("Event:       %s\n", run.EventName)

	if run.App != nil {
		fmt.Printf("App:         %s\n", run.App.Name)
		if run.App.SDKLanguage != "" {
			fmt.Printf("SDK:         %s/%s\n", run.App.SDKLanguage, run.App.SDKVersion)
		}
	}

	if run.QueuedAt != nil {
		fmt.Printf("Queued:      %s\n", run.QueuedAt.Local().Format(time.RFC3339))
	}
	if run.StartedAt != nil {
		fmt.Printf("Started:     %s\n", run.StartedAt.Local().Format(time.RFC3339))
	}
	if run.EndedAt != nil {
		fmt.Printf("Ended:       %s\n", run.EndedAt.Local().Format(time.RFC3339))
	}
	if run.StartedAt != nil && run.EndedAt != nil {
		fmt.Printf("Duration:    %s\n", run.EndedAt.Sub(*run.StartedAt).Round(time.Millisecond))
	}
	if run.IsBatch {
		fmt.Printf("Batch:       true\n")
	}
	if run.CronSchedule != "" {
		fmt.Printf("Cron:        %s\n", run.CronSchedule)
	}

	if run.Output != "" {
		fmt.Printf("\nOutput:\n  %s\n", run.Output)
	}

	if run.TraceID != "" {
		fmt.Printf("\nTrace ID:    %s\n", run.TraceID)
	}

	if run.Trace != nil {
		fmt.Printf("\nTrace:\n")
		printTraceSpan(run.Trace, "  ")
	}

	return nil
}

func printTraceSpan(span *inngest.RunTraceSpan, indent string) {
	label := span.Name
	if span.StepOp != "" {
		label = fmt.Sprintf("%s [%s]", span.Name, span.StepOp)
	}
	dur := fmt.Sprintf("%dms", span.Duration)
	fmt.Printf("%s%-40s %-12s %s\n", indent, label, span.Status, dur)

	for i := range span.Children {
		printTraceSpan(&span.Children[i], indent+"  ")
	}
}

func newRunsCancelCmd() *cobra.Command {
	var (
		force bool
		envID string
	)

	cmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a running function",
		Long:  "Cancel a currently running function execution. Requires an environment ID via --env-id flag or INNGEST_ENV_ID env var.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()
			runID := args[0]

			// Fall back to INNGEST_ENV_ID env var if --env-id not provided.
			if envID == "" {
				envID = os.Getenv("INNGEST_ENV_ID")
			}

			if !force {
				fmt.Fprintf(os.Stderr, "Cancel run %s? [y/N] ", runID)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			run, err := client.CancelRun(ctx, envID, runID)
			if err != nil {
				return fmt.Errorf("cancelling run: %w", err)
			}

			return output.Print(map[string]any{
				"id":     run.ID,
				"status": run.Status,
			}, format)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&envID, "env-id", "", "Inngest environment UUID (or set INNGEST_ENV_ID)")

	return cmd
}

func newRunsReplayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "replay <run-id>",
		Short: "Replay a function run",
		Long:  "Re-execute a function run. Returns the new run ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			newRunID, err := client.RerunRun(ctx, args[0])
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
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			opts := inngest.ListRunsOptions{
				First: 10,
				From:  time.Now(),
			}
			if status != "" {
				opts.Status = inngest.StatusToUpper(strings.Split(status, ","))
			}
			if function != "" {
				opts.FunctionIDs = []string{function}
			}

			var lastCursor string

			fmt.Fprintln(os.Stderr, "Watching for new runs… (Ctrl+C to stop)")

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					fmt.Fprintln(os.Stderr, "\nStopped.")
					return nil
				case <-ticker.C:
					queryOpts := opts
					if lastCursor != "" {
						queryOpts.After = lastCursor
					}

					conn, err := client.ListRuns(ctx, queryOpts)
					if err != nil {
						if ctx.Err() != nil {
							return nil
						}
						fmt.Fprintf(os.Stderr, "Error polling runs: %v\n", err)
						continue
					}

					for i := len(conn.Edges) - 1; i >= 0; i-- {
						edge := conn.Edges[i]
						run := edge.Node
						fnName := ""
						if run.Function != nil {
							fnName = run.Function.Name
						}
						started := ""
						if run.StartedAt != nil {
							started = run.StartedAt.Local().Format("15:04:05")
						} else if run.QueuedAt != nil {
							started = run.QueuedAt.Local().Format("15:04:05")
						}
						fmt.Printf("[%s] %-12s %-40s %-30s %s\n",
							started, run.Status, fnName, run.EventName, run.ID)
					}

					if len(conn.Edges) > 0 {
						lastCursor = conn.Edges[0].Cursor
					}
				}
			}
		},
	}

	cmd.Flags().StringVar(&function, "function", "", "Filter by function ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (comma-separated)")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Poll interval")

	return cmd
}
