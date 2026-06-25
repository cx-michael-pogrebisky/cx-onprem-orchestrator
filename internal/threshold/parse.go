package threshold

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

// knownEngines maps an accepted engine token (after alias resolution) to the
// canonical model.Engine. "kics" aliases to "iac-security" exactly as ast-cli does.
var engineAliases = map[string]model.Engine{
	"sast":               model.EngineSAST,
	"sca":                model.EngineSCA,
	"iac-security":       model.EngineIaC,
	"kics":               model.EngineIaC, // ast-cli rewrites kics-* -> iac-security-*
	"api-security":       model.EngineAPISec,
	"containers":         model.EngineContainers,
	"container-security": model.EngineContainers,
	"secrets":            model.EngineSecrets,
	"dast":               model.EngineDAST,
}

// severitySuffixes are the recognized "-<severity>" endings, checked when
// splitting an "<engine>-<severity>" key.
var severitySuffixes = []model.Severity{
	model.SevCritical, model.SevHigh, model.SevMedium, model.SevLow, model.SevInfo, model.SevTotal,
}

// Parse replicates the ast-cli --threshold normalization byte-for-byte:
//
//  1. strip ALL whitespace
//  2. ',' and ';' are interchangeable separators (canonicalize ',' -> ';')
//  3. lowercase the whole string (fully case-insensitive)
//  4. split on ';'
//  5. split each clause on '=' into key + integer limit
//  6. split the key into engine/severity on the rightmost known-severity suffix
//  7. alias kics-* -> iac-security-*; bare "secrets" / "secrets-<sev>" -> secrets-total
//  8. require limit to be an integer >= 1 (validated here, before any scan)
//
// Clauses with a duplicate canonical key are de-duplicated last-wins.
// An empty or whitespace-only input yields an empty Set and no error.
func Parse(raw string) (Set, error) {
	// (1) strip all whitespace, (2) commas -> semicolons, (3) lowercase.
	s := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, raw)
	s = strings.ReplaceAll(s, ",", ";")
	s = strings.ToLower(s)

	if s == "" {
		return Set{}, nil
	}

	// (4) split on ';'; track last-wins by canonical key while preserving a
	// deterministic final ordering.
	byKey := map[string]Clause{}
	for _, tok := range strings.Split(s, ";") {
		if tok == "" {
			continue
		}
		clause, err := parseClause(tok)
		if err != nil {
			return Set{}, err
		}
		byKey[clause.Key] = clause // last-wins
	}

	out := make([]Clause, 0, len(byKey))
	for _, c := range byKey {
		out = append(out, c)
	}
	sortClauses(out)
	return Set{Clauses: out}, nil
}

func parseClause(tok string) (Clause, error) {
	// (5) split on '='.
	eq := strings.IndexByte(tok, '=')
	if eq < 0 {
		return Clause{}, fmt.Errorf("invalid threshold clause %q: expected <engine>-<severity>=<limit>", tok)
	}
	key := tok[:eq]
	limitStr := tok[eq+1:]
	if key == "" {
		return Clause{}, fmt.Errorf("invalid threshold clause %q: empty engine/severity", tok)
	}

	engine, sev, err := splitKey(key)
	if err != nil {
		return Clause{}, err
	}

	// (8) limit must be an integer >= 1.
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return Clause{}, fmt.Errorf("invalid threshold limit %q in clause %q: must be an integer", limitStr, tok)
	}
	if limit < 1 {
		return Clause{}, fmt.Errorf("invalid threshold limit %d in clause %q: must be >= 1 (cx rejects 0 and negatives)", limit, tok)
	}

	return Clause{
		Engine:   engine,
		Severity: sev,
		Limit:    limit,
		Key:      string(engine) + "-" + string(sev),
	}, nil
}

// splitKey splits "<engine>-<severity>" into (engine, severity), applying the
// kics->iac-security alias and the secrets bare/severity aliasing.
func splitKey(key string) (model.Engine, model.Severity, error) {
	// Find the rightmost recognized severity suffix.
	for _, sev := range severitySuffixes {
		suffix := "-" + string(sev)
		if strings.HasSuffix(key, suffix) {
			enginePart := strings.TrimSuffix(key, suffix)
			engine, ok := engineAliases[enginePart]
			if !ok {
				return "", "", fmt.Errorf("unknown engine %q in threshold key %q", enginePart, key)
			}
			return normalizeEngineSeverity(engine, sev)
		}
	}

	// No severity suffix: only the bare "secrets" form is allowed (-> total).
	if engine, ok := engineAliases[key]; ok && engine == model.EngineSecrets {
		return model.EngineSecrets, model.SevTotal, nil
	}
	if _, ok := engineAliases[key]; ok {
		return "", "", fmt.Errorf("threshold key %q is missing a severity (expected %s-<severity>)", key, key)
	}
	return "", "", fmt.Errorf("unrecognized threshold key %q (expected <engine>-<severity>)", key)
}

// normalizeEngineSeverity applies engine-specific severity rules.
func normalizeEngineSeverity(engine model.Engine, sev model.Severity) (model.Engine, model.Severity, error) {
	if engine == model.EngineSecrets {
		// 2ms has no severity model: secrets-<anything> collapses to the total bucket.
		return model.EngineSecrets, model.SevTotal, nil
	}
	if sev == model.SevTotal {
		// "total" is only meaningful for secrets.
		return "", "", fmt.Errorf("severity 'total' is only valid for the secrets engine, not %q", engine)
	}
	return engine, sev, nil
}
