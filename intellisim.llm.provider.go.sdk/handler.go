package sdk

import "context"

// Handler is the required baseline every provider implements. Simple is the
// universal operation: Level-2 simulation strategies and core's chat-history
// summarization both use it.
type Handler interface {
	Simple(ctx context.Context, req *SimpleRequest) (*Completion, error)
}

// ChatHandler is required when any registered model declares
// CapabilityChat or CapabilityModelGeneration.
type ChatHandler interface {
	Chat(ctx context.Context, req *ChatRequest) (*Completion, error)
}

// VisionHandler is required when any registered model declares CapabilityVisionAnalysis.
type VisionHandler interface {
	Vision(ctx context.Context, req *VisionRequest) (*Completion, error)
}

// ReasoningHandler is required when any registered model declares CapabilityResultsAnalysis.
type ReasoningHandler interface {
	Reasoning(ctx context.Context, req *ReasoningRequest) (*ReasoningCompletion, error)
}
