package simulations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ===================== helpers =====================

func ptrUint64(v uint64) *uint64 { return &v }

// ===================== fixtures =====================

func buildFixtureSim() datasims.Simulation {
	seed := uint64(42)
	steps := uint64(500)
	timeoutMs := uint64(30000)
	popCount := uint64(10)
	minPop := uint64(5)
	maxPop := uint64(20)
	w := uint64(100)
	h := uint64(50)
	tru := true
	fals := false
	thick := uint(2)
	sw := uint(1)
	mean := 3.14
	stddev := 0.5
	minv := 0.0
	maxv := 10.0

	now := time.Now().UTC().Truncate(time.Millisecond)
	startedAt := now.Add(-time.Hour)
	finishedAt := now.Add(-30 * time.Minute)

	providerUID := "openai"
	modelUID := "gpt-4o"
	effort := common.LLMEffortHigh

	// A double ConfigValue for generation params.
	temperature := 0.7
	topP := 0.9

	llmTarget := common.LLMTarget{
		ProviderUID: providerUID,
		ModelUID:    modelUID,
		Effort:      effort,
		TimeoutMs:   &timeoutMs,
		Generation: common.GenerationParams{
			"temperature": {Double: &temperature},
			"top_p":       {Double: &topP},
		},
	}

	// Two agent instances.
	agentPos := common.Point{X: 10, Y: 20}
	agents := []datasims.SimulationAgent{
		{
			ID:       1,
			AgentUID: "wolf",
			Name:     "Wolf #1",
			Position: &agentPos,
			Properties: map[string]interface{}{
				"energy": 80.0,
				"age":    5.0,
			},
			State: "hunting",
		},
		{
			ID:       2,
			AgentUID: "sheep",
			Name:     "Sheep #1",
			Position: nil,
			Properties: map[string]interface{}{
				"energy": 60.0,
			},
			State: "grazing",
		},
	}

	// Environment with two shapes.
	circlePos := common.Point{X: 50, Y: 50}
	radius := 15.0
	polyPos := common.Point{X: 0, Y: 0}
	env := datasims.SimulationEnvironment{
		Width:  200,
		Height: 200,
		Torus:  true,
		GlobalProperties: map[string]interface{}{
			"season": "summer",
		},
		Shapes: []datasims.SimulationShape{
			{
				ID:       1,
				ShapeUID: "water_hole",
				Name:     "Water Hole",
				Type:     datamodels.ShapeTypeCircle,
				Layer:    1,
				Visible:  true,
				Properties: map[string]interface{}{
					"capacity": 100.0,
				},
				Position: circlePos,
				Radius:   &radius,
				Fill:     &tru,
				Stroke:   &fals,
			},
			{
				ID:         2,
				ShapeUID:   "fence",
				Name:       "Fence",
				Type:       datamodels.ShapeTypePolygon,
				Layer:      2,
				Visible:    true,
				Properties: map[string]interface{}{},
				Position:   polyPos,
				Points: []common.Point{
					{X: 10, Y: 10},
					{X: 20, Y: 10},
					{X: 20, Y: 20},
				},
				Thickness:   &thick,
				StrokeWidth: &sw,
			},
		},
	}

	// Two widgets.
	buckets := uint(10)
	widgets := []common.WidgetConfig{
		{
			ID:    "w1",
			Type:  common.WidgetMonitor,
			Title: "Energy Monitor",
			Source: &common.WidgetSource{
				Type:        common.WidgetSourceAgentProperty,
				PropertyUID: "energy",
				AgentUIDs:   []string{"wolf"},
				Aggregation: common.AggAvg,
			},
			Position: common.WidgetPosition{X: 0, Y: 0, W: 3, H: 2},
		},
		{
			ID:    "w2",
			Type:  common.WidgetHistogram,
			Title: "Age Distribution",
			Source: &common.WidgetSource{
				Type:        common.WidgetSourceAgentProperty,
				PropertyUID: "age",
				AgentUIDs:   []string{"wolf"},
			},
			Buckets:  &buckets,
			Position: common.WidgetPosition{X: 3, Y: 0, W: 3, H: 2},
		},
	}

	// Model snapshot with 2 archetypes.
	genSpec := &datamodels.PropertyGeneration{
		Mode:         datamodels.GenModeRandom,
		Distribution: datamodels.DistNormal,
		Mean:         &mean,
		StdDev:       &stddev,
		Min:          &minv,
		Max:          &maxv,
	}
	snapshot := &datamodels.Model{
		ID:        "model-001",
		Name:      "Wolf-Sheep Predation",
		IsSpatial: true,
		Agents: []datamodels.Agent{
			{
				UID:      "wolf",
				Name:     "Wolf",
				Type:     datamodels.AgentTypeArchetype,
				MoveType: datamodels.AgentMoveTypeMovable,
				States:   []string{"hunting", "resting"},
				Properties: []datamodels.AgentProperty{
					{
						UID:        "energy",
						Name:       "Energy",
						Type:       datamodels.PropertyTypeNumber,
						MinValue:   &minv,
						MaxValue:   &maxv,
						Generation: genSpec,
					},
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: popCount,
					MinPopulation:   &minPop,
					MaxPopulation:   &maxPop,
					InitialState:    "hunting",
					CountMode:       datamodels.CountRandomRange,
					Placement: &datamodels.Placement{
						Type: datamodels.PlacementRandom,
					},
				},
			},
			{
				UID:      "sheep",
				Name:     "Sheep",
				Type:     datamodels.AgentTypeArchetype,
				MoveType: datamodels.AgentMoveTypeMovable,
				States:   []string{"grazing", "fleeing"},
				Properties: []datamodels.AgentProperty{
					{UID: "energy", Name: "Energy", Type: datamodels.PropertyTypeNumber},
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: popCount,
					InitialState:    "grazing",
					CountMode:       datamodels.CountFixed,
				},
			},
		},
		Environment: &datamodels.Environment{
			UID:    "environment",
			Width:  uint64(w),
			Height: uint64(h),
			Torus:  false,
			Shapes: []datamodels.Shape{
				{
					UID:         "water_hole",
					Name:        "Water Hole",
					Description: "A circular watering hole",
					Type:        datamodels.ShapeTypeCircle,
					Layer:       1,
					Visible:     true,
					Initialization: datamodels.AgentInitialization{
						PopulationCount: 1,
						CountMode:       datamodels.CountFixed,
					},
					// Circle with a fixed Radius (ScalarSpec.Value set).
					Geometry: &datamodels.ShapeGeometry{
						Radius: &datamodels.ScalarSpec{Value: ptrUint64(25)},
						Fill:   &tru,
					},
				},
				{
					UID:     "meadow",
					Name:    "Meadow",
					Type:    datamodels.ShapeTypeRectangle,
					Layer:   2,
					Visible: true,
					Initialization: datamodels.AgentInitialization{
						PopulationCount: 1,
						CountMode:       datamodels.CountFixed,
					},
					// Rectangle with Width/Height as Min/Max range (no fixed Value).
					Geometry: &datamodels.ShapeGeometry{
						Width:  &datamodels.ScalarSpec{Min: ptrUint64(40), Max: ptrUint64(80)},
						Height: &datamodels.ScalarSpec{Min: ptrUint64(20), Max: ptrUint64(50)},
					},
				},
			},
		},
	}

	// Two generation runs.
	completedAt := now.Add(-45 * time.Minute)
	failedAt := now.Add(-35 * time.Minute)
	runs := []datasims.GenerationRun{
		{
			Seq:        1,
			Seed:       111,
			Scope:      datasims.GenerationScope{Kind: datasims.ScopeFull},
			Status:     datasims.RunCompleted,
			StartedAt:  now.Add(-time.Hour),
			FinishedAt: &completedAt,
			Requests:   5,
		},
		{
			Seq:  2,
			Seed: 222,
			Scope: datasims.GenerationScope{
				Kind:      datasims.ScopeAgents,
				AgentUIDs: []string{"wolf"},
				Parts:     []datasims.GenerationPart{datasims.PartProperties},
			},
			Status:     datasims.RunFailed,
			StartedAt:  now.Add(-40 * time.Minute),
			FinishedAt: &failedAt,
			Error:      "LLM timeout",
			Warnings: []datasims.GenerationWarning{
				{Path: "agents[0].properties.energy", Reason: "value clamped to max"},
			},
			Requests: 2,
		},
	}

	// Token usage — all 5 dimensions.
	tokenUsage := &datasims.SimulationTokenUsage{
		Total: common.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
			Calls:            7,
			LatencyMs:        2500.0,
		},
		ByModel: []common.TokenUsage{
			{
				LLMTarget:        llmTarget,
				PromptTokens:     1000,
				CompletionTokens: 500,
				TotalTokens:      1500,
				Calls:            7,
				LatencyMs:        2500.0,
			},
		},
		ByAgent: []datasims.AgentTokenUsage{
			{
				AgentUID: "wolf",
				TokenUsage: common.TokenUsage{
					PromptTokens:     600,
					CompletionTokens: 300,
					TotalTokens:      900,
					Calls:            4,
					LatencyMs:        1500.0,
				},
			},
		},
		ByPurpose: []datasims.PurposeTokenUsage{
			{
				Purpose: datasims.LLMUsageSimulationGeneration,
				TokenUsage: common.TokenUsage{
					PromptTokens:     1000,
					CompletionTokens: 500,
					TotalTokens:      1500,
					Calls:            7,
					LatencyMs:        2500.0,
				},
			},
		},
		ByGeneration: []common.TokenUsage{
			{
				LLMTarget:        llmTarget,
				PromptTokens:     800,
				CompletionTokens: 400,
				TotalTokens:      1200,
				Calls:            5,
				LatencyMs:        2000.0,
			},
			{
				PromptTokens:     200,
				CompletionTokens: 100,
				TotalTokens:      300,
				Calls:            2,
				LatencyMs:        500.0,
			},
		},
	}

	return datasims.Simulation{
		ID:                          "sim-001",
		Name:                        "Predation Run 1",
		Description:                 "Test simulation",
		ModelID:                     "model-001",
		ModelSnapshot:               snapshot,
		Seed:                        seed,
		Steps:                       steps,
		LLMGenerationTargetOverride: &llmTarget,
		LLMExecutionTargetOverride:  &llmTarget,
		Agents:                      agents,
		Environment:                 &env,
		Widgets:                     widgets,
		Status:                      datasims.SimulationStatusReady,
		StatusText:                  "Ready for execution",
		StatusHistory: []datasims.SimulationStatusChange{
			{Status: datasims.SimulationStatusGenerating, At: startedAt, By: "user-1"},
			{Status: datasims.SimulationStatusReady, At: finishedAt, By: "system"},
		},
		GenerationRuns: runs,
		TokenUsage:     tokenUsage,
		CurrentStep:    0,
		CreatedAt:      startedAt,
		CreatedBy:      "user-1",
		UpdatedAt:      finishedAt,
		UpdatedBy:      "system",
		StartedAt:      &startedAt,
		FinishedAt:     nil,
	}
}

