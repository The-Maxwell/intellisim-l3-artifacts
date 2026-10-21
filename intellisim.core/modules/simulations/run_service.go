package simulations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/auth"
)

// runManagerPort is the narrow slice of *RunManager the run service depends on.
// Keeping the service behind this port lets it be unit-tested with a fake manager
// (no live engine goroutine). *RunManager satisfies it.
type runManagerPort interface {
	// Start validates, claims the one-run slot, and spawns the run loop detached
	// from ctx. Returns ErrRunInFlight / ErrRunCapacity / ErrNotRunnable on the
	// guard paths.
	Start(ctx context.Context, sim *datasims.Simulation) error
	// Control delivers a control message to a live run (ErrNotRunning if none).
	Control(simID string, msg controlMsg) error
	// IsRunning reports whether a live run handle exists for the simulation.
	IsRunning(simID string) bool
	// LiveAgent returns a deep-copied snapshot of one agent from the sim's active
	// run; ok=false when no run is live (or the loop did not answer in time), and
	// (nil, true) when the run is live but the agent id is unknown/removed.
	LiveAgent(simID string, agentID uint64) (*datasims.SimulationAgent, bool)
	// RequestKeyframe asks the live run loop to broadcast a full-state keyframe
	// to its stream room (best-effort; no-op when no run is live).
	RequestKeyframe(simID string)
}

// ticketIssuer is the narrow slice of *TicketStore the run service depends on.
// *TicketStore satisfies it.
type ticketIssuer interface {
	// Issue mints a single-use connect ticket bound to (userID, simID).
	Issue(userID, simID string, now time.Time) string
	// Consume validates and burns a ticket for simID, returning the bound userID.
	Consume(token, simID string, now time.Time) (userID string, ok bool)
}

// RunService is the run-phase use-case layer: start / control / connect-ticket /
// step-history. Every method requires an authenticated principal in ctx and
// enforces owner-or-admin ownership (mirroring Service), then delegates to the
// RunManager (lifecycle), the TicketStore (WS auth), or the steps repository
// (replay history). ConsumeTicket is the only method that does not take ctx — it
// is called from the WS upgrade handler, which has no principal until the ticket
// is consumed; the subsequent AuthorizeStream re-checks ownership.
type RunService interface {
	// Start runs a ready simulation: ownership → optional run-start execution-target
	// override (validated vs simulation_execution + persisted) → ValidateRunnable →
	// RunManager.Start, then reloads and returns the (now running) body for the 202.
	Start(ctx context.Context, id string, execOverride *common.LLMTarget) (*SimulationResponse, error)
	// Control signals a live run (pause / resume / stop, optional speed): ownership →
	// ValidateControl → RunManager.Control. A non-nil speedMs also adjusts the
	// inter-step delay.
	Control(ctx context.Context, id, action string, speedMs *int) error
	// IssueTicket mints a single-use, short-lived WS connect ticket bound to the
	// caller and the simulation: ownership → streamable-status gate → TicketStore.Issue.
	// Returns ErrNotStreamable when the simulation has no live run (only running/paused
	// simulations have an open stream room).
	IssueTicket(ctx context.Context, id string) (ticket string, expiresAt time.Time, err error)
	// ConsumeTicket validates and burns a ticket for the WS upgrade handler.
	ConsumeTicket(ticket, id string) (userID string, ok bool)
	// GetSteps returns a page of persisted steps in [from, to]: ownership → steps
	// repo. A nil to resolves to the last recorded step (via the steps repo's
	// GetStepRange) rather than the zero value, so an omitted ?to returns the
	// full recorded range instead of only step 0; no steps recorded yet yields
	// an empty page (hasMore=false) rather than an error. An explicit to (any
	// value) passes through unchanged.
	GetSteps(ctx context.Context, id string, from uint64, to *uint64, page, pageSize uint64) ([]datasims.SimulationStep, bool, error)
	// ExportSteps prepares a streaming bulk export of the persisted step history
	// (U3): ownership → range resolution (nil to → last recorded step) → a
	// stepExporter that pages through the steps repo writing CSV or JSONL. Any
	// status is exportable (a running sim yields its persisted prefix).
	ExportSteps(ctx context.Context, id, format string, from uint64, to *uint64) (*stepExporter, error)
	// GetAgent returns one agent's CURRENT state: the live runtime snapshot
	// (position/properties/state/memory) when a run is active, else the persisted
	// record. ErrAgentNotFound when the instance id exists in neither.
	GetAgent(ctx context.Context, id string, agentID uint64) (*SimulationAgentDTO, error)
	// AuthorizeStream re-checks ownership AND streamable status on the WS upgrade
	// (defense in depth after the ticket has already proven authorization): returns
	// ErrNotStreamable if the run finished between ticket issue and connect.
	AuthorizeStream(ctx context.Context, id string) error
	// RequestKeyframe asks the live run loop to broadcast a full-state keyframe to
	// its stream room. Called by the WS handler right after a successful Join (the
	// join already proved authorization); best-effort no-op when no run is live.
	RequestKeyframe(id string)
}

