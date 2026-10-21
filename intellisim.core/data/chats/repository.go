package chats

import (
	"context"
)

// ChatsRepository is the storage-agnostic persistence port for chats and chat
// messages. Implementations live under data/mongodb. Not-found conditions are
// reported as common.ErrNotFound by the getters; malformed ids as
// common.ErrInvalidID. Update and Delete are idempotent.
//
// Message reads return messages NEWEST FIRST (created_at desc, _id desc);
// callers assembling chronological views must reverse the slice.
type ChatsRepository interface {
	Create(ctx context.Context, chat Chat) (string, error)
	GetByID(ctx context.Context, id string) (*Chat, error)
	Update(ctx context.Context, id string, chat Chat) error
	Delete(ctx context.Context, id string) error
	// List returns one page of chats (updated_at desc) and whether more
	// pages exist after it.
	List(ctx context.Context, filter ListFilter) ([]Chat, bool, error)

	// ListMessagesBefore returns up to limit messages of the chat strictly
	// older than the message with beforeID (newest first), plus a has-more
	// flag. Empty beforeID = start from the latest message. A beforeID that
	// doesn't exist reports common.ErrNotFound.
	ListMessagesBefore(ctx context.Context, chatID string, limit int, beforeID string) ([]ChatMessage, bool, error)
	GetMessageByID(ctx context.Context, id string) (*ChatMessage, error)
	AddMessage(ctx context.Context, chatID string, message ChatMessage) (string, error)
	DeleteMessagesByChatID(ctx context.Context, chatID string) error
}
