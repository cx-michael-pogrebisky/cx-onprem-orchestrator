package threshold

import (
	"testing"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

func clauseKey(c Clause) string { return c.Key }

func TestParse_CanonicalExample(t *testing.T) {
	// The verbatim example from the ast-cli usage string.
	set, err := Parse("sast-high=10;sca-high=5;iac-security-low=10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Clauses) != 3 {
		t.Fatalf("want 3 clauses, got %d: %+v", len(set.Clauses), set.Clauses)
	}
	want := map[string]int{"sast-high": 10, "sca-high": 5, "iac-security-low": 10}
	for _, c := range set.Clauses {
		if want[clauseKey(c)] != c.Limit {
			t.Errorf("clause %s: want limit %d, got %d", c.Key, want[c.Key], c.Limit)
		}
	}
}

func TestParse_NormalizationRules(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]int // canonical key -> limit
	}{
		{"commas as separators", "sast-high=10,sca-high=5", map[string]int{"sast-high": 10, "sca-high": 5}},
		{"mixed comma/semicolon", "sast-high=10,sca-high=5;sast-low=3", map[string]int{"sast-high": 10, "sca-high": 5, "sast-low": 3}},
		{"whitespace stripped", " sast-high = 10 ; sca-high = 5 ", map[string]int{"sast-high": 10, "sca-high": 5}},
		{"uppercase folded", "SAST-HIGH=10;Sca-Critical=2", map[string]int{"sast-high": 10, "sca-critical": 2}},
		{"kics alias to iac-security", "kics-low=4", map[string]int{"iac-security-low": 4}},
		{"api-security multi-dash engine", "api-security-critical=1", map[string]int{"api-security-critical": 1}},
		{"container-security alias", "container-security-high=3", map[string]int{"containers-high": 3}},
		{"trailing separators ignored", "sast-high=10;;", map[string]int{"sast-high": 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(set.Clauses) != len(tc.want) {
				t.Fatalf("want %d clauses, got %d: %+v", len(tc.want), len(set.Clauses), set.Clauses)
			}
			for _, c := range set.Clauses {
				if w, ok := tc.want[c.Key]; !ok || w != c.Limit {
					t.Errorf("clause %s=%d not expected (want %v)", c.Key, c.Limit, tc.want)
				}
			}
		})
	}
}

func TestParse_LastWins(t *testing.T) {
	set, err := Parse("sast-high=10;sast-high=20")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Clauses) != 1 {
		t.Fatalf("want 1 clause after last-wins dedup, got %d", len(set.Clauses))
	}
	if set.Clauses[0].Limit != 20 {
		t.Errorf("want last value 20, got %d", set.Clauses[0].Limit)
	}
}

func TestParse_SecretsAliasing(t *testing.T) {
	cases := []struct {
		in        string
		wantLimit int
	}{
		{"secrets-total=1", 1},
		{"secrets=3", 3},      // bare secrets -> total
		{"secrets-high=5", 5}, // severity collapses to total
		{"SECRETS-Critical=2", 2},
	}
	for _, tc := range cases {
		set, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.in, err)
		}
		if len(set.Clauses) != 1 {
			t.Fatalf("%s: want 1 clause, got %d", tc.in, len(set.Clauses))
		}
		c := set.Clauses[0]
		if c.Engine != model.EngineSecrets || c.Severity != model.SevTotal {
			t.Errorf("%s: want secrets/total, got %s/%s", tc.in, c.Engine, c.Severity)
		}
		if c.Limit != tc.wantLimit {
			t.Errorf("%s: want limit %d, got %d", tc.in, tc.wantLimit, c.Limit)
		}
	}
	// secrets aliases collapse to a single bucket (last-wins).
	set, err := Parse("secrets-high=5;secrets-low=3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Clauses) != 1 || set.Clauses[0].Limit != 3 {
		t.Errorf("want a single secrets-total clause limit 3, got %+v", set.Clauses)
	}
}

func TestParse_Errors(t *testing.T) {
	cases := []struct{ name, in string }{
		{"limit zero rejected", "sast-high=0"},
		{"negative limit", "sast-high=-1"},
		{"non-integer limit", "sast-high=abc"},
		{"missing severity", "sast=10"},
		{"unknown engine", "bogus-high=10"},
		{"unknown key no severity", "bogus=10"},
		{"missing equals", "sast-high"},
		{"total only valid for secrets", "sast-total=5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.in); err == nil {
				t.Errorf("expected error for %q, got nil", tc.in)
			}
		})
	}
}

func TestParse_Empty(t *testing.T) {
	for _, in := range []string{"", "   ", "\t\n"} {
		set, err := Parse(in)
		if err != nil {
			t.Fatalf("empty input %q should not error: %v", in, err)
		}
		if len(set.Clauses) != 0 {
			t.Errorf("empty input %q should yield 0 clauses, got %d", in, len(set.Clauses))
		}
	}
}

func TestSASTNativeCap(t *testing.T) {
	cases := []struct{ limit, want int }{
		{10, 9}, {1, 0}, {5, 4},
	}
	for _, tc := range cases {
		if got := SASTNativeCap(tc.limit); got != tc.want {
			t.Errorf("SASTNativeCap(%d) = %d, want %d", tc.limit, got, tc.want)
		}
	}
}

func TestEnforce_Inclusive(t *testing.T) {
	set, _ := Parse("iac-security-low=10;iac-security-high=1")
	plan := set.Partition(model.EngineIaC)
	if plan.Route != model.RouteWrapperSide {
		t.Fatalf("IaC should route wrapper-side, got %s", plan.Route)
	}

	// low=12 (>=10 breach), high=0 (<1 no breach)
	counts := model.SeverityCount{model.SevLow: 12, model.SevHigh: 0}
	breaches := Enforce(plan, counts)
	if len(breaches) != 1 || breaches[0].Severity != model.SevLow || breaches[0].Actual != 12 {
		t.Errorf("want single low breach actual=12, got %+v", breaches)
	}

	// Inclusive boundary: low exactly == limit must breach.
	counts = model.SeverityCount{model.SevLow: 10, model.SevHigh: 1}
	breaches = Enforce(plan, counts)
	if len(breaches) != 2 {
		t.Errorf("want 2 breaches at inclusive boundary, got %+v", breaches)
	}

	// Below: low=9 should not breach.
	counts = model.SeverityCount{model.SevLow: 9}
	if b := Enforce(plan, counts); len(b) != 0 {
		t.Errorf("want no breach below limit, got %+v", b)
	}
}