// ===================== tests =====================

func TestSimulationToResponse_FullFixture(t *testing.T) {
	sim := buildFixtureSim()
	validation := ValidationResult{
		Errors:     nil,
		Warnings:   []ValidationIssue{{Path: "agents[0].state", Reason: "unrecognised state"}},
		StartReady: true,
	}

	resp := simulationToResponse(sim, validation)

	// --- top-level scalars ---
	assert.Equal(t, "sim-001", resp.ID)
	assert.Equal(t, "Predation Run 1", resp.Name)
	assert.Equal(t, "Test simulation", resp.Description)
	assert.Equal(t, "model-001", resp.ModelID)
	assert.Equal(t, uint64(42), resp.Seed)
	assert.Equal(t, uint64(500), resp.Steps)
	assert.Equal(t, "ready", resp.Status)
	assert.Equal(t, "Ready for execution", resp.StatusText)
	assert.True(t, resp.Validation.StartReady)

	// --- LLM targets (generation + execution overrides) ---
	require.NotNil(t, resp.LLMGenerationTarget)
	assert.Equal(t, "openai", resp.LLMGenerationTarget.ProviderUID)
	assert.Equal(t, "gpt-4o", resp.LLMGenerationTarget.ModelUID)
	assert.Equal(t, "high", resp.LLMGenerationTarget.Effort)
	require.NotNil(t, resp.LLMGenerationTarget.TimeoutMs)
	assert.Equal(t, uint64(30000), *resp.LLMGenerationTarget.TimeoutMs)
	assert.Len(t, resp.LLMGenerationTarget.Generation, 2)

	require.NotNil(t, resp.LLMExecutionTarget)
	assert.Equal(t, "openai", resp.LLMExecutionTarget.ProviderUID)
	assert.Equal(t, "gpt-4o", resp.LLMExecutionTarget.ModelUID)
	assert.Equal(t, "high", resp.LLMExecutionTarget.Effort)
	require.NotNil(t, resp.LLMExecutionTarget.TimeoutMs)
	assert.Equal(t, uint64(30000), *resp.LLMExecutionTarget.TimeoutMs)
	assert.Len(t, resp.LLMExecutionTarget.Generation, 2)

	// --- agents ---
	require.Len(t, resp.Agents, 2)
	a0 := resp.Agents[0]
	assert.Equal(t, uint64(1), a0.ID)
	assert.Equal(t, "wolf", a0.AgentUID)
	assert.Equal(t, "Wolf #1", a0.Name)
	assert.Equal(t, "hunting", a0.State)
	require.NotNil(t, a0.Position)
	assert.Equal(t, uint64(10), a0.Position.X)
	assert.Equal(t, uint64(20), a0.Position.Y)

	a1 := resp.Agents[1]
	assert.Equal(t, uint64(2), a1.ID)
	assert.Nil(t, a1.Position)

	// --- environment ---
	require.NotNil(t, resp.Environment)
	assert.Equal(t, uint64(200), resp.Environment.Width)
	assert.Equal(t, uint64(200), resp.Environment.Height)
	assert.True(t, resp.Environment.Torus)
	require.Len(t, resp.Environment.Shapes, 2)

	sh0 := resp.Environment.Shapes[0]
	assert.Equal(t, uint64(1), sh0.ID)
	assert.Equal(t, "water_hole", sh0.ShapeUID)
	assert.Equal(t, "circle", sh0.Type)
	require.NotNil(t, sh0.Radius)
	assert.InDelta(t, 15.0, *sh0.Radius, 0.001)
	require.NotNil(t, sh0.Fill)
	assert.True(t, *sh0.Fill)

	sh1 := resp.Environment.Shapes[1]
	assert.Equal(t, "polygon", sh1.Type)
	assert.Len(t, sh1.Points, 3)

	// --- widgets ---
	require.Len(t, resp.Widgets, 2)
	assert.Equal(t, "w1", resp.Widgets[0].ID)
	assert.Equal(t, "monitor", resp.Widgets[0].Type)

	// --- status history ---
	require.Len(t, resp.StatusHistory, 2)
	assert.Equal(t, "generating", resp.StatusHistory[0].Status)
	assert.Equal(t, "ready", resp.StatusHistory[1].Status)

	// --- generation runs ---
	require.Len(t, resp.GenerationRuns, 2)
	run0 := resp.GenerationRuns[0]
	assert.Equal(t, uint64(1), run0.Seq)
	assert.Equal(t, uint64(111), run0.Seed)
	assert.Equal(t, "full", run0.Scope.Kind)
	assert.Equal(t, "completed", run0.Status)
	assert.Empty(t, run0.Warnings)

	run1 := resp.GenerationRuns[1]
	assert.Equal(t, uint64(222), run1.Seed)
	assert.Equal(t, "failed", run1.Status)
	assert.Equal(t, "LLM timeout", run1.Error)
	require.Len(t, run1.Warnings, 1)
	assert.Equal(t, "agents[0].properties.energy", run1.Warnings[0].Path)

	// --- token usage ---
	require.NotNil(t, resp.TokenUsage)
	assert.Equal(t, uint64(1500), resp.TokenUsage.Total.TotalTokens)
	assert.Equal(t, uint64(7), resp.TokenUsage.Total.Calls)
	require.Len(t, resp.TokenUsage.ByModel, 1)
	assert.Equal(t, "openai", resp.TokenUsage.ByModel[0].ProviderUID)
	require.Len(t, resp.TokenUsage.ByAgent, 1)
	assert.Equal(t, "wolf", resp.TokenUsage.ByAgent[0].AgentUID)
	require.Len(t, resp.TokenUsage.ByPurpose, 1)
	assert.Equal(t, "simulation_generation", resp.TokenUsage.ByPurpose[0].Purpose)
	require.Len(t, resp.TokenUsage.ByGeneration, 2)

	// --- model snapshot ---
	require.NotNil(t, resp.ModelSnapshot)
	assert.Equal(t, "Wolf-Sheep Predation", resp.ModelSnapshot.Name)
	assert.True(t, resp.ModelSnapshot.IsSpatial)
	require.Len(t, resp.ModelSnapshot.Agents, 2)

	snapAgent := resp.ModelSnapshot.Agents[0]
	assert.Equal(t, "wolf", snapAgent.UID)
	assert.Equal(t, "archetype", snapAgent.Type)
	assert.Equal(t, "movable", snapAgent.MoveType)
	assert.Equal(t, "random_range", snapAgent.Initialization.CountMode)
	require.NotNil(t, snapAgent.Initialization.Placement)
	assert.Equal(t, "random", snapAgent.Initialization.Placement.Type)
	require.Len(t, snapAgent.Properties, 1)
	require.NotNil(t, snapAgent.Properties[0].Generation)
	assert.Equal(t, "random", snapAgent.Properties[0].Generation.Mode)
	assert.Equal(t, "normal", snapAgent.Properties[0].Generation.Distribution)

	require.NotNil(t, resp.ModelSnapshot.Environment)
	assert.Equal(t, uint64(100), resp.ModelSnapshot.Environment.Width)
	require.Len(t, resp.ModelSnapshot.Environment.Shapes, 2)

	// Shape 0: circle with fixed Radius ScalarSpec.Value — must be preserved losslessly.
	snapCircle := resp.ModelSnapshot.Environment.Shapes[0]
	assert.Equal(t, "water_hole", snapCircle.UID)
	assert.Equal(t, "A circular watering hole", snapCircle.Description)
	assert.Equal(t, "circle", snapCircle.Type)
	require.NotNil(t, snapCircle.Geometry, "circle shape must have geometry")
	require.NotNil(t, snapCircle.Geometry.Radius, "circle geometry must have radius")
	require.NotNil(t, snapCircle.Geometry.Radius.Value, "circle radius must have fixed Value")
	assert.Equal(t, uint64(25), *snapCircle.Geometry.Radius.Value)
	assert.Nil(t, snapCircle.Geometry.Radius.Min, "fixed-value radius must not have Min")
	assert.Nil(t, snapCircle.Geometry.Radius.Max, "fixed-value radius must not have Max")
	require.NotNil(t, snapCircle.Geometry.Fill)
	assert.True(t, *snapCircle.Geometry.Fill)

	// Shape 1: rectangle with Width/Height as Min/Max range — must be preserved losslessly.
	snapRect := resp.ModelSnapshot.Environment.Shapes[1]
	assert.Equal(t, "meadow", snapRect.UID)
	assert.Equal(t, "rectangle", snapRect.Type)
	require.NotNil(t, snapRect.Geometry, "rectangle shape must have geometry")
	require.NotNil(t, snapRect.Geometry.Width, "rectangle geometry must have width")
	assert.Nil(t, snapRect.Geometry.Width.Value, "range width must not have fixed Value")
	require.NotNil(t, snapRect.Geometry.Width.Min)
	require.NotNil(t, snapRect.Geometry.Width.Max)
	assert.Equal(t, uint64(40), *snapRect.Geometry.Width.Min)
	assert.Equal(t, uint64(80), *snapRect.Geometry.Width.Max)
	require.NotNil(t, snapRect.Geometry.Height, "rectangle geometry must have height")
	assert.Nil(t, snapRect.Geometry.Height.Value, "range height must not have fixed Value")
	require.NotNil(t, snapRect.Geometry.Height.Min)
	require.NotNil(t, snapRect.Geometry.Height.Max)
	assert.Equal(t, uint64(20), *snapRect.Geometry.Height.Min)
	assert.Equal(t, uint64(50), *snapRect.Geometry.Height.Max)

	// --- validation ---
	assert.Empty(t, resp.Validation.Errors)
	require.Len(t, resp.Validation.Warnings, 1)
	assert.Equal(t, "agents[0].state", resp.Validation.Warnings[0].Path)
}

