package simulations

import "time"

// --- validation ---

// ValidationIssue is one structural problem or warning found during simulation
// instance validation.
type ValidationIssue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// ValidationResult holds the outcome of validating a simulation instance.
// Task 8 fills in the actual ValidateInstance logic; these types are declared
// here so responses.go and mapper.go compile from Task 7 onward.
type ValidationResult struct {
	Errors     []ValidationIssue `json:"errors"`
	Warnings   []ValidationIssue `json:"warnings"`
	StartReady bool              `json:"start_ready"`
}

// --- status history ---

// StatusChangeDTO mirrors data/simulations.SimulationStatusChange.
type StatusChangeDTO struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
	By     string    `json:"by"`
}

// --- generation runs ---

// GenerationWarningDTO mirrors data/simulations.GenerationWarning.
type GenerationWarningDTO struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// GenerationScopeResponseDTO is the read form of data/simulations.GenerationScope.
type GenerationScopeResponseDTO struct {
	Kind      string   `json:"kind"`
	AgentUIDs []string `json:"agent_uids,omitempty"`
	Parts     []string `json:"parts,omitempty"`
}

// GenerationRunDTO mirrors data/simulations.GenerationRun.
type GenerationRunDTO struct {
	Seq        uint64                     `json:"seq"`
	Seed       uint64                     `json:"seed"`
	Scope      GenerationScopeResponseDTO `json:"scope"`
	Status     string                     `json:"status"`
	StartedAt  time.Time                  `json:"started_at"`
	FinishedAt *time.Time                 `json:"finished_at,omitempty"`
	Error      string                     `json:"error,omitempty"`
	Warnings   []GenerationWarningDTO     `json:"warnings,omitempty"`
	Requests   uint                       `json:"requests"`
}

// --- token usage ---

// TokenUsageDTO mirrors common.TokenUsage (incl. routing-identity fields for the
// by-model breakdown). ProviderUID/ModelUID/Effort are the routing-identity fields
// copied from the call's LLMTarget.
type TokenUsageDTO struct {
	// Routing identity (from embedded LLMTarget — zero for total/by-purpose/by-agent).
	ProviderUID string `json:"provider_uid,omitempty"`
	ModelUID    string `json:"model_uid,omitempty"`
	Effort      string `json:"effort,omitempty"`

	PromptTokens     uint64  `json:"prompt_tokens"`
	CompletionTokens uint64  `json:"completion_tokens"`
	ReasoningTokens  uint64  `json:"reasoning_tokens"`
	TotalTokens      uint64  `json:"total_tokens"`
	Calls            uint64  `json:"calls"`
	LatencyMs        float64 `json:"latency_ms"`
}

// AgentTokenUsageDTO mirrors data/simulations.AgentTokenUsage.
type AgentTokenUsageDTO struct {
	AgentUID string `json:"agent_uid"`
	TokenUsageDTO
}

// PurposeTokenUsageDTO mirrors data/simulations.PurposeTokenUsage.
type PurposeTokenUsageDTO struct {
	Purpose string `json:"purpose"`
	TokenUsageDTO
}

// SimulationTokenUsageDTO mirrors data/simulations.SimulationTokenUsage.
type SimulationTokenUsageDTO struct {
	Total        TokenUsageDTO          `json:"total"`
	ByModel      []TokenUsageDTO        `json:"by_model,omitempty"`
	ByAgent      []AgentTokenUsageDTO   `json:"by_agent,omitempty"`
	ByPurpose    []PurposeTokenUsageDTO `json:"by_purpose,omitempty"`
	ByGeneration []TokenUsageDTO        `json:"by_generation,omitempty"`
	// ByRun is indexed by execution run Seq-1 (ExecutionRun.Seq-1); each entry's
	// routing identity is blanked when that run mixed multiple distinct targets.
	ByRun []TokenUsageDTO `json:"by_run,omitempty"`
}

// --- model snapshot DTOs (trimmed — editor only needs static definitions) ---

// PropertyGenerationDTO mirrors data/models.PropertyGeneration.
type PropertyGenerationDTO struct {
	Mode          string            `json:"mode,omitempty"`
	Distribution  string            `json:"distribution,omitempty"`
	Mean          *float64          `json:"mean,omitempty"`
	StdDev        *float64          `json:"std_dev,omitempty"`
	Min           *float64          `json:"min,omitempty"`
	Max           *float64          `json:"max,omitempty"`
	Choices       []ChoiceWeightDTO `json:"choices,omitempty"`
	PromptContext string            `json:"prompt_context,omitempty"`
}

