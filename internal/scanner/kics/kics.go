// Package kics implements the KICS (IaC) scanner. KICS has no per-severity count
// threshold (only --fail-on by severity), so cx-onprem-orchestrator gates KICS
// WRAPPER-SIDE: it forces JSON output, parses the severity counters, and applies
// the iac-security-<sev> thresholds with cx-exact inclusive (>=) semantics. KICS
// runs natively if the binary is present, else via the checkmarx/kics docker image.
package kics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/filter"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/resolve"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

// DefaultImage is the docker image used in docker mode. M5 replaces the tag with
// a pinned sha256 digest from manifest.lock.
const DefaultImage = "checkmarx/kics:latest"

const reportName = "kics"

func init() {
	scanner.Register(model.EngineIaC, func() scanner.Scanner { return &Scanner{} })
}

// Scanner runs KICS.
type Scanner struct{}

func (s *Scanner) Engine() model.Engine { return model.EngineIaC }

func (s *Scanner) mode(cfg *scanner.Config) string {
	if cfg.Mode != "" {
		return cfg.Mode
	}
	if _, err := exec.LookPath("kics"); err == nil {
		return "native"
	}
	return "docker"
}

func (s *Scanner) Available(_ context.Context, cfg *scanner.Config) error {
	switch s.mode(cfg) {
	case "native":
		bin := cfg.Path
		if bin == "" {
			bin = "kics"
		}
		if _, err := exec.LookPath(bin); err != nil {
			if _, statErr := os.Stat(bin); statErr != nil {
				return fmt.Errorf("kics binary not found (set --kics-path or --kics-mode docker): %w", err)
			}
		}
	case "docker":
		if _, err := exec.LookPath("docker"); err != nil {
			return fmt.Errorf("docker not found for kics docker mode: %w", err)
		}
	default:
		return fmt.Errorf("invalid --kics-mode %q (want native|docker)", cfg.Mode)
	}
	return nil
}

func (s *Scanner) BuildInvocation(cfg *scanner.Config, _ threshold.Plan) (*model.Invocation, error) {
	// KICS emits any subset of its native formats; json is mandatory (wrapper parsing).
	formats, fmtWarn := scanner.SelectEngineFormats(model.EngineIaC, cfg.ReportFormats)
	tr := filter.ToKICSExcludePaths(filter.ParseGlob(cfg.FileFilter), filter.ParseGlob(cfg.IaCFilter))
	warnings := append(append([]string{}, tr.Warnings...), fmtWarn...)

	// Common KICS args. We always gate wrapper-side, so --ignore-on-exit=results
	// keeps the findings-present codes (20-60) non-fatal at the child level.
	scanArgs := []string{
		"scan",
		"--report-formats", strings.Join(formats, ","),
		"--output-name", reportName,
		"--no-progress", "--ci",
		"--ignore-on-exit", "results",
	}
	excludes := append([]string{}, tr.Patterns...)
	// Never scan our own report directory (when it lives inside the scanned tree).
	if cfg.ReportsExcludePath != "" {
		excludes = append(excludes, cfg.ReportsExcludePath, cfg.ReportsExcludePath+"/**")
	}
	if len(excludes) > 0 {
		scanArgs = append(scanArgs, "--exclude-paths", strings.Join(excludes, ","))
	}

	inv := &model.Invocation{
		Engine:    model.EngineIaC,
		OutputDir: cfg.OutputDir,
		Warnings:  warnings,
	}

	switch s.mode(cfg) {
	case "native":
		bin := cfg.Path
		if bin == "" {
			bin = "kics"
		}
		inv.Path = bin
		args := append([]string{}, scanArgs...)
		args = append(args, "-p", cfg.Source, "-o", cfg.OutputDir)
		// In native mode kics needs its query assets: prefer --kics-queries
		// (cfg.Extra["kicsQueries"]), else $CXOO_KICS_QUERIES_PATH (set by the fat image).
		q := cfg.Extra["kicsQueries"]
		if q == "" {
			q = os.Getenv("CXOO_KICS_QUERIES_PATH")
		}
		if q != "" {
			args = append(args, "-q", q)
		}
		args = append(args, cfg.RawArgs...)
		inv.Args = args
	default: // docker
		srcAbs, err := filepath.Abs(cfg.Source)
		if err != nil {
			return nil, fmt.Errorf("resolve source path: %w", err)
		}
		outAbs, err := filepath.Abs(cfg.OutputDir)
		if err != nil {
			return nil, fmt.Errorf("resolve output path: %w", err)
		}
		image, _ := resolve.Image("kics", cfg.Image)
		if image == "" {
			image = DefaultImage
		}
		inv.UsesDocker = true
		inv.DockerMounts = []model.Mount{
			{HostPath: srcAbs, ContainerPath: "/path", ReadOnly: true},
			{HostPath: outAbs, ContainerPath: "/output"},
		}
		args := []string{
			"run", "--rm",
			"-v", srcAbs + ":/path:ro",
			"-v", outAbs + ":/output",
			image,
		}
		args = append(args, scanArgs...)
		args = append(args, "-p", "/path", "-o", "/output")
		args = append(args, cfg.RawArgs...)
		inv.Path = "docker"
		inv.Args = args
	}
	return inv, nil
}

func (s *Scanner) Run(ctx context.Context, inv *model.Invocation, opts execpkg.Options) *model.Result {
	r := scanner.RunInvocation(ctx, inv, opts)
	r.Route = model.RouteWrapperSide
	r.Warnings = inv.Warnings
	r.ReportPaths = collectReports(inv.OutputDir)
	return r
}

// kicsReport is the subset of the KICS JSON report we parse.
type kicsReport struct {
	SeverityCounters map[string]int `json:"severity_counters"`
	TotalCounter     int            `json:"total_counter"`
}

func (s *Scanner) ParseResults(_ context.Context, r *model.Result) error {
	path := filepath.Join(r.OutputDir, reportName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read kics report %s: %w", path, err)
	}
	var rep kicsReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return fmt.Errorf("parse kics report: %w", err)
	}
	counts := model.SeverityCount{}
	for k, v := range rep.SeverityCounters {
		if sev, ok := model.ParseSeverity(k); ok && sev.IsRealSeverity() {
			counts[sev] = v
		}
	}
	r.Counts = counts
	return nil
}

func (s *Scanner) Evaluate(r *model.Result, th threshold.Plan) model.Verdict {
	v := model.Verdict{Engine: model.EngineIaC}
	if r.Err != nil {
		v.Category = model.CatEngineFailure
		v.Message = r.Err.Error()
		return v
	}
	switch exit.InterpretKICS(r.ChildExitCode) {
	case model.CatEngineFailure:
		v.Category = model.CatEngineFailure
		v.Message = fmt.Sprintf("kics engine error (exit %d)", r.ChildExitCode)
		return v
	case model.CatInterrupted:
		v.Category = model.CatInterrupted
		return v
	}
	if th.HasClauses() {
		if breaches := threshold.Enforce(th, r.Counts); len(breaches) > 0 {
			v.Category = model.CatThresholdBreach
			v.Breaches = breaches
			return v
		}
	}
	v.Pass = true
	v.Category = model.CatPass
	return v
}

// kicsFormats are the report formats KICS can emit (native token == unified token).
// collectReports globs every artifact KICS wrote for our report base name.
func collectReports(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, reportName+".*"))
	return matches
}
