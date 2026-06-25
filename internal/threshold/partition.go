package threshold

import (
	"fmt"
	"strings"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

// RouteFor returns how an engine's thresholds are enforced.
//
//   - SAST: pass-through to CxSAST -SAST<Sev> caps (with the limit-1 off-by-one fix).
//   - SCA / Containers / API-Security: pass-through to `cx --threshold`.
//   - KICS / Secrets: wrapper-side (those engines have no per-severity count gate).
//   - DAST: severity-gate via --fail-on (post-v1).
func RouteFor(e model.Engine) model.Route {
	switch e {
	case model.EngineSAST, model.EngineSCA, model.EngineContainers, model.EngineAPISec:
		return model.RoutePassthrough
	case model.EngineIaC, model.EngineSecrets:
		return model.RouteWrapperSide
	case model.EngineDAST:
		return model.RouteSeverityGate
	default:
		return model.RouteNone
	}
}

// Partition builds the per-engine Plan for a single engine from a parsed Set.
func (s Set) Partition(e model.Engine) Plan {
	clauses := s.ForEngine(e)
	return Plan{
		Engine:  e,
		Route:   RouteFor(e),
		Clauses: clauses,
	}
}

// NativeThresholdString renders the plan as a cx-style "<engine>-<sev>=<limit>;..."
// string for pass-through engines (sca, containers, api-security). The engine
// prefix is the canonical cx token (e.g. "containers", "sca").
func (p Plan) NativeThresholdString() string {
	parts := make([]string, 0, len(p.Clauses))
	for _, c := range p.Clauses {
		parts = append(parts, fmt.Sprintf("%s-%s=%d", p.Engine, c.Severity, c.Limit))
	}
	return strings.Join(parts, ";")
}
