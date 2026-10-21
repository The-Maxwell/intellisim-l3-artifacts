package models

// LLMStrategyType defines HOW the LLM is used at runtime. The individual-vs-archetype
// SCOPE is NOT encoded here — it derives from Agent.Type:
//
//	Article strategy        | LLMStrategyType  | Agent.Type
//	------------------------|------------------|-------------------
//	1 Rule-based            | none             | any
//	2 Individual full       | full             | agent
//	3 Archetype full        | full             | archetype
//	4 Individual decision   | decision         | agent
//	5 Archetype decision    | decision         | archetype
//	6 Hybrid                | hybrid (default) | per-agent overrides differ
//
// The Type is the single source of truth for which per-step purposes the LLM owns. The
// UseForMovement/UseForInteraction/UseForProperties/UseForDecisions flags are CANONICAL
// for a given Type (enforced by Canonical / Model.NormalizeLLMStrategies on save):
//
//	none     → all four flags false (no runtime LLM)
//	full     → all four flags true  (LLM owns movement, interaction, properties, decisions)
//	decision → only UseForDecisions true; the other three false
//	hybrid   → the four flags are user-configurable (free combination)
//
// Spawn and population are SIMULATION-GENERATION concerns, not execution: whether the LLM
// seeds the initial population is decided by AgentInitialization.CountMode==CountLLM, and
// per-spawn placement uses AgentInitialization.Placement. They are intentionally absent here.
//
// LLMConfig.DefaultStrategy is the baseline for agents without an override; a per-agent
// Agent.LLMStrategy overrides it. Model-level "hybrid" means "agents define their own".
type LLMStrategyType string

const (
	LLMStrategyNone     LLMStrategyType = "none"
	LLMStrategyFull     LLMStrategyType = "full"
	LLMStrategyDecision LLMStrategyType = "decision"
	LLMStrategyHybrid   LLMStrategyType = "hybrid"
)

// ArchetypeMode controls HOW a single archetype-level LLM result is applied to the
// members of an archetype (Strategies 3 & 5, archetype_full / archetype_decision).
// It is effective ONLY for archetype-type agents; it is inert for plain/individual
// agents and for the "none" strategy (the run engine consults it solely from the
// archetype fan-out path, never the individual one).
//
//	identical    → every member receives the exact same discrete outcome (decision
//	               label / full-control action template). This is the legacy
//	               "representative agent" behaviour.
//	distribution → the LLM returns a WEIGHTED DISTRIBUTION of outcomes per archetype
//	               and each member samples one via its per-agent seeded RNG, so
//	               members of one archetype genuinely diverge from a single LLM call.
//
// An empty ArchetypeMode resolves to "identical" at runtime (backward-safe for
// models persisted before this field existed); NEW models default it to
// "distribution" via Model.StampDefaultArchetypeMode at create/generation time.
type ArchetypeMode string

const (
	ArchetypeModeIdentical    ArchetypeMode = "identical"
	ArchetypeModeDistribution ArchetypeMode = "distribution"
)

// LLMStrategy defines the LLM usage pattern for the four per-step runtime purposes. The
// UseFor* flags are canonical for the Type (see LLMStrategyType) — only "hybrid" lets the
// user pick a free combination.
type LLMStrategy struct {
	Type      LLMStrategyType
	BatchSize int // agents/archetypes per LLM call

	UseForMovement    bool // ignored when Type == none
	UseForInteraction bool // ignored when Type == none
	UseForProperties  bool // ignored when Type == none
	UseForDecisions   bool // ignored when Type == none

	DecisionFrequency int // call LLM every N steps

	// ArchetypeMode + VariationStrength are effective ONLY for archetype-type agents
	// (Strategies 3 & 5); they are inert for plain/individual agents and for none.
	// See ArchetypeMode. An empty mode resolves to identical at runtime; a
	// VariationStrength <= 0 resolves to the engine's built-in jitter default.
	ArchetypeMode     ArchetypeMode // identical (default) | distribution
	VariationStrength float64       // 0..1 per-agent jitter amount; 0 → engine default
}

// Canonical returns a copy of the strategy with the four per-step purpose flags forced to
// the values mandated by Type, so persisted/validated/executed flags can never disagree
// with the chosen Type. Only "hybrid" preserves the user-supplied flags. BatchSize,
// DecisionFrequency, ArchetypeMode and VariationStrength are carried through unchanged
// (defaulting for ArchetypeMode happens only in creation/generation paths via
// Model.StampDefaultArchetypeMode, never here, so a generic update never flips the mode).
func (s LLMStrategy) Canonical() LLMStrategy {
	switch s.Type {
	case LLMStrategyFull:
		s.UseForMovement = true
		s.UseForInteraction = true
		s.UseForProperties = true
		s.UseForDecisions = true
	case LLMStrategyDecision:
		s.UseForMovement = false
		s.UseForInteraction = false
		s.UseForProperties = false
		s.UseForDecisions = true
	case LLMStrategyHybrid:
		// user-configurable — leave the flags as supplied
	default: // LLMStrategyNone or empty
		s.UseForMovement = false
		s.UseForInteraction = false
		s.UseForProperties = false
		s.UseForDecisions = false
	}
	return s
}

// LLMAgentStrategy is the per-agent/archetype runtime LLM behavior override (Strategy 6
// building block). It is intentionally a distinct named type (rather than reusing
// LLMStrategy directly) so agent-specific fields can be added later without affecting the
// shape path. Population COUNT/SIZE lives in AgentInitialization, and per-spawn placement
// guidance in AgentInitialization.Placement.PromptContext — neither belongs here.
type LLMAgentStrategy struct {
	LLMStrategy
}

// LLMShapeStrategy is the per-shape runtime LLM behavior override, analogous to
// LLMAgentStrategy for agents. It is intentionally a distinct type (rather than reusing
// LLMAgentStrategy) so shape-specific fields can be added later without affecting the
// agent path.
type LLMShapeStrategy struct {
	LLMStrategy
}
