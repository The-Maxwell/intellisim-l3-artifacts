package chats

import "sync"

// EventType enumerates the chat SSE event types.
type EventType string

const (
	EventChatStatus     EventType = "chat.status"
	EventMessageCreated EventType = "message.created"
	EventChatUpdated    EventType = "chat.updated"
	EventChatDeleted    EventType = "chat.deleted"
	EventChatError      EventType = "chat.error"
)

// Event is one chat-scoped notification; Payload is the JSON-marshalable DTO
// written to the SSE data field.
type Event struct {
	Type    EventType
	ChatID  string
	Payload any
}

// subscriberBuffer is each subscriber's channel capacity. Chat events are
// low-frequency; a subscriber that falls this far behind is disconnected and
// expected to reconnect + refetch (the documented SSE client contract).
const subscriberBuffer = 16

// EventsHub is a minimal in-process pub/sub keyed by chat id. Designed for
// reuse by model/simulation chats later.
type EventsHub struct {
	mu     sync.Mutex
	nextID int
	subs   map[string]map[int]chan Event
}

func NewEventsHub() *EventsHub {
	return &EventsHub{subs: make(map[string]map[int]chan Event)}
}

// Subscribe registers a listener for one chat. The returned cancel is
// idempotent and closes the channel.
func (h *EventsHub) Subscribe(chatID string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	ch := make(chan Event, subscriberBuffer)
	if h.subs[chatID] == nil {
		h.subs[chatID] = make(map[int]chan Event)
	}
	h.subs[chatID][id] = ch

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.removeLocked(chatID, id)
	}
	return ch, cancel
}

// Publish delivers ev to every subscriber of the chat. Never blocks: a
// subscriber whose buffer is full is disconnected (channel closed).
func (h *EventsHub) Publish(chatID string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, ch := range h.subs[chatID] {
		select {
		case ch <- ev:
		default:
			h.removeLocked(chatID, id)
		}
	}
}

// CloseChat disconnects all subscribers of a chat (used on chat deletion,
// after the chat.deleted event has been published).
func (h *EventsHub) CloseChat(chatID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id := range h.subs[chatID] {
		h.removeLocked(chatID, id)
	}
}

// removeLocked must be called with h.mu held.
func (h *EventsHub) removeLocked(chatID string, id int) {
	chats, ok := h.subs[chatID]
	if !ok {
		return
	}
	ch, ok := chats[id]
	if !ok {
		return
	}
	delete(chats, id)
	if len(chats) == 0 {
		delete(h.subs, chatID)
	}
	close(ch)
}
