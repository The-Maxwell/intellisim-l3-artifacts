package llm

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"intelli.sim.core/data/chats"
)

// tokenBudgeter is the slice of TokenEstimator the fitter needs (stubbed in tests).
type tokenBudgeter interface {
	CountTokens(text string) int
	ValidateTokenBudget(promptTokens, contextWindow, maxOutputTokens int) error
	TrimChatHistory(messages []chats.ChatMessage, promptTokens, contextWindow, maxOutputTokens int) ([]chats.ChatMessage, error)
	SplitForSummarization(messages []chats.ChatMessage, promptTokens, contextWindow, maxOutputTokens int) (*SplitResult, error)
	BuildSummarizationPrompt(messages []chats.ChatMessage, chatType string) string
}

// simpleCaller lets the fitter issue the summarization call through the
// Service (so it gets the same validation/routing as any other call).
type simpleCaller func(ctx context.Context, req SimpleRequest) (*Completion, error)

// ContextFitter makes chat history fit a model's context window, by trimming
// ("cut") or LLM-summarizing ("summarize") older messages.
// Extracted from the old RemoteLLMProvider — core-side concern, not transport.
type ContextFitter struct {
	logger    *zap.Logger
	estimator tokenBudgeter
}

func NewContextFitter(logger *zap.Logger, estimator *TokenEstimator) *ContextFitter {
	return &ContextFitter{logger: logger.Named("context-fitter"), estimator: estimator}
}

// ValidatePrompt checks that the concatenated parts fit the window.
func (f *ContextFitter) ValidatePrompt(contextWindow, maxOutputTokens int, parts ...string) error {
	total := 0
	for _, p := range parts {
		total += f.estimator.CountTokens(p)
	}
	return f.estimator.ValidateTokenBudget(total, contextWindow, maxOutputTokens)
}

// FitChat returns a history that fits the window per the request's overflow
// strategy. Summarization failures fall back to trimming.
func (f *ContextFitter) FitChat(ctx context.Context, req ChatRequest, contextWindow, maxOutputTokens int, simpleCall simpleCaller) ([]chats.ChatMessage, error) {
	promptTokens := f.estimator.CountTokens(req.Message) + f.estimator.CountTokens(req.SystemMessage)
	if err := f.estimator.ValidateTokenBudget(promptTokens, contextWindow, maxOutputTokens); err != nil {
		return nil, fmt.Errorf("chat request too large: %w", err)
	}
	if len(req.Messages) == 0 {
		return nil, nil
	}
	if req.OverflowStrategy != OverflowStrategySummarize {
		return f.estimator.TrimChatHistory(req.Messages, promptTokens, contextWindow, maxOutputTokens)
	}

	split, err := f.estimator.SplitForSummarization(req.Messages, promptTokens, contextWindow, maxOutputTokens)
	if err != nil {
		return nil, err
	}
	if split == nil {
		return req.Messages, nil // everything fits
	}

	f.logger.Info("Summarizing older chat messages",
		zap.Int("to_summarize", len(split.ToSummarize)),
		zap.Int("to_keep", len(split.ToKeep)),
	)
	resp, err := simpleCall(ctx, SimpleRequest{
		Target:       req.Target,
		Purpose:      req.Purpose,
		Message:      f.estimator.BuildSummarizationPrompt(split.ToSummarize, req.ChatType),
		SystemPrompt: "You are a concise summarizer. Output only the summary.",
	})
	if err != nil {
		f.logger.Warn("Summarization call failed, falling back to cut strategy", zap.Error(err))
		return f.estimator.TrimChatHistory(req.Messages, promptTokens, contextWindow, maxOutputTokens)
	}

	summary := chats.ChatMessage{
		Role:        chats.ChatMessageSystemRole,
		TextContent: fmt.Sprintf("[Summary of %d earlier messages]\n%s", len(split.ToSummarize), resp.Text),
	}
	out := make([]chats.ChatMessage, 0, 1+len(split.ToKeep))
	out = append(out, summary)
	out = append(out, split.ToKeep...)
	return out, nil
}
