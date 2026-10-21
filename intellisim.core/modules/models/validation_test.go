package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

func hasCode(issues []ValidationIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

// spatialModelWithFixedShape builds a spatial model with one circle shape that
// has a fixed Geometry.Position and the given population count.
func spatialModelWithFixedShape(pos *common.Point, population uint64) datamodels.Model {
	return datamodels.Model{
		IsSpatial: true,
		Agents:    []datamodels.Agent{{UID: "a", Type: datamodels.AgentTypePlain}},
		Environment: &datamodels.Environment{
			UID:    datamodels.EnvironmentUID,
			Width:  100,
			Height: 100,
			Shapes: []datamodels.Shape{
				{
					UID:  "lake",
					Type: datamodels.ShapeTypeCircle,
					Initialization: datamodels.AgentInitialization{
						PopulationCount: population,
						Placement:       &datamodels.Placement{Type: datamodels.PlacementManual},
					},
					Geometry: &datamodels.ShapeGeometry{
						Position: pos,
						Radius:   &datamodels.ScalarSpec{Value: u64(10)},
					},
				},
			},
		},
	}
}

func validNonSpatialModel() datamodels.Model {
	return datamodels.Model{
		Name:      "valid",
		IsSpatial: false,
		Agents: []datamodels.Agent{
			{
				UID:    "wolf",
				Type:   datamodels.AgentTypePlain,
				States: []string{"hungry", "full"},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 3,
					InitialState:    "hungry",
				},
			},
		},
		Rules: []datamodels.Rule{
			{ID: "r1", AppliesTo: []string{"wolf"}, Enabled: true},
		},
	}
}

func TestValidateShape_DetectsBadEnumAndScalarSpec(t *testing.T) {
	m := datamodels.Model{Agents: []datamodels.Agent{{UID: "a", Type: "bogus"}}}
	issues := ValidateShape(m)
	require.NotEmpty(t, issues) // bad agent type -> error
	require.True(t, hasCode(issues, "enum"))
}

func TestValidateShape_GeometryRequiresPopulationOne(t *testing.T) {
	p := common.Point{X: 5, Y: 5}
	m := spatialModelWithFixedShape(&p, 3) // Position set but pop != 1
	issues := ValidateShape(m)
	require.True(t, hasCode(issues, "geometry_position_population"))
}

// TestLLMMerge_FixedPositionShape_RoundTripsThroughValidator reproduces the Level-1
// round-trip finding: the LLM generates a shape with a fixed geometry.position but omits
// population_count, so the new shape ends with the zero value (0). The generation-save
// normalization (NormalizeSpatial) must coerce it to 1 so the stored model satisfies the
// manual-edit validator (which 422s a fixed-position shape whose population_count != 1).
func TestLLMMerge_FixedPositionShape_RoundTripsThroughValidator(t *testing.T) {
	env := LLMGenerationEnvelope{Model: &LLMModel{Environment: &LLMEnvironment{
		UID:    datamodels.EnvironmentUID,
		Width:  llmU64Ptr(40),
		Height: llmU64Ptr(30),
		Shapes: []LLMShape{{
			UID:  "lake",
			Type: "circle",
			Geometry: &LLMShapeGeometry{
				Position: &LLMPoint{X: 20, Y: 15},
				Radius:   &LLMScalarSpec{Value: llmU64Ptr(5)},
			},
			// Non-llm placement, population_count omitted — the exact E2E case.
		}},
	}}}

	merged, _ := llmMerge(datamodels.Model{}, env, EditScope{})

	lake := findShape(merged, "lake")
	require.NotNil(t, lake)
	require.NotNil(t, lake.Geometry.Position)
	// Root cause: an omitted population_count leaves the zero value on the new shape.
	require.Equal(t, uint64(0), lake.Initialization.PopulationCount)

	// Mirror the generation-save normalization the service runs before persisting.
	merged.NormalizeSpatial()

	// The stored model must now satisfy the manual-edit validator (the 422 was
	// environment.shapes[0].geometry.position / geometry_position_population).
	issues := ValidateShape(merged)
	require.False(t, hasCode(issues, "geometry_position_population"),
		"fixed-position shape must round-trip cleanly through the validator; issues=%v", issues)
	require.Equal(t, uint64(1), findShape(merged, "lake").Initialization.PopulationCount)
}

