// Package sca implements the Cx1 SCA scanner driven through the cx (ast-cli) CLI
// in SCA Resolver mode: `cx scan create --scan-types sca --sca-resolver <bin>`.
// Thresholds are PASS-THROUGH to `cx --threshold "sca-<sev>=N"`; auth is injected
// into the child env (CX_APIKEY) by name, never argv.
package sca

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

const reportName = "sca"

func init() {
	scanner.Register(model.EngineSCA, func() scanner.Scanner { return &Scanner{} })
}

// Scanner runs Cx1 SCA via SCA Resolver.
type Scanner struct{}

func (s *Scanner) Engine() model.Engine { return model.EngineSCA }

func cxBinary(cfg *scanner.Config) string {
	if cfg.Path != "" {
		return cfg.Path
	}
	return "cx"
}

func (s *Scanner) Available(_ context.Context, cfg *scanner.Config) error {
	bin := cxBinary(cfg)
	if _, err := exec.LookPath(bin); err != nil {
		if _, statErr := os.Stat(bin); statErr != nil {
			return fmt.Errorf("cx CLI not found (set --sca-path): %w", err)
		}
	}
	resolver := cfg.Extra["scaResolverPath"]
	if resolver == "" {
		return fmt.Errorf("SCA Resolver mode requires --sca-resolver <path to ScaResolver>")
	}
	if _, err := os.Stat(resolver); err != nil {
		return fmt.Errorf("ScaResolver not found at %q: %w", resolver, err)
	}
	if !hasConfigSibling(resolver) {
		return fmt.Errorf("ScaResolver requires a Configuration.yml next to %q", resolver)
	}
	return scanner.CxAuthAvailable(cfg)
}

func (s *Scanner) BuildInvocation(cfg *scanner.Config, th threshold.Plan) (*model.Invocation, error) {
	resolver := cfg.Extra["scaResolverPath"]
	args := []string{
		"scan", "create",
		"--project-name", cfg.ProjectName,
		"-s", cfg.Source,
		"--scan-types", "sca",
		"--sca-resolver", resolver,
	}
	if cfg.Branch != "" {
		args = append(args, "--branch", cfg.Branch)
	}
	if len(cfg.ResolverArgs) > 0 {
		args = append(args, "--sca-resolver-params", strings.Join(cfg.ResolverArgs, " "))
	}
	// Filters: SCA filter is a regex (verbatim to cx); global file filters too.
	if cfg.SCAFilter != "" {
		args = append(args, "--sca-filter", cfg.SCAFilter)
	}
	if cfg.FileFilter != "" {
		args = append(args, "--file-filter", cfg.FileFilter)
	}
	if cfg.FileInclude != "" {
		args = append(args, "--file-include", cfg.FileInclude)
	}
	if cfg.UseGitignore {
		args = append(args, "--use-gitignore")
	}
	// Threshold pass-through.
	if th.HasClauses() {
		args = append(args, "--threshold", th.NativeThresholdString())
	}
	// Cx1 auth (API key or OAuth2 client-credentials): non-secret flags appended
	// here; secret values are injected into the child env below, never argv.
	authArgs, authEnv, authKeys, err := scanner.CxAuth(cfg)
	if err != nil {
		return nil, err
	}
	args = append(args, authArgs...)
	// Reports — cx emits any subset of its formats as a comma list (json mandatory).
	fmtArg, fmtWarn := scanner.CxReportFormats(cfg.ReportFormats)
	args = append(args,
		"--report-format", fmtArg,
		"--output-name", reportName,
		"--output-path", cfg.OutputDir,
	)
	if cfg.Async {
		args = append(args, "--async")
	}
	args = append(args, cfg.RawArgs...)

	inv := &model.Invocation{
		Engine:    model.EngineSCA,
		Path:      cxBinary(cfg),
		Args:      args,
		OutputDir: cfg.OutputDir,
		Env:       authEnv,
		EnvKeys:   authKeys,
		Warnings:  fmtWarn,
	}
	return inv, nil
}

