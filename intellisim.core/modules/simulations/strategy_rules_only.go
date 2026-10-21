package simulations

// rulesOnlyStrategy is the Strategy-1 (LLMStrategyNone) execution strategy: it
// invokes no LLM and emits no intents. The rules and interactions phases drive
// every agent.
type rulesOnlyStrategy struct{}

func (rulesOnlyStrategy) Name() string { return "rules_only" }

func (rulesOnlyStrategy) Plan(sc *StepContext, group *AgentGroup) (PlanResult, error) {
	return PlanResult{Intents: map[uint64]*AgentIntent{}}, nil
}
