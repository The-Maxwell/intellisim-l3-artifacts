package simulations

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/llm"
	"intelli.sim.core/modules/models"
)

const defaultPageSize = 20
const defaultSteps uint64 = 100

// ValidationError wraps a ValidateInstance result that blocks a manual update.
// The API layer maps it to 422 with the per-field issues.
type ValidationError struct {
	Result ValidationResult
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("simulation validation failed (%d errors)", len(e.Result.Errors))
}

// Service is the simulations use-case layer. Every method requires an
// authenticated principal in ctx (auth.WithPrincipal); ownership is enforced
// here: owner or admin, others see auth.ErrForbidden.
//
// It covers create-from-model (async generation → 202), full/scoped
// regeneration, manual edit (validated), duplicate, list, get, delete, and the
// per-simulation SSE event stream.
type Service interface {
	Create(ctx context.Context, req CreateSimulationRequest) (*SimulationResponse, error)
	Get(ctx context.Context, id string) (*SimulationResponse, error)
	// GetModelSnapshot returns the full frozen model definition the simulation
	// was generated from, in the same shape as GET /models/:id. Unlike the
	// trimmed inline model_snapshot on the simulation response, it carries the
	// complete model (rules, interactions, events, llm config, widgets, images).
	GetModelSnapshot(ctx context.Context, id string) (*models.ModelResponse, error)
	Update(ctx context.Context, id string, req UpdateSimulationRequest) (*SimulationResponse, error)
	Generate(ctx context.Context, id string, req GenerateRequest) (*SimulationResponse, error)
	Duplicate(ctx context.Context, id string, req DuplicateRequest) (*SimulationResponse, error)
	List(ctx context.Context, page, pageSize int, modelIDs []string, sortBy, sortOrder string) (*ListSimulationsResponse, error)
	Delete(ctx context.Context, id string) error
}

type service struct {
	repo     datasims.SimulationsRepository
	models   datamodels.ModelsRepository
	llm      llm.Service
	gen      *Generator
	analyses datasims.AnalysesRepository
	files    datasims.AnalysisFileStore
	steps    datasims.SimulationStepsRepository
	hub      *EventsHub
	pending  *pendingTracker
	cfg      ModuleConfig
	logger   *zap.Logger
}

// NewService builds the simulations service. The pending tracker is constructed
// internally (in-memory generation guard), mirroring the models service.
func NewService(
	repo datasims.SimulationsRepository,
	models datamodels.ModelsRepository,
	llmService llm.Service,
	gen *Generator,
	analyses datasims.AnalysesRepository,
	files datasims.AnalysisFileStore,
	steps datasims.SimulationStepsRepository,
	hub *EventsHub,
	cfg ModuleConfig,
	logger *zap.Logger,
) Service {
	return &service{
		repo:     repo,
		models:   models,
		llm:      llmService,
		gen:      gen,
		analyses: analyses,
		files:    files,
		steps:    steps,
		hub:      hub,
		pending:  newPendingTracker(),
		cfg:      cfg,
		logger:   logger.Named("simulation-service"),
	}
}

// ---- helpers -------------------------------------------------------------------

// getOwnedSimulation loads a simulation and enforces owner-or-admin access.
// Missing and malformed-id simulations yield ErrSimulationNotFound (404); a
// simulation owned by another non-admin user yields auth.ErrForbidden (403).
func (s *service) getOwnedSimulation(ctx context.Context, id string) (*datasims.Simulation, *auth.UserPrincipal, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, nil, err
	}
	sim, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) || errors.Is(err, common.ErrInvalidID) {
			return nil, nil, ErrSimulationNotFound
		}
		return nil, nil, err
	}
	if err := p.Authorize(sim.CreatedBy); err != nil {
		return nil, nil, err
	}
	return sim, p, nil
}

// buildResponse attaches the instance validation to a simulation.
func (s *service) buildResponse(sim datasims.Simulation) *SimulationResponse {
	res := ValidateInstance(sim)
	resp := simulationToResponse(sim, res)
	return &resp
}

