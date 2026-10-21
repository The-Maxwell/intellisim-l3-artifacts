package simulations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// controlAction enumerates the in-flight run control messages delivered over a
// run's control channel.
type controlAction int

const (
	ctrlPause controlAction = iota
	ctrlResume
	ctrlStop
	ctrlSpeed
	ctrlAgentQuery
	ctrlKeyframe
)

// controlMsg is one control directive for a live run. speedMs is meaningful only
// for ctrlSpeed (the new inter-step delay in milliseconds); agentID/reply only
// for ctrlAgentQuery (live-agent snapshot request; the loop answers with a deep
// copy, or nil when the agent id is unknown/removed).
type controlMsg struct {
	action  controlAction
	speedMs int
	agentID uint64
	reply   chan *datasims.SimulationAgent
}

// runHandle is the in-memory record of one live run: a cancel func (closing the
// run context) and the control channel the run loop selects on.
type runHandle struct {
	cancel  context.CancelFunc
	control chan controlMsg
}

// stepBroadcaster is the narrow WebSocket-stream port the RunManager depends on.
// The real StreamHub (Phase 6) satisfies it; tests use a fake. Keeping the engine
// behind this interface lets the run loop compile and be tested without the WS
// transport.
type stepBroadcaster interface {
	// OpenRoom creates the sim's stream room at run start so the room lifecycle is
	// owned by the RunManager (created on start, torn down on completion). Without
	// it the room would only ever be created lazily by a joining client, which
	// could orphan a room for a simulation that is not (or no longer) running.
	// batchSize carries the resolved per-room chunk size (0 → streamFrameDefaultBatch).
	OpenRoom(simID string, batchSize int)
	// BroadcastStep streams one per-entity step frame. changedOnly hints the hub
	// that the frame carries only changed entities (vs a full keyframe).
	BroadcastStep(simID string, step datasims.SimulationStep, changedOnly bool)
	// BroadcastStepUsage streams the run's live token-usage breakdown (cumulative)
	// plus this step's delta once per step, as a step frame.
	BroadcastStepUsage(simID string, step uint64, usage *SimulationTokenUsageDTO, delta StreamTokenDelta)
	// BroadcastKeyframe streams a full-state snapshot for late joiners / reconnects.
	BroadcastKeyframe(simID string, rs *RuntimeState)
	// BroadcastStatus streams a run lifecycle status change.
	BroadcastStatus(simID string, status datasims.SimulationStatus)
	// BroadcastCompletion streams the terminal run status, then the room is torn down.
	BroadcastCompletion(simID string, status datasims.SimulationStatus)
	// CloseRoom tears down the sim's room (all clients disconnected).
	CloseRoom(simID string)
}

// noopBroadcaster is the default stepBroadcaster used when none is wired (the WS
// hub lands in Phase 6). It drops every frame.
type noopBroadcaster struct{}

func (noopBroadcaster) OpenRoom(string, int)                                {}
func (noopBroadcaster) BroadcastStep(string, datasims.SimulationStep, bool) {}
func (noopBroadcaster) BroadcastStepUsage(string, uint64, *SimulationTokenUsageDTO, StreamTokenDelta) {
}
func (noopBroadcaster) BroadcastKeyframe(string, *RuntimeState)               {}
func (noopBroadcaster) BroadcastStatus(string, datasims.SimulationStatus)     {}
func (noopBroadcaster) BroadcastCompletion(string, datasims.SimulationStatus) {}
func (noopBroadcaster) CloseRoom(string)                                      {}

// EventRunProgress is the SSE event type carrying live run progress (step counter +
// status). It complements the WS step stream so list/editor views stay in sync
// without opening a WebSocket.
const EventRunProgress EventType = "run.progress"

// EventRunCompleted is the SSE event type marking a run's terminal transition.
const EventRunCompleted EventType = "run.completed"

// RunProgressPayload carries live run progress for the SSE hub.
type RunProgressPayload struct {
	// Run is the 1-based execution run sequence number (ExecutionRun.Seq).
	Run        uint64                    `json:"run"`
	Step       uint64                    `json:"step"`
	TotalSteps uint64                    `json:"total_steps"`
	Status     datasims.SimulationStatus `json:"status"`
}

