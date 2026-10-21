package models

import (
	"intelli.sim.core/data/common"
)

// EventProducer enumerates who may emit an event.
type EventProducer string

const (
	EventProducerAgent       EventProducer = "agent"
	EventProducerEnvironment EventProducer = "environment"
	EventProducerLLM         EventProducer = "llm"
)

// EventScope controls who can perceive/consume an event.
type EventScope string

const (
	EventScopeGlobal   EventScope = "global"   // perceivable by all agents
	EventScopeLocal    EventScope = "local"    // within Radius of the emit position
	EventScopeTargeted EventScope = "targeted" // only ConsumerUIDs
)

// EventDefinition is a first-class, model-level event definition with producers (agents,
// environment, LLM) and consumers. Producers emit via ActionTypeEmitEvent (Action.EventUID);
// consumers react via ConditionTypeEvent. The runtime event bus (queue, TTL expiry, scope
// delivery) is engine work, out of scope for the data layer.
type EventDefinition struct {
	UID         string         // unique within model; referenced by emit/condition
	Name        string
	Description string
	Scope       EventScope
	Radius      *float64        // for local scope
	Payload     []AgentProperty // typed payload fields

	Producers    []EventProducer // who may emit
	ProducerUIDs []string        // optional: specific agent/archetype UIDs
	// ConsumerUIDs carries scope-dependent semantics:
	//   targeted – explicit recipient set; empty ⇒ no one receives the event.
	//   global/local – optional archetype filter; empty ⇒ any agent (subject to
	//                  local radius for local scope).
	ConsumerUIDs []string

	TTLSteps         uint   // 0 = active only on the emit step; N = active for N additional steps
	LLMPromptContext string

	// LLMTarget routes LLM-produced emissions of this event (Producers contains "llm")
	// to a specific provider/model/effort. nil = inherit, same resolution as Agent.LLMTarget.
	LLMTarget *common.LLMTarget

	// Locked freezes this event against model-generation LLM edits. User-only.
	Locked bool
}
