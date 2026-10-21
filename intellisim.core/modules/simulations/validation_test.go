package simulations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ===================== test helpers =====================

// hasError reports whether any issue in Errors has the given path.
func hasError(t *testing.T, res ValidationResult, path string) bool {
	t.Helper()
	for _, e := range res.Errors {
		if e.Path == path {
			return true
		}
	}
	return false
}

// hasWarning reports whether any issue in Warnings has the given path.
func hasWarning(t *testing.T, res ValidationResult, path string) bool {
	t.Helper()
	for _, w := range res.Warnings {
		if w.Path == path {
			return true
		}
	}
	return false
}

// assertError asserts that Errors contains at least one issue with the given path.
func assertError(t *testing.T, res ValidationResult, path string) {
	t.Helper()
	assert.True(t, hasError(t, res, path),
		"expected error at path %q; errors=%v", path, res.Errors)
}

// assertWarning asserts that Warnings contains at least one issue with the given path.
func assertWarning(t *testing.T, res ValidationResult, path string) {
	t.Helper()
	assert.True(t, hasWarning(t, res, path),
		"expected warning at path %q; warnings=%v", path, res.Warnings)
}

// assertNoError asserts that Errors does NOT contain an issue at path.
func assertNoError(t *testing.T, res ValidationResult, path string) {
	t.Helper()
	assert.False(t, hasError(t, res, path),
		"unexpected error at path %q; errors=%v", path, res.Errors)
}

// ===================== fixture builders =====================

// validSnapshot returns a snapshot model with two archetypes:
//   - "wolf" (spatial movable): number "energy" [0,100], enum "mood" (calm|angry), boolean "alive"
//   - "sheep" (spatial movable): number "speed" [0,10]
//
// Environment: 200×200, circle "lake", polygon "forest".
// States for wolf: ["idle","hunting"], InitialState "idle".
func validSnapshot() *datamodels.Model {
	minPop := uint64(1)
	maxPop := uint64(5)
	return &datamodels.Model{
		Name:      "test-model",
		IsSpatial: true,
		Agents: []datamodels.Agent{
			{
				UID:      "wolf",
				Name:     "Wolf",
				Type:     datamodels.AgentTypeArchetype,
				MoveType: datamodels.AgentMoveTypeMovable,
				States:   []string{"idle", "hunting"},
				Properties: []datamodels.AgentProperty{
					{
						UID:      "energy",
						Type:     datamodels.PropertyTypeNumber,
						MinValue: f64ptr(0),
						MaxValue: f64ptr(100),
					},
					{
						UID:           "mood",
						Type:          datamodels.PropertyTypeEnum,
						AllowedValues: []interface{}{"calm", "angry"},
					},
					{
						UID:  "alive",
						Type: datamodels.PropertyTypeBoolean,
					},
					{
						UID:  "tag",
						Type: datamodels.PropertyTypeString,
					},
					{
						UID:  "color",
						Type: datamodels.PropertyTypeColor,
					},
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 3,
					MinPopulation:   &minPop,
					MaxPopulation:   &maxPop,
					InitialState:    "idle",
				},
			},
			{
				UID:      "sheep",
				Name:     "Sheep",
				Type:     datamodels.AgentTypeArchetype,
				MoveType: datamodels.AgentMoveTypeMovable,
				Properties: []datamodels.AgentProperty{
					{
						UID:      "speed",
						Type:     datamodels.PropertyTypeNumber,
						MinValue: f64ptr(0),
						MaxValue: f64ptr(10),
					},
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 2,
				},
			},
		},
		Environment: &datamodels.Environment{
			Width:  200,
			Height: 200,
			Shapes: []datamodels.Shape{
				{
					UID:  "lake",
					Type: datamodels.ShapeTypeCircle,
				},
				{
					UID:  "forest",
					Type: datamodels.ShapeTypePolygon,
				},
			},
			GlobalProperties: []datamodels.AgentProperty{
				{UID: "temperature", Type: datamodels.PropertyTypeNumber},
			},
		},
	}
}

