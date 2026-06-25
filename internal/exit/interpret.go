package exit

import "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"

// This file documents and centralizes how each underlying scanner's RAW child
// exit code maps to a wrapper ExitCategory. Scanners call these helpers from
// their Evaluate methods. The maps come from source-verified research:
//
//	KICS:   0 clean; 20 INFO; 30 LOW; 40 MEDIUM; 50 HIGH; 60 CRITICAL;
//	        70 remediation err; 126 engine err; 130 interrupt
//	2ms:    0 clean; 1 error; 2 secrets found; 3 error+secrets
//	CxSAST: 0 ok; 1 bad params; 4 login fail; 10-13 SAST threshold breach
//	cx:     0 ok; 1 generic/threshold; 2 SAST; 3 SCA; 4 IaC; 5 APISec engine fail
//	DAST:   0 ok; 2 error (post-v1)

// InterpretKICS maps a KICS exit code. Because the wrapper runs KICS with
// --ignore-on-exit=results and gates wrapper-side, the 20-60 "findings present"
// codes are NOT failures here; only 70/126 are engine failures and 130 is interrupt.
func InterpretKICS(code int) model.ExitCategory {
	switch {
	case code == 0:
		return model.CatPass
	case code >= 20 && code <= 60:
		return model.CatPass // findings present; wrapper-side count decides
	case code == 130:
		return model.CatInterrupted
	default: // 70 remediation, 126 engine error, others
		return model.CatEngineFailure
	}
}

// Interpret2ms maps a 2ms exit code. The wrapper runs 2ms with
// --ignore-on-exit=results and gates wrapper-side, so 2 (secrets found) is not a
// failure at this layer; 1/3 (error) are engine failures.
func Interpret2ms(code int) model.ExitCategory {
	switch code {
	case 0, 2:
		return model.CatPass // 2 = secrets present; wrapper-side count decides
	case 130:
		return model.CatInterrupted
	default: // 1 error, 3 error+secrets
		return model.CatEngineFailure
	}
}

// InterpretCxSAST maps a CxConsolePlugin exit code. 10-13 are bucketed to a SAST
// threshold breach regardless of the plugin's Critical-aware vs pre-Critical
// numbering; the exact severity is recovered from the wrapper's own clauses + XML.
func InterpretCxSAST(code int) model.ExitCategory {
	switch {
	case code == 0:
		return model.CatPass
	case code >= 10 && code <= 13:
		return model.CatThresholdBreach
	case code == 14 || code == 15 || code == 16 || code == 18 || code == 19:
		return model.CatThresholdBreach // OSA/policy/generic threshold codes
	case code == 4:
		return model.CatPrerequisiteMissing // login failed
	case code == 130:
		return model.CatInterrupted
	default: // 1 bad params, 2/3/5/6/7 etc.
		return model.CatEngineFailure
	}
}

// InterpretCx maps an ast-cli (cx) exit code for pass-through engines (SCA,
// containers). cx overloads 1 (generic AND threshold breach), so the caller must
// disambiguate using the parsed report + whether a clause exists; this helper
// returns the default interpretation when no report-based disambiguation applies.
func InterpretCx(code int, hasClause bool) model.ExitCategory {
	switch code {
	case 0:
		return model.CatPass
	case 1:
		if hasClause {
			return model.CatThresholdBreach
		}
		return model.CatEngineFailure
	case 130:
		return model.CatInterrupted
	default: // 2 SAST, 3 SCA, 4 IaC, 5 APISec engine failures
		return model.CatEngineFailure
	}
}
