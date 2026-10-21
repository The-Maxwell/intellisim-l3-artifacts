package models

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
)

func newFixtureModel() models.Model {
	minV, maxV := 0.0, 100.0
	speed := 5.0
	forestWidth, forestHeight := uint64(40), uint64(25)
	clusters, clusterRadius := uint64(4), uint64(6)
	return models.Model{
		ID:     bson.NewObjectID().Hex(),
		ChatID: bson.NewObjectID().Hex(),
		Name:   "predator-prey",
		LLMConfig: models.LLMConfig{
			Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh},
		},
		SimulationConfig: models.SimulationConfig{
			UpdateMode:  models.UpdateModeSynchronous,
			Termination: &models.Condition{Type: models.ConditionTypePopulation, Operator: models.ConditionOpEquals, Value: float64(0)},
			LLMSimulationGenerationConfig: models.LLMConfig{
				Target:        common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini", Effort: common.LLMEffortMedium},
				PromptContext: "generation context",
			},
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{
					Target:        common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh},
					PromptContext: "execution context",
				},
				LLMSimulationAgentConfig: models.LLMSimulationAgentConfig{
					PerceptionRadius: 50,
					MemorySize:       20,
				},
				ParallelWorkers: 3,
				RetryTimeoutMs:  30000,
				MaxRetries:      3,
				StreamBatchSize: 100,
				DefaultStrategy: models.LLMStrategy{Type: models.LLMStrategyHybrid, UseForDecisions: true, ArchetypeMode: models.ArchetypeModeDistribution, VariationStrength: 0.3},
			},
		},
		IsSpatial: true,
		Agents: []models.Agent{{
			UID: "predator", Name: "Predator", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeMovable,
			Properties: []models.AgentProperty{{
				UID: "energy", Name: "Energy", Type: models.PropertyTypeNumber,
				MinValue: &minV, MaxValue: &maxV, DefaultValue: 50.0,
			}},
			States:         []string{"hunting", "resting"},
			Initialization: models.AgentInitialization{PopulationCount: 10, InitialState: "hunting", Placement: &models.Placement{Type: models.PlacementClustered, Clusters: &clusters, ClusterRadius: &clusterRadius}},
		}},
		Environment: &models.Environment{
			UID: models.EnvironmentUID, Width: 100, Height: 100,
			CollisionMode:   models.CollisionBlock,
			CollisionConfig: &models.CollisionConfig{MaxIterations: 12, StuckPolicy: models.CollisionStuckBestEffort},
			Shapes: []models.Shape{{
				UID: "forest", Name: "Forest", Type: models.ShapeTypeRectangle, Visible: true,
				Geometry: &models.ShapeGeometry{
					Position: &common.Point{X: 10, Y: 20},
					Width:    &models.ScalarSpec{Value: &forestWidth},
					Height:   &models.ScalarSpec{Value: &forestHeight},
				},
			}},
		},
		Rules: []models.Rule{{
			ID: "hunt", Name: "Hunt", Priority: 1, AppliesTo: []string{"predator"}, Enabled: true,
			Conditions: []models.Condition{{
				Type: models.ConditionTypeComposite, LogicOperator: models.LogicOpAND,
				SubConditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 30.0}},
			}},
			Actions: []models.Action{{Type: models.ActionTypeMoveToward, TargetUID: "prey", Speed: &speed}},
		}},
		Events: []models.EventDefinition{{
			UID: "alarm", Name: "Alarm", Scope: models.EventScopeGlobal,
			Producers: []models.EventProducer{models.EventProducerAgent},
		}},
	}
}

func TestModelEntityDocumentRoundTrip(t *testing.T) {
	in := newFixtureModel()
	doc, err := ModelToDocument(in)
	if err != nil {
		t.Fatalf("to document: %v", err)
	}
	out := ModelFromDocument(doc)
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", out, in)
	}
}

