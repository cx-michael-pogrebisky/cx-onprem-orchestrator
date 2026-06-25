package secrets

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

func TestBuildInvocation_Docker(t *testing.T) {
	cfg := &scanner.Config{
		Engine:        model.EngineSecrets,
		Mode:          "docker",
		Source:        ".",
		OutputDir:     "out/secrets",
		FileFilter:    "!**/testdata/**",
		SecretsFilter: "!**/*.lock",
	}
	inv, err := (&Scanner{}).BuildInvocation(cfg, threshold.Plan{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	line := inv.Path + " " + strings.Join(inv.Args, " ")
	for _, want := range []string{
		"docker run --rm",
		"-u ", // run as host user so reports are writable
		":/repo:ro",
		":/output",
		"checkmarx/2ms@sha256:", // digest-pinned via manifest.lock
		"filesystem --path /repo",
		"--report-path /output/2ms.json",
		"--ignore-on-exit results",
		"--ignore-pattern **/testdata/**",
		"--ignore-pattern **/*.lock",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q\n got: %s", want, line)
		}
	}
}

func TestParseResults(t *testing.T) {
	dir := t.TempDir()
	rep := `{"totalItemsScanned":3,"totalSecretsFound":2,"results":{"a":[{}],"b":[{}]}}`
	if err := os.WriteFile(filepath.Join(dir, reportName+".json"), []byte(rep), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &model.Result{Engine: model.EngineSecrets, OutputDir: dir, ReportPaths: []string{filepath.Join(dir, reportName+".json")}}
	if err := (&Scanner{}).ParseResults(context.Background(), r); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Counts[model.SevTotal] != 2 {
		t.Errorf("want total=2, got %v", r.Counts)
	}
}

func TestEvaluate(t *testing.T) {
	s := &Scanner{}

	// No threshold: any secret fails.
	r := &model.Result{Engine: model.EngineSecrets, ChildExitCode: 0, Counts: model.SeverityCount{model.SevTotal: 2}}
	if v := s.Evaluate(r, threshold.Plan{}); v.Category != model.CatThresholdBreach {
		t.Errorf("no-threshold + secrets should breach, got %+v", v)
	}
	// No threshold, zero secrets: pass.
	r = &model.Result{Engine: model.EngineSecrets, ChildExitCode: 0, Counts: model.SeverityCount{model.SevTotal: 0}}
	if v := s.Evaluate(r, threshold.Plan{}); !v.Pass {
		t.Errorf("no secrets should pass, got %+v", v)
	}
	// Threshold secrets-total=5, found 2: pass.
	set, _ := threshold.Parse("secrets-total=5")
	plan := set.Partition(model.EngineSecrets)
	r = &model.Result{Engine: model.EngineSecrets, ChildExitCode: 0, Counts: model.SeverityCount{model.SevTotal: 2}}
	if v := s.Evaluate(r, plan); !v.Pass {
		t.Errorf("2 < 5 should pass, got %+v", v)
	}
	// Threshold secrets-total=2, found 2: inclusive breach.
	set, _ = threshold.Parse("secrets-total=2")
	plan = set.Partition(model.EngineSecrets)
	if v := s.Evaluate(r, plan); v.Category != model.CatThresholdBreach {
		t.Errorf("2 >= 2 should breach, got %+v", v)
	}
	// Engine error (exit 1).
	if v := s.Evaluate(&model.Result{Engine: model.EngineSecrets, ChildExitCode: 1}, threshold.Plan{}); v.Category != model.CatEngineFailure {
		t.Errorf("exit 1 should be engine failure, got %+v", v)
	}
}
