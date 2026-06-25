package orchestrator_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/config"
	execpkg "github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/exec"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/model"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/orchestrator"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/scanner"
	"github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/threshold"
)

// tracker records the maximum number of engines running concurrently.
type tracker struct {
	mu       sync.Mutex
	cur, max int
}

func (t *tracker) enter() {
	t.mu.Lock()
	t.cur++
	if t.cur > t.max {
		t.max = t.cur
	}
	t.mu.Unlock()
}
func (t *tracker) leave() { t.mu.Lock(); t.cur--; t.mu.Unlock() }
func (t *tracker) peak() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.max
}

type fakeScanner struct {
	eng model.Engine
	tr  *tracker
}

func (f *fakeScanner) Engine() model.Engine                              { return f.eng }
func (f *fakeScanner) Available(context.Context, *scanner.Config) error  { return nil }
func (f *fakeScanner) ParseResults(context.Context, *model.Result) error { return nil }
func (f *fakeScanner) BuildInvocation(cfg *scanner.Config, _ threshold.Plan) (*model.Invocation, error) {
	return &model.Invocation{Engine: f.eng, Path: "true", OutputDir: cfg.OutputDir}, nil
}
func (f *fakeScanner) Run(_ context.Context, inv *model.Invocation, _ execpkg.Options) *model.Result {
	f.tr.enter()
	defer f.tr.leave()
	time.Sleep(80 * time.Millisecond)
	return &model.Result{Engine: inv.Engine, Ran: true, ChildExitCode: 0}
}
func (f *fakeScanner) Evaluate(_ *model.Result, _ threshold.Plan) model.Verdict {
	return model.Verdict{Engine: f.eng, Pass: true, Category: model.CatPass}
}

func registerFakes(tr *tracker, engines ...model.Engine) {
	for _, e := range engines {
		e := e
		scanner.Register(e, func() scanner.Scanner { return &fakeScanner{eng: e, tr: tr} })
	}
}

func runConfig(t *testing.T, parallel int, engines ...model.Engine) *config.RunConfig {
	rc := &config.RunConfig{Scanners: engines, Parallel: parallel, OnMissing: config.OnMissingFail, EngineConfigs: map[model.Engine]*scanner.Config{}}
	base := t.TempDir()
	for _, e := range engines {
		rc.EngineConfigs[e] = &scanner.Config{Engine: e, OutputDir: filepath.Join(base, string(e))}
	}
	return rc
}

var threeEngines = []model.Engine{model.EngineSAST, model.EngineSCA, model.EngineIaC}

func TestRun_Parallel_RunsConcurrentlyAndSyncs(t *testing.T) {
	tr := &tracker{}
	registerFakes(tr, threeEngines...)
	oc := orchestrator.Run(context.Background(), runConfig(t, 3, threeEngines...), nil)

	if len(oc.Verdicts) != 3 || len(oc.Results) != 3 {
		t.Fatalf("want 3 verdicts+results (synced), got %d/%d", len(oc.Verdicts), len(oc.Results))
	}
	if tr.peak() < 2 {
		t.Errorf("expected concurrent execution (peak >= 2), got %d", tr.peak())
	}
	// Verdicts are returned in deterministic engine order regardless of finish order.
	want := threeEngines
	for i, v := range oc.Verdicts {
		if v.Engine != want[i] {
			t.Errorf("verdict[%d] = %s, want %s (order not preserved)", i, v.Engine, want[i])
		}
	}
}

func TestRun_Sequential_NoConcurrency(t *testing.T) {
	tr := &tracker{}
	registerFakes(tr, threeEngines...)
	oc := orchestrator.Run(context.Background(), runConfig(t, 0, threeEngines...), nil)
	if len(oc.Verdicts) != 3 {
		t.Fatalf("want 3 verdicts, got %d", len(oc.Verdicts))
	}
	if tr.peak() != 1 {
		t.Errorf("sequential run should have peak concurrency 1, got %d", tr.peak())
	}
}
