package simulations

import "time"

// GenerationScopeKind selects the top-level scope of a generation run.
type GenerationScopeKind string

const (
	// ScopeFull regenerates the entire initial state: environment, all agent
	// populations, all properties, and all placements.
	ScopeFull GenerationScopeKind = "full"

	// ScopeEnvironment regenerates only the environment (global properties
	// and shapes); existing agent instances are untouched.
	ScopeEnvironment GenerationScopeKind = "environment"

	// ScopeAgents regenerates agent-related data only; the environment is
	// untouched. Further scoped by AgentUIDs and Parts.
	ScopeAgents GenerationScopeKind = "agents"
)

// GenerationPart selects which aspect of agent generation is (re)created.
// Used only when Kind == ScopeAgents.
type GenerationPart string

const (
	// PartPopulation determines how many instances of each archetype exist.
	PartPopulation GenerationPart = "population"

	// PartProperties fills in per-instance property values.
	PartProperties GenerationPart = "properties"

	// PartPlacement resolves per-instance positions in the environment.
	PartPlacement GenerationPart = "placement"
)

// GenerationScope selects what a generation run (re)creates. Kind drives the
// primary branch; AgentUIDs and Parts further narrow an agents-scoped run.
type GenerationScope struct {
	// Kind is the top-level selector.
	Kind GenerationScopeKind
	// AgentUIDs limits an agents-scoped run to the named archetypes; an empty
	// slice means all archetypes.
	AgentUIDs []string
	// Parts limits an agents-scoped run to the listed aspects; an empty slice
	// means all parts.
	Parts []GenerationPart
}

// GenerationRunStatus is the terminal or intermediate state of a GenerationRun.
type GenerationRunStatus string

const (
	// RunRunning means the generation pipeline is currently executing.
	RunRunning GenerationRunStatus = "running"

	// RunCompleted means the pipeline finished without fatal errors (warnings
	// may still be present in Warnings).
	RunCompleted GenerationRunStatus = "completed"

	// RunFailed means the pipeline encountered a fatal error and did not
	// produce a complete result.
	RunFailed GenerationRunStatus = "failed"
)

// GenerationWarning records a non-fatal degradation that occurred during
// generation, for example an invalid LLM-supplied value that was replaced by a
// random fallback.
type GenerationWarning struct {
	// Path identifies the field or entity that triggered the warning, using
	// dot-notation (e.g. "agents[2].properties.speed").
	Path string
	// Reason is a human-readable explanation of what went wrong and what
	// fallback was applied.
	Reason string
}

// GenerationRun is the audit record of one generation attempt on a simulation.
// Runs are appended in order; Seq is 1-based and corresponds to the index
// Seq-1 in SimulationTokenUsage.ByGeneration.
type GenerationRun struct {
	// Seq is the 1-based sequence number of this run within the simulation.
	Seq uint64
	// Seed is the RNG seed this run drew (and stamped onto Simulation.Seed) at the
	// time it started. Every run — the initial create and every regeneration —
	// records its own seed so the provenance of the current world is always
	// visible, even though Simulation.Seed itself only ever reflects the LATEST run.
	Seed       uint64
	Scope      GenerationScope
	Status     GenerationRunStatus
	StartedAt  time.Time
	FinishedAt *time.Time
	// Error is non-empty when Status == RunFailed; it contains the terminal
	// error message.
	Error    string
	Warnings []GenerationWarning
	// Requests is the number of LLM requests made during the run; equals the
	// sum of Calls folded into ByGeneration[Seq-1].
	Requests uint
}