// validSim returns a fully valid Simulation against validSnapshot().
// Has 2 wolf instances + 1 sheep instance, valid shapes, one widget.
func validSim() datasims.Simulation {
	pt := func(x, y uint64) *common.Point { p := common.Point{X: x, Y: y}; return &p }
	return datasims.Simulation{
		Steps:         10,
		ModelSnapshot: validSnapshot(),
		Agents: []datasims.SimulationAgent{
			{
				ID:       1,
				AgentUID: "wolf",
				Name:     "Wolf-1",
				Position: pt(10, 20),
				State:    "idle",
				Properties: map[string]interface{}{
					"energy": float64(50),
					"mood":   "calm",
					"alive":  true,
					"tag":    "alpha",
					"color":  "#ff0000",
				},
			},
			{
				ID:       2,
				AgentUID: "wolf",
				Name:     "Wolf-2",
				Position: pt(30, 40),
				State:    "hunting",
				Properties: map[string]interface{}{
					"energy": float64(80),
					"mood":   "angry",
					"alive":  true,
					"tag":    "beta",
					"color":  "#00ff00",
				},
			},
			{
				ID:       3,
				AgentUID: "sheep",
				Name:     "Sheep-1",
				Position: pt(100, 100),
				Properties: map[string]interface{}{
					"speed": float64(5),
				},
			},
		},
		Environment: &datasims.SimulationEnvironment{
			Width:  200,
			Height: 200,
			Shapes: []datasims.SimulationShape{
				{
					ID:       1,
					ShapeUID: "lake",
					Type:     datamodels.ShapeTypeCircle,
					Position: common.Point{X: 50, Y: 50},
					Radius:   f64ptr(15.0),
				},
				{
					ID:       2,
					ShapeUID: "forest",
					Type:     datamodels.ShapeTypePolygon,
					Position: common.Point{X: 100, Y: 100},
					Points: []common.Point{
						{X: 0, Y: 0},
						{X: 10, Y: 0},
						{X: 5, Y: 10},
					},
				},
			},
		},
		Widgets: []common.WidgetConfig{
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
			},
		},
	}
}

// ===================== valid fixture smoke test =====================

func TestValidateInstance_ValidFixtureIsClean(t *testing.T) {
	sim := validSim()
	res := ValidateInstance(sim)
	assert.Empty(t, res.Errors, "valid fixture must have no errors: %v", res.Errors)
	assert.True(t, res.StartReady, "valid fixture must be start-ready")
}

// ===================== nil snapshot =====================

func TestValidateInstance_NilSnapshot(t *testing.T) {
	sim := validSim()
	sim.ModelSnapshot = nil
	res := ValidateInstance(sim)
	assertError(t, res, "model_snapshot")
	assert.False(t, res.StartReady)
}

// ===================== steps =====================

func TestValidateInstance_StepsZero(t *testing.T) {
	sim := validSim()
	sim.Steps = 0
	res := ValidateInstance(sim)
	assertError(t, res, "steps")
	assert.False(t, res.StartReady)
}

func TestValidateInstance_StepsOne_OK(t *testing.T) {
	sim := validSim()
	sim.Steps = 1
	res := ValidateInstance(sim)
	assertNoError(t, res, "steps")
}

// ===================== agents =====================

func TestValidateInstance_UnknownAgentUID(t *testing.T) {
	sim := validSim()
	sim.Agents[0].AgentUID = "ghost"
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].agent_uid")
}

func TestValidateInstance_DuplicateInstanceID(t *testing.T) {
	sim := validSim()
	sim.Agents[1].ID = sim.Agents[0].ID // duplicate id=1
	res := ValidateInstance(sim)
	assertError(t, res, "agents[1].id")
}

func TestValidateInstance_BadState_NotInStates(t *testing.T) {
	sim := validSim()
	sim.Agents[0].State = "sleeping" // not in wolf.States
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].state")
}

func TestValidateInstance_EmptyState_OK(t *testing.T) {
	sim := validSim()
	sim.Agents[0].State = "" // empty state is always allowed
	res := ValidateInstance(sim)
	assertNoError(t, res, "agents[0].state")
}

func TestValidateInstance_StateOK_WhenStatesEmpty(t *testing.T) {
	// sheep archetype has no States list; any state value is allowed
	sim := validSim()
	sim.Agents[2].State = "wandering"
	res := ValidateInstance(sim)
	assertNoError(t, res, "agents[2].state")
}

// ===================== property type checks =====================

func TestValidateInstance_Property_UnknownUID(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["ghost_prop"] = "value"
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].properties.ghost_prop")
}

func TestValidateInstance_Property_NumberType_WrongValue(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["energy"] = "not-a-number"
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].properties.energy")
}

func TestValidateInstance_Property_NumberType_IntCoercion_OK(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["energy"] = int64(42)
	res := ValidateInstance(sim)
	assertNoError(t, res, "agents[0].properties.energy")
}

