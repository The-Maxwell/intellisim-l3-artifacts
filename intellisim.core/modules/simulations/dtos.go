package simulations

// LLMTargetDTO is the simulations-module API form of common.LLMTarget. Mirrors
// the models module's ExecLLMTargetDTO shape field-for-field; defined here as
// the simulations module's own type (deliberate duplication per module rules).
type LLMTargetDTO struct {
	ProviderUID string                    `json:"provider_uid" validate:"omitempty,max=100"`
	ModelUID    string                    `json:"model_uid" validate:"omitempty,max=100"`
	Effort      string                    `json:"effort,omitempty" validate:"omitempty,oneof=minimal low medium high max"`
	Generation  map[string]ConfigValueDTO `json:"generation,omitempty"`
	TimeoutMs   *uint64                   `json:"timeout_ms,omitempty" validate:"omitempty,min=1"`
}

// ConfigValueDTO is the JSON form of a single dynamic config value (exactly one
// field set). Mirrors common.ConfigValue field-for-field.
type ConfigValueDTO struct {
	Double *float64 `json:"double,omitempty"`
	Int    *int64   `json:"int,omitempty"`
	Uint   *uint64  `json:"uint,omitempty"`
	Bool   *bool    `json:"bool,omitempty"`
	String *string  `json:"string,omitempty"`
	List   []string `json:"list,omitempty"`
}

// PointDTO mirrors common.Point (uint64 coordinates, json x/y). Matches the
// models module PointDTO shape exactly.
type PointDTO struct {
	X uint64 `json:"x"`
	Y uint64 `json:"y"`
}

// AgentMemoryDTO is the compact, API-facing representation of one
// data/simulations.AgentMemory entry. Details and Timestamp are excluded to
// keep the payload small; consumers that need raw detail should query the
// engine's persisted step timeseries instead.
type AgentMemoryDTO struct {
	Step    uint64 `json:"step"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

// SimulationAgentDTO mirrors data/simulations.SimulationAgent for API exchange.
type SimulationAgentDTO struct {
	ID         uint64                 `json:"id"`
	AgentUID   string                 `json:"agent_uid"`
	Name       string                 `json:"name"`
	Position   *PointDTO              `json:"position,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	State      string                 `json:"state"`
	// Memory is the compact runtime event log for this agent instance; omitted
	// when the agent has no recorded memory (create/edit phase or no-LLM run).
	Memory []AgentMemoryDTO `json:"memory,omitempty"`
}

// SimulationEnvironmentDTO mirrors data/simulations.SimulationEnvironment.
type SimulationEnvironmentDTO struct {
	Width            uint64                 `json:"width"`
	Height           uint64                 `json:"height"`
	Torus            bool                   `json:"torus"`
	CollisionMode    string                 `json:"collision_mode,omitempty"`
	CollisionConfig  *CollisionConfigDTO    `json:"collision_config,omitempty"`
	GlobalProperties map[string]interface{} `json:"global_properties,omitempty"`
	Shapes           []SimulationShapeDTO   `json:"shapes,omitempty"`
}

// CollisionConfigDTO mirrors models.CollisionConfig (U5): block-mode collision
// tuning. Empty stuck_policy = revert (legacy). Module-local type like
// LLMTargetDTO.
type CollisionConfigDTO struct {
	MaxIterations uint   `json:"max_iterations,omitempty" validate:"omitempty,max=64"`
	StuckPolicy   string `json:"stuck_policy,omitempty" validate:"omitempty,oneof=best_effort"`
}

// SimulationShapeDTO mirrors data/simulations.SimulationShape with all geometry
// fields. Position is a value (not pointer) matching the entity.
type SimulationShapeDTO struct {
	ID          uint64                 `json:"id"`
	ShapeUID    string                 `json:"shape_uid"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Layer       uint                   `json:"layer"`
	Visible     bool                   `json:"visible"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Position    PointDTO               `json:"position"`
	Width       *uint64                `json:"width,omitempty"`
	Height      *uint64                `json:"height,omitempty"`
	Radius      *float64               `json:"radius,omitempty"`
	Points      []PointDTO             `json:"points,omitempty"`
	Thickness   *uint                  `json:"thickness,omitempty"`
	Fill        *bool                  `json:"fill,omitempty"`
	Stroke      *bool                  `json:"stroke,omitempty"`
	StrokeWidth *uint                  `json:"stroke_width,omitempty"`
}

// WidgetConfigDTO mirrors common.WidgetConfig field-for-field (json tags
// identical to the entity's). Each module owns its DTO set; this duplication
// across modules is deliberate.
type WidgetConfigDTO struct {
	ID       string            `json:"id" validate:"omitempty,max=100"`
	Type     string            `json:"type" validate:"omitempty,oneof=monitor line_chart bar_chart pie_chart histogram agent_table text_log"`
	Title    string            `json:"title" validate:"omitempty,max=200"`
	Source   *WidgetSourceDTO  `json:"source,omitempty"`
	Series   []WidgetSeriesDTO `json:"series,omitempty" validate:"dive"`
	Columns  []string          `json:"columns,omitempty"`
	Buckets  *uint             `json:"buckets,omitempty"`
	LogKinds []string          `json:"log_kinds,omitempty" validate:"dive,oneof=events state_transitions llm_decisions"`
	Position WidgetPositionDTO `json:"position"`
}