type runService struct {
	repo    datasims.SimulationsRepository
	mgr     runManagerPort
	tickets ticketIssuer
	steps   datasims.SimulationStepsRepository
	llm     llmInvoker
	cfg     RunConfig
	logger  *zap.Logger
}

// NewRunService builds the run-phase service. A nil logger becomes a no-op.
func NewRunService(
	repo datasims.SimulationsRepository,
	mgr runManagerPort,
	tickets ticketIssuer,
	steps datasims.SimulationStepsRepository,
	llm llmInvoker,
	cfg RunConfig,
	logger *zap.Logger,
) RunService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &runService{
		repo:    repo,
		mgr:     mgr,
		tickets: tickets,
		steps:   steps,
		llm:     llm,
		cfg:     cfg,
		logger:  logger.Named("run-service"),
	}
}

// getOwnedSimulation loads a simulation and enforces owner-or-admin access,
// mirroring service.getOwnedSimulation (the run service holds its own deps, so
// the 3-line check is replicated rather than shared). Missing/malformed ids yield
// ErrSimulationNotFound (404); a foreign non-admin sim yields auth.ErrForbidden (403).
func (s *runService) getOwnedSimulation(ctx context.Context, id string) (*datasims.Simulation, *auth.UserPrincipal, error) {
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

// ---- Start --------------------------------------------------------------------

func (s *runService) Start(ctx context.Context, id string, execOverride *common.LLMTarget) (*SimulationResponse, error) {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}

	// A second Start on an already-running (or paused mid-run) simulation is an
	// in-flight conflict (409), not a 422 not_runnable — check this BEFORE the
	// exec-target override below so we never persist an override onto a sim
	// that's already executing. The engine slot check (RunManager.Start) stays
	// the race authority; this is a fast, non-racy reject for the common case.
	if sim.Status == datasims.SimulationStatusRunning || sim.Status == datasims.SimulationStatusPaused {
		return nil, fmt.Errorf("%w: simulation %s status is %q", ErrRunInFlight, id, sim.Status)
	}

	// Run-start override: validate vs the execution capability, then apply it to the
	// simulation (and persist) so both ValidateRunnable and the engine see it.
	if execOverride != nil && execOverride.ProviderUID != "" {
		if err := s.llm.ValidateTarget(*execOverride, common.ModelCapabilitySimulationExecution); err != nil {
			s.logger.Debug("invalid run-start llm execution override", zap.Error(err))
			return nil, ErrInvalidLLMTarget
		}
		sim.LLMExecutionTargetOverride = normalizeOverride(execOverride)
		if err := s.repo.Update(ctx, id, *sim); err != nil {
			s.logger.Error("failed to persist run-start override", zap.String("simulation_id", id), zap.Error(err))
			return nil, err
		}
	}

	if err := ValidateRunnable(sim, s.llm); err != nil {
		return nil, err
	}

	if err := s.mgr.Start(ctx, sim); err != nil {
		return nil, err
	}

	// Reload so the response reflects the running status + appended ExecutionRun
	// the manager just persisted (it mutated a detached copy, not the one above).
	fresh, err := s.repo.GetByID(ctx, id)
	if err != nil {
		// The run started; failing to reload is a read hiccup, not a run failure.
		// Fall back to the in-hand copy so the caller still gets a 202 body.
		s.logger.Warn("failed to reload simulation after start", zap.String("simulation_id", id), zap.Error(err))
		fresh = sim
	}
	resp := simulationToResponse(*fresh, ValidateInstance(*fresh))
	return &resp, nil
}

// ---- Control ------------------------------------------------------------------

func (s *runService) Control(ctx context.Context, id, action string, speedMs *int) error {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return err
	}

	if err := ValidateControl(action, sim.Status); err != nil {
		return err
	}

	// set_speed is a dedicated speed-only action carrying no lifecycle verb: the
	// ctrlSpeed message below is the ONLY directive sent. Sending a lifecycle
	// verb here would be wrong — controlActionFor has no "set_speed" case, so it
	// would fall into its (pause/resume/stop-only) default and wrongly stop the
	// run. pause/resume/stop behavior (with an optional speed piggyback) is
	// unchanged.
	if action != "set_speed" {
		if err := s.mgr.Control(id, controlMsg{action: controlActionFor(action)}); err != nil {
			return err
		}
	}

	// A supplied speed_ms adjusts the inter-step delay as a separate control
	// message (the verb and the speed are independent run-loop directives).
	// ValidateControl + DTO validation (required_if) guarantee speedMs != nil
	// when action == set_speed.
	if speedMs != nil {
		if err := s.mgr.Control(id, controlMsg{action: ctrlSpeed, speedMs: *speedMs}); err != nil {
			return err
		}
	}
	return nil
}