// RunCompletionPayload carries the terminal run outcome for the SSE hub.
type RunCompletionPayload struct {
	Run            uint64                    `json:"run"`
	Status         datasims.SimulationStatus `json:"status"`
	StepsCompleted uint64                    `json:"steps_completed"`
	Error          string                    `json:"error,omitempty"`
}

// RunManager owns every in-process engine run keyed by simulation id. It enforces
// one run per simulation and a global concurrency cap, spawns the per-run goroutine
// (detached from the request context), and reconciles interrupted runs at startup.
type RunManager struct {
	mu     sync.Mutex
	runs   map[string]*runHandle
	cfg    RunConfig
	repo   datasims.SimulationsRepository
	steps  datasims.SimulationStepsRepository
	llmInv llmInvoker
	hub    *EventsHub
	stream stepBroadcaster
	logger *zap.Logger
}

// NewRunManager builds the run manager. stream may be nil (a no-op broadcaster is
// substituted until the WS hub is wired in Phase 6); a nil logger becomes a no-op.
func NewRunManager(
	cfg RunConfig,
	repo datasims.SimulationsRepository,
	steps datasims.SimulationStepsRepository,
	llmInv llmInvoker,
	hub *EventsHub,
	stream stepBroadcaster,
	logger *zap.Logger,
) *RunManager {
	if stream == nil {
		stream = noopBroadcaster{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RunManager{
		runs:   make(map[string]*runHandle),
		cfg:    cfg,
		repo:   repo,
		steps:  steps,
		llmInv: llmInv,
		hub:    hub,
		stream: stream,
		logger: logger.Named("run-manager"),
	}
}

// IsRunning reports whether a live run handle exists for the simulation.
func (m *RunManager) IsRunning(simID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.runs[simID]
	return ok
}

// Start validates the simulation is minimally runnable, claims the one-run slot
// (ErrRunInFlight) within the global capacity (ErrRunCapacity), appends an
// ExecutionRun, transitions the simulation to running, persists, and spawns the
// run loop on a context detached from the caller (context.WithoutCancel). The
// caller may return its 202 immediately; the loop owns the simulation thereafter.
func (m *RunManager) Start(ctx context.Context, sim *datasims.Simulation) error {
	if sim == nil {
		return ErrNotRunnable
	}

	// Claim the one-run slot first: a second Start for the same simulation is an
	// in-flight conflict (409) regardless of the freshly-loaded record's status,
	// which a concurrent caller would already see as running.
	m.mu.Lock()
	if _, busy := m.runs[sim.ID]; busy {
		m.mu.Unlock()
		return ErrRunInFlight
	}
	if m.cfg.MaxConcurrentRuns > 0 && len(m.runs) >= m.cfg.MaxConcurrentRuns {
		m.mu.Unlock()
		return ErrRunCapacity
	}
	if !runnable(sim) {
		m.mu.Unlock()
		return ErrNotRunnable
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	h := &runHandle{cancel: cancel, control: make(chan controlMsg, 8)}
	m.runs[sim.ID] = h
	m.mu.Unlock()

	// Append the audit run + flip status, then persist before the loop spins up so
	// the running state is durable (and reconcilable) even if the process dies.
	now := nowFn().UTC()
	runSeq := uint64(len(sim.ExecutionRuns) + 1)
	sim.ExecutionRuns = append(sim.ExecutionRuns, datasims.ExecutionRun{
		Seq:       runSeq,
		StartedAt: now,
		Status:    datasims.ExecutionRunRunning,
	})
	sim.Status = datasims.SimulationStatusRunning
	sim.StatusText = "running"
	sim.StatusHistory = append(sim.StatusHistory, datasims.SimulationStatusChange{
		Status: datasims.SimulationStatusRunning, At: now, By: "system",
	})
	sim.StartedAt = &now
	sim.UpdatedAt = now
	if sim.TokenUsage == nil {
		sim.TokenUsage = &datasims.SimulationTokenUsage{}
	}

	if err := m.repo.Update(runCtx, sim.ID, *sim); err != nil {
		m.clearHandle(sim.ID)
		cancel()
		m.logger.Error("failed to persist run start", zap.String("simulation_id", sim.ID), zap.Error(err))
		return err
	}

	// Open the stream room now (run start) so its lifecycle is owned by the run,
	// not by the first joining client. finalize() tears it down via CloseRoom.
	m.stream.OpenRoom(sim.ID, resolveStreamBatchSize(sim))
	m.stream.BroadcastStatus(sim.ID, datasims.SimulationStatusRunning)
	m.publishProgress(sim.ID, runSeq, sim.CurrentStep, sim.Steps, datasims.SimulationStatusRunning)

	go m.runLoop(runCtx, h, sim)
	return nil
}

// Control delivers a control message to a live run. ErrNotRunning when no run
// handle exists for the simulation. Delivery is non-blocking-ish: the channel is
// buffered, and a full buffer falls back to a bounded send so a slow loop cannot
// wedge the caller indefinitely while still honoring stop.
func (m *RunManager) Control(simID string, msg controlMsg) error {
	m.mu.Lock()
	h, ok := m.runs[simID]
	m.mu.Unlock()
	if !ok {
		return ErrNotRunning
	}
	select {
	case h.control <- msg:
	case <-time.After(time.Second):
		return ErrNotRunning
	}
	return nil
}

// RequestKeyframe asks the sim's active run loop to broadcast a full-state
// keyframe to its stream room (used right after a WS client joins, so late
// joiners of a running OR paused run sync immediately). Best-effort: no-op when
// no run is live or the control queue is saturated.
func (m *RunManager) RequestKeyframe(simID string) {
	m.mu.Lock()
	h, ok := m.runs[simID]
	m.mu.Unlock()
	if !ok {
		return
	}
	select {
	case h.control <- controlMsg{action: ctrlKeyframe}:
	default:
	}
}

// LiveAgent returns a deep-copied snapshot of one agent from the sim's ACTIVE
// run (running or paused), or ok=false when no run is live or the loop did not
// answer in time. A (nil, true) result means the run is live but the agent id is
// unknown/removed. The query rides the control channel and is answered at a step
// boundary (or immediately while paused), so it never races the step pipeline
// and never blocks the run loop.
func (m *RunManager) LiveAgent(simID string, agentID uint64) (*datasims.SimulationAgent, bool) {
	m.mu.Lock()
	h, ok := m.runs[simID]
	m.mu.Unlock()
	if !ok {
		return nil, false
	}
	reply := make(chan *datasims.SimulationAgent, 1)
	select {
	case h.control <- controlMsg{action: ctrlAgentQuery, agentID: agentID, reply: reply}:
	case <-time.After(time.Second):
		return nil, false
	}
	select {
	case a := <-reply:
		return a, true
	case <-time.After(2 * time.Second):
		return nil, false
	}
}

// copySimulationAgent deep-copies the persistent agent fields (Position,
// Properties, Memory) so callers never alias run-owned state.
func copySimulationAgent(a *datasims.SimulationAgent) *datasims.SimulationAgent {
	cp := *a
	if a.Position != nil {
		p := *a.Position
		cp.Position = &p
	}
	if a.Properties != nil {
		props := make(map[string]interface{}, len(a.Properties))
		for k, v := range a.Properties {
			props[k] = v
		}
		cp.Properties = props
	}
	cp.Memory = append([]datasims.AgentMemory(nil), a.Memory...)
	return &cp
}

// snapshotFinalAgents deep-copies the live runtime agents into standalone
// SimulationAgent values (position/properties/state/memory as of the current
// step), skipping removed/nil agents. Captured at run finalize and persisted to
// Simulation.FinalAgents so GetAgent can surface the real final world post-run
// (the initial sim.Agents is never rewritten). Returns nil for a nil runtime.
func snapshotFinalAgents(rs *RuntimeState) []datasims.SimulationAgent {
	if rs == nil {
		return nil
	}
	out := make([]datasims.SimulationAgent, 0, len(rs.Agents))
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		out = append(out, *copySimulationAgent(ra.A))
	}
	return out
}

// runLoop is the per-run goroutine. It builds the in-memory runtime state, drives
// RunStep until the configured step count (→ finished), an explicit stop
// (→ stopped), or an unrecoverable error (→ failed), honoring pause (block on the
// control channel) and speed (inter-step delay). Steps are buffered and flushed to
// the timeseries in batches; every frame is broadcast over the WS stream and the
// SSE progress hub. On exit it finalizes the ExecutionRun + Statistics + status and
// persists, then publishes completion and tears down the room.
func (m *RunManager) runLoop(ctx context.Context, h *runHandle, sim *datasims.Simulation) {
	runIdx := len(sim.ExecutionRuns) - 1
	runSeq := sim.ExecutionRuns[runIdx].Seq

	final := datasims.SimulationStatusFinished
	termReason := "" // U2: why a finished run ended (condition_met | max_steps)
	var runErr error

	defer func() {
		if r := recover(); r != nil {
			final = datasims.SimulationStatusFailed
			runErr = &panicError{v: r}
			m.logger.Error("run loop panicked", zap.String("simulation_id", sim.ID), zap.Any("panic", r))
		}
		m.finalize(ctx, sim, runIdx, runSeq, final, termReason, runErr)
		m.clearHandle(sim.ID)
		h.cancel()
	}()

	rs, err := NewRuntimeState(sim)
	if err != nil {
		final = datasims.SimulationStatusFailed
		runErr = err
		return
	}

	var llmRT *llmRuntime
	if m.llmInv != nil {
		var simCfg *models.LLMSimulationExecutionConfig
		if sim.ModelSnapshot != nil {
			simCfg = &sim.ModelSnapshot.SimulationConfig.LLMSimulationExecutionConfig
		}
		llmRT = newLLMRuntime(m.llmInv, m.cfg, sim.TokenUsage, runIdx, m.cfg.GenerationTimeout, simCfg, m.logger)
	}

	delay := time.Duration(m.cfg.DefaultStepDelayMs) * time.Millisecond
	flushSize := m.cfg.StepFlushSize
	if flushSize < 1 {
		flushSize = 1
	}
	flushEvery := time.Duration(m.cfg.StepFlushIntervalMs) * time.Millisecond

	var buffer []datasims.SimulationStep
	lastFlush := time.Now()
	flush := func() {
		if len(buffer) == 0 || m.steps == nil {
			buffer = buffer[:0]
			lastFlush = time.Now()
			return
		}
		if err := m.steps.SaveSteps(ctx, buffer); err != nil {
			m.logger.Warn("failed to flush steps batch", zap.String("simulation_id", sim.ID), zap.Error(err))
		}
		buffer = buffer[:0]
		lastFlush = time.Now()
	}

	// prevUsage tracks the cumulative token total as of the PREVIOUS step so each
	// step can carry its own delta. Zero value = pre-run baseline (nothing spent).
	var prevUsage common.TokenUsage

	paused := false
	for {
		// Terminal: reached the configured step count.
		if rs.Step >= sim.Steps {
			final = datasims.SimulationStatusFinished
			termReason = datasims.TerminationMaxSteps
			break
		}

		// Drain any pending control messages; block here while paused.
		stop, quit := m.handleControl(ctx, h, sim, rs, &paused, &delay)
		if quit {
			final = datasims.SimulationStatusStopped
			break
		}
		if stop {
			final = datasims.SimulationStatusStopped
			break
		}
		if paused {
			continue
		}

		if ctx.Err() != nil {
			final = datasims.SimulationStatusStopped
			break
		}

		rows, err := RunStepRows(ctx, rs, llmRT)
		if err != nil {
			final = datasims.SimulationStatusFailed
			runErr = err
			break
		}
		for _, row := range rows {
			buffer = append(buffer, row)
			m.stream.BroadcastStep(sim.ID, row, true)
		}
		// Live token-usage snapshot for the step just completed. Read once per step
		// (never per row) from the run-shared accumulator. Race-free here: the
		// parallel LLM Plan goroutines are joined inside RunStepRows and AddRun is
		// mutex-guarded, so between steps nothing else mutates sim.TokenUsage. The
		// completed step is rs.Step-1 (RunStepRows already incremented rs.Step),
		// matching the step number the entity frames carry.
		if sim.TokenUsage != nil && rs.Step > 0 {
			cur := sim.TokenUsage.Total
			m.stream.BroadcastStepUsage(sim.ID, rs.Step-1, simulationTokenUsageToDTO(sim.TokenUsage), tokenDelta(cur, prevUsage))
			prevUsage = cur
		}
		if len(buffer) >= flushSize || (flushEvery > 0 && time.Since(lastFlush) >= flushEvery) {
			flush()
		}
		m.publishProgress(sim.ID, runSeq, rs.Step, sim.Steps, datasims.SimulationStatusRunning)

		// U2: evaluate the optional early-termination condition at the step
		// boundary (post-commit, post-broadcast). When it holds, the run finishes
		// early; condition_met wins over max_steps if both hold on the same step
		// (the max-steps check at the loop top would only run next iteration).
		if sim.Termination != nil && evalTermination(rs, *sim.Termination) {
			final = datasims.SimulationStatusFinished
			termReason = datasims.TerminationConditionMet
			break
		}

		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				final = datasims.SimulationStatusStopped
				flush()
				sim.Statistics = ComputeStatistics(rs)
				sim.CurrentStep = rs.Step
				sim.FinalAgents = snapshotFinalAgents(rs)
				return
			}
		}
	}

	flush()
	// Capture the post-loop runtime snapshot so finalize persists final stats +
	// step + the final agent world (position/properties/state/memory).
	sim.Statistics = ComputeStatistics(rs)
	sim.CurrentStep = rs.Step
	sim.FinalAgents = snapshotFinalAgents(rs)

	// Log any non-fatal diagnostic warnings accumulated during the run (deduped,
	// bounded to 50 entries so a busy run doesn't flood the log).
	logRunWarnings(m.logger, sim.ID, rs.warnings)
}

