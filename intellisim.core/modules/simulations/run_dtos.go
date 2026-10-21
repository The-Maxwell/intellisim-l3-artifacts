package simulations

import (
	"sort"
	"time"

	datasims "intelli.sim.core/data/simulations"
)

// ===================== Run request / control DTOs =====================

// RunRequest is the body of POST /:id/run. LLMExecutionTarget, when set, is the
// run-start override of the execution LLM: it is validated against the
// simulation_execution capability and persisted onto the simulation before the run
// starts. All other run parameters come from the persisted simulation (seed/steps).
type RunRequest struct {
	LLMExecutionTarget *LLMTargetDTO `json:"llm_execution_target"`
}

// ControlRequest is the body of POST /:id/control. Action is the control verb;
// SpeedMs, when present, sets the inter-step delay in milliseconds (0 =
// fastest). set_speed is a dedicated speed-only action (no lifecycle verb) so
// the UI speed slider works while a run is live, not just piggybacked on
// pause/resume; it requires speed_ms.
type ControlRequest struct {
	Action string `json:"action" validate:"required,oneof=pause resume stop set_speed"`
	// required_if must precede omitempty: go-playground's omitempty short-circuits
	// the rest of the chain on a nil pointer, which would otherwise silently skip
	// required_if for a set_speed action with no speed_ms.
	SpeedMs *int `json:"speed_ms" validate:"required_if=Action set_speed,omitempty,min=0,max=10000"`
}

// StreamTicketResponse is the body of POST /:id/stream-ticket: a single-use,
// short-lived opaque token plus its expiry, used to authenticate the WS upgrade.
type StreamTicketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ===================== Step DTOs (replay / live history) =====================

