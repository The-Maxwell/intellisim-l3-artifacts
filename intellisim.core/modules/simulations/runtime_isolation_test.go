package simulations

import (
	"context"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// TestNewRuntimeState_IsolatesStoredWorld verifies the run engine operates on a
// deep copy of the simulation's agents and environment, so the stored
// Simulation.Agents / Simulation.Environment are never mutated by a run. This is
// the invariant that makes Duplicate of a finished simulation copy the INITIAL
// world (positions/properties/state/memory + initial global/shape properties)
// rather than the final run state.
func TestNewRuntimeState_IsolatesStoredWorld(t *testing.T) {
	model := globalsModel(models.UpdateModeSequential, nil, nil)
	sim := &datasims.Simulation{
		ID: "iso", Seed: 7, Steps: 5,
		ModelSnapshot: model,
		Environment: &datasims.SimulationEnvironment{
			Width: 100, Height: 80,
			GlobalProperties: map[string]interface{}{
				"temperature": float64(20), "locked_const": float64(5), "label": "calm",
			},
			Shapes: []datasims.SimulationShape{
				{ID: 1, ShapeUID: "s1", Properties: map[string]interface{}{"charge": float64(1)}},
			},
		},
		Agents: []datasims.SimulationAgent{
			{ID: 1, AgentUID: "mover", State: "idle",
				Position:   &common.Point{X: 10, Y: 10},
				Properties: map[string]interface{}{"e": float64(1)}},
		},
	}

	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}

	// The runtime must not alias the stored structs.
	if rs.Agents[0].A == &sim.Agents[0] {
		t.Fatal("runtime agent aliases the stored SimulationAgent; the run would mutate sim.Agents in place")
	}
	if rs.Env == sim.Environment {
		t.Fatal("runtime env aliases the stored SimulationEnvironment; the run would mutate sim.Environment in place")
	}
	if rs.shapesByUID["s1"] == &sim.Environment.Shapes[0] {
		t.Fatal("runtime shape aliases the stored SimulationShape; the run would mutate sim.Environment.Shapes in place")
	}

	// Mutating the runtime-owned state must not touch the stored world.
	rs.Agents[0].A.Position.X = 999
	rs.Agents[0].A.Properties["e"] = float64(42)
	rs.Agents[0].A.State = "moved"
	rs.Agents[0].A.Memory = append(rs.Agents[0].A.Memory, datasims.AgentMemory{Step: 1, Summary: "x"})
	rs.Env.GlobalProperties["temperature"] = float64(999)
	rs.shapesByUID["s1"].Properties["charge"] = float64(999)

	if sim.Agents[0].Position.X != 10 {
		t.Errorf("stored agent Position mutated to X=%d; want pristine 10", sim.Agents[0].Position.X)
	}
	if sim.Agents[0].Properties["e"] != float64(1) {
		t.Errorf("stored agent Properties mutated to e=%v; want pristine 1", sim.Agents[0].Properties["e"])
	}
	if sim.Agents[0].State != "idle" {
		t.Errorf("stored agent State mutated to %q; want pristine idle", sim.Agents[0].State)
	}
	if len(sim.Agents[0].Memory) != 0 {
		t.Errorf("stored agent Memory mutated (%d entries); want pristine empty", len(sim.Agents[0].Memory))
	}
	if sim.Environment.GlobalProperties["temperature"] != float64(20) {
		t.Errorf("stored global temperature mutated to %v; want pristine 20", sim.Environment.GlobalProperties["temperature"])
	}
	if sim.Environment.Shapes[0].Properties["charge"] != float64(1) {
		t.Errorf("stored shape charge mutated to %v; want pristine 1", sim.Environment.Shapes[0].Properties["charge"])
	}
}

