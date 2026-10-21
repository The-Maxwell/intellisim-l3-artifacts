package models

import (
	"testing"

	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

func f64(v float64) *float64 { return &v }
func u64(v uint64) *uint64   { return &v }

// sampleModel builds a non-trivial models.Model exercising agents, env+shape,
// rule, interaction, event, locks, and LLM config.
func sampleModel() datamodels.Model {
	return datamodels.Model{
		ID:              "m1",
		ChatID:          "c1",
		Name:            "Predator-Prey",
		Description:     "test model",
		IsSpatial:       true,
		SpatialCellSize: 10,
		LLMConfig: datamodels.LLMConfig{
			Target:        common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortMedium},
			PromptContext: "ctx",
		},
		SimulationConfig: datamodels.SimulationConfig{
			Termination: &datamodels.Condition{Type: datamodels.ConditionTypePopulation, Operator: datamodels.ConditionOpEquals, Value: float64(0)},
			UpdateMode: datamodels.UpdateModeSynchronous,
			LLMSimulationGenerationConfig: datamodels.LLMConfig{
				Target:        common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini", Effort: common.LLMEffortLow},
				PromptContext: "gen-ctx",
			},
			LLMSimulationExecutionConfig: datamodels.LLMSimulationExecutionConfig{
				LLMConfig: datamodels.LLMConfig{
					Target:        common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh},
					PromptContext: "exec-ctx",
				},
				LLMSimulationAgentConfig: datamodels.LLMSimulationAgentConfig{
					PerceptionRadius: 3.5,
					MemorySize:       10,
					FallbackPolicy: &datamodels.FallbackPolicy{
						Steps: []datamodels.FallbackStep{{Type: datamodels.FallbackRetry, MaxRetries: 2}},
					},
				},
				ParallelWorkers: 4,
				RetryTimeoutMs:  30000,
				MaxRetries:      3,
				StreamBatchSize: 100,
				DefaultStrategy: datamodels.LLMStrategy{
					Type:           datamodels.LLMStrategyHybrid,
					BatchSize:      5,
					UseForMovement: true,
				},
			},
		},
		Agents: []datamodels.Agent{
			{
				UID:      "wolf",
				Name:     "Wolf",
				Type:     datamodels.AgentTypeArchetype,
				MoveType: datamodels.AgentMoveTypeMovable,
				States:   []string{"hungry", "full"},
				Properties: []datamodels.AgentProperty{
					{
						UID: "energy", Type: datamodels.PropertyTypeNumber, MinValue: f64(0), MaxValue: f64(100),
						Generation: &datamodels.PropertyGeneration{
							Mode: datamodels.GenModeRandom, Distribution: datamodels.DistNormal,
							Mean: f64(50), StdDev: f64(10), Min: f64(0), Max: f64(100),
						},
					},
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 5,
					MinPopulation:   u64(2),
					MaxPopulation:   u64(10),
					InitialState:    "hungry",
					Placement:       &datamodels.Placement{Type: datamodels.PlacementRandom},
					CountMode:       datamodels.CountRandomRange,
				},
				LLMConfig: &datamodels.LLMAgentConfig{
					LLMConfig: datamodels.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
					Strategy:  datamodels.LLMAgentStrategy{LLMStrategy: datamodels.LLMStrategy{Type: datamodels.LLMStrategyDecision}},
				},
				Locked: true,
			},
		},
		Environment: &datamodels.Environment{
			UID:    datamodels.EnvironmentUID,
			Width:  100,
			Height: 100,
			Torus:  true,
			Shapes: []datamodels.Shape{
				{
					UID:     "lake",
					Name:    "Lake",
					Type:    datamodels.ShapeTypeCircle,
					Layer:   1,
					Visible: true,
					Initialization: datamodels.AgentInitialization{
						PopulationCount: 1,
						Placement:       &datamodels.Placement{Type: datamodels.PlacementManual},
					},
					Geometry: &datamodels.ShapeGeometry{
						Position: &common.Point{X: 50, Y: 50},
						Radius:   &datamodels.ScalarSpec{Value: u64(15)},
					},
					Locked: true,
				},
			},
			GlobalProperties: []datamodels.AgentProperty{
				{
					UID: "temperature", Type: datamodels.PropertyTypeNumber,
					Generation: &datamodels.PropertyGeneration{
						Mode: datamodels.GenModeRandom, Distribution: datamodels.DistChoice,
						Choices: []datamodels.ChoiceWeight{{Value: 10.0, Weight: 1}, {Value: 20.0, Weight: 2}},
					},
				},
			},
		},
		Rules: []datamodels.Rule{
			{
				ID:        "hunt",
				Name:      "Hunt",
				Priority:  10,
				AppliesTo: []string{"wolf"},
				Conditions: []datamodels.Condition{
					{Type: datamodels.ConditionTypeProximity, Operator: datamodels.ConditionOpLessThan, Radius: f64(2)},
				},
				Actions: []datamodels.Action{
					{Type: datamodels.ActionTypeMoveToward, TargetUID: "prey", Speed: f64(1.5)},
				},
				Enabled: true,
				Locked:  true,
			},
		},
		Interactions: []datamodels.Interaction{
			{
				ID:            "eat",
				Type:          datamodels.InteractionTypeAgentAgent,
				InitiatorUIDs: []string{"wolf"},
				TargetUIDs:    []string{"sheep"},
				Radius:        f64(1),
				Enabled:       true,
				Locked:        true,
			},
		},
		Events: []datamodels.EventDefinition{
			{
				UID:       "alarm",
				Name:      "Alarm",
				Scope:     datamodels.EventScopeLocal,
				Radius:    f64(5),
				Producers: []datamodels.EventProducer{datamodels.EventProducerAgent},
				TTLSteps:  3,
				LLMTarget: &common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
				Locked:    true,
			},
		},
		Images: []datamodels.ModelImage{
			{UID: "img1", Name: "layout", MimeType: "image/png", Data: "base64data"},
		},
		Widgets: []common.WidgetConfig{
			{
				ID: "energy_chart", Type: common.WidgetLineChart, Title: "Avg energy",
				Series: []common.WidgetSeries{{
					Label: "wolves", Color: "#FF0000",
					Source: common.WidgetSource{
						Type: common.WidgetSourceAgentProperty, PropertyUID: "energy",
						AgentUIDs: []string{"wolf"}, Aggregation: common.AggAvg,
					},
				}},
				Position: common.WidgetPosition{X: 0, Y: 0, W: 6, H: 4},
			},
			{
				ID: "log", Type: common.WidgetTextLog, Title: "Events",
				LogKinds: []common.WidgetLogKind{common.LogEvents, common.LogStateTransitions},
				Position: common.WidgetPosition{X: 6, Y: 0, W: 6, H: 4},
			},
		},
	}
}

