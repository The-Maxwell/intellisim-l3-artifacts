package simulations

import (
	"time"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
)

// SimulationStatus is the lifecycle state of a simulation.
type SimulationStatus string

const (
	// SimulationStatusGenerating means the initial state is being generated
	// from the model (seed pipeline running).
	SimulationStatusGenerating SimulationStatus = "generating"

	// SimulationStatusReady means the initial state is complete and the
	// simulation is editable (agents/environment may be adjusted before start).
	SimulationStatusReady SimulationStatus = "ready"

	// SimulationStatusGenerationFailed means the generation pipeline did not
	// complete successfully; the simulation may be partially populated.
	SimulationStatusGenerationFailed SimulationStatus = "generation_failed"

	// SimulationStatusQueued means the simulation has been submitted and is
	// waiting for an execution slot.
	// reserved — not produced today
	SimulationStatusQueued SimulationStatus = "queued"

	// SimulationStatusRunning means the simulation engine is actively
	// processing ticks.
	SimulationStatusRunning SimulationStatus = "running"

	// SimulationStatusPaused means execution was paused mid-run; it can be
	// resumed without regeneration.
	SimulationStatusPaused SimulationStatus = "paused"

	// SimulationStatusFinished means the simulation ran to its configured
	// step count and completed normally.
	SimulationStatusFinished SimulationStatus = "finished"

	// SimulationStatusFailed means the simulation engine stopped due to an
	// unrecoverable error.
	SimulationStatusFailed SimulationStatus = "failed"

	// SimulationStatusStopped means the user terminated the run before it
	// reached the configured step count.
	SimulationStatusStopped SimulationStatus = "stopped"
)

// Editable reports whether the simulation may be modified, regenerated, or
// used as the source for a duplicate. Only ready and generation_failed
// simulations are editable; all other statuses lock the record.
func (s SimulationStatus) Editable() bool {
	return s == SimulationStatusReady || s == SimulationStatusGenerationFailed
}

// Streamable reports whether a live run worth streaming exists for this status:
// a WebSocket stream room is created on run start and torn down on completion, so
// only running and paused simulations have an open room to join. Terminal /
// pre-run statuses (ready, finished, failed, stopped, queued, generating) have no
// room, and connecting to them would otherwise orphan a freshly-created room.
func (s SimulationStatus) Streamable() bool {
	return s == SimulationStatusRunning || s == SimulationStatusPaused
}

// SimulationStatusChange records a single status transition (who/when).
type SimulationStatusChange struct {
	Status SimulationStatus
	At     time.Time
	By     string
}

