package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/internal/inngest"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// NewFunctionsCmd returns the "functions" command group.
func NewFunctionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "functions",
		Aliases: []string{"fn"},
		Short:   "List and inspect Inngest functions",
		Long:    "Query functions registered with Inngest Cloud or a local dev server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newFunctionsListCmd())
	cmd.AddCommand(newFunctionsGetCmd())
	cmd.AddCommand(newFunctionsConfigCmd())
	return cmd
}

func newFunctionsListCmd() *cobra.Command {
	var appFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all functions with their triggers and config",
		Long:  "List all functions registered with Inngest Cloud or the local dev server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			functions, err := client.ListFunctions(ctx)
			if err != nil {
				return fmt.Errorf("listing functions: %w", err)
			}

			// Filter by app name if specified.
			if appFilter != "" {
				var filtered []inngest.Function
				lower := strings.ToLower(appFilter)
				for _, fn := range functions {
					if fn.App != nil && strings.ToLower(fn.App.Name) == lower {
						filtered = append(filtered, fn)
					}
				}
				functions = filtered
			}

			if format == output.FormatTable {
				return printFunctionsTable(functions)
			}

			return output.Print(functions, format)
		},
	}

	cmd.Flags().StringVar(&appFilter, "app", "", "Filter by app name")

	return cmd
}

// functionRow is used for table output of functions list.
type functionRow struct {
	Name    string
	Slug    string
	Trigger string
	App     string
	SDK     string
}

func printFunctionsTable(functions []inngest.Function) error {
	rows := make([]functionRow, len(functions))
	for i, fn := range functions {
		var triggers []string
		for _, t := range fn.Triggers {
			triggers = append(triggers, t.Type+":"+t.Value)
		}
		appName := ""
		sdk := ""
		if fn.App != nil {
			appName = fn.App.Name
			sdk = fn.App.SDKLanguage
			if fn.App.SDKVersion != "" {
				sdk += "/" + fn.App.SDKVersion
			}
		}
		rows[i] = functionRow{
			Name:    fn.Name,
			Slug:    fn.Slug,
			Trigger: strings.Join(triggers, ", "),
			App:     appName,
			SDK:     sdk,
		}
	}
	return output.Print(rows, output.FormatTable)
}

func newFunctionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <slug>",
		Short: "Get detailed function info by slug",
		Long:  "Fetch full function details including triggers, configuration, retries, concurrency, and rate limits.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			fn, err := client.GetFunction(ctx, args[0])
			if err != nil {
				return fmt.Errorf("getting function: %w", err)
			}

			if format == output.FormatText {
				return printFunctionDetail(fn)
			}

			return output.Print(fn, format)
		},
	}
}

func printFunctionDetail(fn *inngest.Function) error {
	fmt.Printf("Name:        %s\n", fn.Name)
	fmt.Printf("Slug:        %s\n", fn.Slug)
	fmt.Printf("ID:          %s\n", fn.ID)
	fmt.Printf("App ID:      %s\n", fn.AppID)
	fmt.Printf("URL:         %s\n", fn.URL)
	fmt.Printf("Concurrency: %d\n", fn.Concurrency)

	if fn.App != nil {
		fmt.Printf("\nApp:\n")
		fmt.Printf("  Name:      %s\n", fn.App.Name)
		fmt.Printf("  SDK:       %s/%s\n", fn.App.SDKLanguage, fn.App.SDKVersion)
		if fn.App.Framework != "" {
			fmt.Printf("  Framework: %s\n", fn.App.Framework)
		}
		fmt.Printf("  Connected: %v\n", fn.App.Connected)
	}

	if len(fn.Triggers) > 0 {
		fmt.Printf("\nTriggers:\n")
		for _, t := range fn.Triggers {
			line := fmt.Sprintf("  - %s: %s", t.Type, t.Value)
			if t.Condition != "" {
				line += fmt.Sprintf(" (if %s)", t.Condition)
			}
			fmt.Println(line)
		}
	}

	if fn.Configuration != nil {
		fmt.Printf("\nConfiguration:\n")
		printConfiguration(fn.Configuration)
	}

	return nil
}