func (s *Scanner) Run(ctx context.Context, inv *model.Invocation, opts execpkg.Options) *model.Result {
	r := scanner.RunInvocation(ctx, inv, opts)
	r.Route = model.RoutePassthrough
	r.Warnings = inv.Warnings
	if scaExportStalled(r) {
		r.Warnings = append(r.Warnings, exportStallMessage(r))
	}
	r.ReportPaths = collectReports(inv.OutputDir)
	return r
}

// scaExportStalled reports whether cx failed because the Cx1 SCA results/report
// export never completed, rather than because of a real threshold breach. cx
// gates the SCA threshold AND generates the report from the same Cx1 export
// service; when that service stalls it polls "SCA Export Status: Pending" until a
// hardcoded timeout and then exits non-zero. A per-engine --sca-timeout firing
// during that wait surfaces here as a context deadline.
func scaExportStalled(r *model.Result) bool {
	if r == nil {
		return false
	}
	if errors.Is(r.Err, context.DeadlineExceeded) {
		return true
	}
	out := string(r.Stdout) + "\n" + string(r.Stderr)
	for _, sig := range []string{
		"export generating failed", // cx: "export generating failed - Timed out after 5 minutes"
		"Failed listing results",
		"SCA Export Status is: Failed",
	} {
		if strings.Contains(out, sig) {
			return true
		}
	}
	return false
}

// exportStallMessage explains the stall honestly (this is a Checkmarx backend
// condition, not a code finding) and points at the mitigation.
func exportStallMessage(r *model.Result) string {
	base := "Cx1 SCA export did not complete: the scan ran on Cx1 but its results/report could not be retrieved, " +
		"so the SCA threshold could not be evaluated and no SCA report was written. This is a Checkmarx export-service " +
		"condition, not a code finding."
	if errors.Is(r.Err, context.DeadlineExceeded) {
		return base + " The per-engine --sca-timeout fired while waiting for the export."
	}
	return base + " Bound the wait with --sca-timeout and retry; raise with Checkmarx if it persists."
}

// ParseResults is best-effort for SCA (gating is native/pass-through). Counts are
// left to the cx report; we only record that a report exists.
func (s *Scanner) ParseResults(_ context.Context, _ *model.Result) error { return nil }

func (s *Scanner) Evaluate(r *model.Result, th threshold.Plan) model.Verdict {
	v := model.Verdict{Engine: model.EngineSCA}
	r.NativeGated = th.HasClauses()
	// A stalled/failed Cx1 export makes cx exit non-zero without ever evaluating
	// the gate; that is an engine failure, NOT a threshold breach. Check this
	// BEFORE InterpretCx so cx's overloaded exit 1 is never mislabeled as a breach.
	if scaExportStalled(r) {
		v.Category = model.CatEngineFailure
		v.Message = exportStallMessage(r)
		return v
	}
	if r.Err != nil {
		v.Category = model.CatEngineFailure
		v.Message = r.Err.Error()
		return v
	}
	switch exit.InterpretCx(r.ChildExitCode, th.HasClauses()) {
	case model.CatPass:
		v.Pass = true
		v.Category = model.CatPass
	case model.CatThresholdBreach:
		v.Category = model.CatThresholdBreach
		v.Message = "cx reported an SCA threshold breach (native gate)"
	case model.CatInterrupted:
		v.Category = model.CatInterrupted
	default:
		v.Category = model.CatEngineFailure
		v.Message = fmt.Sprintf("cx SCA engine error (exit %d)", r.ChildExitCode)
	}
	return v
}

func hasConfigSibling(resolver string) bool {
	dir := filepath.Dir(resolver)
	for _, name := range []string{"Configuration.yml", "configuration.yml", "Configuration.yaml", "configuration.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func collectReports(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, reportName+".*"))
	return matches
}