// TestSimulationToResponse_NilOptionals ensures nil optional fields produce
// nil DTOs without panic.
func TestSimulationToResponse_NilOptionals(t *testing.T) {
	sim := datasims.Simulation{
		ID:      "sim-empty",
		ModelID: "model-001",
		Status:  datasims.SimulationStatusReady,
		Steps:   100,
		// ModelSnapshot, Environment, TokenUsage, both LLM target overrides all nil.
	}
	validation := ValidationResult{StartReady: false}

	require.NotPanics(t, func() {
		resp := simulationToResponse(sim, validation)
		assert.Nil(t, resp.ModelSnapshot)
		assert.Nil(t, resp.Environment)
		assert.Nil(t, resp.TokenUsage)
		assert.Nil(t, resp.LLMGenerationTarget)
		assert.Nil(t, resp.LLMExecutionTarget)
		assert.Nil(t, resp.Agents)
		assert.Nil(t, resp.Widgets)
		assert.False(t, resp.Validation.StartReady)
	})
}

// TestRoundTrip_AgentsEnvironmentWidgets tests that agentsFromDTO(agentsToDTO(...))
// and environmentFromDTO(environmentToDTO(...)) and widgetsFromDTO(widgetsToDTO(...))
// produce deep-equal originals.
func TestRoundTrip_AgentsEnvironmentWidgets(t *testing.T) {
	sim := buildFixtureSim()

	// Agents round-trip.
	agentDTOs := agentsToDTO(sim.Agents)
	agentsBack := agentsFromDTO(agentDTOs)
	require.Len(t, agentsBack, len(sim.Agents))
	for i, orig := range sim.Agents {
		back := agentsBack[i]
		assert.Equal(t, orig.ID, back.ID)
		assert.Equal(t, orig.AgentUID, back.AgentUID)
		assert.Equal(t, orig.Name, back.Name)
		assert.Equal(t, orig.State, back.State)
		if orig.Position != nil {
			require.NotNil(t, back.Position)
			assert.Equal(t, *orig.Position, *back.Position)
		} else {
			assert.Nil(t, back.Position)
		}
		assert.Equal(t, orig.Properties, back.Properties)
	}

	// Environment round-trip.
	envDTO := environmentToDTO(sim.Environment)
	envBack := environmentFromDTO(envDTO)
	require.NotNil(t, envBack)
	assert.Equal(t, sim.Environment.Width, envBack.Width)
	assert.Equal(t, sim.Environment.Height, envBack.Height)
	assert.Equal(t, sim.Environment.Torus, envBack.Torus)
	assert.Equal(t, sim.Environment.GlobalProperties, envBack.GlobalProperties)
	require.Len(t, envBack.Shapes, len(sim.Environment.Shapes))

	// Verify first shape (circle).
	origShape := sim.Environment.Shapes[0]
	backShape := envBack.Shapes[0]
	assert.Equal(t, origShape.ID, backShape.ID)
	assert.Equal(t, origShape.ShapeUID, backShape.ShapeUID)
	assert.Equal(t, origShape.Type, backShape.Type)
	assert.Equal(t, origShape.Position, backShape.Position)
	require.NotNil(t, backShape.Radius)
	assert.Equal(t, *origShape.Radius, *backShape.Radius)

	// Verify second shape (polygon with points).
	origPoly := sim.Environment.Shapes[1]
	backPoly := envBack.Shapes[1]
	require.Len(t, backPoly.Points, len(origPoly.Points))
	for j, op := range origPoly.Points {
		assert.Equal(t, op, backPoly.Points[j])
	}

	// Widgets round-trip.
	widgetDTOs := widgetsToDTO(sim.Widgets)
	widgetsBack := widgetsFromDTO(widgetDTOs)
	require.Len(t, widgetsBack, len(sim.Widgets))
	for i, orig := range sim.Widgets {
		back := widgetsBack[i]
		assert.Equal(t, orig.ID, back.ID)
		assert.Equal(t, orig.Type, back.Type)
		assert.Equal(t, orig.Title, back.Title)
		assert.Equal(t, orig.Position, back.Position)
		if orig.Source != nil {
			require.NotNil(t, back.Source)
			assert.Equal(t, *orig.Source, *back.Source)
		}
		if orig.Buckets != nil {
			require.NotNil(t, back.Buckets)
			assert.Equal(t, *orig.Buckets, *back.Buckets)
		}
	}
}

