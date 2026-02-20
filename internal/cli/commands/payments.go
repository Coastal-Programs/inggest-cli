package commands

import (
	"fmt"
	"strconv"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewPaymentsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "payments",
		Aliases: []string{"pay", "p"},
		Short:   "Manage Xero payments",
	}
	cmd.AddCommand(
		newPaymentsListCmd(format, org),
		newPaymentsGetCmd(format, org),
		newPaymentsCreateCmd(format, org),
	)
	return cmd
}

func newPaymentsListCmd(format *string, org *string) *cobra.Command {
	var status string
	var page int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List payments",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			payments, err := client.ListPayments(status, page)
			if err != nil {
				return err
			}
			return output.Print(payments, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: AUTHORISED, DELETED")
	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	return cmd
}

func newPaymentsGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <payment-id>",
		Short: "Get a single payment by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			payment, err := client.GetPayment(args[0])
			if err != nil {
				return err
			}
			return output.Print(payment, output.Format(*format))
		},
	}
}

func newPaymentsCreateCmd(format *string, org *string) *cobra.Command {
	var invoiceID, accountID, accountCode, date, reference, amountStr string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Apply a payment to an invoice",
		RunE: func(cmd *cobra.Command, args []string) error {
			if invoiceID == "" {
				return fmt.Errorf("--invoice-id is required")
			}
			if accountID == "" && accountCode == "" {
				return fmt.Errorf("--account-id or --account-code is required")
			}
			if amountStr == "" {
				return fmt.Errorf("--amount is required")
			}
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				return fmt.Errorf("invalid --amount: %w", err)
			}
			input := xero.PaymentCreateInput{
				Invoice:   xero.InvoiceRef{InvoiceID: invoiceID},
				Account:   xero.AccountRef{AccountID: accountID, Code: accountCode},
				Date:      date,
				Amount:    amount,
				Reference: reference,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			payment, err := client.CreatePayment(input)
			if err != nil {
				return err
			}
			return output.Print(payment, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&invoiceID, "invoice-id", "", "Invoice ID (required)")
	cmd.Flags().StringVar(&accountID, "account-id", "", "Bank account ID")
	cmd.Flags().StringVar(&accountCode, "account-code", "", "Bank account code")
	cmd.Flags().StringVar(&amountStr, "amount", "", "Payment amount (required)")
	cmd.Flags().StringVar(&date, "date", "", "Payment date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&reference, "reference", "", "Payment reference")
	return cmd
}
