package chats

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
)

func TestChatEntityDocumentRoundTrip(t *testing.T) {
	id := bson.NewObjectID().Hex()
	refID := bson.NewObjectID().Hex()
	in := chats.Chat{
		ID:          id,
		Type:        chats.ChatTypeModel,
		Name:        "chat",
		Description: "desc",
		LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortMedium},
		ReferenceID: refID,
		TokenUsage: &common.TokenUsageSummary{
			ByModel: []common.TokenUsage{{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}, TotalTokens: 42, Calls: 1}},
		},
		CreatedBy: "u1", UpdatedBy: "u1",
	}
	doc, err := ChatToDocument(in)
	if err != nil {
		t.Fatalf("to document: %v", err)
	}
	out := ChatFromDocument(doc)
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", out, in)
	}
}

func TestChatToDocumentInvalidID(t *testing.T) {
	if _, err := ChatToDocument(chats.Chat{ID: "not-hex"}); err == nil {
		t.Fatal("want error for malformed id")
	}
	if _, err := ChatToDocument(chats.Chat{ReferenceID: "not-hex"}); err == nil {
		t.Fatal("want error for malformed reference id")
	}
}

func TestChatMessageToDocumentInvalidID(t *testing.T) {
	if _, err := ChatMessageToDocument(chats.ChatMessage{ID: "not-hex"}); err == nil {
		t.Fatal("want error for malformed id")
	}
	if _, err := ChatMessageToDocument(chats.ChatMessage{ChatID: "not-hex"}); err == nil {
		t.Fatal("want error for malformed chat id")
	}
}

func TestChatMessageEntityDocumentRoundTrip(t *testing.T) {
	in := chats.ChatMessage{
		ID:          bson.NewObjectID().Hex(),
		ChatID:      bson.NewObjectID().Hex(),
		Role:        chats.ChatMessageAssistantRole,
		TextContent: "preview",
		RawContent:  `{"spec":true}`,
		TokenUsage:  &common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}, PromptTokens: 10, TotalTokens: 12, Calls: 1},
		CreatedBy:   "llm:openai",
	}
	doc, err := ChatMessageToDocument(in)
	if err != nil {
		t.Fatalf("to document: %v", err)
	}
	out := ChatMessageFromDocument(doc)
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", out, in)
	}
}
