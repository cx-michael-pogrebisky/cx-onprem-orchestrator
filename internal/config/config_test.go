package config

import (
	"testing"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/ci"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

func testEnv(m map[string]string) EnvFunc {
	return func(k string) string { return m[k] }
}

func TestResolve_Defaults(t *testing.T) {
	f := Flags{
		Scanners:  "sast,sca",
		Threshold: "sast-high=10;sca-high=5",
	}
	env := testEnv(map[string]string{"CXSAST_URL": "https://cxsast.lab"})
	ciCtx := ci.Context{Provider: ci.ProviderGitHub, Branch: "main", Workspace: "/ws", Repo: "https://github.com/org/app"}

	rc, err := Resolve(f, env, ciCtx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(rc.Scanners) != 2 {
		t.Fatalf("want 2 engines, got %v", rc.Scanners)
	}
	if rc.Branch != "main" {
		t.Errorf("branch should fall back to CI: %q", rc.Branch)
	}
	if rc.Source != "/ws" {
		t.Errorf("source should fall back to CI workspace: %q", rc.Source)
	}
	if rc.ProjectName != "app" {
		t.Errorf("project name should derive from repo basename: %q", rc.ProjectName)
	}
	// Auth defaults match the user's env-var names.
	if rc.Auth.CxAPIKeyEnv != DefaultCxAPIKeyEnv {
		t.Errorf("cx apikey env default = %q", rc.Auth.CxAPIKeyEnv)
	}
	if rc.Auth.SASTServer != "https://cxsast.lab" {
		t.Errorf("SAST server should default from $CXSAST_URL, got %q", rc.Auth.SASTServer)
	}
	// Output defaults.
	if rc.Output.Path == "" || rc.Output.IgnoreOnExit != "none" {
		t.Errorf("unexpected output defaults: %+v", rc.Output)
	}
	if err := Validate(rc); err != nil {
		t.Errorf("validate should pass: %v", err)
	}
}

func TestResolve_EngineConfigsAndRawArgs(t *testing.T) {
	f := Flags{
		Scanners: "kics",
		RawArgs:  map[string][]string{"kics": {"--exclude-categories=Encryption"}},
	}
	rc, err := Resolve(f, testEnv(nil), ci.Context{Provider: ci.ProviderLocal})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	cfg := rc.EngineConfigs[model.EngineIaC]
	if cfg == nil {
		t.Fatal("missing IaC engine config")
	}
	if len(cfg.RawArgs) != 1 || cfg.RawArgs[0] != "--exclude-categories=Encryption" {
		t.Errorf("raw args not threaded through: %v", cfg.RawArgs)
	}
	if cfg.OutputDir == "" {
		t.Errorf("output dir should be set")
	}
}

func TestResolve_PerEngineReportFormatsOverride(t *testing.T) {
	f := Flags{
		Scanners:              "sca,kics",
		ReportFormats:         "json,sarif",
		ReportFormatsOverride: map[string]string{"sca": "json"},
	}
	rc, err := Resolve(f, testEnv(nil), ci.Context{Provider: ci.ProviderLocal})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	sca := rc.EngineConfigs[model.EngineSCA]
	if sca == nil || len(sca.ReportFormats) != 1 || sca.ReportFormats[0] != "json" {
		t.Errorf("sca should be overridden to [json], got %v", sca.ReportFormats)
	}
	kics := rc.EngineConfigs[model.EngineIaC]
	if kics == nil || len(kics.ReportFormats) != 2 {
		t.Errorf("kics should keep the global [json,sarif], got %v", kics.ReportFormats)
	}
}

func TestValidate_ConflictDetection(t *testing.T) {
	// Raw -SASTHigh collides with the managed cap derived from --threshold sast-high.
	f := Flags{
		Scanners:  "sast",
		Threshold: "sast-high=10",
		RawArgs:   map[string][]string{"sast": {"-SASTHigh=20"}},
		Conflict:  "error",
	}
	rc, err := Resolve(f, testEnv(nil), ci.Context{Provider: ci.ProviderLocal})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := Validate(rc); err == nil {
		t.Errorf("expected a Tier-A/Tier-B conflict error")
	}

	// With raw-wins, the same config validates (conflict resolved by policy).
	f.Conflict = "raw-wins"
	rc2, _ := Resolve(f, testEnv(nil), ci.Context{Provider: ci.ProviderLocal})
	if err := Validate(rc2); err != nil {
		t.Errorf("raw-wins should not error: %v", err)
	}
}

func TestValidate_DASTRejected(t *testing.T) {
	rc, err := Resolve(Flags{Scanners: "sast,dast"}, testEnv(nil), ci.Context{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := Validate(rc); err == nil {
		t.Errorf("dast should be rejected in v1")
	}
}