// isModelReady reports whether a model is complete enough to seed a simulation.
// This is a local minimal readiness check (intentional per-module duplication,
// matching this codebase's DTO convention) so the simulations module does not
// import modules/models. It mirrors the simulation-relevant subset of
// models.ValidateCompleteness: a name, at least one agent, and — for spatial
// models — a sized environment.
func isModelReady(m datamodels.Model) bool {
	if strings.TrimSpace(m.Name) == "" {
		return false
	}
	if len(m.Agents) == 0 {
		return false
	}
	if m.IsSpatial {
		if m.Environment == nil || m.Environment.Width == 0 || m.Environment.Height == 0 {
			return false
		}
	}
	return true
}

// validateOverride validates a user-supplied LLM target override against the given
// capability (simulation_generation for the generation override, simulation_execution
// for the execution override). An empty provider means "no override" and is accepted.
// Any validation failure maps to ErrInvalidLLMTarget (422).
func (s *service) validateOverride(t *common.LLMTarget, capability common.ModelCapability) error {
	if t == nil || t.ProviderUID == "" {
		return nil
	}
	if err := s.llm.ValidateTarget(*t, capability); err != nil {
		s.logger.Debug("invalid llm target override", zap.Error(err), zap.String("capability", string(capability)))
		return ErrInvalidLLMTarget
	}
	return nil
}

// jsSafeSeedBound is the exclusive upper bound for an auto-generated seed:
// 2^53, one above Number.MAX_SAFE_INTEGER (2^53-1 == 9007199254740991). A
// random uint64 seed above this loses precision through ANY JavaScript client
// (JS numbers are IEEE-754 doubles with 53 bits of integer precision) — the
// frontend can neither display nor round-trip it faithfully, defeating "the
// stored seed describes the world". This bound applies ONLY to
// auto-generated seeds (Create/Generate); an explicit user-provided seed is
// exempt and keeps the full uint64 API contract.
const jsSafeSeedBound = uint64(1) << 53

// randomSeed returns a cryptographically-seeded value in [1, jsSafeSeedBound),
// falling back to 1 on a (vanishingly unlikely) RNG read error so generation
// stays deterministic. Staying within jsSafeSeedBound (< math.MaxInt64) also
// keeps every seed fitting a BSON int64 — MongoDB cannot marshal a uint64
// above math.MaxInt64.
func randomSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 1
	}
	// Reduce into [0, jsSafeSeedBound-1) then shift by 1 so the result is
	// always in [1, jsSafeSeedBound) — never 0, never at/above the bound.
	return binary.LittleEndian.Uint64(b[:])%(jsSafeSeedBound-1) + 1
}

// loadReadyModel loads model id, enforces owner-or-admin access, and checks
// simulation readiness. Not-found maps to ErrModelNotFound; not-ready to
// ErrModelNotReady.
func (s *service) loadReadyModel(ctx context.Context, p *auth.UserPrincipal, id string) (*datamodels.Model, error) {
	model, err := s.models.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) || errors.Is(err, common.ErrInvalidID) {
			return nil, ErrModelNotFound
		}
		return nil, err
	}
	if err := p.Authorize(model.CreatedBy); err != nil {
		return nil, err
	}
	if !isModelReady(*model) {
		return nil, ErrModelNotReady
	}
	return model, nil
}

// ---- Create --------------------------------------------------------------------

