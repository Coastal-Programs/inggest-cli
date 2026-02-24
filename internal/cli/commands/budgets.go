package commands

import (
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewBudgetsCmd returns the budgets command group.
func NewBudgetsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budgets",
		Short: "View Xero budgets",
	}
	cmd.AddCommand(
		newBudgetsListCmd(format, org),
		newBudgetsGetCmd(format, org),
	)
	return cmd
}

func newBudgetsListCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List budgets",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			budgets, err := client.ListBudgets()
			if err != nil {
				return err
			}
			return output.Print(budgets, output.Format(*format))
		},
	}
}

func newBudgetsGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <budget-id>",
		Short: "Get a budget by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			budget, err := client.GetBudget(args[0])
			if err != nil {
				return err
			}
			return output.Print(budget, output.Format(*format))
		},
	}
}
