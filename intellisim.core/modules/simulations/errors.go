package simulations

import "errors"

var (
	ErrSimulationNotFound = errors.New("simulation not found")
	ErrModelNotFound      = errors.New("model not found")
	ErrNotEditable        = errors.New("simulation is not in an editable state")
	ErrGenerationInFlight = errors.New("a generation is already in progress for this simulation")
	ErrModelNotReady      = errors.New("model is not ready for simulation generation")
	// ErrInvalidLLMTarget means a user-supplied LLM target override failed
	// validation for the simulation-execution capability. Maps to 422.
	ErrInvalidLLMTarget = errors.New("llm target is not valid for simulation execution")

	// --- run-phase sentinels (engine) ---

	// ErrRunInFlight means a run is already active for this simulation. → 409 run_in_flight.
	ErrRunInFlight = errors.New("simulation run already in flight")
	// ErrNotRunnable means the simulation is not in a state from which a run may start. → 422 not_runnable.
	ErrNotRunnable = errors.New("simulation is not runnable")
	// ErrRunCapacity means the engine is at its concurrent-run capacity. → 429 run_capacity.
	ErrRunCapacity = errors.New("run capacity reached")
	// ErrInvalidTicket means a WebSocket connect ticket is missing, malformed, or expired. → 403 invalid_ticket.
	ErrInvalidTicket = errors.New("invalid or expired stream ticket")
	// ErrNotRunning means a control action targeted a simulation that is not running. → 409 not_running.
	ErrNotRunning = errors.New("simulation is not running")
	// ErrNotStreamable means a WS connect-ticket / stream join targeted a simulation
	// that has no live run (only running/paused simulations have an open stream
	// room). Issuing a ticket or upgrading for any other status would orphan a
	// lazily-created room + its goroutine. → 409 not_streamable.
	ErrNotStreamable = errors.New("simulation has no live run to stream")
	// ErrAgentNotFound means the requested agent instance id exists in neither the
	// live run nor the persisted simulation record. → 404 agent_not_found.
	ErrAgentNotFound = errors.New("agent not found in simulation")
	// ErrModelSnapshotNotFound means the simulation carries no frozen model
	// snapshot (defensive; every simulation created since 2026-06 has one).
	// → 404 model_snapshot_unavailable.
	ErrModelSnapshotNotFound = errors.New("simulation has no model snapshot")
)