func (s *service) Create(ctx context.Context, req CreateSimulationRequest) (*SimulationResponse, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}

	model, err := s.loadReadyModel(ctx, p, req.ModelID)
	if err != nil {
		return nil, err
	}

	genOverride := llmTargetFromDTO(req.LLMGenerationTarget)
	if err := s.validateOverride(genOverride, common.ModelCapabilitySimulationGeneration); err != nil {
		return nil, err
	}
	execOverride := llmTargetFromDTO(req.LLMExecutionTarget)
	if err := s.validateOverride(execOverride, common.ModelCapabilitySimulationExecution); err != nil {
		return nil, err
	}

	seed := randomSeed()
	if req.Seed != nil {
		seed = *req.Seed
	}
	steps := defaultSteps
	if req.Steps != nil {
		steps = *req.Steps
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(model.Name)
	}
	if name == "" {
		name = "Simulation"
	}

	now := time.Now().UTC()
	sim := datasims.Simulation{
		Name:                        name,
		Description:                 strings.TrimSpace(req.Description),
		ModelID:                     model.ID,
		ModelSnapshot:               model,
		Seed:                        seed,
		Steps:                       steps,
		Termination:                 cloneTerminationCondition(model.SimulationConfig.Termination),
		LLMGenerationTargetOverride: genOverride,
		LLMExecutionTargetOverride:  execOverride,
		Widgets:                     cloneWidgets(model.Widgets),
		Status:                      datasims.SimulationStatusGenerating,
		StatusText:                  "generating",
		StatusHistory: []datasims.SimulationStatusChange{
			{Status: datasims.SimulationStatusGenerating, At: now, By: p.ID},
		},
		GenerationRuns: []datasims.GenerationRun{
			{Seq: 1, Seed: seed, Scope: datasims.GenerationScope{Kind: datasims.ScopeFull}, Status: datasims.RunRunning, StartedAt: now},
		},
		TokenUsage: &datasims.SimulationTokenUsage{},
		CreatedAt:  now,
		UpdatedAt:  now,
		CreatedBy:  p.ID,
		UpdatedBy:  p.ID,
	}

	id, err := s.repo.Create(ctx, sim)
	if err != nil {
		s.logger.Error("failed to create simulation", zap.Error(err))
		return nil, err
	}
	sim.ID = id
	s.logger.Info("simulation created", zap.String("simulation_id", id), zap.String("owner", p.ID))

	// Reserve the generation slot and detach the seeding pipeline from the request.
	s.pending.tryStart(id)
	go s.runGeneration(context.WithoutCancel(ctx), id, datasims.GenerationScope{Kind: datasims.ScopeFull})

	return s.buildResponse(sim), nil
}

// ---- runGeneration -------------------------------------------------------------

// runGeneration executes one background generation pass for the simulation,
// finalises the last GenerationRun, persists the result, and publishes the
// outcome events. It loads a FRESH copy of the simulation so it never shares a
// pointer with the request goroutine.
func (s *service) runGeneration(ctx context.Context, id string, scope datasims.GenerationScope) {
	defer s.pending.finish(id)

	sim, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("generation: failed to load simulation", zap.String("simulation_id", id), zap.Error(err))
		return
	}

	if len(sim.GenerationRuns) == 0 {
		s.logger.Error("generation: simulation has no generation runs", zap.String("simulation_id", id))
		return
	}
	runSeq := sim.GenerationRuns[len(sim.GenerationRuns)-1].Seq
	progress := func(stage, detail string) {
		s.hub.Publish(id, Event{
			Type:         EventGenerationProgress,
			SimulationID: id,
			Payload:      GenerationProgressPayload{Run: runSeq, Stage: stage, Detail: detail},
		})
	}

	warnings, requests, gerr := s.gen.Run(ctx, sim, scope, progress)

	now := time.Now().UTC()
	if n := len(sim.GenerationRuns); n > 0 {
		run := &sim.GenerationRuns[n-1]
		run.FinishedAt = &now
		run.Requests = requests
		run.Warnings = warnings
		if gerr != nil {
			run.Status = datasims.RunFailed
			run.Error = gerr.Error()
		} else {
			run.Status = datasims.RunCompleted
		}
	}
	if gerr != nil {
		sim.Status = datasims.SimulationStatusGenerationFailed
		sim.StatusText = "generation failed"
	} else {
		sim.Status = datasims.SimulationStatusReady
		sim.StatusText = "ready"
	}
	sim.StatusHistory = append(sim.StatusHistory, datasims.SimulationStatusChange{Status: sim.Status, At: now, By: "system"})
	sim.UpdatedAt = now

	if err := s.repo.Update(ctx, id, *sim); err != nil {
		s.logger.Error("generation: failed to persist result", zap.String("simulation_id", id), zap.Error(err))
		return
	}

	s.hub.Publish(id, Event{Type: EventSimulationUpdated, SimulationID: id, Payload: s.buildResponse(*sim)})
	if gerr != nil {
		s.logger.Warn("simulation generation failed", zap.String("simulation_id", id), zap.Error(gerr))
		s.hub.Publish(id, Event{Type: EventGenerationFailed, SimulationID: id,
			Payload: map[string]any{"run": runSeq, "error": gerr.Error()}})
		return
	}
	s.hub.Publish(id, Event{Type: EventGenerationCompleted, SimulationID: id,
		Payload: map[string]any{"run": runSeq, "warnings": len(warnings), "requests": requests}})
}

