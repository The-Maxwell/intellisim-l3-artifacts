package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

// ===================== test helpers =====================

func findAgent(m datamodels.Model, uid string) *datamodels.Agent {
	for i := range m.Agents {
		if m.Agents[i].UID == uid {
			return &m.Agents[i]
		}
	}
	return nil
}

func findRule(m datamodels.Model, id string) *datamodels.Rule {
	for i := range m.Rules {
		if m.Rules[i].ID == id {
			return &m.Rules[i]
		}
	}
	return nil
}

func findInteraction(m datamodels.Model, id string) *datamodels.Interaction {
	for i := range m.Interactions {
		if m.Interactions[i].ID == id {
			return &m.Interactions[i]
		}
	}
	return nil
}

func findEvent(m datamodels.Model, uid string) *datamodels.EventDefinition {
	for i := range m.Events {
		if m.Events[i].UID == uid {
			return &m.Events[i]
		}
	}
	return nil
}

func findShape(m datamodels.Model, uid string) *datamodels.Shape {
	if m.Environment == nil {
		return nil
	}
	for i := range m.Environment.Shapes {
		if m.Environment.Shapes[i].UID == uid {
			return &m.Environment.Shapes[i]
		}
	}
	return nil
}

func intP(i int) *int    { return &i }
func boolP(b bool) *bool { return &b }
func uintP(u uint) *uint { return &u }

// hasWarning reports whether any warning mentions the given substring in either
// its path or its reason.
func hasWarning(warns []MergeWarning, substr string) bool {
	for _, w := range warns {
		if containsStr(w.Path, substr) || containsStr(w.Reason, substr) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ===================== tests =====================

func TestLLMMerge_PreservesLockedAgent(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{
		{UID: "predator", Name: "Predator", Locked: true},
		{UID: "prey", Name: "Prey"},
	}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{
		{UID: "predator", Name: "HACKED"}, // attempt to change locked
		{UID: "prey", Name: "Prey v2"},    // allowed
	}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	require.Equal(t, "Predator", findAgent(merged, "predator").Name) // locked preserved
	require.Equal(t, "Prey v2", findAgent(merged, "prey").Name)
	require.True(t, hasWarning(warns, "predator")) // emitted a "locked; ignored" warning
	require.True(t, hasWarning(warns, "locked"))

	// Input must not be mutated.
	require.Equal(t, "Predator", existing.Agents[0].Name)
	require.Equal(t, "Prey", existing.Agents[1].Name)
}

func TestLLMMerge_ScopeLimitsToSection(t *testing.T) {
	existing := datamodels.Model{
		Agents: []datamodels.Agent{{UID: "a", Name: "A"}},
		Rules:  []datamodels.Rule{{ID: "r1", Name: "R1"}},
	}
	env := LLMGenerationEnvelope{Model: &LLMModel{
		Agents: []LLMAgent{{UID: "a", Name: "SHOULD-NOT-APPLY"}},
		Rules:  []LLMRule{{ID: "r1", Name: "R1-updated"}},
	}}

	merged, _ := llmMerge(existing, env, EditScope{Section: "rules"})

	require.Equal(t, "A", findAgent(merged, "a").Name) // agents untouched (out of scope)
	require.Equal(t, "R1-updated", findRule(merged, "r1").Name)
}

func TestLLMMerge_NeverTouchesUserOnlyFields(t *testing.T) {
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}
	existing := datamodels.Model{
		LLMConfig: datamodels.LLMConfig{Target: target},
		Agents:    []datamodels.Agent{{UID: "a", LLMConfig: &datamodels.LLMAgentConfig{LLMConfig: datamodels.LLMConfig{Target: target}}}},
	}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{UID: "a", Name: "renamed"}}}}

	merged, _ := llmMerge(existing, env, EditScope{})

	require.Equal(t, target, merged.LLMConfig.Target)           // model target preserved
	require.NotNil(t, merged.Agents[0].LLMConfig)               // agent config not dropped
	require.Equal(t, target, merged.Agents[0].LLMConfig.Target) // agent target preserved
	require.Equal(t, "renamed", merged.Agents[0].Name)
}