// TestAgentsToDTO_Memory asserts that AgentMemory entries are mapped to their
// compact DTO form (Step/Type/Summary only; Details/Timestamp are excluded) and
// that agents with no memory produce a nil Memory field (omitempty).
func TestAgentsToDTO_Memory(t *testing.T) {
	agents := []datasims.SimulationAgent{
		{
			ID:       1,
			AgentUID: "wolf",
			Name:     "Wolf #1",
			State:    "hunting",
			Memory: []datasims.AgentMemory{
				{Step: 3, Type: datasims.AgentMemoryDecision, Summary: "chose to hunt"},
				{Step: 5, Type: datasims.AgentMemoryRule, Summary: "rule fired: energy_low"},
				{Step: 7, Type: datasims.AgentMemoryLLM, Summary: "llm: move north"},
			},
		},
		{
			ID:       2,
			AgentUID: "sheep",
			Name:     "Sheep #1",
			State:    "grazing",
			// No memory — dto.Memory must be nil (omitempty).
		},
	}

	dtos := agentsToDTO(agents)

	require.Len(t, dtos, 2)

	// Agent with memory.
	a0 := dtos[0]
	assert.Equal(t, uint64(1), a0.ID)
	require.Len(t, a0.Memory, 3)
	assert.Equal(t, uint64(3), a0.Memory[0].Step)
	assert.Equal(t, "decision", a0.Memory[0].Type)
	assert.Equal(t, "chose to hunt", a0.Memory[0].Summary)
	assert.Equal(t, uint64(5), a0.Memory[1].Step)
	assert.Equal(t, "rule", a0.Memory[1].Type)
	assert.Equal(t, "rule fired: energy_low", a0.Memory[1].Summary)
	assert.Equal(t, uint64(7), a0.Memory[2].Step)
	assert.Equal(t, "llm_response", a0.Memory[2].Type)
	assert.Equal(t, "llm: move north", a0.Memory[2].Summary)

	// Agent without memory — field must be nil so it is omitted from JSON.
	a1 := dtos[1]
	assert.Equal(t, uint64(2), a1.ID)
	assert.Nil(t, a1.Memory)
}

