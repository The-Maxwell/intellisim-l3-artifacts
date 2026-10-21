// Package sdk lets an LLM-provider application connect to an IntelliSim core,
// receive invocations, and return results — without touching gRPC or protobuf.
package sdk

// Effort mirrors core's common.LLMEffort. Empty = model/provider default.
type Effort string

const (
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortMax     Effort = "max"
)

// Capability is a model purpose from the closed list. Custom values are rejected.
type Capability string

const (
	CapabilityChat                 Capability = "chat"
	CapabilityModelGeneration      Capability = "model_generation"
	CapabilitySimulationGeneration Capability = "simulation_generation"
	CapabilitySimulationExecution  Capability = "simulation_execution"
	CapabilitySimulationDecisions  Capability = "simulation_decisions"
	CapabilityResultsAnalysis      Capability = "results_analysis"
	CapabilityVisionAnalysis       Capability = "vision_analysis"
)

// Registration identifies the provider and the models it serves.
type Registration struct {
	UID         string // e.g. "openai" — what core's LLMTarget.ProviderUID refers to
	Name        string
	Description string
	Version     string
	Models      []Model // at least one required
}

type Model struct {
	UID             string // e.g. "gpt-4o" — what core's LLMTarget.ModelUID refers to
	Name            string
	Capabilities    []Capability // at least one required, from the closed list
	Efforts         []Effort     // allowed efforts; empty = model has no effort control
	ContextWindow   uint32
	MaxOutputTokens uint32
	ConfigParams    []ConfigParam // tunable params; empty = none
	Reasoning       bool          // reasoning model (UI hint)
	MaxImages       uint32        // max images accepted in ONE vision request (0 = unset → core treats as 1)
}

type ResponseFormat string

const (
	ResponseFormatDefault ResponseFormat = ""
	ResponseFormatText    ResponseFormat = "text"
	ResponseFormatJSON    ResponseFormat = "json"
)

// ConfigValue is a single dynamic config value. Exactly one field is non-nil.
type ConfigValue struct {
	Double *float64
	Int    *int64
	Uint   *uint64
	Bool   *bool
	String *string
	List   []string
}

// GenerationConfig carries per-request tuning as a dynamic key->value map.
// Keys + validity are defined by the model's advertised ConfigParams.
type GenerationConfig struct {
	// Params may be nil when the invocation carried no config. Reads from a nil
	// map are safe; never write to it without a nil check.
	Params map[string]ConfigValue
}

// ConfigParamType is the kind of a tunable parameter.
type ConfigParamType string

const (
	ConfigParamDouble     ConfigParamType = "double"
	ConfigParamInt        ConfigParamType = "int"
	ConfigParamUint       ConfigParamType = "uint"
	ConfigParamBool       ConfigParamType = "bool"
	ConfigParamString     ConfigParamType = "string"
	ConfigParamEnum       ConfigParamType = "enum"
	ConfigParamStringList ConfigParamType = "string_list"
)

// ConfigParam declares one tunable parameter a model accepts (advertised at registration).
type ConfigParam struct {
	Key         string
	Type        ConfigParamType
	Required    bool
	Label       string
	Description string
	Min         *float64
	Max         *float64
	EnumValues  []string
	Default     *ConfigValue
}

type ChatMessage struct {
	Role    string // "system" | "user" | "assistant"
	Content string
}

type SimpleRequest struct {
	ModelUID       string
	Effort         Effort
	Message        string
	SystemPrompt   string
	Config         GenerationConfig
	ResponseFormat ResponseFormat // core-controlled
}

type ChatRequest struct {
	ModelUID       string
	Effort         Effort
	Message        string // the new user message
	SystemMessage  string
	History        []ChatMessage
	Config         GenerationConfig
	ResponseFormat ResponseFormat // core-controlled
}

// VisionImage is one reference image for a vision request. Description, when set,
// is emitted as a text label immediately before the image so the model can tell
// multiple images apart (e.g. "floor plan", "obstacle layer").
type VisionImage struct {
	ImageData   []byte
	ImageFormat string // "png" | "jpeg" | "webp" | "gif"
	Description string // optional label fed before the image
}

type VisionRequest struct {
	ModelUID       string
	Effort         Effort
	Message        string
	Images         []VisionImage // 1..N reference images analysed in ONE call
	Detail         string        // "auto" | "low" | "high"
	SystemPrompt   string
	Config         GenerationConfig
	ResponseFormat ResponseFormat // core-controlled
}

type ReasoningRequest struct {
	ModelUID       string
	Effort         Effort
	Prompt         string
	Context        string
	SystemPrompt   string
	Config         GenerationConfig
	ResponseFormat ResponseFormat // core-controlled
}

// Usage matches core's common.TokenUsage semantics. Zero-value fields are
// auto-filled by the SDK: ModelUID/Effort from the invocation, LatencyMs from
// the measured handler duration.
type Usage struct {
	ModelUID         string
	Effort           Effort
	PromptTokens     uint64
	CompletionTokens uint64
	// ReasoningTokens is the reasoning portion of CompletionTokens (a subset,
	// already included in CompletionTokens/TotalTokens); 0 for non-reasoning models.
	ReasoningTokens uint64
	TotalTokens     uint64
	LatencyMs       float64
}

type Completion struct {
	Text         string
	FinishReason string
	Usage        Usage
}

type ReasoningCompletion struct {
	Analysis     string
	Reasoning    string
	FinishReason string
	Usage        Usage
}

func validCapability(c Capability) bool {
	switch c {
	case CapabilityChat, CapabilityModelGeneration, CapabilitySimulationGeneration,
		CapabilitySimulationExecution, CapabilitySimulationDecisions,
		CapabilityResultsAnalysis, CapabilityVisionAnalysis:
		return true
	}
	return false
}

func validEffort(e Effort) bool {
	switch e {
	case EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortMax:
		return true
	}
	return false
}
