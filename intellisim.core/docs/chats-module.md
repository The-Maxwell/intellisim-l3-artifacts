# Chats Module Reference

The chats module (`modules/chats`) provides a general-purpose, LLM-backed conversation interface. Each chat belongs to one owner and targets a specific LLM model. Chat sessions are plain conversations (`type: "plain"`) not tied to any model or simulation document. Ownership is expressed via the `created_by` field, which stores the principal's `preferred_username` (falling back to `sub`). Accessing a foreign chat — one belonging to another user — returns **403 `forbidden`** for non-admins; a genuinely missing or malformed id returns **404 `not_found`**. Ownership is enforced in the service layer via the shared `auth` helpers (`auth.RequirePrincipal` + `UserPrincipal.Authorize`); admins (`AUTH_ADMIN_ENABLED=true`) bypass the check entirely.

---

## API Reference

All routes are mounted under `/api/v1/intellisim/chats` and require a valid `Authorization: Bearer <token>` header.

| Method | Path | Request | Success | Description |
|--------|------|---------|---------|-------------|
| `POST` | `/chats` | `CreateChatRequest` | 201 `ChatResponse` | Create a new plain chat |
| `GET` | `/chats` | query `page`, `page_size` | 200 `ListChatsResponse` | List caller's chats (newest-updated first) |
| `GET` | `/chats/:id` | — | 200 `ChatResponse` | Get a single chat |
| `PUT` | `/chats/:id` | `UpdateChatRequest` | 200 `ChatResponse` | Rename, re-describe, or retarget |
| `DELETE` | `/chats/:id` | — | 204 | Delete chat + all its messages |
| `GET` | `/chats/:id/messages` | query `before`, `limit` | 200 `MessagesPageResponse` | Cursor-paginated message history |
| `POST` | `/chats/:id/messages` | `SendMessageRequest` | 202 `SendMessageResponse` | Enqueue LLM generation (async) |
| `GET` | `/chats/:id/events` | — | 200 SSE stream | Subscribe to real-time chat events |

### Create chat

```
POST /api/v1/intellisim/chats
```

Request body:
```json
{
  "name": "My Chat",
  "description": "Optional description",
  "llm_target": {
    "provider_uid": "openai-provider",
    "model_uid": "gpt-4o",
    "effort": "medium"
  }
}
```

- `name` is optional (max 100 chars); defaults to `"New Chat"` if blank.
- `description` is optional (max 500 chars).
- `llm_target` is required and validated against the live provider registry. The model must exist and declare the `chat` capability.
- `effort` is optional; when set, must be one of `minimal | low | medium | high | max`.

Response (201):
```json
{
  "id": "6840a1b2c3d4e5f678901234",
  "type": "plain",
  "name": "My Chat",
  "description": "",
  "llm_target": { "provider_uid": "openai-provider", "model_uid": "gpt-4o", "effort": "medium" },
  "model_available": true,
  "status": "idle",
  "token_usage": null,
  "created_at": "2026-06-07T10:00:00Z",
  "updated_at": "2026-06-07T10:00:00Z",
  "created_by": "dev.user",
  "updated_by": "dev.user"
}
```

### Send message

```
POST /api/v1/intellisim/chats/:id/messages
```

Request body:
```json
{
  "content": "Hello! Can you explain agent-based modelling?"
}
```

- `content` is required, non-blank, and capped at 10 000 characters.
- Returns 409 `generation_in_progress` if a generation is already running for this chat.
- Returns 409 `model_unavailable` if the chat's target provider/model is not connected.

Response (202):
```json
{
  "message": {
    "id": "6840b1b2c3d4e5f678901235",
    "chat_id": "6840a1b2c3d4e5f678901234",
    "role": "user",
    "content": "Hello! Can you explain agent-based modelling?",
    "llm_target": null,
    "usage": null,
    "error": null,
    "created_at": "2026-06-07T10:01:00Z",
    "created_by": "dev.user"
  },
  "chat_status": "thinking"
}
```

The 202 response carries the persisted user message and the immediate post-send chat status. The LLM generation continues in the background; results are delivered over SSE.

