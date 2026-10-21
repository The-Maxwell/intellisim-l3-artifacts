package simulations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/llm"
)

// --- fake SimulationsRepository ------------------------------------------------

type fakeSimsRepo struct {
	mu         sync.Mutex
	seq        int
	store      map[string]datasims.Simulation
	lastFilter datasims.ListFilter
	// updateErr, when set, makes Update fail with this error (and skip the
	// store write) so persist-failure paths can be exercised.
	updateErr error
}

func newFakeSimsRepo() *fakeSimsRepo {
	return &fakeSimsRepo{store: map[string]datasims.Simulation{}}
}

func (r *fakeSimsRepo) Create(_ context.Context, sim datasims.Simulation) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	id := fmt.Sprintf("%024d", r.seq)
	sim.ID = id
	r.store[id] = sim
	return id, nil
}

func (r *fakeSimsRepo) GetByID(_ context.Context, id string) (*datasims.Simulation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.store[id]
	if !ok {
		return nil, common.ErrNotFound
	}
	return &s, nil
}

func (r *fakeSimsRepo) Update(_ context.Context, id string, sim datasims.Simulation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	sim.ID = id
	r.store[id] = sim
	return nil
}

func (r *fakeSimsRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, id)
	return nil
}

func (r *fakeSimsRepo) List(_ context.Context, f datasims.ListFilter) ([]datasims.SimulationSummary, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastFilter = f
	out := make([]datasims.SimulationSummary, 0, len(r.store))
	for _, s := range r.store {
		if !matchesListFilter(f, s) {
			continue
		}
		out = append(out, datasims.SimulationSummary{
			ID:        s.ID,
			Name:      s.Name,
			ModelID:   s.ModelID,
			Status:    s.Status,
			Steps:     s.Steps,
			CreatedBy: s.CreatedBy,
		})
	}
	return out, false, nil
}

func (r *fakeSimsRepo) Count(_ context.Context, f datasims.ListFilter) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, s := range r.store {
		if matchesListFilter(f, s) {
			n++
		}
	}
	return n, nil
}

// matchesListFilter mirrors the mongo repo's owner + multi-model match.
func matchesListFilter(f datasims.ListFilter, s datasims.Simulation) bool {
	if f.CreatedBy != "" && s.CreatedBy != f.CreatedBy {
		return false
	}
	if len(f.ModelIDs) > 0 {
		for _, id := range f.ModelIDs {
			if id == s.ModelID {
				return true
			}
		}
		return false
	}
	return true
}

func (r *fakeSimsRepo) get(id string) datasims.Simulation {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.store[id]
}

// --- fake ModelsRepository -----------------------------------------------------

type fakeModelsRepo struct {
	mu    sync.Mutex
	store map[string]datamodels.Model
}

func newFakeModelsRepo() *fakeModelsRepo {
	return &fakeModelsRepo{store: map[string]datamodels.Model{}}
}

func (r *fakeModelsRepo) Create(_ context.Context, model datamodels.Model) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := fmt.Sprintf("m%023d", len(r.store)+1)
	model.ID = id
	r.store[id] = model
	return id, nil
}

func (r *fakeModelsRepo) GetByID(_ context.Context, id string) (*datamodels.Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.store[id]
	if !ok {
		return nil, common.ErrNotFound
	}
	return &m, nil
}

func (r *fakeModelsRepo) Update(_ context.Context, id string, model datamodels.Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	model.ID = id
	r.store[id] = model
	return nil
}

func (r *fakeModelsRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, id)
	return nil
}

func (r *fakeModelsRepo) List(_ context.Context, _ datamodels.ListFilter) ([]datamodels.Model, bool, error) {
	return nil, false, nil
}

func (r *fakeModelsRepo) Count(_ context.Context, _ datamodels.ListFilter) (int64, error) {
	return 0, nil
}

func (r *fakeModelsRepo) ListOptions(_ context.Context, _ string) ([]datamodels.ModelOption, error) {
	return nil, nil
}