// formatConfigLine creates a config line, appending the key suffix if key is non-empty.
func formatConfigLine(label, base, key string) string {
	line := fmt.Sprintf("  %-14s %s", label, base)
	if key != "" {
		line += fmt.Sprintf(" (key: %s)", key)
	}
	return line
}

func printConfiguration(cfg *inngest.FunctionConfiguration) {
	if cfg.Retries != nil {
		dflt := ""
		if cfg.Retries.IsDefault {
			dflt = " (default)"
		}
		fmt.Printf("  Retries:     %d%s\n", cfg.Retries.Value, dflt)
	}
	if len(cfg.Concurrency) > 0 {
		fmt.Printf("  Concurrency:\n")
		for _, c := range cfg.Concurrency {
			limit := 0
			if c.Limit != nil {
				limit = c.Limit.Value
			}
			line := fmt.Sprintf("    - scope: %s, limit: %d", c.Scope, limit)
			if c.Key != "" {
				line += fmt.Sprintf(", key: %s", c.Key)
			}
			fmt.Println(line)
		}
	}
	if cfg.RateLimit != nil {
		fmt.Println(formatConfigLine("Rate Limit:", fmt.Sprintf("%d per %s", cfg.RateLimit.Limit, cfg.RateLimit.Period), cfg.RateLimit.Key))
	}
	if cfg.Debounce != nil {
		fmt.Println(formatConfigLine("Debounce:", cfg.Debounce.Period, cfg.Debounce.Key))
	}
	if cfg.Throttle != nil {
		fmt.Println(formatConfigLine("Throttle:", fmt.Sprintf("limit: %d, burst: %d, period: %s", cfg.Throttle.Limit, cfg.Throttle.Burst, cfg.Throttle.Period), cfg.Throttle.Key))
	}
	if cfg.EventsBatch != nil {
		fmt.Println(formatConfigLine("Events Batch:", fmt.Sprintf("maxSize: %d, timeout: %s", cfg.EventsBatch.MaxSize, cfg.EventsBatch.Timeout), cfg.EventsBatch.Key))
	}
	if cfg.Priority != "" {
		fmt.Printf("  Priority:    %s\n", cfg.Priority)
	}
}

func newFunctionsConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config <slug>",
		Short: "Show function configuration (concurrency, throttle, retry, etc.)",
		Long:  "Fetch a function by slug and display its configuration details.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newCloudClient()
			format := output.Format(state.Output)
			ctx := context.Background()

			fn, err := client.GetFunction(ctx, args[0])
			if err != nil {
				return fmt.Errorf("getting function config: %w", err)
			}

			// Build a combined config view from both config JSON and configuration struct.
			result := buildConfigOutput(fn)

			if format == output.FormatText {
				if fn.Configuration != nil {
					fmt.Printf("Configuration for %s:\n\n", fn.Slug)
					printConfiguration(fn.Configuration)
				}
				if fn.Config != "" {
					fmt.Printf("\nRaw Config:\n")
					var pretty json.RawMessage
					if err := json.Unmarshal([]byte(fn.Config), &pretty); err == nil {
						b, _ := json.MarshalIndent(pretty, "  ", "  ")
						fmt.Printf("  %s\n", string(b))
					} else {
						fmt.Printf("  %s\n", fn.Config)
					}
				}
				return nil
			}

			return output.Print(result, format)
		},
	}
}

func buildConfigOutput(fn *inngest.Function) map[string]any {
	result := map[string]any{
		"slug": fn.Slug,
		"name": fn.Name,
	}

	if fn.Configuration != nil {
		result["configuration"] = fn.Configuration
	}

	if fn.Config != "" {
		var parsed any
		if err := json.Unmarshal([]byte(fn.Config), &parsed); err == nil {
			result["rawConfig"] = parsed
		} else {
			result["rawConfig"] = fn.Config
		}
	}

	return result
}
