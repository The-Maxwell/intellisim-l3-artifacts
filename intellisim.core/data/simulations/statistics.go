package simulations

// AgentPopulation tracks the population of one agent/archetype definition over the run.
type AgentPopulation struct {
	AgentUID string `json:"agent_uid"`
	Current  uint64 `json:"current"`
	Peak     uint64 `json:"peak"`
	Min      uint64 `json:"min"`
	Spawned  uint64 `json:"spawned"` // born during the run (excl. initial population)
	Removed  uint64 `json:"removed"`
	// ByState is the CURRENT state distribution (keys = Agent.States values).
	ByState map[string]uint64 `json:"by_state,omitempty"`
}

// RuleStatistic counts executions of one Rule (model Rule.ID).
type RuleStatistic struct {
	RuleID     string `json:"rule_id"`
	Executions uint64 `json:"executions"`
}

// EventStatistic counts emissions of one EventDefinition (model EventDefinition.UID).
type EventStatistic struct {
	EventUID string `json:"event_uid"`
	Emitted  uint64 `json:"emitted"`
}

// InteractionStatistic counts occurrences of one Interaction (model Interaction.ID).
type InteractionStatistic struct {
	InteractionID string `json:"interaction_id"`
	Count         uint64 `json:"count"`
}

// PropertyStatistic is the CURRENT aggregate of one number-typed AgentProperty across the
// live instances of one agent/archetype definition (e.g. average prey energy). Recomputed on
// persist; the historical series lives in simulation_steps.
type PropertyStatistic struct {
	AgentUID    string  `json:"agent_uid"`
	PropertyUID string  `json:"property_uid"`
	Avg         float64 `json:"avg"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
}

// SimulationStatistics is a materialised summary of the run kept on the Simulation for
// fast reads. Detailed per-step series live in the simulation_steps timeseries and are queried
// on demand. The flat counters are fast totals; the By* slices break them down per model
// definition. (The LLM call count is not duplicated here — see SimulationTokenUsage.Total.Calls.)
type SimulationStatistics struct {
	StepsCompleted uint64 `json:"steps_completed"`
	TotalAgents    uint64 `json:"total_agents"` // current live agent count

	Population []AgentPopulation `json:"population,omitempty"` // per agent/archetype definition

	AgentsSpawned    uint64  `json:"agents_spawned"`
	AgentsRemoved    uint64  `json:"agents_removed"`
	Interactions     uint64  `json:"interactions"`
	Events           uint64  `json:"events"`
	RuleExecutions   uint64  `json:"rule_executions"`
	MovementDistance float64 `json:"movement_distance"` // total distance moved by all agents; float64 — Euclidean distance is fractional even on the integer grid

	ByRule        []RuleStatistic        `json:"by_rule,omitempty"`
	ByEvent       []EventStatistic       `json:"by_event,omitempty"`
	ByInteraction []InteractionStatistic `json:"by_interaction,omitempty"`
	Properties    []PropertyStatistic    `json:"properties,omitempty"` // number-typed property aggregates per agent/archetype

	Custom map[string]interface{} `json:"custom,omitempty"` // flexible extra metrics
}
