package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
	"github.com/spf13/cobra"
)

// maxPages is the upper bound on pagination loops to prevent OOM/hangs on
// high-volume accounts. At 100 runs per page this caps at 5 000 runs.
const maxPages = 50

// NewHealthCmd returns the top-level "health" command.
func NewHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Run connectivity and configuration health checks",
		Long:  "Check signing key, event key, API reachability, and dev server status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := state.Config
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			type checkResult struct {
				Check  string `json:"check"`
				Status string `json:"status"`
				Detail string `json:"detail,omitempty"`
			}

			var results []checkResult
			allPassed := true

			// 1. Signing key configured
			if sk := cfg.GetSigningKey(); sk != "" {
				results = append(results, checkResult{
					Check:  "signing_key",
					Status: "ok",
					Detail: "configured",
				})
			} else {
				results = append(results, checkResult{
					Check:  "signing_key",
					Status: "fail",
					Detail: "not configured — set INNGEST_SIGNING_KEY or run inngest auth login",
				})
				allPassed = false
			}

			// 2. Event key configured
			if ek := cfg.GetEventKey(); ek != "" {
				results = append(results, checkResult{
					Check:  "event_key",
					Status: "ok",
					Detail: "configured",
				})
			} else {
				results = append(results, checkResult{
					Check:  "event_key",
					Status: "warn",
					Detail: "not configured — needed for sending events",
				})
			}

			// 3. API reachability (simple connectivity check)
			var probe interface{}
			err := client.ExecuteGraphQL(ctx, "HealthCheck", `query HealthCheck { __typename }`, nil, &probe)
			if err != nil {
				results = append(results, checkResult{
					Check:  "api",
					Status: "fail",
					Detail: err.Error(),
				})
				allPassed = false
			} else {
				results = append(results, checkResult{
					Check:  "api",
					Status: "ok",
					Detail: "reachable",
				})
			}

			// 4. Dev server reachability (if --dev or auto-detect)
			if state.DevMode || client.IsDevServerRunning(ctx) {
				if client.IsDevServerRunning(ctx) {
					results = append(results, checkResult{
						Check:  "dev_server",
						Status: "ok",
						Detail: "reachable at " + state.DevServer,
					})
				} else {
					results = append(results, checkResult{
						Check:  "dev_server",
						Status: "fail",
						Detail: "not reachable at " + state.DevServer,
					})
					allPassed = false
				}
			} else {
				results = append(results, checkResult{
					Check:  "dev_server",
					Status: "skip",
					Detail: "not in dev mode and server not detected",
				})
			}

			if format == output.FormatText || format == output.FormatTable {
				for _, r := range results {
					icon := "✓"
					switch r.Status {
					case "fail":
						icon = "✗"
					case "warn":
						icon = "!"
					case "skip":
						icon = "-"
					}
					fmt.Printf("  %s  %-15s %s\n", icon, r.Check, r.Detail)
				}
				if !allPassed {
					fmt.Println("\nSome checks failed.")
				} else {
					fmt.Println("\nAll checks passed.")
				}
			} else {
				_ = output.Print(map[string]interface{}{
					"checks":  results,
					"healthy": allPassed,
				}, format)
			}

			if !allPassed {
				cmd.SilenceErrors = true
				return fmt.Errorf("health check failed")
			}
			return nil
		},
	}
}

