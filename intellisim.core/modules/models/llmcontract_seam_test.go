package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	datamodels "intelli.sim.core/data/models"
)

// U7 (spec amendment 2026-07-18): the Level-1 contract exposes the run-stop
// termination condition (U2 subset) and the environment collision mode, and
// completeness validation no longer blocks readiness on DISABLED rules.
// Triggers: EXP-A attempt 3 — the LLM stated the stop condition but could not
// author it (contract gap), emitted an invalid custom-action rule instead,
// could not delete it (merge is upsert-only by design), and disabling it did
// not restore readiness (over-strict validation).

// A disabled rule never executes, so its readiness issues must be warnings,
// not blockers; re-enabling re-runs validation and flags them again.
func TestValidateCompleteness_DisabledRuleIssuesAreWarnings(t *testing.T) {
	m := datamodels.Model{
		Agents: []datamodels.Agent{{UID: "visitor", Type: datamodels.AgentTypePlain}},
		Rules: []datamodels.Rule{{
			ID: "stopper", AppliesTo: []string{"visitor"}, Enabled: false,
			Actions: []datamodels.Action{{Type: datamodels.ActionTypeCustom,
				Parameters: map[string]interface{}{"engine_action": "stop_simulation"}}},
		}},
	}
	res := ValidateCompleteness(m)
	require.True(t, res.SimulationReady, "a disabled rule must not block readiness")
	require.Empty(t, res.Errors)
	require.NotEmpty(t, res.Warnings, "the issue is still surfaced as a warning")

	m.Rules[0].Enabled = true
	res = ValidateCompleteness(m)
	require.False(t, res.SimulationReady, "the same rule enabled still blocks")
	require.NotEmpty(t, res.Errors)
}

// U8c: a position condition must use one of the two engine-supported forms —
// in/not_in + shape target, or in_range + min/max. Anything else is a silent
// always-false no-op in evalPosition and must block readiness.
func TestValidateCompleteness_PositionConditionForms(t *testing.T) {
	base := func(c datamodels.Condition) datamodels.Model {
		return datamodels.Model{
			Agents: []datamodels.Agent{{UID: "visitor", Type: datamodels.AgentTypePlain}},
			Rules: []datamodels.Rule{{ID: "r", AppliesTo: []string{"visitor"}, Enabled: true,
				Conditions: []datamodels.Condition{c},
				Actions:    []datamodels.Action{{Type: datamodels.ActionTypeRemoveAgent}}}},
		}
	}
	mm := func(f float64) *float64 { return &f }

	ok1 := ValidateCompleteness(base(datamodels.Condition{Type: datamodels.ConditionTypePosition,
		Operator: datamodels.ConditionOpIn, TargetUID: "exit_door"}))
	require.True(t, ok1.SimulationReady, "in + target_uid is a supported form")

	ok2 := ValidateCompleteness(base(datamodels.Condition{Type: datamodels.ConditionTypePosition,
		Operator: datamodels.ConditionOpInRange, MinValue: mm(0), MaxValue: mm(5)}))
	require.True(t, ok2.SimulationReady, "in_range + min/max is a supported form")

	bad1 := ValidateCompleteness(base(datamodels.Condition{Type: datamodels.ConditionTypePosition,
		Operator: datamodels.ConditionOpIn})) // no target
	require.False(t, bad1.SimulationReady, "in without a shape target is a silent no-op")

	bad2 := ValidateCompleteness(base(datamodels.Condition{Type: datamodels.ConditionTypePosition,
		Operator: datamodels.ConditionOpEquals, Value: float64(3)}))
	require.False(t, bad2.SimulationReady, "unsupported operator is a silent no-op")
}