func (r *fakeModelsRepo) put(m datamodels.Model) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[m.ID] = m
}

// --- fake llm.Service ----------------------------------------------------------

// fakeSvcLLM satisfies the full llm.Service interface for the service tests. It
// wraps the same Simple/Reasoning behaviour as the generator fake but also adds
// the provider-discovery methods the service's llm.Service field requires.
type fakeSvcLLM struct {
	validateErr error
}

func (f *fakeSvcLLM) Simple(_ context.Context, req llm.SimpleRequest) (*llm.Completion, error) {
	return &llm.Completion{Text: "{}", FinishReason: "stop", Usage: usageFor(req.Target)}, nil
}

func (f *fakeSvcLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.Completion, error) {
	return &llm.Completion{Text: "{}", FinishReason: "stop", Usage: usageFor(req.Target)}, nil
}

func (f *fakeSvcLLM) Vision(_ context.Context, req llm.VisionRequest) (*llm.Completion, error) {
	return &llm.Completion{Text: "{}", FinishReason: "stop", Usage: usageFor(req.Target)}, nil
}

func (f *fakeSvcLLM) Reasoning(_ context.Context, req llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
	return &llm.ReasoningCompletion{Analysis: "{}", FinishReason: "stop", Usage: usageFor(req.Target)}, nil
}

func (f *fakeSvcLLM) Providers() []llm.ProviderInfo                                   { return nil }
func (f *fakeSvcLLM) ProvidersByCapability(common.ModelCapability) []llm.ProviderInfo { return nil }
func (f *fakeSvcLLM) ProvidersByModel(string) []llm.ProviderInfo                      { return nil }
func (f *fakeSvcLLM) ModelDescriptors(string, string) ([]llm.ConfigParamDescriptor, bool, bool) {
	return nil, false, true
}
func (f *fakeSvcLLM) MaxImages(string, string) (uint32, bool) { return 0, false }
func (f *fakeSvcLLM) ValidateTarget(common.LLMTarget, common.ModelCapability) error {
	return f.validateErr
}

// --- harness -------------------------------------------------------------------

func newTestService(t *testing.T, simsRepo datasims.SimulationsRepository, modelsRepo datamodels.ModelsRepository, fake *fakeSvcLLM) (*service, *EventsHub) {
	t.Helper()
	hub := NewEventsHub()
	cfg := ModuleConfig{GenerationTimeout: 5 * time.Second, LLMBatchSize: 40}
	gen := NewGenerator(fake, cfg)
	svc := NewService(simsRepo, modelsRepo, fake, gen, &fakeAnalysesRepo{}, &fakeAnalysisFileStore{}, &fakeStepSink{}, hub, cfg, zap.NewNop()).(*service)
	return svc, hub
}

// fakeAnalysesRepo is a no-op AnalysesRepository for the create/edit service tests
// (the cascade-delete path only needs it to satisfy the dependency).
type fakeAnalysesRepo struct{}

func (f *fakeAnalysesRepo) Create(context.Context, datasims.Analysis) (string, error) { return "", nil }
func (f *fakeAnalysesRepo) GetByID(context.Context, string) (*datasims.Analysis, error) {
	return nil, common.ErrNotFound
}
func (f *fakeAnalysesRepo) Update(context.Context, string, datasims.Analysis) error { return nil }
func (f *fakeAnalysesRepo) Delete(context.Context, string) error                    { return nil }
func (f *fakeAnalysesRepo) ListBySimulation(context.Context, string) ([]datasims.Analysis, error) {
	return nil, nil
}
func (f *fakeAnalysesRepo) DeleteBySimulationID(context.Context, string) error { return nil }
func (f *fakeAnalysesRepo) FindUnfinished(context.Context) ([]datasims.Analysis, error) {
	return nil, nil
}

// fakeAnalysisFileStore is a no-op AnalysisFileStore for the create/edit service tests.
type fakeAnalysisFileStore struct{}

