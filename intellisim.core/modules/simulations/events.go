package simulations

import "sync"

// EventType enumerates the simulation SSE / in-process event types.
type EventType string

const (
	EventGenerationProgress  EventType = "generation.progress"
	EventGenerationCompleted EventType = "generation.completed"
	EventGenerationFailed    EventType = "generation.failed"
	EventSimulationUpdated   EventType = "simulation.updated"
	EventSimulationDeleted   EventType = "simulation.deleted"
	EventAnalysisProgress    EventType = "analysis.progress"
	EventAnalysisCompleted   EventType = "analysis.completed"
	EventAnalysisFailed      EventType = "analysis.failed"
)

// Event is one simulation-scoped notification; Payload is the JSON-marshalable
// DTO written to the SSE data field.
type Event struct {
	Type         EventType
	SimulationID string
	Payload      any
}

// GenerationProgressPayload carries progress detail for a running generation run.
type GenerationProgressPayload struct {
	// Run is the 1-based generation run sequence number (same as GenerationRun.Seq).
	Run    uint64 `json:"run"`
	Stage  string `json:"stage"`
	Detail string `json:"detail"`
}

// AnalysisProgressPayload carries live progress for a running analysis.
type AnalysisProgressPayload struct {
	AnalysisID string `json:"analysis_id"`
	Stage      string `json:"stage"`
	Percent    uint8  `json:"percent"`
}

// AnalysisCompletedPayload announces a finished analysis (with its file metadata).
type AnalysisCompletedPayload struct {
	AnalysisID string `json:"analysis_id"`
	Status     string `json:"status"`
	FileUID    string `json:"file_uid,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
}

// AnalysisFailedPayload announces a failed analysis.
type AnalysisFailedPayload struct {
	AnalysisID string `json:"analysis_id"`
	Error      string `json:"error"`
}

// subscriberBuffer is each subscriber's channel capacity. Simulation events are
// low-frequency; a subscriber that falls this far behind is disconnected and
// expected to reconnect + refetch (the documented SSE client contract).
const subscriberBuffer = 16

// EventsHub is a minimal in-process pub/sub keyed by simulation id. Mirrors the
// chats and models EventsHub pattern exactly.
type EventsHub struct {
	mu     sync.Mutex
	nextID int
	subs   map[string]map[int]chan Event
}

func NewEventsHub() *EventsHub {
	return &EventsHub{subs: make(map[string]map[int]chan Event)}
}

// Subscribe registers a listener for one simulation. The returned cancel is
// idempotent and closes the channel.
func (h *EventsHub) Subscribe(simulationID string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	ch := make(chan Event, subscriberBuffer)
	if h.subs[simulationID] == nil {
		h.subs[simulationID] = make(map[int]chan Event)
	}
	h.subs[simulationID][id] = ch

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.removeLocked(simulationID, id)
	}
	return ch, cancel
}

// Publish delivers ev to every subscriber of the simulation. Never blocks: a
// subscriber whose buffer is full is disconnected (channel closed).
func (h *EventsHub) Publish(simulationID string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, ch := range h.subs[simulationID] {
		select {
		case ch <- ev:
		default:
			h.removeLocked(simulationID, id)
		}
	}
}

// CloseSimulation disconnects all subscribers of a simulation (used on
// simulation deletion, after the simulation.deleted event has been published).
func (h *EventsHub) CloseSimulation(simulationID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id := range h.subs[simulationID] {
		h.removeLocked(simulationID, id)
	}
}

// removeLocked must be called with h.mu held.
func (h *EventsHub) removeLocked(simulationID string, id int) {
	sims, ok := h.subs[simulationID]
	if !ok {
		return
	}
	ch, ok := sims[id]
	if !ok {
		return
	}
	delete(sims, id)
	if len(sims) == 0 {
		delete(h.subs, simulationID)
	}
	close(ch)
}
