package commands

import (
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewBankCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bank",
		Aliases: []string{"b"},
		Short:   "View bank accounts and transactions",
	}
	cmd.AddCommand(
		newBankAccountsCmd(format, org),
		newBankTransactionsCmd(format, org),
		newBankTransactionGetCmd(format, org),
	)
	return cmd
}

func newBankAccountsCmd(format *string, org *string) *cobra.Command {
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "List bank accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if allOrgs {
				clients, tenants, err := getAllClients()
				if err != nil {
					return err
				}
				type result struct {
					Org      string `json:"org"`
					Accounts any    `json:"accounts"`
				}
				results := make([]result, 0, len(clients))
				for i, client := range clients {
					accounts, err := client.ListBankAccounts()
					if err != nil {
						return err
					}
					results = append(results, result{Org: tenants[i].TenantName, Accounts: accounts})
				}
				return output.Print(results, output.Format(*format))
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			accounts, err := client.ListBankAccounts()
			if err != nil {
				return err
			}
			return output.Print(accounts, output.Format(*format))
		},
	}
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newBankTransactionsCmd(format *string, org *string) *cobra.Command {
	var accountID string
	var page int
	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "List bank transactions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if page == 0 {
				page = 1
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			txns, err := client.ListBankTransactions(accountID, page)
			if err != nil {
				return err
			}
			return output.Print(txns, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "Filter to a specific bank account ID")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	return cmd
}

func newBankTransactionGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <transaction-id>",
		Short: "Get a single bank transaction by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			txn, err := client.GetBankTransaction(args[0])
			if err != nil {
				return err
			}
			return output.Print(txn, output.Format(*format))
		},
	}
}
