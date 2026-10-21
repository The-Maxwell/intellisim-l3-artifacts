package models

import "testing"

// TestLLMStrategyCanonical verifies that Canonical forces the four per-step purpose
// flags to the values mandated by Type — only "hybrid" keeps user-supplied flags —
// while carrying BatchSize/DecisionFrequency through unchanged.
func TestLLMStrategyCanonical(t *testing.T) {
	// All four flags set on input, so we can see which Type clears/keeps them.
	base := LLMStrategy{
		BatchSize:         7,
		DecisionFrequency: 3,
		UseForMovement:    true,
		UseForInteraction: true,
		UseForProperties:  true,
		UseForDecisions:   true,
		ArchetypeMode:     ArchetypeModeDistribution,
		VariationStrength: 0.4,
	}

	cases := []struct {
		name                                       string
		typ                                        LLMStrategyType
		movement, interaction, properties, decision bool
	}{
		{"none", LLMStrategyNone, false, false, false, false},
		{"empty", LLMStrategyType(""), false, false, false, false},
		{"full", LLMStrategyFull, true, true, true, true},
		{"decision", LLMStrategyDecision, false, false, false, true},
		{"hybrid keeps flags", LLMStrategyHybrid, true, true, true, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := base
			s.Type = c.typ
			got := s.Canonical()
			if got.UseForMovement != c.movement ||
				got.UseForInteraction != c.interaction ||
				got.UseForProperties != c.properties ||
				got.UseForDecisions != c.decision {
				t.Errorf("Canonical(%q) flags = m%v i%v p%v d%v, want m%v i%v p%v d%v",
					c.typ, got.UseForMovement, got.UseForInteraction, got.UseForProperties, got.UseForDecisions,
					c.movement, c.interaction, c.properties, c.decision)
			}
			if got.BatchSize != 7 || got.DecisionFrequency != 3 {
				t.Errorf("Canonical(%q) dropped BatchSize/DecisionFrequency: %+v", c.typ, got)
			}
			if got.ArchetypeMode != ArchetypeModeDistribution || got.VariationStrength != 0.4 {
				t.Errorf("Canonical(%q) dropped ArchetypeMode/VariationStrength: %+v", c.typ, got)
			}
			if got.Type != c.typ {
				t.Errorf("Canonical(%q) changed Type to %q", c.typ, got.Type)
			}
		})
	}
}

// TestLLMStrategyCanonical_HybridZeroFlags confirms a hybrid with no enabled flags
// stays a no-LLM (rules-only) strategy after canonicalization.
func TestLLMStrategyCanonical_HybridZeroFlags(t *testing.T) {
	s := LLMStrategy{Type: LLMStrategyHybrid}.Canonical()
	if s.UsesLLM() {
		t.Errorf("hybrid with zero flags should not use LLM: %+v", s)
	}
}

// TestNormalizeLLMStrategies verifies the model-wide walk canonicalizes the default
// strategy plus every per-agent and per-shape override (the common default AND the
// user-specific overrides), leaving non-LLM agents/shapes untouched.
func TestNormalizeLLMStrategies(t *testing.T) {
	m := &Model{
		SimulationConfig: SimulationConfig{
			LLMSimulationExecutionConfig: LLMSimulationExecutionConfig{
				// "full" default must end up with all four flags on.
				DefaultStrategy: LLMStrategy{Type: LLMStrategyFull},
			},
		},
		Agents: []Agent{
			{ // decision override with stray non-decision flags → cleared
				LLMConfig: &LLMAgentConfig{
					Strategy: LLMAgentStrategy{LLMStrategy: LLMStrategy{
						Type:           LLMStrategyDecision,
						UseForMovement: true,
						UseForDecisions: true,
					}},
				},
			},
			{LLMConfig: nil}, // no override → no panic
		},
		Environment: &Environment{
			Shapes: []Shape{
				{ // none override with stray flags → all cleared
					LLMConfig: &LLMShapeConfig{
						Strategy: LLMShapeStrategy{LLMStrategy: LLMStrategy{
							Type:             LLMStrategyNone,
							UseForProperties: true,
						}},
					},
				},
			},
		},
	}

	m.NormalizeLLMStrategies()

	def := m.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy
	if !(def.UseForMovement && def.UseForInteraction && def.UseForProperties && def.UseForDecisions) {
		t.Errorf("default full strategy not fully enabled: %+v", def)
	}

	a := m.Agents[0].LLMConfig.Strategy.LLMStrategy
	if a.UseForMovement || a.UseForInteraction || a.UseForProperties || !a.UseForDecisions {
		t.Errorf("agent decision strategy not canonical (decisions only): %+v", a)
	}

	sh := m.Environment.Shapes[0].LLMConfig.Strategy.LLMStrategy
	if sh.UseForMovement || sh.UseForInteraction || sh.UseForProperties || sh.UseForDecisions {
		t.Errorf("shape none strategy not cleared: %+v", sh)
	}
}