func TestValidateInstance_Property_BooleanType_WrongValue(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["alive"] = "yes"
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].properties.alive")
}

func TestValidateInstance_Property_StringType_WrongValue(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["tag"] = 42
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].properties.tag")
}

func TestValidateInstance_Property_ColorType_WrongValue(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["color"] = 0xFF0000 // int, not string
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].properties.color")
}

func TestValidateInstance_Property_EnumType_NotInAllowedValues(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["mood"] = "sleepy"
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].properties.mood")
}

func TestValidateInstance_Property_EnumType_NotString(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["mood"] = 123
	res := ValidateInstance(sim)
	assertError(t, res, "agents[0].properties.mood")
}

func TestValidateInstance_Property_EnumType_NoAllowedValues_AnyStringOK(t *testing.T) {
	// mood without AllowedValues: any string is fine
	sim := validSim()
	snap := validSnapshot()
	for i, pr := range snap.Agents[0].Properties {
		if pr.UID == "mood" {
			snap.Agents[0].Properties[i].AllowedValues = nil
		}
	}
	sim.ModelSnapshot = snap
	sim.Agents[0].Properties["mood"] = "anything"
	res := ValidateInstance(sim)
	assertNoError(t, res, "agents[0].properties.mood")
}

// ===================== shapes =====================

func TestValidateInstance_Shape_UnknownShapeUID(t *testing.T) {
	sim := validSim()
	sim.Environment.Shapes[0].ShapeUID = "ghost_shape"
	res := ValidateInstance(sim)
	assertError(t, res, "environment.shapes[0].shape_uid")
}

func TestValidateInstance_Shape_Circle_NoRadius(t *testing.T) {
	sim := validSim()
	sim.Environment.Shapes[0].Radius = nil // circle without radius
	res := ValidateInstance(sim)
	assertError(t, res, "environment.shapes[0]")
}

func TestValidateInstance_Shape_Rectangle_MissingDimensions(t *testing.T) {
	sim := validSim()
	snap := validSnapshot()
	snap.Environment.Shapes = append(snap.Environment.Shapes, datamodels.Shape{
		UID:  "box",
		Type: datamodels.ShapeTypeRectangle,
	})
	sim.ModelSnapshot = snap
	sim.Environment.Shapes = append(sim.Environment.Shapes, datasims.SimulationShape{
		ID:       3,
		ShapeUID: "box",
		Type:     datamodels.ShapeTypeRectangle,
		// Width and Height both nil
	})
	res := ValidateInstance(sim)
	assertError(t, res, "environment.shapes[2]")
}

func TestValidateInstance_Shape_Rectangle_OK(t *testing.T) {
	sim := validSim()
	snap := validSnapshot()
	snap.Environment.Shapes = append(snap.Environment.Shapes, datamodels.Shape{
		UID:  "box",
		Type: datamodels.ShapeTypeRectangle,
	})
	sim.ModelSnapshot = snap
	w := u64ptr(20)
	h := u64ptr(30)
	sim.Environment.Shapes = append(sim.Environment.Shapes, datasims.SimulationShape{
		ID:       3,
		ShapeUID: "box",
		Type:     datamodels.ShapeTypeRectangle,
		Width:    w,
		Height:   h,
	})
	res := ValidateInstance(sim)
	assertNoError(t, res, "environment.shapes[2]")
}

func TestValidateInstance_Shape_Polygon_TooFewPoints(t *testing.T) {
	sim := validSim()
	sim.Environment.Shapes[1].Points = []common.Point{{X: 0, Y: 0}, {X: 10, Y: 0}} // only 2
	res := ValidateInstance(sim)
	assertError(t, res, "environment.shapes[1]")
}

func TestValidateInstance_Shape_Line_TooFewPoints(t *testing.T) {
	sim := validSim()
	snap := validSnapshot()
	snap.Environment.Shapes = append(snap.Environment.Shapes, datamodels.Shape{
		UID:  "road",
		Type: datamodels.ShapeTypeLine,
	})
	sim.ModelSnapshot = snap
	sim.Environment.Shapes = append(sim.Environment.Shapes, datasims.SimulationShape{
		ID:       3,
		ShapeUID: "road",
		Type:     datamodels.ShapeTypeLine,
		Points:   []common.Point{{X: 0, Y: 0}}, // only 1 point
	})
	res := ValidateInstance(sim)
	assertError(t, res, "environment.shapes[2]")
}

