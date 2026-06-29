package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/ci"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

// scannerTokens maps every accepted --scanners token to its engine.
var scannerTokens = map[string]model.Engine{
	"sast":               model.EngineSAST,
	"sca":                model.EngineSCA,
	"kics":               model.EngineIaC,
	"iac-security":       model.EngineIaC,
	"iac":                model.EngineIaC,
	"secrets":            model.EngineSecrets,
	"2ms":                model.EngineSecrets,
	"twoms":              model.EngineSecrets,
	"containers":         model.EngineContainers,
	"container-security": model.EngineContainers,
	"container":          model.EngineContainers,
	"dast":               model.EngineDAST,
}

// CanonicalToken returns the canonical --scanners token (and flag-map key) for an engine.
func CanonicalToken(e model.Engine) string {
	switch e {
	case model.EngineSAST:
		return "sast"
	case model.EngineSCA:
		return "sca"
	case model.EngineIaC:
		return "kics"
	case model.EngineSecrets:
		return "secrets"
	case model.EngineContainers:
		return "containers"
	case model.EngineDAST:
		return "dast"
	default:
		return string(e)
	}
}

// ParseScanners parses the --scanners value into an ordered, de-duplicated engine
// list. "all" expands to model.AllEngines. An empty value is an error (selection
// is required).
func ParseScanners(s string) ([]model.Engine, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("--scanners is required (e.g. --scanners sast,sca,kics,secrets,containers or --scanners all)")
	}
	seen := map[model.Engine]bool{}
	var out []model.Engine
	add := func(e model.Engine) {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	for _, tok := range strings.Split(s, ",") {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		if tok == "all" {
			for _, e := range model.AllEngines {
				add(e)
			}
			continue
		}
		e, ok := scannerTokens[tok]
		if !ok {
			return nil, fmt.Errorf("unknown scanner %q (valid: sast, sca, kics, secrets, containers)", tok)
		}
		add(e)
	}
	return out, nil
}

// EnvFunc looks up an environment variable.
type EnvFunc func(string) string

// Resolve builds a validated RunConfig from flags, environment, and detected CI
// context. Validation errors are returned (callers map them to exit code 30).
func Resolve(f Flags, env EnvFunc, ciCtx ci.Context) (*RunConfig, error) {
	engines, err := ParseScanners(f.Scanners)
	if err != nil {
		return nil, err
	}

	thr, err := threshold.Parse(f.Threshold)
	if err != nil {
		return nil, err
	}

	rc := &RunConfig{
		Scanners:     engines,
		ProjectName:  f.ProjectName,
		Branch:       firstNonEmpty(f.Branch, ciCtx.Branch),
		ThresholdRaw: f.Threshold,
		Threshold:    thr,
		Async:        f.Async,
		Parallel:     f.Parallel,
		FailFast:     f.FailFast,
		DryRun:       f.DryRun,
		Timeout:      f.Timeout,
		CI:           ciCtx,
		Filters: Filters{
			FileFilter:           f.FileFilter,
			FileInclude:          f.FileInclude,
			UseGitignore:         f.UseGitignore,
			SAST:                 f.SASTFilter,
			SCA:                  f.SCAFilter,
			IaC:                  f.IaCFilter,
			ContainersFileFolder: f.ContainersFileFolderFilter,
			ContainersPackage:    f.ContainersPackageFilter,
			ContainersImageTag:   f.ContainersImageTagFilter,
			Secrets:              f.SecretsFilter,
		},
		Output: Output{
			Formats:      splitCSV(f.ReportFormats),
			Path:         firstNonEmpty(f.OutputPath, "./cxoo-reports"),
			Name:         firstNonEmpty(f.OutputName, "cxoo"),
			IgnoreOnExit: firstNonEmpty(f.IgnoreOnExit, "none"),
		},
		Auth: AuthConfig{
			CxAPIKeyEnv:        firstNonEmpty(f.CxAPIKeyEnv, DefaultCxAPIKeyEnv),
			CxBaseURI:          f.CxBaseURI,
			CxBaseAuthURI:      f.CxBaseAuthURI,
			CxTenant:           f.CxTenant,
			CxClientID:         f.CxClientID,
			CxClientSecretEnv:  f.CxClientSecretEnv,
			CxClientSecretFile: f.CxClientSecretFile,
			SASTServer:         firstNonEmpty(f.SASTServer, env(DefaultSASTServerEnv)),
			SASTUser:           f.SASTUser,
			SASTUserEnv:        firstNonEmpty(f.SASTUserEnv, DefaultSASTUserEnv),
			SASTPasswordEnv:    firstNonEmpty(f.SASTPasswordEnv, DefaultSASTPasswordEnv),
			SASTTokenEnv:       f.SASTTokenEnv,
			SASTSSO:            f.SASTSSO,
		},
	}

	// Fat-image defaults: the bundled tools export their locations so the image
	// works without extra flags. Explicit flags always win.
	if f.ScaResolverPath == "" {
		f.ScaResolverPath = env("CXOO_SCA_RESOLVER")
	}
	if v := env("CXOO_SAST_PATH"); v != "" {
		if f.Path == nil {
			f.Path = map[string]string{}
		}
		if f.Path["sast"] == "" {
			f.Path["sast"] = v
		}
	}
	if f.KicsQueries == "" {
		f.KicsQueries = env("CXOO_KICS_QUERIES_PATH")
	}

	// Source: flag > CI workspace > ".".
	rc.Source = firstNonEmpty(f.Source, ciCtx.Workspace, ".")

	// Policies with defaults.
	rc.OnMissing = OnMissingPolicy(firstNonEmpty(f.OnMissing, string(OnMissingFail)))
	rc.Conflict = ConflictPolicy(firstNonEmpty(f.Conflict, string(ConflictError)))

	// Project name default: derive from repo or source basename.
	if rc.ProjectName == "" {
		rc.ProjectName = deriveProjectName(ciCtx.Repo, rc.Source)
	}

	rc.EngineConfigs = buildEngineConfigs(rc, f)
	return rc, nil
}

