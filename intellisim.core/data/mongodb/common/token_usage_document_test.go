package common

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
)

func TestTokenUsageDocumentRoundTrip(t *testing.T) {
	in := TokenUsageDocument{
		LLMTargetDocument: LLMTargetDocument{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh},
		PromptTokens:      100,
		CompletionTokens:  50,
		TotalTokens:       150,
		Calls:             2,
		LatencyMs:         12.5,
	}
	data, err := bson.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out TokenUsageDocument
	if err := bson.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", out, in)
	}
}

func TestTokenUsageSummaryDocumentRoundTrip(t *testing.T) {
	in := TokenUsageSummaryDocument{
		Total: TokenUsageDocument{PromptTokens: 150, CompletionTokens: 70, TotalTokens: 220, Calls: 3},
		ByModel: []TokenUsageDocument{
			{LLMTargetDocument: LLMTargetDocument{ModelUID: "gpt-4o", Effort: common.LLMEffortHigh}, PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, Calls: 2},
			{LLMTargetDocument: LLMTargetDocument{ModelUID: "gpt-4o-mini"}, PromptTokens: 50, CompletionTokens: 20, TotalTokens: 70, Calls: 1},
		},
	}
	data, err := bson.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out TokenUsageSummaryDocument
	if err := bson.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", out, in)
	}
}

func TestTokenUsageEntityDocumentRoundTrip(t *testing.T) {
	entity := common.TokenUsage{
		LLMTarget:    common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh},
		PromptTokens: 10, TotalTokens: 12, Calls: 1,
	}
	back := TokenUsageFromDocument(TokenUsageToDocument(entity))
	if !reflect.DeepEqual(back, entity) {
		t.Fatalf("entity round-trip mismatch:\n got  %+v\n want %+v", back, entity)
	}
}
