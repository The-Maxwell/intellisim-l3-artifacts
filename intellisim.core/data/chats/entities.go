package chats

import (
	"time"

	"intelli.sim.core/data/common"
)

type ChatType string

const (
	ChatTypePlain ChatType = "plain"
	ChatTypeModel ChatType = "model"
)

type ChatMessageRole string

// Chat messages are persisted with one of three roles. Note that the per-request
// system prompt sent to the LLM is assembled at call time (from LLMConfig.PromptContext
// and the prompt builders) and is NOT stored as a message — these roles describe only
// messages that are kept as part of the conversation thread:
//
//   - user:      messages authored by the researcher.
//   - assistant: responses produced by the LLM (e.g. model-generation output, replies).
//   - system:    system-authored messages persisted in the thread itself, e.g. a pinned
//     per-chat instruction or a system/status event surfaced in the chat
//     timeline. This is distinct from the per-request system prompt above,
//     which is built dynamically and never stored.
const (
	ChatMessageUserRole      ChatMessageRole = "user"
	ChatMessageAssistantRole ChatMessageRole = "assistant"
	ChatMessageSystemRole    ChatMessageRole = "system"
)

// Chat is the storage-agnostic chat entity. IDs are hex strings; ReferenceID is ""
// when the chat has no associated model (i.e. for plain chats).
type Chat struct {
	ID          string
	Type        ChatType
	Name        string
	Description string
	LLMTarget   common.LLMTarget          // which LLM serves this chat (provider, model, effort)
	ReferenceID string                    // model id when Type == ChatTypeModel; "" = unset (plain chats have no reference)
	TokenUsage  *common.TokenUsageSummary // running aggregate, per-model breakdown
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

// MessageError records a failed assistant generation, persisted so history
// stays truthful after reload. Code matches the API error codes
// (model_unavailable, effort_not_allowed, timeout, upstream_error, internal, ...).
type MessageError struct {
	Code    string
	Message string
}

// ListFilter narrows and pages chat listing. Empty CreatedBy = no owner filter
// (admin listing); empty Type = all types. Page is 1-based.
type ListFilter struct {
	CreatedBy string
	Type      ChatType
	Page      int
	PageSize  int
}

// ChatMessage is one persisted message of a chat thread.
type ChatMessage struct {
	ID          string
	ChatID      string
	Role        ChatMessageRole
	TextContent string             // plain text / preview
	RawContent  string             // full payload (e.g. structured model spec)
	TokenUsage  *common.TokenUsage // assistant messages only; model is TokenUsage.ModelUID
	LLMTarget   *common.LLMTarget  // assistant messages: target that produced (or failed) this reply
	Error       *MessageError      // non-nil = failed generation (TextContent empty)
	CreatedAt   time.Time
	CreatedBy   string
}