// ---- Get / Update / Generate ---------------------------------------------------

func (s *service) Get(ctx context.Context, id string) (*SimulationResponse, error) {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.buildResponse(*sim), nil
}

// GetModelSnapshot returns the frozen full source-model copy embedded in the
// simulation, mapped to the models module's response shape so the frontend can
// reuse its model types/components verbatim. Read-only: validation is left empty
// (a frozen snapshot has no live validation) and the mapper is pure.
func (s *service) GetModelSnapshot(ctx context.Context, id string) (*models.ModelResponse, error) {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}
	if sim.ModelSnapshot == nil {
		return nil, ErrModelSnapshotNotFound
	}
	resp := models.NewModelMapper().ToModelResponse(*sim.ModelSnapshot, models.ValidationResult{})
	return &resp, nil
}

func (s *service) Update(ctx context.Context, id string, req UpdateSimulationRequest) (*SimulationResponse, error) {
	sim, p, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sim.Status.Editable() {
		return nil, ErrNotEditable
	}

	genOverride := llmTargetFromDTO(req.LLMGenerationTarget)
	if err := s.validateOverride(genOverride, common.ModelCapabilitySimulationGeneration); err != nil {
		return nil, err
	}
	execOverride := llmTargetFromDTO(req.LLMExecutionTarget)
	if err := s.validateOverride(execOverride, common.ModelCapabilitySimulationExecution); err != nil {
		return nil, err
	}

	// Apply editable fields; preserve snapshot/usage/runs/status/audit-create.
	sim.Name = strings.TrimSpace(req.Name)
	sim.Description = strings.TrimSpace(req.Description)
	sim.Steps = req.Steps
	sim.Seed = req.Seed
	sim.Termination = terminationFromDTO(req.Termination)
	sim.LLMGenerationTargetOverride = genOverride
	sim.LLMExecutionTargetOverride = execOverride
	sim.Agents = agentsFromDTO(req.Agents)
	sim.Environment = environmentFromDTO(req.Environment)
	sim.Widgets = widgetsFromDTO(req.Widgets)
	sim.UpdatedAt = time.Now().UTC()
	sim.UpdatedBy = p.ID

	if res := ValidateInstance(*sim); len(res.Errors) > 0 {
		return nil, &ValidationError{Result: res}
	}

	if err := s.repo.Update(ctx, id, *sim); err != nil {
		s.logger.Error("failed to update simulation", zap.String("simulation_id", id), zap.Error(err))
		return nil, err
	}

	resp := s.buildResponse(*sim)
	s.hub.Publish(id, Event{Type: EventSimulationUpdated, SimulationID: id, Payload: resp})
	return resp, nil
}

