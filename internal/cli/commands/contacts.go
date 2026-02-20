package commands

import (
	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewContactsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "contacts",
		Aliases: []string{"con", "c"},
		Short:   "Manage Xero contacts (customers and suppliers)",
	}
	cmd.AddCommand(
		newContactsListCmd(format, org),
		newContactsGetCmd(format, org),
		newContactsCreateCmd(format, org),
		newContactsUpdateCmd(format, org),
	)
	return cmd
}

func newContactsListCmd(format *string, org *string) *cobra.Command {
	var search string
	var isCustomer, isSupplier bool
	var page int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List contacts",
		Example: `  xero contacts list
  xero contacts list --search "Acme"
  xero contacts list --customer --org "Agency Name"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			contacts, err := client.ListContacts(search, isCustomer, isSupplier, page)
			if err != nil {
				return err
			}
			return output.Print(contacts, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Search by name or email")
	cmd.Flags().BoolVar(&isCustomer, "customer", false, "Filter to customers only")
	cmd.Flags().BoolVar(&isSupplier, "supplier", false, "Filter to suppliers only")
	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	return cmd
}

func newContactsGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:  "get <contact-id>",
		Short: "Get a single contact by ID",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			contact, err := client.GetContact(args[0])
			if err != nil {
				return err
			}
			return output.Print(contact, output.Format(*format))
		},
	}
}

func newContactsCreateCmd(format *string, org *string) *cobra.Command {
	var (
		name, firstName, lastName string
		email, accountNum         string
		taxNumber                 string
		isCustomer, isSupplier    bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new contact",
		RunE: func(cmd *cobra.Command, args []string) error {
			input := xero.ContactCreateInput{
				Name:          name,
				FirstName:     firstName,
				LastName:      lastName,
				EmailAddress:  email,
				AccountNumber: accountNum,
				TaxNumber:     taxNumber,
				IsCustomer:    isCustomer,
				IsSupplier:    isSupplier,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			contact, err := client.CreateContact(input)
			if err != nil {
				return err
			}
			return output.Print(contact, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Contact name (required)")
	cmd.Flags().StringVar(&firstName, "first-name", "", "First name")
	cmd.Flags().StringVar(&lastName, "last-name", "", "Last name")
	cmd.Flags().StringVar(&email, "email", "", "Email address")
	cmd.Flags().StringVar(&accountNum, "account-number", "", "Account number")
	cmd.Flags().StringVar(&taxNumber, "tax-number", "", "Tax/VAT number")
	cmd.Flags().BoolVar(&isCustomer, "customer", false, "Mark as a customer")
	cmd.Flags().BoolVar(&isSupplier, "supplier", false, "Mark as a supplier")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newContactsUpdateCmd(format *string, org *string) *cobra.Command {
	var name, firstName, lastName, email, accountNum string
	cmd := &cobra.Command{
		Use:  "update <contact-id>",
		Short: "Update an existing contact",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := xero.ContactCreateInput{
				Name:          name,
				FirstName:     firstName,
				LastName:      lastName,
				EmailAddress:  email,
				AccountNumber: accountNum,
			}
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			contact, err := client.UpdateContact(args[0], input)
			if err != nil {
				return err
			}
			return output.Print(contact, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Contact name")
	cmd.Flags().StringVar(&firstName, "first-name", "", "First name")
	cmd.Flags().StringVar(&lastName, "last-name", "", "Last name")
	cmd.Flags().StringVar(&email, "email", "", "Email address")
	cmd.Flags().StringVar(&accountNum, "account-number", "", "Account number")
	return cmd
}
