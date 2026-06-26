// Package orchestrator drives the selected scanners: it runs and collects every
// engine's results FIRST (the reports-before-gating barrier), then parses and
// evaluates them, then aggregates a single exit code. Sequential by default;
// --parallel runs engines concurrently.
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
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

	// stageCfg returns the config for an engine if it is in this run, else nil.
	stageCfg := func(engine model.Engine) *scanner.Config {
		for _, st := range stages {
			if st.engine == engine {
				return st.cfg
			}
		}
		return nil
	}
	// orderContainersBeforeSCA moves the SCA stage to immediately follow the
	// Containers stage (stable for all other engines), so sequential scheduling
	// runs Containers first; the gate enforces the same order under --parallel.
	orderContainersBeforeSCA := func(in []*staged) []*staged {
		var sca *staged
		withoutSCA := make([]*staged, 0, len(in))
		for _, st := range in {
			if st.engine == model.EngineSCA {
				sca = st
				continue
			}
			withoutSCA = append(withoutSCA, st)
		}
		out := make([]*staged, 0, len(in))
		for _, st := range withoutSCA {
			out = append(out, st)
			if st.engine == model.EngineContainers && sca != nil {
				out = append(out, sca)
			}
		}
		return out
	}

	// Cx1 backend de-confliction: SCA and Container Security scans that target the
	// SAME Cx1 project must not be initiated concurrently — the backend
	// intermittently fails one when both start in close proximity against one
	// project. When both engines run against the same effective project name
	// (honoring any per-engine --project-name passthrough override), serialize the
	// pair: Containers first, then SCA after it. All other engines stay parallel.
	var scaGate chan struct{} // closed when Containers finishes; nil = no serialization
	if scaCfg, contCfg := stageCfg(model.EngineSCA), stageCfg(model.EngineContainers); scaCfg != nil && contCfg != nil {
		if p := effectiveCxProject(scaCfg); p != "" && p == effectiveCxProject(contCfg) {
			scaGate = make(chan struct{})
			stages = orderContainersBeforeSCA(stages)
			if display != nil {
				fmt.Fprintf(display, "[orchestrator] SCA and Container Security target the same Cx1 project %q — serializing (containers first, then sca)\n", p)
			}
		}
	}
	// waitForDep blocks the SCA stage until Containers has finished (or the run is
	// cancelled). signalDone closes the gate once Containers finishes.
	waitForDep := func(st *staged) {
		if scaGate != nil && st.engine == model.EngineSCA {
			select {
			case <-scaGate:
			case <-runCtx.Done():
			}
		}
	}
	signalDone := func(st *staged) {
		if scaGate != nil && st.engine == model.EngineContainers {
			close(scaGate)
		}
	}

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
			waitForDep(st)
			runStage(st)
			signalDone(st)
		}
	} else {
		sem := make(chan struct{}, conc)
		var wg sync.WaitGroup
		for _, st := range stages {
			wg.Add(1)
			go func(st *staged) {
				defer wg.Done()
				// Wait for any dependency (SCA→Containers) BEFORE taking a slot,
				// so a blocked SCA never starves Containers of the semaphore.
				waitForDep(st)
				sem <- struct{}{}
				defer func() { <-sem }()
				runStage(st)
				signalDone(st)
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

// effectiveCxProject returns the Cx1 project name an engine will actually scan
// against: cfg.ProjectName, unless a raw --project-name passthrough overrides it.
// cx uses the LAST --project-name on the argv and raw (Tier-B) args are appended
// last, so a raw override wins. Both forms are handled: "--project-name=VALUE"
// (one =-bound token) and "--project-name" "VALUE" (two tokens).
func effectiveCxProject(cfg *scanner.Config) string {
	name := cfg.ProjectName
	for i := 0; i < len(cfg.RawArgs); i++ {
		a := cfg.RawArgs[i]
		switch {
		case strings.HasPrefix(a, "--project-name="):
			name = strings.TrimPrefix(a, "--project-name=")
		case a == "--project-name" && i+1 < len(cfg.RawArgs):
			name = cfg.RawArgs[i+1]
			i++
		}
	}
	return strings.TrimSpace(name)
}

// unavailable builds the verdict for a missing/unavailable engine, honoring the
// --on-missing policy.
func unavailable(rc *config.RunConfig, e model.Engine, msg string) *model.Verdict {
	if rc.OnMissing == config.OnMissingSkipWarn {
		return &model.Verdict{Engine: e, Pass: true, Category: model.CatSkipped, Message: msg}
	}
	return &model.Verdict{Engine: e, Pass: false, Category: model.CatPrerequisiteMissing, Message: msg}
}
