package simulations

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
)

// --- fakes ----------------------------------------------------------------------

// fakeSimRepo is an in-memory SimulationsRepository for the engine tests. It only
// implements the methods the run loop touches (GetByID + Update); the rest return
// zero values so the interface is satisfied.
type fakeSimRepo struct {
	mu   sync.Mutex
	sims map[string]*datasims.Simulation
	// listFilters records every filter List was called with (reconcile
	// assertions).
	listFilters []datasims.ListFilter
}

func newFakeSimRepo(sims ...*datasims.Simulation) *fakeSimRepo {
	r := &fakeSimRepo{sims: map[string]*datasims.Simulation{}}
	for _, s := range sims {
		cp := *s
		r.sims[s.ID] = &cp
	}
	return r
}

func (r *fakeSimRepo) Create(ctx context.Context, sim datasims.Simulation) (string, error) {
	return "", nil
}

func (r *fakeSimRepo) GetByID(ctx context.Context, id string) (*datasims.Simulation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sims[id]
	if !ok {
		return nil, common.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *fakeSimRepo) Update(ctx context.Context, id string, sim datasims.Simulation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := sim
	r.sims[id] = &cp
	return nil
}

func (r *fakeSimRepo) Delete(ctx context.Context, id string) error { return nil }

func (r *fakeSimRepo) List(ctx context.Context, f datasims.ListFilter) ([]datasims.SimulationSummary, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listFilters = append(r.listFilters, f)
	out := make([]datasims.SimulationSummary, 0, len(r.sims))
	for _, s := range r.sims {
		if len(f.Statuses) > 0 && !slices.Contains(f.Statuses, s.Status) {
			continue
		}
		out = append(out, datasims.SimulationSummary{
			ID:     s.ID,
			Name:   s.Name,
			Status: s.Status,
			Steps:  s.Steps,
		})
	}
	return out, false, nil
}

func (r *fakeSimRepo) Count(ctx context.Context, f datasims.ListFilter) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.sims)), nil
}

// snapshot returns a deep-enough copy of the stored simulation for assertions.
func (r *fakeSimRepo) snapshot(id string) *datasims.Simulation {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sims[id]
	if !ok {
		return nil
	}
	cp := *s
	return &cp
}

// setStatus mutates the stored status directly (test setup for reconcile).
func (r *fakeSimRepo) setStatus(id string, status datasims.SimulationStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sims[id]; ok {
		s.Status = status
	}
}

// fakeStepSink is an in-memory SimulationStepsRepository capturing SaveSteps batches.
// GetStepRange is programmable via rangeMin/rangeMax/rangeErr (defaults to (0,
// 0, nil) so existing engine tests that never call it are unaffected).
type fakeStepSink struct {
	mu    sync.Mutex
	saved []datasims.SimulationStep

	rangeMin, rangeMax int64
	rangeErr           error
	rangeCalls         int
}

func (f *fakeStepSink) SaveSteps(ctx context.Context, steps []datasims.SimulationStep) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, steps...)
	return nil
}

func (f *fakeStepSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.saved)
}

func (f *fakeStepSink) GetStepRange(ctx context.Context, simulationID string) (int64, int64, error) {
	f.mu.Lock()
	f.rangeCalls++
	f.mu.Unlock()
	return f.rangeMin, f.rangeMax, f.rangeErr
}
func (f *fakeStepSink) GetSteps(ctx context.Context, simID string, from, to, page, pageSize uint64) ([]datasims.SimulationStep, bool, error) {
	return nil, false, nil
}
func (f *fakeStepSink) AggregatePropertyByStep(ctx context.Context, simulationID, propertyUID string, agentUIDs []string, stepRange *datasims.StepRange, aggregation datasims.Aggregation) ([]datasims.TimeSeriesPoint, error) {
	return nil, nil
}
func (f *fakeStepSink) CountByAgentTypePerStep(ctx context.Context, simulationID string, stepRange *datasims.StepRange) ([]datasims.PopulationPoint, error) {
	return nil, nil
}
func (f *fakeStepSink) FindSpatialAtStep(ctx context.Context, simulationID string, step int64, propertyUIDs []string) ([]datasims.SpatialAgentData, error) {
	return nil, nil
}
func (f *fakeStepSink) FindDistinctProperties(ctx context.Context, simulationID string) ([]datasims.PropertyInfo, error) {
	return nil, nil
}
func (f *fakeStepSink) AggregateShapePropertyByStep(ctx context.Context, simulationID, propertyUID string, shapeUIDs []string, stepRange *datasims.StepRange, aggregation datasims.Aggregation) ([]datasims.TimeSeriesPoint, error) {
	return nil, nil
}
func (f *fakeStepSink) FindDistinctShapeProperties(ctx context.Context, simulationID string) ([]datasims.PropertyInfo, error) {
	return nil, nil
}
func (f *fakeStepSink) DeleteBySimulation(ctx context.Context, simulationID string) error {
	return nil
}

