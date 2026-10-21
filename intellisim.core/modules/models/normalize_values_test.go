package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	datamodels "intelli.sim.core/data/models"
)

// modelForNormalize builds a minimal model with one "energy" number property
// and one "label" string property on agent "wolf", plus a "temperature" number
// global property — enough surface to exercise every resolution path.
func modelForNormalize() *datamodels.Model {
	return &datamodels.Model{
		Name: "normalize",
		Agents: []datamodels.Agent{
			{
				UID:  "wolf",
				Type: datamodels.AgentTypePlain,
				Properties: []datamodels.AgentProperty{
					{UID: "energy", Type: datamodels.PropertyTypeNumber},
					{UID: "label", Type: datamodels.PropertyTypeString},
				},
			},
		},
		Environment: &datamodels.Environment{
			UID: datamodels.EnvironmentUID,
			GlobalProperties: []datamodels.AgentProperty{
				{UID: "temperature", Type: datamodels.PropertyTypeNumber},
			},
		},
	}
}

func TestNormalizeRuleValues_NumericProperty_StringValueCoercedToFloat(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpGreaterThan, Value: "1"},
			},
			Actions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "42"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, float64(1), m.Rules[0].Conditions[0].Value)
	require.Equal(t, float64(42), m.Rules[0].Actions[0].Value)
}

func TestNormalizeRuleValues_StringProperty_KeepsString(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "label", Operator: datamodels.ConditionOpEquals, Value: "1"},
			},
			Actions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "label", Value: "1"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Rules[0].Conditions[0].Value, "a declared string property must never be coerced")
	require.Equal(t, "1", m.Rules[0].Actions[0].Value)
}

// TestNormalizeRuleValues_EnvGlobalTarget verifies the environment resolution
// path: a global_property condition (globals namespace) and a set_property
// action with TargetUID=="environment" (globals only, never the agent namespace).
func TestNormalizeRuleValues_EnvGlobalTarget(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{datamodels.EnvironmentUID},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: datamodels.ConditionOpGreaterThan, Value: "30"},
			},
			Actions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, TargetUID: datamodels.EnvironmentUID, PropertyUID: "temperature", Value: "99"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, float64(30), m.Rules[0].Conditions[0].Value)
	require.Equal(t, float64(99), m.Rules[0].Actions[0].Value)
}

// TestNormalizeRuleValues_CompositeRecursion verifies that a composite
// condition's sub-conditions are normalized too.
func TestNormalizeRuleValues_CompositeRecursion(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				{
					Type:          datamodels.ConditionTypeComposite,
					LogicOperator: datamodels.LogicOpAND,
					SubConditions: []datamodels.Condition{
						{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpGreaterThan, Value: "5"},
						{
							Type:          datamodels.ConditionTypeComposite,
							LogicOperator: datamodels.LogicOpOR,
							SubConditions: []datamodels.Condition{
								{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpLessThan, Value: "90"},
							},
						},
					},
				},
			},
		},
	}

	NormalizeRuleValues(m)

	top := m.Rules[0].Conditions[0]
	require.Equal(t, float64(5), top.SubConditions[0].Value)
	require.Equal(t, float64(90), top.SubConditions[1].SubConditions[0].Value)
}

// TestNormalizeRuleValues_NoPropertyResolvable_RandomStepCoerced verifies the
// fallback: when a condition's PropertyUID resolves to nothing (or the
// condition type has none at all), only random/step get coerced.
func TestNormalizeRuleValues_NoPropertyResolvable_RandomStepCoerced(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeRandom, Value: "0.5"},
				{Type: datamodels.ConditionTypeStep, Operator: datamodels.ConditionOpGreaterOrEq, Value: "10"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, float64(0.5), m.Rules[0].Conditions[0].Value)
	require.Equal(t, float64(10), m.Rules[0].Conditions[1].Value)
}

// TestNormalizeRuleValues_NoPropertyResolvable_OtherTypesUntouched verifies the
// explicit concern behind the fallback scoping: decision/state labels (and any
// other condition type without a resolvable property) must NEVER be coerced,
// even when they happen to look numeric.
func TestNormalizeRuleValues_NoPropertyResolvable_OtherTypesUntouched(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeDecision, Value: "1"},
				{Type: datamodels.ConditionTypeState, Value: "1"},
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "ghost-property", Value: "1"}, // unresolvable property
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Rules[0].Conditions[0].Value, "decision labels must stay strings")
	require.Equal(t, "1", m.Rules[0].Conditions[1].Value, "state labels must stay strings")
	require.Equal(t, "1", m.Rules[0].Conditions[2].Value, "an unresolvable property must not be coerced")
}

func TestNormalizeRuleValues_NilModel_NoPanic(t *testing.T) {
	NormalizeRuleValues(nil)
}

