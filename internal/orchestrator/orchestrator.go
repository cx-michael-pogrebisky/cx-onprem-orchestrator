// Package orchestrator drives the selected scanners: it runs and collects every
// engine's results FIRST (the reports-before-gating barrier), then parses and
// evaluates them, then aggregates a single exit code. Sequential by default;
// --parallel runs engines concurrently.
package orchestrator

import (
	"context"
	"fmt"
	"os"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/config"
	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
)

// Outcome bundles the per-engine results and verdicts of a run.
type Outcome struct {
	Results  []*model.Result
	Verdicts []model.Verdict
}

// Run executes the selected engines. display is where prefixed child output is
// mirrored (e.g. os.Stderr).
func Run(ctx context.Context, rc *config.RunConfig, display *os.File) Outcome {
	var out Outcome

	type staged struct {
		engine  model.Engine
		sc      scanner.Scanner
		cfg     *scanner.Config
		result  *model.Result
		verdict *model.Verdict // set early for skip/prereq/config short-circuits
	}
	var stages []*staged

	// Phase 1: run + collect every engine (barrier: nothing is gated yet).
	for _, e := range rc.Scanners {
		st := &staged{engine: e, cfg: rc.EngineConfigs[e]}
		stages = append(stages, st)

		if !scanner.Registered(e) {
			st.verdict = unavailable(rc, e, "no scanner registered for this engine")
			continue
		}
		sc, err := scanner.Get(e)
		if err != nil {
			st.verdict = unavailable(rc, e, err.Error())
			continue
		}
		st.sc = sc

		if err := sc.Available(ctx, st.cfg); err != nil {
			st.verdict = unavailable(rc, e, err.Error())
			continue
		}

		thPlan := rc.Threshold.Partition(e)
		inv, err := sc.BuildInvocation(st.cfg, thPlan)
		if err != nil {
			v := model.Verdict{Engine: e, Pass: false, Category: model.CatConfigError, Message: err.Error()}
			st.verdict = &v
			continue
		}

		if err := os.MkdirAll(inv.OutputDir, 0o755); err != nil {
			v := model.Verdict{Engine: e, Pass: false, Category: model.CatEngineFailure, Message: fmt.Sprintf("cannot create output dir: %v", err)}
			st.verdict = &v
			continue
		}

		opts := execpkg.Options{Display: display, Prefix: fmt.Sprintf("[%s] ", e)}
		st.result = sc.Run(ctx, inv, opts)
		st.result.Engine = e
		st.result.Ran = true
		st.result.Route = thPlan.Route

		if rc.FailFast && st.result.Err != nil {
			break
		}
	}

	// Phase 2: parse + evaluate (gating happens only after the barrier).
	for _, st := range stages {
		if st.verdict != nil { // short-circuited (skipped/prereq/config)
			out.Verdicts = append(out.Verdicts, *st.verdict)
			if st.result != nil {
				out.Results = append(out.Results, st.result)
			}
			continue
		}
		if st.result == nil {
			continue // not reached (fail-fast break before some engines ran)
		}
		thPlan := rc.Threshold.Partition(st.engine)
		_ = st.sc.ParseResults(ctx, st.result)
		v := st.sc.Evaluate(st.result, thPlan)
		v.Engine = st.engine
		out.Results = append(out.Results, st.result)
		out.Verdicts = append(out.Verdicts, v)
	}

	return out
}

// unavailable builds the verdict for a missing/unavailable engine, honoring the
// --on-missing policy.
func unavailable(rc *config.RunConfig, e model.Engine, msg string) *model.Verdict {
	if rc.OnMissing == config.OnMissingSkipWarn {
		return &model.Verdict{Engine: e, Pass: true, Category: model.CatSkipped, Message: msg}
	}
	return &model.Verdict{Engine: e, Pass: false, Category: model.CatPrerequisiteMissing, Message: msg}
}
