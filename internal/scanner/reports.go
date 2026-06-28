package scanner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

// EngineFormats declares one engine's report-format support: the unified tokens it
// can emit (sorted, including Mandatory) and the mandatory format the wrapper always
// requests for parsing / the run summary. This is the single source of truth used by
// both the engines (via SelectEngineFormats) and the `schema` command / config
// builder, so the page's per-scanner format options can never drift.
type EngineFormats struct {
	Mandatory string   `json:"mandatory"`
	Supported []string `json:"supported"`
}

var engineReportFormats = map[model.Engine]EngineFormats{
	model.EngineSAST:       {Mandatory: "xml", Supported: []string{"csv", "pdf", "rtf", "xml"}},
	model.EngineSCA:        {Mandatory: "json", Supported: UnifiedReportFormats()}, // cx (ast-cli)
	model.EngineIaC:        {Mandatory: "json", Supported: []string{"asff", "codeclimate", "csv", "cyclonedx", "glsast", "html", "json", "junit", "pdf", "sarif", "sonarqube"}},
	model.EngineSecrets:    {Mandatory: "json", Supported: []string{"json", "sarif", "yaml"}},
	model.EngineContainers: {Mandatory: "json", Supported: UnifiedReportFormats()}, // cx (ast-cli)
}

// EngineReportFormats returns a copy of the engine's report-format support.
func EngineReportFormats(e model.Engine) EngineFormats {
	ef := engineReportFormats[e]
	return EngineFormats{Mandatory: ef.Mandatory, Supported: append([]string(nil), ef.Supported...)}
}

// SelectEngineFormats restricts a unified --report-formats request to what the given
// engine supports (always including its mandatory format). Engines call this instead
// of declaring their own format sets, keeping one source of truth.
func SelectEngineFormats(e model.Engine, requested []string) (selected, warnings []string) {
	ef := engineReportFormats[e]
	sup := make(map[string]bool, len(ef.Supported))
	for _, f := range ef.Supported {
		sup[f] = true
	}
	return SelectFormats(requested, sup, ef.Mandatory)
}

// SelectFormats restricts the unified --report-formats request to the formats an
// engine actually supports, always including `mandatory` (the machine-readable
// format the wrapper needs for parsing / the run summary). Order is preserved and
// duplicates removed; every requested format the engine cannot emit yields a
// warning so coverage gaps are never silent.
func SelectFormats(requested []string, supported map[string]bool, mandatory string) (selected, warnings []string) {
	seen := map[string]bool{}
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			selected = append(selected, f)
		}
	}
	if mandatory != "" {
		add(mandatory)
	}
	for _, f := range requested {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "" {
			continue
		}
		if supported[f] {
			add(f)
		} else if f != mandatory {
			warnings = append(warnings,
				fmt.Sprintf("report format %q is not supported by this engine and was skipped", f))
		}
	}
	return selected, warnings
}

// UnifiedReportFormats returns the sorted set of accepted --report-formats tokens.
// It is the single source of truth for the format enum (used by the `schema`
// command that drives the config builder, so the two never drift).
func UnifiedReportFormats() []string {
	out := make([]string, 0, len(cxFormats))
	for k := range cxFormats {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cxFormats maps unified report-format tokens to the cx (ast-cli) --report-format
// native tokens. cx accepts a comma-separated list in --report-format.
var cxFormats = map[string]string{
	"json":         "json",
	"sarif":        "sarif",
	"sbom":         "sbom",
	"pdf":          "pdf",
	"markdown":     "markdown",
	"html":         "summaryHTML",
	"summary-html": "summaryHTML",
	"summary-json": "summaryJSON",
	"gl-sast":      "gl-sast-report",
	"gl-sca":       "gl-sca-report",
}

// CxReportFormats maps the unified --report-formats request to a cx --report-format
// comma-separated value (always including json), warning on unsupported tokens.
func CxReportFormats(requested []string) (string, []string) {
	supported := make(map[string]bool, len(cxFormats))
	for k := range cxFormats {
		supported[k] = true
	}
	selected, warnings := SelectFormats(requested, supported, "json")
	var native []string
	seen := map[string]bool{}
	for _, f := range selected {
		if t := cxFormats[f]; t != "" && !seen[t] {
			seen[t] = true
			native = append(native, t)
		}
	}
	return strings.Join(native, ","), warnings
}
