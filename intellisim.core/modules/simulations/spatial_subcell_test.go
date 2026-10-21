package simulations

import (
	"math"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// U4 acceptance 1+2: fractional commanded speeds must converge to the commanded
// distance instead of being re-rounded away every step (1.33 → 133±1 over 100
// steps; 0.3 → 30±1 with no stall).
func TestIntegrateMovementBodyRem_FractionalSpeedConverges(t *testing.T) {
	cases := []struct {
		name  string
		speed float64
		steps int
		want  float64 // expected covered distance in cells
	}{
		{"rimea walking 1.33", 1.33, 100, 133},
		{"slow 0.3 no stall", 0.3, 100, 30},
		{"biased fraction 3.325", 3.325, 100, 332.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pos := common.Point{X: 10, Y: 10}
			rem := Vec2{}
			body := pointBody(Vec2{X: float64(pos.X), Y: float64(pos.Y)})
			for i := 0; i < tc.steps; i++ {
				pos, rem = integrateMovementBodyRem(pos, rem, Vector{DX: tc.speed}, body, 1000, 100, false)
				if math.Abs(rem.X) > 0.5+1e-9 || math.Abs(rem.Y) > 0.5+1e-9 {
					t.Fatalf("step %d: remainder out of bounds: %+v", i, rem)
				}
			}
			got := float64(pos.X) - 10
			if math.Abs(got-tc.want) > 1 {
				t.Fatalf("covered %.1f cells in %d steps at speed %v; want %.1f ± 1", got, tc.steps, tc.speed, tc.want)
			}
		})
	}
}

// U4 wiring: the spatial phase itself must carry the per-agent remainder so a
// fractional velocity converges at the engine level, not just in the integrator
// helper.
func TestApplySpatial_SubCellSpeedConverges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		speed float64
		want  float64
	}{
		{"rimea walking 1.33", 1.33, 133},
		{"slow 0.3 no stall", 0.3, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ra := &RuntimeAgent{
				A:         &datasims.SimulationAgent{ID: 1, Position: &common.Point{X: 10, Y: 10}},
				Archetype: &models.Agent{MoveType: models.AgentMoveTypeMovable},
				Velocity:  Vector{DX: tc.speed},
			}
			rs := newRS(&datasims.SimulationEnvironment{Width: 1000, Height: 100}, ra)
			for i := 0; i < 100; i++ {
				applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
			}
			got := float64(ra.A.Position.X) - 10
			if math.Abs(got-tc.want) > 1 {
				t.Fatalf("covered %.1f cells in 100 steps at speed %v; want %.1f ± 1", got, tc.speed, tc.want)
			}
		})
	}
}

// U4 sync mode: the working-copy clone and its commit must carry the movement
// remainder, or synchronous-update runs would silently re-round every step.
func TestSyncClone_CarriesMovementRemainder(t *testing.T) {
	live := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, Position: &common.Point{X: 10, Y: 10}},
		Archetype: &models.Agent{MoveType: models.AgentMoveTypeMovable},
		rem:       Vec2{X: 0.25, Y: -0.25},
	}
	w := cloneRuntimeAgent(live)
	if w.rem != live.rem {
		t.Fatalf("clone must carry rem: got %+v want %+v", w.rem, live.rem)
	}
	w.rem = Vec2{X: -0.4, Y: 0.1}
	commitRuntimeAgent(live, w)
	if live.rem != (Vec2{X: -0.4, Y: 0.1}) {
		t.Fatalf("commit must copy rem back: got %+v", live.rem)
	}
}

