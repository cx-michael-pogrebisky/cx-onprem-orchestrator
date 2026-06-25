package scanner

import (
	"fmt"
	"strings"
)

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
