package simulations

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/llm"
)

// ResultsSeriesQuery parameterizes the results/series endpoint.
type ResultsSeriesQuery struct {
	PropertyUID string
	AgentUIDs   []string // definition UIDs: agent archetypes, or shape templates when EntityType == "shape"
	From        *int64
	To          *int64
	Aggregation string
	EntityType  string // "" | "agent" | "shape" ("" = agent)
}

// AnalysisService is the Level-3 results-analysis use-case layer. Ownership is
// enforced via the parent simulation (owner or admin), mirroring the create/edit
// service. Analyses run asynchronously (202 + SSE progress).
type AnalysisService interface {
	Create(ctx context.Context, simID string, req AnalyzeRequest) (*AnalysisResponse, error)
	List(ctx context.Context, simID string) (*ListAnalysesResponse, error)
	Get(ctx context.Context, simID, analysisID string) (*AnalysisResponse, error)
	Delete(ctx context.Context, simID, analysisID string) error
	OpenFile(ctx context.Context, simID, analysisID string) (io.ReadCloser, *datasims.AnalysisFileInfo, string, error)
	ResultsSummary(ctx context.Context, simID string) (*ResultsSummaryDTO, error)
	ResultsSeries(ctx context.Context, simID string, q ResultsSeriesQuery) (*ResultsSeriesDTO, error)
	// Reconcile marks any analysis left in a non-terminal status (pending/running)
	// as failed — used for crash-recovery on startup.
	Reconcile(ctx context.Context) error
}

type analysisService struct {
	repo     datasims.SimulationsRepository
	analyses datasims.AnalysesRepository
	files    datasims.AnalysisFileStore
	steps    datasims.SimulationStepsRepository
	llm      llm.Service
	hub      *EventsHub
	cfg      AnalysisConfig
	logger   *zap.Logger
}

// NewAnalysisService builds the analysis service (FX provider).
func NewAnalysisService(
	repo datasims.SimulationsRepository,
	analyses datasims.AnalysesRepository,
	files datasims.AnalysisFileStore,
	steps datasims.SimulationStepsRepository,
	llmService llm.Service,
	hub *EventsHub,
	cfg AnalysisConfig,
	logger *zap.Logger,
) AnalysisService {
	return &analysisService{
		repo: repo, analyses: analyses, files: files, steps: steps,
		llm: llmService, hub: hub, cfg: cfg, logger: logger.Named("analysis-service"),
	}
}

// requireOwnedSimulation loads a simulation and enforces owner-or-admin access.
// Package-level twin of service.getOwnedSimulation (analysisService is a separate struct).
func requireOwnedSimulation(ctx context.Context, repo datasims.SimulationsRepository, id string) (*datasims.Simulation, *auth.UserPrincipal, error) {
	p, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return nil, nil, err
	}
	sim, err := repo.GetByID(ctx, id)
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

// requireOwnedAnalysis loads an analysis under an owned simulation.
func (s *analysisService) requireOwnedAnalysis(ctx context.Context, simID, analysisID string) (*datasims.Simulation, *datasims.Analysis, error) {
	sim, _, err := requireOwnedSimulation(ctx, s.repo, simID)
	if err != nil {
		return nil, nil, err
	}
	a, err := s.analyses.GetByID(ctx, analysisID)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) || errors.Is(err, common.ErrInvalidID) {
			return nil, nil, ErrAnalysisNotFound
		}
		return nil, nil, err
	}
	if a.SimulationID != sim.ID {
		return nil, nil, ErrAnalysisNotFound
	}
	return sim, a, nil
}

