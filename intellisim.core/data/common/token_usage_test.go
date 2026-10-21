package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenUsageSummary_Add(t *testing.T) {
	s := &TokenUsageSummary{}

	s.Add(TokenUsage{
		LLMTarget:    LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
		PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, Calls: 1, LatencyMs: 100,
	})
	s.Add(TokenUsage{
		LLMTarget:    LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
		PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, Calls: 1, LatencyMs: 50,
	})
	s.Add(TokenUsage{
		LLMTarget:    LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
		PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2, Calls: 1, LatencyMs: 10,
	})

	assert.Equal(t, LLMTarget{}, s.Total.LLMTarget) // grand total carries no target
	assert.Equal(t, uint64(47), s.Total.TotalTokens)
	assert.Equal(t, uint64(3), s.Total.Calls)
	assert.Len(t, s.ByModel, 2)
	assert.Equal(t, uint64(45), s.ByModel[0].TotalTokens) // gpt-4o aggregated
	assert.Equal(t, uint64(2), s.ByModel[1].TotalTokens)
}
