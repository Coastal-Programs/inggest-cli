package commands

import (
	"encoding/json"
	"fmt"

	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewPurchaseOrdersCmd returns the purchase orders command group.
func NewPurchaseOrdersCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "po",
		Short: "Manage Xero purchase orders",
	}
	cmd.AddCommand(
		newPOListCmd(format, org),
		newPOGetCmd(format, org),
		newPOCreateCmd(format, org),
	)
	return cmd
}

func newPOListCmd(format *string, org *string) *cobra.Command {
	var status string
	var page int
	var allOrgs bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List purchase orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			if allOrgs {
				clients, tenants, err := getAllClients()
				if err != nil {
					return err
				}
				type result struct {
					Org            string               `json:"org"`
					TenantID       string               `json:"tenant_id"`
					PurchaseOrders []xero.PurchaseOrder `json:"purchase_orders"`
				}
				results := make([]result, 0, len(clients))
				for i, client := range clients {
					pos, err := client.ListPurchaseOrders(status, page)
					if err != nil {
						return fmt.Errorf("[%s] %w", tenants[i].TenantName, err)
					}
					results = append(results, result{
						Org:            tenants[i].TenantName,
						TenantID:       tenants[i].TenantID,
						PurchaseOrders: pos,
					})
				}
				return output.Print(results, output.Format(*format))
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			pos, err := client.ListPurchaseOrders(status, page)
			if err != nil {
				return err
			}
			return output.Print(pos, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: DRAFT, SUBMITTED, AUTHORISED, BILLED, DELETED")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (100 per page, 0 = fetch all)")
	cmd.Flags().BoolVar(&allOrgs, "all-orgs", false, "Fetch from all connected orgs")
	return cmd
}

func newPOGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <purchase-order-id>",
		Short: "Get a purchase order by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			po, err := client.GetPurchaseOrder(args[0])
			if err != nil {
				return err
			}
			return output.Print(po, output.Format(*format))
		},
	}
}

func newPOCreateCmd(format *string, org *string) *cobra.Command {
	var contactID, deliveryDate, lineItemsJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a purchase order",
		Example: `  xero po create \
    --contact-id abc-123 \
    --line-items '[{"Description":"Office supplies","Quantity":10,"UnitAmount":25,"AccountCode":"300"}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var lineItems []xero.LineItem
			if err := json.Unmarshal([]byte(lineItemsJSON), &lineItems); err != nil {
				return fmt.Errorf("parsing --line-items: %w", err)
			}
			input := xero.PurchaseOrderCreateInput{
				Contact:      xero.ContactRef{ContactID: contactID},
				DeliveryDate: deliveryDate,
				LineItems:    lineItems,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			po, err := client.CreatePurchaseOrder(input)
			if err != nil {
				return err
			}
			return output.Print(po, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&contactID, "contact-id", "", "Contact ID (required)")
	cmd.Flags().StringVar(&deliveryDate, "delivery-date", "", "Delivery date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&lineItemsJSON, "line-items", "", "JSON array of line items (required)")
	_ = cmd.MarkFlagRequired("contact-id")
	_ = cmd.MarkFlagRequired("line-items")
	return cmd
}