// U4 collision semantics: when block-mode collision response corrects an agent's
// integrated position on an axis, that axis's remainder must be zeroed — the
// sub-cell state must never disagree with a collision-corrected integer position
// (otherwise a wall-pressed agent keeps phantom sub-cell progress into the wall
// and oscillates off its resolved position). The untouched axis keeps its remainder.
func TestApplySpatial_CollisionZeroesCorrectedAxisRemainder(t *testing.T) {
	// Solid full-height wall at x ∈ [100,120]; circle agent (r=5) walking +1.6/step
	// straight into it, carrying a pre-seeded Y remainder that no collision ever
	// corrects (pure-X press: MTV pushes and the touch-revert fallback both leave
	// Y at its pre-step value, so the Y axis is never "changed" by the collision
	// layer).
	wall := datasims.SimulationShape{Type: models.ShapeTypeRectangle, Position: common.Point{X: 100, Y: 0}, Width: cu64(20), Height: cu64(200)}
	ra := circleAgent(1, 90, 50, 5)
	ra.Velocity = Vector{DX: 1.6}
	ra.rem = Vec2{Y: 0.25}
	rs := newRS(blockEnv(wall), ra)

	for i := 0; i < 7; i++ {
		applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	}

	// Wall-pressed: the whole footprint stays out of the wall (center ≤ face − r).
	if Overlap(bodyForAgent(ra), bodyForShape(&wall), rs.Env.Width, rs.Env.Height, false) {
		t.Fatalf("agent ended overlapping the wall: pos=%v", ra.A.Position)
	}
	if ra.A.Position.X < 90 || ra.A.Position.X > 95 {
		t.Fatalf("expected wall-pressed center X in [90,95], got %v (rem=%+v)", ra.A.Position, ra.rem)
	}
	if ra.rem.X != 0 {
		t.Fatalf("corrected-axis remainder must be zeroed, got rem.X=%v", ra.rem.X)
	}
	if ra.rem.Y != 0.25 {
		t.Fatalf("untouched-axis remainder must be preserved (0.25), got rem.Y=%v", ra.rem.Y)
	}
}

// U4 teleport semantics: a teleport discards any accumulated sub-cell remainder —
// the agent rematerializes exactly on its target cell.
func TestApplyTeleport_ResetsMovementRemainder(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, Position: &common.Point{X: 10, Y: 10}},
		Archetype: &models.Agent{MoveType: models.AgentMoveTypeMovable},
		rem:       Vec2{X: 0.4, Y: -0.3},
	}
	rs := newRS(&datasims.SimulationEnvironment{Width: 100, Height: 100}, ra)
	applyTeleport(rs, ra, models.Action{Parameters: map[string]any{"x": 50.0, "y": 60.0}})
	if *ra.A.Position != (common.Point{X: 50, Y: 60}) {
		t.Fatalf("teleport target mismatch: %v", ra.A.Position)
	}
	if ra.rem != (Vec2{}) {
		t.Fatalf("teleport must reset the remainder, got %+v", ra.rem)
	}
}

// U4 acceptance 3 (regression guard): with a zero remainder, the sub-cell
// integrator is position-identical to the legacy IntegrateMovementBody for every
// wrap/clamp regime, and integer velocities on integer clamp bounds never grow a
// remainder — integer-speed models replay exactly as before.
func TestIntegrateMovementBodyRem_IntegerIdentity(t *testing.T) {
	point := pointBody(Vec2{})
	fracBody := CollisionBody{Kind: BodyCircle, AABBHalf: Vec2{X: 2.5, Y: 2.5}}
	cases := []struct {
		name  string
		p     common.Point
		v     Vector
		torus bool
	}{
		{"interior", common.Point{X: 10, Y: 10}, Vector{DX: 3, DY: -2}, false},
		{"clamp low", common.Point{X: 1, Y: 1}, Vector{DX: -5, DY: 0}, false},
		{"clamp high", common.Point{X: 98, Y: 99}, Vector{DX: 5, DY: 3}, false},
		{"torus wrap high", common.Point{X: 98, Y: 99}, Vector{DX: 5, DY: 3}, true},
		{"torus wrap low", common.Point{X: 1, Y: 0}, Vector{DX: -3, DY: -1}, true},
	}
	for _, body := range []struct {
		name string
		b    CollisionBody
	}{{"point", point}, {"fractional half-extent", fracBody}} {
		for _, tc := range cases {
			t.Run(body.name+"/"+tc.name, func(t *testing.T) {
				got, rem := integrateMovementBodyRem(tc.p, Vec2{}, tc.v, body.b, 100, 100, tc.torus)
				want := IntegrateMovementBody(tc.p, tc.v, body.b, 100, 100, tc.torus)
				if got != want {
					t.Fatalf("position diverged from legacy integrator: got %v want %v", got, want)
				}
				if body.b.AABBHalf == (Vec2{}) && rem != (Vec2{}) {
					t.Fatalf("integer velocity on integer bounds must keep a zero remainder, got %+v", rem)
				}
			})
		}
	}
}
