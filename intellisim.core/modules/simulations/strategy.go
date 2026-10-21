package simulations

import (
	"context"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
)

// purpose names one of the four per-step LLM participation slots.
type purpose string

const (
	purposeMovement    purpose = "movement"
	purposeProperties  purpose = "properties"
	purposeDecision    purpose = "decision"
	purposeInteraction purpose = "interaction"
)

// DecisionIntent is one labelled decision the agent chose this step.
type DecisionIntent struct {
	Label  string
	Params map[string]any
}

// ActionIntent is a constrained action the rules/spatial phases apply (the
// full-control action vocabulary mapped onto the engine's action model).
type ActionIntent struct {
	Type        models.ActionType
	PropertyUID string
	Value       any
	Modifier    models.PropertyModifier
	TargetUID   string
	Speed       *float64
	Direction   *float64
	NewState    string
	// EventUID and EventData carry emit_event intent (Task 17: LLM-emitted events).
	// These are populated by validateRawAction for the emit_event action type and
	// consumed in ApplyIntentActions via applyEmitEvent(..., EventProducerLLM).
	EventUID  string
	EventData any
}

// EmitIntent is a discrete LLM-requested event emission. It is gathered during
// the (potentially parallel) LLM phase into AgentIntent.Emits and applied
// sequentially in ApplyIntentActions via applyEmitEvent(..., EventProducerLLM).
// Keeping emits separate from Actions makes the authorization path unambiguous
// and avoids touching the actionPurpose gating that guards existing action types.
type EmitIntent struct {
	EventUID string
	Data     any
}

// MovementIntent is the agent's desired movement for the step.
type MovementIntent struct {
	Intent    string // toward|away|wander|hold
	TargetRef string // agentUID|shapeUID|"x,y"
	Speed     float64
	// Resolved, when non-nil, is a pre-computed per-agent velocity that the spatial
	// phase must use VERBATIM (applySpatial honours it instead of re-resolving via
	// resolveMovement). The archetype variation helper sets it so the per-agent
	// heading/speed jitter it applies actually reaches integration; individual
	// strategies leave it nil so applySpatial resolves the intent as before.
	Resolved *Vector
}

// WeightedDecision is one weighted option in an archetype DECISION distribution
// (archetype_decision + ArchetypeModeDistribution). Each member samples one Decision
// via its per-agent RNG (pickChoice over the weights).
type WeightedDecision struct {
	Decision DecisionIntent
	Weight   float64
}

// WeightedActionSet is one weighted whole-action-set variant in an archetype FULL
// distribution (archetype_full + ArchetypeModeDistribution). Each member samples ONE
// variant (its complete Actions + Emits), never a mix across variants.
type WeightedActionSet struct {
	Actions []ActionIntent
	Emits   []EmitIntent
	Weight  float64
}

// AgentIntent aggregates an agent's LLM-derived intents for one step.
type AgentIntent struct {
	Decisions  []DecisionIntent
	Actions    []ActionIntent
	Movement   *MovementIntent
	Properties map[string]any
	// Emits carries LLM-requested event emissions (Task 17). Each entry is
	// applied sequentially in ApplyIntentActions via
	// applyEmitEvent(..., EventProducerLLM) AFTER the Actions loop, so event
	// queueing never races with the parallel LLM-gather phase.
	Emits []EmitIntent
	// LLMNote is a compact human-readable summary of the LLM response that
	// produced this intent (e.g. "actions: set_property,move" or "chose: flee").
	// It is written to AgentMemoryLLM at the end of the mutation phase so the
	// agent's recent LLM context can feed back into future prompts.
	LLMNote string

	// DecisionDist and ActionVariants are DIRECTIVE-TEMPLATE-ONLY fields, populated by
	// the archetype strategies under ArchetypeModeDistribution and consumed by
	// ApplyDirectiveWithVariation (which samples one outcome per member). They never
	// survive into a per-agent intent (fan-out collapses them to a single sampled
	// Decisions/Actions), so mergeIntents intentionally ignores them — do NOT merge.
	DecisionDist   []WeightedDecision  // archetype_decision distribution
	ActionVariants []WeightedActionSet // archetype_full distribution
}

// PlanResult is what a strategy returns for one archetype group's step.
type PlanResult struct {
	Intents  map[uint64]*AgentIntent
	Warnings []string
}

// AgentGroup is one archetype's members for a step, with the resolved strategy
// and LLM target shared by the group.
type AgentGroup struct {
	ArchetypeUID string
	Archetype    *models.Agent
	Members      []*RuntimeAgent
	Strategy     models.LLMStrategy // resolved (agent override or model default)
	Target       common.LLMTarget   // resolved
}

// StepContext carries per-step shared deps to strategies.
type StepContext struct {
	RS  *RuntimeState
	LLM *llmRuntime
	Cfg RunConfig
	Ctx context.Context
}

// ExecutionStrategy plans one archetype group's intents for a step. The six
// article strategies each implement it; selectStrategy maps configuration to the
// concrete type.
type ExecutionStrategy interface {
	Name() string
	Plan(sc *StepContext, group *AgentGroup) (PlanResult, error)
}

// selectStrategy maps (LLMStrategyType × Agent.Type) → the concrete strategy:
//
//	none                         → rulesOnlyStrategy        (no LLM)
//	full     × agent (plain)     → individualFullStrategy
//	full     × archetype         → archetypeFullStrategy
//	decision × agent (plain)     → individualDecisionStrategy
//	decision × archetype         → archetypeDecisionStrategy
//	hybrid   × any               → hybridStrategy           (routes per-archetype)
//
// Hybrid agents resolve to their own per-archetype strategy inside hybridStrategy.
// Token usage folds inside the strategies via sc.LLM.
func selectStrategy(strat models.LLMStrategy, agentType models.AgentType) ExecutionStrategy {
	switch strat.Type {
	case models.LLMStrategyFull:
		if agentType == models.AgentTypePlain {
			return individualFullStrategy{}
		}
		return archetypeFullStrategy{}
	case models.LLMStrategyDecision:
		if agentType == models.AgentTypePlain {
			return individualDecisionStrategy{}
		}
		return archetypeDecisionStrategy{}
	case models.LLMStrategyHybrid:
		return hybridStrategy{}
	default: // LLMStrategyNone and unknown
		return rulesOnlyStrategy{}
	}
}
