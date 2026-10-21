// Package chats is the MongoDB implementation of the data/chats repository port.
// Document types never leave this package: the repository maps them to/from the
// storage-agnostic entities at every boundary.
package chats

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/chats"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

// ChatDocument is the bson form of chats.Chat.
type ChatDocument struct {
	ID          bson.ObjectID                          `bson:"_id,omitempty"`
	Type        chats.ChatType                         `bson:"type"`
	Name        string                                 `bson:"name"`
	Description string                                 `bson:"description"`
	LLMTarget   mongocommon.LLMTargetDocument          `bson:"llm_target"`
	ReferenceID *bson.ObjectID                         `bson:"reference_id,omitempty"`
	TokenUsage  *mongocommon.TokenUsageSummaryDocument `bson:"token_usage,omitempty"`
	CreatedAt   time.Time                              `bson:"created_at"`
	UpdatedAt   time.Time                              `bson:"updated_at"`
	CreatedBy   string                                 `bson:"created_by"`
	UpdatedBy   string                                 `bson:"updated_by"`
}

// ChatMessageDocument is the bson form of chats.ChatMessage.
type ChatMessageDocument struct {
	ID          bson.ObjectID                   `bson:"_id,omitempty"`
	ChatID      bson.ObjectID                   `bson:"chat_id"`
	Role        chats.ChatMessageRole           `bson:"role"`
	TextContent string                          `bson:"text_content,omitempty"`
	RawContent  string                          `bson:"raw_content,omitempty"`
	TokenUsage  *mongocommon.TokenUsageDocument `bson:"token_usage,omitempty"`
	LLMTarget   *mongocommon.LLMTargetDocument  `bson:"llm_target,omitempty"`
	Error       *MessageErrorDocument           `bson:"error,omitempty"`
	CreatedAt   time.Time                       `bson:"created_at"`
	CreatedBy   string                          `bson:"created_by"`
}

// MessageErrorDocument is the bson form of chats.MessageError.
type MessageErrorDocument struct {
	Code    string `bson:"code"`
	Message string `bson:"message"`
}
