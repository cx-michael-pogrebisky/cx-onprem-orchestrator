package containers

import (
	"strings"
	"testing"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

func TestBuildInvocation_PassThrough(t *testing.T) {
	t.Setenv("CX1_APIKEY", "secret-value")
	cfg := &scanner.Config{
		Engine:                     model.EngineContainers,
		Source:                     ".",
		ProjectName:                "demo",
		Branch:                     "main",
		OutputDir:                  "out/containers",
		ContainersFileFolderFilter: "!*.log",
		ContainersPackageFilter:    "^internal-.*",
		ContainersImageTagFilter:   "!*:dev",
		CxAPIKeyEnv:                "CX1_APIKEY",
		Extra:                      map[string]string{"containerImages": "myorg/app:1.4.2,debian:10"},
	}
	set, _ := threshold.Parse("containers-high=3")
	plan := set.Partition(model.EngineContainers)

	inv, err := (&Scanner{}).BuildInvocation(cfg, plan)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	line := inv.Path + " " + strings.Join(inv.Args, " ")
	for _, want := range []string{
		"cx scan create",
		"--scan-types container-security",
		"--container-images myorg/app:1.4.2,debian:10",
		"--containers-file-folder-filter !*.log",
		"--containers-package-filter ^internal-.*",
		"--containers-image-tag-filter !*:dev",
		"--threshold containers-high=3",
		"--report-format json",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q\n got: %s", want, line)
		}
	}
	if strings.Contains(line, "secret-value") {
		t.Errorf("API key leaked into argv")
	}
	found := false
	for _, e := range inv.EnvKeys {
		if e == "CX_APIKEY" {
			found = true
		}
	}
	if !found {
		t.Errorf("CX_APIKEY should be injected by name, got %v", inv.EnvKeys)
	}
}

func TestEvaluate(t *testing.T) {
	s := &Scanner{}
	set, _ := threshold.Parse("containers-high=3")
	plan := set.Partition(model.EngineContainers)
	// exit 1 + clause -> breach.
	if v := s.Evaluate(&model.Result{Engine: model.EngineContainers, ChildExitCode: 1}, plan); v.Category != model.CatThresholdBreach {
		t.Errorf("exit 1 + clause should breach, got %+v", v)
	}
	// exit 0 -> pass.
	if v := s.Evaluate(&model.Result{Engine: model.EngineContainers, ChildExitCode: 0}, plan); !v.Pass {
		t.Errorf("exit 0 should pass")
	}
}