func TestValidateCompleteness_SpatialNeedsEnvDims(t *testing.T) {
	m := datamodels.Model{IsSpatial: true} // no environment
	res := ValidateCompleteness(m)
	require.False(t, res.SimulationReady)
	require.NotEmpty(t, res.Errors)
}

func TestValidateCompleteness_CleanModelIsReady(t *testing.T) {
	res := ValidateCompleteness(validNonSpatialModel())
	require.True(t, res.SimulationReady)
	require.Empty(t, res.Errors)
}

// ===================== property generation =====================

func hasIssueAt(issues []ValidationIssue, path string) bool {
	for _, i := range issues {
		if i.Path == path {
			return true
		}
	}
	return false
}

// modelWithProp wraps a single property into a minimal, otherwise-valid model.
func modelWithProp(p datamodels.AgentProperty) datamodels.Model {
	return datamodels.Model{Agents: []datamodels.Agent{{
		UID: "wolf", Type: datamodels.AgentTypePlain, Properties: []datamodels.AgentProperty{p},
	}}}
}

func TestValidateShape_Generation_InvalidMode(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{Mode: "bogus"},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.mode"))
}

func TestValidateShape_Generation_RandomRequiresDistribution(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeRandom},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.distribution"))
}

func TestValidateShape_Generation_NormalRequiresMeanStdDev(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeRandom, Distribution: datamodels.DistNormal},
	})
	issues := ValidateShape(m)
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.mean"))
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.std_dev"))
}

func TestValidateShape_Generation_NormalStdDevMustBePositive(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeRandom, Distribution: datamodels.DistNormal,
			Mean: f64(50), StdDev: f64(0),
		},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.std_dev"))
}

func TestValidateShape_Generation_NormalRequiresNumberProperty(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "label", Type: datamodels.PropertyTypeString,
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeRandom, Distribution: datamodels.DistNormal,
			Mean: f64(1), StdDev: f64(1),
		},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.distribution"))
}

func TestValidateShape_Generation_UniformRequiresNumberProperty(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "label", Type: datamodels.PropertyTypeString,
		Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeRandom, Distribution: datamodels.DistUniform},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.distribution"))
}

func TestValidateShape_Generation_ChoiceRequiresChoices(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeRandom, Distribution: datamodels.DistChoice},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.choices"))
}

func TestValidateShape_Generation_ChoiceWeightAndValueChecked(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeRandom, Distribution: datamodels.DistChoice,
			Choices: []datamodels.ChoiceWeight{{Value: "high", Weight: 0}}, // string on number prop, weight 0
		},
	})
	issues := ValidateShape(m)
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.choices[0].weight"))
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.choices[0].value"))
}

func TestValidateShape_Generation_EnumChoiceMustBeAllowedValue(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "mood", Type: datamodels.PropertyTypeEnum, AllowedValues: []interface{}{"calm", "angry"},
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeRandom, Distribution: datamodels.DistChoice,
			Choices: []datamodels.ChoiceWeight{{Value: "sleepy", Weight: 1}},
		},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.choices[0].value"))
}

func TestValidateShape_Generation_ValidChoiceOnBooleanPasses(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "alive", Type: datamodels.PropertyTypeBoolean,
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeRandom, Distribution: datamodels.DistChoice,
			Choices: []datamodels.ChoiceWeight{{Value: true, Weight: 1}, {Value: false, Weight: 3}},
		},
	})
	require.Empty(t, ValidateShape(m))
}

func TestValidateShape_Generation_MinMustBeLessThanMax(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeRandom, Distribution: datamodels.DistUniform,
			Min: f64(5), Max: f64(5),
		},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].properties[0].generation.min"))
}

func TestValidateShape_Generation_DefaultAndLLMForbidRandomFields(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeDefault, Distribution: datamodels.DistNormal,
			Mean: f64(1), StdDev: f64(1),
			Choices: []datamodels.ChoiceWeight{{Value: 1.0, Weight: 1}},
		},
	})
	issues := ValidateShape(m)
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.distribution"))
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.mean"))
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.std_dev"))
	require.True(t, hasIssueAt(issues, "agents[0].properties[0].generation.choices"))

	// llm mode with only a prompt hint is clean.
	m2 := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeLLM, PromptContext: "hint"},
	})
	require.Empty(t, ValidateShape(m2))
}

