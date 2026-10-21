package simulations

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	"intelli.sim.core/data/simulations"
)

// newFixtureSimulation builds a complete Simulation entity covering all
// fields: model snapshot, 2 archetypes (2 + 1 agents with positions +
// multi-type properties), environment with shapes, 2 widgets, 2 generation
// runs (one completed with warnings, one failed with error), token usage
// across all 5 dimensions, status history, both generation + execution LLM
// target overrides, and nil StartedAt/FinishedAt.
func newFixtureSimulation() simulations.Simulation {
	modelID := bson.NewObjectID().Hex()
	simID := bson.NewObjectID().Hex()

	// compact model snapshot
	minV, maxV := 0.0, 100.0
	energy := 50.0
	snapshot := models.Model{
		ID:   modelID,
		Name: "predator-prey",
		LLMConfig: models.LLMConfig{
			Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
		},
		IsSpatial: true,
		Agents: []models.Agent{
			{
				UID: "predator", Name: "Predator",
				Type:     models.AgentTypeArchetype,
				MoveType: models.AgentMoveTypeMovable,
				Properties: []models.AgentProperty{{
					UID:          "energy",
					Name:         "Energy",
					Type:         models.PropertyTypeNumber,
					MinValue:     &minV,
					MaxValue:     &maxV,
					DefaultValue: energy,
				}},
				States:         []string{"hunting", "resting"},
				Initialization: models.AgentInitialization{PopulationCount: 10, InitialState: "hunting"},
			},
			{
				UID: "prey", Name: "Prey",
				Type:           models.AgentTypeArchetype,
				MoveType:       models.AgentMoveTypeMovable,
				Initialization: models.AgentInitialization{PopulationCount: 20, InitialState: "grazing"},
			},
		},
		Environment: &models.Environment{
			UID: models.EnvironmentUID, Width: 100, Height: 100,
		},
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	finishedAt := now.Add(10 * time.Second)

	// generation + execution timeout overrides
	timeoutMs := uint64(30000)
	genOverride := &common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini", TimeoutMs: &timeoutMs}
	execTimeoutMs := uint64(45000)
	execOverride := &common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh, TimeoutMs: &execTimeoutMs}

	return simulations.Simulation{
		ID:            simID,
		ModelID:       modelID,
		Name:          "run-1",
		Description:   "integration test run",
		ModelSnapshot: &snapshot,
		Seed:          42,
		Steps:         200,
		Termination: &models.Condition{
			Type:          models.ConditionTypeComposite,
			LogicOperator: models.LogicOpOR,
			SubConditions: []models.Condition{
				{Type: models.ConditionTypePopulation, Operator: models.ConditionOpEquals, Value: float64(0), TargetUIDs: []string{"prey"}},
				{Type: models.ConditionTypeStep, Operator: models.ConditionOpGreaterOrEq, Value: float64(150)},
			},
		},

		LLMGenerationTargetOverride: genOverride,
		LLMExecutionTargetOverride:  execOverride,

		Agents: []simulations.SimulationAgent{
			{
				ID:       1,
				AgentUID: "predator",
				Name:     "Predator #1",
				Position: &common.Point{X: 1, Y: 2},
				Properties: map[string]interface{}{
					"energy": float64(75),
					"alive":  true,
					"name":   "alpha",
				},
				State: "hunting",
				Memory: []simulations.AgentMemory{
					{Step: 1, Type: simulations.AgentMemoryDecision, Summary: "hunt prey", Timestamp: now},
					{Step: 2, Type: simulations.AgentMemoryMovement, Summary: "moved north", Timestamp: now},
				},
			},
			{
				ID:       2,
				AgentUID: "predator",
				Name:     "Predator #2",
				Position: &common.Point{X: 5, Y: 7},
				Properties: map[string]interface{}{
					"energy": float64(40),
					"alive":  true,
				},
				State: "resting",
			},
			{
				ID:         3,
				AgentUID:   "prey",
				Name:       "Prey #1",
				Properties: map[string]interface{}{"energy": float64(90), "alive": true},
				State:      "grazing",
			},
		},

		Environment: &simulations.SimulationEnvironment{
			Width:           100,
			Height:          100,
			Torus:           true,
			CollisionMode:   models.CollisionBlock,
			CollisionConfig: &models.CollisionConfig{MaxIterations: 12, StuckPolicy: models.CollisionStuckBestEffort},
			GlobalProperties: map[string]interface{}{
				"phase": "day",
				"temp":  float64(22),
			},
			Shapes: []simulations.SimulationShape{
				{
					ID:         1,
					ShapeUID:   "forest",
					Name:       "Forest",
					Type:       models.ShapeTypeRectangle,
					Visible:    true,
					Layer:      1,
					Position:   common.Point{X: 10, Y: 20},
					Width:      uint64Ptr(40),
					Height:     uint64Ptr(25),
					Properties: map[string]interface{}{"color": "#336633"},
				},
				{
					ID:       2,
					ShapeUID: "lake",
					Name:     "Lake",
					Type:     models.ShapeTypeCircle,
					Visible:  true,
					Position: common.Point{X: 60, Y: 60},
					Radius:   float64Ptr(15.5),
					Fill:     boolPtr(true),
					Points: []common.Point{
						{X: 1, Y: 2},
						{X: 3, Y: 4},
					},
				},
				{
					ID:          3,
					ShapeUID:    "river",
					Name:        "River",
					Type:        models.ShapeTypeLine,
					Visible:     true,
					Position:    common.Point{X: 0, Y: 40},
					Points:      []common.Point{{X: 0, Y: 0}, {X: 100, Y: 5}},
					Thickness:   uintPtr(3),
					Stroke:      boolPtr(true),
					StrokeWidth: uintPtr(2),
				},
			},
		},

		Widgets: []common.WidgetConfig{
			{
				ID:    "w1",
				Type:  common.WidgetLineChart,
				Title: "Energy over time",
				Source: &common.WidgetSource{
					Type:        common.WidgetSourceAgentProperty,
					PropertyUID: "energy",
					AgentUIDs:   []string{"predator"},
					Aggregation: common.AggAvg,
				},
				Position: common.WidgetPosition{X: 0, Y: 0, W: 6, H: 3},
			},
			{
				ID:       "w2",
				Type:     common.WidgetMonitor,
				Title:    "Population",
				Position: common.WidgetPosition{X: 6, Y: 0, W: 6, H: 3},
			},
		},

		Status:     simulations.SimulationStatusReady,
		StatusText: "ready to run",
		StatusHistory: []simulations.SimulationStatusChange{
			{Status: simulations.SimulationStatusGenerating, At: now, By: "u1"},
			{Status: simulations.SimulationStatusReady, At: now.Add(time.Second), By: "u1"},
		},

		GenerationRuns: []simulations.GenerationRun{
			{
				Seq:        1,
				Seed:       123,
				Scope:      simulations.GenerationScope{Kind: simulations.ScopeFull},
				Status:     simulations.RunCompleted,
				StartedAt:  now,
				FinishedAt: &finishedAt,
				Warnings: []simulations.GenerationWarning{
					{Path: "agents[0].properties.energy", Reason: "invalid LLM value; used fallback 50"},
				},
				Requests: 3,
			},
			{
				Seq:       2,
				Seed:      456,
				Scope:     simulations.GenerationScope{Kind: simulations.ScopeAgents, AgentUIDs: []string{"predator"}, Parts: []simulations.GenerationPart{simulations.PartProperties}},
				Status:    simulations.RunFailed,
				StartedAt: now.Add(time.Minute),
				Error:     "LLM timeout after 30s",
				Requests:  1,
			},
		},

		ExecutionRuns: []simulations.ExecutionRun{
			{
				Seq:               1,
				StartedAt:         now,
				FinishedAt:        &finishedAt,
				StepsCompleted:    200,
				Status:            simulations.ExecutionRunFinished,
				TerminationReason: simulations.TerminationMaxSteps,
			},
			{
				Seq:            2,
				StartedAt:      now.Add(2 * time.Minute),
				StepsCompleted: 17,
				Status:         simulations.ExecutionRunFailed,
				Error:          "engine panic at step 17",
			},
		},

		Statistics: &simulations.SimulationStatistics{
			StepsCompleted: 200,
			TotalAgents:    25,
			Population: []simulations.AgentPopulation{
				{
					AgentUID: "predator", Current: 8, Peak: 12, Min: 5, Spawned: 3, Removed: 2,
					ByState: map[string]uint64{"hunting": 5, "resting": 3},
				},
				{AgentUID: "prey", Current: 17, Peak: 20, Min: 14, Spawned: 4, Removed: 7},
			},
			AgentsSpawned:    7,
			AgentsRemoved:    9,
			Interactions:     42,
			Events:           5,
			RuleExecutions:   1024,
			MovementDistance: 3456.75,
			ByRule:           []simulations.RuleStatistic{{RuleID: "rule-1", Executions: 600}, {RuleID: "rule-2", Executions: 424}},
			ByEvent:          []simulations.EventStatistic{{EventUID: "born", Emitted: 4}},
			ByInteraction:    []simulations.InteractionStatistic{{InteractionID: "hunt", Count: 42}},
			Properties: []simulations.PropertyStatistic{
				{AgentUID: "predator", PropertyUID: "energy", Avg: 55.5, Min: 10, Max: 100},
			},
			Custom: map[string]interface{}{"avg_lifespan": float64(73)},
		},

		TokenUsage: &simulations.SimulationTokenUsage{
			Total: common.TokenUsage{TotalTokens: 500, PromptTokens: 300, CompletionTokens: 200, Calls: 5},
			ByModel: []common.TokenUsage{
				{
					LLMTarget:        common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
					TotalTokens:      500,
					PromptTokens:     300,
					CompletionTokens: 200,
					Calls:            5,
				},
			},
			ByAgent: []simulations.AgentTokenUsage{
				{AgentUID: "predator", TokenUsage: common.TokenUsage{TotalTokens: 300, Calls: 3}},
				{AgentUID: "prey", TokenUsage: common.TokenUsage{TotalTokens: 200, Calls: 2}},
			},
			ByPurpose: []simulations.PurposeTokenUsage{
				{Purpose: simulations.LLMUsageSimulationGeneration, TokenUsage: common.TokenUsage{TotalTokens: 400, Calls: 4}},
				{Purpose: simulations.LLMUsageSimulationInteraction, TokenUsage: common.TokenUsage{TotalTokens: 100, Calls: 1}},
			},
			ByGeneration: []common.TokenUsage{
				{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
					TotalTokens: 300,
					Calls:       3,
				},
				{
					TotalTokens: 200,
					Calls:       2,
				},
			},
			ByRun: []common.TokenUsage{
				{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
					TotalTokens: 120,
					Calls:       6,
				},
				{
					TotalTokens: 80,
					Calls:       4,
				},
			},
		},

		CurrentStep: 0,
		CreatedAt:   now,
		CreatedBy:   "u1",
		UpdatedAt:   now,
		UpdatedBy:   "u1",
		StartedAt:   nil,
		FinishedAt:  nil,
	}
}

