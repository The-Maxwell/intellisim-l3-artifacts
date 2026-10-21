package simulations

import (
	"testing"

	"intelli.sim.core/data/common"
	models "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

func TestShapeRecordable(t *testing.T) {
	mut := models.AgentProperty{UID: "water_level", Type: models.PropertyTypeNumber}
	imm := models.AgentProperty{UID: "depth", Type: models.PropertyTypeNumber, Immutable: true}
	llmFull := &models.LLMShapeConfig{Strategy: models.LLMShapeStrategy{LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyFull}}}
	llmNone := &models.LLMShapeConfig{Strategy: models.LLMShapeStrategy{LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyNone}}}

	cases := []struct {
		name string
		def  *models.Shape
		want bool
	}{
		{"nil def", nil, false},
		{"no properties even with llm", &models.Shape{LLMConfig: llmFull}, false},
		{"mutable property", &models.Shape{Properties: []models.AgentProperty{mut}}, true},
		{"immutable only, no llm", &models.Shape{Properties: []models.AgentProperty{imm}}, false},
		{"immutable only, llm none", &models.Shape{Properties: []models.AgentProperty{imm}, LLMConfig: llmNone}, false},
		{"immutable only, llm full", &models.Shape{Properties: []models.AgentProperty{imm}, LLMConfig: llmFull}, true},
	}
	for _, tc := range cases {
		if got := shapeRecordable(tc.def); got != tc.want {
			t.Errorf("%s: shapeRecordable=%v want %v", tc.name, got, tc.want)
		}
	}
}

// shapeRowsRS builds a minimal RuntimeState with two shapes: lake (recordable,
// id 5) and wall (not recordable, id 3), plus a second lake instance (id 2) to
// exercise id-sorted emission.
func shapeRowsRS() *RuntimeState {
	return &RuntimeState{
		Sim: &datasims.Simulation{ID: "sim-shapes"},
		Env: &datasims.SimulationEnvironment{
			Shapes: []datasims.SimulationShape{
				{ID: 5, ShapeUID: "lake", Position: common.Point{X: 20, Y: 15}, Properties: map[string]interface{}{"water_level": 50.0}},
				{ID: 3, ShapeUID: "wall", Position: common.Point{X: 1, Y: 1}, Properties: map[string]interface{}{"height": 2.0}},
				{ID: 2, ShapeUID: "lake", Position: common.Point{X: 30, Y: 5}, Properties: map[string]interface{}{"water_level": 80.0}},
			},
		},
		recordableShapes: map[string]bool{"lake": true, "wall": false},
	}
}

func TestBuildStepRows_ShapeRows(t *testing.T) {
	rs := shapeRowsRS()
	rows := buildStepRows(rs, 7)

	var shapes []datasims.SimulationStep
	for _, r := range rows {
		if r.Metadata.Type == datasims.MetadataTypeShape {
			shapes = append(shapes, r)
		}
	}
	if len(shapes) != 2 {
		t.Fatalf("shape rows=%d want 2 (only recordable lake instances)", len(shapes))
	}
	// id-sorted: instance 2 before instance 5
	if shapes[0].Metadata.ReferenceID != 2 || shapes[1].Metadata.ReferenceID != 5 {
		t.Fatalf("shape rows not id-sorted: %d, %d", shapes[0].Metadata.ReferenceID, shapes[1].Metadata.ReferenceID)
	}
	r := shapes[1]
	if r.Metadata.UID != "lake" || r.Metadata.SimulationID != "sim-shapes" || r.Metadata.Step != 7 {
		t.Fatalf("bad shape row metadata: %+v", r.Metadata)
	}
	if r.Position == nil || r.Position.X != 20 || r.Position.Y != 15 {
		t.Fatalf("bad shape row position: %+v", r.Position)
	}
	if r.Properties["water_level"] != 50.0 {
		t.Fatalf("bad shape row properties: %+v", r.Properties)
	}
	// Properties must be a copy: mutating the runtime map must not affect the row.
	rs.Env.Shapes[0].Properties["water_level"] = 1.0
	if r.Properties["water_level"] != 50.0 {
		t.Fatal("shape row properties alias the runtime map")
	}
}

func TestBuildStepRows_NoRecordableShapes_NoShapeRows(t *testing.T) {
	rs := shapeRowsRS()
	rs.recordableShapes = nil // test-literal states / models without recordable shapes
	for _, r := range buildStepRows(rs, 1) {
		if r.Metadata.Type == datasims.MetadataTypeShape {
			t.Fatalf("unexpected shape row: %+v", r.Metadata)
		}
	}
}
