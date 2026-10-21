package models

import (
	"intelli.sim.core/data/models"
)

// LLMStrategyDocument defines the LLM usage pattern.
//
// LLMStrategyType defines HOW the LLM is used at runtime. The individual-vs-archetype
// SCOPE is NOT encoded here — it derives from AgentDocument.Type:
//
//	Article strategy        | LLMStrategyType  | AgentDocument.Type
//	------------------------|------------------|-------------------
//	1 Rule-based            | none             | any
//	2 Individual full       | full             | agent
//	3 Archetype full        | full             | archetype
//	4 Individual decision   | decision         | agent
//	5 Archetype decision    | decision         | archetype
//	6 Hybrid                | hybrid (default) | per-agent overrides differ
//
// LLMConfigDocument.DefaultStrategy is the baseline for agents without an override; a per-agent
// AgentDocument.LLMStrategy overrides it. Model-level "hybrid" means "agents define their own".
// The UseFor* flags are canonical for the Type (none→all off, full→all on, decision→decisions
// only, hybrid→free); see models.LLMStrategy.
type LLMStrategyDocument struct {
	Type      models.LLMStrategyType `bson:"type"`
	BatchSize int                    `bson:"batch_size,omitempty"` // agents/archetypes per LLM call

	UseForMovement    bool `bson:"use_for_movement"`
	UseForInteraction bool `bson:"use_for_interaction"`
	UseForProperties  bool `bson:"use_for_properties"`
	UseForDecisions   bool `bson:"use_for_decisions"`

	DecisionFrequency int `bson:"decision_frequency,omitempty"` // call LLM every N steps

	// ArchetypeMode + VariationStrength control archetype-scope result application
	// (archetype-type agents only). omitempty keeps pre-existing docs byte-stable:
	// an empty mode / zero strength are the backward-compatible runtime defaults.
	ArchetypeMode     models.ArchetypeMode `bson:"archetype_mode,omitempty"`
	VariationStrength float64              `bson:"variation_strength,omitempty"`
}

// LLMAgentStrategyDocument is the per-agent/archetype runtime LLM behavior override (Strategy 6
// building block). Population COUNT/SIZE lives in AgentInitialization; placement guidance in
// AgentInitialization.Placement.PromptContext — neither belongs here.
type LLMAgentStrategyDocument struct {
	LLMStrategyDocument `bson:",inline"`
}

// LLMShapeStrategyDocument is the per-shape runtime LLM behavior override, analogous to
// LLMAgentStrategyDocument for agents. It is intentionally a distinct type (rather than reusing
// LLMAgentStrategyDocument) so shape-specific fields can be added later without affecting the
// agent path.
type LLMShapeStrategyDocument struct {
	LLMStrategyDocument `bson:",inline"`
}
