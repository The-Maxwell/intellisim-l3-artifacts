package chats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
)

func TestMapper_ToChatResponse(t *testing.T) {
	m := NewChatMapper()
	now := time.Now().UTC()
	chat := datachats.Chat{
		ID: "c1", Type: datachats.ChatTypePlain, Name: "My chat",
		LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortLow},
		CreatedAt: now, UpdatedAt: now, CreatedBy: "dev.user", UpdatedBy: "dev.user",
	}

	resp := m.ToChatResponse(chat, true, ChatStatusThinking)
	assert.Equal(t, "c1", resp.ID)
	assert.Equal(t, "plain", resp.Type)
	assert.Equal(t, LLMTargetDTO{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: "low"}, resp.LLMTarget)
	assert.True(t, resp.ModelAvailable)
	assert.Equal(t, ChatStatusThinking, resp.Status)
	assert.Equal(t, "dev.user", resp.CreatedBy)
}

func TestMapper_ToMessageResponse_ContentAndError(t *testing.T) {
	m := NewChatMapper()
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}

	ok := m.ToMessageResponse(datachats.ChatMessage{
		ID: "m1", ChatID: "c1", Role: datachats.ChatMessageAssistantRole,
		TextContent: "hello", LLMTarget: &target,
		TokenUsage: &common.TokenUsage{LLMTarget: target, TotalTokens: 5},
	})
	assert.Equal(t, "assistant", ok.Role)
	assert.Equal(t, "hello", ok.Content)
	assert.Equal(t, "openai", ok.LLMTarget.ProviderUID)
	assert.Nil(t, ok.Error)

	failed := m.ToMessageResponse(datachats.ChatMessage{
		ID: "m2", ChatID: "c1", Role: datachats.ChatMessageAssistantRole,
		LLMTarget: &target, Error: &datachats.MessageError{Code: "timeout", Message: "generation timed out"},
	})
	assert.Equal(t, "timeout", failed.Error.Code)
	assert.Empty(t, failed.Content)
}

func TestMapper_TargetFromDTO(t *testing.T) {
	target := targetFromDTO(LLMTargetDTO{ProviderUID: "p", ModelUID: "m", Effort: "high"})
	assert.Equal(t, common.LLMTarget{ProviderUID: "p", ModelUID: "m", Effort: common.LLMEffortHigh}, target)
}
