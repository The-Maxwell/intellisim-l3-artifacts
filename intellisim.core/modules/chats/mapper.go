package chats

import (
	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
)

// ChatMapper converts between domain entities and API DTOs. Pure — the
// service supplies live values (availability, status).
type ChatMapper struct{}

func NewChatMapper() *ChatMapper { return &ChatMapper{} }

func (m *ChatMapper) ToChatResponse(chat datachats.Chat, modelAvailable bool, status string) ChatResponse {
	return ChatResponse{
		ID:             chat.ID,
		Type:           string(chat.Type),
		Name:           chat.Name,
		Description:    chat.Description,
		LLMTarget:      targetToDTO(chat.LLMTarget),
		ModelAvailable: modelAvailable,
		Status:         status,
		ModelID:        chat.ReferenceID,
		TokenUsage:     chat.TokenUsage,
		CreatedAt:      chat.CreatedAt,
		UpdatedAt:      chat.UpdatedAt,
		CreatedBy:      chat.CreatedBy,
		UpdatedBy:      chat.UpdatedBy,
	}
}

func (m *ChatMapper) ToMessageResponse(msg datachats.ChatMessage) ChatMessageResponse {
	resp := ChatMessageResponse{
		ID:        msg.ID,
		ChatID:    msg.ChatID,
		Role:      string(msg.Role),
		Content:   msg.TextContent,
		Usage:     msg.TokenUsage,
		CreatedAt: msg.CreatedAt,
		CreatedBy: msg.CreatedBy,
	}
	if msg.LLMTarget != nil {
		dto := targetToDTO(*msg.LLMTarget)
		resp.LLMTarget = &dto
	}
	if msg.Error != nil {
		resp.Error = &MessageErrorDTO{Code: msg.Error.Code, Message: msg.Error.Message}
	}
	return resp
}

// ToMessagesResponse maps a CHRONOLOGICAL slice (callers reverse the
// repository's newest-first order before calling).
func (m *ChatMapper) ToMessagesResponse(msgs []datachats.ChatMessage) []ChatMessageResponse {
	out := make([]ChatMessageResponse, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, m.ToMessageResponse(msg))
	}
	return out
}

func targetToDTO(t common.LLMTarget) LLMTargetDTO {
	return LLMTargetDTO{ProviderUID: t.ProviderUID, ModelUID: t.ModelUID, Effort: string(t.Effort)}
}

func targetFromDTO(d LLMTargetDTO) common.LLMTarget {
	return common.LLMTarget{ProviderUID: d.ProviderUID, ModelUID: d.ModelUID, Effort: common.LLMEffort(d.Effort)}
}
