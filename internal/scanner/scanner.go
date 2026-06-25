// Package scanner defines the Scanner interface implemented by each engine, plus
// a registry the CLI/orchestrator use to obtain scanners for the selected engines.
// Per-engine implementations live in subpackages (sast, sca, kics, secrets,
// containers) and register themselves via Register in an init().
package scanner

import (
	"context"
	"fmt"
	"sort"
	"time"

	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

// Config is the subset of resolved run configuration a scanner needs. It is
// defined here (rather than importing internal/config) to keep the dependency
// direction one-way: config builds these and hands them to scanners, and
// scanners never import config.
type Config struct {
	Engine        model.Engine
	Mode          string // "native" | "docker"
	Path          string // binary/JAR/script path (native)
	Image         string // image ref or digest (docker)
	Source        string // workspace path with the code under test
	ProjectName   string
	Branch        string
	OutputDir     string // wrapper-controlled per-engine report directory
	ReportFormats []string
	IgnoreOnExit  string   // none|results|errors|all (unified)
	RawArgs       []string // Tier-B --<engine>-arg passthrough (verbatim, in order)
	ResolverArgs  []string // SCA only: forwarded into --sca-resolver-params
	Async         bool
	// Timeout, when > 0, bounds this single engine's run (set from --<engine>-timeout).
	Timeout time.Duration

	// Filters (raw cx-style values; each engine translates to its native flags).
	FileFilter                 string
	FileInclude                string
	UseGitignore               bool
	SASTFilter                 string
	SCAFilter                  string // regex
	IaCFilter                  string
	SecretsFilter              string
	ContainersFileFolderFilter string
	ContainersPackageFilter    string // regex
	ContainersImageTagFilter   string // wildcard

	// Auth selectors (env names + non-secret values). Secret VALUES are read from
	// the named env vars at run time and injected into the child env, never argv.
	// Cx1 supports two mutually-exclusive modes: API key (default), or OAuth2
	// client-credentials (client-id/secret + base-uri + base-auth-uri + tenant).
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

	// Engine-specific knobs (only the relevant ones are set per engine):
	Extra map[string]string
	// Env carries names of env vars to inject into the child (values read at run).
	EnvInject map[string]string
}

// Scanner orchestrates one engine. Implementations must be pure in
// BuildInvocation (no IO beyond stat) so --dry-run can print the exact argv.
type Scanner interface {
	Engine() model.Engine

	// Available checks prerequisites WITHOUT scanning (binary/docker/auth/Java8/
	// ScaResolver+configuration.yml). A non-nil error means the engine cannot run.
	Available(ctx context.Context, cfg *Config) error

	// BuildInvocation translates resolved config + the engine's threshold plan
	// into a concrete, printable Invocation.
	BuildInvocation(cfg *Config, th threshold.Plan) (*model.Invocation, error)

	// Run executes the invocation, returning a Result even on non-zero child exit
	// (a breach is not an error).
	Run(ctx context.Context, inv *model.Invocation, opts execpkg.Options) *model.Result

	// ParseResults reads collected reports into r.Counts/r.Findings. Mandatory for
	// wrapper-side engines (KICS, secrets); best-effort for pass-through engines.
	ParseResults(ctx context.Context, r *model.Result) error

	// Evaluate produces the per-engine verdict from the result + threshold plan.
	Evaluate(r *model.Result, th threshold.Plan) model.Verdict
}

// Factory constructs a Scanner.
type Factory func() Scanner

var registry = map[model.Engine]Factory{}

// Register adds a scanner factory. Called from engine subpackages' init().
func Register(e model.Engine, f Factory) {
	registry[e] = f
}

// Get returns a fresh Scanner for an engine, or an error if none is registered.
func Get(e model.Engine) (Scanner, error) {
	f, ok := registry[e]
	if !ok {
		return nil, fmt.Errorf("no scanner registered for engine %q", e)
	}
	return f(), nil
}

// Registered reports whether an engine has a registered scanner.
func Registered(e model.Engine) bool {
	_, ok := registry[e]
	return ok
}

// RegisteredEngines returns the registered engines in canonical order.
func RegisteredEngines() []model.Engine {
	var out []model.Engine
	for e := range registry {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RunInvocation is the shared Run helper: it executes inv via internal/exec and
// maps the result into a model.Result. A non-zero child exit is recorded, not
// treated as an error (a threshold breach is expected). Engine implementations
// typically call this and then set ReportPaths.
func RunInvocation(ctx context.Context, inv *model.Invocation, opts execpkg.Options) *model.Result {
	// Per-engine time-box (independent of the global --timeout). On expiry the
	// child is killed; execpkg records the cancellation as a non-nil Err, which
	// the engine's Evaluate then surfaces as an engine failure (never a breach).
	if inv.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, inv.Timeout)
		defer cancel()
	}
	er := execpkg.Run(ctx, inv, opts)
	mode := "native"
	if inv.UsesDocker {
		mode = "docker"
	}
	return &model.Result{
		Engine:        inv.Engine,
		Ran:           true,
		Mode:          mode,
		ChildExitCode: er.ExitCode,
		OutputDir:     inv.OutputDir,
		Stdout:        er.Stdout,
		Stderr:        er.Stderr,
		Err:           er.Err,
	}
}
