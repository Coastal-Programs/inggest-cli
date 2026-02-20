package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewInvoicesCmd returns the invoices command group.
func NewInvoicesCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invoices",
		Aliases: []string{"inv", "i"},
		Short:   "Manage Xero invoices and bills",
	}
	cmd.AddCommand(
		newInvoicesListCmd(format, org),
		newInvoicesGetCmd(format, org),
		newInvoicesCreateCmd(format, org),
		newInvoicesVoidCmd(format, org),
		newInvoicesEmailCmd(format, org),
	)
	return cmd
}

func newInvoicesListCmd(format *string, org *string) *cobra.Command {
	var status, invType, dateFrom, dateTo string
	var page int
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List invoices",
		Example: `  xero invoices list
  xero invoices list --status AUTHORISED --from 2024-01-01 --to 2024-03-31
  xero invoices list --all-orgs
  xero invoices list --org "Agency Name"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if allOrgs {
				clients, tenants, err := getAllClients()
				if err != nil {
					return err
				}
				type result struct {
					Org      string         `json:"org"`
					TenantID string         `json:"tenant_id"`
					Invoices []xero.Invoice `json:"invoices"`
				}
				results := make([]result, 0, len(clients))
				for i, client := range clients {
					invoices, err := client.ListInvoices(status, invType, dateFrom, dateTo, page)
					if err != nil {
						return fmt.Errorf("[%s] %w", tenants[i].TenantName, err)
					}
					results = append(results, result{
						Org:      tenants[i].TenantName,
						TenantID: tenants[i].TenantID,
						Invoices: invoices,
					})
				}
				return output.Print(results, output.Format(*format))
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			invoices, err := client.ListInvoices(status, invType, dateFrom, dateTo, page)
			if err != nil {
				return err
			}
			return output.Print(invoices, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: DRAFT, SUBMITTED, AUTHORISED, PAID, VOIDED")
	cmd.Flags().StringVar(&invType, "type", "", "Filter by type: ACCREC (invoice) or ACCPAY (bill)")
	cmd.Flags().StringVar(&dateFrom, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dateTo, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (100 per page, 0 = fetch all)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newInvoicesGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <invoice-id>",
		Short: "Get a single invoice by ID or number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			inv, err := client.GetInvoice(args[0])
			if err != nil {
				return err
			}
			return output.Print(inv, output.Format(*format))
		},
	}
}

func newInvoicesCreateCmd(format *string, org *string) *cobra.Command {
	var (
		contactID, contactName string
		invType, date, dueDate string
		lineItemsJSON          string
		reference              string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new invoice or bill",
		Example: `  xero invoices create \
    --contact-id abc-123 \
    --type ACCREC \
    --due-date 2024-03-31 \
    --line-items '[{"Description":"Consulting","Quantity":10,"UnitAmount":150,"AccountCode":"200"}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if contactID == "" && contactName == "" {
				return fmt.Errorf("--contact-id or --contact-name is required")
			}
			if lineItemsJSON == "" {
				return fmt.Errorf("--line-items is required")
			}
			var lineItems []xero.LineItem
			if err := json.Unmarshal([]byte(lineItemsJSON), &lineItems); err != nil {
				return fmt.Errorf("parsing --line-items: %w", err)
			}
			if invType == "" {
				invType = "ACCREC"
			}
			input := xero.InvoiceCreateInput{
				Type:      invType,
				Contact:   xero.ContactRef{ContactID: contactID, Name: contactName},
				Date:      date,
				DueDate:   dueDate,
				LineItems: lineItems,
				Reference: reference,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			inv, err := client.CreateInvoice(input)
			if err != nil {
				return err
			}
			return output.Print(inv, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&contactID, "contact-id", "", "Contact ID")
	cmd.Flags().StringVar(&contactName, "contact-name", "", "Contact name")
	cmd.Flags().StringVar(&invType, "type", "ACCREC", "Invoice type: ACCREC or ACCPAY")
	cmd.Flags().StringVar(&date, "date", "", "Invoice date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "Due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&lineItemsJSON, "line-items", "", "JSON array of line items (required)")
	cmd.Flags().StringVar(&reference, "reference", "", "Reference number")
	return cmd
}

func newInvoicesVoidCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "void <invoice-id>",
		Short: "Void an invoice",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			inv, err := client.VoidInvoice(args[0])
			if err != nil {
				return err
			}
			return output.Print(inv, output.Format(*format))
		},
	}
}

func newInvoicesEmailCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "email <invoice-id>",
		Short: "Send an invoice by email to the contact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			if err := client.EmailInvoice(args[0]); err != nil {
				return err
			}
			return output.Print(map[string]string{
				"status":     "sent",
				"invoice_id": args[0],
			}, output.Format(*format))
		},
	}
}