// TestSimulationEntityDocumentRoundTrip verifies that a full Simulation entity
// survives the entity → document → bson.Marshal → bson.Unmarshal → document →
// entity round-trip without loss or mutation.
func TestSimulationEntityDocumentRoundTrip(t *testing.T) {
	in := newFixtureSimulation()

	doc, err := SimulationToDocument(in)
	if err != nil {
		t.Fatalf("SimulationToDocument: %v", err)
	}

	// Full bson encode/decode so that interface{} fields (properties, details)
	// go through the driver's type coercion — this catches bson.D / primitive.A
	// drift that only surfaces against a real (or in-process) bson round-trip.
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	var docAfterBSON SimulationDocument
	if err := bson.Unmarshal(raw, &docAfterBSON); err != nil {
		t.Fatalf("bson.Unmarshal: %v", err)
	}

	out := SimulationFromDocument(docAfterBSON)
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", out, in)
	}
}

// TestSimulationToDocumentInvalidID verifies that malformed IDs produce errors.
func TestSimulationToDocumentInvalidID(t *testing.T) {
	if _, err := SimulationToDocument(simulations.Simulation{ID: "nope"}); err == nil {
		t.Fatal("want error for malformed simulation id")
	}
	if _, err := SimulationToDocument(simulations.Simulation{ModelID: "nope"}); err == nil {
		t.Fatal("want error for malformed model id")
	}
}

