package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the cx-onprem-orchestrator version and build info",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "cx-onprem-orchestrator %s (%s/%s, %s)\n",
				Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
			return nil
		},
	}
}
