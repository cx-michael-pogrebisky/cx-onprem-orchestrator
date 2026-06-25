package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/filter"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

// Validate enforces the configuration rules that must hold before any subprocess
// runs. A returned error maps to exit code 30 (ConfigError). Non-fatal issues are
// appended to rc.Warnings.
func Validate(rc *RunConfig) error {
	var errs []string

	// DAST is reserved for post-v1.
	for _, e := range rc.Scanners {
		if e == model.EngineDAST {
			errs = append(errs, "engine \"dast\" is not available in this version (DAST is planned post-v1)")
		}
	}

	// --async with any threshold is rejected: cx and CxSAST both require a
	// synchronous scan to evaluate thresholds.
	if rc.Async && len(rc.Threshold.Clauses) > 0 {
		errs = append(errs, "--async cannot be combined with --threshold: thresholds require a synchronous scan")
	}

	// Regex filters must compile.
	if err := filter.ValidateRegex("--sca-filter", rc.Filters.SCA); err != nil {
		errs = append(errs, err.Error())
	}
	if err := filter.ValidateRegex("--containers-package-filter", rc.Filters.ContainersPackage); err != nil {
		errs = append(errs, err.Error())
	}

	// Cx1 OAuth2 client-credentials auth requires the base/auth URIs + tenant
	// (the API key would otherwise auto-derive them).
	if rc.Auth.CxClientID != "" {
		var miss []string
		if rc.Auth.CxBaseURI == "" {
			miss = append(miss, "--cx-base-uri")
		}
		if rc.Auth.CxBaseAuthURI == "" {
			miss = append(miss, "--cx-base-auth-uri")
		}
		if rc.Auth.CxTenant == "" {
			miss = append(miss, "--cx-tenant")
		}
		if len(miss) > 0 {
			errs = append(errs, fmt.Sprintf("--cx-client-id (client-credentials auth) also requires: %s", strings.Join(miss, ", ")))
		}
	}

	// Policy enums.
	switch rc.OnMissing {
	case OnMissingFail, OnMissingSkipWarn:
	default:
		errs = append(errs, fmt.Sprintf("--on-missing %q invalid (want fail|skip-warn)", rc.OnMissing))
	}
	switch rc.Conflict {
	case ConflictError, ConflictRawWins, ConflictUnifiedWins:
	default:
		errs = append(errs, fmt.Sprintf("--conflict %q invalid (want error|raw-wins|unified-wins)", rc.Conflict))
	}

	// ignore-on-exit enum.
	switch rc.Output.IgnoreOnExit {
	case "none", "results", "errors", "all":
	default:
		errs = append(errs, fmt.Sprintf("--ignore-on-exit %q invalid (want none|results|errors|all)", rc.Output.IgnoreOnExit))
	}

	if rc.Parallel < 0 {
		errs = append(errs, "--parallel must be >= 0")
	}

	// Tier-A / Tier-B conflict detection (default policy = error).
	if err := checkConflicts(rc); err != nil {
		errs = append(errs, err.Error())
	}

	// Warnings (non-fatal): thresholds for engines not selected are dead clauses.
	selected := map[model.Engine]bool{}
	for _, e := range rc.Scanners {
		selected[e] = true
	}
	for _, e := range rc.Threshold.Engines() {
		if !selected[e] {
			rc.Warnings = append(rc.Warnings,
				fmt.Sprintf("threshold references engine %q which is not in --scanners; clause has no effect", e))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n  - "))
	}
	return nil
}

// managedFlags maps each engine to the native flags that cxscan generates from
// Tier-A inputs. A raw --<engine>-arg whose leading token matches one of these
// (while the corresponding Tier-A input is set) is a collision.
var managedFlags = map[model.Engine][]string{
	model.EngineSAST:       {"-SASTCritical", "-SASTHigh", "-SASTMedium", "-SASTLow", "-LocationPathExclude", "-LocationFilesExclude"},
	model.EngineSCA:        {"--threshold", "--sca-filter", "--file-filter", "--file-include", "--use-gitignore"},
	model.EngineContainers: {"--threshold", "--containers-file-folder-filter", "--containers-package-filter", "--containers-image-tag-filter", "--file-filter"},
	model.EngineIaC:        {"--exclude-paths", "-e", "--fail-on", "--ignore-on-exit"},
	model.EngineSecrets:    {"--ignore-pattern", "--ignore-on-exit"},
}

func checkConflicts(rc *RunConfig) error {
	if rc.Conflict != ConflictError {
		return nil // raw-wins / unified-wins handled at build time with a warning
	}
	for _, e := range rc.Scanners {
		cfg := rc.EngineConfigs[e]
		if cfg == nil {
			continue
		}
		managed := managedFlags[e]
		tierASet := engineHasTierAInputs(rc, e)
		for _, raw := range cfg.RawArgs {
			name := rawFlagName(raw)
			for _, m := range managed {
				if strings.EqualFold(name, m) && tierASet {
					return fmt.Errorf("engine %q: raw arg %q collides with a flag cxscan manages from --threshold/--filter (set --conflict=raw-wins or unified-wins to resolve)", e, name)
				}
			}
		}
	}
	return nil
}

// engineHasTierAInputs reports whether any unified threshold/filter input is set
// for an engine (so a raw collision is meaningful).
func engineHasTierAInputs(rc *RunConfig, e model.Engine) bool {
	if len(rc.Threshold.ForEngine(e)) > 0 {
		return true
	}
	switch e {
	case model.EngineSAST:
		return rc.Filters.SAST != "" || rc.Filters.FileFilter != ""
	case model.EngineSCA:
		return rc.Filters.SCA != "" || rc.Filters.FileFilter != "" || rc.Filters.FileInclude != "" || rc.Filters.UseGitignore
	case model.EngineContainers:
		return rc.Filters.ContainersFileFolder != "" || rc.Filters.ContainersPackage != "" || rc.Filters.ContainersImageTag != "" || rc.Filters.FileFilter != ""
	case model.EngineIaC:
		return rc.Filters.IaC != "" || rc.Filters.FileFilter != ""
	case model.EngineSecrets:
		return rc.Filters.Secrets != "" || rc.Filters.FileFilter != ""
	}
	return false
}

// rawFlagName extracts the flag name from a raw passthrough token like
// "-SASTHigh=9" or "--exclude-paths".
func rawFlagName(raw string) string {
	s := raw
	if i := strings.IndexByte(s, '='); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
