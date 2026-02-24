package cli

import (
	"os"

	"github.com/jakeschepis/zeus-cli/internal/cli/commands"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	orgQuery     string
)

// Execute runs the root command.
func Execute(version string) error {
	root := newRootCmd(version)
	if err := root.Execute(); err != nil {
		output.PrintError(err.Error(), nil)
		return err
	}
	return nil
}

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xero",
		Short: "Xero accounting CLI",
		Long: `xero is a command-line interface for the Xero accounting API.
It returns structured JSON output, making it ideal for scripting and AI agents.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json, text, table")
	cmd.PersistentFlags().StringVar(&orgQuery, "org", "", "Target org by name or ID (defaults to active org)")

	cmd.SetErr(os.Stderr)
	cmd.SetOut(os.Stdout)

	cmd.AddCommand(
		commands.NewVersionCmd(version),
		commands.NewAuthCmd(&outputFormat),
		commands.NewOrgsCmd(&outputFormat, &orgQuery),
		commands.NewInvoicesCmd(&outputFormat, &orgQuery),
		commands.NewContactsCmd(&outputFormat, &orgQuery),
		commands.NewAccountsCmd(&outputFormat, &orgQuery),
		commands.NewPaymentsCmd(&outputFormat, &orgQuery),
		commands.NewReportsCmd(&outputFormat, &orgQuery),
		commands.NewBankCmd(&outputFormat, &orgQuery),
		commands.NewItemsCmd(&outputFormat, &orgQuery),
		commands.NewConfigCmd(&outputFormat),
		commands.NewCreditNotesCmd(&outputFormat, &orgQuery),
		commands.NewTrackingCmd(&outputFormat, &orgQuery),
		commands.NewJournalsCmd(&outputFormat, &orgQuery),
		commands.NewJournalLedgerCmd(&outputFormat, &orgQuery),
		commands.NewPurchaseOrdersCmd(&outputFormat, &orgQuery),
		commands.NewBudgetsCmd(&outputFormat, &orgQuery),
		commands.NewOverpaymentsCmd(&outputFormat, &orgQuery),
		commands.NewPrepaymentsCmd(&outputFormat, &orgQuery),
		commands.NewQuotesCmd(&outputFormat, &orgQuery),
		commands.NewTaxRatesCmd(&outputFormat, &orgQuery),
	)

	return cmd
}