// controlActionFor maps a validated LIFECYCLE control verb (pause/resume/stop)
// to its controlAction; set_speed never reaches this (Control short-circuits it
// before the call). An unknown verb (defensively) maps to ctrlStop.
func controlActionFor(action string) controlAction {
	switch action {
	case "pause":
		return ctrlPause
	case "resume":
		return ctrlResume
	case "stop":
		return ctrlStop
	default:
		return ctrlStop
	}
}

// ---- Tickets ------------------------------------------------------------------

func (s *runService) IssueTicket(ctx context.Context, id string) (string, time.Time, error) {
	sim, p, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return "", time.Time{}, err
	}
	// Only a live run (running/paused) has a stream room to connect to. Refusing a
	// ticket for any other status stops a WS upgrade from lazily orphaning a room +
	// goroutine for a finished/failed/never-started simulation.
	if !sim.Status.Streamable() {
		return "", time.Time{}, ErrNotStreamable
	}
	now := nowFn()
	token := s.tickets.Issue(p.ID, id, now)
	expiresAt := now.Add(time.Duration(s.cfg.StreamTicketTTLSec) * time.Second)
	return token, expiresAt, nil
}

func (s *runService) ConsumeTicket(ticket, id string) (string, bool) {
	return s.tickets.Consume(ticket, id, nowFn())
}

// ---- Steps / stream -----------------------------------------------------------

func (s *runService) GetSteps(ctx context.Context, id string, from uint64, to *uint64, page, pageSize uint64) ([]datasims.SimulationStep, bool, error) {
	if _, _, err := s.getOwnedSimulation(ctx, id); err != nil {
		return nil, false, err
	}

	resolvedTo := uint64(0)
	if to != nil {
		resolvedTo = *to
	} else {
		// No explicit to: resolve the window's upper bound to the last recorded
		// step instead of defaulting to 0 (which would only ever return step 0).
		_, maxStep, err := s.steps.GetStepRange(ctx, id)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				// No steps persisted yet — an empty page, not a failure.
				return []datasims.SimulationStep{}, false, nil
			}
			return nil, false, err
		}
		resolvedTo = uint64(maxStep)
	}

	return s.steps.GetSteps(ctx, id, from, resolvedTo, page, pageSize)
}

func (s *runService) RequestKeyframe(id string) {
	s.mgr.RequestKeyframe(id)
}

func (s *runService) GetAgent(ctx context.Context, id string, agentID uint64) (*SimulationAgentDTO, error) {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}
	// Live run (running/paused): ask the run loop for the current snapshot. The
	// reply is answered at a step boundary, so it cannot race the pipeline; a
	// timeout (very long LLM step) falls back to the persisted record below.
	if live, ok := s.mgr.LiveAgent(id, agentID); ok {
		if live == nil {
			return nil, ErrAgentNotFound
		}
		dto := agentsToDTO([]datasims.SimulationAgent{*live})[0]
		return &dto, nil
	}
	// After a run ends, prefer the final runtime agents captured at finalize
	// (position/properties/state/memory as of the last step) so replay shows the
	// real final world instead of the initial generated config. FinalAgents is nil
	// for sims that never finished a run or finished before this field existed, in
	// which case we fall back to the initial sim.Agents (historical behavior).
	for i := range sim.FinalAgents {
		if sim.FinalAgents[i].ID == agentID {
			dto := agentsToDTO([]datasims.SimulationAgent{sim.FinalAgents[i]})[0]
			return &dto, nil
		}
	}
	for i := range sim.Agents {
		if sim.Agents[i].ID == agentID {
			dto := agentsToDTO([]datasims.SimulationAgent{sim.Agents[i]})[0]
			return &dto, nil
		}
	}
	return nil, ErrAgentNotFound
}

func (s *runService) AuthorizeStream(ctx context.Context, id string) error {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return err
	}
	// Re-check streamability at WS-upgrade time (defense in depth): the run may have
	// finished in the window between IssueTicket and the connect, in which case the
	// room is gone and joining would orphan a fresh one.
	if !sim.Status.Streamable() {
		return ErrNotStreamable
	}
	return nil
}
