package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// Translation is the result of mapping a unified glob set onto one engine's
// native exclusion mechanism.
type Translation struct {
	// Patterns are the native exclusion patterns to pass to the engine (meaning
	// is engine-specific: glob exclude-paths for KICS, name globs for 2ms, etc.).
	Patterns []string
	Warnings []string
}

// ToKICSExcludePaths maps the union of global + per-engine globs onto KICS
// `-e/--exclude-paths` (glob, near-verbatim). Include-only intent cannot be
// expressed by KICS and is warned about.
func ToKICSExcludePaths(global, perEngine GlobSet) Translation {
	var t Translation
	t.Patterns = append(t.Patterns, global.Excludes()...)
	t.Patterns = append(t.Patterns, perEngine.Excludes()...)
	t.Patterns = dedupe(t.Patterns)
	warnIncludeOnly(&t, "KICS --exclude-paths", global, perEngine)
	return t
}

// ToSecretsIgnorePatterns maps globs onto 2ms `--ignore-pattern` (matches file/
// folder NAME, not full path). Include-only intent is warned about, and full-path
// globs are flagged as approximate.
func ToSecretsIgnorePatterns(global, perEngine GlobSet) Translation {
	var t Translation
	t.Patterns = append(t.Patterns, global.Excludes()...)
	t.Patterns = append(t.Patterns, perEngine.Excludes()...)
	t.Patterns = dedupe(t.Patterns)
	warnIncludeOnly(&t, "2ms --ignore-pattern", global, perEngine)
	return t
}

// CxSASTNames is the lossy translation of path globs into CxSAST folder-name and
// file-name exclusion lists (-LocationPathExclude / -LocationFilesExclude). CxSAST
// matches NAMES, not path globs, so this is inherently approximate and always
// carries a warning.
type CxSASTNames struct {
	Folders  []string
	Files    []string
	Warnings []string
}

// ToCxSASTNames performs the lossy glob -> name-pattern translation.
func ToCxSASTNames(global, perEngine GlobSet) CxSASTNames {
	var out CxSASTNames
	seenFolder := map[string]bool{}
	seenFile := map[string]bool{}

	add := func(gs GlobSet) {
		for _, pat := range gs.Excludes() {
			name, isFile := lastSegmentName(pat)
			if name == "" {
				out.Warnings = append(out.Warnings,
					fmt.Sprintf("CxSAST: glob %q has no concrete name to exclude; ignored — use --sast-arg=-LocationPathExclude=... for exact control", pat))
				continue
			}
			if isFile {
				if !seenFile[name] {
					out.Files = append(out.Files, name)
					seenFile[name] = true
				}
			} else {
				if !seenFolder[name] {
					out.Folders = append(out.Folders, name)
					seenFolder[name] = true
				}
			}
			out.Warnings = append(out.Warnings,
				fmt.Sprintf("CxSAST: glob %q translated to a NAME pattern %q (path context lost; matches that name anywhere)", pat, name))
		}
		if gs.HasIncludeOnlyIntent() {
			out.Warnings = append(out.Warnings,
				"CxSAST: include-only filters (e.g. \"!**/**,**/src/**\") cannot be expressed via -LocationPathExclude; set -LocationPath to the include root or use --sast-arg")
		}
	}
	add(global)
	add(perEngine)
	return out
}

// ValidateRegex compiles a regex filter value (used for --sca-filter and
// --containers-package-filter), returning an error if it does not compile. The
// value is forwarded verbatim; it is never synthesized from a glob.
func ValidateRegex(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if _, err := regexp.Compile(value); err != nil {
		return fmt.Errorf("%s: invalid regular expression %q: %w", field, value, err)
	}
	return nil
}

// lastSegmentName extracts the most specific NAME from a path glob and reports
// whether it looks like a file pattern (vs a directory name). Heuristic:
//   - trailing "/**", "/*", "/" => directory; take the last non-glob segment
//   - otherwise take the last segment; if it contains a '.' or a '*' with an
//     extension it is treated as a file pattern, else a folder name.
func lastSegmentName(glob string) (name string, isFile bool) {
	g := strings.TrimSpace(glob)
	g = strings.TrimRight(g, "/")
	dir := false
	// Strip trailing glob segments like "/**" or "/*".
	for strings.HasSuffix(g, "/**") || strings.HasSuffix(g, "/*") {
		dir = true
		g = strings.TrimSuffix(g, "/**")
		g = strings.TrimSuffix(g, "/*")
		g = strings.TrimRight(g, "/")
	}
	if g == "" || g == "**" || g == "*" {
		return "", false
	}
	segs := strings.Split(g, "/")
	last := segs[len(segs)-1]
	// Skip pure-glob trailing segments.
	for last == "**" || last == "*" {
		segs = segs[:len(segs)-1]
		if len(segs) == 0 {
			return "", false
		}
		last = segs[len(segs)-1]
		dir = true
	}
	if dir {
		return last, false
	}
	// Decide file vs folder by the shape of the last segment.
	if strings.Contains(last, ".") {
		return last, true
	}
	return last, false
}

func warnIncludeOnly(t *Translation, target string, sets ...GlobSet) {
	for _, gs := range sets {
		if gs.HasIncludeOnlyIntent() {
			t.Warnings = append(t.Warnings,
				fmt.Sprintf("%s is exclude-only: include-only intent in %q is ignored (the engine cannot restrict to includes)", target, gs.Raw))
		}
	}
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