func (f *fakeAnalysisFileStore) Upload(context.Context, string, string, string, string, []byte) (string, int64, error) {
	return "", 0, nil
}
func (f *fakeAnalysisFileStore) Open(context.Context, string) (io.ReadCloser, *datasims.AnalysisFileInfo, error) {
	return nil, nil, common.ErrNotFound
}
func (f *fakeAnalysisFileStore) Delete(context.Context, string) error             { return nil }
func (f *fakeAnalysisFileStore) DeleteBySimulation(context.Context, string) error { return nil }

func ctxAs(id string, admin bool) context.Context {
	return auth.WithPrincipal(context.Background(), &auth.UserPrincipal{ID: id, IsAdmin: admin})
}

// readyModel builds a minimal simulation-ready model owned by owner.
func readyModel(id, owner string) datamodels.Model {
	return datamodels.Model{
		ID:        id,
		Name:      "Predator-Prey",
		CreatedBy: owner,
		Agents: []datamodels.Agent{
			{
				UID:    "prey",
				Name:   "Prey",
				Type:   datamodels.AgentTypePlain,
				States: []string{"alive", "dead"},
				Properties: []datamodels.AgentProperty{
					{UID: "energy", Name: "Energy", Type: datamodels.PropertyTypeNumber},
				},
				Initialization: datamodels.AgentInitialization{PopulationCount: 5},
			},
		},
	}
}

// readySim builds a ready simulation owned by owner referencing the given model.
func readySim(id, owner string, model datamodels.Model) datasims.Simulation {
	m := model
	now := time.Now().UTC()
	return datasims.Simulation{
		ID:            id,
		Name:          "Sim",
		ModelID:       model.ID,
		ModelSnapshot: &m,
		Steps:         100,
		Status:        datasims.SimulationStatusReady,
		StatusText:    "ready",
		Agents: []datasims.SimulationAgent{
			{ID: 1, AgentUID: "prey", Name: "Prey #1", State: "alive"},
		},
		GenerationRuns: []datasims.GenerationRun{
			{Seq: 1, Scope: datasims.GenerationScope{Kind: datasims.ScopeFull}, Status: datasims.RunCompleted, StartedAt: now},
		},
		TokenUsage: &datasims.SimulationTokenUsage{},
		CreatedAt:  now,
		UpdatedAt:  now,
		CreatedBy:  owner,
		UpdatedBy:  owner,
	}
}

// --- tests ---------------------------------------------------------------------

func TestService_Update_NonEditableStatuses_ErrNotEditable(t *testing.T) {
	nonEditable := []datasims.SimulationStatus{
		datasims.SimulationStatusGenerating,
		datasims.SimulationStatusQueued,
		datasims.SimulationStatusRunning,
		datasims.SimulationStatusPaused,
		datasims.SimulationStatusFinished,
		datasims.SimulationStatusFailed,
		datasims.SimulationStatusStopped,
	}
	for _, st := range nonEditable {
		t.Run(string(st), func(t *testing.T) {
			model := readyModel("m1", "alice")
			sim := readySim("s1", "alice", model)
			sim.Status = st
			simsRepo := newFakeSimsRepo()
			simsRepo.store["s1"] = sim
			modelsRepo := newFakeModelsRepo()
			modelsRepo.put(model)

			svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
			_, err := svc.Update(ctxAs("alice", false), "s1", UpdateSimulationRequest{Name: "X", Steps: 100})
			require.ErrorIs(t, err, ErrNotEditable)
		})
	}
}