// fakeBroadcaster records broadcast calls and signals completion.
type fakeBroadcaster struct {
	mu         sync.Mutex
	opened     bool
	steps      int
	stepUsages int
	lastUsage  *SimulationTokenUsageDTO
	lastDelta  StreamTokenDelta
	keyframes  int
	statuses   []datasims.SimulationStatus
	completion datasims.SimulationStatus
	closed     bool
	doneCh     chan struct{}
}

func newFakeBroadcaster() *fakeBroadcaster {
	return &fakeBroadcaster{doneCh: make(chan struct{}, 1)}
}

func (b *fakeBroadcaster) OpenRoom(simID string, batchSize int) {
	b.mu.Lock()
	b.opened = true
	b.mu.Unlock()
}
func (b *fakeBroadcaster) BroadcastStep(simID string, step datasims.SimulationStep, changedOnly bool) {
	b.mu.Lock()
	b.steps++
	b.mu.Unlock()
}
func (b *fakeBroadcaster) BroadcastStepUsage(simID string, step uint64, usage *SimulationTokenUsageDTO, delta StreamTokenDelta) {
	b.mu.Lock()
	b.stepUsages++
	b.lastUsage = usage
	b.lastDelta = delta
	b.mu.Unlock()
}
func (b *fakeBroadcaster) BroadcastKeyframe(simID string, rs *RuntimeState) {
	b.mu.Lock()
	b.keyframes++
	b.mu.Unlock()
}
func (b *fakeBroadcaster) BroadcastStatus(simID string, status datasims.SimulationStatus) {
	b.mu.Lock()
	b.statuses = append(b.statuses, status)
	b.mu.Unlock()
}
func (b *fakeBroadcaster) BroadcastCompletion(simID string, status datasims.SimulationStatus) {
	b.mu.Lock()
	b.completion = status
	b.mu.Unlock()
	select {
	case b.doneCh <- struct{}{}:
	default:
	}
}
func (b *fakeBroadcaster) CloseRoom(simID string) {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
}

func (b *fakeBroadcaster) finalStatus() datasims.SimulationStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.completion
}

func (b *fakeBroadcaster) roomOpened() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.opened
}

func (b *fakeBroadcaster) roomClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

func (b *fakeBroadcaster) stepCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.steps
}

// waitDone blocks until BroadcastCompletion fires or the deadline elapses.
func (b *fakeBroadcaster) waitDone(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case <-b.doneCh:
	case <-time.After(d):
		t.Fatalf("run did not complete within %s", d)
	}
}

// --- fixtures -------------------------------------------------------------------

// runnableSim builds a ready, rules-only spatial simulation with n movers and the
// given step count. It reuses the step-pipeline rulesOnlyModel so the run loop
// drives real RunStep ticks with no LLM.
func runnableSim(id string, steps uint64, n int) *datasims.Simulation {
	model := rulesOnlyModel()
	sim := &datasims.Simulation{
		ID:    id,
		Seed:  42,
		Steps: steps,
		Environment: &datasims.SimulationEnvironment{
			Width: 100, Height: 80, Torus: true,
		},
		ModelSnapshot: model,
		Status:        datasims.SimulationStatusReady,
		StatusText:    "ready",
		TokenUsage:    &datasims.SimulationTokenUsage{},
	}
	for i := 0; i < n; i++ {
		sim.Agents = append(sim.Agents, datasims.SimulationAgent{
			ID:         uint64(i + 1),
			AgentUID:   "mover",
			State:      "idle",
			Position:   &common.Point{X: uint64(10 + 7*i), Y: uint64(10 + 5*i)},
			Properties: map[string]interface{}{"energy": float64(100)},
		})
	}
	return sim
}