func (s *analysisService) Create(ctx context.Context, simID string, req AnalyzeRequest) (*AnalysisResponse, error) {
	sim, p, err := requireOwnedSimulation(ctx, s.repo, simID)
	if err != nil {
		return nil, err
	}
	if !analyzableStatus(sim.Status) {
		return nil, ErrNotAnalyzable
	}
	min, max, err := s.steps.GetStepRange(ctx, sim.ID)
	if errors.Is(err, common.ErrNotFound) || (err == nil && max < min) {
		return nil, ErrNoResults
	}
	if err != nil {
		return nil, err
	}

	target := targetFromAnalyzeRequest(req)
	if err := s.llm.ValidateTarget(target, common.ModelCapabilityResultsAnalysis); err != nil {
		s.logger.Debug("invalid analysis llm target", zap.Error(err))
		return nil, ErrInvalidLLMTarget
	}

	sel := selectionFromDTO(req.Selection)
	recorded, perr := s.steps.FindDistinctProperties(ctx, sim.ID)
	if perr != nil {
		return nil, perr
	}
	if verr := validateAnalysisSelection(sel, sim.ModelSnapshot, recorded, min, max); verr != nil {
		return nil, verr
	}

	// Freeze the analyzed window: resolve effective from/to (clamped to the
	// recorded range) and store them back so a later-resumed paused run cannot
	// extend the window and StepsCovered stays deterministic.
	from, to := min, max
	if sel.StepFrom != nil && int64(*sel.StepFrom) > from {
		from = int64(*sel.StepFrom)
	}
	if sel.StepTo != nil && int64(*sel.StepTo) < to {
		to = int64(*sel.StepTo)
	}
	if from < min {
		from = min
	}
	if to > max {
		to = max
	}
	if to < from {
		to = from
	}
	fromU, toU := uint64(from), uint64(to)
	sel.StepFrom = &fromU
	sel.StepTo = &toU

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = sim.Name + " — Analysis"
	}
	now := time.Now().UTC()
	a := datasims.Analysis{
		SimulationID:               sim.ID,
		Name:                       name,
		ContextMessage:             strings.TrimSpace(req.ContextMessage),
		Language:                   datasims.AnalysisLanguage(req.Language),
		LLMTarget:                  target,
		Selection:                  sel,
		Status:                     datasims.AnalysisStatusPending,
		StatusText:                 "queued",
		Progress:                   datasims.AnalysisProgress{Stage: "queued", Percent: 0},
		SimulationStatusAtAnalysis: sim.Status,
		TokenUsage:                 &common.TokenUsageSummary{},
		CreatedAt:                  now,
		UpdatedAt:                  now,
		CreatedBy:                  p.ID,
	}
	id, err := s.analyses.Create(ctx, a)
	if err != nil {
		s.logger.Error("failed to create analysis", zap.Error(err))
		return nil, err
	}
	a.ID = id
	s.logger.Info("analysis created", zap.String("analysis_id", id), zap.String("simulation_id", sim.ID))

	go s.runAnalysis(context.WithoutCancel(ctx), sim.ID, id)

	resp := analysisToResponse(a)
	return &resp, nil
}