// SecretValues returns the distinct, non-trivial secret VALUES referenced by the
// run — the Cx1 API key, Cx1 client secret, and CxSAST password/token — read from
// their configured env vars now. Used to redact them from --dry-run/validate output
// and from streamed child logs (CxSAST passes -CxPassword/-CxToken on the child
// argv, so the value can otherwise surface). Values shorter than 4 chars are skipped
// to avoid mangling unrelated output.
func SecretValues(rc *RunConfig, env EnvFunc) []string {
	set := map[string]bool{}
	add := func(name string) {
		if name == "" {
			return
		}
		if v := env(name); len(v) >= 4 {
			set[v] = true
		}
	}
	for _, cfg := range rc.EngineConfigs {
		add(cfg.SASTPasswordEnv)
		add(cfg.SASTTokenEnv)
		add(cfg.CxAPIKeyEnv)
		add(cfg.CxClientSecretEnv)
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	return out
}

func buildEngineConfigs(rc *RunConfig, f Flags) map[model.Engine]*scanner.Config {
	out := map[model.Engine]*scanner.Config{}
	for _, e := range rc.Scanners {
		tok := CanonicalToken(e)
		// Per-engine --<engine>-report-formats override, else the global set.
		formats := rc.Output.Formats
		if ov := mapGet(f.ReportFormatsOverride, tok); ov != "" {
			formats = splitCSV(ov)
		}
		// Per-family project-name override: CxSAST and the Cx1 engines (SCA,
		// Container Security) may use different project names; everything else
		// falls back to the shared rc.ProjectName.
		projectName := rc.ProjectName
		switch e {
		case model.EngineSAST:
			if f.SASTProjectName != "" {
				projectName = f.SASTProjectName
			}
		case model.EngineSCA, model.EngineContainers:
			if f.CxProjectName != "" {
				projectName = f.CxProjectName
			}
		}
		cfg := &scanner.Config{
			Engine:        e,
			Mode:          mapGet(f.Mode, tok),
			Path:          mapGet(f.Path, tok),
			Image:         mapGet(f.Image, tok),
			Source:        rc.Source,
			ProjectName:   projectName,
			Branch:        rc.Branch,
			OutputDir:     filepath.Join(rc.Output.Path, string(e)),
			ReportFormats: formats,
			IgnoreOnExit:  rc.Output.IgnoreOnExit,
			RawArgs:       mapGetSlice(f.RawArgs, tok),
			Async:         rc.Async,
			Extra:         map[string]string{},
			EnvInject:     map[string]string{},

			FileFilter:                 rc.Filters.FileFilter,
			FileInclude:                rc.Filters.FileInclude,
			UseGitignore:               rc.Filters.UseGitignore,
			SASTFilter:                 rc.Filters.SAST,
			SCAFilter:                  rc.Filters.SCA,
			IaCFilter:                  rc.Filters.IaC,
			SecretsFilter:              rc.Filters.Secrets,
			ContainersFileFolderFilter: rc.Filters.ContainersFileFolder,
			ContainersPackageFilter:    rc.Filters.ContainersPackage,
			ContainersImageTagFilter:   rc.Filters.ContainersImageTag,

			CxAPIKeyEnv:        rc.Auth.CxAPIKeyEnv,
			CxBaseURI:          rc.Auth.CxBaseURI,
			CxBaseAuthURI:      rc.Auth.CxBaseAuthURI,
			CxTenant:           rc.Auth.CxTenant,
			CxClientID:         rc.Auth.CxClientID,
			CxClientSecretEnv:  rc.Auth.CxClientSecretEnv,
			CxClientSecretFile: rc.Auth.CxClientSecretFile,
			SASTServer:         rc.Auth.SASTServer,
			SASTUser:           rc.Auth.SASTUser,
			SASTUserEnv:        rc.Auth.SASTUserEnv,
			SASTPasswordEnv:    rc.Auth.SASTPasswordEnv,
			SASTTokenEnv:       rc.Auth.SASTTokenEnv,
			SASTSSO:            rc.Auth.SASTSSO,
		}
		switch e {
		case model.EngineSAST:
			cfg.Extra["sastJava"] = f.SASTJava
			cfg.Extra["sastTeam"] = f.SASTTeam
		case model.EngineSCA:
			cfg.ResolverArgs = f.ScaResolverArgs
			cfg.Extra["scaResolverPath"] = f.ScaResolverPath
		case model.EngineIaC:
			cfg.Extra["kicsQueries"] = f.KicsQueries
		case model.EngineContainers:
			cfg.Extra["containerImages"] = f.ContainerImages
		}
		out[e] = cfg
	}
	return out
}

func deriveProjectName(repo, source string) string {
	if repo != "" {
		base := repo
		base = strings.TrimSuffix(base, ".git")
		if i := strings.LastIndexAny(base, "/:"); i >= 0 {
			base = base[i+1:]
		}
		if base != "" {
			return base
		}
	}
	abs, err := filepath.Abs(source)
	if err == nil {
		return filepath.Base(abs)
	}
	return filepath.Base(source)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mapGet(m map[string]string, k string) string {
	if m == nil {
		return ""
	}
	return m[k]
}

func mapGetSlice(m map[string][]string, k string) []string {
	if m == nil {
		return nil
	}
	return m[k]
}