func TestLLMMerge_MalformedFieldKeepsPrevious(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{
		{UID: "a", Type: "agent", Initialization: datamodels.AgentInitialization{PopulationCount: 5}},
	}}
	// LLM sends an invalid type -> keep previous; the bad field is skipped, not panicked.
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{UID: "a", Type: "not-a-type"}}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	require.Equal(t, datamodels.AgentType("agent"), findAgent(merged, "a").Type) // invalid -> previous kept
	require.NotEmpty(t, warns)
	require.True(t, hasWarning(warns, "type"))
}

func TestLLMMerge_DoesNotAliasIncomingMaps(t *testing.T) {
	// An action's Parameters map in the incoming envelope must not be shared with the merged model.
	env := LLMGenerationEnvelope{Model: &LLMModel{Rules: []LLMRule{{
		ID: "r1", Name: "R", Actions: []LLMAction{{Type: "custom", Parameters: map[string]interface{}{"k": "v"}}},
	}}}}
	merged, _ := llmMerge(datamodels.Model{}, env, EditScope{})
	// mutate the envelope's map after merge:
	env.Model.Rules[0].Actions[0].Parameters["k"] = "MUTATED"
	got := findRule(merged, "r1").Actions[0].Parameters["k"]
	require.Equal(t, "v", got, "merged model must not alias the envelope's Parameters map")
}

func TestLLMMerge_RetainsEntitiesAbsentFromIncoming(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{
		{UID: "a", Name: "A"}, {UID: "b", Name: "B"},
	}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{UID: "a", Name: "A2", Type: "agent"}}}}
	merged, _ := llmMerge(existing, env, EditScope{})
	require.Equal(t, "A2", findAgent(merged, "a").Name) // updated
	require.NotNil(t, findAgent(merged, "b"))           // retained, not deleted
	require.Equal(t, "B", findAgent(merged, "b").Name)
}

// ===================== field-level retention (partial edits) =====================