func TestValidateShape_Generation_ValidNormalPasses(t *testing.T) {
	m := modelWithProp(datamodels.AgentProperty{
		UID: "energy", Type: datamodels.PropertyTypeNumber,
		Generation: &datamodels.PropertyGeneration{
			Mode: datamodels.GenModeRandom, Distribution: datamodels.DistNormal,
			Mean: f64(50), StdDev: f64(10), Min: f64(0), Max: f64(100),
		},
	})
	require.Empty(t, ValidateShape(m))
}

// ===================== count mode =====================

func modelWithInit(init datamodels.AgentInitialization) datamodels.Model {
	return datamodels.Model{Agents: []datamodels.Agent{{
		UID: "wolf", Type: datamodels.AgentTypePlain, Initialization: init,
	}}}
}

func TestValidateShape_CountMode_InvalidEnum(t *testing.T) {
	m := modelWithInit(datamodels.AgentInitialization{PopulationCount: 1, CountMode: "bogus"})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].initialization.count_mode"))
}

func TestValidateShape_CountMode_RandomRangeRequiresBounds(t *testing.T) {
	m := modelWithInit(datamodels.AgentInitialization{PopulationCount: 1, CountMode: datamodels.CountRandomRange})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].initialization.count_mode"))

	m = modelWithInit(datamodels.AgentInitialization{
		PopulationCount: 1, CountMode: datamodels.CountRandomRange,
		MinPopulation: u64(9), MaxPopulation: u64(3),
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].initialization.min_population"))

	m = modelWithInit(datamodels.AgentInitialization{
		PopulationCount: 1, CountMode: datamodels.CountRandomRange,
		MinPopulation: u64(3), MaxPopulation: u64(9),
	})
	require.Empty(t, ValidateShape(m))
}

func TestValidateShape_CountMode_LLMBoundsOrderedWhenSet(t *testing.T) {
	m := modelWithInit(datamodels.AgentInitialization{
		PopulationCount: 1, CountMode: datamodels.CountLLM,
		MinPopulation: u64(9), MaxPopulation: u64(3),
	})
	require.True(t, hasIssueAt(ValidateShape(m), "agents[0].initialization.min_population"))

	// llm without bounds is fine.
	m = modelWithInit(datamodels.AgentInitialization{PopulationCount: 1, CountMode: datamodels.CountLLM})
	require.Empty(t, ValidateShape(m))
}

func TestValidateShape_CountMode_OnShape(t *testing.T) {
	p := common.Point{X: 1, Y: 1}
	m := spatialModelWithFixedShape(&p, 1)
	m.Environment.Shapes[0].Initialization.CountMode = "bogus"
	require.True(t, hasIssueAt(ValidateShape(m), "environment.shapes[0].initialization.count_mode"))
}

// ===================== widgets =====================

// modelForWidgets builds an otherwise-valid model with a known archetype ("wolf",
// properties energy+speed), a global property ("temperature"), and the widgets.
func modelForWidgets(ws ...common.WidgetConfig) datamodels.Model {
	return datamodels.Model{
		Agents: []datamodels.Agent{{
			UID: "wolf", Type: datamodels.AgentTypeArchetype,
			Properties: []datamodels.AgentProperty{
				{UID: "energy", Type: datamodels.PropertyTypeNumber},
				{UID: "speed", Type: datamodels.PropertyTypeNumber},
			},
		}},
		Environment: &datamodels.Environment{
			UID:              datamodels.EnvironmentUID,
			GlobalProperties: []datamodels.AgentProperty{{UID: "temperature", Type: datamodels.PropertyTypeNumber}},
		},
		Widgets: ws,
	}
}

func monitorWidget(id, propUID string, agentUIDs ...string) common.WidgetConfig {
	return common.WidgetConfig{
		ID: id, Type: common.WidgetMonitor, Title: "T",
		Source: &common.WidgetSource{
			Type: common.WidgetSourceAgentProperty, PropertyUID: propUID,
			AgentUIDs: agentUIDs, Aggregation: common.AggAvg,
		},
	}
}

