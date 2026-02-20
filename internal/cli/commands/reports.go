package commands

import (
	"fmt"
	"time"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewReportsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reports",
		Aliases: []string{"rep", "r"},
		Short:   "Fetch Xero financial reports",
	}
	cmd.AddCommand(
		newReportsPLCmd(format, org),
		newReportsBalanceSheetCmd(format, org),
		newReportsTrialBalanceCmd(format, org),
		newReportsAgedReceivablesCmd(format, org),
		newReportsAgedPayablesCmd(format, org),
	)
	return cmd
}

func today() string { return time.Now().Format("2006-01-02") }
func firstOfYear() string {
	now := time.Now()
	return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
}

func newReportsPLCmd(format *string, org *string) *cobra.Command {
	var fromDate, toDate string
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "profit-loss",
		Short: "Profit & Loss report",
		Example: `  xero reports profit-loss
  xero reports profit-loss --from 2024-01-01 --to 2024-03-31
  xero reports profit-loss --all-orgs --from 2024-01-01 --to 2024-12-31`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromDate == "" {
				fromDate = firstOfYear()
			}
			if toDate == "" {
				toDate = today()
			}
			if allOrgs {
				return runReportAllOrgs(format, func(client *xero.Client) (any, error) {
					return client.GetProfitAndLoss(fromDate, toDate)
				})
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			report, err := client.GetProfitAndLoss(fromDate, toDate)
			if err != nil {
				return err
			}
			return output.Print(report, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD, defaults to Jan 1 of current year)")
	cmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Run report across all connected orgs")
	return cmd
}

func newReportsBalanceSheetCmd(format *string, org *string) *cobra.Command {
	var date string
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "balance-sheet",
		Short: "Balance sheet report",
		Example: `  xero reports balance-sheet
  xero reports balance-sheet --all-orgs --date 2024-06-30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				date = today()
			}
			if allOrgs {
				d := date
				return runReportAllOrgs(format, func(client *xero.Client) (any, error) {
					return client.GetBalanceSheet(d)
				})
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			report, err := client.GetBalanceSheet(date)
			if err != nil {
				return err
			}
			return output.Print(report, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Report date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Run report across all connected orgs")
	return cmd
}

func newReportsTrialBalanceCmd(format *string, org *string) *cobra.Command {
	var date string
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "trial-balance",
		Short: "Trial balance report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				date = today()
			}
			if allOrgs {
				d := date
				return runReportAllOrgs(format, func(client *xero.Client) (any, error) {
					return client.GetTrialBalance(d)
				})
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			report, err := client.GetTrialBalance(date)
			if err != nil {
				return err
			}
			return output.Print(report, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Report date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Run report across all connected orgs")
	return cmd
}

func newReportsAgedReceivablesCmd(format *string, org *string) *cobra.Command {
	var date, contactID string
	cmd := &cobra.Command{
		Use:   "aged-receivables",
		Short: "Aged receivables report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				date = today()
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			report, err := client.GetAgedReceivables(date, contactID)
			if err != nil {
				return err
			}
			return output.Print(report, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Report date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&contactID, "contact-id", "", "Filter to a specific contact")
	return cmd
}

func newReportsAgedPayablesCmd(format *string, org *string) *cobra.Command {
	var date, contactID string
	cmd := &cobra.Command{
		Use:   "aged-payables",
		Short: "Aged payables report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				date = today()
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			report, err := client.GetAgedPayables(date, contactID)
			if err != nil {
				return err
			}
			return output.Print(report, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Report date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&contactID, "contact-id", "", "Filter to a specific contact")
	return cmd
}

// runReportAllOrgs runs a report function against every connected org and returns aggregated JSON.
func runReportAllOrgs(format *string, fn func(*xero.Client) (any, error)) error {
	clients, tenants, err := getAllClients()
	if err != nil {
		return err
	}
	type result struct {
		Org      string `json:"org"`
		TenantID string `json:"tenant_id"`
		Report   any    `json:"report"`
		Error    string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(clients))
	for i, client := range clients {
		r, err := fn(client)
		row := result{Org: tenants[i].TenantName, TenantID: tenants[i].TenantID}
		if err != nil {
			row.Error = fmt.Sprintf("%v", err)
		} else {
			row.Report = r
		}
		results = append(results, row)
	}
	return output.Print(results, output.Format(*format))
}
