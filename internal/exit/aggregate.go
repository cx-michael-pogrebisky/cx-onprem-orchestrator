package exit

import "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"

// Aggregate reduces per-engine verdicts to a single process exit code using a
// stable worst-wins precedence:
//
//	interrupted (130) > config (30) > prerequisite-missing (31)
//	  > engine-failure+breach (21) > engine-failure (20) > breach (10) > success (0)
//
// Skipped engines (via --on-missing skip-warn) are an explicit opt-in and do NOT
// fail the run on their own; they are recorded in the run summary. CodePartialSuccess
// (11) is reserved and not emitted in v1.
func Aggregate(verdicts []model.Verdict) Code {
	var hasBreach, hasEngineFail, hasConfig, hasPrereq, hasInterrupt bool
	for _, v := range verdicts {
		switch v.Category {
		case model.CatThresholdBreach:
			hasBreach = true
		case model.CatEngineFailure:
			hasEngineFail = true
		case model.CatConfigError:
			hasConfig = true
		case model.CatPrerequisiteMissing:
			hasPrereq = true
		case model.CatInterrupted:
			hasInterrupt = true
		}
	}
	switch {
	case hasInterrupt:
		return CodeInterrupted
	case hasConfig:
		return CodeConfigError
	case hasPrereq:
		return CodePrerequisiteMissing
	case hasEngineFail && hasBreach:
		return CodeEngineFailureAndBreach
	case hasEngineFail:
		return CodeEngineFailure
	case hasBreach:
		return CodeThresholdBreach
	default:
		return CodeSuccess
	}
}
