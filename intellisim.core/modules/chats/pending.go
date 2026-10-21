package chats

import "sync"

// pendingTracker enforces one in-flight generation per chat and answers the
// chat's transient status (idle/thinking). In-memory only by design: a crash
// kills the generation goroutine with it, so the reset-to-idle is correct.
type pendingTracker struct {
	mu    sync.Mutex
	chats map[string]struct{}
}

func newPendingTracker() *pendingTracker {
	return &pendingTracker{chats: make(map[string]struct{})}
}

// tryStart marks the chat as generating; false if one is already in flight.
func (p *pendingTracker) tryStart(chatID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, busy := p.chats[chatID]; busy {
		return false
	}
	p.chats[chatID] = struct{}{}
	return true
}

func (p *pendingTracker) finish(chatID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.chats, chatID)
}

func (p *pendingTracker) isThinking(chatID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, busy := p.chats[chatID]
	return busy
}
