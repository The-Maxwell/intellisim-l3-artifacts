// Package common holds the MongoDB document forms of the shared value types in
// data/common, plus ID and value-normalization helpers used by all mongo mappers.
package common

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
)

// PointDocument is the bson form of common.Point.
type PointDocument struct {
	X uint64 `bson:"x"`
	Y uint64 `bson:"y"`
}

// ConfigValueDocument is the bson form of common.ConfigValue (single-key tagged).
type ConfigValueDocument struct {
	Double *float64 `bson:"double,omitempty"`
	Int    *int64   `bson:"int,omitempty"`
	Uint   *uint64  `bson:"uint,omitempty"`
	Bool   *bool    `bson:"bool,omitempty"`
	String *string  `bson:"string,omitempty"`
	List   []string `bson:"list,omitempty"`
}

// LLMTargetDocument is the bson form of common.LLMTarget.
type LLMTargetDocument struct {
	ProviderUID string                         `bson:"provider_uid,omitempty"`
	ModelUID    string                         `bson:"model_uid,omitempty"`
	Effort      common.LLMEffort               `bson:"effort,omitempty"`
	Generation  map[string]ConfigValueDocument `bson:"generation,omitempty"`
	TimeoutMs   *uint64                        `bson:"timeout_ms,omitempty"`
}

// TokenUsageDocument is the bson form of common.TokenUsage.
type TokenUsageDocument struct {
	LLMTargetDocument `bson:",inline"`

	PromptTokens     uint64  `bson:"prompt_tokens"`
	CompletionTokens uint64  `bson:"completion_tokens"`
	ReasoningTokens  uint64  `bson:"reasoning_tokens"`
	TotalTokens      uint64  `bson:"total_tokens"`
	Calls            uint64  `bson:"calls"`
	LatencyMs        float64 `bson:"latency_ms"`
}

// TokenUsageSummaryDocument is the bson form of common.TokenUsageSummary.
type TokenUsageSummaryDocument struct {
	Total   TokenUsageDocument   `bson:"total"`
	ByModel []TokenUsageDocument `bson:"by_model"`
}

// FileDocument is the bson form of common.File.
type FileDocument struct {
	UID         string          `bson:"uid"`
	Name        string          `bson:"name"`
	Type        common.FileType `bson:"type"`
	MimeType    string          `bson:"mime_type"`
	Size        int64           `bson:"size"`
	GridFSID    bson.ObjectID   `bson:"gridfs_id"`
	Step        *int64          `bson:"step,omitempty"`
	Description string          `bson:"description,omitempty"`
	CreatedAt   time.Time       `bson:"created_at"`
	CreatedBy   string          `bson:"created_by"`
}
