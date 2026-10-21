package simulations

import (
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

func boolPt(b bool) *bool { return &b }

// pointAgent builds a movable runtime agent with the default point footprint.
func pointAgent(id uint64, x, y uint64) *RuntimeAgent {
	return &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: id, AgentUID: "visitor", Position: &common.Point{X: x, Y: y}},
		Archetype: &models.Agent{MoveType: models.AgentMoveTypeMovable},
	}
}

// U8a: a shape whose Fill is explicitly false is PASSABLE — it does not
// participate in collision response. Fill==nil and Fill==true stay solid
// (legacy pinned). Trigger: EXP-A attempt 4 — the LLM (twice) set the exit
// door's fill to false expecting an opening, but fill was visual-only, so the
// doorway stayed sealed and no visitor could ever leave.
func TestResolve_UnfilledShapeIsPassable(t *testing.T) {
	door := datasims.SimulationShape{Type: models.ShapeTypeRectangle,
		Position: common.Point{X: 95, Y: 95}, Width: cu64(10), Height: cu64(10), Fill: boolPt(false)}
	a := circleAgent(1, 100, 100, 3) // centered inside the unfilled shape
	before := common.Point{X: 100, Y: 100}
	rs := newRS(blockEnv(door), a)
	resolveCollisions(rs, rs.Agents, map[uint64]common.Point{1: before})
	if *a.A.Position != before {
		t.Fatalf("agent inside an unfilled shape must not be pushed/reverted; moved to %v", *a.A.Position)
	}
}

func TestResolve_FilledAndDefaultShapesStaySolid(t *testing.T) {
	for name, fill := range map[string]*bool{"nil": nil, "true": boolPt(true)} {
		t.Run(name, func(t *testing.T) {
			rect := datasims.SimulationShape{Type: models.ShapeTypeRectangle,
				Position: common.Point{X: 95, Y: 95}, Width: cu64(10), Height: cu64(10), Fill: fill}
			a := circleAgent(1, 100, 100, 3)
			before := common.Point{X: 100, Y: 100}
			rs := newRS(blockEnv(rect), a)
			resolveCollisions(rs, rs.Agents, map[uint64]common.Point{1: before})
			if *a.A.Position == before {
				t.Fatalf("fill=%s shape must stay solid (agent should be pushed out or reverted)", name)
			}
		})
	}
}

// U8b: the position condition supports operator "in" / "not_in" with a shape
// target — true when the agent's body overlaps any instance of the target
// shape template. Trigger: EXP-A attempts 3 & 4 — the LLM's natural removal
// rule {"type":"position","operator":"in","target_uid":"exit_door"} was a
// silent always-false no-op (evalPosition only knew in_range on raw coords).
func TestEvalPosition_InShape(t *testing.T) {
	door := datasims.SimulationShape{ShapeUID: "exit_door", Type: models.ShapeTypeRectangle,
		Position: common.Point{X: 10, Y: 10}, Width: cu64(2), Height: cu64(4)}
	inside := pointAgent(1, 11, 12)
	outside := pointAgent(2, 40, 40)
	rs := newRS(blockEnv(door), inside, outside)

	condIn := models.Condition{Type: models.ConditionTypePosition, Operator: models.ConditionOpIn, TargetUID: "exit_door"}
	if !EvalCondition(rs, rs.Agents[0], nil, condIn) {
		t.Fatal("agent inside the door shape must satisfy position-in")
	}
	if EvalCondition(rs, rs.Agents[1], nil, condIn) {
		t.Fatal("agent far away must not satisfy position-in")
	}

	condNotIn := models.Condition{Type: models.ConditionTypePosition, Operator: models.ConditionOpNotIn, TargetUID: "exit_door"}
	if EvalCondition(rs, rs.Agents[0], nil, condNotIn) {
		t.Fatal("agent inside must not satisfy position-not_in")
	}
	if !EvalCondition(rs, rs.Agents[1], nil, condNotIn) {
		t.Fatal("agent outside must satisfy position-not_in")
	}

	// Unknown target shape: in=false, not_in=true (nothing to be inside of).
	condGhost := models.Condition{Type: models.ConditionTypePosition, Operator: models.ConditionOpIn, TargetUID: "ghost"}
	if EvalCondition(rs, rs.Agents[0], nil, condGhost) {
		t.Fatal("position-in with an unknown shape must be false")
	}
}

// U8b: position-in works on PASSABLE (unfilled) shapes too — the whole point
// of a door/sensor region is detecting agents inside it while they pass.
func TestEvalPosition_InUnfilledShape(t *testing.T) {
	door := datasims.SimulationShape{ShapeUID: "exit_door", Type: models.ShapeTypeRectangle,
		Position: common.Point{X: 10, Y: 10}, Width: cu64(2), Height: cu64(4), Fill: boolPt(false)}
	inside := pointAgent(1, 11, 12)
	rs := newRS(blockEnv(door), inside)
	cond := models.Condition{Type: models.ConditionTypePosition, Operator: models.ConditionOpIn, TargetUID: "exit_door"}
	if !EvalCondition(rs, rs.Agents[0], nil, cond) {
		t.Fatal("agent inside an unfilled door shape must satisfy position-in")
	}
}

// move_toward a SHAPE must aim at the shape's middle cell, not its top-left
// anchor. Found live (V2 calibration pilot): 100 agents targeting a 1×3 door
// rect all converged on the anchor cell — the one row a circular footprint
// overlaps the flanking wall in — and the whole evacuation gridlocked.
func TestNearestTargetPos_ShapeAimsAtMiddleCell(t *testing.T) {
	door := datasims.SimulationShape{ShapeUID: "exit_door", Type: models.ShapeTypeRectangle,
		Position: common.Point{X: 39, Y: 18}, Width: cu64(1), Height: cu64(3)}
	agent := pointAgent(1, 5, 19)
	rs := newRS(blockEnv(door), agent)
	got := nearestTargetPos(rs, agent, []string{"exit_door"})
	if got == nil {
		t.Fatal("no target resolved")
	}
	want := common.Point{X: 39, Y: 19} // middle cell of rows 18..20
	if *got != want {
		t.Fatalf("shape aim point = %v, want the middle cell %v", *got, want)
	}
}
