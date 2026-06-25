// Package report collects engine artifacts (the reports-before-gating barrier),
// normalizes findings into exploitable-only severity counts, and writes the
// machine-readable run-summary.json plus a human summary table.
package report

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

// CIInfo mirrors the detected CI context for the summary.
type CIInfo struct {
	Provider  string `json:"provider"`
	Branch    string `json:"branch,omitempty"`
	Commit    string `json:"commit,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Source    string `json:"source,omitempty"`
}

// EngineSummary is the per-engine record in the run summary.
type EngineSummary struct {
	Engine        string               `json:"engine"`
	Ran           bool                 `json:"ran"`
	Mode          string               `json:"mode,omitempty"`
	ChildExitCode int                  `json:"childExitCode"`
	Verdict       string               `json:"verdict"`
	Route         string               `json:"route,omitempty"`
	NativeGated   bool                 `json:"nativeGated,omitempty"`
	Breaches      []model.BreachDetail `json:"breaches,omitempty"`
	Counts        model.SeverityCount  `json:"counts,omitempty"`
	Reports       []string             `json:"reports,omitempty"`
	Warnings      []string             `json:"warnings,omitempty"`
	Message       string               `json:"message,omitempty"`
}

// Summary is the full run-summary.json document.
type Summary struct {
	SchemaVersion int             `json:"schemaVersion"`
	ExitCode      int             `json:"exitCode"`
	ExitCategory  string          `json:"exitCategory"`
	Version       string          `json:"cxooVersion"`
	CI            CIInfo          `json:"ci"`
	Threshold     string          `json:"threshold,omitempty"`
	Engines       []EngineSummary `json:"engines"`
	Warnings      []string        `json:"warnings,omitempty"`
}

// Write serializes the summary as indented JSON to <dir>/run-summary.json.
func (s Summary) Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "run-summary.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