// NewMetricsCmd returns the top-level "metrics" command.
func NewMetricsCmd() *cobra.Command {
	var (
		since    string
		function string
	)

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show run metrics and success/failure rates",
		Long:  "Query recent runs and compute total counts, status breakdown, success/failure rates, and duration percentiles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			duration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since duration %q: %w", since, err)
			}
			fromTime := time.Now().Add(-duration)

			// Fetch all runs in the period by paginating.
			var allRuns []inngest.FunctionRun
			var cursor string
			truncated := false
			page := 0
			for {
				opts := inngest.ListRunsOptions{
					First: 100,
					After: cursor,
					From:  fromTime,
				}
				if function != "" {
					opts.FunctionIDs = []string{function}
				}

				conn, err := client.ListRuns(ctx, opts)
				if err != nil {
					return fmt.Errorf("querying runs: %w", err)
				}

				for _, edge := range conn.Edges {
					allRuns = append(allRuns, edge.Node)
				}

				if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" {
					break
				}
				cursor = conn.PageInfo.EndCursor

				page++
				if page >= maxPages {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: pagination limit reached (%d pages). Results may be incomplete.\n", maxPages)
					truncated = true
					break
				}
			}

			// Compute metrics.
			total := len(allRuns)
			statusCounts := map[string]int{}
			var durations []time.Duration

			for _, run := range allRuns {
				statusCounts[run.Status]++
				if run.StartedAt != nil && run.EndedAt != nil {
					durations = append(durations, run.EndedAt.Sub(*run.StartedAt))
				}
			}

			completed := statusCounts["COMPLETED"]
			failed := statusCounts["FAILED"]
			running := statusCounts["RUNNING"]
			cancelled := statusCounts["CANCELLED"]

			successRate := 0.0
			failureRate := 0.0
			if total > 0 {
				successRate = float64(completed) / float64(total) * 100
				failureRate = float64(failed) / float64(total) * 100
			}

			// Compute duration percentiles.
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

			percentile := func(p float64) time.Duration {
				if len(durations) == 0 {
					return 0
				}
				idx := int(float64(len(durations)-1) * p)
				return durations[idx]
			}

			p50 := percentile(0.5)
			p90 := percentile(0.9)
			p99 := percentile(0.99)

			result := map[string]interface{}{
				"period":      since,
				"total":       total,
				"completed":   completed,
				"failed":      failed,
				"running":     running,
				"cancelled":   cancelled,
				"successRate": fmt.Sprintf("%.1f%%", successRate),
				"failureRate": fmt.Sprintf("%.1f%%", failureRate),
			}

			if len(durations) > 0 {
				result["durationSamples"] = len(durations)
				result["p50"] = p50.Round(time.Millisecond).String()
				result["p90"] = p90.Round(time.Millisecond).String()
				result["p99"] = p99.Round(time.Millisecond).String()
			}

			if truncated {
				result["truncated"] = true
				result["truncatedAt"] = total
			}

			if format == output.FormatText || format == output.FormatTable {
				fmt.Printf("Metrics (last %s):\n\n", since)
				fmt.Printf("  Total runs:    %d\n", total)
				fmt.Printf("  Completed:     %d\n", completed)
				fmt.Printf("  Failed:        %d\n", failed)
				fmt.Printf("  Running:       %d\n", running)
				fmt.Printf("  Cancelled:     %d\n", cancelled)
				fmt.Printf("  Success rate:  %.1f%%\n", successRate)
				fmt.Printf("  Failure rate:  %.1f%%\n", failureRate)
				if len(durations) > 0 {
					fmt.Printf("\n  Duration (%d samples):\n", len(durations))
					fmt.Printf("    P50:  %s\n", p50.Round(time.Millisecond))
					fmt.Printf("    P90:  %s\n", p90.Round(time.Millisecond))
					fmt.Printf("    P99:  %s\n", p99.Round(time.Millisecond))
				}
				if truncated {
					fmt.Printf("\n  Note: results truncated at %d runs. Use --since with a shorter duration for complete metrics.\n", total)
				}
				return nil
			}

			return output.Print(result, format)
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Time period to query (e.g. 1h, 24h, 7d)")
	cmd.Flags().StringVar(&function, "function", "", "Filter by function ID")

	return cmd
}

// NewBacklogCmd returns the top-level "backlog" command.
func NewBacklogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backlog",
		Short: "Show currently queued and running runs per function",
		Long:  "Query runs with RUNNING or QUEUED status and group by function name.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			// Fetch running and queued runs.
			var allRuns []inngest.FunctionRun
			truncated := false
			for _, status := range []string{"RUNNING", "QUEUED"} {
				var cursor string
				page := 0
				for {
					opts := inngest.ListRunsOptions{
						First:  100,
						After:  cursor,
						Status: []string{status},
						From:   time.Now().Add(-24 * time.Hour),
					}

					conn, err := client.ListRuns(ctx, opts)
					if err != nil {
						return fmt.Errorf("querying %s runs: %w", strings.ToLower(status), err)
					}

					for _, edge := range conn.Edges {
						allRuns = append(allRuns, edge.Node)
					}

					if !conn.PageInfo.HasNextPage || conn.PageInfo.EndCursor == "" {
						break
					}
					cursor = conn.PageInfo.EndCursor

					page++
					if page >= maxPages {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: pagination limit reached (%d pages) for %s runs. Results may be incomplete.\n", maxPages, strings.ToLower(status))
						truncated = true
						break
					}
				}
			}

			// Group by function name.
			type backlogEntry struct {
				Function string `json:"function"`
				Running  int    `json:"running"`
				Queued   int    `json:"queued"`
				Total    int    `json:"total"`
			}

			counts := map[string]*backlogEntry{}
			for _, run := range allRuns {
				fnName := "(unknown)"
				if run.Function != nil {
					fnName = run.Function.Name
				}
				entry, ok := counts[fnName]
				if !ok {
					entry = &backlogEntry{Function: fnName}
					counts[fnName] = entry
				}
				switch run.Status {
				case "RUNNING":
					entry.Running++
				case "QUEUED":
					entry.Queued++
				}
				entry.Total++
			}

			// Sort by total descending.
			entries := make([]backlogEntry, 0, len(counts))
			for _, e := range counts {
				entries = append(entries, *e)
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Total > entries[j].Total
			})

			if len(entries) == 0 {
				if format == output.FormatText || format == output.FormatTable {
					fmt.Println("No queued or running functions.")
					return nil
				}
				return output.Print([]backlogEntry{}, format)
			}

			if truncated {
				if format == output.FormatText || format == output.FormatTable {
					fmt.Printf("\nNote: results truncated at %d runs. Use --since with a shorter duration for complete metrics.\n", len(allRuns))
				}
			}

			if format == output.FormatJSON {
				result := map[string]interface{}{
					"entries": entries,
				}
				if truncated {
					result["truncated"] = true
					result["truncatedAt"] = len(allRuns)
				}
				return output.Print(result, format)
			}

			if format == output.FormatTable {
				return output.Print(entries, output.FormatTable)
			}

			return output.Print(entries, format)
		},
	}
}
