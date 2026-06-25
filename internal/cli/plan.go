package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/ci"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/config"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/filter"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
	"github.com/spf13/cobra"
)

// resolveConfig assembles flags, detects CI, resolves, and validates.
func resolveConfig(b *boundFlags) (*config.RunConfig, error) {
	ciCtx := ci.Detect(getenv, ci.GitIntrospect)
	rc, err := config.Resolve(b.toConfigFlags(), getenv, ciCtx)
	if err != nil {
		return nil, exit.Wrap(exit.CodeConfigError, err)
	}
	if err := config.Validate(rc); err != nil {
		return nil, exit.Wrap(exit.CodeConfigError, err)
	}
	return rc, nil
}

// getenv is overridable in tests.
var getenv = os.Getenv

// printPlan renders the resolved run plan: selected engines, threshold routing,
// filter translations + warnings, and (when an engine's scanner is registered)
// the exact native argv that would be executed.
func printPlan(w io.Writer, rc *config.RunConfig) {
	fmt.Fprintf(w, "Plan for project %q on branch %q (CI: %s)\n", rc.ProjectName, orNA(rc.Branch), rc.CI.Provider)
	fmt.Fprintf(w, "Source: %s\n", rc.Source)
	fmt.Fprintf(w, "Engines: %s\n", joinEngines(rc.Scanners))
	if rc.ThresholdRaw != "" {
		fmt.Fprintf(w, "Threshold: %s\n", rc.ThresholdRaw)
	}
	for _, wmsg := range rc.Warnings {
		fmt.Fprintf(w, "WARN: %s\n", wmsg)
	}
	fmt.Fprintln(w)

	for _, e := range rc.Scanners {
		thPlan := rc.Threshold.Partition(e)
		fmt.Fprintf(w, "── %s ──\n", e)
		fmt.Fprintf(w, "  threshold route: %s\n", routeDesc(thPlan))
		if thPlan.HasClauses() {
			var parts []string
			for _, c := range thPlan.Clauses {
				parts = append(parts, fmt.Sprintf("%s=%d", c.Severity, c.Limit))
			}
			fmt.Fprintf(w, "  threshold clauses: %s\n", strings.Join(parts, ", "))
		}
		printFilterPlan(w, rc, e)

		if !scanner.Registered(e) {
			fmt.Fprintf(w, "  invocation: (scanner not yet implemented)\n\n")
			continue
		}
		sc, _ := scanner.Get(e)
		inv, err := sc.BuildInvocation(rc.EngineConfigs[e], thPlan)
		if err != nil {
			fmt.Fprintf(w, "  invocation: ERROR: %v\n\n", err)
			continue
		}
		fmt.Fprintf(w, "  invocation: %s %s\n", inv.Path, strings.Join(inv.Args, " "))
		if len(inv.EnvKeys) > 0 {
			fmt.Fprintf(w, "  env: %s (values read at runtime, redacted)\n", strings.Join(inv.EnvKeys, ", "))
		}
		for _, m := range inv.DockerMounts {
			ro := ""
			if m.ReadOnly {
				ro = ":ro"
			}
			fmt.Fprintf(w, "  mount: %s -> %s%s\n", m.HostPath, m.ContainerPath, ro)
		}
		fmt.Fprintln(w)
	}
}

func printFilterPlan(w io.Writer, rc *config.RunConfig, e model.Engine) {
	global := filter.ParseGlob(rc.Filters.FileFilter)
	switch e {
	case model.EngineIaC:
		tr := filter.ToKICSExcludePaths(global, filter.ParseGlob(rc.Filters.IaC))
		if len(tr.Patterns) > 0 {
			fmt.Fprintf(w, "  filter -> --exclude-paths: %s\n", strings.Join(tr.Patterns, ","))
		}
		printWarnings(w, tr.Warnings)
	case model.EngineSecrets:
		tr := filter.ToSecretsIgnorePatterns(global, filter.ParseGlob(rc.Filters.Secrets))
		if len(tr.Patterns) > 0 {
			fmt.Fprintf(w, "  filter -> --ignore-pattern: %s\n", strings.Join(tr.Patterns, ","))
		}
		printWarnings(w, tr.Warnings)
	case model.EngineSAST:
		names := filter.ToCxSASTNames(global, filter.ParseGlob(rc.Filters.SAST))
		if len(names.Folders) > 0 {
			fmt.Fprintf(w, "  filter -> -LocationPathExclude: %s\n", strings.Join(names.Folders, ","))
		}
		if len(names.Files) > 0 {
			fmt.Fprintf(w, "  filter -> -LocationFilesExclude: %s\n", strings.Join(names.Files, ","))
		}
		printWarnings(w, names.Warnings)
	case model.EngineSCA:
		if rc.Filters.SCA != "" {
			fmt.Fprintf(w, "  filter -> cx --sca-filter (regex): %s\n", rc.Filters.SCA)
		}
	case model.EngineContainers:
		if rc.Filters.ContainersFileFolder != "" {
			fmt.Fprintf(w, "  filter -> --containers-file-folder-filter: %s\n", rc.Filters.ContainersFileFolder)
		}
		if rc.Filters.ContainersPackage != "" {
			fmt.Fprintf(w, "  filter -> --containers-package-filter (regex): %s\n", rc.Filters.ContainersPackage)
		}
		if rc.Filters.ContainersImageTag != "" {
			fmt.Fprintf(w, "  filter -> --containers-image-tag-filter (wildcard): %s\n", rc.Filters.ContainersImageTag)
		}
	}
}

func printWarnings(w io.Writer, warns []string) {
	for _, x := range warns {
		fmt.Fprintf(w, "  WARN: %s\n", x)
	}
}

func routeDesc(p threshold.Plan) string {
	if !p.HasClauses() {
		return "none (no threshold for this engine)"
	}
	switch p.Route {
	case model.RoutePassthrough:
		return "pass-through (native flag enforces)"
	case model.RouteWrapperSide:
		return "wrapper-side (parse reports, count >= limit)"
	case model.RouteSeverityGate:
		return "severity-gate (--fail-on)"
	default:
		return string(p.Route)
	}
}

func joinEngines(es []model.Engine) string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = string(e)
	}
	return strings.Join(parts, ", ")
}

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Resolve and validate config, then print the native plan without scanning",
		Args:  cobra.NoArgs,
	}
	b := registerRunFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		rc, err := resolveConfig(b)
		if err != nil {
			return err
		}
		printPlan(cmd.OutOrStdout(), rc)
		return nil
	}
	return cmd
}