func TestModelGenerationExtensionsRoundTrip(t *testing.T) {
	mean, stddev := 50.0, 10.0
	genMin, genMax := 20.0, 80.0
	promptCtx := "set a high value for competitive agents"

	in := models.Model{
		ID:   bson.NewObjectID().Hex(),
		Name: "generation-extensions",
		Agents: []models.Agent{{
			UID: "agent-a", Name: "Agent A", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeStatic,
			Properties: []models.AgentProperty{
				{
					// normal-distribution generation spec
					UID: "health", Name: "Health", Type: models.PropertyTypeNumber,
					Generation: &models.PropertyGeneration{
						Mode:         models.GenModeRandom,
						Distribution: models.DistNormal,
						Mean:         &mean,
						StdDev:       &stddev,
						Min:          &genMin,
						Max:          &genMax,
					},
				},
				{
					// weighted-choice generation spec; includes a float64 value to
					// exercise bson numeric round-trip for ChoiceWeight.Value (interface{}).
					UID: "state_bias", Name: "StateBias", Type: models.PropertyTypeEnum,
					Generation: &models.PropertyGeneration{
						Mode:         models.GenModeRandom,
						Distribution: models.DistChoice,
						Choices: []models.ChoiceWeight{
							{Value: "aggressive", Weight: 0.7},
							{Value: "passive", Weight: 0.3},
							{Value: 1.5, Weight: 0.5},
						},
					},
				},
				{
					// llm generation spec
					UID: "personality", Name: "Personality", Type: models.PropertyTypeString,
					Generation: &models.PropertyGeneration{
						Mode:          models.GenModeLLM,
						PromptContext: promptCtx,
					},
				},
			},
			States: []string{"idle", "active"},
			Initialization: models.AgentInitialization{
				PopulationCount: 5,
				InitialState:    "idle",
				CountMode:       models.CountRandomRange,
			},
		}, {
			UID: "agent-b", Name: "B", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeStatic,
			Initialization: models.AgentInitialization{
				PopulationCount: 1,
				CountMode:       models.CountLLM,
			},
		}, {
			UID: "agent-c", Name: "C", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeStatic,
			Initialization: models.AgentInitialization{
				PopulationCount: 2,
				// CountMode left empty: treated as CountFixed and must survive
				// the bson round-trip as the empty string (count_mode omitted).
			},
		}},
		Widgets: []common.WidgetConfig{
			{
				ID: "w-monitor", Type: common.WidgetMonitor, Title: "Health Monitor",
				Source: &common.WidgetSource{
					Type:        common.WidgetSourceAgentProperty,
					PropertyUID: "health",
					AgentUIDs:   []string{"agent-a"},
					Aggregation: common.AggAvg,
				},
				Position: common.WidgetPosition{X: 0, Y: 0, W: 4, H: 2},
			},
			{
				ID: "w-chart", Type: common.WidgetLineChart, Title: "State Distribution Over Time",
				Series: []common.WidgetSeries{
					{
						Label:  "Aggressive",
						Color:  "#e74c3c",
						Source: common.WidgetSource{Type: common.WidgetSourcePopulation, AgentUIDs: []string{"agent-a"}},
					},
					{
						Label:  "Passive",
						Color:  "#2ecc71",
						Source: common.WidgetSource{Type: common.WidgetSourceStateDistribution, AgentUIDs: []string{"agent-a"}},
					},
				},
				Position: common.WidgetPosition{X: 4, Y: 0, W: 8, H: 4},
			},
		},
	}

	doc, err := ModelToDocument(in)
	if err != nil {
		t.Fatalf("to document: %v", err)
	}

	// Exercise real BSON encode/decode so that type drift in interface{} fields
	// (e.g. ChoiceWeight.Value, DefaultValue, AllowedValues) is caught here rather
	// than only at runtime against a live MongoDB instance.
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("bson marshal: %v", err)
	}
	var docAfterBSON ModelDocument
	if err := bson.Unmarshal(raw, &docAfterBSON); err != nil {
		t.Fatalf("bson unmarshal: %v", err)
	}

	out := ModelFromDocument(docAfterBSON)
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", out, in)
	}
}

func TestModelToDocumentInvalidID(t *testing.T) {
	if _, err := ModelToDocument(models.Model{ID: "nope"}); err == nil {
		t.Fatal("want error for malformed model id")
	}
	if _, err := ModelToDocument(models.Model{ChatID: "nope"}); err == nil {
		t.Fatal("want error for malformed chat id")
	}
}

func TestPropertyFromDocumentNormalizesValues(t *testing.T) {
	d := AgentPropertyDocument{
		UID:           "cfg",
		DefaultValue:  bson.D{{Key: "nested", Value: bson.A{int32(1)}}},
		AllowedValues: []interface{}{bson.D{{Key: "a", Value: "b"}}},
	}
	p := propertyFromDocument(d)
	if _, ok := p.DefaultValue.(map[string]interface{}); !ok {
		t.Fatalf("DefaultValue not normalized: %T", p.DefaultValue)
	}
	if _, ok := p.AllowedValues[0].(map[string]interface{}); !ok {
		t.Fatalf("AllowedValues not normalized: %T", p.AllowedValues[0])
	}
}