### Get messages (paginated)

```
GET /api/v1/intellisim/chats/:id/messages?limit=30&before=<message_id>
```

Response (200):
```json
{
  "messages": [
    {
      "id": "6840b0b2c3d4e5f678901230",
      "chat_id": "6840a1b2c3d4e5f678901234",
      "role": "user",
      "content": "First message",
      "llm_target": null,
      "usage": null,
      "error": null,
      "created_at": "2026-06-07T09:50:00Z",
      "created_by": "dev.user"
    },
    {
      "id": "6840b0b2c3d4e5f678901231",
      "chat_id": "6840a1b2c3d4e5f678901234",
      "role": "assistant",
      "content": "Agent-based modelling is...",
      "llm_target": { "provider_uid": "openai-provider", "model_uid": "gpt-4o", "effort": "" },
      "usage": {
        "provider_uid": "openai-provider",
        "model_uid": "gpt-4o",
        "effort": "",
        "prompt_tokens": 120,
        "completion_tokens": 340,
        "total_tokens": 460,
        "calls": 1,
        "latency_ms": 1823.5
      },
      "error": null,
      "created_at": "2026-06-07T09:50:05Z",
      "created_by": "llm:openai-provider"
    }
  ],
  "has_more": true
}
```

Messages in the response are chronological (oldest first within the page). The cursor `before` points to the oldest message in the current view; when `has_more` is true, older pages exist.

---

## Error Envelope

All errors follow the `APIError` shape:
```json
{ "code": "not_found", "message": "chat not found" }
```

For validation failures `fields` is populated:
```json
{
  "code": "validation_failed",
  "message": "request validation failed",
  "fields": {
    "content": "max=10000",
    "llm_target.provider_uid": "required"
  }
}
```

### Error Code Table

| HTTP | Code | Cause |
|------|------|-------|
| 401 | `unauthorized` | Missing or invalid bearer token |
| 400 | `bad_request` | Malformed request body (unparseable JSON) |
| 400 | `invalid_cursor` | `before` message does not belong to this chat |
| 403 | `forbidden` | Chat exists but is owned by another user (caller is not an admin) |
| 404 | `not_found` | Chat does not exist (missing or malformed id) |
| 409 | `model_unavailable` | Chat's target provider/model is not connected right now |
| 409 | `effort_not_allowed` | Chat's target effort is not in the model's declared effort list |
| 409 | `generation_in_progress` | Another LLM generation is already running for this chat |
| 422 | `validation_failed` | DTO field constraint violated, or `ErrNoChanges` |
| 500 | `internal` | Unexpected server error |

---

## Pagination Contract

Message pagination uses keyset (cursor) pagination, not page/offset. The cursor is a `before=<message_id>` query parameter pointing to the oldest message in the current view.

- Default page size: 30 messages. Maximum: 100.
- Pages are returned **chronological** (oldest first) within each page. The repository fetches newest-first and the service reverses the slice before returning it.
- `has_more: true` means older messages exist and another `before` request with the oldest visible message ID will fetch the next page.
- The cursor ID must belong to the same chat; an ID from a different chat returns 400 `invalid_cursor`.
- MongoDB index supporting the query: `chat_messages { chat_id, created_at: -1, _id: -1 }`.
- Chat list uses offset pagination (`page`, `page_size`) sorted by `updated_at: -1`; the supporting index is on `chats { created_by, updated_at: -1 }`.

**Infinite-scroll pattern (frontend):**
1. Load initial page (no `before`): newest 30 messages.
2. When the user scrolls to the top, call with `before=messages[0].id`.
3. Prepend the returned page; preserve scroll position.
4. Stop when `has_more` is false.

---

## Async Send Flow

Sending a message is a two-phase operation:

1. **HTTP phase (synchronous, fast):**
   1. Verify caller owns the chat (`getOwnedChat`).
   2. Validate the chat's `llm_target` against the live registry (`checkTarget`). Returns 409 if the model is not connected.
   3. Acquire the pending lock (`pendingTracker.tryStart`). Returns 409 `generation_in_progress` if already taken.
   4. Persist the user message to MongoDB (`repo.AddMessage`).
   5. Publish `message.created` and `chat.status {status: "thinking"}` SSE events.
   6. Detach a goroutine for the LLM call (`go s.generate(...)`).
   7. Return 202 with the user message DTO and `chat_status: "thinking"`.

