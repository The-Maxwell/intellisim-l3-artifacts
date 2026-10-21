package models

import (
	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

// EventDefinitionDocument is a first-class, model-level event definition with producers (agents,
// environment, LLM) and consumers. Producers emit via ActionTypeEmitEvent (Action.EventUID);
// consumers react via ConditionTypeEvent. The runtime event bus (queue, TTL expiry, scope
// delivery) is engine work, out of scope for the data layer.
type EventDefinitionDocument struct {
	UID         string                  `bson:"uid"` // unique within model; referenced by emit/condition
	Name        string                  `bson:"name"`
	Description string                  `bson:"description,omitempty"`
	Scope       models.EventScope       `bson:"scope"`
	Radius      *float64                `bson:"radius,omitempty"`  // for local scope
	Payload     []AgentPropertyDocument `bson:"payload,omitempty"` // typed payload fields

	Producers    []models.EventProducer `bson:"producers"`               // who may emit
	ProducerUIDs []string               `bson:"producer_uids,omitempty"` // optional: specific agent/archetype UIDs
	ConsumerUIDs []string               `bson:"consumer_uids,omitempty"` // optional: reacting agent/archetype UIDs (empty = any)

	TTLSteps         uint   `bson:"ttl_steps,omitempty"` // 0 = active only on the emit step; N = active for N additional steps
	LLMPromptContext string `bson:"llm_prompt_context,omitempty"`

	// LLMTarget routes LLM-produced emissions of this event (Producers contains "llm")
	// to a specific provider/model/effort. nil = inherit, same resolution as Agent.LLMTarget.
	LLMTarget *mongocommon.LLMTargetDocument `bson:"llm_target,omitempty"`

	Locked bool `bson:"locked,omitempty"`
}