func (s *service) Generate(ctx context.Context, id string, req GenerateRequest) (*SimulationResponse, error) {
	sim, p, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sim.Status.Editable() {
		return nil, ErrNotEditable
	}
	if !s.pending.tryStart(id) {
		return nil, ErrGenerationInFlight
	}

	// From here on every error path BEFORE the goroutine spawns must release
	// the pending slot, or it would leak and block all future generations.
	scope := scopeFromDTO(req.Scope)

	if req.LLMGenerationTarget != nil {
		override := llmTargetFromDTO(req.LLMGenerationTarget)
		if err := s.validateOverride(override, common.ModelCapabilitySimulationGeneration); err != nil {
			s.pending.finish(id)
			return nil, err
		}
		sim.LLMGenerationTargetOverride = override
	}

	// A full regeneration refreshes the model snapshot from the current model.
	if scope.Kind == datasims.ScopeFull {
		model, err := s.loadReadyModel(ctx, p, sim.ModelID)
		if err != nil {
			s.pending.finish(id)
			return nil, err
		}
		sim.ModelSnapshot = model
	}

	// Every regeneration draws a fresh seed (or uses the caller-supplied one) and
	// stamps it onto both the simulation and the run record, so the stored seed
	// always describes the CURRENT world — a full-scope regen that rolls seed X
	// reproduces exactly what CREATING a new simulation with seed X would produce
	// (see generator.go: the generation RNG derives from sim.Seed alone).
	seed := randomSeed()
	if req.Seed != nil {
		seed = *req.Seed
	}
	sim.Seed = seed

	now := time.Now().UTC()
	sim.GenerationRuns = append(sim.GenerationRuns, datasims.GenerationRun{
		Seq:       uint64(len(sim.GenerationRuns) + 1),
		Seed:      seed,
		Scope:     scope,
		Status:    datasims.RunRunning,
		StartedAt: now,
	})
	sim.Status = datasims.SimulationStatusGenerating
	sim.StatusText = "generating"
	sim.StatusHistory = append(sim.StatusHistory, datasims.SimulationStatusChange{Status: sim.Status, At: now, By: p.ID})
	sim.UpdatedAt = now

	if err := s.repo.Update(ctx, id, *sim); err != nil {
		s.pending.finish(id)
		s.logger.Error("failed to queue generation", zap.String("simulation_id", id), zap.Error(err))
		return nil, err
	}

	go s.runGeneration(context.WithoutCancel(ctx), id, scope)

	return s.buildResponse(*sim), nil
}

// ---- Duplicate -----------------------------------------------------------------

func (s *service) Duplicate(ctx context.Context, id string, req DuplicateRequest) (*SimulationResponse, error) {
	src, p, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = src.Name + " (copy)"
	}

	now := time.Now().UTC()
	dup := datasims.Simulation{
		Name:                        name,
		Description:                 src.Description,
		ModelID:                     src.ModelID,
		ModelSnapshot:               src.ModelSnapshot, // immutable frozen snapshot; safe to share
		Seed:                        src.Seed,
		Steps:                       src.Steps,
		Termination:                 cloneTerminationCondition(src.Termination),
		LLMGenerationTargetOverride: cloneTarget(src.LLMGenerationTargetOverride),
		LLMExecutionTargetOverride:  cloneTarget(src.LLMExecutionTargetOverride),
		Agents:                      cloneAgents(src.Agents),
		Environment:                 cloneEnvironment(src.Environment),
		Widgets:                     cloneWidgets(src.Widgets),
		Status:                      datasims.SimulationStatusReady,
		StatusText:                  "ready",
		StatusHistory: []datasims.SimulationStatusChange{
			{Status: datasims.SimulationStatusReady, At: now, By: p.ID},
		},
		GenerationRuns: nil,
		TokenUsage:     &datasims.SimulationTokenUsage{},
		CurrentStep:    0,
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      p.ID,
		UpdatedBy:      p.ID,
	}

	newID, err := s.repo.Create(ctx, dup)
	if err != nil {
		s.logger.Error("failed to duplicate simulation", zap.String("simulation_id", id), zap.Error(err))
		return nil, err
	}
	dup.ID = newID
	s.logger.Info("simulation duplicated", zap.String("source_id", id), zap.String("simulation_id", newID))
	return s.buildResponse(dup), nil
}

// ---- List / Delete -------------------------------------------------------------