// Reconcile marks any analysis still persisted as pending/running (whose
// driving goroutine died with the previous process) as failed, so analyses
// always reach a terminal state after a restart. Mirrors RunManager.Reconcile.
func (s *analysisService) Reconcile(ctx context.Context) error {
	unfinished, err := s.analyses.FindUnfinished(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for i := range unfinished {
		a := unfinished[i]
		a.Status = datasims.AnalysisStatusFailed
		a.StatusText = "interrupted by server restart"
		a.Error = "interrupted by server restart"
		a.Progress = datasims.AnalysisProgress{Stage: "failed", Percent: 100}
		a.FinishedAt = &now
		a.UpdatedAt = now
		if uerr := s.analyses.Update(ctx, a.ID, a); uerr != nil {
			s.logger.Warn("failed to reconcile interrupted analysis", zap.String("analysis_id", a.ID), zap.Error(uerr))
		}
	}
	return nil
}

// runAnalysis executes the pipeline on a detached context and persists outcomes.
func (s *analysisService) runAnalysis(ctx context.Context, simID, analysisID string) {
	a, err := s.analyses.GetByID(ctx, analysisID)
	if err != nil {
		s.logger.Error("runAnalysis: load analysis failed", zap.String("analysis_id", analysisID), zap.Error(err))
		return
	}
	sim, err := s.repo.GetByID(ctx, simID)
	if err != nil {
		s.fail(ctx, a, "simulation no longer available")
		return
	}

	now := time.Now().UTC()
	a.Status = datasims.AnalysisStatusRunning
	a.StatusText = "running"
	a.StartedAt = &now
	a.Progress = datasims.AnalysisProgress{Stage: "starting", Percent: 5}
	a.UpdatedAt = now
	_ = s.analyses.Update(ctx, a.ID, *a)
	s.publishProgress(a.SimulationID, a.ID, "starting", 5)

	pipeline := newAnalysisPipeline(s.steps, s.llm, s.cfg, s.logger)
	progress := func(stage string, pct uint8) {
		a.Progress = datasims.AnalysisProgress{Stage: stage, Percent: pct}
		a.UpdatedAt = time.Now().UTC()
		_ = s.analyses.Update(ctx, a.ID, *a)
		s.publishProgress(a.SimulationID, a.ID, stage, pct)
	}

	res, err := pipeline.run(ctx, sim, a, a.Selection, progress)
	if err != nil {
		s.logger.Warn("analysis pipeline failed", zap.String("analysis_id", a.ID), zap.Error(err))
		s.fail(ctx, a, err.Error())
		return
	}

	// Guard against an orphaned GridFS file: if the analysis (and thus its
	// simulation) was deleted mid-pipeline, do not write a new file that nothing
	// will ever reference or clean up.
	if _, gerr := s.analyses.GetByID(ctx, a.ID); errors.Is(gerr, common.ErrNotFound) {
		s.logger.Info("analysis deleted mid-pipeline; skipping file upload", zap.String("analysis_id", a.ID))
		return
	}

	// Persist the PDF to GridFS.
	gridfsID, size, ferr := s.files.Upload(ctx, res.FileName, "application/pdf", a.SimulationID, a.ID, res.PDF)
	if ferr != nil {
		s.logger.Error("analysis pdf upload failed", zap.String("analysis_id", a.ID), zap.Error(ferr))
		s.fail(ctx, a, "failed to store the report file")
		return
	}

	fin := time.Now().UTC()
	a.Report = &res.Report
	a.StepsCovered = res.StepsCovered
	a.TokenUsage = res.Usage
	a.Requests = res.Requests
	a.Warnings = res.Warnings
	a.File = &common.File{
		UID:       uuid.NewString(),
		Name:      res.FileName,
		Type:      common.SimulationFileAnalysisReport,
		MimeType:  "application/pdf",
		Size:      size,
		GridFSID:  gridfsID,
		CreatedAt: fin,
		CreatedBy: a.CreatedBy,
	}
	a.Status = datasims.AnalysisStatusCompleted
	a.StatusText = "completed"
	a.Progress = datasims.AnalysisProgress{Stage: "completed", Percent: 100}
	a.FinishedAt = &fin
	a.UpdatedAt = fin
	if err := s.analyses.Update(ctx, a.ID, *a); err != nil {
		s.logger.Error("failed to persist completed analysis", zap.String("analysis_id", a.ID), zap.Error(err))
		return
	}
	s.hub.Publish(a.SimulationID, Event{
		Type: EventAnalysisCompleted, SimulationID: a.SimulationID,
		Payload: AnalysisCompletedPayload{AnalysisID: a.ID, Status: string(a.Status), FileUID: a.File.UID, FileName: a.File.Name, FileSize: a.File.Size},
	})
	s.logger.Info("analysis completed", zap.String("analysis_id", a.ID))
}

func (s *analysisService) fail(ctx context.Context, a *datasims.Analysis, msg string) {
	now := time.Now().UTC()
	a.Status = datasims.AnalysisStatusFailed
	a.StatusText = "failed"
	a.Error = msg
	a.Progress = datasims.AnalysisProgress{Stage: "failed", Percent: 100}
	a.FinishedAt = &now
	a.UpdatedAt = now
	if err := s.analyses.Update(ctx, a.ID, *a); err != nil {
		s.logger.Error("failed to persist failed analysis", zap.String("analysis_id", a.ID), zap.Error(err))
	}
	s.hub.Publish(a.SimulationID, Event{
		Type: EventAnalysisFailed, SimulationID: a.SimulationID,
		Payload: AnalysisFailedPayload{AnalysisID: a.ID, Error: msg},
	})
}

func (s *analysisService) publishProgress(simID, analysisID, stage string, pct uint8) {
	s.hub.Publish(simID, Event{
		Type: EventAnalysisProgress, SimulationID: simID,
		Payload: AnalysisProgressPayload{AnalysisID: analysisID, Stage: stage, Percent: pct},
	})
}

func (s *analysisService) List(ctx context.Context, simID string) (*ListAnalysesResponse, error) {
	sim, _, err := requireOwnedSimulation(ctx, s.repo, simID)
	if err != nil {
		return nil, err
	}
	list, err := s.analyses.ListBySimulation(ctx, sim.ID)
	if err != nil {
		return nil, err
	}
	items := make([]AnalysisSummaryDTO, 0, len(list))
	for _, a := range list {
		items = append(items, analysisToSummaryDTO(a))
	}
	return &ListAnalysesResponse{Items: items}, nil
}

func (s *analysisService) Get(ctx context.Context, simID, analysisID string) (*AnalysisResponse, error) {
	_, a, err := s.requireOwnedAnalysis(ctx, simID, analysisID)
	if err != nil {
		return nil, err
	}
	resp := analysisToResponse(*a)
	return &resp, nil
}

func (s *analysisService) Delete(ctx context.Context, simID, analysisID string) error {
	_, a, err := s.requireOwnedAnalysis(ctx, simID, analysisID)
	if err != nil {
		return err
	}
	if a.File != nil && a.File.GridFSID != "" {
		if derr := s.files.Delete(ctx, a.File.GridFSID); derr != nil {
			s.logger.Warn("failed to delete analysis file", zap.String("analysis_id", a.ID), zap.Error(derr))
		}
	}
	if err := s.analyses.Delete(ctx, a.ID); err != nil {
		return err
	}
	return nil
}

func (s *analysisService) OpenFile(ctx context.Context, simID, analysisID string) (io.ReadCloser, *datasims.AnalysisFileInfo, string, error) {
	_, a, err := s.requireOwnedAnalysis(ctx, simID, analysisID)
	if err != nil {
		return nil, nil, "", err
	}
	if a.Status != datasims.AnalysisStatusCompleted || a.File == nil || a.File.GridFSID == "" {
		return nil, nil, "", ErrAnalysisNotReady
	}
	rc, info, err := s.files.Open(ctx, a.File.GridFSID)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, nil, "", ErrAnalysisNotFound
		}
		return nil, nil, "", err
	}
	return rc, info, a.File.Name, nil
}

