package chats

import (
	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/chats"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

func ChatToDocument(c chats.Chat) (ChatDocument, error) {
	id, err := mongocommon.ParseOptionalID(c.ID)
	if err != nil {
		return ChatDocument{}, err
	}
	var refID *bson.ObjectID
	if c.ReferenceID != "" {
		oid, err := mongocommon.ParseID(c.ReferenceID)
		if err != nil {
			return ChatDocument{}, err
		}
		refID = &oid
	}
	return ChatDocument{
		ID:          id,
		Type:        c.Type,
		Name:        c.Name,
		Description: c.Description,
		LLMTarget:   mongocommon.LLMTargetToDocument(c.LLMTarget),
		ReferenceID: refID,
		TokenUsage:  mongocommon.TokenUsageSummaryPtrToDocument(c.TokenUsage),
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
		CreatedBy:   c.CreatedBy,
		UpdatedBy:   c.UpdatedBy,
	}, nil
}

func ChatFromDocument(d ChatDocument) chats.Chat {
	refID := ""
	if d.ReferenceID != nil {
		refID = mongocommon.FormatID(*d.ReferenceID)
	}
	return chats.Chat{
		ID:          mongocommon.FormatOptionalID(d.ID),
		Type:        d.Type,
		Name:        d.Name,
		Description: d.Description,
		LLMTarget:   mongocommon.LLMTargetFromDocument(d.LLMTarget),
		ReferenceID: refID,
		TokenUsage:  mongocommon.TokenUsageSummaryPtrFromDocument(d.TokenUsage),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
		CreatedBy:   d.CreatedBy,
		UpdatedBy:   d.UpdatedBy,
	}
}

func ChatMessageToDocument(m chats.ChatMessage) (ChatMessageDocument, error) {
	id, err := mongocommon.ParseOptionalID(m.ID)
	if err != nil {
		return ChatMessageDocument{}, err
	}
	chatID, err := mongocommon.ParseOptionalID(m.ChatID)
	if err != nil {
		return ChatMessageDocument{}, err
	}
	return ChatMessageDocument{
		ID:          id,
		ChatID:      chatID,
		Role:        m.Role,
		TextContent: m.TextContent,
		RawContent:  m.RawContent,
		TokenUsage:  mongocommon.TokenUsagePtrToDocument(m.TokenUsage),
		LLMTarget:   mongocommon.LLMTargetPtrToDocument(m.LLMTarget),
		Error:       messageErrorToDocument(m.Error),
		CreatedAt:   m.CreatedAt,
		CreatedBy:   m.CreatedBy,
	}, nil
}

func ChatMessageFromDocument(d ChatMessageDocument) chats.ChatMessage {
	return chats.ChatMessage{
		ID:          mongocommon.FormatOptionalID(d.ID),
		ChatID:      mongocommon.FormatOptionalID(d.ChatID),
		Role:        d.Role,
		TextContent: d.TextContent,
		RawContent:  d.RawContent,
		TokenUsage:  mongocommon.TokenUsagePtrFromDocument(d.TokenUsage),
		LLMTarget:   mongocommon.LLMTargetPtrFromDocument(d.LLMTarget),
		Error:       messageErrorFromDocument(d.Error),
		CreatedAt:   d.CreatedAt,
		CreatedBy:   d.CreatedBy,
	}
}

func messageErrorToDocument(e *chats.MessageError) *MessageErrorDocument {
	if e == nil {
		return nil
	}
	return &MessageErrorDocument{Code: e.Code, Message: e.Message}
}

func messageErrorFromDocument(d *MessageErrorDocument) *chats.MessageError {
	if d == nil {
		return nil
	}
	return &chats.MessageError{Code: d.Code, Message: d.Message}
}