// modelForNormalizeConflict builds a model with two archetypes that declare the
// SAME PropertyUID ("energy") with DIFFERENT types: "wolf" as number, "sheep" as
// string. This is the scenario behind the applies_to scoping bug: a single
// global PropertyUID->type map (first-declaration-wins) would resolve "energy"
// the same way for every rule regardless of which archetype it actually applies
// to.
func modelForNormalizeConflict() *datamodels.Model {
	return &datamodels.Model{
		Name: "normalize-conflict",
		Agents: []datamodels.Agent{
			{
				UID:  "wolf",
				Type: datamodels.AgentTypePlain,
				Properties: []datamodels.AgentProperty{
					{UID: "energy", Type: datamodels.PropertyTypeNumber},
				},
			},
			{
				UID:  "sheep",
				Type: datamodels.AgentTypePlain,
				Properties: []datamodels.AgentProperty{
					{UID: "energy", Type: datamodels.PropertyTypeString},
				},
			},
		},
	}
}

// TestNormalizeRuleValues_AppliesToScopesPropertyType_StringTypedArchetype
// verifies a rule scoped ONLY to the string-typed archetype ("sheep") keeps its
// string value even though another archetype ("wolf") declares the same
// PropertyUID as a number.
func TestNormalizeRuleValues_AppliesToScopesPropertyType_StringTypedArchetype(t *testing.T) {
	m := modelForNormalizeConflict()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"sheep"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpEquals, Value: "1"},
			},
			Actions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "1"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Rules[0].Conditions[0].Value, "sheep's energy is string-typed; must not be coerced")
	require.Equal(t, "1", m.Rules[0].Actions[0].Value)
}

// TestNormalizeRuleValues_AppliesToScopesPropertyType_NumberTypedArchetype
// verifies a rule scoped ONLY to the number-typed archetype ("wolf") coerces the
// numeric-looking string.
func TestNormalizeRuleValues_AppliesToScopesPropertyType_NumberTypedArchetype(t *testing.T) {
	m := modelForNormalizeConflict()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpEquals, Value: "1"},
			},
			Actions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "1"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, float64(1), m.Rules[0].Conditions[0].Value, "wolf's energy is number-typed; must be coerced")
	require.Equal(t, float64(1), m.Rules[0].Actions[0].Value)
}

// TestNormalizeRuleValues_AppliesToConflict_LeavesValueUntouched verifies that
// when a rule's applies_to spans BOTH archetypes that disagree on the
// PropertyUID's type, normalization is skipped entirely (safety: never guess).
func TestNormalizeRuleValues_AppliesToConflict_LeavesValueUntouched(t *testing.T) {
	m := modelForNormalizeConflict()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf", "sheep"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpEquals, Value: "1"},
			},
			Actions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "1"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Rules[0].Conditions[0].Value, "conflicting applies_to types must not be coerced")
	require.Equal(t, "1", m.Rules[0].Actions[0].Value)
}

// TestNormalizeRuleValues_EmptyAppliesTo_ConflictAcrossAllArchetypesLeftUntouched
// verifies the "applies to every archetype" case (empty AppliesTo) also applies
// the conflict-safety rule when archetypes disagree on the shared PropertyUID's
// type.
func TestNormalizeRuleValues_EmptyAppliesTo_ConflictAcrossAllArchetypesLeftUntouched(t *testing.T) {
	m := modelForNormalizeConflict()
	m.Rules = []datamodels.Rule{
		{
			ID: "r1",
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpEquals, Value: "1"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Rules[0].Conditions[0].Value)
}

// --- Item 3: interactions -------------------------------------------------

// TestNormalizeRuleValues_Interaction_InitiatorAndTargetActionsCoerced verifies
// InitiatorActions resolve against InitiatorUIDs and TargetActions resolve
// against TargetUIDs, using the numeric "energy" property (shared model
// fixture: single "wolf" archetype, no type conflict).
func TestNormalizeRuleValues_Interaction_InitiatorAndTargetActionsCoerced(t *testing.T) {
	m := modelForNormalize()
	m.Interactions = []datamodels.Interaction{
		{
			ID:            "i1",
			Type:          datamodels.InteractionTypeAgentAgent,
			InitiatorUIDs: []string{"wolf"},
			TargetUIDs:    []string{"wolf"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpGreaterThan, Value: "1"},
			},
			InitiatorActions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "5"},
			},
			TargetActions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "6"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, float64(1), m.Interactions[0].Conditions[0].Value)
	require.Equal(t, float64(5), m.Interactions[0].InitiatorActions[0].Value)
	require.Equal(t, float64(6), m.Interactions[0].TargetActions[0].Value)
}

