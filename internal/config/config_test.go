package config

import (
	"strings"
	"testing"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/ci"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
)

func testEnv(m map[string]string) EnvFunc {
	return func(k string) string { return m[k] }
}

func TestSecretValues(t *testing.T) {
	rc := &RunConfig{EngineConfigs: map[model.Engine]*scanner.Config{
		model.EngineSAST: {SASTPasswordEnv: "PW", SASTTokenEnv: "TOK"},
		model.EngineSCA:  {CxAPIKeyEnv: "KEY", CxClientSecretEnv: "CS"},
	}}
	// Synthetic, obviously-non-credential test values (length >= 4 to pass the filter).
	env := func(k string) string {
		return map[string]string{"PW": "alpha-aaaa", "TOK": "bc", "KEY": "beta-bbbb", "CS": "gamma-cccc"}[k]
	}
	got := SecretValues(rc, env)
	has := func(v string) bool {
		for _, x := range got {
			if x == v {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"alpha-aaaa", "beta-bbbb", "gamma-cccc"} {
		if !has(want) {
			t.Errorf("SecretValues missing %q: %v", want, got)
		}
	}
	if has("bc") { // below the 4-char floor, excluded to avoid mangling output
		t.Errorf("short value should be excluded: %v", got)
	}
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

func TestResolve_ReportsKeptOutsideScannedTree(t *testing.T) {
	env := testEnv(nil)
	ci := ci.Context{Provider: ci.ProviderLocal}

	// Default (no --output-path): reports default OUTSIDE the source, so no engine
	// exclusion is needed and no warning is raised.
	rc, err := Resolve(Flags{Scanners: "kics", Source: "."}, env, ci)
	if err != nil {
		t.Fatal(err)
	}
	if rc.ReportsExcludeRel != "" {
		t.Errorf("default output should be outside source (no exclusion), got rel=%q path=%q", rc.ReportsExcludeRel, rc.Output.Path)
	}
	if strings.Contains(rc.Output.Path, "/.") || rc.Output.Path == "" {
		t.Errorf("unexpected default output path %q", rc.Output.Path)
	}

	// Explicit output INSIDE the source: excluded from scans + warned.
	rc2, err := Resolve(Flags{Scanners: "kics,secrets,sast", Source: ".", OutputPath: "reports"}, env, ci)
	if err != nil {
		t.Fatal(err)
	}
	if rc2.ReportsExcludeRel != "reports" {
		t.Errorf("in-source output should set exclusion to %q, got %q", "reports", rc2.ReportsExcludeRel)
	}
	for _, e := range rc2.Scanners {
		if got := rc2.EngineConfigs[e].ReportsExcludePath; got != "reports" {
			t.Errorf("%s cfg.ReportsExcludePath = %q, want reports", e, got)
		}
	}
	if !hasWarn(rc2.Warnings, "INSIDE the scanned source") {
		t.Errorf("expected an inside-source warning, got %v", rc2.Warnings)
	}

	// Explicit output OUTSIDE the source: no exclusion.
	rc3, err := Resolve(Flags{Scanners: "kics", Source: ".", OutputPath: "/tmp/cxoo-out-test"}, env, ci)
	if err != nil {
		t.Fatal(err)
	}
	if rc3.ReportsExcludeRel != "" {
		t.Errorf("absolute out-of-tree output should not be excluded, got %q", rc3.ReportsExcludeRel)
	}
}

func hasWarn(ws []string, sub string) bool {
	for _, w := range ws {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
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