// handleControl drains every queued control message (non-blocking) and, when
// paused, blocks waiting for a resume/stop. It returns stop=true when a ctrlStop
// was seen and quit=true when the context was cancelled while blocked.
func (m *RunManager) handleControl(ctx context.Context, h *runHandle, sim *datasims.Simulation, rs *RuntimeState, paused *bool, delay *time.Duration) (stop, quit bool) {
	apply := func(msg controlMsg) (stopReq bool) {
		switch msg.action {
		case ctrlStop:
			return true
		case ctrlPause:
			if !*paused {
				*paused = true
				m.markPaused(ctx, sim, rs, true)
			}
		case ctrlResume:
			if *paused {
				*paused = false
				m.markPaused(ctx, sim, rs, false)
			}
		case ctrlSpeed:
			if msg.speedMs >= 0 {
				*delay = time.Duration(msg.speedMs) * time.Millisecond
			}
		case ctrlKeyframe:
			// A client joined the stream room: broadcast a full-state keyframe so
			// late joiners (notably of a PAUSED run, where no step frames flow)
			// sync immediately. Handled here (step boundary / paused block), so
			// the snapshot cannot race the step pipeline.
			m.stream.BroadcastKeyframe(sim.ID, rs)
			// Also sync the cumulative token-usage breakdown (zero delta) so a fresh
			// or paused client sees current totals without waiting for a step frame.
			if sim.TokenUsage != nil {
				m.stream.BroadcastStepUsage(sim.ID, rs.Step, simulationTokenUsageToDTO(sim.TokenUsage), StreamTokenDelta{})
			}
		case ctrlAgentQuery:
			// Answer with a deep copy so the requester never aliases run-owned
			// state. Handled here (step boundary / paused block), so the read
			// cannot race the step pipeline. Non-blocking send: the reply channel
			// is buffered and the requester may have timed out.
			if msg.reply != nil {
				var snap *datasims.SimulationAgent
				if ra, ok := rs.ByID[msg.agentID]; ok && ra != nil && ra.A != nil && !ra.removed {
					snap = copySimulationAgent(ra.A)
				}
				select {
				case msg.reply <- snap:
				default:
				}
			}
		}
		return false
	}

	// Drain everything currently queued without blocking.
	for {
		select {
		case msg := <-h.control:
			if apply(msg) {
				return true, false
			}
			continue
		default:
		}
		break
	}

	// While paused, block until a control message or context cancellation arrives.
	for *paused {
		select {
		case msg := <-h.control:
			if apply(msg) {
				return true, false
			}
		case <-ctx.Done():
			return false, true
		}
	}
	return false, false
}

