package commands

import (
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewTrackingCmd returns the tracking command group.
func NewTrackingCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tracking",
		Aliases: []string{"track"},
		Short:   "Manage Xero tracking categories",
	}
	cmd.AddCommand(
		newTrackingListCmd(format, org),
		newTrackingGetCmd(format, org),
		newTrackingCreateCmd(format, org),
		newTrackingOptionCmd(format, org),
	)
	return cmd
}

func newTrackingListCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracking categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			categories, err := client.ListTrackingCategories()
			if err != nil {
				return err
			}
			return output.Print(categories, output.Format(*format))
		},
	}
}

func newTrackingGetCmd(format *string, org *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <category-id>",
		Short: "Get a tracking category by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			cat, err := client.GetTrackingCategory(args[0])
			if err != nil {
				return err
			}
			return output.Print(cat, output.Format(*format))
		},
	}
}

func newTrackingCreateCmd(format *string, org *string) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tracking category",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			cat, err := client.CreateTrackingCategory(name)
			if err != nil {
				return err
			}
			return output.Print(cat, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Category name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newTrackingOptionCmd(format *string, org *string) *cobra.Command {
	optionCmd := &cobra.Command{
		Use:   "option",
		Short: "Manage tracking category options",
	}
	optionCmd.AddCommand(newTrackingOptionAddCmd(format, org))
	return optionCmd
}

func newTrackingOptionAddCmd(format *string, org *string) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add <category-id>",
		Short: "Add an option to a tracking category",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := getClientForOrg(*org)
			if err != nil {
				return err
			}
			opt, err := client.AddTrackingOption(args[0], name)
			if err != nil {
				return err
			}
			return output.Print(opt, output.Format(*format))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Option name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
