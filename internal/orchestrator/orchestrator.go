// Package orchestrator drives the selected scanners: it runs and collects every
// engine's results FIRST (the reports-before-gating barrier), then parses and
// evaluates them, then aggregates a single exit code. Sequential by default;
// --parallel runs engines concurrently.
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sync"

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
	stages := make([]*staged, len(rc.Scanners))
	for i, e := range rc.Scanners {
		stages[i] = &staged{engine: e, cfg: rc.EngineConfigs[e]}
	}

	// runCtx lets --fail-fast cancel the other in-flight engines on the first
	// execution error (parallel mode).
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// runStage performs everything up to and including the child scan for one
	// engine, mutating ONLY its own *staged — so it is safe to run concurrently.
	runStage := func(st *staged) {
		e := st.engine
		if !scanner.Registered(e) {
			st.verdict = unavailable(rc, e, "no scanner registered for this engine")
			return
		}
		sc, err := scanner.Get(e)
		if err != nil {
			st.verdict = unavailable(rc, e, err.Error())
			return
		}
		st.sc = sc
		if err := sc.Available(runCtx, st.cfg); err != nil {
			st.verdict = unavailable(rc, e, err.Error())
			return
		}
		thPlan := rc.Threshold.Partition(e)
		inv, err := sc.BuildInvocation(st.cfg, thPlan)
		if err != nil {
			st.verdict = &model.Verdict{Engine: e, Category: model.CatConfigError, Message: err.Error()}
			return
		}
		if err := os.MkdirAll(inv.OutputDir, 0o755); err != nil {
			st.verdict = &model.Verdict{Engine: e, Category: model.CatEngineFailure, Message: fmt.Sprintf("cannot create output dir: %v", err)}
			return
		}
		opts := execpkg.Options{Display: display, Prefix: fmt.Sprintf("[%s] ", e)}
		st.result = sc.Run(runCtx, inv, opts)
		st.result.Engine = e
		st.result.Ran = true
		st.result.Route = thPlan.Route
		if rc.FailFast && st.result.Err != nil {
			cancel() // stop other in-flight engines
		}
	}

	// Phase 1 (the reports-before-gating barrier): run + collect every engine,
	// either sequentially (--parallel 0) or up to N concurrently. Concurrency is
	// safe because each goroutine mutates only its own stage and child output is
	// serialized line-by-line by internal/exec.
	conc := rc.Parallel
	if conc <= 0 {
		conc = 1
	}
	if conc > len(stages) {
		conc = len(stages)
	}
	if conc <= 1 {
		for _, st := range stages {
			if rc.FailFast && runCtx.Err() != nil {
				break
			}
			runStage(st)
		}
	} else {
		sem := make(chan struct{}, conc)
		var wg sync.WaitGroup
		for _, st := range stages {
			wg.Add(1)
			go func(st *staged) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				runStage(st)
			}(st)
		}
		wg.Wait()
	}

	// Phase 2 (synced, after the barrier): parse + evaluate in deterministic
	// engine order; the caller then aggregates a single exit code.
	for _, st := range stages {
		if st.verdict != nil { // short-circuited (skipped/prereq/config)
			out.Verdicts = append(out.Verdicts, *st.verdict)
			if st.result != nil {
				out.Results = append(out.Results, st.result)
			}
			continue
		}
		if st.result == nil {
			continue // not reached (cancelled before this engine started)
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
