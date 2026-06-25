package scanner

import (
	"strings"
	"testing"
)

func TestSelectFormats(t *testing.T) {
	supported := map[string]bool{"xml": true, "pdf": true, "csv": true, "rtf": true}
	selected, warnings := SelectFormats([]string{"json", "pdf", "sarif", "csv"}, supported, "xml")
	// mandatory xml first, then supported pdf, csv; json+sarif unsupported -> warnings.
	got := strings.Join(selected, ",")
	if got != "xml,pdf,csv" {
		t.Errorf("selected = %q, want xml,pdf,csv", got)
	}
	if len(warnings) != 2 {
		t.Errorf("want 2 warnings (json, sarif), got %v", warnings)
	}
}

func TestSelectFormats_EmptyRequestStillMandatory(t *testing.T) {
	selected, warnings := SelectFormats(nil, map[string]bool{"json": true}, "json")
	if strings.Join(selected, ",") != "json" || len(warnings) != 0 {
		t.Errorf("empty request should yield just the mandatory format, got %v / %v", selected, warnings)
	}
}

func TestSelectFormats_NoDuplicateMandatory(t *testing.T) {
	selected, _ := SelectFormats([]string{"json", "json", "sarif"}, map[string]bool{"json": true, "sarif": true}, "json")
	if strings.Join(selected, ",") != "json,sarif" {
		t.Errorf("want deduped json,sarif, got %v", selected)
	}
}

func TestCxReportFormats(t *testing.T) {
	arg, warnings := CxReportFormats([]string{"json", "sarif", "html", "pdf", "csv"})
	// html -> summaryHTML; csv unsupported by cx -> warning; json mandatory.
	if arg != "json,sarif,summaryHTML,pdf" {
		t.Errorf("cx report-format = %q, want json,sarif,summaryHTML,pdf", arg)
	}
	if len(warnings) != 1 {
		t.Errorf("want 1 warning (csv), got %v", warnings)
	}
}