func TestModelMapper_RoundTrip_EmptyModel(t *testing.T) {
	var mapper ModelMapper
	in := datamodels.Model{Name: "empty"}
	got := mapper.FromInputDTO(mapper.ToInputDTO(in))
	require.Equal(t, in.Name, got.Name)
	require.Nil(t, got.Agents)
	require.Nil(t, got.Rules)
	require.Nil(t, got.Interactions)
	require.Nil(t, got.Events)
	require.Nil(t, got.Environment)
}

func TestModelMapper_RoundTrip(t *testing.T) {
	m := sampleModel()
	var mapper ModelMapper
	got := mapper.FromInputDTO(mapper.ToInputDTO(m))

	require.Equal(t, m.Name, got.Name)
	require.Equal(t, m.Description, got.Description)
	require.Equal(t, m.SimulationConfig.UpdateMode, got.SimulationConfig.UpdateMode)
	require.Equal(t, m.IsSpatial, got.IsSpatial)
	require.Equal(t, m.SpatialCellSize, got.SpatialCellSize)

	// LLM config (model generation + full simulation config)
	require.Equal(t, m.LLMConfig, got.LLMConfig)
	require.Equal(t, m.SimulationConfig, got.SimulationConfig)

	// Agents
	require.Len(t, got.Agents, 1)
	require.Equal(t, m.Agents[0].UID, got.Agents[0].UID)
	require.Equal(t, m.Agents[0].Type, got.Agents[0].Type)
	require.Equal(t, m.Agents[0].Locked, got.Agents[0].Locked)
	require.Equal(t, m.Agents[0].Properties, got.Agents[0].Properties)
	require.Equal(t, m.Agents[0].Initialization, got.Agents[0].Initialization)
	require.Equal(t, m.Agents[0].LLMConfig, got.Agents[0].LLMConfig)

	// Environment + shape geometry
	require.NotNil(t, got.Environment)
	require.Equal(t, m.Environment.UID, got.Environment.UID)
	require.Equal(t, m.Environment.Width, got.Environment.Width)
	require.Equal(t, m.Environment.Shapes[0].Locked, got.Environment.Shapes[0].Locked)
	require.Equal(t, m.Environment.Shapes[0].Geometry.Position, got.Environment.Shapes[0].Geometry.Position)
	require.Equal(t, m.Environment.Shapes[0].Geometry.Radius, got.Environment.Shapes[0].Geometry.Radius)
	require.Equal(t, m.Environment.GlobalProperties, got.Environment.GlobalProperties)

	// New model extensions: property generation, count mode, widgets.
	require.Equal(t, m.Agents[0].Properties[0].Generation, got.Agents[0].Properties[0].Generation)
	require.Equal(t, m.Agents[0].Initialization.CountMode, got.Agents[0].Initialization.CountMode)
	require.Equal(t, m.Environment.GlobalProperties[0].Generation, got.Environment.GlobalProperties[0].Generation)
	require.Equal(t, m.Widgets, got.Widgets)

	// Rules / interactions / events / images
	require.Equal(t, m.Rules, got.Rules)
	require.Equal(t, m.Interactions, got.Interactions)
	require.Equal(t, m.Events, got.Events)
	require.Equal(t, m.Images, got.Images)

	// Full deep round trip of the editable body (ignoring server-owned fields,
	// which ToInputDTO/FromInputDTO intentionally drop).
	want := m
	want.ID = ""
	want.ChatID = ""
	want.CreatedAt = got.CreatedAt
	want.UpdatedAt = got.UpdatedAt
	want.CreatedBy = ""
	want.UpdatedBy = ""
	require.Equal(t, want, got)
}
