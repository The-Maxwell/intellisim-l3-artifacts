package models

import (
	"time"

	"intelli.sim.core/data/common"
)

// ModelInputDTO is the editable model shape accepted on PUT (form or JSON) and
// returned on GET. It mirrors the domain models.Model 1:1, with json snake_case
// tags and validate tags for field-shape rules.
type ModelInputDTO struct {
	Name             string              `json:"name" validate:"omitempty,max=120"`
	Description      string              `json:"description" validate:"omitempty,max=2000"`
	IsSpatial        bool                `json:"is_spatial"`
	SpatialCellSize  uint64              `json:"spatial_cell_size"`
	LLMConfig        LLMConfigDTO        `json:"llm_config"`
	SimulationConfig SimulationConfigDTO `json:"simulation_config"`
	Agents           []AgentDTO          `json:"agents" validate:"dive"`
	Environment      *EnvironmentDTO     `json:"environment,omitempty"`
	Rules            []RuleDTO           `json:"rules" validate:"dive"`
	Interactions     []InteractionDTO    `json:"interactions" validate:"dive"`
	Events           []EventDefDTO       `json:"events" validate:"dive"`
	Images           []ModelImageDTO     `json:"images" validate:"dive"`
	Widgets          []WidgetConfigDTO   `json:"widgets,omitempty" validate:"dive"`
}

// --- agents ---

type AgentDTO struct {
	UID            string                 `json:"uid" validate:"omitempty,max=100"`
	Name           string                 `json:"name" validate:"omitempty,max=200"`
	Description    string                 `json:"description" validate:"omitempty,max=2000"`
	Type           string                 `json:"type" validate:"omitempty,oneof=agent archetype"`
	MoveType       string                 `json:"move_type" validate:"omitempty,oneof=static movable"`
	Properties     []AgentPropertyDTO     `json:"properties" validate:"dive"`
	States         []string               `json:"states"`
	Initialization AgentInitializationDTO `json:"initialization"`
	LLMConfig      *LLMAgentConfigDTO     `json:"llm_config,omitempty"`
	Footprint      *AgentFootprintDTO     `json:"footprint,omitempty"`
	Display        *DisplayDTO            `json:"display,omitempty"`
	Collidable     *bool                  `json:"collidable,omitempty"`
	Locked         bool                   `json:"locked"`
	CreatedAt      time.Time              `json:"created_at,omitempty"`
	UpdatedAt      time.Time              `json:"updated_at,omitempty"`
}

// AgentFootprintDTO is the agent's simulation collision body (centered on the agent
// position). Dimensions are fixed per archetype.
type AgentFootprintDTO struct {
	Kind   string     `json:"kind" validate:"omitempty,oneof=point circle rectangle polygon"`
	Radius *float64   `json:"radius,omitempty"`
	Width  *uint64    `json:"width,omitempty"`
	Height *uint64    `json:"height,omitempty"`
	Points []PointDTO `json:"points,omitempty"`
}

// DisplayDTO is the VISUAL representation shared by agents and shapes (image / figure).
type DisplayDTO struct {
	Kind   string           `json:"kind" validate:"omitempty,oneof=auto image figure"`
	Image  *DisplayImageDTO `json:"image,omitempty"`
	Figure []PointDTO       `json:"figure,omitempty"`
	Scale  *float64         `json:"scale,omitempty"`
}

// DisplayImageDTO is a base64 sprite. On save the backend compresses oversized images and
// rewrites Data/MimeType/Width/Height with the compressed result.
type DisplayImageDTO struct {
	Data     string `json:"data"`
	MimeType string `json:"mime_type,omitempty"`
	Width    uint64 `json:"width,omitempty"`
	Height   uint64 `json:"height,omitempty"`
}

