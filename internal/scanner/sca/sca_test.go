package sca

import (
	"strings"
	"testing"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

func TestBuildInvocation_PassThrough(t *testing.T) {
	t.Setenv("CX1_APIKEY", "secret-token-value")
	cfg := &scanner.Config{
		Engine:       model.EngineSCA,
		Source:       "./app",
		ProjectName:  "demo",
		Branch:       "main",
		OutputDir:    "out/sca",
		SCAFilter:    "(?i).*test.*",
		FileFilter:   "!**/**,**/src/**",
		ResolverArgs: []string{"--excludes", "**/generated/**"},
		CxAPIKeyEnv:  "CX1_APIKEY",
		RawArgs:      []string{"--project-tags=team:sec"},
		Extra:        map[string]string{"scaResolverPath": "/opt/sca/ScaResolver"},
	}
	set, _ := threshold.Parse("sca-high=5;sca-medium=10")
	plan := set.Partition(model.EngineSCA)

	s := &Scanner{}
	inv, err := s.BuildInvocation(cfg, plan)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	line := inv.Path + " " + strings.Join(inv.Args, " ")
	for _, want := range []string{
		"cx scan create",
		"--project-name demo",
		"-s ./app",
		"--scan-types sca",
		"--sca-resolver /opt/sca/ScaResolver",
		"--sca-resolver-params --excludes **/generated/**",
		"--sca-filter (?i).*test.*",
		"--file-filter !**/**,**/src/**",
		"--threshold sca-high=5;sca-medium=10",
		"--report-format json",
		"--output-name sca",
		"--project-tags=team:sec", // raw passthrough
	} {
		if !strings.Contains(line, want) {
			t.Errorf("sca invocation missing %q\n got: %s", want, line)
		}
	}
	// The secret value must never appear in argv.
	if strings.Contains(line, "secret-token-value") {
		t.Errorf("API key leaked into argv: %s", line)
	}
	// It must be injected as an env var by name instead.
	foundEnv := false
	for _, e := range inv.Env {
		if strings.HasPrefix(e, "CX_APIKEY=") {
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Errorf("expected CX_APIKEY injected into child env, got %v", inv.EnvKeys)
	}
}

func TestEvaluate_PassThroughCodes(t *testing.T) {
	s := &Scanner{}
	set, _ := threshold.Parse("sca-high=5")
	plan := set.Partition(model.EngineSCA)

	// cx exit 1 with a clause -> breach.
	v := s.Evaluate(&model.Result{Engine: model.EngineSCA, ChildExitCode: 1}, plan)
	if v.Category != model.CatThresholdBreach {
		t.Errorf("exit 1 + clause should be breach, got %+v", v)
	}
	// cx exit 0 -> pass.
	v = s.Evaluate(&model.Result{Engine: model.EngineSCA, ChildExitCode: 0}, plan)
	if !v.Pass {
		t.Errorf("exit 0 should pass, got %+v", v)
	}
	// cx exit 3 (SCA engine fail) -> engine failure.
	v = s.Evaluate(&model.Result{Engine: model.EngineSCA, ChildExitCode: 3}, plan)
	if v.Category != model.CatEngineFailure {
		t.Errorf("exit 3 should be engine failure, got %+v", v)
	}
}
