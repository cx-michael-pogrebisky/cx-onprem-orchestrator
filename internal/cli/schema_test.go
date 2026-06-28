package cli

import "testing"

func TestBuildSchema_CoversRunFlags(t *testing.T) {
	s := BuildSchema()

	if s.Command != "run" {
		t.Errorf("command = %q, want run", s.Command)
	}
	if len(s.Flags) == 0 {
		t.Fatal("no flags in schema")
	}
	// Every flag must be classified into a known group (no "other" leaks): the
	// config builder relies on the grouping to lay out controls.
	known := map[string]bool{
		"selection": true, "threshold": true, "filters": true, "output": true,
		"orchestration": true, "auth-cx1": true, "auth-sast": true, "tool": true,
		"passthrough": true,
	}
	for _, f := range s.Flags {
		if !known[f.Group] {
			t.Errorf("flag --%s has unclassified group %q (update classifyFlag)", f.Name, f.Group)
		}
		if f.Type == "" {
			t.Errorf("flag --%s has empty type", f.Name)
		}
	}

	// Enumerations the builder needs.
	if len(s.Engines) != 5 {
		t.Errorf("engines = %v, want 5", s.Engines)
	}
	if len(s.Severities) != 5 || s.Severities[0] != "critical" {
		t.Errorf("severities = %v", s.Severities)
	}
	if !contains(s.ReportFormats, "json") || !contains(s.ReportFormats, "sarif") {
		t.Errorf("report formats missing json/sarif: %v", s.ReportFormats)
	}

	// A few representative flags must be present (guards against accidental removal).
	for _, want := range []string{"scanners", "threshold", "report-formats", "sast-team", "sca-resolver", "parallel"} {
		if findFlag(s, want) == nil {
			t.Errorf("expected flag --%s in schema", want)
		}
	}
}

func TestClassifyFlag_PerEngine(t *testing.T) {
	cases := []struct{ name, group, engine string }{
		{"sast-arg", "passthrough", "sast"},
		{"kics-path", "tool", "kics"},
		{"sca-report-formats", "output", "sca"},
		{"containers-timeout", "orchestration", "containers"},
		{"sca-resolver-arg", "passthrough", "sca"},
		{"cx-apikey-env", "auth-cx1", ""},
		{"sast-team", "auth-sast", "sast"},
		{"threshold", "threshold", ""},
		{"scanners", "selection", ""},
	}
	for _, c := range cases {
		g, e := classifyFlag(c.name)
		if g != c.group || e != c.engine {
			t.Errorf("classifyFlag(%q) = (%q,%q), want (%q,%q)", c.name, g, e, c.group, c.engine)
		}
	}
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func findFlag(s Schema, name string) *SchemaFlag {
	for i := range s.Flags {
		if s.Flags[i].Name == name {
			return &s.Flags[i]
		}
	}
	return nil
}