// TestApplyShapeActions_DoesNotMutateStoredShape proves shape-property isolation
// through the ACTUAL engine write path: applyShapeActions (the function the
// agent_shape interaction phase calls) mutates the runtime shape handle
// (rs.shapesByUID / rs.Env.Shapes, the run-owned clone), and the stored
// sim.Environment.Shapes stays at its initial property values. This is the shape
// analog of the global-property isolation and guarantees a Duplicate of a run
// where shape properties changed still copies the INITIAL shape properties.
func TestApplyShapeActions_DoesNotMutateStoredShape(t *testing.T) {
	sim := &datasims.Simulation{
		ID: "iso-shape", Seed: 3, Steps: 5,
		ModelSnapshot: globalsModel(models.UpdateModeSequential, nil, nil),
		Environment: &datasims.SimulationEnvironment{
			Width: 100, Height: 80,
			Shapes: []datasims.SimulationShape{
				{ID: 1, ShapeUID: "zone", Properties: map[string]interface{}{"charge": float64(1), "active": false}},
			},
		},
	}

	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}

	// Drive the real engine write path against the runtime shape handle.
	shape := rs.shapesByUID["zone"]
	if shape == nil {
		t.Fatal("runtime shape handle not indexed")
	}
	applyShapeActions(shape, []models.Action{
		{Type: models.ActionTypeSetProperty, PropertyUID: "charge", Value: float64(5)},
		{Type: models.ActionTypeModifyProperty, PropertyUID: "charge", Modifier: models.PropertyModifierAdd, Value: float64(10)},
		{Type: models.ActionTypeSetProperty, PropertyUID: "active", Value: true},
	})

	// Runtime reflects the change (precondition).
	if got := shape.Properties["charge"]; got != float64(15) {
		t.Fatalf("runtime shape charge = %v, want 15 (5 then +10)", got)
	}
	if got := shape.Properties["active"]; got != true {
		t.Fatalf("runtime shape active = %v, want true", got)
	}

	// Stored shape stays pristine initial.
	stored := sim.Environment.Shapes[0].Properties
	if stored["charge"] != float64(1) {
		t.Errorf("stored shape charge = %v after run-time write; want pristine 1", stored["charge"])
	}
	if stored["active"] != false {
		t.Errorf("stored shape active = %v after run-time write; want pristine false", stored["active"])
	}
}

// TestRunStep_DoesNotMutateStoredWorld drives the actual per-step pipeline and
// verifies the stored Simulation.Agents / Simulation.Environment stay at their
// initial values while the runtime state reflects the changes. Guards the whole
// run pipeline (not just NewRuntimeState) against leaking final state into the
// stored world.
func TestRunStep_DoesNotMutateStoredWorld(t *testing.T) {
	withFixedClock(t)

	model := globalsModel(models.UpdateModeSequential, []models.Rule{agentGlobalAddRule()}, nil)
	sim := &datasims.Simulation{
		ID: "iso-run", Seed: 11, Steps: 10,
		ModelSnapshot: model,
		Environment: &datasims.SimulationEnvironment{
			Width: 100, Height: 80,
			GlobalProperties: map[string]interface{}{
				"temperature": float64(20), "locked_const": float64(5), "label": "calm",
			},
		},
		Agents: []datasims.SimulationAgent{
			{ID: 1, AgentUID: "mover", State: "idle",
				Position:   &common.Point{X: 10, Y: 10},
				Properties: map[string]interface{}{}},
		},
	}

	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	if _, err := RunStep(context.Background(), rs, nil); err != nil {
		t.Fatalf("RunStep: %v", err)
	}

	// Precondition: the run actually changed the runtime environment (the agent
	// rule added 1 to temperature). If this fails the test is not exercising the
	// mutation and the isolation assertion below would be meaningless.
	if got := rs.Env.GlobalProperties["temperature"]; got != float64(21) {
		t.Fatalf("runtime temperature = %v, want 21 (run did not mutate runtime env)", got)
	}

	// The stored world must remain pristine after the run.
	if got := sim.Environment.GlobalProperties["temperature"]; got != float64(20) {
		t.Errorf("stored global temperature = %v after run; want pristine 20", got)
	}
	if sim.Agents[0].State != "idle" {
		t.Errorf("stored agent State = %q after run; want pristine idle", sim.Agents[0].State)
	}
	if sim.Agents[0].Position.X != 10 || sim.Agents[0].Position.Y != 10 {
		t.Errorf("stored agent Position = %+v after run; want pristine (10,10)", *sim.Agents[0].Position)
	}
	if len(sim.Agents[0].Memory) != 0 {
		t.Errorf("stored agent Memory grew to %d entries after run; want pristine empty", len(sim.Agents[0].Memory))
	}
}