func TestValidateShape_Widgets_StructuralIssuesSurface(t *testing.T) {
	m := modelForWidgets(common.WidgetConfig{ID: "w1", Type: common.WidgetMonitor, Title: "T"}) // no source
	require.True(t, hasIssueAt(ValidateShape(m), "widgets[0].source"))
}

func TestValidateShape_Widgets_DuplicateID(t *testing.T) {
	m := modelForWidgets(monitorWidget("w1", "energy"), monitorWidget("w1", "speed"))
	require.True(t, hasIssueAt(ValidateShape(m), "widgets[1].id"))
}

func TestValidateShape_Widgets_UnknownAgentUID(t *testing.T) {
	m := modelForWidgets(monitorWidget("w1", "energy", "ghost"))
	require.True(t, hasIssueAt(ValidateShape(m), "widgets[0].source.agent_uids[0]"))
}

func TestValidateShape_Widgets_PropertyMustExistOnReferencedAgents(t *testing.T) {
	m := modelForWidgets(monitorWidget("w1", "nonexistent", "wolf"))
	require.True(t, hasIssueAt(ValidateShape(m), "widgets[0].source.property_uid"))

	// Empty agent_uids: any archetype carrying the property is enough.
	m = modelForWidgets(monitorWidget("w2", "energy"))
	require.Empty(t, ValidateShape(m))
}

func TestValidateShape_Widgets_GlobalPropertyMustExist(t *testing.T) {
	m := modelForWidgets(common.WidgetConfig{
		ID: "w1", Type: common.WidgetMonitor, Title: "T",
		Source: &common.WidgetSource{Type: common.WidgetSourceGlobalProperty, PropertyUID: "humidity"},
	})
	require.True(t, hasIssueAt(ValidateShape(m), "widgets[0].source.property_uid"))
}

func TestValidateShape_Widgets_AgentTableColumnsMustBeProperties(t *testing.T) {
	m := modelForWidgets(common.WidgetConfig{
		ID: "t1", Type: common.WidgetAgentTable, Title: "Wolves",
		Source:  &common.WidgetSource{Type: common.WidgetSourceAgentProperty, AgentUIDs: []string{"wolf"}},
		Columns: []string{"energy", "mass"},
	})
	issues := ValidateShape(m)
	require.False(t, hasIssueAt(issues, "widgets[0].columns[0]")) // energy exists on wolf
	require.True(t, hasIssueAt(issues, "widgets[0].columns[1]"))  // mass does not
}

func TestValidateShape_Widgets_SeriesSourcesChecked(t *testing.T) {
	m := modelForWidgets(common.WidgetConfig{
		ID: "c1", Type: common.WidgetLineChart, Title: "Chart",
		Series: []common.WidgetSeries{
			{Label: "ok", Source: common.WidgetSource{Type: common.WidgetSourceAgentProperty, PropertyUID: "energy", Aggregation: common.AggAvg}},
			{Label: "bad", Source: common.WidgetSource{Type: common.WidgetSourceAgentProperty, PropertyUID: "ghost_prop", Aggregation: common.AggAvg}},
		},
	})
	issues := ValidateShape(m)
	require.False(t, hasIssueAt(issues, "widgets[0].series[0].source.property_uid"))
	require.True(t, hasIssueAt(issues, "widgets[0].series[1].source.property_uid"))
}

// ===================== environment-scoped rules (Task 18) =====================

// validModelWithEnvironmentRule returns a complete, otherwise-valid model whose
// single rule targets the reserved "environment" UID via AppliesTo. The model
// must validate as SimulationReady — the environment singleton is a known target.
func validModelWithEnvironmentRule() datamodels.Model {
	return datamodels.Model{
		Name:      "env-rule-model",
		IsSpatial: false,
		Agents: []datamodels.Agent{
			{UID: "ant", Type: datamodels.AgentTypePlain},
		},
		Environment: &datamodels.Environment{
			UID: datamodels.EnvironmentUID,
			GlobalProperties: []datamodels.AgentProperty{
				{UID: "temperature", Type: datamodels.PropertyTypeNumber},
			},
		},
		Rules: []datamodels.Rule{
			{
				ID:        "env-rule",
				AppliesTo: []string{datamodels.EnvironmentUID},
				Enabled:   true,
			},
		},
	}
}