// ChoiceWeightDTO mirrors data/models.ChoiceWeight.
type ChoiceWeightDTO struct {
	Value  interface{} `json:"value"`
	Weight float64     `json:"weight"`
}

// SnapshotPropertyDTO mirrors data/models.AgentProperty (trimmed to fields the
// sim editor needs).
type SnapshotPropertyDTO struct {
	UID              string                 `json:"uid"`
	Name             string                 `json:"name"`
	Type             string                 `json:"type"`
	Description      string                 `json:"description,omitempty"`
	MinValue         *float64               `json:"min_value,omitempty"`
	MaxValue         *float64               `json:"max_value,omitempty"`
	DefaultValue     interface{}            `json:"default_value,omitempty"`
	AllowedValues    []interface{}          `json:"allowed_values,omitempty"`
	Immutable        bool                   `json:"immutable"`
	LLMPromptContext string                 `json:"llm_prompt_context,omitempty"`
	Generation       *PropertyGenerationDTO `json:"generation,omitempty"`
}

// SnapshotPlacementDTO mirrors data/models.Placement.
type SnapshotPlacementDTO struct {
	Type            string     `json:"type,omitempty"`
	ShapeUID        string     `json:"shape_uid,omitempty"`
	PromptContext   string     `json:"prompt_context,omitempty"`
	Clusters      *uint64 `json:"clusters,omitempty"`
	ClusterRadius *uint64 `json:"cluster_radius,omitempty"`
}

// SnapshotInitializationDTO mirrors data/models.AgentInitialization.
type SnapshotInitializationDTO struct {
	PopulationCount uint64                `json:"population_count"`
	MinPopulation   *uint64               `json:"min_population,omitempty"`
	MaxPopulation   *uint64               `json:"max_population,omitempty"`
	InitialState    string                `json:"initial_state,omitempty"`
	CountMode       string                `json:"count_mode,omitempty"`
	Placement       *SnapshotPlacementDTO `json:"placement,omitempty"`
}

// SnapshotAgentDTO mirrors data/models.Agent (trimmed: no rules/LLMConfig).
type SnapshotAgentDTO struct {
	UID            string                    `json:"uid"`
	Name           string                    `json:"name"`
	Type           string                    `json:"type"`
	MoveType       string                    `json:"move_type"`
	Properties     []SnapshotPropertyDTO     `json:"properties,omitempty"`
	States         []string                  `json:"states,omitempty"`
	Initialization SnapshotInitializationDTO `json:"initialization"`
	// Footprint + Display let the sim editor / runner render the agent's body and visuals.
	Footprint  *SnapshotFootprintDTO `json:"footprint,omitempty"`
	Display    *SnapshotDisplayDTO   `json:"display,omitempty"`
	Collidable *bool                 `json:"collidable,omitempty"`
}

// SnapshotFootprintDTO mirrors data/models.AgentFootprint (the agent's simulation body).
type SnapshotFootprintDTO struct {
	Kind   string     `json:"kind"`
	Radius *float64   `json:"radius,omitempty"`
	Width  *uint64    `json:"width,omitempty"`
	Height *uint64    `json:"height,omitempty"`
	Points []PointDTO `json:"points,omitempty"`
}

// SnapshotDisplayDTO mirrors data/models.Display (visual representation).
type SnapshotDisplayDTO struct {
	Kind   string                   `json:"kind"`
	Image  *SnapshotDisplayImageDTO `json:"image,omitempty"`
	Figure []PointDTO               `json:"figure,omitempty"`
	Scale  *float64                 `json:"scale,omitempty"`
}

// SnapshotDisplayImageDTO mirrors data/models.DisplayImageData (compressed base64 sprite).
type SnapshotDisplayImageDTO struct {
	Data     string `json:"data"`
	MimeType string `json:"mime_type,omitempty"`
	Width    uint64 `json:"width,omitempty"`
	Height   uint64 `json:"height,omitempty"`
}

// ScalarSpecDTO mirrors data/models.ScalarSpec: a fixed value or a [Min..Max]
// random range. Exactly one of (Value) or (Min+Max) is set by the mapper.
type ScalarSpecDTO struct {
	Value *uint64 `json:"value,omitempty"`
	Min   *uint64 `json:"min,omitempty"`
	Max   *uint64 `json:"max,omitempty"`
}

