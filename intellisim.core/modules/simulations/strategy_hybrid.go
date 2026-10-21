package simulations

import "intelli.sim.core/data/models"

// hybridStrategy is the Strategy-6 (hybrid) execution strategy: each archetype runs
// under its OWN configured strategy. Per group it resolves the archetype's concrete
// strategy (the archetype's own LLMConfig.Strategy when set, else the model's
// DefaultStrategy) and delegates the group to that strategy's Plan. A `none`
// archetype routes to rules_only (no LLM); an LLM-driven archetype routes to the
// matching individual/archetype strategy. Token usage folds inside the delegate
// via sc.LLM.
type hybridStrategy struct{}

func (hybridStrategy) Name() string { return "hybrid" }

// Plan resolves the group's per-archetype strategy and delegates. The delegate
// reads group.Strategy for its flags/batch size/frequency, so Plan also overlays
// the resolved per-archetype strategy onto the group before delegating (the
// model-level "hybrid" placeholder must not leak into the delegate).
func (hybridStrategy) Plan(sc *StepContext, group *AgentGroup) (PlanResult, error) {
	empty := PlanResult{Intents: map[uint64]*AgentIntent{}}
	if sc == nil || sc.RS == nil || group == nil || group.Archetype == nil {
		return empty, nil
	}

	resolved := resolveArchetypeStrategy(sc.RS.Model, group.Archetype)
	delegate := selectStrategy(resolved, group.Archetype.Type)

	// Guard against a degenerate self-route (e.g. a per-archetype strategy that
	// itself resolves to hybrid): never recurse into hybrid — degrade to rules.
	if _, isHybrid := delegate.(hybridStrategy); isHybrid {
		return empty, nil
	}

	// Delegate strategies read group.Strategy; hand them the resolved per-archetype
	// strategy so the model-level hybrid placeholder never reaches them.
	sub := *group
	sub.Strategy = resolved
	return delegate.Plan(sc, &sub)
}

// resolveArchetypeStrategy returns the concrete LLMStrategy for one archetype under
// a hybrid model: the archetype's own LLMConfig.Strategy when its Type is set
// (non-empty), else the model's DefaultStrategy. A hybrid model means "archetypes
// define their own", so the default itself being hybrid degrades to none for any
// archetype that did not override it.
func resolveArchetypeStrategy(model *models.Model, arch *models.Agent) models.LLMStrategy {
	if arch != nil && arch.LLMConfig != nil && arch.LLMConfig.Strategy.Type != "" {
		return arch.LLMConfig.Strategy.LLMStrategy
	}
	if model != nil && model.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy.Type != "" &&
		model.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy.Type != models.LLMStrategyHybrid {
		return model.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy
	}
	return models.LLMStrategy{Type: models.LLMStrategyNone}
}