func (s *analysisService) ResultsSummary(ctx context.Context, simID string) (*ResultsSummaryDTO, error) {
	sim, _, err := requireOwnedSimulation(ctx, s.repo, simID)
	if err != nil {
		return nil, err
	}
	min, max, rerr := s.steps.GetStepRange(ctx, sim.ID)
	hasResults := rerr == nil && max >= min
	out := &ResultsSummaryDTO{
		StepRange:  ResultsStepRangeDTO{Min: uint64(maxInt64(min, 0)), Max: uint64(maxInt64(max, 0))},
		HasResults: hasResults,
		FocusAreas: []string{"population", "properties", "spatial", "interactions", "events", "rules"},
	}
	// Agent types from the model snapshot.
	nameByUID := map[string]string{}
	typeByUID := map[string]string{}
	if sim.ModelSnapshot != nil {
		for _, ag := range sim.ModelSnapshot.Agents {
			out.AgentTypes = append(out.AgentTypes, ResultsAgentTypeDTO{UID: ag.UID, Name: ag.Name, Type: string(ag.Type)})
			for _, p := range ag.Properties {
				nameByUID[p.UID] = p.Name
				typeByUID[p.UID] = string(p.Type)
			}
		}
	}
	// Properties recorded in the steps (intersect with model names/types).
	recorded, rerr2 := s.steps.FindDistinctProperties(ctx, sim.ID)
	if rerr2 != nil {
		return nil, rerr2
	}
	for _, rp := range recorded {
		name := nameByUID[rp.UID]
		if name == "" {
			name = rp.UID
		}
		t := typeByUID[rp.UID]
		if t == "" {
			// Normalize recorded-inference numeric labels to the model
			// vocabulary so the UI matches the analysis numeric filter.
			t = normalizeRecordedType(rp.DataType)
		}
		out.Properties = append(out.Properties, ResultsPropertyDTO{UID: rp.UID, Name: name, Type: t, EntityType: "agent"})
	}
	// Shape properties recorded in the steps (intersect with shape names/types
	// from the model snapshot's shape declarations).
	shapeNameByUID := map[string]string{}
	shapeTypeByUID := map[string]string{}
	if sim.ModelSnapshot != nil && sim.ModelSnapshot.Environment != nil {
		for _, sh := range sim.ModelSnapshot.Environment.Shapes {
			for _, p := range sh.Properties {
				shapeNameByUID[p.UID] = p.Name
				shapeTypeByUID[p.UID] = string(p.Type)
			}
		}
	}
	shapeRecorded, srerr := s.steps.FindDistinctShapeProperties(ctx, sim.ID)
	if srerr != nil {
		return nil, srerr
	}
	for _, rp := range shapeRecorded {
		name := shapeNameByUID[rp.UID]
		if name == "" {
			name = rp.UID
		}
		t := shapeTypeByUID[rp.UID]
		if t == "" {
			t = normalizeRecordedType(rp.DataType)
		}
		out.Properties = append(out.Properties, ResultsPropertyDTO{UID: rp.UID, Name: name, Type: t, EntityType: "shape"})
	}
	return out, nil
}