// SnapshotGeometryDTO mirrors data/models.ShapeGeometry losslessly so the
// editor's add-instance palette can use template geometry.
type SnapshotGeometryDTO struct {
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

// SnapshotShapeDTO mirrors data/models.Shape (trimmed: no LLMConfig, no locked).
type SnapshotShapeDTO struct {
	UID            string                    `json:"uid"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description,omitempty"`
	Type           string                    `json:"type"`
	Layer          uint                      `json:"layer"`
	Visible        bool                      `json:"visible"`
	Properties     []SnapshotPropertyDTO     `json:"properties,omitempty"`
	Initialization SnapshotInitializationDTO `json:"initialization"`
	Geometry       *SnapshotGeometryDTO      `json:"geometry,omitempty"`
	Display        *SnapshotDisplayDTO       `json:"display,omitempty"`
}

// SnapshotEnvironmentDTO mirrors data/models.Environment (trimmed).
type SnapshotEnvironmentDTO struct {
	UID              string                `json:"uid,omitempty"`
	Width            uint64                `json:"width"`
	Height           uint64                `json:"height"`
	Torus            bool                  `json:"torus"`
	CollisionMode    string                `json:"collision_mode,omitempty"`
	GlobalProperties []SnapshotPropertyDTO `json:"global_properties,omitempty"`
	Shapes           []SnapshotShapeDTO    `json:"shapes,omitempty"`
}

// ModelSnapshotDTO is the trimmed model definition the sim editor needs. It
// omits rules, interactions, events, images, and llm_config — those are not
// edited in the sim editor.
type ModelSnapshotDTO struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	IsSpatial   bool                    `json:"is_spatial"`
	Agents      []SnapshotAgentDTO      `json:"agents,omitempty"`
	Environment *SnapshotEnvironmentDTO `json:"environment,omitempty"`
}

// --- list / summary ---

// SimulationSummaryDTO mirrors data/simulations.SimulationSummary.
type SimulationSummaryDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ModelID     string    `json:"model_id"`
	ModelName   string    `json:"model_name,omitempty"`
	Status      string    `json:"status"`
	Steps       uint64    `json:"steps"`
	AgentCount  int       `json:"agent_count"`
	TotalTokens uint64    `json:"total_tokens"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
}

// ListSimulationsResponse is the paged list response.
type ListSimulationsResponse struct {
	Items   []SimulationSummaryDTO `json:"items"`
	Total   int                    `json:"total"` // total simulations matching the filter (across all pages)
	HasMore bool                   `json:"has_more"`
}

// --- full simulation response ---

// SimulationResponse is the complete simulation representation returned by GET
// and after mutating operations.
type SimulationResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ModelID     string `json:"model_id"`

	Seed  uint64 `json:"seed"`
	Steps uint64 `json:"steps"`

	// Termination is the optional early-stop condition (U2); nil = run to Steps.
	Termination *TerminationConditionDTO `json:"termination,omitempty"`

	LLMGenerationTarget *LLMTargetDTO             `json:"llm_generation_target,omitempty"`
	LLMExecutionTarget  *LLMTargetDTO             `json:"llm_execution_target,omitempty"`
	Agents              []SimulationAgentDTO      `json:"agents,omitempty"`
	Environment         *SimulationEnvironmentDTO `json:"environment,omitempty"`
	Widgets             []WidgetConfigDTO         `json:"widgets,omitempty"`

	Status        string            `json:"status"`
	StatusText    string            `json:"status_text,omitempty"`
	StatusHistory []StatusChangeDTO `json:"status_history,omitempty"`

	GenerationRuns []GenerationRunDTO       `json:"generation_runs,omitempty"`
	TokenUsage     *SimulationTokenUsageDTO `json:"token_usage,omitempty"`

	// ExecutionRuns is the audit trail of every engine run, oldest first.
	ExecutionRuns []ExecutionRunDTO `json:"execution_runs,omitempty"`
	// Statistics is the final computed statistics of the most recent run; nil
	// until a run completes (data/simulations.SimulationStatistics shape).
	Statistics interface{} `json:"statistics,omitempty"`

	CurrentStep uint64 `json:"current_step"`

	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by"`
	UpdatedAt  time.Time  `json:"updated_at"`
	UpdatedBy  string     `json:"updated_by"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	ModelSnapshot *ModelSnapshotDTO `json:"model_snapshot,omitempty"`

	// Validation carries the canonical start_ready flag and any issues found.
	Validation ValidationResult `json:"validation"`
}