// TestLLMMerge_PartialRuleUpdate_PreservesOmittedFields is the live repro: the LLM
// emits only {id, priority}; every omitted field must keep its current value.
func TestLLMMerge_PartialRuleUpdate_PreservesOmittedFields(t *testing.T) {
	existing := datamodels.Model{Rules: []datamodels.Rule{{
		ID:            "energy_decay_rule",
		Name:          "Energy Decay",
		Description:   "Drains energy each step",
		Priority:      3,
		AppliesTo:     []string{"fox", "wolf", "rabbit"},
		Conditions:    []datamodels.Condition{{Type: datamodels.ConditionTypeStep}},
		Actions:       []datamodels.Action{{Type: datamodels.ActionTypeModifyProperty, PropertyUID: "energy"}},
		ExecuteOnce:   false,
		CooldownSteps: 2,
		Enabled:       true,
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Rules: []LLMRule{{
		ID: "energy_decay_rule", Priority: llmIPtr(9),
	}}}}

	merged, _ := llmMerge(existing, env, EditScope{Section: "rules", UID: "energy_decay_rule"})
	r := findRule(merged, "energy_decay_rule")
	require.NotNil(t, r)
	require.Equal(t, 9, r.Priority)                                  // the only change
	require.Equal(t, "Energy Decay", r.Name)                         // kept
	require.Equal(t, "Drains energy each step", r.Description)       // kept
	require.True(t, r.Enabled)                                       // kept (not reset to false)
	require.Equal(t, []string{"fox", "wolf", "rabbit"}, r.AppliesTo) // kept (not nil)
	require.Len(t, r.Conditions, 1)                                  // kept (not 0)
	require.Len(t, r.Actions, 1)                                     // kept (not 0)
	require.False(t, r.ExecuteOnce)                                  // kept
	require.Equal(t, 2, r.CooldownSteps)                             // kept
}

// TestLLMMerge_ExplicitEnabledFalseApplies asserts an explicit zero value is NOT
// treated as absent — "enabled": false on an update disables the rule.
func TestLLMMerge_ExplicitEnabledFalseApplies(t *testing.T) {
	existing := datamodels.Model{Rules: []datamodels.Rule{{ID: "r1", Name: "R1", Enabled: true, Priority: 5}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Rules: []LLMRule{{ID: "r1", Enabled: llmBPtr(false)}}}}

	merged, _ := llmMerge(existing, env, EditScope{})
	r := findRule(merged, "r1")
	require.False(t, r.Enabled)     // explicit false applied
	require.Equal(t, 5, r.Priority) // omitted, kept
	require.Equal(t, "R1", r.Name)  // omitted, kept
}

// TestLLMMerge_EmptyArrayClearsList_NilKeeps covers the slice contract on update.
func TestLLMMerge_EmptyArrayClearsList_NilKeeps(t *testing.T) {
	existing := datamodels.Model{Rules: []datamodels.Rule{{
		ID:         "r1",
		AppliesTo:  []string{"a", "b"},
		Conditions: []datamodels.Condition{{Type: datamodels.ConditionTypeStep}},
	}}}

	// Explicit empty array clears applies_to; nil conditions keep the previous list.
	env := LLMGenerationEnvelope{Model: &LLMModel{Rules: []LLMRule{{
		ID: "r1", AppliesTo: []string{},
	}}}}
	merged, _ := llmMerge(existing, env, EditScope{})
	r := findRule(merged, "r1")
	require.Empty(t, r.AppliesTo)   // explicit [] cleared the list
	require.Len(t, r.Conditions, 1) // nil kept the previous conditions
}

// TestLLMMerge_NewRuleEnabledDefaultsTrue: a created rule with enabled absent is active.
func TestLLMMerge_NewRuleEnabledDefaultsTrue(t *testing.T) {
	env := LLMGenerationEnvelope{Model: &LLMModel{Rules: []LLMRule{{
		ID: "new_rule", Name: "New", // enabled, execute_once, priority all absent
	}}}}
	merged, _ := llmMerge(datamodels.Model{}, env, EditScope{})
	r := findRule(merged, "new_rule")
	require.NotNil(t, r)
	require.True(t, r.Enabled)      // defaults true for a new rule
	require.False(t, r.ExecuteOnce) // other bools default false
	require.Equal(t, 0, r.Priority)
}

// TestLLMMerge_PartialInteractionUpdate_PreservesOmittedFields mirrors the rule case.
func TestLLMMerge_PartialInteractionUpdate_PreservesOmittedFields(t *testing.T) {
	existing := datamodels.Model{Interactions: []datamodels.Interaction{{
		ID:            "hunt",
		Name:          "Hunt",
		Type:          datamodels.InteractionTypeAgentAgent,
		InitiatorUIDs: []string{"fox"},
		TargetUIDs:    []string{"rabbit"},
		Bidirectional: true,
		UseLLM:        true,
		Enabled:       true,
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Interactions: []LLMInteraction{{
		ID: "hunt", Enabled: llmBPtr(false),
	}}}}

	merged, _ := llmMerge(existing, env, EditScope{})
	it := findInteraction(merged, "hunt")
	require.False(t, it.Enabled)                                    // explicit false applied
	require.Equal(t, "Hunt", it.Name)                               // kept
	require.Equal(t, datamodels.InteractionTypeAgentAgent, it.Type) // kept
	require.Equal(t, []string{"fox"}, it.InitiatorUIDs)             // kept
	require.Equal(t, []string{"rabbit"}, it.TargetUIDs)             // kept
	require.True(t, it.Bidirectional)                               // kept
	require.True(t, it.UseLLM)                                      // kept
}

// TestLLMMerge_NewInteractionEnabledDefaultsTrue: a created interaction is active.
func TestLLMMerge_NewInteractionEnabledDefaultsTrue(t *testing.T) {
	env := LLMGenerationEnvelope{Model: &LLMModel{Interactions: []LLMInteraction{{
		ID: "graze", Type: "agent_shape",
	}}}}
	merged, _ := llmMerge(datamodels.Model{}, env, EditScope{})
	it := findInteraction(merged, "graze")
	require.NotNil(t, it)
	require.True(t, it.Enabled)        // defaults true
	require.False(t, it.Bidirectional) // other bools default false
	require.False(t, it.UseLLM)
}

// TestLLMMerge_PartialEventUpdate_PreservesOmittedFields covers events.
func TestLLMMerge_PartialEventUpdate_PreservesOmittedFields(t *testing.T) {
	existing := datamodels.Model{Events: []datamodels.EventDefinition{{
		UID:          "alarm",
		Name:         "Alarm",
		Scope:        datamodels.EventScopeLocal,
		Producers:    []datamodels.EventProducer{datamodels.EventProducerAgent},
		ProducerUIDs: []string{"fox"},
		TTLSteps:     5,
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Events: []LLMEvent{{
		UID: "alarm", TTLSteps: llmUPtr(9),
	}}}}

	merged, _ := llmMerge(existing, env, EditScope{})
	e := findEvent(merged, "alarm")
	require.Equal(t, uint(9), e.TTLSteps)                                                    // changed
	require.Equal(t, "Alarm", e.Name)                                                        // kept
	require.Equal(t, datamodels.EventScopeLocal, e.Scope)                                    // kept (not reset)
	require.Equal(t, []datamodels.EventProducer{datamodels.EventProducerAgent}, e.Producers) // kept
	require.Equal(t, []string{"fox"}, e.ProducerUIDs)                                        // kept
}

// TestLLMMerge_PartialAgentUpdate_PreservesPopulationAndFields covers the agent
// path including initialization.population_count being kept when omitted.
func TestLLMMerge_PartialAgentUpdate_PreservesPopulationAndFields(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{{
		UID:      "fox",
		Name:     "Fox",
		Type:     datamodels.AgentTypePlain,
		MoveType: datamodels.AgentMoveTypeMovable,
		States:   []string{"alive", "dead"},
		Initialization: datamodels.AgentInitialization{
			PopulationCount: 40,
			InitialState:    "alive",
		},
	}}}
	// LLM changes only the name; population_count and everything else omitted.
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{
		UID: "fox", Name: "Red Fox",
	}}}}

	merged, _ := llmMerge(existing, env, EditScope{})
	a := findAgent(merged, "fox")
	require.Equal(t, "Red Fox", a.Name)                            // changed
	require.Equal(t, datamodels.AgentTypePlain, a.Type)            // kept
	require.Equal(t, datamodels.AgentMoveTypeMovable, a.MoveType)  // kept
	require.Equal(t, []string{"alive", "dead"}, a.States)          // kept (nil states)
	require.Equal(t, uint64(40), a.Initialization.PopulationCount) // kept (absent count)
	require.Equal(t, "alive", a.Initialization.InitialState)       // kept
}