// normalizeRecordedType maps the recorded-inference numeric labels
// ("integer"/"float") to the model vocabulary ("number"); other labels pass through.
func normalizeRecordedType(t string) string {
	if t == "integer" || t == "float" {
		return "number"
	}
	return t
}

func (s *analysisService) ResultsSeries(ctx context.Context, simID string, q ResultsSeriesQuery) (*ResultsSeriesDTO, error) {
	sim, _, err := requireOwnedSimulation(ctx, s.repo, simID)
	if err != nil {
		return nil, err
	}
	if q.EntityType != "" && q.EntityType != "agent" && q.EntityType != "shape" {
		return nil, &ValidationError{Result: ValidationResult{Errors: []ValidationIssue{
			{Path: "entity_type", Reason: "unknown entity_type value: " + q.EntityType},
		}}}
	}
	forShapes := q.EntityType == "shape"
	if forShapes && q.PropertyUID == "" {
		return nil, &ValidationError{Result: ValidationResult{Errors: []ValidationIssue{
			{Path: "property", Reason: "property is required when entity_type=shape"},
		}}}
	}
	stepRange := &datasims.StepRange{Start: q.From, End: q.To}
	out := &ResultsSeriesDTO{PropertyUID: q.PropertyUID}
	if q.PropertyUID != "" {
		// Field-path safety + existence: reject unsafe characters or any UID
		// not among the recorded distinct properties (avoids an unvalidated
		// user-controlled Mongo field path).
		if strings.ContainsAny(q.PropertyUID, ".$") {
			return nil, &ValidationError{Result: ValidationResult{Errors: []ValidationIssue{
				{Path: "property", Reason: "property contains illegal characters"},
			}}}
		}
		recorded, rerr := s.steps.FindDistinctProperties(ctx, sim.ID)
		if forShapes {
			recorded, rerr = s.steps.FindDistinctShapeProperties(ctx, sim.ID)
		}
		if rerr != nil {
			return nil, rerr
		}
		known := false
		for _, rp := range recorded {
			if rp.UID == q.PropertyUID {
				known = true
				break
			}
		}
		if !known {
			return nil, &ValidationError{Result: ValidationResult{Errors: []ValidationIssue{
				{Path: "property", Reason: "unknown property uid: " + q.PropertyUID},
			}}}
		}
		agg := datasims.AggregationAvg
		if q.Aggregation != "" {
			if !datasims.IsValidAggregation(datasims.Aggregation(q.Aggregation)) {
				return nil, &ValidationError{Result: ValidationResult{Errors: []ValidationIssue{
					{Path: "aggregation", Reason: "unknown aggregation value: " + q.Aggregation},
				}}}
			}
			agg = datasims.Aggregation(q.Aggregation)
		}
		var pts []datasims.TimeSeriesPoint
		if forShapes {
			pts, err = s.steps.AggregateShapePropertyByStep(ctx, sim.ID, q.PropertyUID, q.AgentUIDs, stepRange, agg)
		} else {
			pts, err = s.steps.AggregatePropertyByStep(ctx, sim.ID, q.PropertyUID, q.AgentUIDs, stepRange, agg)
		}
		if err != nil {
			return nil, err
		}
		for _, p := range pts {
			out.Series = append(out.Series, ResultsSeriesPointDTO{Step: p.Step, Value: p.Value})
		}
		return out, nil
	}
	pop, err := s.steps.CountByAgentTypePerStep(ctx, sim.ID, stepRange)
	if err != nil {
		return nil, err
	}
	for _, p := range pop {
		out.Population = append(out.Population, PopulationCountDTO{Step: p.Step, Count: p.Count})
	}
	return out, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
