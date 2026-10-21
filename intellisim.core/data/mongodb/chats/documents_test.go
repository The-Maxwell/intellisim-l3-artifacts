package chats

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

func TestChatMessageRoundTrip(t *testing.T) {
	msg := ChatMessageDocument{
		Role:        chats.ChatMessageAssistantRole,
		TextContent: "preview",
		RawContent:  `{"spec":true}`,
		TokenUsage: &mongocommon.TokenUsageDocument{
			LLMTargetDocument: mongocommon.LLMTargetDocument{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh},
			PromptTokens:      10, TotalTokens: 12, Calls: 1,
		},
	}
	data, err := bson.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out ChatMessageDocument
	if err := bson.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Role != chats.ChatMessageAssistantRole {
		t.Fatalf("role: %q", out.Role)
	}
	if out.TextContent != "preview" || out.RawContent != `{"spec":true}` {
		t.Fatalf("content not preserved: text=%q raw=%q", out.TextContent, out.RawContent)
	}
	if out.TokenUsage == nil || out.TokenUsage.ModelUID != "gpt-4o" || out.TokenUsage.ProviderUID != "openai" ||
		out.TokenUsage.Effort != common.LLMEffortHigh {
		t.Fatalf("token usage target not preserved: %+v", out.TokenUsage)
	}
	if out.TokenUsage.PromptTokens != 10 || out.TokenUsage.TotalTokens != 12 || out.TokenUsage.Calls != 1 {
		t.Fatalf("token usage numerics not preserved: %+v", out.TokenUsage)
	}
}

func TestChatDocumentRoundTrip(t *testing.T) {
	doc := ChatDocument{
		Type:      chats.ChatTypeModel,
		LLMTarget: mongocommon.LLMTargetDocument{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortMedium},
		TokenUsage: &mongocommon.TokenUsageSummaryDocument{
			ByModel: []mongocommon.TokenUsageDocument{{
				LLMTargetDocument: mongocommon.LLMTargetDocument{ProviderUID: "openai", ModelUID: "gpt-4o"},
				TotalTokens:       42, Calls: 1,
			}},
		},
	}
	data, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out ChatDocument
	if err := bson.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.LLMTarget.ProviderUID != "openai" || out.LLMTarget.ModelUID != "gpt-4o" ||
		out.LLMTarget.Effort != common.LLMEffortMedium {
		t.Fatalf("llm target not preserved: %+v", out.LLMTarget)
	}
	if out.TokenUsage == nil || len(out.TokenUsage.ByModel) != 1 || out.TokenUsage.ByModel[0].ModelUID != "gpt-4o" {
		t.Fatalf("token usage summary not preserved: %+v", out.TokenUsage)
	}
}