// fastRunConfig zeroes the inter-step delay and flush interval so the run loop
// runs to completion immediately in tests.
func fastRunConfig() RunConfig {
	return RunConfig{
		MaxConcurrentRuns:   4,
		StepFlushSize:       2,
		StepFlushIntervalMs: 0,
		DefaultStepDelayMs:  0,
	}
}

func newTestManager(repo datasims.SimulationsRepository, steps datasims.SimulationStepsRepository, b stepBroadcaster) *RunManager {
	return NewRunManager(fastRunConfig(), repo, steps, nil, NewEventsHub(), b, nil)
}

// --- tests ----------------------------------------------------------------------

func TestRunManager_RunsToFinished(t *testing.T) {
	withFixedClock(t)
	sim := runnableSim("sim-fin", 3, 2)
	repo := newFakeSimRepo(sim)
	sink := &fakeStepSink{}
	b := newFakeBroadcaster()
	m := newTestManager(repo, sink, b)

	if err := m.Start(context.Background(), sim); err != nil {
		t.Fatalf("Start: %v", err)
	}
	b.waitDone(t, 2*time.Second)

	got := repo.snapshot("sim-fin")
	if got.Status != datasims.SimulationStatusFinished {
		t.Fatalf("status=%q want finished", got.Status)
	}
	if got.CurrentStep != 3 {
		t.Fatalf("CurrentStep=%d want 3", got.CurrentStep)
	}
	if len(got.ExecutionRuns) != 1 {
		t.Fatalf("ExecutionRuns=%d want 1", len(got.ExecutionRuns))
	}
	run := got.ExecutionRuns[0]
	if run.Status != datasims.ExecutionRunFinished {
		t.Fatalf("run status=%q want finished", run.Status)
	}
	if run.StepsCompleted != 3 {
		t.Fatalf("run StepsCompleted=%d want 3", run.StepsCompleted)
	}
	if run.FinishedAt == nil {
		t.Fatal("run FinishedAt should be set")
	}
	// A run with no pauses: active duration equals the whole wall clock and is
	// never negative. Guards the finalize call site (the pause-exclusion math is
	// covered exhaustively by TestActiveRunDuration).
	if run.ActiveDuration < 0 {
		t.Fatalf("run ActiveDuration=%v must not be negative", run.ActiveDuration)
	}
	if want := run.FinishedAt.Sub(run.StartedAt); run.ActiveDuration != want {
		t.Fatalf("run ActiveDuration=%v want %v (no pauses ⇒ full wall clock)", run.ActiveDuration, want)
	}
	if got.Statistics == nil {
		t.Fatal("Statistics should be persisted on finish")
	}
	if b.finalStatus() != datasims.SimulationStatusFinished {
		t.Fatalf("broadcast completion=%q want finished", b.finalStatus())
	}
	// Room lifecycle is owned by the run: opened at start, torn down on completion.
	if !b.roomOpened() {
		t.Fatal("OpenRoom should be called at run start (RunManager owns room lifecycle)")
	}
	if !b.roomClosed() {
		t.Fatal("CloseRoom should be called on completion")
	}
	// Steps were persisted (2 agents × 3 ticks = 6 agent rows minimum).
	if sink.count() == 0 {
		t.Fatal("no steps persisted")
	}
	if m.IsRunning("sim-fin") {
		t.Fatal("handle should be cleared after finish")
	}
}

