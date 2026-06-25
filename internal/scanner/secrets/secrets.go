// Package secrets implements the 2ms (Too Many Secrets) scanner. 2ms has no
// severity model and no count threshold, so cx-onprem-orchestrator gates it
// WRAPPER-SIDE on a single "secrets-total" bucket: it forces a JSON report,
// reads totalSecretsFound, and applies secrets-total=<N> (inclusive >=). When no
// secrets threshold is set, ANY secret fails (secrets-specific default). 2ms runs
// natively if present, else via the checkmarx/2ms docker image.
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/filter"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/resolve"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

// DefaultImage is the docker image used in docker mode (M5 pins to a digest).
const DefaultImage = "checkmarx/2ms:latest"

const reportName = "2ms"

// twomsFormats are the report formats 2ms can emit (inferred from file extension).
var twomsFormats = map[string]bool{"json": true, "yaml": true, "sarif": true}

func init() {
	scanner.Register(model.EngineSecrets, func() scanner.Scanner { return &Scanner{} })
}

// Scanner runs 2ms.
type Scanner struct{}

func (s *Scanner) Engine() model.Engine { return model.EngineSecrets }

func (s *Scanner) mode(cfg *scanner.Config) string {
	if cfg.Mode != "" {
		return cfg.Mode
	}
	if _, err := exec.LookPath("2ms"); err == nil {
		return "native"
	}
	return "docker"
}

func (s *Scanner) Available(_ context.Context, cfg *scanner.Config) error {
	switch s.mode(cfg) {
	case "native":
		bin := cfg.Path
		if bin == "" {
			bin = "2ms"
		}
		if _, err := exec.LookPath(bin); err != nil {
			if _, statErr := os.Stat(bin); statErr != nil {
				return fmt.Errorf("2ms binary not found (set --secrets-path or --secrets-mode docker): %w", err)
			}
		}
	case "docker":
		if _, err := exec.LookPath("docker"); err != nil {
			return fmt.Errorf("docker not found for 2ms docker mode: %w", err)
		}
	default:
		return fmt.Errorf("invalid --secrets-mode %q (want native|docker)", cfg.Mode)
	}
	return nil
}

func (s *Scanner) BuildInvocation(cfg *scanner.Config, _ threshold.Plan) (*model.Invocation, error) {
	tr := filter.ToSecretsIgnorePatterns(filter.ParseGlob(cfg.FileFilter), filter.ParseGlob(cfg.SecretsFilter))
	// 2ms infers report format from the file extension; json is mandatory (parsing).
	formats, fmtWarn := scanner.SelectFormats(cfg.ReportFormats, twomsFormats, "json")

	inv := &model.Invocation{
		Engine:    model.EngineSecrets,
		OutputDir: cfg.OutputDir,
		Warnings:  append(append([]string{}, tr.Warnings...), fmtWarn...),
	}

	switch s.mode(cfg) {
	case "native":
		bin := cfg.Path
		if bin == "" {
			bin = "2ms"
		}
		args := []string{"filesystem", "--path", cfg.Source, "--ignore-on-exit", "results"}
		for _, f := range formats {
			args = append(args, "--report-path", filepath.Join(cfg.OutputDir, reportName+"."+f))
		}
		for _, p := range tr.Patterns {
			args = append(args, "--ignore-pattern", p)
		}
		args = append(args, cfg.RawArgs...)
		inv.Path = bin
		inv.Args = args
	default: // docker
		srcAbs, err := filepath.Abs(cfg.Source)
		if err != nil {
			return nil, fmt.Errorf("resolve source: %w", err)
		}
		outAbs, err := filepath.Abs(cfg.OutputDir)
		if err != nil {
			return nil, fmt.Errorf("resolve output: %w", err)
		}
		image, _ := resolve.Image("secrets", cfg.Image)
		if image == "" {
			image = DefaultImage
		}
		inv.UsesDocker = true
		inv.DockerMounts = []model.Mount{
			{HostPath: srcAbs, ContainerPath: "/repo", ReadOnly: true},
			{HostPath: outAbs, ContainerPath: "/output"},
		}
		args := []string{
			"run", "--rm",
			"-u", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), // write reports as the host user
			"-v", srcAbs + ":/repo:ro",
			"-v", outAbs + ":/output",
			image,
			"filesystem", "--path", "/repo",
			"--ignore-on-exit", "results",
		}
		for _, f := range formats {
			args = append(args, "--report-path", "/output/"+reportName+"."+f)
		}
		for _, p := range tr.Patterns {
			args = append(args, "--ignore-pattern", p)
		}
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
	if m, _ := filepath.Glob(filepath.Join(inv.OutputDir, reportName+".*")); len(m) > 0 {
		r.ReportPaths = m
	}
	return r
}

// twomsReport is the subset of the 2ms JSON report we parse.
type twomsReport struct {
	TotalItemsScanned int `json:"totalItemsScanned"`
	TotalSecretsFound int `json:"totalSecretsFound"`
}

func (s *Scanner) ParseResults(_ context.Context, r *model.Result) error {
	jsonPath := filepath.Join(r.OutputDir, reportName+".json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("read 2ms report %s: %w", jsonPath, err)
	}
	var rep twomsReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return fmt.Errorf("parse 2ms report: %w", err)
	}
	r.Counts = model.SeverityCount{model.SevTotal: rep.TotalSecretsFound}
	return nil
}

func (s *Scanner) Evaluate(r *model.Result, th threshold.Plan) model.Verdict {
	v := model.Verdict{Engine: model.EngineSecrets}
	if r.Err != nil {
		v.Category = model.CatEngineFailure
		v.Message = r.Err.Error()
		return v
	}
	switch exit.Interpret2ms(r.ChildExitCode) {
	case model.CatEngineFailure:
		v.Category = model.CatEngineFailure
		v.Message = fmt.Sprintf("2ms engine error (exit %d)", r.ChildExitCode)
		return v
	case model.CatInterrupted:
		v.Category = model.CatInterrupted
		return v
	}
	count := r.Counts[model.SevTotal]
	if th.HasClauses() {
		if breaches := threshold.Enforce(th, r.Counts); len(breaches) > 0 {
			v.Category = model.CatThresholdBreach
			v.Breaches = breaches
			return v
		}
	} else if count >= 1 {
		// No explicit threshold: any secret fails (secrets-specific default).
		v.Category = model.CatThresholdBreach
		v.Breaches = []model.BreachDetail{{Severity: model.SevTotal, Limit: 1, Actual: count}}
		return v
	}
	v.Pass = true
	v.Category = model.CatPass
	return v
}
