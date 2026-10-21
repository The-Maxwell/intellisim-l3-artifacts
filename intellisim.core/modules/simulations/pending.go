package simulations

import "sync"

// pendingTracker enforces one in-flight generation per simulation. In-memory
// only by design: a crash kills the generation goroutine with it, so the
// reset-to-idle is correct.
type pendingTracker struct {
	mu          sync.Mutex
	simulations map[string]struct{}
}

func newPendingTracker() *pendingTracker {
	return &pendingTracker{simulations: make(map[string]struct{})}
}

// tryStart marks the simulation as generating; returns false if one is already
// in flight.
func (p *pendingTracker) tryStart(simulationID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, busy := p.simulations[simulationID]; busy {
		return false
	}
	p.simulations[simulationID] = struct{}{}
	return true
}

func (p *pendingTracker) finish(simulationID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.simulations, simulationID)
}

func (p *pendingTracker) isRunning(simulationID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, busy := p.simulations[simulationID]
	return busy
}