func TestService_Update_Ready_Succeeds(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Update(ctxAs("alice", false), "s1", UpdateSimulationRequest{
		Name:  "Renamed",
		Steps: 50,
		Seed:  7,
		Agents: []SimulationAgentDTO{
			{ID: 1, AgentUID: "prey", Name: "Prey #1", State: "alive"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "Renamed", resp.Name)
	require.Equal(t, uint64(50), resp.Steps)
	require.Equal(t, uint64(7), resp.Seed)

	// Snapshot/usage/runs preserved.
	persisted := simsRepo.get("s1")
	require.NotNil(t, persisted.ModelSnapshot)
	require.NotNil(t, persisted.TokenUsage)
	require.Len(t, persisted.GenerationRuns, 1)
	require.Equal(t, "alice", persisted.CreatedBy)
}

func TestService_Update_InvalidInstance_ReturnsValidationError(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	_, err := svc.Update(ctxAs("alice", false), "s1", UpdateSimulationRequest{
		Name:  "X",
		Steps: 100,
		Agents: []SimulationAgentDTO{
			{ID: 1, AgentUID: "ghost", Name: "Ghost", State: ""}, // unknown archetype
		},
	})
	require.Error(t, err)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T", err)
	require.NotEmpty(t, ve.Result.Errors)
}

func TestService_Generate_InFlight_ErrGenerationInFlight(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})

	// Pre-occupy the generation slot deterministically.
	require.True(t, svc.pending.tryStart("s1"))
	defer svc.pending.finish("s1")

	_, err := svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{Scope: GenerationScopeDTO{Kind: "agents"}})
	require.ErrorIs(t, err, ErrGenerationInFlight)
}

func TestService_Generate_ReleasesPendingOnEarlyError(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.ModelID = "missing" // ScopeFull will fail to load the model → early error
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	// note: model "missing" intentionally NOT registered

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})

	_, err := svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{Scope: GenerationScopeDTO{Kind: "full"}})
	require.ErrorIs(t, err, ErrModelNotFound)
	// The pending slot must have been released so a retry can run.
	require.False(t, svc.pending.isRunning("s1"), "pending slot leaked after early error")
}

func TestService_Create_CopiesWidgetsAndQueuesRun(t *testing.T) {
	model := readyModel("m1", "alice")
	model.Widgets = []common.WidgetConfig{
		{ID: "w1", Type: common.WidgetType("monitor"), Title: "Population"},
	}
	simsRepo := newFakeSimsRepo()
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Create(ctxAs("alice", false), CreateSimulationRequest{ModelID: "m1"})
	require.NoError(t, err)
	require.Equal(t, string(datasims.SimulationStatusGenerating), resp.Status)

	persisted := simsRepo.get(resp.ID)
	require.Len(t, persisted.Widgets, 1)
	require.Equal(t, "w1", persisted.Widgets[0].ID)
	require.Len(t, persisted.GenerationRuns, 1)
	require.Equal(t, uint64(1), persisted.GenerationRuns[0].Seq)
	require.Equal(t, datasims.ScopeFull, persisted.GenerationRuns[0].Scope.Kind)
	require.NotZero(t, persisted.Seed) // random seed assigned

	// let the async generation finish so the goroutine doesn't outlive the test
	waitNotRunning(t, svc, resp.ID)
}

func TestService_Create_ModelNotReady_ErrModelNotReady(t *testing.T) {
	model := datamodels.Model{ID: "m1", Name: "Empty", CreatedBy: "alice"} // no agents
	simsRepo := newFakeSimsRepo()
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	_, err := svc.Create(ctxAs("alice", false), CreateSimulationRequest{ModelID: "m1"})
	require.ErrorIs(t, err, ErrModelNotReady)
}

func TestService_Create_ForeignModelNonAdmin_Forbidden(t *testing.T) {
	model := readyModel("m1", "alice")
	simsRepo := newFakeSimsRepo()
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	_, err := svc.Create(ctxAs("bob", false), CreateSimulationRequest{ModelID: "m1"})
	require.ErrorIs(t, err, auth.ErrForbidden)
}

