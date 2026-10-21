package models

import "sync"

// pendingTracker enforces one in-flight generation per model and answers the
// model-chat's transient status (idle/thinking). In-memory only by design: a
// crash kills the generation goroutine with it, so the reset-to-idle is correct.
type pendingTracker struct {
	mu     sync.Mutex
	models map[string]struct{}
}

func newPendingTracker() *pendingTracker {
	return &pendingTracker{models: make(map[string]struct{})}
}

// tryStart marks the model as generating; false if one is already in flight.
func (p *pendingTracker) tryStart(modelID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, busy := p.models[modelID]; busy {
		return false
	}
	p.models[modelID] = struct{}{}
	return true
}

func (p *pendingTracker) finish(modelID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.models, modelID)
}

func (p *pendingTracker) isThinking(modelID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, busy := p.models[modelID]
	return busy
}
