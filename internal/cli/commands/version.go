package commands

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Coastal-Programs/inggest-cli/internal/cli/state"
	"github.com/Coastal-Programs/inggest-cli/pkg/output"
)

// NewVersionCmd returns the "version" command.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := map[string]string{
				"version": state.AppVersion,
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			}
			return output.Print(info, output.Format(state.Output))
		},
	}
}
