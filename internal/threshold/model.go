// Package threshold parses the cx-CLI-verbatim --threshold string, partitions
// clauses per engine with their enforcement route, and enforces wrapper-side
// thresholds with cx-exact (inclusive, exploitable-only) semantics.
package threshold

import (
	"sort"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

// Clause is one parsed "<engine>-<severity>=<limit>" threshold.
type Clause struct {
	Engine   model.Engine
	Severity model.Severity
	Limit    int
	// Key is the canonical "<engine>-<severity>" used for last-wins dedup.
	Key string
}

// Set is the fully-parsed, de-duplicated collection of clauses from one
// --threshold string.
type Set struct {
	Clauses []Clause
}

// ForEngine returns the clauses that apply to a single engine.
func (s Set) ForEngine(e model.Engine) []Clause {
	var out []Clause
	for _, c := range s.Clauses {
		if c.Engine == e {
			out = append(out, c)
		}
	}
	return out
}

// Engines returns the distinct engines referenced by the set, in canonical order.
func (s Set) Engines() []model.Engine {
	seen := map[model.Engine]bool{}
	for _, c := range s.Clauses {
		seen[c.Engine] = true
	}
	var out []model.Engine
	for _, e := range engineOrder {
		if seen[e] {
			out = append(out, e)
		}
	}
	return out
}

// Plan is the per-engine partition: the clauses for one engine plus how they are
// enforced (pass-through to a native flag vs wrapper-side counting).
type Plan struct {
	Engine  model.Engine
	Route   model.Route
	Clauses []Clause
}

// HasClauses reports whether the plan carries any threshold for the engine.
func (p Plan) HasClauses() bool { return len(p.Clauses) > 0 }

// engineOrder is the canonical ordering used to render clauses deterministically.
var engineOrder = []model.Engine{
	model.EngineSAST,
	model.EngineSCA,
	model.EngineIaC,
	model.EngineSecrets,
	model.EngineContainers,
	model.EngineAPISec,
	model.EngineDAST,
}

func engineRank(e model.Engine) int {
	for i, x := range engineOrder {
		if x == e {
			return i
		}
	}
	return len(engineOrder) + 1
}

// sortClauses orders clauses by engine then severity for deterministic output.
func sortClauses(cs []Clause) {
	sort.SliceStable(cs, func(i, j int) bool {
		if engineRank(cs[i].Engine) != engineRank(cs[j].Engine) {
			return engineRank(cs[i].Engine) < engineRank(cs[j].Engine)
		}
		return cs[i].Severity.Rank() < cs[j].Severity.Rank()
	})
}