type AgentPropertyDTO struct {
	UID              string                 `json:"uid" validate:"omitempty,max=100"`
	Name             string                 `json:"name" validate:"omitempty,max=200"`
	Type             string                 `json:"type" validate:"omitempty,oneof=string number enum boolean color"`
	Description      string                 `json:"description" validate:"omitempty,max=2000"`
	MinValue         *float64               `json:"min_value,omitempty"`
	MaxValue         *float64               `json:"max_value,omitempty"`
	DefaultValue     interface{}            `json:"default_value,omitempty"`
	AllowedValues    []interface{}          `json:"allowed_values,omitempty"`
	Immutable        bool                   `json:"immutable"`
	LLMPromptContext string                 `json:"llm_prompt_context,omitempty"`
	Generation       *PropertyGenerationDTO `json:"generation,omitempty"`
}

// PropertyGenerationDTO mirrors models.PropertyGeneration: how the simulation
// generator produces the property's initial value at t=0.
type PropertyGenerationDTO struct {
	Mode          string            `json:"mode" validate:"omitempty,oneof=default random llm"`
	Distribution  string            `json:"distribution,omitempty" validate:"omitempty,oneof=uniform normal choice"`
	Mean          *float64          `json:"mean,omitempty"`
	StdDev        *float64          `json:"std_dev,omitempty"`
	Min           *float64          `json:"min,omitempty"`
	Max           *float64          `json:"max,omitempty"`
	Choices       []ChoiceWeightDTO `json:"choices,omitempty" validate:"dive"`
	PromptContext string            `json:"prompt_context,omitempty" validate:"omitempty,max=2000"`
}

// ChoiceWeightDTO mirrors models.ChoiceWeight.
type ChoiceWeightDTO struct {
	Value  interface{} `json:"value"`
	Weight float64     `json:"weight"`
}

type AgentInitializationDTO struct {
	PopulationCount uint64        `json:"population_count"`
	MinPopulation   *uint64       `json:"min_population,omitempty"`
	MaxPopulation   *uint64       `json:"max_population,omitempty"`
	InitialState    string        `json:"initial_state,omitempty"`
	Placement       *PlacementDTO `json:"placement,omitempty"`
	CountMode       string        `json:"count_mode,omitempty" validate:"omitempty,oneof=fixed random_range llm"`
}

type PlacementDTO struct {
	Type          string `json:"type" validate:"omitempty,oneof=random uniform clustered at_shape manual llm"`
	ShapeUID      string `json:"shape_uid,omitempty"`
	PromptContext string `json:"prompt_context,omitempty"`

	Clusters      *uint64 `json:"clusters,omitempty"`       // clustered: number of centres
	ClusterRadius *uint64 `json:"cluster_radius,omitempty"` // clustered: gaussian spread
}

// --- environment + shapes ---

type EnvironmentDTO struct {
	UID              string             `json:"uid" validate:"omitempty,max=100"`
	Width            uint64             `json:"width"`
	Height           uint64             `json:"height"`
	Torus            bool               `json:"torus"`
	Shapes           []ShapeDTO         `json:"shapes" validate:"dive"`
	GlobalProperties []AgentPropertyDTO `json:"global_properties" validate:"dive"`
	CollisionMode    string             `json:"collision_mode,omitempty" validate:"omitempty,oneof=block"`
	// CollisionConfig tunes block-mode collision response (U5); absent = engine
	// defaults (4 iterations, revert). Copied onto simulations at generation.
	CollisionConfig  *CollisionConfigDTO `json:"collision_config,omitempty"`
	LLMPromptContext string              `json:"llm_prompt_context,omitempty"`
}

// CollisionConfigDTO mirrors models.CollisionConfig (U5). Empty stuck_policy =
// revert (legacy behavior).
type CollisionConfigDTO struct {
	MaxIterations uint   `json:"max_iterations,omitempty" validate:"omitempty,max=64"`
	StuckPolicy   string `json:"stuck_policy,omitempty" validate:"omitempty,oneof=best_effort"`
}