// TestValidateCompleteness_EnvironmentRuleIsReady asserts that a rule whose
// AppliesTo contains the reserved EnvironmentUID ("environment") is NOT flagged
// as an unresolved_ref and that the model is SimulationReady.
func TestValidateCompleteness_EnvironmentRuleIsReady(t *testing.T) {
	m := validModelWithEnvironmentRule()
	res := ValidateCompleteness(m)
	// Must not produce an unresolved_ref error for the environment UID.
	for _, e := range res.Errors {
		if e.Code == "unresolved_ref" {
			t.Fatalf("unexpected unresolved_ref error: %+v", e)
		}
	}
	require.True(t, res.SimulationReady, "model with environment-scoped rule must be SimulationReady")
}

// TestValidateCompleteness_EnvironmentRuleMixedAppliesToIsReady verifies the same
// exemption when AppliesTo mixes a real agent UID and the environment UID.
func TestValidateCompleteness_EnvironmentRuleMixedAppliesToIsReady(t *testing.T) {
	m := validModelWithEnvironmentRule()
	// Also target a real agent alongside the environment.
	m.Rules[0].AppliesTo = []string{"ant", datamodels.EnvironmentUID}
	res := ValidateCompleteness(m)
	for _, e := range res.Errors {
		if e.Code == "unresolved_ref" {
			t.Fatalf("unexpected unresolved_ref error: %+v", e)
		}
	}
	require.True(t, res.SimulationReady)
}

// TestValidateCompleteness_UnknownUIDStillFlagged ensures the exemption is
// narrow: only EnvironmentUID is accepted; truly unknown UIDs are still flagged.
func TestValidateCompleteness_UnknownUIDStillFlagged(t *testing.T) {
	m := validModelWithEnvironmentRule()
	m.Rules[0].AppliesTo = []string{"ghost"}
	res := ValidateCompleteness(m)
	require.True(t, hasCode(res.Errors, "unresolved_ref"), "unknown uid must still produce unresolved_ref")
	require.False(t, res.SimulationReady)
}

// ===================== C8: readiness blockers (ISS-05 + ISS-11 validation half) =====================
//
// These pin ValidateCompleteness's engine-silent-no-op readiness checks: a
// condition/action shaped so the matching modules/simulations/rules.go handler
// always treats it as a no-op is an addErr (blocks SimulationReady) but must
// NEVER block save (that stays ValidateShape's job — not exercised here).

// modelWithRuleCondition builds an otherwise-valid model with one agent-scoped
// rule carrying a single condition.
func modelWithRuleCondition(c datamodels.Condition) datamodels.Model {
	return datamodels.Model{
		Name: "readiness",
		Agents: []datamodels.Agent{
			{UID: "wolf", Type: datamodels.AgentTypePlain, Properties: []datamodels.AgentProperty{
				{UID: "energy", Type: datamodels.PropertyTypeNumber},
			}},
		},
		Rules: []datamodels.Rule{
			{ID: "r1", AppliesTo: []string{"wolf"}, Enabled: true, Conditions: []datamodels.Condition{c}},
		},
	}
}

// modelWithRuleAction builds an otherwise-valid model with one agent-scoped
// rule carrying a single action.
func modelWithRuleAction(a datamodels.Action) datamodels.Model {
	return datamodels.Model{
		Name: "readiness",
		Agents: []datamodels.Agent{
			{UID: "wolf", Type: datamodels.AgentTypePlain, Properties: []datamodels.AgentProperty{
				{UID: "energy", Type: datamodels.PropertyTypeNumber},
			}},
		},
		Rules: []datamodels.Rule{
			{ID: "r1", AppliesTo: []string{"wolf"}, Enabled: true, Actions: []datamodels.Action{a}},
		},
	}
}

func TestValidateCompleteness_Condition_ProximityRequiresRadius(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeProximity}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Condition_CountRequiresRadius(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeCount}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Condition_ProximityWithRadiusIsReady(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeProximity, Radius: f64(5)}))
	require.False(t, hasCode(res.Errors, "missing_field"))
	require.True(t, res.SimulationReady)
}

func TestValidateCompleteness_Condition_CompositeRequiresSubConditions(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeComposite}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Condition_EventRequiresTargetOrProperty(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeEvent}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Condition_EventWithPropertyUIDIsReady(t *testing.T) {
	// PropertyUID is the documented TargetUID fallback for identifying the event.
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeEvent, PropertyUID: "alarm"}))
	require.False(t, hasCode(res.Errors, "missing_field"))
}