// TestRoundTrip_LLMTarget tests llmTargetFromDTO(llmTargetToDTO(..)) round-trip.
func TestRoundTrip_LLMTarget(t *testing.T) {
	sim := buildFixtureSim()
	orig := *sim.LLMGenerationTargetOverride

	dto := llmTargetToDTO(orig)
	back := llmTargetFromDTO(&dto)
	require.NotNil(t, back)
	assert.Equal(t, orig.ProviderUID, back.ProviderUID)
	assert.Equal(t, orig.ModelUID, back.ModelUID)
	assert.Equal(t, orig.Effort, back.Effort)
	require.NotNil(t, back.TimeoutMs)
	assert.Equal(t, *orig.TimeoutMs, *back.TimeoutMs)
	assert.Len(t, back.Generation, len(orig.Generation))
}

// TestScopeRoundTrip tests scopeFromDTO(scopeToDTO(...)).
func TestScopeRoundTrip(t *testing.T) {
	orig := datasims.GenerationScope{
		Kind:      datasims.ScopeAgents,
		AgentUIDs: []string{"wolf", "sheep"},
		Parts:     []datasims.GenerationPart{datasims.PartProperties, datasims.PartPlacement},
	}
	dto := scopeToDTO(orig)
	// The manual field copy below bridges GenerationScopeResponseDTO (response form) and
	// GenerationScopeDTO (request form); they share the same field names today.
	reqDTO := GenerationScopeDTO{
		Kind:      dto.Kind,
		AgentUIDs: dto.AgentUIDs,
		Parts:     dto.Parts,
	}
	back := scopeFromDTO(reqDTO)
	assert.Equal(t, orig.Kind, back.Kind)
	assert.Equal(t, orig.AgentUIDs, back.AgentUIDs)
	require.Len(t, back.Parts, len(orig.Parts))
	for i, p := range orig.Parts {
		assert.Equal(t, p, back.Parts[i])
	}
}

