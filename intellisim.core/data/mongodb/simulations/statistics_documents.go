package simulations

// AgentPopulationDocument tracks the population of one agent/archetype definition over the run.
type AgentPopulationDocument struct {
	AgentUID string `bson:"agent_uid"`
	Current  uint64 `bson:"current"`
	Peak     uint64 `bson:"peak"`
	Min      uint64 `bson:"min"`
	Spawned  uint64 `bson:"spawned"` // born during the run (excl. initial population)
	Removed  uint64 `bson:"removed"`
	// ByState is the CURRENT state distribution (keys = Agent.States values).
	ByState map[string]uint64 `bson:"by_state,omitempty"`
}

// RuleStatisticDocument counts executions of one Rule (model Rule.ID).
type RuleStatisticDocument struct {
	RuleID     string `bson:"rule_id"`
	Executions uint64 `bson:"executions"`
}

// EventStatisticDocument counts emissions of one EventDefinition (model EventDefinition.UID).
type EventStatisticDocument struct {
	EventUID string `bson:"event_uid"`
	Emitted  uint64 `bson:"emitted"`
}

// InteractionStatisticDocument counts occurrences of one Interaction (model Interaction.ID).
type InteractionStatisticDocument struct {
	InteractionID string `bson:"interaction_id"`
	Count         uint64 `bson:"count"`
}

// PropertyStatisticDocument is the CURRENT aggregate of one number-typed AgentProperty across the
// live instances of one agent/archetype definition (e.g. average prey energy). Recomputed on
// persist; the historical series lives in simulation_steps.
type PropertyStatisticDocument struct {
	AgentUID    string  `bson:"agent_uid"`
	PropertyUID string  `bson:"property_uid"`
	Avg         float64 `bson:"avg"`
	Min         float64 `bson:"min"`
	Max         float64 `bson:"max"`
}

// SimulationStatisticsDocument is a materialised summary of the run kept on the SimulationDocument for
// fast reads. Detailed per-step series live in the simulation_steps timeseries and are queried
// on demand. The flat counters are fast totals; the By* slices break them down per model
// definition. (The LLM call count is not duplicated here — see SimulationTokenUsage.Total.Calls.)
type SimulationStatisticsDocument struct {
	StepsCompleted uint64 `bson:"steps_completed"`
	TotalAgents    uint64 `bson:"total_agents"` // current live agent count

	Population []AgentPopulationDocument `bson:"population,omitempty"` // per agent/archetype definition

	AgentsSpawned    uint64  `bson:"agents_spawned"`
	AgentsRemoved    uint64  `bson:"agents_removed"`
	Interactions     uint64  `bson:"interactions"`
	Events           uint64  `bson:"events"`
	RuleExecutions   uint64  `bson:"rule_executions"`
	MovementDistance float64 `bson:"movement_distance"` // total distance moved by all agents; float64 — Euclidean distance is fractional even on the integer grid

	ByRule        []RuleStatisticDocument        `bson:"by_rule,omitempty"`
	ByEvent       []EventStatisticDocument       `bson:"by_event,omitempty"`
	ByInteraction []InteractionStatisticDocument `bson:"by_interaction,omitempty"`
	Properties    []PropertyStatisticDocument    `bson:"properties,omitempty"` // number-typed property aggregates per agent/archetype

	Custom map[string]interface{} `bson:"custom,omitempty"` // flexible extra metrics
}
