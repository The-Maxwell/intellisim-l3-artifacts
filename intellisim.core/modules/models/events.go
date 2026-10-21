package models

import "sync"

// EventType enumerates the model-chat SSE event types.
type EventType string

const (
	EventChatStatus     EventType = "chat.status"
	EventMessageCreated EventType = "message.created"
	EventModelUpdated   EventType = "model.updated"
	EventChatError      EventType = "chat.error"
	EventModelDeleted   EventType = "model.deleted"
)

// Event is one model-scoped notification; Payload is the JSON-marshalable DTO
// written to the SSE data field.
type Event struct {
	Type    EventType
	ModelID string
	Payload any
}

// subscriberBuffer is each subscriber's channel capacity. Model events are
// low-frequency; a subscriber that falls this far behind is disconnected and
// expected to reconnect + refetch (the documented SSE client contract).
const subscriberBuffer = 16

// EventsHub is a minimal in-process pub/sub keyed by model id. Mirrors the
// chats EventsHub.
type EventsHub struct {
	mu     sync.Mutex
	nextID int
	subs   map[string]map[int]chan Event
}

func NewEventsHub() *EventsHub {
	return &EventsHub{subs: make(map[string]map[int]chan Event)}
}

// Subscribe registers a listener for one model. The returned cancel is
// idempotent and closes the channel.
func (h *EventsHub) Subscribe(modelID string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	ch := make(chan Event, subscriberBuffer)
	if h.subs[modelID] == nil {
		h.subs[modelID] = make(map[int]chan Event)
	}
	h.subs[modelID][id] = ch

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.removeLocked(modelID, id)
	}
	return ch, cancel
}

// Publish delivers ev to every subscriber of the model. Never blocks: a
// subscriber whose buffer is full is disconnected (channel closed).
func (h *EventsHub) Publish(modelID string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, ch := range h.subs[modelID] {
		select {
		case ch <- ev:
		default:
			h.removeLocked(modelID, id)
		}
	}
}

// CloseChat disconnects all subscribers of a model (used on model deletion,
// after the model.deleted event has been published).
func (h *EventsHub) CloseChat(modelID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id := range h.subs[modelID] {
		h.removeLocked(modelID, id)
	}
}

// removeLocked must be called with h.mu held.
func (h *EventsHub) removeLocked(modelID string, id int) {
	models, ok := h.subs[modelID]
	if !ok {
		return
	}
	ch, ok := models[id]
	if !ok {
		return
	}
	delete(models, id)
	if len(models) == 0 {
		delete(h.subs, modelID)
	}
	close(ch)
}
