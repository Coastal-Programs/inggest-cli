package commands

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// maxPages is the upper bound on pagination loops to prevent OOM/hangs on
// high-volume accounts. At 100 runs per page this caps at 5 000 runs.
const maxPages = 50

// paginateRuns fetches runs using pagination until all matching runs are retrieved
// or the page limit is reached.
func paginateRuns(ctx context.Context, client *inngest.Client, opts inngest.ListRunsOptions, w io.Writer) ([]inngest.FunctionRun, bool, error) {
	var allRuns []inngest.FunctionRun
	truncated := false
	for page := 0; ; page++ {
		result, err := client.ListRuns(ctx, opts)
		if err != nil {
			return nil, false, err
		}
		allRuns = append(allRuns, result.Runs...)
		if !result.Page.HasMore || result.Page.Cursor == "" {
			break
		}
		opts.After = result.Page.Cursor
		if page+1 >= maxPages {
			fmt.Fprintf(w, "Warning: pagination limit reached (%d pages). Results may be incomplete.\n", maxPages)
			truncated = true
			break
		}
	}
	return allRuns, truncated, nil
}

// computeMetrics calculates run metrics from a list of runs.
func computeMetrics(allRuns []inngest.FunctionRun, since string, truncated bool) map[string]any {
	total := len(allRuns)
	statusCounts := map[string]int{}
	var durations []time.Duration

	for _, run := range allRuns {
		statusCounts[run.Status]++
		switch {
		case run.DurationMs > 0:
			durations = append(durations, time.Duration(run.DurationMs)*time.Millisecond)
		case run.StartedAt != nil && run.EndedAt != nil:
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

	slices.Sort(durations)

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

	result := map[string]any{
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

	return result
}

// printMetricsText prints metrics in human-readable text format.
func printMetricsText(result map[string]any) {
	fmt.Printf("Metrics (last %s):\n\n", result["period"])
	fmt.Printf("  Total runs:    %d\n", result["total"])
	fmt.Printf("  Completed:     %d\n", result["completed"])
	fmt.Printf("  Failed:        %d\n", result["failed"])
	fmt.Printf("  Running:       %d\n", result["running"])
	fmt.Printf("  Cancelled:     %d\n", result["cancelled"])
	fmt.Printf("  Success rate:  %s\n", result["successRate"])
	fmt.Printf("  Failure rate:  %s\n", result["failureRate"])
	if samples, ok := result["durationSamples"]; ok {
		fmt.Printf("\n  Duration (%d samples):\n", samples)
		fmt.Printf("    P50:  %s\n", result["p50"])
		fmt.Printf("    P90:  %s\n", result["p90"])
		fmt.Printf("    P99:  %s\n", result["p99"])
	}
	if _, ok := result["truncated"]; ok {
		fmt.Printf("\n  Note: results truncated at %d runs. Use --since with a shorter duration for complete metrics.\n", result["truncatedAt"])
	}
}

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
			ctx := cmd.Context()

			type checkResult struct {
				Check  string `json:"check"`
				Status string `json:"status"`
				Detail string `json:"detail,omitempty"`
			}

			var results []checkResult
			allPassed := true

			// 1. API credential configured
			if cfg.GetAPICredential() != "" {
				results = append(results, checkResult{
					Check:  "api_key",
					Status: "ok",
					Detail: "configured",
				})
			} else {
				results = append(results, checkResult{
					Check:  "api_key",
					Status: "fail",
					Detail: "not configured — set INNGEST_API_KEY / INNGEST_SIGNING_KEY or run inngest auth login",
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

			// 3. API reachability + credential validity: an authenticated v2 call.
			// A bad key surfaces here as a 401 instead of an empty result.
			if _, err := client.ListEnvironments(ctx); err != nil {
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
					Detail: "reachable, credential accepted",
				})
			}

			// 4. Dev server reachability (if --dev or auto-detect)
			devUp := client.IsDevServerRunning(ctx)
			switch {
			case devUp:
				results = append(results, checkResult{
					Check:  "dev_server",
					Status: "ok",
					Detail: "reachable at " + state.DevServer,
				})
			case state.DevMode:
				results = append(results, checkResult{
					Check:  "dev_server",
					Status: "fail",
					Detail: "not reachable at " + state.DevServer,
				})
				allPassed = false
			default:
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
				_ = output.Print(map[string]any{
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
		app      string
	)

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show run metrics and success/failure rates",
		Long:  "Query recent runs and compute total counts, status breakdown, success/failure rates, and duration percentiles.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)

			fromTime, err := parseSince("since", since)
			if err != nil {
				return err
			}

			opts := inngest.ListRunsOptions{
				First:       inngest.MaxRunsPageSize,
				From:        fromTime,
				FunctionIDs: splitCSV(function),
				AppIDs:      splitCSV(app),
			}
			allRuns, truncated, err := paginateRuns(cmd.Context(), client, opts, cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("querying runs: %w", err)
			}

			result := computeMetrics(allRuns, since, truncated)

			if format == output.FormatText || format == output.FormatTable {
				printMetricsText(result)
				return nil
			}

			return output.Print(result, format)
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Time period to query (e.g. 1h, 24h, 168h)")
	cmd.Flags().StringVar(&function, "function", "", "Filter by function ID (comma-separated)")
	cmd.Flags().StringVar(&app, "app", "", "Filter by app ID (comma-separated)")

	return cmd
}

type backlogEntry struct {
	Function string `json:"function"`
	Running  int    `json:"running"`
	Queued   int    `json:"queued"`
	Total    int    `json:"total"`
}

// groupRunsByFunction groups runs by function name, sorted by total descending.
func groupRunsByFunction(allRuns []inngest.FunctionRun) []backlogEntry {
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

	entries := make([]backlogEntry, 0, len(counts))
	for _, e := range counts {
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Total > entries[j].Total
	})
	return entries
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

			// One server-side filtered query for both statuses.
			opts := inngest.ListRunsOptions{
				First:  inngest.MaxRunsPageSize,
				Status: []string{"RUNNING", "QUEUED"},
				From:   time.Now().Add(-24 * time.Hour),
			}
			allRuns, truncated, err := paginateRuns(cmd.Context(), client, opts, cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("querying runs: %w", err)
			}

			entries := groupRunsByFunction(allRuns)

			if len(entries) == 0 {
				if format == output.FormatText || format == output.FormatTable {
					fmt.Println("No queued or running functions.")
					return nil
				}
				return output.Print([]backlogEntry{}, format)
			}

			if truncated && (format == output.FormatText || format == output.FormatTable) {
				fmt.Printf("\nNote: results truncated at %d runs. Use --since with a shorter duration for complete metrics.\n", len(allRuns))
			}

			if format == output.FormatJSON {
				result := map[string]any{
					"entries": entries,
				}
				if truncated {
					result["truncated"] = true
					result["truncatedAt"] = len(allRuns)
				}
				return output.Print(result, format)
			}

			return output.Print(entries, format)
		},
	}
}
