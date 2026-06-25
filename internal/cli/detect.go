package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/ci"
	"github.com/spf13/cobra"
)

func newDetectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detect",
		Short: "Print the detected CI system and resolved repo/branch/commit/workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := ci.Detect(os.Getenv, ci.GitIntrospect)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintf(w, "provider\t%s\n", ctx.Provider)
			fmt.Fprintf(w, "branch\t%s\n", orNA(ctx.Branch))
			fmt.Fprintf(w, "commit\t%s\n", orNA(ctx.Commit))
			fmt.Fprintf(w, "repo\t%s\n", orNA(ctx.Repo))
			fmt.Fprintf(w, "workspace\t%s\n", orNA(ctx.Workspace))
			w.Flush()
			for _, n := range ctx.Notes {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", n)
			}
			return nil
		},
	}
}

func orNA(s string) string {
	if s == "" {
		return "(not detected)"
	}
	return s
}