2. **Background phase (goroutine, async):**
   1. Load history: up to `CHATS_HISTORY_LIMIT` messages older than the new user message, chronological, with error-marked messages filtered out.
   2. Call `llm.Service.Chat(...)` with a `CHATS_GENERATION_TIMEOUT`-bounded context.
   3. Build the assistant message — either with `text_content` from the completion, or with an `error` field (`{code, message}`) if the LLM call failed.
   4. Persist the assistant message unconditionally. A failed generation is recorded as an error-marked assistant message so chat history remains truthful across reloads.
   5. Update the chat's `token_usage` aggregate and `updated_at`.
   6. Publish `message.created` SSE event for the assistant message.
   7. Release the pending lock and publish `chat.status {status: "idle"}`. The lock is always released before the idle event so a client reacting to idle never sees a stale `thinking` status.

---

## SSE Protocol

The SSE endpoint is:
```
GET /api/v1/intellisim/chats/:id/events
```

The response is `Content-Type: text/event-stream`. The same ownership check that protects `GET /chats/:id` is enforced before opening the stream. Only the owner (or an admin with `AUTH_ADMIN_ENABLED=true`) may subscribe.

### Event Types

All events are named events with a JSON `data` field.

**`message.created`** — new message (user or assistant):
```
event: message.created
data: {"id":"6840b1...","chat_id":"6840a1...","role":"assistant","content":"Agent-based modelling is...","llm_target":{"provider_uid":"openai-provider","model_uid":"gpt-4o","effort":""},"usage":{...},"error":null,"created_at":"2026-06-07T10:01:04Z","created_by":"llm:openai-provider"}

```

**`chat.status`** — thinking/idle transition:
```
event: chat.status
data: {"chat_id":"6840a1...","status":"thinking"}

```

**`chat.updated`** — chat renamed, re-described, or retargeted (full `ChatResponse`):
```
event: chat.updated
data: {"id":"6840a1...","name":"Renamed Chat","llm_target":{...},"model_available":true,"status":"idle",...}

```

**`chat.deleted`** — chat was deleted:
```
event: chat.deleted
data: {"chat_id":"6840a1..."}

```

**`chat.error`** — a background operation failed (rare path):
```
event: chat.error
data: {"chat_id":"6840a1...","code":"internal","message":"failed to save assistant message"}

```

### Heartbeat

A comment-only ping is sent every 25 seconds to keep the connection alive through proxies:
```
: ping

```

### Reconnect Contract

There is **no replay**. The SSE stream is a live notification channel only; it does not replay events that occurred while the client was disconnected. On reconnect, clients must refetch the chat object and the latest message page to reconstruct current state. `useChatEvents` in the frontend implements this via the `onReconnect` callback.

A slow subscriber whose channel buffer (capacity 16 events) overflows is disconnected immediately. The client is expected to reconnect and refetch.

---

## Status Semantics

Chat status (`idle` / `thinking`) is **computed, not persisted**. The `pendingTracker` is an in-memory map keyed by chat ID. When a generation goroutine is running for a chat, the tracker holds its ID; the `toChatResponse` helper queries the tracker when building any `ChatResponse`.

A server crash (or restart) empties the tracker. Any in-flight generation goroutine is also killed by the crash. After restart the status is correctly `idle` — no stale `thinking` state leaks into the API. The error-marked assistant message left in MongoDB (if persist succeeded before the crash) makes the failure visible in history.

---

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `CHATS_GENERATION_TIMEOUT` | `180s` | Deadline for one background LLM call. The context passed to `llm.Service.Chat` is cancelled after this duration. |
| `CHATS_HISTORY_LIMIT` | `50` | Maximum number of prior messages sent as conversation context. The `llm.Service` `ContextFitter` then trims further to fit the model's context window. |
