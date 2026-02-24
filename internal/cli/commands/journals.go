package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewJournalsCmd returns the manual journals command group.
func NewJournalsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "journals",
		Aliases: []string{"j"},
		Short:   "Manage Xero manual journals",
	}
	cmd.AddCommand(
		newJournalsListCmd(format, org),
		newJournalsGetCmd(format, org),
		newJournalsCreateCmd(format, org),
	)
	return cmd
}

func newJournalsListCmd(format *string, org *string) *cobra.Command {
	var fromDate, toDate string
	var page int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List manual journals",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			journals, err := client.ListManualJournals(fromDate, toDate, page)
			if err != nil {
				return err
			}
			return output.Print(journals, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (100 per page, 0 = fetch all)")
	return cmd
}

func newJournalsGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <journal-id>",
		Short: "Get a manual journal by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			j, err := client.GetManualJournal(args[0])
			if err != nil {
				return err
			}
			return output.Print(j, output.Format(*format))
		},
	}
}

func newJournalsCreateCmd(format *string, org *string) *cobra.Command {
	var narration, date, linesJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a manual journal",
		Example: `  xero journals create \
    --narration "Accrual entry" \
    --lines '[{"AccountCode":"800","LineAmount":500,"Description":"Accrued income"},{"AccountCode":"801","LineAmount":-500}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var lines []xero.ManualJournalLine
			if err := json.Unmarshal([]byte(linesJSON), &lines); err != nil {
				return fmt.Errorf("parsing --lines: %w", err)
			}
			input := xero.ManualJournalCreateInput{
				Narration:    narration,
				Date:         date,
				JournalLines: lines,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			j, err := client.CreateManualJournal(input)
			if err != nil {
				return err
			}
			return output.Print(j, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&narration, "narration", "", "Journal narration (required)")
	cmd.Flags().StringVar(&date, "date", "", "Journal date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&linesJSON, "lines", "", "JSON array of journal lines (required)")
	_ = cmd.MarkFlagRequired("narration")
	_ = cmd.MarkFlagRequired("lines")
	return cmd
}

// NewJournalLedgerCmd returns the journal ledger command group (read-only audit trail).
func NewJournalLedgerCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "journal-ledger",
		Aliases: []string{"jl"},
		Short:   "Browse the Xero journal ledger (read-only audit trail)",
	}
	cmd.AddCommand(newJournalLedgerListCmd(format, org))
	return cmd
}

func newJournalLedgerListCmd(format *string, org *string) *cobra.Command {
	var fromDate, toDate string
	var offset int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List journal ledger entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			entries, err := client.ListJournalEntries(fromDate, toDate, offset)
			if err != nil {
				return err
			}
			return output.Print(entries, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Record offset (0 = fetch all)")
	return cmd
}