func TestService_Duplicate_ResetsState(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	// give the source some history + usage to prove it's reset on the copy
	sim.TokenUsage = &datasims.SimulationTokenUsage{Total: common.TokenUsage{TotalTokens: 999, Calls: 3}}
	sim.StatusHistory = []datasims.SimulationStatusChange{
		{Status: datasims.SimulationStatusGenerating, At: time.Now(), By: "alice"},
		{Status: datasims.SimulationStatusReady, At: time.Now(), By: "alice"},
	}
	sim.CurrentStep = 42
	// give the source agent a Properties map so we can prove the duplicate's map
	// is a fresh copy and never aliases the source's.
	sim.Agents[0].Properties = map[string]interface{}{"x": 1}
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Duplicate(ctxAs("bob", true), "s1", DuplicateRequest{}) // admin duplicates anyone's
	require.NoError(t, err)
	require.NotEqual(t, "s1", resp.ID)
	require.Equal(t, "Sim (copy)", resp.Name)
	require.Equal(t, string(datasims.SimulationStatusReady), resp.Status)

	dup := simsRepo.get(resp.ID)
	require.Equal(t, datasims.SimulationStatusReady, dup.Status)
	require.Nil(t, dup.GenerationRuns)
	require.NotNil(t, dup.TokenUsage)
	require.Zero(t, dup.TokenUsage.Total.TotalTokens) // usage zeroed
	require.Len(t, dup.StatusHistory, 1)              // single fresh "ready" entry
	require.Equal(t, datasims.SimulationStatusReady, dup.StatusHistory[0].Status)
	require.Zero(t, dup.CurrentStep)
	require.Equal(t, "bob", dup.CreatedBy)
	require.Equal(t, "bob", dup.UpdatedBy)

	// Deep-copy isolation: mutating the duplicate's agent Properties must not
	// touch the source simulation's agent Properties.
	require.NotEmpty(t, dup.Agents)
	dup.Agents[0].Properties["x"] = 999
	src := simsRepo.get("s1")
	require.Equal(t, 1, src.Agents[0].Properties["x"], "source agent Properties must be unchanged after mutating the duplicate")
}

// TestService_Duplicate_UsesInitialNotFinal verifies that duplicating a
// finished simulation copies the INITIAL world (Simulation.Agents, kept pristine
// by the run engine) and never the final snapshot (FinalAgents) or run
// statistics. The copy must be a clean, ready, re-runnable simulation.
func TestService_Duplicate_UsesInitialNotFinal(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)

	// A finished simulation: Agents holds the initial world (the engine keeps it
	// pristine), FinalAgents holds a divergent final world, Statistics is set.
	sim.Status = datasims.SimulationStatusFinished
	sim.StatusText = "finished"
	sim.CurrentStep = 5
	sim.Agents[0].Position = &common.Point{X: 10, Y: 10}
	sim.Agents[0].Properties = map[string]interface{}{"energy": float64(100)}
	sim.FinalAgents = []datasims.SimulationAgent{
		{
			ID: 1, AgentUID: "prey", Name: "Prey #1", State: "dead",
			Position:   &common.Point{X: 77, Y: 88},
			Properties: map[string]interface{}{"energy": float64(0)},
			Memory:     []datasims.AgentMemory{{Step: 5, Type: datasims.AgentMemoryMovement, Summary: "moved"}},
		},
	}
	sim.Statistics = &datasims.SimulationStatistics{StepsCompleted: 5}

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Duplicate(ctxAs("alice", false), "s1", DuplicateRequest{})
	require.NoError(t, err)

	dup := simsRepo.get(resp.ID)
	require.Equal(t, datasims.SimulationStatusReady, dup.Status)
	require.Zero(t, dup.CurrentStep)
	require.Len(t, dup.Agents, 1)

	// The duplicate must reflect the source's INITIAL agent state, not the final
	// snapshot.
	require.Equal(t, uint64(10), dup.Agents[0].Position.X)
	require.Equal(t, uint64(10), dup.Agents[0].Position.Y)
	require.Equal(t, float64(100), dup.Agents[0].Properties["energy"])
	require.Equal(t, "alive", dup.Agents[0].State)
	require.Empty(t, dup.Agents[0].Memory, "duplicate must not carry runtime memory")

	// Run artifacts must not be carried over.
	require.Nil(t, dup.FinalAgents, "duplicate must not carry the final snapshot")
	require.Nil(t, dup.Statistics, "duplicate must start without run statistics")
}

