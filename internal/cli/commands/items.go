package commands

import (
	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewItemsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "items",
		Aliases: []string{"it"},
		Short:   "Manage Xero inventory items",
	}
	cmd.AddCommand(
		newItemsListCmd(format, org),
		newItemsGetCmd(format, org),
		newItemsCreateCmd(format, org),
	)
	return cmd
}

func newItemsListCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all inventory items",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			items, err := client.ListItems()
			if err != nil {
				return err
			}
			return output.Print(items, output.Format(*format))
		},
	}
}

func newItemsGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <item-id-or-code>",
		Short: "Get a single item by ID or code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			item, err := client.GetItem(args[0])
			if err != nil {
				return err
			}
			return output.Print(item, output.Format(*format))
		},
	}
}

func newItemsCreateCmd(format *string, org *string) *cobra.Command {
	var (
		code, name, description  string
		salesPrice, buyPrice     float64
		salesAccount, buyAccount string
		isSold, isPurchased      bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new inventory item",
		RunE: func(cmd *cobra.Command, args []string) error {
			input := xero.ItemCreateInput{
				Code:        code,
				Name:        name,
				Description: description,
				IsSold:      isSold,
				IsPurchased: isPurchased,
				SalesDetails:    xero.ItemDetails{UnitPrice: salesPrice, AccountCode: salesAccount},
				PurchaseDetails: xero.ItemDetails{UnitPrice: buyPrice, AccountCode: buyAccount},
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			item, err := client.CreateItem(input)
			if err != nil {
				return err
			}
			return output.Print(item, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&code, "code", "", "Item code (required)")
	cmd.Flags().StringVar(&name, "name", "", "Item name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Item description")
	cmd.Flags().Float64Var(&salesPrice, "sales-price", 0, "Default sales unit price")
	cmd.Flags().StringVar(&salesAccount, "sales-account", "", "Sales account code")
	cmd.Flags().Float64Var(&buyPrice, "purchase-price", 0, "Default purchase unit price")
	cmd.Flags().StringVar(&buyAccount, "purchase-account", "", "Purchase account code")
	cmd.Flags().BoolVar(&isSold, "sold", false, "Item is sold")
	cmd.Flags().BoolVar(&isPurchased, "purchased", false, "Item is purchased")
	cmd.MarkFlagRequired("code")
	cmd.MarkFlagRequired("name")
	return cmd
}
