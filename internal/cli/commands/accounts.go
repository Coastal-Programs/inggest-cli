package commands

import (
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewAccountsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "accounts",
		Aliases: []string{"acc"},
		Short:   "View the Xero chart of accounts",
	}
	cmd.AddCommand(
		newAccountsListCmd(format, org),
		newAccountsGetCmd(format, org),
	)
	return cmd
}

func newAccountsListCmd(format *string, org *string) *cobra.Command {
	var accountType, class string
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accounts",
		Example: `  xero accounts list
  xero accounts list --type REVENUE
  xero accounts list --all-orgs`,
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
					accounts, err := client.ListAccounts(accountType, class)
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
			accounts, err := client.ListAccounts(accountType, class)
			if err != nil {
				return err
			}
			return output.Print(accounts, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&accountType, "type", "", "Account type (e.g. REVENUE, EXPENSE, ASSET)")
	cmd.Flags().StringVar(&class, "class", "", "Account class (ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newAccountsGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <account-id-or-code>",
		Short: "Get a single account by ID or code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			account, err := client.GetAccount(args[0])
			if err != nil {
				return err
			}
			return output.Print(account, output.Format(*format))
		},
	}
}
