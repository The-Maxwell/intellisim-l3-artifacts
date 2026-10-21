package models

import "intelli.sim.core/data/common"

// UsesLLM reports whether this strategy actually invokes the LLM for any per-step
// runtime purpose (movement, properties, decisions, interactions). A strategy with a
// set Type but no enabled purpose runs entirely on rules. Spawn/Population are
// generation-time concerns and are intentionally excluded here.
func (s LLMStrategy) UsesLLM() bool {
	if s.Type == LLMStrategyNone || s.Type == "" {
		return false
	}
	return s.UseForMovement || s.UseForProperties || s.UseForDecisions || s.UseForInteraction
}

// RequiredCapability maps a resolved strategy type to the LLM capability its runtime
// calls require: full control needs simulation_execution; decision control needs
// simulation_decisions. Hybrid/unknown defaults to the (narrower) decision capability.
func (s LLMStrategy) RequiredCapability() common.ModelCapability {
	if s.Type == LLMStrategyFull {
		return common.ModelCapabilitySimulationExecution
	}
	return common.ModelCapabilitySimulationDecisions
}

// UsesLLMForGeneration reports whether the simulation generator will issue any LLM call
// when seeding a simulation from this model — i.e. whether any agent/shape resolves its
// population count, placement, or a property value via the LLM. When true, the model's
// SimulationConfig.LLMSimulationGenerationConfig.Target must be set and capable of
// simulation_generation.
func (m *Model) UsesLLMForGeneration() bool {
	if m == nil {
		return false
	}
	if m.Environment != nil && propertiesUseLLMGen(m.Environment.GlobalProperties) {
		return true
	}
	for i := range m.Agents {
		if initUsesLLMGen(m.Agents[i].Initialization) || propertiesUseLLMGen(m.Agents[i].Properties) {
			return true
		}
	}
	if m.Environment != nil {
		for i := range m.Environment.Shapes {
			sh := &m.Environment.Shapes[i]
			if initUsesLLMGen(sh.Initialization) || propertiesUseLLMGen(sh.Properties) {
				return true
			}
		}
	}
	return false
}

func initUsesLLMGen(init AgentInitialization) bool {
	if init.CountMode == CountLLM {
		return true
	}
	return init.Placement != nil && init.Placement.Type == PlacementLLM
}

func propertiesUseLLMGen(props []AgentProperty) bool {
	for i := range props {
		if props[i].Generation != nil && props[i].Generation.Mode == GenModeLLM {
			return true
		}
	}
	return false
}

// NormalizeLLMStrategies canonicalizes every runtime LLM strategy on the model (the
// execution default strategy plus every per-agent and per-shape override) so the persisted
// UseFor* purpose flags always match the strategy Type (see LLMStrategy.Canonical). Call
// it before validating/persisting a model so storage, validation and the run engine all
// observe the same canonical flags. It is applied uniformly to the common default strategy
// and the user-specific overrides.
func (m *Model) NormalizeLLMStrategies() {
	if m == nil {
		return
	}
	exec := &m.SimulationConfig.LLMSimulationExecutionConfig
	exec.DefaultStrategy = exec.DefaultStrategy.Canonical()
	for i := range m.Agents {
		if a := m.Agents[i].LLMConfig; a != nil {
			a.Strategy.LLMStrategy = a.Strategy.LLMStrategy.Canonical()
		}
	}
	if m.Environment != nil {
		for i := range m.Environment.Shapes {
			if sh := m.Environment.Shapes[i].LLMConfig; sh != nil {
				sh.Strategy.LLMStrategy = sh.Strategy.LLMStrategy.Canonical()
			}
		}
	}
}

// StampDefaultArchetypeMode sets ArchetypeMode = distribution on the model's
// execution default strategy and every per-agent override whose ArchetypeMode is
// still empty, so NEW models default to per-agent distribution sampling for
// archetype agents. It is applied ONLY in model creation/generation paths (never on a
// generic update, and never inside Canonical/NormalizeLLMStrategies which run on every
// save) — so existing persisted models with an empty mode keep resolving to identical
// at runtime (backward-safe). Per-shape strategies are intentionally left untouched:
// the per-shape LLM phase does not use archetype-style fan-out.
func (m *Model) StampDefaultArchetypeMode() {
	if m == nil {
		return
	}
	exec := &m.SimulationConfig.LLMSimulationExecutionConfig
	if exec.DefaultStrategy.ArchetypeMode == "" {
		exec.DefaultStrategy.ArchetypeMode = ArchetypeModeDistribution
	}
	for i := range m.Agents {
		if a := m.Agents[i].LLMConfig; a != nil && a.Strategy.ArchetypeMode == "" {
			a.Strategy.ArchetypeMode = ArchetypeModeDistribution
		}
	}
}

// UsesLLMForExecution reports whether running a simulation from this model will invoke
// the LLM for any per-step purpose — i.e. the model default strategy or any per-agent /
// per-shape strategy override uses the LLM. When true, the model's
// SimulationConfig.LLMSimulationExecutionConfig.Target must be set and capable of every
// capability returned by RequiredExecutionCapabilities.
func (m *Model) UsesLLMForExecution() bool {
	if m == nil {
		return false
	}
	if m.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy.UsesLLM() {
		return true
	}
	for i := range m.Agents {
		if a := m.Agents[i].LLMConfig; a != nil && a.Strategy.UsesLLM() {
			return true
		}
	}
	if m.Environment != nil {
		for i := range m.Environment.Shapes {
			if sh := m.Environment.Shapes[i].LLMConfig; sh != nil && sh.Strategy.UsesLLM() {
				return true
			}
		}
	}
	return false
}

// RequiredExecutionCapabilities is the deduplicated set of LLM capabilities the model's
// execution-default target must satisfy, derived from every LLM-using strategy (model
// default + per-agent + per-shape). A model mixing full and decision strategies requires
// an execution model advertising both. Empty when execution uses no LLM.
func (m *Model) RequiredExecutionCapabilities() []common.ModelCapability {
	if m == nil {
		return nil
	}
	seen := map[common.ModelCapability]bool{}
	var out []common.ModelCapability
	add := func(s LLMStrategy) {
		if !s.UsesLLM() {
			return
		}
		caps := []common.ModelCapability{s.RequiredCapability()}
		if s.Type == LLMStrategyHybrid {
			// Hybrid agents may use either control mode; require both.
			caps = []common.ModelCapability{
				common.ModelCapabilitySimulationExecution,
				common.ModelCapabilitySimulationDecisions,
			}
		}
		for _, c := range caps {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	add(m.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy)
	for i := range m.Agents {
		if a := m.Agents[i].LLMConfig; a != nil {
			add(a.Strategy.LLMStrategy)
		}
	}
	if m.Environment != nil {
		for i := range m.Environment.Shapes {
			if sh := m.Environment.Shapes[i].LLMConfig; sh != nil {
				add(sh.Strategy.LLMStrategy)
			}
		}
	}
	return out
}