// AgentStepDTO is one agent's state within a step frame. Position is omitted for
// non-spatial models; Properties/State carry the per-step snapshot. AgentUID is
// the archetype's model-definition UID (StepMetadata.UID), matching the live WS
// AgentFrame.agent_uid so archetype-scoped widgets resolve identically in replay.
type AgentStepDTO struct {
	ID         uint64                 `json:"id"`
	AgentUID   string                 `json:"agent_uid,omitempty"`
	Position   *PointDTO              `json:"position,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	State      string                 `json:"state,omitempty"`
	// Decision mirrors the live WS AgentFrame.decision: the LLM decision summary
	// applied on this step (set only on decision steps).
	Decision string `json:"decision,omitempty"`
}

// ShapeStepDTO is one shape instance's state within a step frame.
type ShapeStepDTO struct {
	ID         uint64                 `json:"id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// StepDTO is the full state of one simulation step: every agent plus the
// environment's global properties and the step's statistics.
type StepDTO struct {
	Step             uint64                 `json:"step"`
	Agents           []AgentStepDTO         `json:"agents"`
	Shapes           []ShapeStepDTO         `json:"shapes,omitempty"`
	GlobalProperties map[string]interface{} `json:"global_properties,omitempty"`
	Statistics       interface{}            `json:"statistics,omitempty"`
}

// StepsPageResponse is the paged response of GET /:id/steps (replay/scrub).
type StepsPageResponse struct {
	Items   []StepDTO `json:"items"`
	HasMore bool      `json:"has_more"`
}

// ===================== Execution run DTO =====================

// ExecutionRunDTO mirrors data/simulations.ExecutionRun (json snake_case): the
// audit record of one engine run attempt.
type ExecutionRunDTO struct {
	Seq            uint64     `json:"seq"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	StepsCompleted uint64     `json:"steps_completed"`
	Status         string     `json:"status"`
	Error          string     `json:"error,omitempty"`
	// ActiveDurationMs is the run's active (pause-excluding) duration in
	// milliseconds; 0 until the run finalizes.
	ActiveDurationMs uint64 `json:"active_duration_ms"`
	// TerminationReason says why a FINISHED run ended (U2): "condition_met"
	// (the simulation's termination condition fired) or "max_steps" (configured
	// step count reached). Empty for stopped/failed/legacy runs.
	TerminationReason string `json:"termination_reason,omitempty"`
}

// ===================== Mappers =====================

// executionRunToDTO converts one ExecutionRun entity to its DTO.
func executionRunToDTO(r datasims.ExecutionRun) ExecutionRunDTO {
	active := r.ActiveDuration.Milliseconds()
	if active < 0 {
		active = 0
	}
	return ExecutionRunDTO{
		Seq:               r.Seq,
		StartedAt:         r.StartedAt,
		FinishedAt:        r.FinishedAt,
		StepsCompleted:    r.StepsCompleted,
		Status:            string(r.Status),
		Error:             r.Error,
		ActiveDurationMs:  uint64(active),
		TerminationReason: r.TerminationReason,
	}
}

// executionRunsToDTO converts the audit trail, preserving order. Returns nil for
// a nil/empty input (mirrors generationRunsToDTO).
func executionRunsToDTO(runs []datasims.ExecutionRun) []ExecutionRunDTO {
	if runs == nil {
		return nil
	}
	out := make([]ExecutionRunDTO, 0, len(runs))
	for _, r := range runs {
		out = append(out, executionRunToDTO(r))
	}
	return out
}

// stepsToDTOs folds the per-entity timeseries rows of a step page into one
// StepDTO per distinct step number. A SimulationStep is one row per ENTITY
// (agent / shape / environment); a StepDTO aggregates a whole tick:
//
//   - agent rows (Type == MetadataTypeAgent) become AgentStepDTO entries, keyed
//     and sorted by ReferenceID (the runtime instance id);
//   - shape rows (Type == MetadataTypeShape) become ShapeStepDTO entries, sorted
//     by ReferenceID;
//   - the environment row (Type == MetadataTypeEnvironment) supplies
//     GlobalProperties and the per-step accumulated Statistics snapshot;
//   - agent state comes from the typed SimulationStep.State field (the engine
//     writes it there, not into Properties).
//
// Output steps are ordered ascending by step number; the input order within a
// step is preserved for agents of equal ReferenceID stability.
func stepsToDTOs(steps []datasims.SimulationStep) []StepDTO {
	if len(steps) == 0 {
		return nil
	}

	// Group rows by step number, preserving first-seen step ordering.
	type stepGroup struct {
		step   uint64
		agents []AgentStepDTO
		shapes []ShapeStepDTO
		global map[string]interface{}
		stats  interface{}
	}
	groups := make(map[int64]*stepGroup)
	order := make([]int64, 0)

	for _, row := range steps {
		g, ok := groups[row.Metadata.Step]
		if !ok {
			g = &stepGroup{step: uint64(row.Metadata.Step)}
			groups[row.Metadata.Step] = g
			order = append(order, row.Metadata.Step)
		}
		switch row.Metadata.Type {
		case datasims.MetadataTypeAgent:
			g.agents = append(g.agents, agentStepRowToDTO(row))
		case datasims.MetadataTypeShape:
			g.shapes = append(g.shapes, ShapeStepDTO{ID: row.Metadata.ReferenceID, Properties: row.Properties})
		case datasims.MetadataTypeEnvironment:
			if g.global == nil {
				g.global = row.Properties
			}
			// The environment row carries the per-step accumulated statistics
			// snapshot (the same numbers the live WS frame sends), so replay can
			// rebuild the text_log "events" widget faithfully.
			if g.stats == nil && row.Statistics != nil {
				g.stats = row.Statistics
			}
		}
	}

	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	out := make([]StepDTO, 0, len(order))
	for _, s := range order {
		g := groups[s]
		sort.SliceStable(g.agents, func(i, j int) bool { return g.agents[i].ID < g.agents[j].ID })
		sort.SliceStable(g.shapes, func(i, j int) bool { return g.shapes[i].ID < g.shapes[j].ID })
		out = append(out, StepDTO{
			Step:             g.step,
			Agents:           g.agents,
			Shapes:           g.shapes,
			GlobalProperties: g.global,
			Statistics:       g.stats,
		})
	}
	return out
}

// agentStepRowToDTO converts one agent timeseries row to an AgentStepDTO. Agent
// state comes from the typed SimulationStep.State field (the engine writes it
// there via buildStepRows), matching the live WS AgentFrame.state so replay is
// faithful to the live stream.
func agentStepRowToDTO(row datasims.SimulationStep) AgentStepDTO {
	dto := AgentStepDTO{
		ID:         row.Metadata.ReferenceID,
		AgentUID:   row.Metadata.UID,
		Properties: row.Properties,
		State:      row.State,
		Decision:   row.Decision,
	}
	if row.Position != nil {
		p := pointToDTO(*row.Position)
		dto.Position = &p
	}
	return dto
}
