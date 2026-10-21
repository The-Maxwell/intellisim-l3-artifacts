// Package simulations is the MongoDB implementation of the data/simulations repository port.
// Document types never leave this package: the repository maps them to/from the
// storage-agnostic entities at every boundary.
package simulations

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
	mongomodels "intelli.sim.core/data/mongodb/models"
	"intelli.sim.core/data/simulations"
)

// SimulationDocument is the MongoDB representation of a simulations.Simulation.
// The model snapshot is frozen at creation and must not be rewritten per-step.
// Heavy per-step state (detailed agent memory history) lives in simulation_steps.
type SimulationDocument struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Name        string        `bson:"name,omitempty"`
	Description string        `bson:"description,omitempty"`

	ModelID bson.ObjectID `bson:"model_id"`

	// ModelSnapshot is the frozen model definition taken at creation. Immutable
	// after first persist — never rewritten on per-step updates.
	ModelSnapshot *mongomodels.ModelDocument `bson:"model_snapshot,omitempty"`

	Seed  uint64 `bson:"seed"`
	Steps uint64 `bson:"steps"`

	// Termination is the optional early-stop condition (U2), reusing the models
	// package's condition encoding. Absent = run to Steps.
	Termination *mongomodels.ConditionDocument `bson:"termination,omitempty"`

	// LLMGenerationTargetOverride / LLMExecutionTargetOverride are the optional
	// simulation-level LLM target overrides for the generation and execution phases.
	LLMGenerationTargetOverride *mongocommon.LLMTargetDocument `bson:"llm_generation_target_override,omitempty"`
	LLMExecutionTargetOverride  *mongocommon.LLMTargetDocument `bson:"llm_execution_target_override,omitempty"`

	Agents      []SimulationAgentDocument      `bson:"agents"`
	Environment *SimulationEnvironmentDocument `bson:"environment,omitempty"`

	// FinalAgents is the runtime agent state captured at run finalize
	// (position/properties/state/memory as of the last step). Distinct from
	// Agents (the initial generated config); nil until a run finishes.
	FinalAgents []SimulationAgentDocument `bson:"final_agents,omitempty"`

	Widgets []mongocommon.WidgetConfigDocument `bson:"widgets,omitempty"`

	Status        simulations.SimulationStatus     `bson:"status"`
	StatusText    string                           `bson:"status_text,omitempty"`
	StatusHistory []SimulationStatusChangeDocument `bson:"status_history,omitempty"`

	GenerationRuns []GenerationRunDocument `bson:"generation_runs,omitempty"`

	// ExecutionRuns is the audit trail of every engine run, oldest first.
	ExecutionRuns []ExecutionRunDocument `bson:"execution_runs,omitempty"`
	// Statistics holds the final computed statistics of the most recent run;
	// nil until a run completes.
	Statistics *SimulationStatisticsDocument `bson:"statistics,omitempty"`

	TokenUsage *SimulationTokenUsageDocument `bson:"token_usage,omitempty"`

	CurrentStep uint64 `bson:"current_step"`

	CreatedAt  time.Time  `bson:"created_at"`
	CreatedBy  string     `bson:"created_by"`
	UpdatedAt  time.Time  `bson:"updated_at"`
	UpdatedBy  string     `bson:"updated_by"`
	StartedAt  *time.Time `bson:"started_at,omitempty"`
	FinishedAt *time.Time `bson:"finished_at,omitempty"`
}

// SimulationStatusChangeDocument records a single status transition (who/when).
type SimulationStatusChangeDocument struct {
	Status simulations.SimulationStatus `bson:"status"`
	At     time.Time                    `bson:"at"`
	By     string                       `bson:"by"`
}

// ExecutionRunDocument is the MongoDB representation of one engine run, symmetric
// to GenerationRunDocument. Seq is 1-based and matches index Seq-1 in
// SimulationTokenUsageDocument.ByRun.
type ExecutionRunDocument struct {
	Seq            uint64                         `bson:"seq"`
	StartedAt      time.Time                      `bson:"started_at"`
	FinishedAt     *time.Time                     `bson:"finished_at,omitempty"`
	StepsCompleted uint64                         `bson:"steps_completed"`
	Status         simulations.ExecutionRunStatus `bson:"status"`
	Error          string                         `bson:"error,omitempty"`
	// ActiveDurationMs is the run's active (pause-excluding) duration in
	// milliseconds; 0 for runs that never finalized or predate the field.
	ActiveDurationMs int64 `bson:"active_duration_ms,omitempty"`
	// TerminationReason says why a finished run ended (U2): condition_met |
	// max_steps. Empty for stopped/failed/legacy runs.
	TerminationReason string `bson:"termination_reason,omitempty"`
}

// SimulationAgentDocument is the runtime state of one agent instance.
// Static attributes (Type/MoveType/Description/rules) are resolved from
// ModelSnapshot via AgentUID; only live/mutable state is stored here.
type SimulationAgentDocument struct {
	ID       uint64 `bson:"id"`        // runtime instance id, unique within simulation agents
	AgentUID string `bson:"agent_uid"` // model Agent.UID this instance was created from
	Name     string `bson:"name"`

	Position   *mongocommon.PointDocument `bson:"position,omitempty"`
	Properties map[string]interface{}     `bson:"properties"`
	State      string                     `bson:"state,omitempty"`
	Memory     []AgentMemoryDocument      `bson:"memory,omitempty"`
}

// AgentMemoryDocument is one persisted event in an agent's runtime memory log.
type AgentMemoryDocument struct {
	Step      uint64                      `bson:"step"`
	Type      simulations.AgentMemoryType `bson:"type"`
	Summary   string                      `bson:"summary"`
	Details   interface{}                 `bson:"details,omitempty"` // polymorphic; decodes as bson.D on read
	Timestamp time.Time                   `bson:"timestamp"`
}

// SimulationEnvironmentDocument is the concrete runtime environment state.
// Width/Height/Torus/GlobalProperties/Shapes capture the current live values.
type SimulationEnvironmentDocument struct {
	Width            uint64                    `bson:"width"`
	Height           uint64                    `bson:"height"`
	Torus            bool                      `bson:"torus"`
	CollisionMode    models.CollisionMode                  `bson:"collision_mode,omitempty"`
	CollisionConfig  *mongomodels.CollisionConfigDocument  `bson:"collision_config,omitempty"`
	GlobalProperties map[string]interface{}    `bson:"global_properties,omitempty"`
	Shapes           []SimulationShapeDocument `bson:"shapes,omitempty"`
}

// SimulationShapeDocument is a placed shape instance with fully resolved geometry.
type SimulationShapeDocument struct {
	ID       uint64           `bson:"id"`        // runtime instance id, unique within simulation shapes
	ShapeUID string           `bson:"shape_uid"` // model Shape.UID this instance was created from
	Name     string           `bson:"name"`
	Type     models.ShapeType `bson:"type"`
	Layer    uint             `bson:"layer,omitempty"`
	Visible  bool             `bson:"visible"`

	Properties map[string]interface{} `bson:"properties,omitempty"`

	Position *mongocommon.PointDocument  `bson:"position"`
	Width    *uint64                     `bson:"width,omitempty"`
	Height   *uint64                     `bson:"height,omitempty"`
	Radius   *float64                    `bson:"radius,omitempty"`
	Points   []mongocommon.PointDocument `bson:"points,omitempty"`

	Thickness   *uint `bson:"thickness,omitempty"`
	Fill        *bool `bson:"fill,omitempty"`
	Stroke      *bool `bson:"stroke,omitempty"`
	StrokeWidth *uint `bson:"stroke_width,omitempty"`
}
