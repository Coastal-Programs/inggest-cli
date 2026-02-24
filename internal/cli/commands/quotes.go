package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewQuotesCmd returns the quotes command group.
func NewQuotesCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quotes",
		Aliases: []string{"q"},
		Short:   "Manage Xero quotes",
	}
	cmd.AddCommand(
		newQuotesListCmd(format, org),
		newQuotesGetCmd(format, org),
		newQuotesCreateCmd(format, org),
		newQuotesConvertCmd(format, org),
	)
	return cmd
}

func newQuotesListCmd(format *string, org *string) *cobra.Command {
	var status string
	var page int
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List quotes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if allOrgs {
				clients, tenants, err := getAllClients()
				if err != nil {
					return err
				}
				type result struct {
					Org      string       `json:"org"`
					TenantID string       `json:"tenant_id"`
					Quotes   []xero.Quote `json:"quotes"`
				}
				results := make([]result, 0, len(clients))
				for i, client := range clients {
					qs, err := client.ListQuotes(status, page)
					if err != nil {
						return fmt.Errorf("[%s] %w", tenants[i].TenantName, err)
					}
					results = append(results, result{
						Org:      tenants[i].TenantName,
						TenantID: tenants[i].TenantID,
						Quotes:   qs,
					})
				}
				return output.Print(results, output.Format(*format))
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			qs, err := client.ListQuotes(status, page)
			if err != nil {
				return err
			}
			return output.Print(qs, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: DRAFT, SENT, DECLINED, ACCEPTED, INVOICED, DELETED")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (100 per page, 0 = fetch all)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newQuotesGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <quote-id>",
		Short: "Get a single quote by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			q, err := client.GetQuote(args[0])
			if err != nil {
				return err
			}
			return output.Print(q, output.Format(*format))
		},
	}
}

func newQuotesCreateCmd(format *string, org *string) *cobra.Command {
	var contactID, expiryDate, lineItemsJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new quote",
		Example: `  xero quotes create \
    --contact-id abc-123 \
    --expiry-date 2026-03-31 \
    --line-items '[{"Description":"Consulting","Quantity":5,"UnitAmount":200,"AccountCode":"200"}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var lineItems []xero.LineItem
			if err := json.Unmarshal([]byte(lineItemsJSON), &lineItems); err != nil {
				return fmt.Errorf("parsing --line-items: %w", err)
			}
			input := xero.QuoteCreateInput{
				Contact:    xero.ContactRef{ContactID: contactID},
				ExpiryDate: expiryDate,
				LineItems:  lineItems,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			q, err := client.CreateQuote(input)
			if err != nil {
				return err
			}
			return output.Print(q, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&contactID, "contact-id", "", "Contact ID (required)")
	cmd.Flags().StringVar(&expiryDate, "expiry-date", "", "Quote expiry date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&lineItemsJSON, "line-items", "", "JSON array of line items (required)")
	_ = cmd.MarkFlagRequired("contact-id")
	_ = cmd.MarkFlagRequired("line-items")
	return cmd
}

func newQuotesConvertCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "convert <quote-id>",
		Short: "Convert a quote to an invoice",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			inv, err := client.ConvertQuoteToInvoice(args[0])
			if err != nil {
				return err
			}
			return output.Print(inv, output.Format(*format))
		},
	}
}
