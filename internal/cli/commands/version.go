package commands

import (
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewVersionCmd returns the version command.
func NewVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the xero CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return output.Print(map[string]string{"version": version}, output.FormatJSON)
		},
	}
}