func TestService_List_NonAdmin_ForcesOwnerFilter(t *testing.T) {
	simsRepo := newFakeSimsRepo()
	modelsRepo := newFakeModelsRepo()
	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})

	_, err := svc.List(ctxAs("alice", false), 1, 20, nil, "", "")
	require.NoError(t, err)
	require.Equal(t, "alice", simsRepo.lastFilter.CreatedBy)
}

func TestService_List_Admin_NoOwnerFilter(t *testing.T) {
	simsRepo := newFakeSimsRepo()
	modelsRepo := newFakeModelsRepo()
	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})

	_, err := svc.List(ctxAs("admin", true), 1, 20, []string{"model-x"}, "", "")
	require.NoError(t, err)
	require.Equal(t, "", simsRepo.lastFilter.CreatedBy)
	require.Equal(t, []string{"model-x"}, simsRepo.lastFilter.ModelIDs)
}

func TestService_Generate_FromGenerationFailed_Succeeds(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusGenerationFailed
	sim.StatusText = "generation failed"
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{Scope: GenerationScopeDTO{Kind: "full"}})
	require.NoError(t, err)
	require.Equal(t, string(datasims.SimulationStatusGenerating), resp.Status)

	persisted := simsRepo.get("s1")
	require.Len(t, persisted.GenerationRuns, 2) // original failed run + new run
	require.Equal(t, uint64(2), persisted.GenerationRuns[1].Seq)
	require.Equal(t, datasims.ScopeFull, persisted.GenerationRuns[1].Scope.Kind)

	// let the async generation finish so the goroutine doesn't outlive the test
	waitNotRunning(t, svc, "s1")
}

func TestService_Generate_RepoUpdateFails_ReleasesPending(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	simsRepo.updateErr = errors.New("boom") // the Generate persist will fail
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	_, err := svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{Scope: GenerationScopeDTO{Kind: "agents"}})
	require.Error(t, err)
	require.ErrorContains(t, err, "boom")
	// The pending slot must have been released so a retry can run.
	require.False(t, svc.pending.isRunning("s1"), "pending slot leaked after persist failure")
}

// --- ISS-03: regeneration rolls a fresh seed --------------------------------

// TestService_Generate_RollsFreshSeed verifies that a regeneration draws a new
// seed (distinct from the simulation's current one), persists it to sim.Seed,
// and stamps the SAME value onto the appended GenerationRun record.
func TestService_Generate_RollsFreshSeed(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Seed = 42
	sim.GenerationRuns[0].Seed = 42 // the create-time run's own recorded provenance
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})
	resp, err := svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{Scope: GenerationScopeDTO{Kind: "full"}})
	require.NoError(t, err)

	persisted := simsRepo.get("s1")
	require.NotEqual(t, uint64(42), persisted.Seed, "regeneration must roll a fresh seed")
	require.Equal(t, persisted.Seed, resp.Seed)
	require.Len(t, persisted.GenerationRuns, 2)
	require.Equal(t, persisted.Seed, persisted.GenerationRuns[1].Seed,
		"the appended run record must carry the seed it drew")
	// The FIRST (create-time) run also carries its own seed (42), proving every
	// run's provenance is independently visible.
	require.Equal(t, uint64(42), persisted.GenerationRuns[0].Seed)

	waitNotRunning(t, svc, "s1")
}

// TestService_Generate_TwoRegensProduceDifferentSeeds pins down the "rolls a
// fresh seed" behavior end-to-end across two successive regenerations: a second
// Generate must not reuse the seed the first one drew.
func TestService_Generate_TwoRegensProduceDifferentSeeds(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)
	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})

	_, err := svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{Scope: GenerationScopeDTO{Kind: "full"}})
	require.NoError(t, err)
	waitNotRunning(t, svc, "s1")
	seed1 := simsRepo.get("s1").Seed

	_, err = svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{Scope: GenerationScopeDTO{Kind: "full"}})
	require.NoError(t, err)
	waitNotRunning(t, svc, "s1")
	seed2 := simsRepo.get("s1").Seed

	require.NotEqual(t, seed1, seed2, "each regeneration must roll a fresh seed")
}

