// Package model holds the pure, dependency-free domain types shared across
// cx-onprem-orchestrator (engines, severities, findings, results, verdicts,
// invocations). It is a leaf package: it imports only the standard library, so
// every other internal package can depend on it without import cycles.
package model

import "strings"

// Engine identifies one underlying scanner.
type Engine string

const (
	EngineSAST       Engine = "sast"         // CxSAST on-prem (CxConsolePlugin)
	EngineSCA        Engine = "sca"          // Cx1 SCA via SCA Resolver
	EngineIaC        Engine = "iac-security" // KICS; --scanners token is "kics"
	EngineSecrets    Engine = "secrets"      // 2ms; wrapper-local engine
	EngineContainers Engine = "containers"   // Cx1 Container Security
	EngineDAST       Engine = "dast"         // post-v1; reserved
	EngineAPISec     Engine = "api-security" // reserved (pass-through to cx)
)

// AllEngines is the canonical ordered set used by "--scanners all".
// DAST is intentionally excluded from v1.
var AllEngines = []Engine{EngineSAST, EngineSCA, EngineIaC, EngineSecrets, EngineContainers}

// Severity is a normalized finding severity. "total" is a synthetic bucket used
// only by the secrets engine (2ms has no severity model).
type Severity string

const (
	SevCritical Severity = "critical"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevLow      Severity = "low"
	SevInfo     Severity = "info"
	SevTotal    Severity = "total"
)

// orderedSeverities ranks severities from most to least severe for "worst" logic.
var orderedSeverities = []Severity{SevCritical, SevHigh, SevMedium, SevLow, SevInfo}

// Rank returns a sortable rank where lower = more severe. SevTotal and unknowns
// sort last.
func (s Severity) Rank() int {
	for i, sev := range orderedSeverities {
		if sev == s {
			return i
		}
	}
	return len(orderedSeverities) + 1
}

// IsRealSeverity reports whether s is one of the five real severities (not the
// synthetic "total").
func (s Severity) IsRealSeverity() bool {
	switch s {
	case SevCritical, SevHigh, SevMedium, SevLow, SevInfo:
		return true
	default:
		return false
	}
}

// ParseSeverity normalizes a free-form severity token (case-insensitive).
// The bool is false for unrecognized tokens.
func ParseSeverity(s string) (Severity, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return SevCritical, true
	case "high":
		return SevHigh, true
	case "medium":
		return SevMedium, true
	case "low":
		return SevLow, true
	case "info", "information", "informational":
		return SevInfo, true
	case "total":
		return SevTotal, true
	default:
		return "", false
	}
}

// SeverityCount is a per-severity tally of findings.
type SeverityCount map[Severity]int

// Total returns the sum across the five real severities (ignores SevTotal to
// avoid double counting).
func (sc SeverityCount) Total() int {
	n := 0
	for sev, c := range sc {
		if sev.IsRealSeverity() {
			n += c
		}
	}
	return n
}

// Finding is a single normalized result from any engine.
type Finding struct {
	Engine      Engine   `json:"engine"`
	Severity    Severity `json:"severity"`
	Exploitable bool     `json:"exploitable"` // cx counts exploitable-only toward thresholds
	QueryID     string   `json:"queryId,omitempty"`
	Location    string   `json:"location,omitempty"`
}

// BreachDetail records a single threshold clause that was breached
// (Actual >= Limit triggered it).
type BreachDetail struct {
	Severity Severity `json:"severity"`
	Limit    int      `json:"limit"`
	Actual   int      `json:"actual"`
}

// ExitCategory is the per-engine verdict category fed to exit-code aggregation.
type ExitCategory int

const (
	CatPass ExitCategory = iota
	CatThresholdBreach
	CatEngineFailure
	CatConfigError
	CatPrerequisiteMissing
	CatSkipped
	CatInterrupted
)

func (c ExitCategory) String() string {
	switch c {
	case CatPass:
		return "pass"
	case CatThresholdBreach:
		return "breach"
	case CatEngineFailure:
		return "engine-failure"
	case CatConfigError:
		return "config-error"
	case CatPrerequisiteMissing:
		return "prerequisite-missing"
	case CatSkipped:
		return "skipped"
	case CatInterrupted:
		return "interrupted"
	default:
		return "unknown"
	}
}

// Mount is a host->container bind mount for docker-backed engines.
type Mount struct {
	HostPath      string `json:"hostPath"`
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly"`
}

// Invocation is the concrete, fully-resolved plan for launching one engine's
// child process. It is produced by Scanner.BuildInvocation as a pure value (no
// IO beyond stat), so it can be printed by --dry-run and asserted in tests.
type Invocation struct {
	Engine       Engine   `json:"engine"`
	Path         string   `json:"path"`            // binary path OR "docker"
	Args         []string `json:"args"`            // argv (translated Tier-A + raw Tier-B passthrough)
	Env          []string `json:"-"`               // child env (e.g. CX_APIKEY); never serialized/logged raw
	EnvKeys      []string `json:"envKeys"`          // names of env vars injected, for --dry-run display
	WorkDir      string   `json:"workDir,omitempty"`
	DockerMounts []Mount  `json:"dockerMounts,omitempty"`
	OutputDir    string   `json:"outputDir"` // wrapper-controlled dir the child writes reports into
	UsesDocker   bool     `json:"usesDocker"`
	// Warnings surfaced during translation (e.g. lossy CxSAST filter conversion).
	Warnings []string `json:"warnings,omitempty"`
}

// Route describes how a threshold is enforced for an engine.
type Route string

const (
	RoutePassthrough  Route = "passthrough"   // native flag enforces the threshold
	RouteWrapperSide  Route = "wrapper-side"  // wrapper parses reports and counts
	RouteSeverityGate Route = "severity-gate" // DAST --fail-on (severity only)
	RouteNone         Route = "none"          // no threshold for this engine
)

// Result is the outcome of running one engine (before evaluation).
type Result struct {
	Engine        Engine        `json:"engine"`
	Ran           bool          `json:"ran"`
	Mode          string        `json:"mode,omitempty"` // native|docker
	ChildExitCode int           `json:"childExitCode"`
	NativeGated   bool          `json:"nativeGated"` // child already applied the threshold (pass-through)
	Route         Route         `json:"route"`
	Counts        SeverityCount `json:"counts,omitempty"`
	Findings      []Finding     `json:"-"` // populated only when wrapper-side counting is needed
	OutputDir     string        `json:"-"` // where the child wrote its reports (for ParseResults)
	ReportPaths   []string      `json:"reports,omitempty"`
	Warnings      []string      `json:"warnings,omitempty"`
	Stdout        []byte        `json:"-"`
	Stderr        []byte        `json:"-"`
	// Err is an execution-level error only (spawn/timeout/IO). A threshold breach
	// is NOT an error — it is a normal Result with a non-zero ChildExitCode.
	Err error `json:"-"`
}

// Verdict is the per-engine pass/fail decision.
type Verdict struct {
	Engine   Engine         `json:"engine"`
	Pass     bool           `json:"pass"`
	Category ExitCategory   `json:"category"`
	Breaches []BreachDetail `json:"breaches,omitempty"`
	Message  string         `json:"message,omitempty"`
}
