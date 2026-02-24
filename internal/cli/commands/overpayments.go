package commands

import (
	"fmt"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewOverpaymentsCmd returns the overpayments command group.
func NewOverpaymentsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "overpayments",
		Aliases: []string{"op"},
		Short:   "Manage Xero overpayments",
	}
	cmd.AddCommand(
		newOverpaymentsListCmd(format, org),
		newOverpaymentsApplyCmd(format, org),
	)
	return cmd
}

func newOverpaymentsListCmd(format *string, org *string) *cobra.Command {
	var page int
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List overpayments",
		RunE: func(cmd *cobra.Command, args []string) error {
			if allOrgs {
				clients, tenants, err := getAllClients()
				if err != nil {
					return err
				}
				type result struct {
					Org          string             `json:"org"`
					TenantID     string             `json:"tenant_id"`
					Overpayments []xero.Overpayment `json:"overpayments"`
				}
				results := make([]result, 0, len(clients))
				for i, client := range clients {
					ops, err := client.ListOverpayments(page)
					if err != nil {
						return fmt.Errorf("[%s] %w", tenants[i].TenantName, err)
					}
					results = append(results, result{
						Org:          tenants[i].TenantName,
						TenantID:     tenants[i].TenantID,
						Overpayments: ops,
					})
				}
				return output.Print(results, output.Format(*format))
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			ops, err := client.ListOverpayments(page)
			if err != nil {
				return err
			}
			return output.Print(ops, output.Format(*format))
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "Page number (100 per page, 0 = fetch all)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newOverpaymentsApplyCmd(format *string, org *string) *cobra.Command {
	var overpaymentID, invoiceID string
	var amount float64
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply an overpayment against an invoice",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			op, err := client.ApplyOverpayment(overpaymentID, invoiceID, amount)
			if err != nil {
				return err
			}
			return output.Print(op, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&overpaymentID, "overpayment-id", "", "Overpayment ID (required)")
	cmd.Flags().StringVar(&invoiceID, "invoice-id", "", "Invoice ID to apply against (required)")
	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount to apply (required)")
	_ = cmd.MarkFlagRequired("overpayment-id")
	_ = cmd.MarkFlagRequired("invoice-id")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}

// NewPrepaymentsCmd returns the prepayments command group.
func NewPrepaymentsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "prepayments",
		Aliases: []string{"pp"},
		Short:   "Manage Xero prepayments",
	}
	cmd.AddCommand(
		newPrepaymentsListCmd(format, org),
		newPrepaymentsApplyCmd(format, org),
	)
	return cmd
}

func newPrepaymentsListCmd(format *string, org *string) *cobra.Command {
	var page int
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List prepayments",
		RunE: func(cmd *cobra.Command, args []string) error {
			if allOrgs {
				clients, tenants, err := getAllClients()
				if err != nil {
					return err
				}
				type result struct {
					Org         string            `json:"org"`
					TenantID    string            `json:"tenant_id"`
					Prepayments []xero.Prepayment `json:"prepayments"`
				}
				results := make([]result, 0, len(clients))
				for i, client := range clients {
					pps, err := client.ListPrepayments(page)
					if err != nil {
						return fmt.Errorf("[%s] %w", tenants[i].TenantName, err)
					}
					results = append(results, result{
						Org:         tenants[i].TenantName,
						TenantID:    tenants[i].TenantID,
						Prepayments: pps,
					})
				}
				return output.Print(results, output.Format(*format))
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			pps, err := client.ListPrepayments(page)
			if err != nil {
				return err
			}
			return output.Print(pps, output.Format(*format))
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "Page number (100 per page, 0 = fetch all)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newPrepaymentsApplyCmd(format *string, org *string) *cobra.Command {
	var prepaymentID, invoiceID string
	var amount float64
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a prepayment against an invoice",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			pp, err := client.ApplyPrepayment(prepaymentID, invoiceID, amount)
			if err != nil {
				return err
			}
			return output.Print(pp, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&prepaymentID, "prepayment-id", "", "Prepayment ID (required)")
	cmd.Flags().StringVar(&invoiceID, "invoice-id", "", "Invoice ID to apply against (required)")
	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount to apply (required)")
	_ = cmd.MarkFlagRequired("prepayment-id")
	_ = cmd.MarkFlagRequired("invoice-id")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}
