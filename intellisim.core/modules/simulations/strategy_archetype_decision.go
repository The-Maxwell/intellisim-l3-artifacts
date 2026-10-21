package simulations

import "intelli.sim.core/data/common"

// archetypeDecisionStrategy is the Strategy-5 (decision × archetype) execution
// strategy: for the decision purpose it asks the LLM for ONE high-level label PER
// ARCHETYPE (from the archetype's DecisionMenu), optionally batching ≤
// Strategy.BatchSize archetypes into one call (demuxed by archetype_uid). The
// single label is written to every member (recorded as LastDecision); per-agent
// rules + state then produce the behavioural divergence. Other enabled purposes
// (properties / movement) use the archetype directive + ApplyDirectiveWithVariation
// for per-agent variation. Capability for the decision purpose:
// ModelCapabilitySimulationDecisions.
type archetypeDecisionStrategy struct{}

func (archetypeDecisionStrategy) Name() string { return "archetype_decision" }

func (s archetypeDecisionStrategy) Plan(sc *StepContext, group *AgentGroup) (PlanResult, error) {
	if group == nil {
		return PlanResult{Intents: map[uint64]*AgentIntent{}}, nil
	}
	byArch := planArchetypeBatch(sc, []*AgentGroup{group}, s)
	if pr, ok := byArch[group.ArchetypeUID]; ok {
		return pr, nil
	}
	return PlanResult{Intents: map[uint64]*AgentIntent{}}, nil
}

func (archetypeDecisionStrategy) archetypeKind() archetypeKind { return archKindDecision }
func (archetypeDecisionStrategy) capability() common.ModelCapability {
	return common.ModelCapabilitySimulationDecisions
}