// markPaused flips the persisted status between running and paused mid-run and
// broadcasts the change. Persistence failures are logged but never abort the run.
func (m *RunManager) markPaused(ctx context.Context, sim *datasims.Simulation, rs *RuntimeState, paused bool) {
	status := datasims.SimulationStatusRunning
	text := "running"
	if paused {
		status = datasims.SimulationStatusPaused
		text = "paused"
	}
	now := nowFn().UTC()
	sim.Status = status
	sim.StatusText = text
	sim.StatusHistory = append(sim.StatusHistory, datasims.SimulationStatusChange{Status: status, At: now, By: "system"})
	sim.CurrentStep = rs.Step
	sim.UpdatedAt = now
	if err := m.repo.Update(ctx, sim.ID, *sim); err != nil {
		m.logger.Warn("failed to persist pause/resume", zap.String("simulation_id", sim.ID), zap.Error(err))
	}
	m.stream.BroadcastStatus(sim.ID, status)
	m.publishProgress(sim.ID, runSeqOf(sim), rs.Step, sim.Steps, status)
}

// activeRunDuration computes the time a run spent actually RUNNING — the wall
// clock from startedAt to finishedAt minus every interval the run spent paused.
// Paused intervals are read from the status history: each `paused` entry whose
// timestamp falls inside the run window [startedAt, finishedAt] contributes the
// gap until the next transition (or finishedAt, if the run finalized while
// paused). Entries outside the window (e.g. a prior run's transitions) are
// ignored. The result is clamped to [0, +inf). Pure/deterministic — no clock.
func activeRunDuration(startedAt, finishedAt time.Time, history []datasims.SimulationStatusChange) time.Duration {
	total := finishedAt.Sub(startedAt)
	if total <= 0 {
		return 0
	}
	var paused time.Duration
	for i, h := range history {
		if h.Status != datasims.SimulationStatusPaused {
			continue
		}
		// Only this run's pauses: within (startedAt, finishedAt].
		if h.At.Before(startedAt) || h.At.After(finishedAt) {
			continue
		}
		end := finishedAt
		if i+1 < len(history) && !history[i+1].At.After(finishedAt) {
			end = history[i+1].At
		}
		if gap := end.Sub(h.At); gap > 0 {
			paused += gap
		}
	}
	active := total - paused
	if active < 0 {
		active = 0
	}
	return active
}