type ShapeDTO struct {
	UID            string                 `json:"uid" validate:"omitempty,max=100"`
	Name           string                 `json:"name" validate:"omitempty,max=200"`
	Description    string                 `json:"description" validate:"omitempty,max=2000"`
	Type           string                 `json:"type" validate:"omitempty,oneof=rectangle circle line polygon"`
	Layer          uint                   `json:"layer"`
	Visible        bool                   `json:"visible"`
	Properties     []AgentPropertyDTO     `json:"properties" validate:"dive"`
	Initialization AgentInitializationDTO `json:"initialization"`
	Geometry       *ShapeGeometryDTO      `json:"geometry,omitempty"`
	Display        *DisplayDTO            `json:"display,omitempty"`
	LLMConfig      *LLMShapeConfigDTO     `json:"llm_config,omitempty"`
	Locked         bool                   `json:"locked"`
}

type ShapeGeometryDTO struct {
	Position    *PointDTO      `json:"position,omitempty"`
	Width       *ScalarSpecDTO `json:"width,omitempty"`
	Height      *ScalarSpecDTO `json:"height,omitempty"`
	Radius      *ScalarSpecDTO `json:"radius,omitempty"`
	Points      []PointDTO     `json:"points,omitempty"`
	Thickness   *uint          `json:"thickness,omitempty"`
	Fill        *bool          `json:"fill,omitempty"`
	Stroke      *bool          `json:"stroke,omitempty"`
	StrokeWidth *uint          `json:"stroke_width,omitempty"`
}

type ScalarSpecDTO struct {
	Value *uint64 `json:"value,omitempty"`
	Min   *uint64 `json:"min,omitempty"`
	Max   *uint64 `json:"max,omitempty"`
}

type PointDTO struct {
	X uint64 `json:"x"`
	Y uint64 `json:"y"`
}

// --- rules ---

type RuleDTO struct {
	ID            string         `json:"id" validate:"omitempty,max=100"`
	Name          string         `json:"name" validate:"omitempty,max=200"`
	Description   string         `json:"description" validate:"omitempty,max=2000"`
	Priority      int            `json:"priority"`
	AppliesTo     []string       `json:"applies_to"`
	Conditions    []ConditionDTO `json:"conditions" validate:"dive"`
	Actions       []ActionDTO    `json:"actions" validate:"dive"`
	ExecuteOnce   bool           `json:"execute_once"`
	CooldownSteps int            `json:"cooldown_steps"`
	Enabled       bool           `json:"enabled"`
	Locked        bool           `json:"locked"`
}

type ConditionDTO struct {
	Type          string                 `json:"type" validate:"omitempty,oneof=global_property property position proximity step composite count barrier state distance decision random memory event custom population"`
	PropertyUID   string                 `json:"property_uid,omitempty"`
	Operator      string                 `json:"operator,omitempty" validate:"omitempty,oneof=eq neq gt lt gte lte contains in_range in not_in"`
	Value         interface{}            `json:"value,omitempty"`
	MinValue      *float64               `json:"min_value,omitempty"`
	MaxValue      *float64               `json:"max_value,omitempty"`
	Radius        *float64               `json:"radius,omitempty"`
	TargetUID     string                 `json:"target_uid,omitempty"`
	TargetUIDs    []string               `json:"target_uids,omitempty"`
	LogicOperator string                 `json:"logic_operator,omitempty" validate:"omitempty,oneof=AND OR NOT"`
	SubConditions []ConditionDTO         `json:"sub_conditions,omitempty" validate:"dive"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	Description   string                 `json:"description,omitempty"`
}

type ActionDTO struct {
	Type             string                 `json:"type" validate:"omitempty,oneof=set_property modify_property move move_toward move_away interact emit_event transition call_llm spawn_agent remove_agent transform teleport custom"`
	PropertyUID      string                 `json:"property_uid,omitempty"`
	Value            interface{}            `json:"value,omitempty"`
	Modifier         string                 `json:"modifier,omitempty" validate:"omitempty,oneof=set add subtract multiply divide increment decrement toggle"`
	TargetUID        string                 `json:"target_uid,omitempty"`
	TargetUIDs       []string               `json:"target_uids,omitempty"`
	Speed            *float64               `json:"speed,omitempty"`
	Direction        *float64               `json:"direction,omitempty"`
	EventUID         string                 `json:"event_uid,omitempty"`
	EventData        interface{}            `json:"event_data,omitempty"`
	NewState         string                 `json:"new_state,omitempty"`
	SpawnAgentUID    string                 `json:"spawn_agent_uid,omitempty"`
	SpawnCount       int                    `json:"spawn_count,omitempty"`
	SpawnProperties  map[string]interface{} `json:"spawn_properties,omitempty"`
	SpawnPosition    *SpawnPositionDTO      `json:"spawn_position,omitempty"`
	SpawnInheritFrom []string               `json:"spawn_inherit_from,omitempty"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Description      string                 `json:"description,omitempty"`
	LLMPromptContext string                 `json:"llm_prompt_context,omitempty"`
}