func TestValidateInstance_Shape_Line_OK(t *testing.T) {
	sim := validSim()
	snap := validSnapshot()
	snap.Environment.Shapes = append(snap.Environment.Shapes, datamodels.Shape{
		UID:  "road",
		Type: datamodels.ShapeTypeLine,
	})
	sim.ModelSnapshot = snap
	sim.Environment.Shapes = append(sim.Environment.Shapes, datasims.SimulationShape{
		ID:       3,
		ShapeUID: "road",
		Type:     datamodels.ShapeTypeLine,
		Points:   []common.Point{{X: 0, Y: 0}, {X: 50, Y: 50}},
	})
	res := ValidateInstance(sim)
	assertNoError(t, res, "environment.shapes[2]")
}

func TestValidateInstance_NilEnvironment_ShapeChecksSkipped(t *testing.T) {
	sim := validSim()
	sim.Environment = nil
	res := ValidateInstance(sim)
	// No shape errors, no environment errors
	for _, e := range res.Errors {
		assert.NotContains(t, e.Path, "environment.shapes", "no shape errors when environment is nil")
	}
}

// ===================== widgets — structural =====================

func TestValidateInstance_Widget_StructuralIssue(t *testing.T) {
	sim := validSim()
	sim.Widgets = []common.WidgetConfig{
		{ID: "bad", Type: common.WidgetMonitor, Title: "T"}, // no source
	}
	res := ValidateInstance(sim)
	assertError(t, res, "widgets[0].source")
}

// ===================== widgets — referential =====================

func TestValidateInstance_Widget_UnknownAgentUID(t *testing.T) {
	sim := validSim()
	sim.Widgets[0].Source.AgentUIDs = []string{"ghost"}
	res := ValidateInstance(sim)
	assertError(t, res, "widgets[0].source.agent_uids[0]")
}

func TestValidateInstance_Widget_UnknownPropertyUID(t *testing.T) {
	sim := validSim()
	sim.Widgets[0].Source.PropertyUID = "nonexistent"
	res := ValidateInstance(sim)
	assertError(t, res, "widgets[0].source.property_uid")
}

func TestValidateInstance_Widget_GlobalPropertyRef_OK(t *testing.T) {
	sim := validSim()
	sim.Widgets = []common.WidgetConfig{
		{
			ID:    "w2",
			Type:  common.WidgetMonitor,
			Title: "Temp",
			Source: &common.WidgetSource{
				Type:        common.WidgetSourceGlobalProperty,
				PropertyUID: "temperature",
			},
		},
	}
	res := ValidateInstance(sim)
	assertNoError(t, res, "widgets[0].source.property_uid")
}

func TestValidateInstance_Widget_GlobalPropertyRef_Unknown(t *testing.T) {
	sim := validSim()
	sim.Widgets = []common.WidgetConfig{
		{
			ID:    "w2",
			Type:  common.WidgetMonitor,
			Title: "Temp",
			Source: &common.WidgetSource{
				Type:        common.WidgetSourceGlobalProperty,
				PropertyUID: "humidity",
			},
		},
	}
	res := ValidateInstance(sim)
	assertError(t, res, "widgets[0].source.property_uid")
}

func TestValidateInstance_Widget_AgentTable_BadColumn(t *testing.T) {
	sim := validSim()
	sim.Widgets = []common.WidgetConfig{
		{
			ID:    "t1",
			Type:  common.WidgetAgentTable,
			Title: "Wolves",
			Source: &common.WidgetSource{
				Type:      common.WidgetSourceAgentProperty,
				AgentUIDs: []string{"wolf"},
			},
			Columns: []string{"energy", "nonexistent_col"},
		},
	}
	res := ValidateInstance(sim)
	assertNoError(t, res, "widgets[0].columns[0]") // energy is valid
	assertError(t, res, "widgets[0].columns[1]")   // nonexistent_col is invalid
}

func TestValidateInstance_Widget_PopulationRef_UnknownAgent(t *testing.T) {
	sim := validSim()
	sim.Widgets = []common.WidgetConfig{
		{
			ID:    "pop1",
			Type:  common.WidgetMonitor,
			Title: "Pop",
			Source: &common.WidgetSource{
				Type:      common.WidgetSourcePopulation,
				AgentUIDs: []string{"ghost_agent"},
			},
		},
	}
	res := ValidateInstance(sim)
	assertError(t, res, "widgets[0].source.agent_uids[0]")
}

