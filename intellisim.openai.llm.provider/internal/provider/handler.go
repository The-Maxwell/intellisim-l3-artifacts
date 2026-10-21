package provider

import (
	"context"

	"go.uber.org/zap"

	sdk "github.com/The-Maxwell/intellisim.llm.provider.go.sdk"
)

// OpenAIHandler implements sdk.Handler, sdk.ChatHandler, sdk.VisionHandler,
// and sdk.ReasoningHandler. Core validates capabilities/efforts against our
// registration; the only local guard is the model whitelist.
type OpenAIHandler struct {
	logger *zap.Logger
	client *OpenAIClient
}

func NewOpenAIHandler(logger *zap.Logger, client *OpenAIClient) *OpenAIHandler {
	return &OpenAIHandler{logger: logger.Named("openai-handler"), client: client}
}

func (h *OpenAIHandler) Simple(ctx context.Context, req *sdk.SimpleRequest) (*sdk.Completion, error) {
	if err := checkModel(req.ModelUID); err != nil {
		return nil, err
	}
	resp, err := h.client.Simple(ctx, req)
	if err != nil {
		h.logger.Error("Simple failed", zap.String("model", req.ModelUID), zap.Error(err))
		return nil, err
	}
	return toCompletion(resp), nil
}

func (h *OpenAIHandler) Chat(ctx context.Context, req *sdk.ChatRequest) (*sdk.Completion, error) {
	if err := checkModel(req.ModelUID); err != nil {
		return nil, err
	}
	resp, err := h.client.Chat(ctx, req)
	if err != nil {
		h.logger.Error("Chat failed", zap.String("model", req.ModelUID), zap.Error(err))
		return nil, err
	}
	return toCompletion(resp), nil
}

func (h *OpenAIHandler) Vision(ctx context.Context, req *sdk.VisionRequest) (*sdk.Completion, error) {
	if err := checkModel(req.ModelUID); err != nil {
		return nil, err
	}
	resp, err := h.client.Vision(ctx, req)
	if err != nil {
		h.logger.Error("Vision failed", zap.String("model", req.ModelUID), zap.Error(err))
		return nil, err
	}
	return toCompletion(resp), nil
}

func (h *OpenAIHandler) Reasoning(ctx context.Context, req *sdk.ReasoningRequest) (*sdk.ReasoningCompletion, error) {
	if err := checkModel(req.ModelUID); err != nil {
		return nil, err
	}
	result, err := h.client.Reasoning(ctx, req)
	if err != nil {
		h.logger.Error("Reasoning failed", zap.String("model", req.ModelUID), zap.Error(err))
		return nil, err
	}
	return &sdk.ReasoningCompletion{
		Analysis: result.Analysis, Reasoning: result.Reasoning,
		FinishReason: result.FinishReason,
		Usage:        toUsage(result.Model, result.Usage, result.LatencyMs),
	}, nil
}

func checkModel(uid string) error {
	if !IsModelSupported(uid) {
		return sdk.Errorf(sdk.ErrCodeModelNotSupported, "model %q is not supported by this provider", uid)
	}
	return nil
}

func toCompletion(resp *LLMResponse) *sdk.Completion {
	return &sdk.Completion{
		Text:         resp.Content,
		FinishReason: resp.FinishReason,
		Usage:        toUsage(resp.Model, resp.Usage, resp.LatencyMs),
	}
}

// toUsage adapts OpenAI usage to SDK usage. Effort is left empty — the SDK
// auto-fills it with the requested effort.
func toUsage(model string, u UsageStats, latencyMs float64) sdk.Usage {
	return sdk.Usage{
		ModelUID:         model,
		PromptTokens:     uint64(u.PromptTokens),
		CompletionTokens: uint64(u.CompletionTokens),
		ReasoningTokens:  uint64(u.ReasoningTokens),
		TotalTokens:      uint64(u.TotalTokens),
		LatencyMs:        latencyMs,
	}
}
