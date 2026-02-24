package commands

import (
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewTaxRatesCmd returns the tax-rates command group.
func NewTaxRatesCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tax-rates",
		Aliases: []string{"tax"},
		Short:   "View Xero tax rates",
	}
	cmd.AddCommand(newTaxRatesListCmd(format, org))
	return cmd
}

func newTaxRatesListCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tax rates",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			rates, err := client.ListTaxRates()
			if err != nil {
				return err
			}
			return output.Print(rates, output.Format(*format))
		},
	}
}
