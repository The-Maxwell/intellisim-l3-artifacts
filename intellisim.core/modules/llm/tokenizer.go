package llm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"
	"go.uber.org/zap"
	"intelli.sim.core/data/chats"
)

// ErrContextWindowExceeded is returned when the prompt alone exceeds the
// model's context window and cannot be processed even without chat history.
var ErrContextWindowExceeded = errors.New("prompt exceeds model context window")

// TokenBudgetError provides details about a context window overflow.
type TokenBudgetError struct {
	PromptTokens    int
	MaxOutputTokens int
	ContextWindow   int
}

func (e *TokenBudgetError) Error() string {
	return fmt.Sprintf(
		"prompt exceeds model context window: prompt_tokens=%d, max_output_tokens=%d, context_window=%d, overflow=%d tokens",
		e.PromptTokens, e.MaxOutputTokens, e.ContextWindow,
		e.PromptTokens+e.MaxOutputTokens+safetyMarginTokens-e.ContextWindow,
	)
}

func (e *TokenBudgetError) Unwrap() error {
	return ErrContextWindowExceeded
}

const (
	defaultContextWindow   = 128000
	defaultMaxOutputTokens = 4096
	safetyMarginTokens     = 200
	tokensPerMessage       = 4   // overhead per chat message (role, separators)
	summaryTargetRatio     = 0.2 // summarize old messages into ~20% of their original token count
)

// TokenEstimator estimates token counts and trims chat history to fit context windows.
type TokenEstimator struct {
	logger   *zap.Logger
	encoding *tiktoken.Tiktoken
}

// NewTokenEstimator creates a TokenEstimator using cl100k_base encoding (GPT-4/GPT-3.5).
func NewTokenEstimator(logger *zap.Logger) *TokenEstimator {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		logger.Warn("Failed to load cl100k_base encoding, token estimation disabled", zap.Error(err))
		return &TokenEstimator{logger: logger.Named("token-estimator")}
	}
	return &TokenEstimator{
		logger:   logger.Named("token-estimator"),
		encoding: enc,
	}
}

// CountTokens returns the estimated token count for a text string.
func (te *TokenEstimator) CountTokens(text string) int {
	if te.encoding == nil {
		return 0
	}
	return len(te.encoding.Encode(text, nil, nil))
}

// CountChatTokens estimates the total tokens for a slice of chat messages.
func (te *TokenEstimator) CountChatTokens(messages []chats.ChatMessage) int {
	if te.encoding == nil {
		return 0
	}
	total := 0
	for _, m := range messages {
		total += te.CountTokens(m.TextContent) + tokensPerMessage
	}
	return total
}

// ValidateTokenBudget checks whether the prompt (without any chat history)
// fits within the model's context window. Returns a TokenBudgetError if not.
func (te *TokenEstimator) ValidateTokenBudget(promptTokens, contextWindow, maxOutputTokens int) error {
	if te.encoding == nil {
		return nil
	}

	if contextWindow <= 0 {
		contextWindow = defaultContextWindow
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}

	if promptTokens+maxOutputTokens+safetyMarginTokens > contextWindow {
		return &TokenBudgetError{
			PromptTokens:    promptTokens,
			MaxOutputTokens: maxOutputTokens,
			ContextWindow:   contextWindow,
		}
	}
	return nil
}

// TrimChatHistory removes the oldest messages so that the total prompt fits
// within the model's context window. Messages are expected in chronological
// order (oldest first). The function keeps the latest messages.
//
// Returns an error if the prompt alone (without any history) exceeds the context window.
func (te *TokenEstimator) TrimChatHistory(
	messages []chats.ChatMessage,
	promptTokens int,
	contextWindow int,
	maxOutputTokens int,
) ([]chats.ChatMessage, error) {
	if te.encoding == nil {
		return messages, nil
	}

	if contextWindow <= 0 {
		contextWindow = defaultContextWindow
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}

	available := contextWindow - promptTokens - maxOutputTokens - safetyMarginTokens
	if available <= 0 {
		te.logger.Warn("No token budget left for chat history after prompt",
			zap.Int("context_window", contextWindow),
			zap.Int("prompt_tokens", promptTokens),
			zap.Int("max_output_tokens", maxOutputTokens),
		)
		// Prompt itself doesn't fit — return error
		return nil, &TokenBudgetError{
			PromptTokens:    promptTokens,
			MaxOutputTokens: maxOutputTokens,
			ContextWindow:   contextWindow,
		}
	}

	// Walk from the end (newest) and accumulate until budget exhausted.
	totalTokens := 0
	cutIndex := len(messages) // start: include nothing
	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := te.CountTokens(messages[i].TextContent) + tokensPerMessage
		if totalTokens+msgTokens > available {
			break
		}
		totalTokens += msgTokens
		cutIndex = i
	}

	trimmed := len(messages) - (len(messages) - cutIndex)
	if trimmed > 0 {
		te.logger.Info("Trimmed chat history to fit context window",
			zap.Int("original_messages", len(messages)),
			zap.Int("kept_messages", len(messages)-cutIndex),
			zap.Int("history_tokens", totalTokens),
			zap.Int("available_tokens", available),
		)
	}

	return messages[cutIndex:], nil
}