type SpawnPositionDTO struct {
	Type     string   `json:"type" validate:"omitempty,oneof=same random offset nearby at_shape"`
	OffsetX  *float64 `json:"offset_x,omitempty"`
	OffsetY  *float64 `json:"offset_y,omitempty"`
	Radius   *float64 `json:"radius,omitempty"`
	ShapeUID string   `json:"shape_uid,omitempty"`
}

// --- interactions ---

type InteractionDTO struct {
	ID               string         `json:"id" validate:"omitempty,max=100"`
	Name             string         `json:"name" validate:"omitempty,max=200"`
	Description      string         `json:"description" validate:"omitempty,max=2000"`
	Type             string         `json:"type" validate:"omitempty,oneof=agent_agent agent_shape agent_global"`
	InitiatorUIDs    []string       `json:"initiator_uids,omitempty"`
	TargetUIDs       []string       `json:"target_uids,omitempty"`
	Radius           *float64       `json:"radius,omitempty"`
	Conditions       []ConditionDTO `json:"conditions" validate:"dive"`
	InitiatorActions []ActionDTO    `json:"initiator_actions" validate:"dive"`
	TargetActions    []ActionDTO    `json:"target_actions" validate:"dive"`
	Bidirectional    bool           `json:"bidirectional"`
	UseLLM           bool           `json:"use_llm"`
	Enabled          bool           `json:"enabled"`
	Locked           bool           `json:"locked"`
	LLMPromptContext string         `json:"llm_prompt_context,omitempty"`
}

// --- events ---

type EventDefDTO struct {
	UID              string             `json:"uid" validate:"omitempty,max=100"`
	Name             string             `json:"name" validate:"omitempty,max=200"`
	Description      string             `json:"description" validate:"omitempty,max=2000"`
	Scope            string             `json:"scope" validate:"omitempty,oneof=global local targeted"`
	Radius           *float64           `json:"radius,omitempty"`
	Payload          []AgentPropertyDTO `json:"payload" validate:"dive"`
	Producers        []string           `json:"producers" validate:"dive,oneof=agent environment llm"`
	ProducerUIDs     []string           `json:"producer_uids,omitempty"`
	ConsumerUIDs     []string           `json:"consumer_uids,omitempty"`
	TTLSteps         uint               `json:"ttl_steps"`
	LLMPromptContext string             `json:"llm_prompt_context,omitempty"`
	LLMTarget        *ExecLLMTargetDTO  `json:"llm_target,omitempty"`
	Locked           bool               `json:"locked"`
}

// --- llm config / strategy / fallback ---

// LLMConfigDTO is the reusable (target + prompt context) pair used for model
// generation, simulation generation, and (embedded) simulation execution.
type LLMConfigDTO struct {
	Target        ExecLLMTargetDTO `json:"target"`
	PromptContext string           `json:"prompt_context,omitempty"`
}

// SimulationConfigDTO mirrors models.SimulationConfig: the update mode plus the
// simulation-generation and simulation-execution LLM configs.
type SimulationConfigDTO struct {
	UpdateMode                    string                          `json:"update_mode" validate:"omitempty,oneof=sequential synchronous"`
	LLMSimulationGenerationConfig LLMConfigDTO                    `json:"llm_simulation_generation_config"`
	LLMSimulationExecutionConfig  LLMSimulationExecutionConfigDTO `json:"llm_simulation_execution_config"`

	// Termination is the default early-stop condition stamped onto simulations
	// created from this model (U2); validated on the simulation side. Restricted
	// to population | step | global_property | composite.
	Termination *ConditionDTO `json:"termination,omitempty"`
}

