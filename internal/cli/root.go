// Package cli wires the cobra command tree. It contains no business logic: it
// binds flags, assembles a config.Flags, and delegates to internal/config,
// internal/ci, and internal/orchestrator.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X .../internal/cli.Version=...".
var Version = "0.0.0-dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cx-onprem-orchestrator",
		Short: "Unified Checkmarx multi-scanner CLI",
		Long: `cx-onprem-orchestrator orchestrates an arbitrary subset of Checkmarx scanners
(CxSAST on-prem, SCA via SCA Resolver, KICS, 2ms secrets, Container Security) in a
single invocation, replicates the cx CLI threshold and file-filter syntax, lets you
pass exact native flags to each engine, and reduces all results to one exit code.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newVersionCmd(),
		newDetectCmd(),
		newValidateCmd(),
		newRunCmd(),
	)
	return root
}

// Execute builds the command tree, runs it, and returns the process exit code.
func Execute() int {
	root := newRootCmd()
	err := root.Execute()
	if err == nil {
		return int(exit.CodeSuccess)
	}
	var ce *exit.CodedError
	if errors.As(err, &ce) {
		if ce.Err != nil {
			fmt.Fprintf(os.Stderr, "cx-onprem-orchestrator: %v\n", ce.Err)
		}
		return ce.Code.Int()
	}
	// Uncoded errors (e.g. cobra flag/usage errors) are configuration errors.
	fmt.Fprintf(os.Stderr, "cx-onprem-orchestrator: %v\n", err)
	return int(exit.CodeConfigError)
}
