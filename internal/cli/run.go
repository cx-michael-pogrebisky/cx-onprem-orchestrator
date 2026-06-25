package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/config"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/orchestrator"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/report"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the selected Checkmarx scanners and gate on thresholds",
		Args:  cobra.NoArgs,
	}
	b := registerRunFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		rc, err := resolveConfig(b)
		if err != nil {
			return err
		}
		if rc.DryRun {
			printPlan(cmd.OutOrStdout(), rc)
			return nil
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if rc.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, rc.Timeout)
			defer cancel()
		}

		outcome := orchestrator.Run(ctx, rc, os.Stderr)
		code := exit.Aggregate(outcome.Verdicts)

		sum := buildSummary(rc, outcome, code)
		if path, werr := sum.Write(rc.Output.Path); werr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write run summary: %v\n", werr)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nrun summary: %s\n", path)
		}
		printHumanSummary(cmd.OutOrStdout(), sum)

		if code != exit.CodeSuccess {
			return &exit.CodedError{Code: code}
		}
		return nil
	}
	return cmd
}

func buildSummary(rc *config.RunConfig, oc orchestrator.Outcome, code exit.Code) report.Summary {
	sum := report.Summary{
		SchemaVersion: 1,
		ExitCode:      code.Int(),
		ExitCategory:  code.String(),
		Version:       Version,
		Threshold:     rc.ThresholdRaw,
		Warnings:      rc.Warnings,
		CI: report.CIInfo{
			Provider:  string(rc.CI.Provider),
			Branch:    rc.Branch,
			Commit:    rc.CI.Commit,
			Repo:      rc.CI.Repo,
			Workspace: rc.CI.Workspace,
			Source:    rc.Source,
		},
	}
	// Index results by engine for joining with verdicts.
	resByEngine := map[model.Engine]*model.Result{}
	for _, r := range oc.Results {
		resByEngine[r.Engine] = r
	}
	for _, v := range oc.Verdicts {
		es := report.EngineSummary{
			Engine:   string(v.Engine),
			Verdict:  v.Category.String(),
			Breaches: v.Breaches,
			Message:  v.Message,
		}
		if r := resByEngine[v.Engine]; r != nil {
			es.Ran = r.Ran
			es.Mode = r.Mode
			es.ChildExitCode = r.ChildExitCode
			es.Route = string(r.Route)
			es.NativeGated = r.NativeGated
			es.Counts = r.Counts
			es.Reports = r.ReportPaths
			es.Warnings = r.Warnings
		}
		sum.Engines = append(sum.Engines, es)
	}
	return sum
}

func printHumanSummary(w io.Writer, sum report.Summary) {
	fmt.Fprintf(w, "\nResult: %s (exit %d)\n", sum.ExitCategory, sum.ExitCode)
	for _, e := range sum.Engines {
		line := fmt.Sprintf("  %-14s %s", e.Engine, e.Verdict)
		if e.Ran {
			line += fmt.Sprintf(" (child exit %d)", e.ChildExitCode)
		}
		if len(e.Breaches) > 0 {
			line += " breaches:"
			for _, b := range e.Breaches {
				line += fmt.Sprintf(" %s>=%d(got %d)", b.Severity, b.Limit, b.Actual)
			}
		}
		if e.Message != "" {
			line += " — " + e.Message
		}
		fmt.Fprintln(w, line)
	}
}