// WidgetSourceDTO mirrors common.WidgetSource.
type WidgetSourceDTO struct {
	Type        string   `json:"type" validate:"omitempty,oneof=agent_property global_property population state_distribution sim_metric"`
	PropertyUID string   `json:"property_uid,omitempty" validate:"omitempty,max=100"`
	AgentUIDs   []string `json:"agent_uids,omitempty"`
	Aggregation string   `json:"aggregation,omitempty" validate:"omitempty,oneof=avg sum min max count median"`
	Metric      string   `json:"metric,omitempty" validate:"omitempty,oneof=total_agents agents_spawned agents_removed interactions events rule_executions movement_distance"`
}

// WidgetSeriesDTO mirrors common.WidgetSeries.
type WidgetSeriesDTO struct {
	Label  string          `json:"label" validate:"omitempty,max=200"`
	Color  string          `json:"color,omitempty" validate:"omitempty,max=20"`
	Source WidgetSourceDTO `json:"source"`
}

// WidgetPositionDTO mirrors common.WidgetPosition.
type WidgetPositionDTO struct {
	X uint `json:"x"`
	Y uint `json:"y"`
	W uint `json:"w"`
	H uint `json:"h"`
}

// --- request DTOs ---

// CreateSimulationRequest creates a simulation from a model. The two optional LLM
// target overrides take precedence over the model's simulation-generation and
// simulation-execution configs respectively (validated against the matching capability).
type CreateSimulationRequest struct {
	ModelID             string        `json:"model_id" validate:"required"`
	Name                string        `json:"name" validate:"omitempty,max=100"`
	Description         string        `json:"description" validate:"omitempty,max=500"`
	Steps               *uint64       `json:"steps,omitempty" validate:"omitempty,min=1,max=100000"`
	Seed                *uint64       `json:"seed,omitempty"`
	LLMGenerationTarget *LLMTargetDTO `json:"llm_generation_target"`
	LLMExecutionTarget  *LLMTargetDTO `json:"llm_execution_target"`
}

// GenerateRequest triggers a (re)generation run scoped to a subset of the
// simulation state. LLMGenerationTarget overrides the generation LLM for this run.
// Seed is optional: omitted, the service rolls a fresh one (every regeneration
// draws a new seed so the stored seed always describes the current world);
// supplied, the service uses it as-is (e.g. to reproduce a known world).
type GenerateRequest struct {
	Scope               GenerationScopeDTO `json:"scope" validate:"required"`
	LLMGenerationTarget *LLMTargetDTO      `json:"llm_generation_target"`
	Seed                *uint64            `json:"seed,omitempty"`
}

// GenerationScopeDTO is the API form of data/simulations.GenerationScope.
type GenerationScopeDTO struct {
	Kind      string   `json:"kind" validate:"required,oneof=full environment agents"`
	AgentUIDs []string `json:"agent_uids" validate:"omitempty,dive,required"`
	Parts     []string `json:"parts" validate:"omitempty,dive,oneof=population properties placement"`
}

// DuplicateRequest optionally overrides the name of the duplicated simulation.
type DuplicateRequest struct {
	Name string `json:"name" validate:"omitempty,max=100"`
}

// TerminationConditionDTO is the API form of the optional run-termination
// condition (U2). It mirrors models.Condition restricted to the termination
// subset — population | step | global_property | composite — so a run can stop
// early (e.g. an evacuation ends when the building empties). Deliberate module-
// local type (like LLMTargetDTO), not a reuse of the models module's ConditionDTO.
type TerminationConditionDTO struct {
	Type          string                    `json:"type" validate:"required,oneof=population step global_property composite"`
	PropertyUID   string                    `json:"property_uid,omitempty" validate:"omitempty,max=100"`
	Operator      string                    `json:"operator,omitempty" validate:"omitempty,oneof=eq neq gt lt gte lte contains in_range in not_in"`
	Value         interface{}               `json:"value,omitempty"`
	MinValue      *float64                  `json:"min_value,omitempty"`
	MaxValue      *float64                  `json:"max_value,omitempty"`
	TargetUID     string                    `json:"target_uid,omitempty" validate:"omitempty,max=100"`
	TargetUIDs    []string                  `json:"target_uids,omitempty" validate:"omitempty,dive,required"`
	LogicOperator string                    `json:"logic_operator,omitempty" validate:"omitempty,oneof=AND OR NOT"`
	SubConditions []TerminationConditionDTO `json:"sub_conditions,omitempty" validate:"omitempty,max=10,dive"`
}

// UpdateSimulationRequest is a full manual replace of all editable fields.
type UpdateSimulationRequest struct {
	Name                string                    `json:"name" validate:"required,max=100"`
	Description         string                    `json:"description" validate:"max=500"`
	Steps               uint64                    `json:"steps" validate:"min=1,max=100000"`
	Seed                uint64                    `json:"seed"` // zero is valid: deterministic seed
	Termination         *TerminationConditionDTO  `json:"termination,omitempty"`
	LLMGenerationTarget *LLMTargetDTO             `json:"llm_generation_target"`
	LLMExecutionTarget  *LLMTargetDTO             `json:"llm_execution_target"`
	Agents              []SimulationAgentDTO      `json:"agents" validate:"dive"`
	Environment         *SimulationEnvironmentDTO `json:"environment"`
	Widgets             []WidgetConfigDTO         `json:"widgets" validate:"dive"`
}
