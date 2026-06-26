// Package config resolves the effective run configuration from CLI flags,
// environment variables, and CI auto-detection (precedence: flag > env > CI-detect
// > default), validates it (cx-exact threshold rules, filter types, async+
// threshold, conflicts), and produces the per-engine scanner configs.
package config

import (
	"time"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/ci"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

// Default auth env-var names. These match the variables the user already exports,
// so no renaming is needed.
const (
	DefaultCxAPIKeyEnv     = "CX1_APIKEY"      // re-exported to children as CX_APIKEY
	DefaultSASTUserEnv     = "CXSAST_USERNAME" // -> -CxUser
	DefaultSASTPasswordEnv = "CXSAST_PASSWORD" // -> -CxPassword
	DefaultSASTServerEnv   = "CXSAST_URL"      // -> -CxServer
)

// Flags is the raw flag surface populated by the CLI layer from cobra. Per-engine
// maps are keyed by the canonical --scanners token (sast, sca, kics, secrets,
// containers).
type Flags struct {
	Scanners string
	Source   string

	ProjectName string
	Branch      string

	Threshold string

	FileFilter   string
	FileInclude  string
	UseGitignore bool

	SASTFilter                 string
	SCAFilter                  string // regex
	IaCFilter                  string
	ContainersFileFolderFilter string
	ContainersPackageFilter    string // regex
	ContainersImageTagFilter   string // wildcard
	SecretsFilter              string

	ReportFormats string
	OutputPath    string
	OutputName    string
	IgnoreOnExit  string

	OnMissing string
	Conflict  string
	Async     bool
	Parallel  int
	FailFast  bool
	DryRun    bool
	Timeout   time.Duration

	// Auth selectors (env names / non-secret values only).
	CxAPIKeyEnv        string
	CxBaseURI          string
	CxBaseAuthURI      string
	CxTenant           string
	CxClientID         string
	CxClientSecretEnv  string
	CxClientSecretFile string
	SASTServer         string
	SASTUserEnv        string
	SASTPasswordEnv    string
	SASTTokenEnv       string
	SASTSSO            bool
	SASTJava           string // JDK home or java path for the CxConsolePlugin (Java 11+)
	SASTTeam           string // CxSAST team/full-path prefix for -ProjectName (e.g. CxServer/SP)

	// Per-engine resolution + passthrough (keyed by scanner token).
	Mode    map[string]string
	Path    map[string]string
	Image   map[string]string
	RawArgs map[string][]string

	// ReportFormatsOverride lets a single engine emit a different format set than
	// the global --report-formats (keyed by scanner token; CSV value).
	ReportFormatsOverride map[string]string

	// Engine-specific inputs.
	ScaResolverPath string
	ScaResolverArgs []string
	ContainerImages string
}

// RunConfig is the validated, resolved configuration for one run.
type RunConfig struct {
	Scanners    []model.Engine
	Source      string
	ProjectName string
	Branch      string

	ThresholdRaw string
	Threshold    threshold.Set

	Filters Filters
	Output  Output

	OnMissing OnMissingPolicy
	Conflict  ConflictPolicy
	Async     bool
	Parallel  int
	FailFast  bool
	DryRun    bool
	Timeout   time.Duration

	Auth AuthConfig
	CI   ci.Context

	// EngineConfigs is the per-engine scanner config, in Scanners order.
	EngineConfigs map[model.Engine]*scanner.Config

	// Warnings accumulated during resolution (e.g. lossy filter notices).
	Warnings []string
}

// Filters holds the unified filter surface (cx-verbatim names; types preserved).
type Filters struct {
	FileFilter           string
	FileInclude          string
	UseGitignore         bool
	SAST                 string
	SCA                  string // regex
	IaC                  string
	ContainersFileFolder string
	ContainersPackage    string // regex
	ContainersImageTag   string // wildcard
	Secrets              string
}

// Output holds report/output configuration.
type Output struct {
	Formats      []string
	Path         string
	Name         string
	IgnoreOnExit string
}

// AuthConfig holds resolved auth selectors (env names + non-secret values).
type AuthConfig struct {
	CxAPIKeyEnv        string
	CxBaseURI          string
	CxBaseAuthURI      string
	CxTenant           string
	CxClientID         string
	CxClientSecretEnv  string
	CxClientSecretFile string
	SASTServer         string
	SASTUserEnv        string
	SASTPasswordEnv    string
	SASTTokenEnv       string
	SASTSSO            bool
}

// OnMissingPolicy controls behavior when a requested engine's prerequisites are absent.
type OnMissingPolicy string

const (
	OnMissingFail     OnMissingPolicy = "fail"
	OnMissingSkipWarn OnMissingPolicy = "skip-warn"
)

// ConflictPolicy controls Tier-A vs Tier-B collisions.
type ConflictPolicy string

const (
	ConflictError       ConflictPolicy = "error"
	ConflictRawWins     ConflictPolicy = "raw-wins"
	ConflictUnifiedWins ConflictPolicy = "unified-wins"
)
