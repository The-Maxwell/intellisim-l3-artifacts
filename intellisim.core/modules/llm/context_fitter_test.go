package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/data/chats"
)

type stubBudgeter struct {
	validateErr error
	trimmed     []chats.ChatMessage
	split       *SplitResult
	splitErr    error
}

func (s *stubBudgeter) CountTokens(text string) int { return len(text) }
func (s *stubBudgeter) ValidateTokenBudget(promptTokens, contextWindow, maxOutputTokens int) error {
	return s.validateErr
}
func (s *stubBudgeter) TrimChatHistory(m []chats.ChatMessage, p, c, o int) ([]chats.ChatMessage, error) {
	return s.trimmed, nil
}
func (s *stubBudgeter) SplitForSummarization(m []chats.ChatMessage, p, c, o int) (*SplitResult, error) {
	return s.split, s.splitErr
}
func (s *stubBudgeter) BuildSummarizationPrompt(m []chats.ChatMessage, chatType string) string {
	return "SUMMARIZE-THIS"
}

func msg(text string) chats.ChatMessage {
	return chats.ChatMessage{Role: chats.ChatMessageUserRole, TextContent: text}
}

func newFitter(b tokenBudgeter) *ContextFitter {
	return &ContextFitter{logger: zap.NewNop(), estimator: b}
}

func TestFitChat_PromptTooLarge(t *testing.T) {
	f := newFitter(&stubBudgeter{validateErr: errors.New("too big")})
	_, err := f.FitChat(context.Background(), ChatRequest{Message: "x"}, 100, 10, nil)
	assert.ErrorContains(t, err, "chat request too large")
}

func TestFitChat_NoMessages(t *testing.T) {
	f := newFitter(&stubBudgeter{})
	out, err := f.FitChat(context.Background(), ChatRequest{Message: "x"}, 100, 10, nil)
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestFitChat_CutStrategy(t *testing.T) {
	trimmed := []chats.ChatMessage{msg("recent")}
	f := newFitter(&stubBudgeter{trimmed: trimmed})
	out, err := f.FitChat(context.Background(), ChatRequest{
		Message: "x", Messages: []chats.ChatMessage{msg("old"), msg("recent")},
	}, 100, 10, nil)
	require.NoError(t, err)
	assert.Equal(t, trimmed, out)
}

func TestFitChat_SummarizeAllFits(t *testing.T) {
	f := newFitter(&stubBudgeter{split: nil})
	in := []chats.ChatMessage{msg("a")}
	out, err := f.FitChat(context.Background(), ChatRequest{
		Message: "x", Messages: in, OverflowStrategy: OverflowStrategySummarize,
	}, 100, 10, nil)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestFitChat_SummarizeSuccess(t *testing.T) {
	f := newFitter(&stubBudgeter{split: &SplitResult{
		ToSummarize: []chats.ChatMessage{msg("old1"), msg("old2")},
		ToKeep:      []chats.ChatMessage{msg("recent")},
	}})
	var captured *SimpleRequest
	simple := func(ctx context.Context, req SimpleRequest) (*Completion, error) {
		captured = &req
		return &Completion{Text: "the summary"}, nil
	}
	out, err := f.FitChat(context.Background(), ChatRequest{
		Message: "x", Messages: []chats.ChatMessage{msg("old1"), msg("old2"), msg("recent")},
		OverflowStrategy: OverflowStrategySummarize, ChatType: "model",
	}, 100, 10, simple)
	require.NoError(t, err)

	require.NotNil(t, captured)
	assert.Equal(t, "SUMMARIZE-THIS", captured.Message)

	require.Len(t, out, 2)
	assert.Equal(t, chats.ChatMessageSystemRole, out[0].Role)
	assert.Contains(t, out[0].TextContent, "the summary")
	assert.Contains(t, out[0].TextContent, "[Summary of 2 earlier messages]")
	assert.Equal(t, "recent", out[1].TextContent)
}

func TestFitChat_SummarizeFallsBackToTrim(t *testing.T) {
	trimmed := []chats.ChatMessage{msg("kept")}
	f := newFitter(&stubBudgeter{
		split:   &SplitResult{ToSummarize: []chats.ChatMessage{msg("old")}, ToKeep: nil},
		trimmed: trimmed,
	})
	simple := func(ctx context.Context, req SimpleRequest) (*Completion, error) {
		return nil, errors.New("llm down")
	}
	out, err := f.FitChat(context.Background(), ChatRequest{
		Message: "x", Messages: []chats.ChatMessage{msg("old")},
		OverflowStrategy: OverflowStrategySummarize,
	}, 100, 10, simple)
	require.NoError(t, err)
	assert.Equal(t, trimmed, out)
}
