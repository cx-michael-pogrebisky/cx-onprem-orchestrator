package kics

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

func argLine(inv *model.Invocation) string { return inv.Path + " " + strings.Join(inv.Args, " ") }

func TestBuildInvocation_Docker(t *testing.T) {
	s := &Scanner{}
	cfg := &scanner.Config{
		Engine:        model.EngineIaC,
		Mode:          "docker",
		Source:        ".",
		OutputDir:     "out/iac",
		ReportFormats: []string{"json", "sarif"},
		FileFilter:    "!**/**,**/src/**",
		IaCFilter:     "!**/vendor/**",
		RawArgs:       []string{"--exclude-categories=Encryption"},
	}
	inv, err := s.BuildInvocation(cfg, threshold.Plan{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	line := argLine(inv)
	for _, want := range []string{
		"docker run --rm",
		":/path:ro",
		":/output",
		"checkmarx/kics@sha256:", // digest-pinned via manifest.lock
		"scan",
		"--ignore-on-exit results", // wrapper gates; child must not fail on findings
		"--exclude-paths **/vendor/**",
		"-p /path -o /output",
		"--exclude-categories=Encryption", // raw passthrough appended last
	} {
		if !strings.Contains(line, want) {
			t.Errorf("docker invocation missing %q\n got: %s", want, line)
		}
	}
	// The include-all sentinel must NOT leak as a literal exclude.
	if strings.Contains(line, "exclude-paths **/**") {
		t.Errorf("match-all sentinel leaked into exclude-paths: %s", line)
	}
}

func TestBuildInvocation_ExcludesReportsDir(t *testing.T) {
	s := &Scanner{}
	cfg := &scanner.Config{
		Engine: model.EngineIaC, Mode: "docker", Source: ".", OutputDir: "reports/iac",
		ReportFormats: []string{"json"}, ReportsExcludePath: "reports", ReportsExcludeName: "reports",
	}
	inv, err := s.BuildInvocation(cfg, threshold.Plan{})
	if err != nil {
		t.Fatal(err)
	}
	line := argLine(inv)
	if !strings.Contains(line, "--exclude-paths") || !strings.Contains(line, "reports/**") {
		t.Errorf("KICS must exclude its own reports dir, got: %s", line)
	}
}

func TestParseResults(t *testing.T) {
	dir := t.TempDir()
	report := `{"total_counter":7,"severity_counters":{"CRITICAL":1,"HIGH":3,"MEDIUM":2,"LOW":1,"INFO":4,"TRACE":9}}`
	if err := os.WriteFile(filepath.Join(dir, reportName+".json"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Scanner{}
	r := &model.Result{Engine: model.EngineIaC, OutputDir: dir}
	if err := s.ParseResults(context.Background(), r); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Counts[model.SevCritical] != 1 || r.Counts[model.SevHigh] != 3 || r.Counts[model.SevLow] != 1 {
		t.Errorf("unexpected counts: %v", r.Counts)
	}
	// TRACE is not a model severity and must be ignored.
	if _, ok := r.Counts["trace"]; ok {
		t.Errorf("TRACE should be ignored, got %v", r.Counts)
	}
}

func TestEvaluate_WrapperSideGating(t *testing.T) {
	s := &Scanner{}
	set, _ := threshold.Parse("iac-security-high=1")
	plan := set.Partition(model.EngineIaC)

	// Breach: 3 high >= 1.
	r := &model.Result{Engine: model.EngineIaC, ChildExitCode: 0, Counts: model.SeverityCount{model.SevHigh: 3}}
	v := s.Evaluate(r, plan)
	if v.Category != model.CatThresholdBreach || len(v.Breaches) != 1 {
		t.Errorf("expected breach, got %+v", v)
	}

	// Pass: 0 high.
	r = &model.Result{Engine: model.EngineIaC, ChildExitCode: 0, Counts: model.SeverityCount{model.SevHigh: 0}}
	v = s.Evaluate(r, plan)
	if !v.Pass {
		t.Errorf("expected pass, got %+v", v)
	}

	// Engine error (exit 126) -> failure regardless of counts.
	r = &model.Result{Engine: model.EngineIaC, ChildExitCode: 126}
	v = s.Evaluate(r, plan)
	if v.Category != model.CatEngineFailure {
		t.Errorf("expected engine failure for exit 126, got %+v", v)
	}
}
