package exit

import (
	"testing"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
)

func v(cat model.ExitCategory) model.Verdict { return model.Verdict{Category: cat} }

func TestAggregate_WorstWins(t *testing.T) {
	cases := []struct {
		name string
		in   []model.Verdict
		want Code
	}{
		{"all pass", []model.Verdict{v(model.CatPass), v(model.CatPass)}, CodeSuccess},
		{"breach only", []model.Verdict{v(model.CatPass), v(model.CatThresholdBreach)}, CodeThresholdBreach},
		{"engine fail beats breach", []model.Verdict{v(model.CatThresholdBreach), v(model.CatEngineFailure)}, CodeEngineFailureAndBreach},
		{"engine fail no breach", []model.Verdict{v(model.CatPass), v(model.CatEngineFailure)}, CodeEngineFailure},
		{"config dominates", []model.Verdict{v(model.CatThresholdBreach), v(model.CatConfigError)}, CodeConfigError},
		{"prereq", []model.Verdict{v(model.CatPass), v(model.CatPrerequisiteMissing)}, CodePrerequisiteMissing},
		{"interrupt dominates all", []model.Verdict{v(model.CatConfigError), v(model.CatInterrupted)}, CodeInterrupted},
		{"skipped does not fail", []model.Verdict{v(model.CatPass), v(model.CatSkipped)}, CodeSuccess},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Aggregate(tc.in); got != tc.want {
				t.Errorf("Aggregate = %d (%s), want %d (%s)", got, got, tc.want, tc.want)
			}
		})
	}
}

func TestInterpretKICS(t *testing.T) {
	if InterpretKICS(50) != model.CatPass {
		t.Errorf("KICS 50 (HIGH findings) should be pass at this layer (wrapper gates)")
	}
	if InterpretKICS(126) != model.CatEngineFailure {
		t.Errorf("KICS 126 should be engine failure")
	}
}

func TestInterpretCxSAST_Bucket(t *testing.T) {
	for code := 10; code <= 13; code++ {
		if InterpretCxSAST(code) != model.CatThresholdBreach {
			t.Errorf("CxSAST %d should bucket to threshold breach", code)
		}
	}
	if InterpretCxSAST(4) != model.CatPrerequisiteMissing {
		t.Errorf("CxSAST 4 should be login/prereq failure")
	}
}

func TestInterpretCx_OverloadedOne(t *testing.T) {
	if InterpretCx(1, true) != model.CatThresholdBreach {
		t.Errorf("cx exit 1 with a clause should be a breach")
	}
	if InterpretCx(1, false) != model.CatEngineFailure {
		t.Errorf("cx exit 1 without a clause should be engine failure")
	}
	if InterpretCx(3, false) != model.CatEngineFailure {
		t.Errorf("cx exit 3 (SCA engine fail) should be engine failure")
	}
}