// TestNormalizeRuleValues_Interaction_EnvTargetedActionCoerced verifies an
// InitiatorAction with TargetUID=="environment" resolves against globals.
func TestNormalizeRuleValues_Interaction_EnvTargetedActionCoerced(t *testing.T) {
	m := modelForNormalize()
	m.Interactions = []datamodels.Interaction{
		{
			ID:            "i1",
			Type:          datamodels.InteractionTypeAgentGlobal,
			InitiatorUIDs: []string{"wolf"},
			InitiatorActions: []datamodels.Action{
				{Type: datamodels.ActionTypeSetProperty, TargetUID: datamodels.EnvironmentUID, PropertyUID: "temperature", Value: "50"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, float64(50), m.Interactions[0].InitiatorActions[0].Value)
}

// TestNormalizeRuleValues_Interaction_CompositeRecursion verifies interaction
// Conditions recurse into SubConditions, same as rule conditions.
func TestNormalizeRuleValues_Interaction_CompositeRecursion(t *testing.T) {
	m := modelForNormalize()
	m.Interactions = []datamodels.Interaction{
		{
			ID:            "i1",
			Type:          datamodels.InteractionTypeAgentAgent,
			InitiatorUIDs: []string{"wolf"},
			TargetUIDs:    []string{"wolf"},
			Conditions: []datamodels.Condition{
				{
					Type:          datamodels.ConditionTypeComposite,
					LogicOperator: datamodels.LogicOpAND,
					SubConditions: []datamodels.Condition{
						{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpGreaterThan, Value: "5"},
					},
				},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, float64(5), m.Interactions[0].Conditions[0].SubConditions[0].Value)
}

// TestNormalizeRuleValues_Interaction_AppliesToConflict_InitiatorVsTargetLeftUntouched
// mirrors the rule-level conflict test: an initiator_actions PropertyUID that
// resolves against a DIFFERENT archetype than the target must not bleed across
// (InitiatorActions scope = InitiatorUIDs only; TargetActions scope =
// TargetUIDs only), and a Conditions PropertyUID that conflicts across the
// union of InitiatorUIDs+TargetUIDs must be left untouched.
func TestNormalizeRuleValues_Interaction_AppliesToConflict_InitiatorVsTargetLeftUntouched(t *testing.T) {
	m := modelForNormalizeConflict()
	m.Interactions = []datamodels.Interaction{
		{
			ID:            "i1",
			Type:          datamodels.InteractionTypeAgentAgent,
			InitiatorUIDs: []string{"wolf"},  // energy: number
			TargetUIDs:    []string{"sheep"}, // energy: string
			Conditions: []datamodels.Condition{
				// union(wolf,sheep) disagree on "energy" -> must not be coerced
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpEquals, Value: "1"},
			},
			InitiatorActions: []datamodels.Action{
				// scoped ONLY to wolf (number) -> coerced
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "2"},
			},
			TargetActions: []datamodels.Action{
				// scoped ONLY to sheep (string) -> untouched
				{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: "3"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Interactions[0].Conditions[0].Value, "conflicting union of initiator/target types must not be coerced")
	require.Equal(t, float64(2), m.Interactions[0].InitiatorActions[0].Value, "initiator_actions scoped to wolf (number) must be coerced")
	require.Equal(t, "3", m.Interactions[0].TargetActions[0].Value, "target_actions scoped to sheep (string) must not be coerced")
}

// --- event / memory conditions: PropertyUID is NOT a property reference ----

// TestNormalizeRuleValues_EventCondition_StringPayloadValueNotCoerced guards
// against resolveCondition treating an event condition's PropertyUID as a
// property reference. On an event condition PropertyUID is an event-UID fallback
// (see eventConditionUID in modules/simulations/rules.go), NOT a property; Value
// is an event payload label compared against the event's Data via matchEvent. If
// the event UID collides with a declared numeric property UID in scope, a
// numeric-looking string payload label must NOT be coerced to float64.
func TestNormalizeRuleValues_EventCondition_StringPayloadValueNotCoerced(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				// PropertyUID "energy" is the event-UID fallback and collides
				// with wolf's declared NUMBER property "energy".
				{Type: datamodels.ConditionTypeEvent, PropertyUID: "energy", Operator: datamodels.ConditionOpEquals, Value: "1"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Rules[0].Conditions[0].Value,
		"event payload label must stay a string even when its event UID collides with a numeric property UID")
}

// TestNormalizeRuleValues_MemoryCondition_StringValueNotCoerced guards the same
// hazard for memory conditions. evalMemory (modules/simulations/rules.go) never
// resolves a property by PropertyUID — Value is a memory summary/label matched
// via MemoryMatch — so a memory condition's Value must never be coerced against a
// colliding numeric property UID.
func TestNormalizeRuleValues_MemoryCondition_StringValueNotCoerced(t *testing.T) {
	m := modelForNormalize()
	m.Rules = []datamodels.Rule{
		{
			ID:        "r1",
			AppliesTo: []string{"wolf"},
			Conditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeMemory, PropertyUID: "energy", Operator: datamodels.ConditionOpContains, Value: "1"},
			},
		},
	}

	NormalizeRuleValues(m)

	require.Equal(t, "1", m.Rules[0].Conditions[0].Value,
		"memory label must stay a string even when PropertyUID collides with a numeric property UID")
}