func (s *service) List(ctx context.Context, page, pageSize int, modelIDs []string, sortBy, sortOrder string) (*ListSimulationsResponse, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	filter := datasims.ListFilter{
		ModelIDs:  modelIDs,
		Page:      page,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}
	if !p.IsAdmin {
		filter.CreatedBy = p.ID
	}
	summaries, hasMore, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	items := make([]SimulationSummaryDTO, 0, len(summaries))
	for _, sum := range summaries {
		items = append(items, summaryToDTO(sum))
	}
	return &ListSimulationsResponse{Items: items, Total: int(total), HasMore: hasMore}, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return err
	}
	// Cascade: analysis PDF files (GridFS) → analysis records → recorded steps.
	// Best-effort: log + continue so a partial-cleanup failure never blocks deletion.
	if err := s.files.DeleteBySimulation(ctx, sim.ID); err != nil {
		s.logger.Warn("failed to delete analysis files", zap.String("simulation_id", id), zap.Error(err))
	}
	if err := s.analyses.DeleteBySimulationID(ctx, sim.ID); err != nil {
		s.logger.Warn("failed to delete analyses", zap.String("simulation_id", id), zap.Error(err))
	}
	if err := s.steps.DeleteBySimulation(ctx, sim.ID); err != nil {
		s.logger.Warn("failed to delete simulation steps", zap.String("simulation_id", id), zap.Error(err))
	}
	if err := s.repo.Delete(ctx, sim.ID); err != nil {
		s.logger.Error("failed to delete simulation", zap.String("simulation_id", id), zap.Error(err))
		return err
	}
	s.hub.Publish(id, Event{Type: EventSimulationDeleted, SimulationID: id, Payload: map[string]string{"id": id}})
	s.hub.CloseSimulation(id)
	return nil
}

// ---- clone helpers -------------------------------------------------------------

// cloneWidgets returns a shallow copy of the widget slice. Widget descriptors
// are value types whose nested slices/maps are not mutated in place by the
// edit/duplicate paths (they are wholesale-replaced), so a re-slice is enough to
// avoid backing-array aliasing between simulations.
func cloneWidgets(in []common.WidgetConfig) []common.WidgetConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]common.WidgetConfig, len(in))
	copy(out, in)
	return out
}

func cloneTarget(t *common.LLMTarget) *common.LLMTarget {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func cloneAgents(in []datasims.SimulationAgent) []datasims.SimulationAgent {
	if in == nil {
		return nil
	}
	out := make([]datasims.SimulationAgent, len(in))
	copy(out, in)
	// deep-copy Properties/Memory so a duplicate never aliases the source's maps (engine may mutate live agent properties in place).
	for i := range out {
		out[i].Properties = cloneProps(in[i].Properties)
		if in[i].Memory != nil {
			out[i].Memory = make([]datasims.AgentMemory, len(in[i].Memory))
			copy(out[i].Memory, in[i].Memory)
		}
	}
	return out
}

// cloneCollisionConfig deep-copies the optional U5 collision tuning so clones
// never alias the source's config.
func cloneCollisionConfig(in *datamodels.CollisionConfig) *datamodels.CollisionConfig {
	if in == nil {
		return nil
	}
	c := *in
	return &c
}

func cloneEnvironment(in *datasims.SimulationEnvironment) *datasims.SimulationEnvironment {
	if in == nil {
		return nil
	}
	c := *in
	c.CollisionConfig = cloneCollisionConfig(in.CollisionConfig)
	c.GlobalProperties = cloneProps(in.GlobalProperties)
	if in.Shapes != nil {
		c.Shapes = make([]datasims.SimulationShape, len(in.Shapes))
		copy(c.Shapes, in.Shapes)
		for i := range c.Shapes {
			c.Shapes[i].Properties = cloneProps(in.Shapes[i].Properties)
			if in.Shapes[i].Points != nil {
				c.Shapes[i].Points = make([]common.Point, len(in.Shapes[i].Points))
				copy(c.Shapes[i].Points, in.Shapes[i].Points)
			}
		}
	}
	return &c
}

// cloneProps returns a shallow copy of a property map, preserving a nil input as
// nil. Values are copied by reference (the simulation property maps hold scalars
// and already-normalised plain values), but the map itself is fresh so mutations
// on a clone never leak back to the source.
func cloneProps(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
