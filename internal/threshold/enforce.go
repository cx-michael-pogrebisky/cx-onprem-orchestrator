package threshold

import "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"

// Enforce applies a plan's clauses to a wrapper-side count map using cx-exact
// semantics: a clause is breached when the per-severity count is INCLUSIVE
// >= the limit. Counts must already be exploitable-only (suppressed / not-
// exploitable findings excluded by report.Normalize). Severity buckets are not
// rolled up: an iac-security-high clause compares against the "high" count only;
// "or higher" is expressed by the user supplying multiple clauses (OR semantics).
func Enforce(plan Plan, counts model.SeverityCount) []model.BreachDetail {
	var breaches []model.BreachDetail
	for _, c := range plan.Clauses {
		actual := counts[c.Severity]
		if actual >= c.Limit {
			breaches = append(breaches, model.BreachDetail{
				Severity: c.Severity,
				Limit:    c.Limit,
				Actual:   actual,
			})
		}
	}
	return breaches
}

// SASTNativeCap converts a cx-inclusive limit (fail when count >= limit) into the
// CxSAST CxConsolePlugin cap value, which fails on a STRICT greater-than
// (count > cap). Therefore cap = limit - 1. Parse guarantees limit >= 1, so the
// returned cap is always >= 0.
//
//	sast-high=10 -> -SASTHigh 9   (fails at 10+ findings, matching cx's >=)
//	sast-high=1  -> -SASTHigh 0   (fails on any High finding)
func SASTNativeCap(limit int) int {
	cap := limit - 1
	if cap < 0 {
		cap = 0
	}
	return cap
}