// SplitResult holds the result of splitting chat history for summarization.
type SplitResult struct {
	// ToSummarize are the older messages that don't fit and should be summarized.
	ToSummarize []chats.ChatMessage
	// ToKeep are the recent messages that fit within the budget.
	ToKeep []chats.ChatMessage
	// SummaryMaxTokens is the target token budget for the summary text.
	SummaryMaxTokens int
}

// SplitForSummarization splits messages into two groups: older messages to be
// summarized and recent messages to keep. It also calculates how many tokens
// the summary should target so that summary + kept messages fit the context window.
//
// Returns nil if all messages already fit (no summarization needed).
func (te *TokenEstimator) SplitForSummarization(
	messages []chats.ChatMessage,
	promptTokens int,
	contextWindow int,
	maxOutputTokens int,
) (*SplitResult, error) {
	if te.encoding == nil {
		return nil, nil
	}

	if contextWindow <= 0 {
		contextWindow = defaultContextWindow
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}

	available := contextWindow - promptTokens - maxOutputTokens - safetyMarginTokens
	if available <= 0 {
		return nil, &TokenBudgetError{
			PromptTokens:    promptTokens,
			MaxOutputTokens: maxOutputTokens,
			ContextWindow:   contextWindow,
		}
	}

	// Check if all messages already fit
	totalChatTokens := te.CountChatTokens(messages)
	if totalChatTokens <= available {
		return nil, nil // no summarization needed
	}

	// Walk from the end (newest) keeping messages that fit in ~80% of budget.
	// Reserve the remaining ~20% for the summary of older messages.
	summaryBudget := int(float64(available) * summaryTargetRatio)
	keepBudget := available - summaryBudget - tokensPerMessage // -tokensPerMessage for the summary message itself

	totalKeptTokens := 0
	cutIndex := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := te.CountTokens(messages[i].TextContent) + tokensPerMessage
		if totalKeptTokens+msgTokens > keepBudget {
			break
		}
		totalKeptTokens += msgTokens
		cutIndex = i
	}

	// If cutIndex is 0, everything fits in the keep budget (shouldn't happen given the check above)
	if cutIndex == 0 {
		return nil, nil
	}

	te.logger.Info("Splitting chat history for summarization",
		zap.Int("total_messages", len(messages)),
		zap.Int("to_summarize", cutIndex),
		zap.Int("to_keep", len(messages)-cutIndex),
		zap.Int("summary_budget_tokens", summaryBudget),
	)

	return &SplitResult{
		ToSummarize:      messages[:cutIndex],
		ToKeep:           messages[cutIndex:],
		SummaryMaxTokens: summaryBudget,
	}, nil
}

// BuildSummarizationPrompt creates a prompt asking the LLM to summarize older
// chat messages. The prompt instructs the LLM to preserve key context relevant
// to the conversation type (model building, simulation, or plain chat).
func (te *TokenEstimator) BuildSummarizationPrompt(messages []chats.ChatMessage, chatType string) string {
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation history concisely. ")

	switch chatType {
	case "model":
		sb.WriteString("This conversation is about building/refining an agent-based model (ABM). ")
		sb.WriteString("Preserve all key decisions about: agent types and their properties, rules, interactions, ")
		sb.WriteString("environment setup, LLM strategy, and any constraints the user specified. ")
	case "simulation":
		sb.WriteString("This conversation is about configuring/running an agent-based simulation. ")
		sb.WriteString("Preserve all key decisions about: agent instances and their initial states, ")
		sb.WriteString("environment configuration, simulation parameters, and user-requested changes. ")
	default:
		sb.WriteString("Preserve the key points, decisions, and any important context from the conversation. ")
	}

	sb.WriteString("Output ONLY the summary text, no headers or formatting.\n\n")
	sb.WriteString("--- CONVERSATION HISTORY ---\n")

	for _, m := range messages {
		role := "User"
		if m.Role == chats.ChatMessageSystemRole {
			role = "Assistant"
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", role, m.TextContent))
	}

	return sb.String()
}