// ===================== property generation (fail-soft) =====================

func TestLLMMerge_GenerationApplied(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{{
		UID: "fox", Type: datamodels.AgentTypePlain,
		Properties: []datamodels.AgentProperty{{UID: "energy", Type: datamodels.PropertyTypeNumber}},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{
		UID: "fox",
		Properties: []LLMAgentProperty{{UID: "energy", Generation: &LLMPropertyGeneration{
			Mode: "random", Distribution: "normal", Mean: f64(50), StdDev: f64(10),
		}}},
	}}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	g := findAgent(merged, "fox").Properties[0].Generation
	require.NotNil(t, g)
	require.Equal(t, datamodels.GenModeRandom, g.Mode)
	require.Equal(t, datamodels.DistNormal, g.Distribution)
	require.Equal(t, 50.0, *g.Mean)
	require.Equal(t, 10.0, *g.StdDev)
	require.Empty(t, warns)
}

func TestLLMMerge_GenerationInvalidModeKeepsPreviousOnUpdate(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{{
		UID: "fox", Type: datamodels.AgentTypePlain,
		Properties: []datamodels.AgentProperty{{
			UID: "energy", Type: datamodels.PropertyTypeNumber,
			Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeDefault},
		}},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{
		UID: "fox",
		Properties: []LLMAgentProperty{{UID: "energy", Generation: &LLMPropertyGeneration{
			Mode: "bogus", PromptContext: "hint",
		}}},
	}}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	g := findAgent(merged, "fox").Properties[0].Generation
	require.NotNil(t, g)
	require.Equal(t, datamodels.GenModeDefault, g.Mode) // invalid -> previous kept
	require.Equal(t, "hint", g.PromptContext)           // valid field still applied
	require.True(t, hasWarning(warns, "generation.mode"))
}

func TestLLMMerge_GenerationInvalidModeDroppedOnCreate(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{{
		UID: "fox", Type: datamodels.AgentTypePlain,
		Properties: []datamodels.AgentProperty{{UID: "energy", Type: datamodels.PropertyTypeNumber}},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{
		UID:        "fox",
		Properties: []LLMAgentProperty{{UID: "energy", Generation: &LLMPropertyGeneration{Mode: "bogus"}}},
	}}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	require.Nil(t, findAgent(merged, "fox").Properties[0].Generation) // no previous -> spec dropped
	require.True(t, hasWarning(warns, "generation.mode"))
}

func TestLLMMerge_GenerationInvalidDistributionKeepsPrevious(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{{
		UID: "fox", Type: datamodels.AgentTypePlain,
		Properties: []datamodels.AgentProperty{{
			UID: "energy", Type: datamodels.PropertyTypeNumber,
			Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeRandom, Distribution: datamodels.DistUniform},
		}},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{
		UID:        "fox",
		Properties: []LLMAgentProperty{{UID: "energy", Generation: &LLMPropertyGeneration{Distribution: "weird"}}},
	}}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	g := findAgent(merged, "fox").Properties[0].Generation
	require.NotNil(t, g)
	require.Equal(t, datamodels.DistUniform, g.Distribution) // invalid -> previous kept
	require.True(t, hasWarning(warns, "generation.distribution"))
}

// ===================== count mode (fail-soft) =====================

func TestLLMMerge_CountModeApplied(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{{
		UID: "fox", Type: datamodels.AgentTypePlain,
		Initialization: datamodels.AgentInitialization{PopulationCount: 5},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{
		UID: "fox",
		Initialization: LLMAgentInitialization{
			CountMode: "random_range", MinPopulation: llmU64Ptr(2), MaxPopulation: llmU64Ptr(8),
		},
	}}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	a := findAgent(merged, "fox")
	require.Equal(t, datamodels.CountRandomRange, a.Initialization.CountMode)
	require.Equal(t, uint64(2), *a.Initialization.MinPopulation)
	require.Equal(t, uint64(8), *a.Initialization.MaxPopulation)
	require.Equal(t, uint64(5), a.Initialization.PopulationCount) // omitted -> kept
	require.Empty(t, warns)
}

func TestLLMMerge_CountModeInvalidKeepsPrevious(t *testing.T) {
	existing := datamodels.Model{Agents: []datamodels.Agent{{
		UID: "fox", Type: datamodels.AgentTypePlain,
		Initialization: datamodels.AgentInitialization{PopulationCount: 5, CountMode: datamodels.CountLLM},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{
		UID:            "fox",
		Initialization: LLMAgentInitialization{CountMode: "bogus"},
	}}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	require.Equal(t, datamodels.CountLLM, findAgent(merged, "fox").Initialization.CountMode)
	require.True(t, hasWarning(warns, "count_mode"))
}

// ===================== widgets (fail-soft upsert) =====================

func findWidget(m datamodels.Model, id string) *common.WidgetConfig {
	for i := range m.Widgets {
		if m.Widgets[i].ID == id {
			return &m.Widgets[i]
		}
	}
	return nil
}

func TestLLMMerge_WidgetsUpsertByID(t *testing.T) {
	existing := datamodels.Model{Widgets: []common.WidgetConfig{{
		ID: "w1", Type: common.WidgetMonitor, Title: "Old title",
		Source:   &common.WidgetSource{Type: common.WidgetSourcePopulation},
		Position: common.WidgetPosition{X: 0, Y: 0, W: 4, H: 2},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Widgets: []LLMWidget{
		{ID: "w1", Title: "Population"},                                             // partial update: title only
		{ID: "w2", Type: "text_log", Title: "Events", LogKinds: []string{"events"}}, // new widget
	}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	require.Empty(t, warns)
	w1 := findWidget(merged, "w1")
	require.NotNil(t, w1)
	require.Equal(t, "Population", w1.Title)
	require.Equal(t, common.WidgetMonitor, w1.Type)                              // kept
	require.NotNil(t, w1.Source)                                                 // kept
	require.Equal(t, common.WidgetPosition{X: 0, Y: 0, W: 4, H: 2}, w1.Position) // kept

	w2 := findWidget(merged, "w2")
	require.NotNil(t, w2)
	require.Equal(t, common.WidgetTextLog, w2.Type)
	require.Equal(t, []common.WidgetLogKind{common.LogEvents}, w2.LogKinds)
}

func TestLLMMerge_WidgetInvalidSkippedOnCreate(t *testing.T) {
	// monitor without a source is structurally invalid -> skipped, warned.
	env := LLMGenerationEnvelope{Model: &LLMModel{Widgets: []LLMWidget{
		{ID: "bad", Type: "monitor", Title: "No source"},
	}}}

	merged, warns := llmMerge(datamodels.Model{}, env, EditScope{})

	require.Nil(t, findWidget(merged, "bad"))
	require.True(t, hasWarning(warns, "widgets[bad]"))
}

func TestLLMMerge_WidgetInvalidUpdateKeepsPrevious(t *testing.T) {
	existing := datamodels.Model{Widgets: []common.WidgetConfig{{
		ID: "w1", Type: common.WidgetMonitor, Title: "Monitor",
		Source: &common.WidgetSource{Type: common.WidgetSourcePopulation},
	}}}
	env := LLMGenerationEnvelope{Model: &LLMModel{Widgets: []LLMWidget{
		{ID: "w1", Type: "nonsense_type"},
	}}}

	merged, warns := llmMerge(existing, env, EditScope{})

	w1 := findWidget(merged, "w1")
	require.NotNil(t, w1)
	require.Equal(t, common.WidgetMonitor, w1.Type) // invalid update -> previous kept
	require.Equal(t, "Monitor", w1.Title)
	require.True(t, hasWarning(warns, "widgets[w1]"))
}

func TestLLMMerge_WidgetsNilKeepsExisting(t *testing.T) {
	existing := datamodels.Model{Widgets: []common.WidgetConfig{{
		ID: "w1", Type: common.WidgetMonitor, Title: "Monitor",
		Source: &common.WidgetSource{Type: common.WidgetSourcePopulation},
	}}}
	// Agents-only envelope: absent widgets list must not delete existing widgets.
	env := LLMGenerationEnvelope{Model: &LLMModel{Agents: []LLMAgent{{UID: "fox", Type: "agent"}}}}

	merged, _ := llmMerge(existing, env, EditScope{})

	require.NotNil(t, findWidget(merged, "w1"))
}

func TestLLMMerge_WidgetsScopeLimitsToSection(t *testing.T) {
	existing := datamodels.Model{
		Agents: []datamodels.Agent{{UID: "a", Name: "A"}},
		Widgets: []common.WidgetConfig{{
			ID: "w1", Type: common.WidgetMonitor, Title: "Old",
			Source: &common.WidgetSource{Type: common.WidgetSourcePopulation},
		}},
	}
	env := LLMGenerationEnvelope{Model: &LLMModel{
		Agents:  []LLMAgent{{UID: "a", Name: "SHOULD-NOT-APPLY"}},
		Widgets: []LLMWidget{{ID: "w1", Title: "New"}},
	}}

	merged, _ := llmMerge(existing, env, EditScope{Section: "widgets"})

	require.Equal(t, "A", findAgent(merged, "a").Name) // out of scope, untouched
	require.Equal(t, "New", findWidget(merged, "w1").Title)
}

// TestLLMMerge_PartialShapeUpdate_PreservesOmittedFields covers shapes.
func TestLLMMerge_PartialShapeUpdate_PreservesOmittedFields(t *testing.T) {
	w := uint64(10)
	existing := datamodels.Model{Environment: &datamodels.Environment{
		UID:    datamodels.EnvironmentUID,
		Width:  100,
		Height: 100,
		Shapes: []datamodels.Shape{{
			UID:      "lake",
			Name:     "Lake",
			Type:     datamodels.ShapeTypeRectangle,
			Layer:    2,
			Visible:  true,
			Geometry: &datamodels.ShapeGeometry{Width: &datamodels.ScalarSpec{Value: &w}},
		}},
	}}
	// LLM toggles visibility only.
	env := LLMGenerationEnvelope{Model: &LLMModel{Environment: &LLMEnvironment{
		Shapes: []LLMShape{{UID: "lake", Visible: llmBPtr(false)}},
	}}}

	merged, _ := llmMerge(existing, env, EditScope{Section: "environment", UID: "lake"})
	s := findShape(merged, "lake")
	require.NotNil(t, s)
	require.False(t, s.Visible)                             // changed
	require.Equal(t, "Lake", s.Name)                        // kept
	require.Equal(t, datamodels.ShapeTypeRectangle, s.Type) // kept
	require.Equal(t, uint(2), s.Layer)                      // kept (absent layer)
	require.NotNil(t, s.Geometry)                           // geometry kept
	require.NotNil(t, s.Geometry.Width)                     // nested geometry kept
	require.Equal(t, uint64(100), merged.Environment.Width) // env scalars untouched (shape scope)
}
