package simulations

import (
	"fmt"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// lookupModel builds a model with events, interactions, shapes, and global
// properties so every NewRuntimeState lookup map has entries.
func lookupModel() *models.Model {
	return &models.Model{
		SimulationConfig: models.SimulationConfig{UpdateMode: models.UpdateModeSequential},
		IsSpatial:        true,
		SpatialCellSize:  10,
		Agents: []models.Agent{{
			UID: "mover", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable,
			States:         []string{"idle"},
			Initialization: models.AgentInitialization{InitialState: "idle"},
		}},
		Environment: &models.Environment{
			UID: models.EnvironmentUID, Width: 100, Height: 80,
			Shapes: []models.Shape{
				{UID: "pond", Type: models.ShapeTypeCircle},
				{UID: "wall", Type: models.ShapeTypeRectangle},
			},
			GlobalProperties: []models.AgentProperty{
				{UID: "temperature", Type: models.PropertyTypeNumber, DefaultValue: float64(20)},
			},
		},
		Events: []models.EventDefinition{
			{UID: "alarm", Scope: models.EventScopeGlobal},
			{UID: "ping", Scope: models.EventScopeLocal},
		},
		Interactions: []models.Interaction{
			{ID: "greet", Type: models.InteractionTypeAgentAgent, Enabled: true},
		},
	}
}

func lookupState(t *testing.T) *RuntimeState {
	t.Helper()
	model := lookupModel()
	sim := &datasims.Simulation{
		ID: "sim-lookup", Seed: 1, Steps: 10,
		Environment: &datasims.SimulationEnvironment{
			Width: 100, Height: 80,
			GlobalProperties: map[string]interface{}{"temperature": float64(20)},
			Shapes: []datasims.SimulationShape{
				{ID: 1, ShapeUID: "pond", Type: models.ShapeTypeCircle, Position: common.Point{X: 10, Y: 10}},
				{ID: 2, ShapeUID: "pond", Type: models.ShapeTypeCircle, Position: common.Point{X: 50, Y: 50}},
				{ID: 3, ShapeUID: "wall", Type: models.ShapeTypeRectangle, Position: common.Point{X: 70, Y: 20}},
			},
		},
		Agents: []datasims.SimulationAgent{{
			ID: 1, AgentUID: "mover", State: "idle",
			Position: &common.Point{X: 5, Y: 5}, Properties: map[string]interface{}{},
		}},
		ModelSnapshot: model,
	}
	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	return rs
}

// TestNewRuntimeState_LookupMaps verifies the per-run lookup maps are populated
// and agree with the linear scans they replace.
func TestNewRuntimeState_LookupMaps(t *testing.T) {
	rs := lookupState(t)

	// eventDefs matches the linear scan for every declared UID (and misses).
	for _, uid := range []string{"alarm", "ping", "absent"} {
		var want *models.EventDefinition
		for i := range rs.Model.Events {
			if rs.Model.Events[i].UID == uid {
				want = &rs.Model.Events[i]
				break
			}
		}
		if got := eventDefByUID(rs, uid); got != want {
			t.Errorf("eventDefByUID(%q) = %p, want %p", uid, got, want)
		}
	}

	// shapesByUID resolves the FIRST instance per ShapeUID (slice order).
	if got := findShapeByUID(rs, "pond"); got == nil || got.ID != 1 {
		t.Errorf("findShapeByUID(pond) = %+v, want instance ID 1", got)
	}
	if got := findShapeByUID(rs, "wall"); got == nil || got.ID != 3 {
		t.Errorf("findShapeByUID(wall) = %+v, want instance ID 3", got)
	}
	if got := findShapeByUID(rs, "absent"); got != nil {
		t.Errorf("findShapeByUID(absent) = %+v, want nil", got)
	}

	// modelShapeDefs and envPropDefs point into the model.
	if rs.modelShapeDefs["pond"] != &rs.Model.Environment.Shapes[0] {
		t.Error("modelShapeDefs[pond] does not point at the model shape")
	}
	if rs.envPropDefs["temperature"] != &rs.Model.Environment.GlobalProperties[0] {
		t.Error("envPropDefs[temperature] does not point at the model global property")
	}
	if rs.interactionsByID["greet"] != &rs.Model.Interactions[0] {
		t.Error("interactionsByID[greet] does not point at the model interaction")
	}
}

// TestLookupFallbacks verifies the consumers still work on states built without
// NewRuntimeState (bare literals, as many engine tests construct).
func TestLookupFallbacks(t *testing.T) {
	model := lookupModel()
	rs := &RuntimeState{
		Model: model,
		Env: &datasims.SimulationEnvironment{
			Shapes: []datasims.SimulationShape{{ID: 7, ShapeUID: "pond", Type: models.ShapeTypeCircle}},
		},
	}
	if got := eventDefByUID(rs, "alarm"); got == nil || got.UID != "alarm" {
		t.Errorf("fallback eventDefByUID(alarm) = %+v", got)
	}
	if got := findShapeByUID(rs, "pond"); got == nil || got.ID != 7 {
		t.Errorf("fallback findShapeByUID(pond) = %+v", got)
	}
}

// TestWarnOnce verifies dedup and nil-safety.
func TestWarnOnce(t *testing.T) {
	rs := lookupState(t)
	for i := 0; i < 3; i++ {
		warnOnce(rs, "same message")
	}
	warnOnce(rs, "other message")
	if len(rs.warnings) != 2 {
		t.Fatalf("warnings = %v, want exactly 2 distinct entries", rs.warnings)
	}

	// Bare state without a warned set: appends every time (nil-safe, no dedup).
	bare := &RuntimeState{}
	warnOnce(bare, "m")
	warnOnce(bare, "m")
	if len(bare.warnings) != 2 {
		t.Fatalf("bare warnings = %v, want 2 (no dedup without warned set)", bare.warnings)
	}

	// The warned set is shared by reference across a shallow copy (workView).
	view := *rs
	view.warnings = nil
	warnOnce(&view, "same message") // already seen on rs -> dropped
	if len(view.warnings) != 0 {
		t.Fatalf("view warnings = %v, want dedup across shallow copy", view.warnings)
	}
	_ = fmt.Sprintf("%v", view.warned)
}
