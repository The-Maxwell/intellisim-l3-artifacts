package chats

import "errors"

// Service-level sentinels; the API layer maps them to APIError responses.
// Authorization sentinels (no-principal -> 401, foreign-owner -> 403) live in
// modules/auth (auth.ErrNoPrincipal / auth.ErrForbidden) and are mapped here.
var (
	// ErrChatNotFound is returned for genuinely missing or malformed-id chats
	// (404). A chat owned by another user is NOT collapsed into this — it
	// yields auth.ErrForbidden (403) instead.
	ErrChatNotFound         = errors.New("chats: chat not found")
	ErrModelUnavailable     = errors.New("chats: llm target is not available")
	ErrEffortNotAllowed     = errors.New("chats: effort not allowed for model")
	ErrGenerationInProgress = errors.New("chats: generation already in progress")
	ErrInvalidCursor        = errors.New("chats: invalid pagination cursor")
	ErrNoChanges            = errors.New("chats: update request has no changes")
)
