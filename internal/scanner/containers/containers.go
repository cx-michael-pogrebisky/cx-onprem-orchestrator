// Package containers implements the Cx1 Container Security scanner driven through
// the cx (ast-cli) CLI: `cx scan create --scan-types container-security`.
// Thresholds are PASS-THROUGH to `cx --threshold "containers-<sev>=N"`. Container
// filters preserve their native types: file/folder (glob), package (regex), image
// tag (wildcard). Auth (CX_APIKEY) is injected into the child env by name.
package containers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exit"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

const reportName = "containers"

func init() {
	scanner.Register(model.EngineContainers, func() scanner.Scanner { return &Scanner{} })
}

// Scanner runs Cx1 Container Security.
type Scanner struct{}

func (s *Scanner) Engine() model.Engine { return model.EngineContainers }

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
			return fmt.Errorf("cx CLI not found (set --containers-path): %w", err)
		}
	}
	if os.Getenv(cfg.CxAPIKeyEnv) == "" {
		return fmt.Errorf("Cx1 API key not set: env %s is empty", cfg.CxAPIKeyEnv)
	}
	return nil
}

func (s *Scanner) BuildInvocation(cfg *scanner.Config, th threshold.Plan) (*model.Invocation, error) {
	// cx requires -s even for image-only scans.
	args := []string{
		"scan", "create",
		"--project-name", cfg.ProjectName,
		"-s", cfg.Source,
		"--scan-types", "container-security",
	}
	if cfg.Branch != "" {
		args = append(args, "--branch", cfg.Branch)
	}
	if imgs := cfg.Extra["containerImages"]; imgs != "" {
		args = append(args, "--container-images", imgs)
	}
	// Filters (native types preserved verbatim).
	if cfg.ContainersFileFolderFilter != "" {
		args = append(args, "--containers-file-folder-filter", cfg.ContainersFileFolderFilter)
	}
	if cfg.ContainersPackageFilter != "" {
		args = append(args, "--containers-package-filter", cfg.ContainersPackageFilter)
	}
	if cfg.ContainersImageTagFilter != "" {
		args = append(args, "--containers-image-tag-filter", cfg.ContainersImageTagFilter)
	}
	if cfg.FileFilter != "" {
		args = append(args, "--file-filter", cfg.FileFilter)
	}
	// Threshold pass-through.
	if th.HasClauses() {
		args = append(args, "--threshold", th.NativeThresholdString())
	}
	if cfg.CxBaseURI != "" {
		args = append(args, "--base-uri", cfg.CxBaseURI)
	}
	if cfg.CxTenant != "" {
		args = append(args, "--tenant", cfg.CxTenant)
	}
	args = append(args,
		"--report-format", "json",
		"--output-name", reportName,
		"--output-path", cfg.OutputDir,
	)
	if cfg.Async {
		args = append(args, "--async")
	}
	args = append(args, cfg.RawArgs...)

	inv := &model.Invocation{
		Engine:    model.EngineContainers,
		Path:      cxBinary(cfg),
		Args:      args,
		OutputDir: cfg.OutputDir,
	}
	if val := os.Getenv(cfg.CxAPIKeyEnv); val != "" {
		inv.Env = append(inv.Env, "CX_APIKEY="+val)
		inv.EnvKeys = append(inv.EnvKeys, "CX_APIKEY")
	}
	return inv, nil
}

func (s *Scanner) Run(ctx context.Context, inv *model.Invocation, opts execpkg.Options) *model.Result {
	r := scanner.RunInvocation(ctx, inv, opts)
	r.Route = model.RoutePassthrough
	p := filepath.Join(inv.OutputDir, reportName+".json")
	if _, err := os.Stat(p); err == nil {
		r.ReportPaths = []string{p}
	}
	return r
}

func (s *Scanner) ParseResults(_ context.Context, _ *model.Result) error { return nil }

func (s *Scanner) Evaluate(r *model.Result, th threshold.Plan) model.Verdict {
	v := model.Verdict{Engine: model.EngineContainers}
	r.NativeGated = th.HasClauses()
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
		v.Message = "cx reported a container-security threshold breach (native gate)"
	case model.CatInterrupted:
		v.Category = model.CatInterrupted
	default:
		v.Category = model.CatEngineFailure
		v.Message = fmt.Sprintf("cx container-security engine error (exit %d)", r.ChildExitCode)
	}
	return v
}