func TestValidateCompleteness_Condition_CustomRequiresParametersProperty(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeCustom}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Condition_CustomWithParametersPropertyIsReady(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{
		Type:       datamodels.ConditionTypeCustom,
		Parameters: map[string]interface{}{"property": "energy"},
	}))
	require.False(t, hasCode(res.Errors, "missing_field"))
}

func TestValidateCompleteness_Condition_GlobalPropertyRequiresPropertyUID(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeGlobalProperty}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Condition_PropertyRequiresPropertyUID(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleCondition(datamodels.Condition{Type: datamodels.ConditionTypeProperty}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

// TestValidateCompleteness_Condition_CompositeRecursesIntoSubConditions verifies
// that a composite's own shape is fine (non-empty SubConditions) but a malformed
// sub-condition is still flagged at the nested sub_conditions path.
func TestValidateCompleteness_Condition_CompositeRecursesIntoSubConditions(t *testing.T) {
	m := modelWithRuleCondition(datamodels.Condition{
		Type:          datamodels.ConditionTypeComposite,
		LogicOperator: datamodels.LogicOpAND,
		SubConditions: []datamodels.Condition{
			{Type: datamodels.ConditionTypeProximity}, // missing radius
		},
	})
	res := ValidateCompleteness(m)
	require.True(t, hasIssueAt(res.Errors, "rules[0].conditions[0].sub_conditions[0].radius"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Action_SpawnAgentRequiresSpawnAgentUID(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeSpawnAgent}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Action_TransformRequiresSpawnAgentUIDOrTargetUID(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeTransform}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)

	res = ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeTransform, TargetUID: "wolf"}))
	require.False(t, hasCode(res.Errors, "missing_field"))
}

func TestValidateCompleteness_Action_EmitEventRequiresEventUID(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeEmitEvent}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Action_TransitionRequiresNewState(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeTransition}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Action_SetPropertyRequiresPropertyUIDAndValue(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeSetProperty}))
	errs := res.Errors
	require.True(t, hasIssueAt(errs, "rules[0].actions[0].property_uid"))
	require.True(t, hasIssueAt(errs, "rules[0].actions[0].value"))
	require.False(t, res.SimulationReady)

	res = ValidateCompleteness(modelWithRuleAction(datamodels.Action{
		Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: 1.0,
	}))
	require.False(t, hasCode(res.Errors, "missing_field"))
}

func TestValidateCompleteness_Action_ModifyPropertyRequiresPropertyUID(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeModifyProperty}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Action_InteractRequiresTargetUID(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeInteract}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)
}

func TestValidateCompleteness_Action_CustomRequiresParametersSetProperties(t *testing.T) {
	res := ValidateCompleteness(modelWithRuleAction(datamodels.Action{Type: datamodels.ActionTypeCustom}))
	require.True(t, hasCode(res.Errors, "missing_field"))
	require.False(t, res.SimulationReady)

	res = ValidateCompleteness(modelWithRuleAction(datamodels.Action{
		Type:       datamodels.ActionTypeCustom,
		Parameters: map[string]interface{}{"set_properties": map[string]interface{}{"energy": 1.0}},
	}))
	require.False(t, hasCode(res.Errors, "missing_field"))
}