// finalize closes out a run: finalize the ExecutionRun audit record, persist the
// terminal status + statistics + current step, then publish completion over the WS
// stream + SSE hub and tear down the room.
func (m *RunManager) finalize(ctx context.Context, sim *datasims.Simulation, runIdx int, runSeq uint64, final datasims.SimulationStatus, termReason string, runErr error) {
	now := nowFn().UTC()
	if runIdx >= 0 && runIdx < len(sim.ExecutionRuns) {
		run := &sim.ExecutionRuns[runIdx]
		run.FinishedAt = &now
		run.StepsCompleted = sim.CurrentStep
		switch final {
		case datasims.SimulationStatusFinished:
			run.Status = datasims.ExecutionRunFinished
			run.TerminationReason = termReason // U2: condition_met | max_steps (finished runs only)
		case datasims.SimulationStatusStopped:
			run.Status = datasims.ExecutionRunStopped
		default:
			run.Status = datasims.ExecutionRunFailed
		}
		if runErr != nil {
			run.Error = runErr.Error()
		}
		// Compute BEFORE the terminal StatusHistory entry is appended below: the
		// window end is `now` (= FinishedAt), so a run that finalized while paused
		// still has its trailing paused gap excluded.
		run.ActiveDuration = activeRunDuration(run.StartedAt, now, sim.StatusHistory)
	}

	sim.Status = final
	sim.StatusText = string(final)
	if runErr != nil {
		sim.StatusText = "failed: " + runErr.Error()
	}
	sim.StatusHistory = append(sim.StatusHistory, datasims.SimulationStatusChange{Status: final, At: now, By: "system"})
	sim.FinishedAt = &now
	sim.UpdatedAt = now

	if err := m.repo.Update(ctx, sim.ID, *sim); err != nil {
		m.logger.Error("failed to persist run finalization", zap.String("simulation_id", sim.ID), zap.Error(err))
	}

	stepsCompleted := sim.CurrentStep
	errText := ""
	if runErr != nil {
		errText = runErr.Error()
	}
	if m.hub != nil {
		m.hub.Publish(sim.ID, Event{
			Type:         EventRunCompleted,
			SimulationID: sim.ID,
			Payload:      RunCompletionPayload{Run: runSeq, Status: final, StepsCompleted: stepsCompleted, Error: errText},
		})
		m.hub.Publish(sim.ID, Event{Type: EventSimulationUpdated, SimulationID: sim.ID, Payload: nil})
	}
	m.stream.BroadcastCompletion(sim.ID, final)
	m.stream.CloseRoom(sim.ID)
}