// TestNilEnvironment ensures environmentToDTO(nil) returns nil without panic.
func TestNilEnvironment(t *testing.T) {
	require.NotPanics(t, func() {
		dto := environmentToDTO(nil)
		assert.Nil(t, dto)
	})
}

// TestNilTokenUsage ensures simulationTokenUsageToDTO(nil) returns nil without panic.
func TestNilTokenUsage(t *testing.T) {
	require.NotPanics(t, func() {
		dto := simulationTokenUsageToDTO(nil)
		assert.Nil(t, dto)
	})
}

// TestNilModelSnapshot ensures modelSnapshotToDTO(nil) returns nil without panic.
func TestNilModelSnapshot(t *testing.T) {
	require.NotPanics(t, func() {
		dto := modelSnapshotToDTO(nil)
		assert.Nil(t, dto)
	})
}

// TestSummaryToDTO verifies summaryToDTO maps every field.
func TestSummaryToDTO(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	s := datasims.SimulationSummary{
		ID:          "s1",
		Name:        "Run A",
		Description: "test",
		ModelID:     "m1",
		ModelName:   "MyModel",
		Status:      datasims.SimulationStatusFinished,
		Steps:       1000,
		AgentCount:  50,
		TotalTokens: 9999,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "user-x",
	}
	dto := summaryToDTO(s)
	assert.Equal(t, "s1", dto.ID)
	assert.Equal(t, "Run A", dto.Name)
	assert.Equal(t, "test", dto.Description)
	assert.Equal(t, "m1", dto.ModelID)
	assert.Equal(t, "MyModel", dto.ModelName)
	assert.Equal(t, "finished", dto.Status)
	assert.Equal(t, uint64(1000), dto.Steps)
	assert.Equal(t, 50, dto.AgentCount)
	assert.Equal(t, uint64(9999), dto.TotalTokens)
	assert.Equal(t, now, dto.CreatedAt)
	assert.Equal(t, now, dto.UpdatedAt)
	assert.Equal(t, "user-x", dto.CreatedBy)
}
