package filter

import "testing"

func TestParseGlob(t *testing.T) {
	gs := ParseGlob("!**/**,**/src/**,!**/test/**")
	if !gs.IncludeAllFirst {
		t.Errorf("want IncludeAllFirst=true for list starting with '!'")
	}
	if len(gs.Rules) != 3 {
		t.Fatalf("want 3 rules, got %d", len(gs.Rules))
	}
	inc := gs.Includes()
	if len(inc) != 1 || inc[0] != "**/src/**" {
		t.Errorf("want one include **/src/**, got %v", inc)
	}
	exc := gs.Excludes()
	// The leading "!**/**" is the include-all sentinel (dropped); only the real
	// "!**/test/**" exclude remains.
	if len(exc) != 1 || exc[0] != "**/test/**" {
		t.Errorf("want one real exclude **/test/**, got %v", exc)
	}
	if !gs.HasIncludeOnlyIntent() {
		t.Errorf("want HasIncludeOnlyIntent=true")
	}
}

func TestParseGlob_IncludeLed(t *testing.T) {
	gs := ParseGlob("*.java, *.go ")
	if gs.IncludeAllFirst {
		t.Errorf("include-led list should not set IncludeAllFirst")
	}
	if gs.HasIncludeOnlyIntent() {
		t.Errorf("include-led list is not 'include-only intent' in our sense")
	}
	if len(gs.Includes()) != 2 {
		t.Errorf("want 2 includes, got %v", gs.Includes())
	}
}

func TestToKICSExcludePaths(t *testing.T) {
	global := ParseGlob("!**/node_modules/**")
	per := ParseGlob("!**/vendor/**")
	tr := ToKICSExcludePaths(global, per)
	if len(tr.Patterns) != 2 {
		t.Fatalf("want 2 exclude patterns, got %v", tr.Patterns)
	}
	// global is include-all-first with an exclude only -> not include-only intent.
	if len(tr.Warnings) != 0 {
		t.Errorf("did not expect warnings, got %v", tr.Warnings)
	}
}

func TestToKICS_IncludeOnlyWarns(t *testing.T) {
	global := ParseGlob("!**/**,**/src/**")
	tr := ToKICSExcludePaths(global, GlobSet{})
	if len(tr.Warnings) == 0 {
		t.Errorf("expected an include-only warning for KICS")
	}
}

func TestExcludes_DropsMatchAllSentinel(t *testing.T) {
	// "!**/**" is the include-all-first sentinel and must NOT become a literal
	// exclude (which would exclude the whole tree).
	gs := ParseGlob("!**/**,!**/vendor/**")
	exc := gs.Excludes()
	if len(exc) != 1 || exc[0] != "**/vendor/**" {
		t.Errorf("want only the real exclude **/vendor/**, got %v", exc)
	}
	tr := ToKICSExcludePaths(gs, GlobSet{})
	for _, p := range tr.Patterns {
		if isMatchAll(p) {
			t.Errorf("match-all sentinel %q leaked into KICS exclude-paths", p)
		}
	}
}

func TestToCxSASTNames(t *testing.T) {
	global := ParseGlob("!**/test/**,!**/*.min.js,!docs/")
	names := ToCxSASTNames(global, GlobSet{})
	hasFolder := func(n string) bool {
		for _, f := range names.Folders {
			if f == n {
				return true
			}
		}
		return false
	}
	hasFile := func(n string) bool {
		for _, f := range names.Files {
			if f == n {
				return true
			}
		}
		return false
	}
	if !hasFolder("test") {
		t.Errorf("want folder 'test', got folders=%v files=%v", names.Folders, names.Files)
	}
	if !hasFolder("docs") {
		t.Errorf("want folder 'docs', got folders=%v", names.Folders)
	}
	if !hasFile("*.min.js") {
		t.Errorf("want file '*.min.js', got files=%v", names.Files)
	}
	if len(names.Warnings) == 0 {
		t.Errorf("lossy translation must warn")
	}
}

func TestValidateRegex(t *testing.T) {
	if err := ValidateRegex("--sca-filter", "(?i).*(test|mock).*"); err != nil {
		t.Errorf("valid regex rejected: %v", err)
	}
	if err := ValidateRegex("--sca-filter", "("); err == nil {
		t.Errorf("invalid regex accepted")
	}
	if err := ValidateRegex("--sca-filter", ""); err != nil {
		t.Errorf("empty regex should be allowed: %v", err)
	}
}