// Reconcile marks every persisted running/paused simulation that has no live run
// handle as failed (with an explanatory StatusText) and finalizes its open
// ExecutionRun. It is called once at startup (process restart leaves no live
// goroutines, so any running/paused record is a crash remnant). Wire it via the FX
// lifecycle OnStart hook in Phase 6.
func (m *RunManager) Reconcile(ctx context.Context) error {
	page := 1
	for {
		// Status-filtered listing (U9): only interrupted simulations are of
		// interest; an unfiltered walk over the whole collection blew past the
		// FX start timeout once the database grew large.
		summaries, hasMore, err := m.repo.List(ctx, datasims.ListFilter{
			Statuses: []datasims.SimulationStatus{
				datasims.SimulationStatusRunning,
				datasims.SimulationStatusPaused,
			},
			Page:     page,
			PageSize: 100,
		})
		if err != nil {
			return err
		}
		for _, sum := range summaries {
			if sum.Status != datasims.SimulationStatusRunning && sum.Status != datasims.SimulationStatusPaused {
				continue
			}
			if m.IsRunning(sum.ID) {
				continue
			}
			m.reconcileOne(ctx, sum.ID)
		}
		if !hasMore {
			break
		}
		page++
	}
	return nil
}

// reconcileOne loads a single interrupted simulation, marks it failed, and
// finalizes its open execution run.
func (m *RunManager) reconcileOne(ctx context.Context, id string) {
	sim, err := m.repo.GetByID(ctx, id)
	if err != nil {
		m.logger.Warn("reconcile: failed to load simulation", zap.String("simulation_id", id), zap.Error(err))
		return
	}
	if sim.Status != datasims.SimulationStatusRunning && sim.Status != datasims.SimulationStatusPaused {
		return
	}
	now := nowFn().UTC()
	if n := len(sim.ExecutionRuns); n > 0 {
		run := &sim.ExecutionRuns[n-1]
		if run.Status == datasims.ExecutionRunRunning {
			run.FinishedAt = &now
			run.Status = datasims.ExecutionRunFailed
			run.StepsCompleted = sim.CurrentStep
			run.Error = "run interrupted by process restart"
			run.ActiveDuration = activeRunDuration(run.StartedAt, now, sim.StatusHistory)
		}
	}
	sim.Status = datasims.SimulationStatusFailed
	sim.StatusText = "run interrupted by process restart"
	sim.StatusHistory = append(sim.StatusHistory, datasims.SimulationStatusChange{Status: datasims.SimulationStatusFailed, At: now, By: "system"})
	sim.FinishedAt = &now
	sim.UpdatedAt = now
	if err := m.repo.Update(ctx, id, *sim); err != nil {
		m.logger.Error("reconcile: failed to persist failed status", zap.String("simulation_id", id), zap.Error(err))
		return
	}
	m.logger.Info("reconciled interrupted run to failed", zap.String("simulation_id", id))
}

