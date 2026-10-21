package llm

import (
	"context"
	"time"

	"intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
)

// Provider is a connected LLM provider — the routing target behind Service.
// Service validates targets BEFORE calling a Provider, so implementations are
// validation-free. RemoteProvider (gRPC) is the only implementation today; the
// interface stays exported so a future in-process provider can join the registry.
type Provider interface {
	UID() string
	Info() ProviderInfo
	Simple(ctx context.Context, req SimpleRequest) (*Completion, error)
	Chat(ctx context.Context, req ChatRequest) (*Completion, error)
	Vision(ctx context.Context, req VisionRequest) (*Completion, error)
	Reasoning(ctx context.Context, req ReasoningRequest) (*ReasoningCompletion, error)
}

// ProviderInfo is the registration snapshot of a connected provider.
// Also the GET /llms response item.
type ProviderInfo struct {
	UID         string      `json:"uid"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	ConnectedAt time.Time   `json:"connected_at"`
	Models      []ModelInfo `json:"models"`
}

type ModelInfo struct {
	UID             string                   `json:"uid"`
	Name            string                   `json:"name"`
	Capabilities    []common.ModelCapability `json:"capabilities"`
	Efforts         []common.LLMEffort       `json:"efforts"` // empty = no effort control
	ContextWindow   uint32                   `json:"context_window"`
	MaxOutputTokens uint32                   `json:"max_output_tokens"`
	ConfigParams    []ConfigParamDescriptor  `json:"config_params"`
	Reasoning       bool                     `json:"reasoning"`
	MaxImages       uint32                   `json:"max_images"` // max images per vision request (0 = unset → core treats as 1)
}

// ContextOverflowStrategy defines how Chat handles history that exceeds the
// context window. Core-side concern — never sent to providers.
type ContextOverflowStrategy string

const (
	OverflowStrategyCut       ContextOverflowStrategy = "cut"
	OverflowStrategySummarize ContextOverflowStrategy = "summarize"
)

type ResponseFormat string

const (
	ResponseFormatDefault ResponseFormat = ""
	ResponseFormatText    ResponseFormat = "text"
	ResponseFormatJSON    ResponseFormat = "json"
)

// ConfigParamType mirrors the wire descriptor type.
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

// ConfigParamDescriptor is one advertised tunable parameter (exposed via GET /llms).
type ConfigParamDescriptor struct {
	Key         string              `json:"key"`
	Type        ConfigParamType     `json:"type"`
	Required    bool                `json:"required"`
	Label       string              `json:"label,omitempty"`
	Description string              `json:"description,omitempty"`
	Min         *float64            `json:"min,omitempty"`
	Max         *float64            `json:"max,omitempty"`
	EnumValues  []string            `json:"enum_values,omitempty"`
	Default     *common.ConfigValue `json:"default,omitempty"`
}

// SimpleRequest is a single-turn completion — the Level-2 workhorse.
type SimpleRequest struct {
	Target         common.LLMTarget       // ProviderUID + ModelUID required; Effort optional
	Purpose        common.ModelCapability // optional: what the call is FOR; validated when set
	Message        string
	SystemPrompt   string
	ResponseFormat ResponseFormat // core-controlled
}

// ChatRequest is a multi-turn completion with history (Level 1 / Level 3 chats).
type ChatRequest struct {
	Target           common.LLMTarget
	Purpose          common.ModelCapability
	Message          string // the new user message
	SystemMessage    string
	Messages         []chats.ChatMessage     // chronological, oldest first
	OverflowStrategy ContextOverflowStrategy // core-side: "cut" (default) or "summarize"
	ChatType         string                  // "plain" | "model" | "simulation" — summarization context
	ResponseFormat   ResponseFormat          // core-controlled
}

// VisionImage is one reference image for a vision request. Description, when set,
// is sent as a text label before the image so the model can tell multiple
// reference images apart (e.g. "floor plan", "obstacle layer").
type VisionImage struct {
	Data        []byte // raw bytes; decode ModelImage base64 before calling
	Format      string // "png" | "jpeg" | "webp" | "gif"
	Description string // optional label fed before the image
}

// VisionRequest analyses one or more images (environment/shape extraction).
type VisionRequest struct {
	Target         common.LLMTarget
	Purpose        common.ModelCapability
	Message        string
	Images         []VisionImage // 1..N reference images analysed in ONE call
	Detail         string        // "auto" | "low" | "high"
	SystemPrompt   string
	ResponseFormat ResponseFormat // core-controlled
}

// ReasoningRequest is a deep-analysis call (Level 3 results analysis).
type ReasoningRequest struct {
	Target         common.LLMTarget
	Purpose        common.ModelCapability
	Prompt         string
	Context        string
	SystemPrompt   string
	ResponseFormat ResponseFormat // core-controlled
}

type Completion struct {
	Text         string            `json:"text"`
	FinishReason string            `json:"finish_reason"`
	Usage        common.TokenUsage `json:"usage"` // fully attributed; Calls=1
}

type ReasoningCompletion struct {
	Analysis     string            `json:"analysis"`
	Reasoning    string            `json:"reasoning"`
	FinishReason string            `json:"finish_reason"`
	Usage        common.TokenUsage `json:"usage"`
}