// TestNormalizeLLMStrategies_DoesNotStampArchetypeMode confirms that the generic
// normalize/canonical path (run on every save, including UpdateModel) leaves an empty
// ArchetypeMode empty — so an existing model with no mode keeps resolving to identical
// at runtime and is never silently flipped to distribution by a plain update.
func TestNormalizeLLMStrategies_DoesNotStampArchetypeMode(t *testing.T) {
	m := &Model{
		SimulationConfig: SimulationConfig{
			LLMSimulationExecutionConfig: LLMSimulationExecutionConfig{
				DefaultStrategy: LLMStrategy{Type: LLMStrategyFull}, // ArchetypeMode empty
			},
		},
		Agents: []Agent{{
			LLMConfig: &LLMAgentConfig{
				Strategy: LLMAgentStrategy{LLMStrategy: LLMStrategy{Type: LLMStrategyDecision}},
			},
		}},
	}

	m.NormalizeLLMStrategies()

	if got := m.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy.ArchetypeMode; got != "" {
		t.Errorf("normalize flipped empty default ArchetypeMode to %q (must stay empty)", got)
	}
	if got := m.Agents[0].LLMConfig.Strategy.ArchetypeMode; got != "" {
		t.Errorf("normalize flipped empty agent ArchetypeMode to %q (must stay empty)", got)
	}
}

// TestStampDefaultArchetypeMode verifies the create/generation default-stamp: empty
// modes on the execution default + per-agent overrides become distribution, an
// explicit mode is preserved, a nil per-agent config does not panic, and per-shape
// strategies are intentionally left untouched.
func TestStampDefaultArchetypeMode(t *testing.T) {
	m := &Model{
		SimulationConfig: SimulationConfig{
			LLMSimulationExecutionConfig: LLMSimulationExecutionConfig{
				DefaultStrategy: LLMStrategy{Type: LLMStrategyFull}, // empty → distribution
			},
		},
		Agents: []Agent{
			{LLMConfig: &LLMAgentConfig{ // empty → distribution
				Strategy: LLMAgentStrategy{LLMStrategy: LLMStrategy{Type: LLMStrategyFull}},
			}},
			{LLMConfig: &LLMAgentConfig{ // explicit identical → preserved
				Strategy: LLMAgentStrategy{LLMStrategy: LLMStrategy{Type: LLMStrategyDecision, ArchetypeMode: ArchetypeModeIdentical}},
			}},
			{LLMConfig: nil}, // no override → no panic
		},
		Environment: &Environment{
			Shapes: []Shape{{
				LLMConfig: &LLMShapeConfig{
					Strategy: LLMShapeStrategy{LLMStrategy: LLMStrategy{Type: LLMStrategyFull}},
				},
			}},
		},
	}

	m.StampDefaultArchetypeMode()

	if got := m.SimulationConfig.LLMSimulationExecutionConfig.DefaultStrategy.ArchetypeMode; got != ArchetypeModeDistribution {
		t.Errorf("default ArchetypeMode = %q, want distribution", got)
	}
	if got := m.Agents[0].LLMConfig.Strategy.ArchetypeMode; got != ArchetypeModeDistribution {
		t.Errorf("agent[0] (empty) ArchetypeMode = %q, want distribution", got)
	}
	if got := m.Agents[1].LLMConfig.Strategy.ArchetypeMode; got != ArchetypeModeIdentical {
		t.Errorf("agent[1] (explicit) ArchetypeMode = %q, want identical (preserved)", got)
	}
	if got := m.Environment.Shapes[0].LLMConfig.Strategy.ArchetypeMode; got != "" {
		t.Errorf("shape ArchetypeMode = %q, want empty (shapes untouched)", got)
	}
}