// --- helpers --------------------------------------------------------------------

// clearHandle removes a run's handle from the registry (idempotent).
func (m *RunManager) clearHandle(simID string) {
	m.mu.Lock()
	delete(m.runs, simID)
	m.mu.Unlock()
}

// publishProgress emits a run-progress SSE event when a hub is wired.
func (m *RunManager) publishProgress(simID string, runSeq, step, total uint64, status datasims.SimulationStatus) {
	if m.hub == nil {
		return
	}
	m.hub.Publish(simID, Event{
		Type:         EventRunProgress,
		SimulationID: simID,
		Payload:      RunProgressPayload{Run: runSeq, Step: step, TotalSteps: total, Status: status},
	})
}

// runnable is the minimal run-start gate (status ready, ≥1 agent, Steps ≥ 1). The
// full ValidateRunnable (target capability checks, etc.) lands in Phase 6.
func runnable(sim *datasims.Simulation) bool {
	if sim == nil {
		return false
	}
	if sim.Status != datasims.SimulationStatusReady {
		return false
	}
	if sim.Steps < 1 {
		return false
	}
	if len(sim.Agents) < 1 {
		return false
	}
	// Mirror of the ValidateRunnable zero-dimension guard: a spatial run with a
	// degenerate world would silently collapse every position to (0,0).
	if sim.ModelSnapshot != nil && sim.ModelSnapshot.IsSpatial &&
		(sim.Environment == nil || sim.Environment.Width == 0 || sim.Environment.Height == 0) {
		return false
	}
	return true
}