// collision_mode flows from the envelope into the environment; invalid values
// warn and keep the previous mode.
func TestEnvelope_CollisionModeApplied(t *testing.T) {
	raw := `{"display_message":"d","model":{"environment":{"width":10,"height":10,"collision_mode":"block"}}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	merged, warns := llmMerge(datamodels.Model{IsSpatial: true}, env, EditScope{})
	require.NotNil(t, merged.Environment)
	require.Equal(t, datamodels.CollisionBlock, merged.Environment.CollisionMode)
	require.Empty(t, warns)

	raw2 := `{"display_message":"d","model":{"environment":{"collision_mode":"bouncy"}}}`
	env2, err := parseEnvelope(raw2)
	require.NoError(t, err)
	merged2, warns2 := llmMerge(merged, env2, EditScope{})
	require.Equal(t, datamodels.CollisionBlock, merged2.Environment.CollisionMode)
	require.NotEmpty(t, warns2)
}

// termination flows from the envelope into SimulationConfig.Termination when
// it uses the U2-supported subset; unsupported types warn and keep previous.
func TestEnvelope_TerminationApplied(t *testing.T) {
	raw := `{"display_message":"d","model":{"termination":{"type":"population","operator":"eq","value":0,"target_uid":"visitor"}}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	merged, warns := llmMerge(datamodels.Model{}, env, EditScope{})
	require.Empty(t, warns)
	require.NotNil(t, merged.SimulationConfig.Termination)
	require.Equal(t, datamodels.ConditionTypePopulation, merged.SimulationConfig.Termination.Type)
	require.Equal(t, "visitor", merged.SimulationConfig.Termination.TargetUID)

	// Unsupported type -> warning, previous kept.
	raw2 := `{"display_message":"d","model":{"termination":{"type":"random"}}}`
	env2, err := parseEnvelope(raw2)
	require.NoError(t, err)
	merged2, warns2 := llmMerge(merged, env2, EditScope{})
	require.NotEmpty(t, warns2)
	require.Equal(t, datamodels.ConditionTypePopulation, merged2.SimulationConfig.Termination.Type)

	// Composite recurses over the subset: an event sub-condition is rejected.
	raw3 := `{"display_message":"d","model":{"termination":{"type":"composite","logic_operator":"AND",
		"sub_conditions":[{"type":"population","operator":"eq","value":0},{"type":"event","target_uid":"x"}]}}}`
	env3, err := parseEnvelope(raw3)
	require.NoError(t, err)
	merged3, warns3 := llmMerge(merged, env3, EditScope{})
	require.NotEmpty(t, warns3)
	require.Equal(t, datamodels.ConditionTypePopulation, merged3.SimulationConfig.Termination.Type)
}

// Termination applies at whole-model scope only — a scoped rules edit must not
// touch it.
func TestEnvelope_TerminationIgnoredOutsideWholeModelScope(t *testing.T) {
	raw := `{"display_message":"d","model":{"termination":{"type":"step","operator":"gte","value":50}}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	merged, _ := llmMerge(datamodels.Model{}, env, EditScope{Section: "rules"})
	require.Nil(t, merged.SimulationConfig.Termination)
}

// The current-model projection serializes both seam fields so the LLM can see
// and edit them.
func TestProjectToContract_IncludesSeamFields(t *testing.T) {
	m := datamodels.Model{
		IsSpatial: true,
		Environment: &datamodels.Environment{
			UID: datamodels.EnvironmentUID, Width: 10, Height: 10,
			CollisionMode: datamodels.CollisionBlock,
		},
		SimulationConfig: datamodels.SimulationConfig{
			Termination: &datamodels.Condition{
				Type: datamodels.ConditionTypePopulation, Operator: datamodels.ConditionOpEquals,
				Value: float64(0), TargetUID: "visitor",
			},
		},
	}
	out := projectToContract(m, EditScope{})
	require.NotNil(t, out.Environment)
	require.Equal(t, "block", out.Environment.CollisionMode)
	require.NotNil(t, out.Termination)
	require.Equal(t, "population", out.Termination.Type)
	require.Equal(t, "visitor", out.Termination.TargetUID)
}