// TestStepEntityDocumentRoundTrip verifies the SimulationStep mapper (used by
// steps_repository.go) for both a populated State and the zero-value (omitted)
// case, ensuring backward-compatible deserialization of old rows without State.
func TestStepEntityDocumentRoundTrip(t *testing.T) {
	t.Run("with state", func(t *testing.T) {
		in := simulations.SimulationStep{
			Metadata: simulations.StepMetadata{
				SimulationID: bson.NewObjectID().Hex(),
				Step:         3,
				ReferenceID:  1,
				Type:         simulations.MetadataTypeAgent,
				UID:          "predator",
			},
			Position:   &common.Point{X: 4, Y: 7},
			Properties: map[string]interface{}{"energy": float64(50)},
			State:      "hunting",
			Decision:   "chase prey",
		}
		doc, err := StepToDocument(in)
		if err != nil {
			t.Fatalf("StepToDocument: %v", err)
		}
		out := StepFromDocument(doc)
		if !reflect.DeepEqual(in, out) {
			t.Fatalf("step round-trip mismatch:\n got  %+v\n want %+v", out, in)
		}
	})

	t.Run("without state (backward compat)", func(t *testing.T) {
		in := simulations.SimulationStep{
			Metadata: simulations.StepMetadata{
				SimulationID: bson.NewObjectID().Hex(),
				Step:         5,
				ReferenceID:  2,
				Type:         simulations.MetadataTypeAgent,
				UID:          "prey",
			},
			Position:   &common.Point{X: 10, Y: 20},
			Properties: map[string]interface{}{"energy": float64(80)},
			// State intentionally omitted — simulates an old timeseries row
		}
		doc, err := StepToDocument(in)
		if err != nil {
			t.Fatalf("StepToDocument: %v", err)
		}
		out := StepFromDocument(doc)
		if !reflect.DeepEqual(in, out) {
			t.Fatalf("step round-trip mismatch:\n got  %+v\n want %+v", out, in)
		}
		if out.State != "" {
			t.Fatalf("expected empty State for old row, got %q", out.State)
		}
	})
}

// TestMemoryDetailsNormalizedOnRead verifies that bson.D in AgentMemoryDocument
// Details is converted to map[string]interface{} by memoryFromDocument.
func TestMemoryDetailsNormalizedOnRead(t *testing.T) {
	d := AgentMemoryDocument{
		Step:    1,
		Type:    simulations.AgentMemoryDecision,
		Details: bson.D{{Key: "target", Value: "prey"}},
	}
	m := memoryFromDocument(d)
	if _, ok := m.Details.(map[string]interface{}); !ok {
		t.Fatalf("Details not normalized: got %T, want map[string]interface{}", m.Details)
	}
}

// ---- helpers ----

func uint64Ptr(v uint64) *uint64    { return &v }
func float64Ptr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool          { return &v }
func uintPtr(v uint) *uint          { return &v }