// runSeqOf returns the current (last) execution run sequence number, or 0.
func runSeqOf(sim *datasims.Simulation) uint64 {
	if n := len(sim.ExecutionRuns); n > 0 {
		return sim.ExecutionRuns[n-1].Seq
	}
	return 0
}

// resolveStreamBatchSize picks the batch size to use for the run's WebSocket stream
// room. It reads the model-level default from
// sim.ModelSnapshot.SimulationConfig.LLMSimulationExecutionConfig.StreamBatchSize (uint > 0), and
// falls back to streamFrameDefaultBatch when no override is set or the snapshot is nil.
// The actual clamping of values < 1 to the default is also performed by chunkStep as
// defense-in-depth, so passing 0 here is safe.
func resolveStreamBatchSize(sim *datasims.Simulation) int {
	if sim == nil || sim.ModelSnapshot == nil {
		return streamFrameDefaultBatch
	}
	cfg := sim.ModelSnapshot.SimulationConfig.LLMSimulationExecutionConfig
	if cfg.StreamBatchSize > 0 {
		return int(cfg.StreamBatchSize)
	}
	return streamFrameDefaultBatch
}

// panicError wraps a recovered panic value as an error so run finalization can
// record it on the ExecutionRun.
type panicError struct{ v any }

func (e *panicError) Error() string {
	return fmt.Sprintf("run panicked: %v", e.v)
}

// logRunWarnings emits deduped, bounded run-time diagnostic warnings via the
// run logger. Only the first occurrence of each unique warning is logged; at most
// maxWarningsLogged unique entries are emitted so a pathological run cannot flood
// the log.
const maxWarningsLogged = 50

func logRunWarnings(logger *zap.Logger, simID string, warnings []string) {
	if len(warnings) == 0 || logger == nil {
		return
	}
	seen := make(map[string]struct{}, len(warnings))
	count := 0
	for _, w := range warnings {
		if _, dup := seen[w]; dup {
			continue
		}
		seen[w] = struct{}{}
		logger.Warn("simulation run advisory", zap.String("simulation_id", simID), zap.String("warning", w))
		count++
		if count >= maxWarningsLogged {
			logger.Warn("simulation run advisory: further warnings suppressed (limit reached)",
				zap.String("simulation_id", simID),
				zap.Int("total_warnings", len(warnings)),
				zap.Int("unique_shown", count),
			)
			break
		}
	}
}
