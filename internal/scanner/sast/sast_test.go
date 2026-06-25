package sast

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

func fakePlugin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CxConsolePlugin-CLI-1.1.41.jar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runCxConsole.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildInvocation_CapsAndFilters(t *testing.T) {
	t.Setenv("CXSAST_USERNAME", "admin")
	t.Setenv("CXSAST_PASSWORD", "a,b!c")
	dir := fakePlugin(t)
	cfg := &scanner.Config{
		Engine:          model.EngineSAST,
		Path:            filepath.Join(dir, "runCxConsole.sh"),
		Source:          ".",
		ProjectName:     "demo",
		OutputDir:       filepath.Join(dir, "out"),
		SASTServer:      "http://cx",
		SASTUserEnv:     "CXSAST_USERNAME",
		SASTPasswordEnv: "CXSAST_PASSWORD",
		SASTFilter:      "!**/test/**,!**/*.min.js",
		Extra:           map[string]string{"sastJava": "/no/such/java"},
	}
	set, _ := threshold.Parse("sast-high=10;sast-critical=1")
	plan := set.Partition(model.EngineSAST)

	inv, err := (&Scanner{}).BuildInvocation(cfg, plan)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	line := strings.Join(inv.Args, " ")
	for _, want := range []string{
		"Scan",
		"-CxServer http://cx",
		"-ProjectName demo",
		"-LocationType folder",
		"-SASTHigh 9",     // 10 - 1 (off-by-one)
		"-SASTCritical 0", // 1 - 1
		"-LocationPathExclude test",
		"-LocationFilesExclude *.min.js",
		"-ReportXML",
		"-CxUser admin",
		"-CxPassword a,b!c",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q\n got: %s", want, line)
		}
	}
	if len(inv.Warnings) == 0 {
		t.Errorf("expected lossy-filter warnings")
	}
	if !strings.HasSuffix(inv.Path, "/no/such/java") {
		t.Errorf("java path = %q", inv.Path)
	}
}

func TestParseResults_XML(t *testing.T) {
	dir := t.TempDir()
	xmlDoc := `<?xml version="1.0"?>
<CxXMLResults>
  <Query name="SQL_Injection">
    <Result Severity="High"/>
    <Result Severity="High"/>
    <Result Severity="Critical"/>
  </Query>
  <Query name="XSS">
    <Result Severity="Medium"/>
    <Result Severity="Information"/>
  </Query>
</CxXMLResults>`
	p := filepath.Join(dir, reportName+".xml")
	if err := os.WriteFile(p, []byte(xmlDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = p
	r := &model.Result{Engine: model.EngineSAST, OutputDir: dir}
	if err := (&Scanner{}).ParseResults(context.Background(), r); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Counts[model.SevHigh] != 2 || r.Counts[model.SevCritical] != 1 || r.Counts[model.SevMedium] != 1 || r.Counts[model.SevInfo] != 1 {
		t.Errorf("unexpected counts: %v", r.Counts)
	}
}

func TestEvaluate_ExitCodes(t *testing.T) {
	s := &Scanner{}
	set, _ := threshold.Parse("sast-high=10")
	plan := set.Partition(model.EngineSAST)

	// Critical-aware breach code 11 (High) with parsed counts -> accurate breach.
	r := &model.Result{Engine: model.EngineSAST, ChildExitCode: 11, Counts: model.SeverityCount{model.SevHigh: 14}}
	v := s.Evaluate(r, plan)
	if v.Category != model.CatThresholdBreach || len(v.Breaches) != 1 || v.Breaches[0].Actual != 14 {
		t.Errorf("expected accurate high breach, got %+v", v)
	}

	// Pre-Critical breach code 10 still buckets to breach.
	r = &model.Result{Engine: model.EngineSAST, ChildExitCode: 10, Counts: model.SeverityCount{model.SevHigh: 14}}
	if v := s.Evaluate(r, plan); v.Category != model.CatThresholdBreach {
		t.Errorf("exit 10 should bucket to breach, got %+v", v)
	}

	// Login failure -> prerequisite.
	if v := s.Evaluate(&model.Result{Engine: model.EngineSAST, ChildExitCode: 4}, plan); v.Category != model.CatPrerequisiteMissing {
		t.Errorf("exit 4 should be prereq, got %+v", v)
	}
	// Success.
	if v := s.Evaluate(&model.Result{Engine: model.EngineSAST, ChildExitCode: 0}, plan); !v.Pass {
		t.Errorf("exit 0 should pass")
	}
	// Bad params -> engine failure.
	if v := s.Evaluate(&model.Result{Engine: model.EngineSAST, ChildExitCode: 1}, plan); v.Category != model.CatEngineFailure {
		t.Errorf("exit 1 should be engine failure, got %+v", v)
	}
}

func TestTeamProject(t *testing.T) {
	cases := []struct{ team, project, want string }{
		{"CxServer/SP", "TnR_Demo", `CxServer\SP\TnR_Demo`},      // forward slashes normalized
		{`CxServer\SP`, "TnR_Demo", `CxServer\SP\TnR_Demo`},      // already backslash
		{"/CxServer/SP/", "p", `CxServer\SP\p`},                  // leading/trailing trimmed
		{"", "p", "p"},                                           // no team -> bare project
	}
	for _, c := range cases {
		if got := teamProject(c.team, c.project); got != c.want {
			t.Errorf("teamProject(%q,%q) = %q, want %q", c.team, c.project, got, c.want)
		}
	}
}

func TestBuildInvocation_TeamPrefix(t *testing.T) {
	t.Setenv("CXSAST_USERNAME", "u")
	t.Setenv("CXSAST_PASSWORD", "p")
	dir := fakePlugin(t)
	cfg := &scanner.Config{
		Engine: model.EngineSAST, Path: filepath.Join(dir, "runCxConsole.sh"),
		Source: ".", ProjectName: "TnR_Demo", OutputDir: filepath.Join(dir, "out"),
		SASTServer: "http://cx", SASTUserEnv: "CXSAST_USERNAME", SASTPasswordEnv: "CXSAST_PASSWORD",
		Extra: map[string]string{"sastTeam": "CxServer/SP"},
	}
	inv, err := (&Scanner{}).BuildInvocation(cfg, threshold.Plan{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(inv.Args, " "), `-ProjectName CxServer\SP\TnR_Demo`) {
		t.Errorf("expected team-qualified project name, got: %s", strings.Join(inv.Args, " "))
	}
}

func TestJavaMajorParsing(t *testing.T) {
	// Indirectly verify the version regex via the documented formats.
	cases := map[string]int{
		`openjdk version "11.0.31" 2026-04-21`: 11,
		`java version "1.8.0_401"`:             8,
		`openjdk version "17.0.10" 2024`:       17,
		`openjdk version "21" 2023-09-19`:      21,
	}
	for out, want := range cases {
		m := javaVerRe.FindStringSubmatch(out)
		if m == nil {
			t.Errorf("no match for %q", out)
			continue
		}
		got := 0
		if m[1] == "1" && m[2] != "" {
			got = atoi(m[2])
		} else {
			got = atoi(m[1])
		}
		if got != want {
			t.Errorf("version %q -> %d, want %d", out, got, want)
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
