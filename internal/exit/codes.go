// Package exit defines the stable, frozen exit-code contract for
// cx-onprem-orchestrator and the logic that interprets child-process exit codes
// and aggregates per-engine verdicts into a single process exit code.
//
// The integers below are a PUBLIC CONTRACT: CI gates branch on them. They live
// in a reserved band chosen so they never collide with any underlying scanner's
// own exit codes (KICS 20/30/40/50/60/70/126/130; 2ms 1/2/3; CxSAST 1/4/10-13;
// cx 1/2/3/4/5; DAST 2). Do not renumber without a major version bump.
package exit

// Code is a cx-onprem-orchestrator process exit code.
type Code int

const (
	// CodeSuccess: all selected engines ran; no threshold breached.
	CodeSuccess Code = 0
	// CodeThresholdBreach: >=1 engine breached a threshold; all engines executed cleanly.
	CodeThresholdBreach Code = 10
	// CodePartialSuccess: some engines passed; >=1 engine failed to execute (no breach); reports collected.
	CodePartialSuccess Code = 11
	// CodeEngineFailure: >=1 engine crashed/timed out; none breached.
	CodeEngineFailure Code = 20
	// CodeEngineFailureAndBreach: both an execution failure AND a threshold breach occurred.
	CodeEngineFailureAndBreach Code = 21
	// CodeConfigError: bad threshold/filter, async+threshold, mutually-exclusive flags,
	// Tier-A/Tier-B conflict, missing required input. Detected before any subprocess runs.
	CodeConfigError Code = 30
	// CodePrerequisiteMissing: Available() failed for a requested engine (no Java 8, no
	// docker, no ScaResolver/configuration.yml, no auth) and --on-missing=fail.
	CodePrerequisiteMissing Code = 31
	// CodeOrchestrationError: wrapper-internal failure (cannot write reports, recovered panic).
	CodeOrchestrationError Code = 40
	// CodeInterrupted: SIGINT/SIGTERM / context cancelled.
	CodeInterrupted Code = 130
)

// String renders a human-readable category name for a wrapper exit code.
func (c Code) String() string {
	switch c {
	case CodeSuccess:
		return "Success"
	case CodeThresholdBreach:
		return "ThresholdBreach"
	case CodePartialSuccess:
		return "PartialSuccess"
	case CodeEngineFailure:
		return "EngineFailure"
	case CodeEngineFailureAndBreach:
		return "EngineFailureAndBreach"
	case CodeConfigError:
		return "ConfigError"
	case CodePrerequisiteMissing:
		return "PrerequisiteMissing"
	case CodeOrchestrationError:
		return "OrchestrationError"
	case CodeInterrupted:
		return "Interrupted"
	default:
		return "Unknown"
	}
}

// Int returns the integer value for os.Exit.
func (c Code) Int() int { return int(c) }