// TestService_Generate_ExplicitSeed_UsesGivenValue verifies the optional
// request-level seed override: when supplied, Generate uses it verbatim instead
// of rolling a random one.
func TestService_Generate_ExplicitSeed_UsesGivenValue(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)
	svc, _ := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})

	explicit := uint64(777)
	_, err := svc.Generate(ctxAs("alice", false), "s1", GenerateRequest{
		Scope: GenerationScopeDTO{Kind: "full"}, Seed: &explicit,
	})
	require.NoError(t, err)

	persisted := simsRepo.get("s1")
	require.Equal(t, explicit, persisted.Seed)
	require.Equal(t, explicit, persisted.GenerationRuns[len(persisted.GenerationRuns)-1].Seed)

	waitNotRunning(t, svc, "s1")
}

func TestService_Delete_PublishesDeletedEvent(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim
	modelsRepo := newFakeModelsRepo()
	modelsRepo.put(model)

	svc, hub := newTestService(t, simsRepo, modelsRepo, &fakeSvcLLM{})

	sub, cancel := hub.Subscribe("s1")
	defer cancel()

	require.NoError(t, svc.Delete(ctxAs("alice", false), "s1"))

	select {
	case ev, ok := <-sub:
		require.True(t, ok, "subscriber channel closed before delivering the deleted event")
		require.Equal(t, EventSimulationDeleted, ev.Type)
		require.Equal(t, "s1", ev.SimulationID)
	case <-time.After(time.Second):
		t.Fatal("did not receive simulation.deleted event")
	}

	// The simulation must no longer be in the repo.
	_, getErr := simsRepo.GetByID(context.Background(), "s1")
	require.ErrorIs(t, getErr, common.ErrNotFound)
}

// waitNotRunning blocks until the background generation goroutine for id releases
// the pending slot, so test goroutines don't outlive the test.
func waitNotRunning(t *testing.T, svc *service, id string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		if !svc.pending.isRunning(id) {
			return
		}
		select {
		case <-deadline:
			t.Fatal("generation did not finish in time")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestRandomSeed_FitsBSONInt64 guards the regression where a random uint64 seed
// above math.MaxInt64 could not be marshaled to BSON (MongoDB int64), failing
// ~half of all simulation creates. randomSeed masks to 63 bits, so every value
// must fit a signed int64.
func TestRandomSeed_FitsBSONInt64(t *testing.T) {
	const maxInt64 = uint64(1<<63 - 1)
	for i := 0; i < 10000; i++ {
		if s := randomSeed(); s > maxInt64 {
			t.Fatalf("randomSeed() = %d exceeds math.MaxInt64 (%d)", s, maxInt64)
		}
	}
}

// TestRandomSeed_JSSafeRange guards the live-verification finding that an
// auto-generated uint64 seed (e.g. 8063019124532861000) loses precision
// through ANY JavaScript client once it exceeds Number.MAX_SAFE_INTEGER
// (2^53-1 == 9007199254740991): the frontend can neither display nor
// round-trip such a seed faithfully, defeating "the stored seed describes the
// world". randomSeed() must draw uniformly from [1, 2^53).
func TestRandomSeed_JSSafeRange(t *testing.T) {
	const jsMaxSafeInt = uint64(1<<53 - 1) // Number.MAX_SAFE_INTEGER
	for i := 0; i < 10000; i++ {
		s := randomSeed()
		if s == 0 {
			t.Fatalf("randomSeed() = 0, want > 0")
		}
		if s > jsMaxSafeInt {
			t.Fatalf("randomSeed() = %d exceeds Number.MAX_SAFE_INTEGER (%d)", s, jsMaxSafeInt)
		}
	}
}