func TestRunManager_PauseResumeStop(t *testing.T) {
	withFixedClock(t)
	// A long run so we can pause/resume/stop before it finishes on its own.
	sim := runnableSim("sim-ctl", 100000, 1)
	repo := newFakeSimRepo(sim)
	sink := &fakeStepSink{}
	b := newFakeBroadcaster()
	// Non-zero delay so the loop is still alive across control messages.
	cfg := fastRunConfig()
	cfg.DefaultStepDelayMs = 5
	m := NewRunManager(cfg, repo, sink, nil, NewEventsHub(), b, nil)

	if err := m.Start(context.Background(), sim); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Pause: the loop should stop advancing.
	if err := m.Control("sim-ctl", controlMsg{action: ctrlPause}); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !waitForStatus(repo, "sim-ctl", datasims.SimulationStatusPaused, time.Second) {
		t.Fatalf("expected paused status, got %q", repo.snapshot("sim-ctl").Status)
	}
	pausedAt := repo.snapshot("sim-ctl").CurrentStep
	time.Sleep(50 * time.Millisecond)
	if step := repo.snapshot("sim-ctl").CurrentStep; step != pausedAt {
		t.Fatalf("paused run advanced from %d to %d", pausedAt, step)
	}

	// Resume: the loop advances again.
	if err := m.Control("sim-ctl", controlMsg{action: ctrlResume}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !waitForStatus(repo, "sim-ctl", datasims.SimulationStatusRunning, time.Second) {
		t.Fatalf("expected running status after resume, got %q", repo.snapshot("sim-ctl").Status)
	}

	// Stop: the loop ends with stopped status.
	if err := m.Control("sim-ctl", controlMsg{action: ctrlStop}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	b.waitDone(t, 2*time.Second)
	got := repo.snapshot("sim-ctl")
	if got.Status != datasims.SimulationStatusStopped {
		t.Fatalf("status=%q want stopped", got.Status)
	}
	if len(got.ExecutionRuns) != 1 || got.ExecutionRuns[0].Status != datasims.ExecutionRunStopped {
		t.Fatalf("execution run not finalized as stopped: %+v", got.ExecutionRuns)
	}

	// Control on a now-finished run is rejected.
	if err := m.Control("sim-ctl", controlMsg{action: ctrlPause}); err != ErrNotRunning {
		t.Fatalf("control after stop err=%v want ErrNotRunning", err)
	}
}

func TestRunManager_OneRunPerSim(t *testing.T) {
	withFixedClock(t)
	sim := runnableSim("sim-dup", 100000, 1)
	repo := newFakeSimRepo(sim)
	sink := &fakeStepSink{}
	b := newFakeBroadcaster()
	cfg := fastRunConfig()
	cfg.DefaultStepDelayMs = 5
	m := NewRunManager(cfg, repo, sink, nil, NewEventsHub(), b, nil)

	if err := m.Start(context.Background(), sim); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	t.Cleanup(func() {
		_ = m.Control("sim-dup", controlMsg{action: ctrlStop})
		b.waitDone(t, 2*time.Second)
	})

	// Second Start while the first is running → ErrRunInFlight. Use a fresh sim
	// pointer (as a real caller would, reloaded from the repo) so we never alias
	// the running loop's simulation.
	second := runnableSim("sim-dup", 100000, 1)
	if err := m.Start(context.Background(), second); err != ErrRunInFlight {
		t.Fatalf("second Start err=%v want ErrRunInFlight", err)
	}
}

func TestRunManager_RunCapacity(t *testing.T) {
	withFixedClock(t)
	cfg := fastRunConfig()
	cfg.MaxConcurrentRuns = 1
	cfg.DefaultStepDelayMs = 5

	simA := runnableSim("sim-a", 100000, 1)
	simB := runnableSim("sim-b", 100000, 1)
	repo := newFakeSimRepo(simA, simB)
	sink := &fakeStepSink{}
	b := newFakeBroadcaster()
	m := NewRunManager(cfg, repo, sink, nil, NewEventsHub(), b, nil)

	if err := m.Start(context.Background(), simA); err != nil {
		t.Fatalf("Start A: %v", err)
	}
	t.Cleanup(func() {
		_ = m.Control("sim-a", controlMsg{action: ctrlStop})
		b.waitDone(t, 2*time.Second)
	})

	if err := m.Start(context.Background(), simB); err != ErrRunCapacity {
		t.Fatalf("Start B err=%v want ErrRunCapacity", err)
	}
}

func TestRunManager_StartRejectsNotRunnable(t *testing.T) {
	withFixedClock(t)
	// Not ready.
	sim := runnableSim("sim-nr", 3, 1)
	sim.Status = datasims.SimulationStatusFinished
	repo := newFakeSimRepo(sim)
	m := newTestManager(repo, &fakeStepSink{}, newFakeBroadcaster())
	if err := m.Start(context.Background(), sim); err != ErrNotRunnable {
		t.Fatalf("not-ready Start err=%v want ErrNotRunnable", err)
	}

	// Zero steps.
	sim2 := runnableSim("sim-nostep", 0, 1)
	repo2 := newFakeSimRepo(sim2)
	m2 := newTestManager(repo2, &fakeStepSink{}, newFakeBroadcaster())
	if err := m2.Start(context.Background(), sim2); err != ErrNotRunnable {
		t.Fatalf("zero-steps Start err=%v want ErrNotRunnable", err)
	}

	// No agents.
	sim3 := runnableSim("sim-noagent", 3, 0)
	repo3 := newFakeSimRepo(sim3)
	m3 := newTestManager(repo3, &fakeStepSink{}, newFakeBroadcaster())
	if err := m3.Start(context.Background(), sim3); err != ErrNotRunnable {
		t.Fatalf("no-agents Start err=%v want ErrNotRunnable", err)
	}
}

func TestReconcileInterruptedRuns_MarksFailed(t *testing.T) {
	withFixedClock(t)
	running := runnableSim("sim-run", 10, 1)
	running.Status = datasims.SimulationStatusRunning
	running.ExecutionRuns = []datasims.ExecutionRun{
		{Seq: 1, Status: datasims.ExecutionRunRunning, StartedAt: stepFixedNow},
	}
	paused := runnableSim("sim-pause", 10, 1)
	paused.Status = datasims.SimulationStatusPaused
	ready := runnableSim("sim-ok", 10, 1) // untouched

	repo := newFakeSimRepo(running, paused, ready)
	m := newTestManager(repo, &fakeStepSink{}, newFakeBroadcaster())

	if err := m.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if got := repo.snapshot("sim-run"); got.Status != datasims.SimulationStatusFailed {
		t.Fatalf("running sim status=%q want failed", got.Status)
	}
	// The interrupted run record is finalized as failed.
	if r := repo.snapshot("sim-run").ExecutionRuns; len(r) != 1 || r[0].Status != datasims.ExecutionRunFailed {
		t.Fatalf("execution run not marked failed: %+v", r)
	}
	if got := repo.snapshot("sim-pause"); got.Status != datasims.SimulationStatusFailed {
		t.Fatalf("paused sim status=%q want failed", got.Status)
	}
	if got := repo.snapshot("sim-ok"); got.Status != datasims.SimulationStatusReady {
		t.Fatalf("ready sim status=%q want ready (untouched)", got.Status)
	}
}

// Reconcile must ask the repository for running/paused simulations only —
// paging through the whole collection at startup blew past the FX start
// timeout once the campaign database grew (U9).
func TestReconcile_ListsOnlyInterruptedStatuses(t *testing.T) {
	withFixedClock(t)
	repo := newFakeSimRepo(runnableSim("sim-ok", 10, 1))
	m := newTestManager(repo, &fakeStepSink{}, newFakeBroadcaster())

	if err := m.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(repo.listFilters) == 0 {
		t.Fatal("Reconcile issued no List calls")
	}
	want := []datasims.SimulationStatus{
		datasims.SimulationStatusRunning,
		datasims.SimulationStatusPaused,
	}
	for i, f := range repo.listFilters {
		if !slices.Equal(f.Statuses, want) {
			t.Fatalf("List call %d filter statuses = %v, want %v", i, f.Statuses, want)
		}
	}
}

// waitForStatus polls the repo until the simulation reaches status or the
// deadline elapses.
func waitForStatus(repo *fakeSimRepo, id string, status datasims.SimulationStatus, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if s := repo.snapshot(id); s != nil && s.Status == status {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return false
}
