package simulations

import (
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// circleAgent builds a movable runtime agent with a circle footprint at (x,y).
func circleAgent(id uint64, x, y uint64, r float64) *RuntimeAgent {
	return &RuntimeAgent{
		A: &datasims.SimulationAgent{ID: id, Position: &common.Point{X: x, Y: y}},
		Archetype: &models.Agent{
			MoveType:  models.AgentMoveTypeMovable,
			Footprint: &models.AgentFootprint{Kind: models.FootprintCircle, Radius: cf64(r)},
		},
	}
}

func blockEnv(shapes ...datasims.SimulationShape) *datasims.SimulationEnvironment {
	return &datasims.SimulationEnvironment{Width: 200, Height: 200, CollisionMode: models.CollisionBlock, Shapes: shapes}
}

func newRS(env *datasims.SimulationEnvironment, agents ...*RuntimeAgent) *RuntimeState {
	rs := &RuntimeState{Env: env, Agents: agents, ByID: map[uint64]*RuntimeAgent{}}
	for _, ra := range agents {
		rs.ByID[ra.A.ID] = ra
	}
	return rs
}

func beforeMap(agents ...*RuntimeAgent) map[uint64]common.Point {
	m := make(map[uint64]common.Point)
	for _, ra := range agents {
		m[ra.A.ID] = *ra.A.Position
	}
	return m
}

func TestResolve_AgentVsAgent_MTVSplit(t *testing.T) {
	a := circleAgent(1, 50, 50, 10)
	b := circleAgent(2, 56, 50, 10) // centers 6 apart, radii sum 20 → overlap depth 14
	rs := newRS(blockEnv(), a, b)
	resolveCollisions(rs, rs.Agents, beforeMap(a, b))

	// After resolution the two circles must no longer overlap (edge-to-edge ≥ ~0).
	d := BodyDistance(bodyForAgent(a), bodyForAgent(b), rs.Env.Width, rs.Env.Height, false)
	if d < -1 {
		t.Fatalf("agents still overlapping after resolve: dist=%v", d)
	}
	if Overlap(bodyForAgent(a), bodyForAgent(b), rs.Env.Width, rs.Env.Height, false) && d == 0 {
		// allow a ≤1px residual from integer rounding
		if dd := BodyDistance(bodyForAgent(a), bodyForAgent(b), rs.Env.Width, rs.Env.Height, false); dd > 2 {
			t.Fatalf("unexpected separation %v", dd)
		}
	}
	// Both moved roughly symmetrically away from x=53 (the midpoint).
	if a.A.Position.X >= b.A.Position.X {
		t.Errorf("expected a pushed left of b: a=%v b=%v", a.A.Position, b.A.Position)
	}
}

func TestResolve_AgentVsStaticShape_PushOut(t *testing.T) {
	// Solid rectangle [40,40]-[120,120]; a circle agent sitting at its center must be pushed out.
	rect := datasims.SimulationShape{Type: models.ShapeTypeRectangle, Position: common.Point{X: 40, Y: 40}, Width: cu64(80), Height: cu64(80)}
	a := circleAgent(1, 80, 80, 5)
	rs := newRS(blockEnv(rect), a)
	resolveCollisions(rs, rs.Agents, beforeMap(a))
	if Overlap(bodyForAgent(a), bodyForShape(&rect), rs.Env.Width, rs.Env.Height, false) {
		t.Errorf("agent still overlaps the solid rectangle after push-out: pos=%v", a.A.Position)
	}
}

func TestResolve_CancelFallback(t *testing.T) {
	// A small world fully covered by a solid shape: push-out cannot succeed, so the agent is
	// reverted to its (clear) pre-phase position.
	bigRect := datasims.SimulationShape{Type: models.ShapeTypeRectangle, Position: common.Point{X: 0, Y: 0}, Width: cu64(200), Height: cu64(200)}
	a := circleAgent(1, 100, 100, 5)
	before := common.Point{X: 10, Y: 10} // pretend it was clear here last tick
	rs := newRS(blockEnv(bigRect), a)
	resolveCollisions(rs, rs.Agents, map[uint64]common.Point{1: before})
	if *a.A.Position != before {
		t.Errorf("agent stuck in a solid shape should cancel-move to before %v, got %v", before, a.A.Position)
	}
}

func TestResolve_Deterministic(t *testing.T) {
	run := func() (common.Point, common.Point, common.Point) {
		a := circleAgent(1, 50, 50, 10)
		b := circleAgent(2, 54, 50, 10)
		c := circleAgent(3, 52, 56, 10)
		rs := newRS(blockEnv(), a, b, c)
		resolveCollisions(rs, rs.Agents, beforeMap(a, b, c))
		return *a.A.Position, *b.A.Position, *c.A.Position
	}
	a1, b1, c1 := run()
	a2, b2, c2 := run()
	if a1 != a2 || b1 != b2 || c1 != c2 {
		t.Errorf("collision resolution not deterministic: run1=%v,%v,%v run2=%v,%v,%v", a1, b1, c1, a2, b2, c2)
	}
}

func TestResolve_CoincidentTieBreak(t *testing.T) {
	a := circleAgent(1, 50, 50, 10)
	b := circleAgent(2, 50, 50, 10) // exactly coincident
	rs := newRS(blockEnv(), a, b)
	resolveCollisions(rs, rs.Agents, beforeMap(a, b))
	if a.A.Position.X == b.A.Position.X && a.A.Position.Y == b.A.Position.Y {
		t.Errorf("coincident agents must be separated, both at %v", a.A.Position)
	}
}

func TestResolve_NoneMode_NoOp(t *testing.T) {
	// applySpatial must not resolve when CollisionMode is empty (none).
	a := circleAgent(1, 50, 50, 10)
	b := circleAgent(2, 52, 50, 10) // overlapping
	env := &datasims.SimulationEnvironment{Width: 200, Height: 200} // CollisionNone
	rs := newRS(env, a, b)
	applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	if *a.A.Position != (common.Point{X: 50, Y: 50}) || *b.A.Position != (common.Point{X: 52, Y: 50}) {
		t.Errorf("none mode must not move overlapping agents: a=%v b=%v", a.A.Position, b.A.Position)
	}
}

func TestResolve_NonCollidableSkipped(t *testing.T) {
	a := circleAgent(1, 50, 50, 10)
	b := circleAgent(2, 52, 50, 10)
	no := false
	b.Archetype.Collidable = &no // b opts out → neither is pushed by the pair
	rs := newRS(blockEnv(), a, b)
	resolveCollisions(rs, rs.Agents, beforeMap(a, b))
	if *b.A.Position != (common.Point{X: 52, Y: 50}) {
		t.Errorf("non-collidable agent must not move, got %v", b.A.Position)
	}
	if *a.A.Position != (common.Point{X: 50, Y: 50}) {
		t.Errorf("agent must not be pushed by a non-collidable peer, got %v", a.A.Position)
	}
}