// TestValidateCompleteness_EnvScope_UnsupportedConditionType verifies the
// env-scope condition gating: a rule whose AppliesTo targets the environment
// singleton is flagged when a condition type outside evalEnvCondition's
// supported set is used.
func TestValidateCompleteness_EnvScope_UnsupportedConditionType(t *testing.T) {
	m := validModelWithEnvironmentRule()
	m.Rules[0].Conditions = []datamodels.Condition{
		{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy"}, // agent-positional; unsupported for env
	}
	res := ValidateCompleteness(m)
	require.True(t, hasCode(res.Errors, "env_condition_unsupported"))
	require.False(t, res.SimulationReady)
}

// TestValidateCompleteness_EnvScope_SupportedConditionTypesAreReady verifies
// every condition type evalEnvCondition supports is NOT flagged env_condition_unsupported.
func TestValidateCompleteness_EnvScope_SupportedConditionTypesAreReady(t *testing.T) {
	for _, c := range []datamodels.Condition{
		{Type: datamodels.ConditionTypeGlobalProperty, PropertyUID: "temperature"},
		{Type: datamodels.ConditionTypeStep, Value: 1.0},
		{Type: datamodels.ConditionTypeRandom, Value: 0.5},
		{Type: datamodels.ConditionTypeEvent, TargetUID: "alarm"},
	} {
		m := validModelWithEnvironmentRule()
		m.Rules[0].Conditions = []datamodels.Condition{c}
		res := ValidateCompleteness(m)
		require.False(t, hasCode(res.Errors, "env_condition_unsupported"), "condition type %q must be supported for env-scoped rules", c.Type)
	}
}

// TestValidateCompleteness_EnvScope_CompositeRecursesIntoSubConditions verifies
// that the env-scope gate recurses into a composite's sub-conditions.
func TestValidateCompleteness_EnvScope_CompositeRecursesIntoSubConditions(t *testing.T) {
	m := validModelWithEnvironmentRule()
	m.Rules[0].Conditions = []datamodels.Condition{
		{
			Type:          datamodels.ConditionTypeComposite,
			LogicOperator: datamodels.LogicOpAND,
			SubConditions: []datamodels.Condition{
				{Type: datamodels.ConditionTypeGlobalProperty, PropertyUID: "temperature"}, // ok
				{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy"},            // unsupported for env
			},
		},
	}
	res := ValidateCompleteness(m)
	require.True(t, hasIssueAt(res.Errors, "rules[0].conditions[0].sub_conditions[1]"))
	require.True(t, hasCode(res.Errors, "env_condition_unsupported"))
}

// TestValidateCompleteness_CompleteRuleProducesNoErrors is the converse of the
// above: a rule with a fully-formed condition + action of every checked type
// must produce zero readiness errors and leave the model SimulationReady.
func TestValidateCompleteness_CompleteRuleProducesNoErrors(t *testing.T) {
	m := datamodels.Model{
		Name: "complete",
		Agents: []datamodels.Agent{
			{UID: "wolf", Type: datamodels.AgentTypePlain, Properties: []datamodels.AgentProperty{
				{UID: "energy", Type: datamodels.PropertyTypeNumber},
			}},
		},
		Rules: []datamodels.Rule{
			{
				ID:        "r1",
				AppliesTo: []string{"wolf"},
				Enabled:   true,
				Conditions: []datamodels.Condition{
					{Type: datamodels.ConditionTypeProperty, PropertyUID: "energy", Operator: datamodels.ConditionOpGreaterThan, Value: 10.0},
					{
						Type:          datamodels.ConditionTypeComposite,
						LogicOperator: datamodels.LogicOpAND,
						SubConditions: []datamodels.Condition{
							{Type: datamodels.ConditionTypeProximity, Radius: f64(5)},
							{Type: datamodels.ConditionTypeCount, Radius: f64(5)},
						},
					},
					{Type: datamodels.ConditionTypeEvent, TargetUID: "alarm"},
					{Type: datamodels.ConditionTypeCustom, Parameters: map[string]interface{}{"property": "energy"}},
					{Type: datamodels.ConditionTypeGlobalProperty, PropertyUID: "temperature"},
				},
				Actions: []datamodels.Action{
					{Type: datamodels.ActionTypeSpawnAgent, SpawnAgentUID: "wolf"},
					{Type: datamodels.ActionTypeTransform, SpawnAgentUID: "wolf"},
					{Type: datamodels.ActionTypeEmitEvent, EventUID: "alarm"},
					{Type: datamodels.ActionTypeTransition, NewState: "hungry"},
					{Type: datamodels.ActionTypeSetProperty, PropertyUID: "energy", Value: 5.0},
					{Type: datamodels.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: datamodels.PropertyModifierIncrement},
					{Type: datamodels.ActionTypeInteract, TargetUID: "share-food"},
					{Type: datamodels.ActionTypeCustom, Parameters: map[string]interface{}{"set_properties": map[string]interface{}{"energy": 1.0}}},
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
	res := ValidateCompleteness(m)
	require.Empty(t, res.Errors, "complete rule must produce no readiness errors, got %+v", res.Errors)
	require.True(t, res.SimulationReady)
}