type LLMAgentConfigDTO struct {
	Target           ExecLLMTargetDTO             `json:"target"`
	PromptContext    string                       `json:"prompt_context,omitempty"`
	Strategy         LLMAgentStrategyDTO          `json:"strategy"`
	SimulationConfig *LLMSimulationAgentConfigDTO `json:"simulation_config,omitempty"`
}

type LLMShapeConfigDTO struct {
	Target        ExecLLMTargetDTO    `json:"target"`
	PromptContext string              `json:"prompt_context,omitempty"`
	Strategy      LLMShapeStrategyDTO `json:"strategy"`
}

// LLMStrategyDTO is the API shape of a runtime LLM strategy. The UseFor* flags are
// canonical for Type (none→all off, full→all on, decision→decisions only, hybrid→free);
// the backend re-normalizes them on save, so non-canonical input is corrected.
type LLMStrategyDTO struct {
	Type              string `json:"type" validate:"omitempty,oneof=none full decision hybrid"`
	BatchSize         int    `json:"batch_size"`
	UseForMovement    bool   `json:"use_for_movement"`
	UseForInteraction bool   `json:"use_for_interaction"`
	UseForProperties  bool   `json:"use_for_properties"`
	UseForDecisions   bool   `json:"use_for_decisions"`
	DecisionFrequency int    `json:"decision_frequency"`
	// ArchetypeMode + VariationStrength apply only to archetype-type agents (how a
	// single archetype-level LLM result is fanned out to members). Empty mode →
	// identical at runtime; VariationStrength is the per-agent jitter amount (0..1,
	// 0 → engine default).
	ArchetypeMode     string  `json:"archetype_mode,omitempty" validate:"omitempty,oneof=identical distribution"`
	VariationStrength float64 `json:"variation_strength,omitempty" validate:"omitempty,gte=0,lte=1"`
}

type LLMAgentStrategyDTO struct {
	LLMStrategyDTO
}

type LLMShapeStrategyDTO struct {
	LLMStrategyDTO
}

type LLMSimulationAgentConfigDTO struct {
	PerceptionRadius  float64            `json:"perception_radius"`
	InteractionRadius float64            `json:"interaction_radius"`
	MemorySize        int                `json:"memory_size"`
	FallbackPolicy    *FallbackPolicyDTO `json:"fallback_policy,omitempty"`
}

// LLMSimulationExecutionConfigDTO mirrors models.LLMSimulationExecutionConfig: the
// default execution target + prompt context, the default per-agent runtime/spatial
// knobs (embedded), run-engine tuning, and the default strategy.
type LLMSimulationExecutionConfigDTO struct {
	Target        ExecLLMTargetDTO `json:"target"`
	PromptContext string           `json:"prompt_context,omitempty"`
	LLMSimulationAgentConfigDTO
	ParallelWorkers uint           `json:"parallel_workers"`
	RetryTimeoutMs  uint           `json:"retry_timeout_ms"`
	MaxRetries      uint           `json:"max_retries"`
	StreamBatchSize uint           `json:"stream_batch_size"`
	DefaultStrategy LLMStrategyDTO `json:"default_strategy"`
}

type FallbackPolicyDTO struct {
	Steps []FallbackStepDTO `json:"steps" validate:"dive"`
}

type FallbackStepDTO struct {
	Type         string `json:"type" validate:"omitempty,oneof=retry rules_only last_decision skip"`
	MaxRetries   uint   `json:"max_retries"`
	RetryDelayMs uint   `json:"retry_delay_ms"`
}

// --- images ---

