package chats

import (
	"time"

	"intelli.sim.core/data/common"
)

// Chat status values surfaced to the API (computed from the in-memory pending
// tracker — never persisted).
const (
	ChatStatusIdle     = "idle"
	ChatStatusThinking = "thinking"
)

// LLMTargetDTO is the API form of common.LLMTarget.
type LLMTargetDTO struct {
	ProviderUID string `json:"provider_uid" validate:"required,min=1,max=100"`
	ModelUID    string `json:"model_uid" validate:"required,min=1,max=100"`
	Effort      string `json:"effort,omitempty" validate:"omitempty,oneof=minimal low medium high max"`
}

type CreateChatRequest struct {
	Name        string        `json:"name" validate:"omitempty,max=100"`
	Description string        `json:"description" validate:"omitempty,max=500"`
	LLMTarget   *LLMTargetDTO `json:"llm_target" validate:"required"`
}

type UpdateChatRequest struct {
	Name        *string       `json:"name" validate:"omitempty,notblank,min=1,max=100"`
	Description *string       `json:"description" validate:"omitempty,max=500"`
	LLMTarget   *LLMTargetDTO `json:"llm_target"`
}

// HasChanges mirrors the "at least one field" rule (400 otherwise).
func (r UpdateChatRequest) HasChanges() bool {
	return r.Name != nil || r.Description != nil || r.LLMTarget != nil
}

type SendMessageRequest struct {
	Content string `json:"content" validate:"required,notblank,max=10000"`
}

type ListChatsQuery struct {
	Page     int `query:"page" json:"page" validate:"omitempty,min=1"`
	PageSize int `query:"page_size" json:"page_size" validate:"omitempty,min=1,max=100"`
}

type ListMessagesQuery struct {
	Limit  int    `query:"limit" json:"limit" validate:"omitempty,min=1,max=100"`
	Before string `query:"before" json:"before" validate:"omitempty,len=24,hexadecimal"`
}

type ChatResponse struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	LLMTarget   LLMTargetDTO `json:"llm_target"`
	// ModelAvailable reflects the LIVE provider registry: false means the
	// chat's target cannot serve right now and sends will be rejected.
	ModelAvailable bool                      `json:"model_available"`
	Status         string                    `json:"status"`
	ModelID        string                    `json:"model_id,omitempty"`
	TokenUsage     *common.TokenUsageSummary `json:"token_usage,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
	CreatedBy      string                    `json:"created_by"`
	UpdatedBy      string                    `json:"updated_by"`
}

type MessageErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ChatMessageResponse struct {
	ID        string             `json:"id"`
	ChatID    string             `json:"chat_id"`
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	LLMTarget *LLMTargetDTO      `json:"llm_target,omitempty"`
	Usage     *common.TokenUsage `json:"usage,omitempty"`
	Error     *MessageErrorDTO   `json:"error,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	CreatedBy string             `json:"created_by"`
}

type ListChatsResponse struct {
	Chats    []ChatResponse `json:"chats"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	HasMore  bool           `json:"has_more"`
}

type MessagesPageResponse struct {
	// Messages are CHRONOLOGICAL (oldest → newest) within the page.
	Messages []ChatMessageResponse `json:"messages"`
	HasMore  bool                  `json:"has_more"`
}

type SendMessageResponse struct {
	Message    ChatMessageResponse `json:"message"`
	ChatStatus string              `json:"chat_status"`
}

// SSE payloads (event data for types that aren't full DTOs).
type ChatStatusPayload struct {
	ChatID string `json:"chat_id"`
	Status string `json:"status"`
}

type ChatDeletedPayload struct {
	ChatID string `json:"chat_id"`
}

type ChatErrorPayload struct {
	ChatID  string `json:"chat_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