// Simulation is one instance of a model: a generated initial state that is
// editable until the run starts. The model snapshot is frozen at creation and
// refreshed only on a full regeneration; partial regenerations leave it as-is.
//
// ID is a hex string (empty before first persist). All UID cross-references
// within Agents/Environment/Shapes resolve against ModelSnapshot.
type Simulation struct {
	ID          string
	Name        string
	Description string

	ModelID string
	// ModelSnapshot is the frozen model definition taken at create time. It is
	// the authoritative source for all static definitions (agent archetypes,
	// shape templates, rules, interactions). Excluded from the JSON API
	// response — the frontend fetches the model separately.
	ModelSnapshot *models.Model

	// Seed is the random seed used during the generation pipeline. The same
	// seed combined with the same model produces an identical initial world
	// (deterministic random generation).
	Seed uint64
	// Steps is the configured run length (total tick count).
	Steps uint64
	// Termination is an optional early-stop condition (U2) evaluated once per
	// step after the step's mutations commit; when it holds, the run finishes
	// early (status finished, ExecutionRun.TerminationReason = condition_met).
	// Restricted to the population/step/global_property/composite condition
	// subset. Stamped from Model.SimulationConfig.Termination at create time;
	// editable while the simulation is editable. Nil = run to Steps.
	Termination *models.Condition

	// LLMGenerationTargetOverride is an optional simulation-level LLM target that
	// takes precedence over the model's SimulationConfig.LLMSimulationGenerationConfig.Target
	// during the generation (env/agent seeding) phase. Validated against the
	// simulation_generation capability. Set at create / regenerate time.
	LLMGenerationTargetOverride *common.LLMTarget

	// LLMExecutionTargetOverride is an optional simulation-level LLM target that
	// takes precedence over the model's SimulationConfig.LLMSimulationExecutionConfig.Target
	// for all Level-2 run calls without a more specific agent-level override.
	// Validated against the simulation_execution capability. Set at create / edit /
	// run-start time.
	LLMExecutionTargetOverride *common.LLMTarget

	// Agents is the INITIAL generated (then optionally user-edited) roster. The
	// run engine operates on a deep copy of it (NewRuntimeState → copySimulationAgent),
	// so it is never rewritten during or after a run: it always reflects the
	// initial world. This is what Duplicate copies and what GET /:id returns as
	// the config/editor view. The final agent world lives in FinalAgents and the
	// simulation_steps timeseries.
	Agents []SimulationAgent
	// FinalAgents is the runtime agent state captured at run finalize —
	// position/properties/state/memory as of the last executed step. It is
	// DISTINCT from Agents (see above); nil for sims that never finished a run or
	// finished before this field existed. Consumed by GetAgent (and the replay
	// agent inspector) to show the real final world post-run.
	FinalAgents []SimulationAgent
	// Environment is nil only for non-spatial models that have no global
	// properties defined. Like Agents, it stays the INITIAL config: the run
	// engine deep-copies it (NewRuntimeState → cloneEnvironment) so run-time
	// mutations to global/shape properties never touch the stored value. Final
	// environment state is recoverable from the simulation_steps timeseries (the
	// per-step environment + shape rows).
	Environment *SimulationEnvironment

	// Widgets holds the dashboard widget layout for this simulation instance;
	// initialised as a copy of the model's widget template at create time.
	Widgets []common.WidgetConfig

	Status        SimulationStatus
	StatusText    string
	StatusHistory []SimulationStatusChange
	// GenerationRuns is the audit trail of every generation attempt, oldest
	// first. Index Seq-1 of each entry corresponds to the same index in
	// TokenUsage.ByGeneration.
	GenerationRuns []GenerationRun

	// ExecutionRuns is the audit trail of every engine run, oldest first.
	ExecutionRuns []ExecutionRun
	// Statistics holds the final computed statistics of the most recent run;
	// nil until a run completes. Live stats are held in memory during a run.
	Statistics *SimulationStatistics

	TokenUsage *SimulationTokenUsage

	// CurrentStep is the runtime progress counter; it is not modified during
	// the create/edit phase.
	CurrentStep uint64

	CreatedAt  time.Time
	CreatedBy  string
	UpdatedAt  time.Time
	UpdatedBy  string
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// SimulationAgent is one live agent instance in the simulation's initial
// (and runtime) state.
type SimulationAgent struct {
	// ID is the numeric instance id, unique within the simulation's agent
	// slice.
	ID uint64
	// AgentUID is the model Agent.UID this instance was created from.
	AgentUID string
	Name     string
	// Position is set only for spatial models.
	Position   *common.Point
	Properties map[string]interface{}
	// State is the state-machine state name; one of the model Agent.States.
	State string
	// Memory is the runtime event log for this agent instance; populated
	// during execution, not during the create/edit phase.
	Memory []AgentMemory
}

// SimulationEnvironment is the placed, concrete environment state for this
// simulation instance.
type SimulationEnvironment struct {
	Width  uint64
	Height uint64
	Torus  bool
	// CollisionConfig tunes block-mode collision response (U5; copied from the
	// model environment at generation like CollisionMode). Nil = engine defaults.
	CollisionConfig *models.CollisionConfig
	// CollisionMode is the world's collision-response mode (copied from the model
	// Environment at generation). Empty ⇒ no physical blocking.
	CollisionMode models.CollisionMode
	// GlobalProperties holds the runtime values of the model
	// Environment.GlobalProperties. Keyed by property UID.
	GlobalProperties map[string]interface{}
	Shapes           []SimulationShape
}

// SimulationShape is a concrete shape instance with fully resolved geometry
// (all ScalarSpec ranges collapsed to single values, all placements evaluated
// to absolute coordinates).
type SimulationShape struct {
	// ID is the numeric instance id, unique within the simulation's shape
	// slice.
	ID uint64
	// ShapeUID is the model Shape.UID this instance was created from.
	ShapeUID string
	Name     string
	Type     models.ShapeType
	Layer    uint
	Visible  bool
	// Properties holds the runtime values of the shape's AgentProperty list.
	// Keyed by property UID.
	Properties map[string]interface{}
	// Position is the absolute anchor coordinate (top-left for rectangles,
	// centre for circles, anchor origin for polygons/lines).
	Position common.Point
	Width    *uint64
	Height   *uint64
	Radius   *float64
	// Points are anchor-relative vertices for polygon/line shapes.
	Points      []common.Point
	Thickness   *uint
	Fill        *bool
	Stroke      *bool
	StrokeWidth *uint
}

// AgentMemoryType classifies an agent memory entry. Per the article the engine
// records decision (rule-based or LLM-driven choice), interaction (agent-agent,
// agent-shape, or agent-global contact), rule (which rule fired and why), and
// llm_response (prior LLM output kept for context control). movement and
// property are implementation extensions not named in the article's taxonomy.
type AgentMemoryType string

const (
	// AgentMemoryMovement records a positional movement event in the agent's
	// memory log.
	AgentMemoryMovement AgentMemoryType = "movement"

	// AgentMemoryDecision records a decision event (rule-based or LLM-driven)
	// in the agent's memory log.
	AgentMemoryDecision AgentMemoryType = "decision"

	// AgentMemoryInteraction records an agent-agent, agent-shape, or
	// agent-global interaction event in the agent's memory log.
	AgentMemoryInteraction AgentMemoryType = "interaction"

	// AgentMemoryProperty records a property-change event in the agent's
	// memory log.
	AgentMemoryProperty AgentMemoryType = "property"

	// AgentMemoryRule records which rule fired and the reason it was triggered,
	// per the article's rule-execution memory taxonomy.
	AgentMemoryRule AgentMemoryType = "rule"

	// AgentMemoryLLM records a prior LLM output kept in the agent's memory for
	// context control in subsequent prompts, per the article's LLM-response
	// memory taxonomy.
	AgentMemoryLLM AgentMemoryType = "llm_response"
)

// AgentMemory is one persisted event in an agent's runtime memory log.
type AgentMemory struct {
	Step    uint64
	Type    AgentMemoryType
	Summary string
	// Details is a polymorphic event payload; normalised to plain maps/slices
	// by the MongoDB mapper on read.
	Details   interface{}
	Timestamp time.Time
}
