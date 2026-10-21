package chats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recvEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		require.True(t, ok, "channel closed unexpectedly")
		return ev
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}

func TestEventsHub_PublishReachesOnlySameChatSubscribers(t *testing.T) {
	hub := NewEventsHub()
	subA, cancelA := hub.Subscribe("chat-a")
	defer cancelA()
	subB, cancelB := hub.Subscribe("chat-b")
	defer cancelB()

	hub.Publish("chat-a", Event{Type: EventChatStatus, ChatID: "chat-a"})

	ev := recvEvent(t, subA)
	assert.Equal(t, EventChatStatus, ev.Type)
	select {
	case <-subB:
		t.Fatal("chat-b subscriber must not receive chat-a events")
	default:
	}
}

func TestEventsHub_CancelStopsDelivery(t *testing.T) {
	hub := NewEventsHub()
	sub, cancel := hub.Subscribe("c")
	cancel()
	cancel() // double-cancel must not panic
	_, ok := <-sub
	assert.False(t, ok, "channel must be closed after cancel")
	hub.Publish("c", Event{Type: EventChatStatus, ChatID: "c"}) // must not panic
}

func TestEventsHub_SlowSubscriberIsDisconnected(t *testing.T) {
	hub := NewEventsHub()
	sub, cancel := hub.Subscribe("c")
	defer cancel()
	// Overflow the buffer (subscriberBuffer) without reading.
	for i := 0; i < subscriberBuffer+5; i++ {
		hub.Publish("c", Event{Type: EventChatStatus, ChatID: "c"})
	}
	// Drain: the channel must be closed after the overflow disconnect.
	n := 0
	for range sub {
		n++
	}
	assert.LessOrEqual(t, n, subscriberBuffer)
}

func TestEventsHub_CloseChatClosesAllSubscribers(t *testing.T) {
	hub := NewEventsHub()
	sub1, _ := hub.Subscribe("c")
	sub2, _ := hub.Subscribe("c")
	hub.CloseChat("c")
	_, ok1 := <-sub1
	_, ok2 := <-sub2
	assert.False(t, ok1)
	assert.False(t, ok2)
}