func TestValidateInstance_Widget_StateDistribution_ValidAgent(t *testing.T) {
	sim := validSim()
	sim.Widgets = []common.WidgetConfig{
		{
			ID:    "sd1",
			Type:  common.WidgetPieChart,
			Title: "State",
			Source: &common.WidgetSource{
				Type:      common.WidgetSourceStateDistribution,
				AgentUIDs: []string{"wolf"},
			},
		},
	}
	res := ValidateInstance(sim)
	assertNoError(t, res, "widgets[0].source.agent_uids[0]")
}

func TestValidateInstance_Widget_SeriesSourceChecked(t *testing.T) {
	sim := validSim()
	sim.Widgets = []common.WidgetConfig{
		{
			ID:    "c1",
			Type:  common.WidgetLineChart,
			Title: "Chart",
			Series: []common.WidgetSeries{
				{
					Label: "ok",
					Source: common.WidgetSource{
						Type:        common.WidgetSourceAgentProperty,
						PropertyUID: "energy",
						AgentUIDs:   []string{"wolf"},
						Aggregation: common.AggAvg,
					},
				},
				{
					Label: "bad",
					Source: common.WidgetSource{
						Type:        common.WidgetSourceAgentProperty,
						PropertyUID: "ghost_prop",
						AgentUIDs:   []string{"wolf"},
						Aggregation: common.AggAvg,
					},
				},
			},
		},
	}
	res := ValidateInstance(sim)
	assertNoError(t, res, "widgets[0].series[0].source.property_uid")
	assertError(t, res, "widgets[0].series[1].source.property_uid")
}

// ===================== warnings =====================

func TestValidateInstance_Warning_NumericOutOfRange(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Properties["energy"] = float64(150) // > MaxValue 100
	res := ValidateInstance(sim)
	assertWarning(t, res, "agents[0].properties.energy")
}

func TestValidateInstance_Warning_CountOutOfRange(t *testing.T) {
	sim := validSim()
	// wolf bounds: min=1, max=5; put 10 wolves
	for i := uint64(4); i <= 10; i++ {
		sim.Agents = append(sim.Agents, datasims.SimulationAgent{
			ID:       i,
			AgentUID: "wolf",
			Name:     "Wolf-X",
			Position: &common.Point{X: 10, Y: 10},
			State:    "idle",
		})
	}
	res := ValidateInstance(sim)
	assertWarning(t, res, "agents")
}

func TestValidateInstance_Error_SpatialNoPosition(t *testing.T) {
	sim := validSim()
	sim.Agents[0].Position = nil // spatial model but no position ⇒ awaits manual placement
	res := ValidateInstance(sim)
	// Unplaced spatial agents are a blocking error (not an advisory warning) so
	// the run is gated until the user places them in the world.
	assertError(t, res, "agents")
	assert.False(t, res.StartReady, "an unplaced agent must block start")
}

func TestValidateInstance_Warning_PositionOutOfBounds(t *testing.T) {
	sim := validSim()
	// world is 200x200; put agent at (250, 100)
	sim.Agents[0].Position = &common.Point{X: 250, Y: 100}
	res := ValidateInstance(sim)
	assertWarning(t, res, "agents[0].position")
}

func TestValidateInstance_Warning_NoWidgets(t *testing.T) {
	sim := validSim()
	sim.Widgets = nil
	res := ValidateInstance(sim)
	assertWarning(t, res, "widgets")
}

// ===================== StartReady semantics =====================

func TestValidateInstance_StartReady_FalseWhenErrors(t *testing.T) {
	sim := validSim()
	sim.Steps = 0 // blocking error
	res := ValidateInstance(sim)
	assert.False(t, res.StartReady)
}

func TestValidateInstance_StartReady_FalseWhenNoAgents(t *testing.T) {
	sim := validSim()
	sim.Agents = nil
	res := ValidateInstance(sim)
	assert.False(t, res.StartReady)
}

func TestValidateInstance_StartReady_TrueWithWarningsOnly(t *testing.T) {
	sim := validSim()
	sim.Widgets = nil // triggers no-widget warning but no error
	res := ValidateInstance(sim)
	assert.Empty(t, res.Errors, "should have no errors")
	assert.True(t, res.StartReady)
}

// ===================== edge cases =====================

func TestValidateInstance_MultipleErrorsAccumulate(t *testing.T) {
	sim := validSim()
	sim.Steps = 0
	sim.Agents[0].AgentUID = "ghost"
	sim.Agents[1].ID = sim.Agents[0].ID
	res := ValidateInstance(sim)
	require.GreaterOrEqual(t, len(res.Errors), 3, "should accumulate multiple errors")
}