type ModelImageDTO struct {
	UID         string `json:"uid,omitempty"`
	Name        string `json:"name,omitempty" validate:"omitempty,max=200"`
	MimeType    string `json:"mime_type,omitempty"`
	Data        string `json:"data,omitempty"`
	Description string `json:"description,omitempty" validate:"omitempty,max=2000"`
}

// --- widgets ---

// WidgetConfigDTO mirrors common.WidgetConfig field-for-field (json tags identical
// to the entity's). Each module owns its DTO set, so this duplication across
// modules is deliberate.
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

// --- top-level responses ---

// ModelResponse is ModelInputDTO + server-owned fields (ids, timestamps, chat, validation).
type ModelResponse struct {
	ID         string           `json:"id"`
	ChatID     string           `json:"chat_id,omitempty"`
	Model      ModelInputDTO    `json:"model"` // the editable body
	Validation ValidationResult `json:"validation"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	CreatedBy  string           `json:"created_by"`
	UpdatedBy  string           `json:"updated_by"`
}

type ModelSummaryResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	IsSpatial       bool      `json:"is_spatial"`
	HasChat         bool      `json:"has_chat"`
	AgentCount      int       `json:"agent_count"`
	SimulationReady bool      `json:"simulation_ready"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedBy       string    `json:"created_by"`
}

type ListModelsResponse struct {
	Models   []ModelSummaryResponse `json:"models"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int                    `json:"total"` // total models matching the filter (across all pages)
	HasMore  bool                   `json:"has_more"`
}

// ModelOptionResponse is the minimal id+name pair for selector UIs.
type ModelOptionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListModelOptionsResponse is the unpaginated id+name list (GET /models/options).
type ListModelOptionsResponse struct {
	Options []ModelOptionResponse `json:"options"`
}

// --- chat sub-resource (model-chat lives in shared chats collections) ---

type ModelChatResponse struct {
	ModelID        string                    `json:"model_id"`
	ChatID         string                    `json:"chat_id"`
	LLMTarget      ExecLLMTargetDTO          `json:"llm_target"`
	ModelAvailable bool                      `json:"model_available"`
	Status         string                    `json:"status"` // idle | thinking
	TokenUsage     *common.TokenUsageSummary `json:"token_usage,omitempty"`
}

type ModelChatMessageResponse struct {
	ID        string             `json:"id"`
	ChatID    string             `json:"chat_id"`
	Role      string             `json:"role"`
	Content   string             `json:"content"`       // TextContent (display message)
	Raw       string             `json:"raw,omitempty"` // RawContent (full spec json) — assistant only
	LLMTarget *ExecLLMTargetDTO  `json:"llm_target,omitempty"`
	Usage     *common.TokenUsage `json:"usage,omitempty"`
	Error     *MessageErrorDTO   `json:"error,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	CreatedBy string             `json:"created_by"`
}

type MessageErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type MessagesPageResponse struct {
	Messages []ModelChatMessageResponse `json:"messages"`
	HasMore  bool                       `json:"has_more"`
}

type SendMessageResponse struct {
	Message    ModelChatMessageResponse `json:"message"`
	ChatStatus string                   `json:"chat_status"`
}

// --- validation ---

type ValidationIssue struct {
	Path     string `json:"path"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // error | warning
}

type ValidationResult struct {
	Errors          []ValidationIssue `json:"errors"`
	Warnings        []ValidationIssue `json:"warnings"`
	SimulationReady bool              `json:"simulation_ready"`
}

// --- SSE payloads ---

const (
	ModelChatStatusIdle     = "idle"
	ModelChatStatusThinking = "thinking"
)

type ChatStatusPayload struct {
	ModelID string `json:"model_id"`
	Status  string `json:"status"`
}

// ModelUpdatedPayload carries the full updated model on the SSE model.updated
// event. The validation result is already inside Model (ModelResponse.Validation),
// so it is not duplicated at the top level.
type ModelUpdatedPayload struct {
	ModelID string        `json:"model_id"`
	Model   ModelResponse `json:"model"`
}

type ChatErrorPayload struct {
	ModelID string `json:"model_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ModelDeletedPayload struct {
	ModelID string `json:"model_id"`
}
