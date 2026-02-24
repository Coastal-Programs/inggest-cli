package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewCreditNotesCmd returns the credit-notes command group.
func NewCreditNotesCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "credit-notes",
		Aliases: []string{"cn"},
		Short:   "Manage Xero credit notes",
	}
	cmd.AddCommand(
		newCreditNotesListCmd(format, org),
		newCreditNotesGetCmd(format, org),
		newCreditNotesCreateCmd(format, org),
		newCreditNotesApplyCmd(format, org),
	)
	return cmd
}

func newCreditNotesListCmd(format *string, org *string) *cobra.Command {
	var status, cnType string
	var page int
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List credit notes",
		Example: `  xero credit-notes list
  xero credit-notes list --status AUTHORISED --type ACCREC
  xero credit-notes list --all-orgs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if allOrgs {
				clients, tenants, err := getAllClients()
				if err != nil {
					return err
				}
				type result struct {
					Org         string            `json:"org"`
					TenantID    string            `json:"tenant_id"`
					CreditNotes []xero.CreditNote `json:"credit_notes"`
				}
				results := make([]result, 0, len(clients))
				for i, client := range clients {
					cns, err := client.ListCreditNotes(status, cnType, page)
					if err != nil {
						return fmt.Errorf("[%s] %w", tenants[i].TenantName, err)
					}
					results = append(results, result{
						Org:         tenants[i].TenantName,
						TenantID:    tenants[i].TenantID,
						CreditNotes: cns,
					})
				}
				return output.Print(results, output.Format(*format))
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			cns, err := client.ListCreditNotes(status, cnType, page)
			if err != nil {
				return err
			}
			return output.Print(cns, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: DRAFT, SUBMITTED, AUTHORISED, PAID, VOIDED")
	cmd.Flags().StringVar(&cnType, "type", "", "Filter by type: ACCREC or ACCPAY")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (100 per page, 0 = fetch all)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newCreditNotesGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <credit-note-id>",
		Short: "Get a single credit note by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			cn, err := client.GetCreditNote(args[0])
			if err != nil {
				return err
			}
			return output.Print(cn, output.Format(*format))
		},
	}
}

func newCreditNotesCreateCmd(format *string, org *string) *cobra.Command {
	var contactID, cnType, date, dueDate, lineItemsJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new credit note",
		Example: `  xero credit-notes create \
    --contact-id abc-123 \
    --type ACCREC \
    --line-items '[{"Description":"Refund","Quantity":1,"UnitAmount":100,"AccountCode":"200"}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var lineItems []xero.LineItem
			if err := json.Unmarshal([]byte(lineItemsJSON), &lineItems); err != nil {
				return fmt.Errorf("parsing --line-items: %w", err)
			}
			input := xero.CreditNoteCreateInput{
				Type:      cnType,
				Contact:   xero.ContactRef{ContactID: contactID},
				Date:      date,
				DueDate:   dueDate,
				LineItems: lineItems,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			cn, err := client.CreateCreditNote(input)
			if err != nil {
				return err
			}
			return output.Print(cn, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&contactID, "contact-id", "", "Contact ID (required)")
	cmd.Flags().StringVar(&cnType, "type", "ACCREC", "Credit note type: ACCREC or ACCPAY")
	cmd.Flags().StringVar(&date, "date", "", "Credit note date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "Due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&lineItemsJSON, "line-items", "", "JSON array of line items (required)")
	_ = cmd.MarkFlagRequired("contact-id")
	_ = cmd.MarkFlagRequired("line-items")
	return cmd
}

func newCreditNotesApplyCmd(format *string, org *string) *cobra.Command {
	var creditNoteID, invoiceID string
	var amount float64
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a credit note against an invoice",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			cn, err := client.ApplyCreditNote(creditNoteID, invoiceID, amount)
			if err != nil {
				return err
			}
			return output.Print(cn, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&creditNoteID, "credit-note-id", "", "Credit note ID (required)")
	cmd.Flags().StringVar(&invoiceID, "invoice-id", "", "Invoice ID to apply against (required)")
	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount to apply (required)")
	_ = cmd.MarkFlagRequired("credit-note-id")
	_ = cmd.MarkFlagRequired("invoice-id")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}
