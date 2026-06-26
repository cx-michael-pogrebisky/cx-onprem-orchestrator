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

// tracker records the maximum number of engines running concurrently and the
// ordered start/end events (for asserting serialization).
type tracker struct {
	mu       sync.Mutex
	cur, max int
	events   []string
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
func (t *tracker) record(s string) { t.mu.Lock(); t.events = append(t.events, s); t.mu.Unlock() }
func (t *tracker) order() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.events...)
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
	f.tr.record(string(f.eng) + ":start")
	time.Sleep(80 * time.Millisecond)
	f.tr.record(string(f.eng) + ":end")
	f.tr.leave()
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
	rc.ProjectName = "proj"
	base := t.TempDir()
	for _, e := range engines {
		rc.EngineConfigs[e] = &scanner.Config{Engine: e, ProjectName: "proj", OutputDir: filepath.Join(base, string(e))}
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

func indexOf(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}

// scaContIaC keeps SCA first in selection order to prove the orchestrator
// reorders Containers ahead of SCA (not relying on selection order).
var scaContIaC = []model.Engine{model.EngineSCA, model.EngineContainers, model.EngineIaC}

func TestRun_SerializesContainersBeforeSCA_SameProject(t *testing.T) {
	tr := &tracker{}
	registerFakes(tr, scaContIaC...)
	// parallel 4 would otherwise run all three at once; same project forces the
	// SCA/Containers pair to serialize (containers first), IaC stays parallel.
	oc := orchestrator.Run(context.Background(), runConfig(t, 4, scaContIaC...), nil)
	if len(oc.Verdicts) != 3 {
		t.Fatalf("want 3 verdicts, got %d", len(oc.Verdicts))
	}
	ev := tr.order()
	ce, ss := indexOf(ev, "containers:end"), indexOf(ev, "sca:start")
	if ce < 0 || ss < 0 || ce > ss {
		t.Errorf("containers must finish before sca starts; events=%v", ev)
	}
}

func TestRun_DifferentProject_NotSerialized(t *testing.T) {
	tr := &tracker{}
	registerFakes(tr, scaContIaC...)
	rc := runConfig(t, 4, scaContIaC...)
	// A per-engine --project-name passthrough makes SCA target a different Cx1
	// project, so the pair must NOT be serialized (they may overlap).
	rc.EngineConfigs[model.EngineSCA].RawArgs = []string{"--project-name=proj-sca"}
	oc := orchestrator.Run(context.Background(), rc, nil)
	if len(oc.Verdicts) != 3 {
		t.Fatalf("want 3 verdicts, got %d", len(oc.Verdicts))
	}
	if tr.peak() < 2 {
		t.Errorf("different projects should allow concurrency (peak >= 2), got %d", tr.peak())
	}
}

func TestRun_SerializesContainersBeforeSCA_Sequential(t *testing.T) {
	tr := &tracker{}
	registerFakes(tr, scaContIaC...)
	// Even sequential (parallel 0) must run Containers before SCA despite SCA
	// appearing first in selection order (the reorder guarantees it).
	orchestrator.Run(context.Background(), runConfig(t, 0, scaContIaC...), nil)
	ev := tr.order()
	if ce, ss := indexOf(ev, "containers:end"), indexOf(ev, "sca:start"); ce < 0 || ss < 0 || ce > ss {
		t.Errorf("sequential: containers must finish before sca starts; events=%v", ev)
	}
}
