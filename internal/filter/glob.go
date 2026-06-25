// Package filter parses the cx-CLI-verbatim glob/Nant file-filter syntax and
// translates it to each engine's native exclusion mechanism. The unified glob
// surface is the canonical input; regex (--sca-filter, --containers-package-filter)
// and wildcard (--containers-image-tag-filter) filters are NEVER synthesized from
// it — they are dedicated verbatim passthrough, validated but not converted.
package filter

import "strings"

// Rule is one comma-separated glob/Nant pattern.
type Rule struct {
	Pattern string // pattern with any leading '!' stripped
	Exclude bool   // true if the token began with '!'
}

// GlobSet is a parsed --file-filter / --sast-filter / --iac-security-filter value.
//
// cx/Nant semantics, replicated verbatim:
//   - comma-separated list of patterns
//   - a leading '!' marks an exclusion
//   - if the list STARTS WITH an exclusion, all files are included first and then
//     filtered down (IncludeAllFirst); if it starts with an inclusion, nothing is
//     included first and the list adds files
//   - include patterns should precede excludes; patterns apply in order
type GlobSet struct {
	Raw             string
	IncludeAllFirst bool
	Rules           []Rule
}

// ParseGlob parses a comma-separated glob/Nant filter value. Whitespace around
// each token is trimmed; empty tokens are skipped. An empty input yields an empty
// (zero-rule) set.
func ParseGlob(s string) GlobSet {
	gs := GlobSet{Raw: s}
	first := true
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		excl := strings.HasPrefix(tok, "!")
		pat := tok
		if excl {
			pat = strings.TrimSpace(strings.TrimPrefix(tok, "!"))
		}
		if first {
			gs.IncludeAllFirst = excl
			first = false
		}
		if pat == "" {
			continue
		}
		gs.Rules = append(gs.Rules, Rule{Pattern: pat, Exclude: excl})
	}
	return gs
}

// IsEmpty reports whether the set has no rules.
func (gs GlobSet) IsEmpty() bool { return len(gs.Rules) == 0 }

// Excludes returns the meaningful exclusion patterns in order. A match-all
// pattern (e.g. "!**/**") is the Nant "include everything first" sentinel, not a
// real exclusion, so it is dropped — emitting it verbatim to an exclude-only
// engine would wrongly exclude the entire tree.
func (gs GlobSet) Excludes() []string {
	var out []string
	for _, r := range gs.Rules {
		if r.Exclude && !isMatchAll(r.Pattern) {
			out = append(out, r.Pattern)
		}
	}
	return out
}

// isMatchAll reports whether a glob matches the entire tree (the include-all
// sentinel when used as an exclusion).
func isMatchAll(pattern string) bool {
	switch strings.TrimSpace(pattern) {
	case "**", "**/**", "**/*", "*", "/**", "*/**", "**/":
		return true
	default:
		return false
	}
}

// Includes returns the explicit inclusion patterns in order (excludes the
// implicit "all" of an IncludeAllFirst set).
func (gs GlobSet) Includes() []string {
	var out []string
	for _, r := range gs.Rules {
		if !r.Exclude {
			out = append(out, r.Pattern)
		}
	}
	return out
}

// HasIncludeOnlyIntent reports whether the set expresses "scan only these
// includes" — i.e. it starts with an exclude-all (!**/**) and then adds includes.
// This intent cannot be represented by exclusion-only engines (KICS, 2ms, CxSAST)
// and must be surfaced as a warning.
func (gs GlobSet) HasIncludeOnlyIntent() bool {
	if !gs.IncludeAllFirst {
		return false
	}
	for _, r := range gs.Rules {
		if !r.Exclude {
			return true
		}
	}
	return false
}
