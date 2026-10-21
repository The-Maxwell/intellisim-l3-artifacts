package simulations

import (
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// U5: best_effort keeps the partial resolution the push iterations achieved
// instead of reverting a wedged agent — the freeze disappears, the transient
// overlap keeps being pushed out on later steps.
func TestResolve_BestEffort_NoRevert(t *testing.T) {
	// A world fully covered by a solid shape: push-out cannot fully succeed.
	bigRect := datasims.SimulationShape{Type: models.ShapeTypeRectangle, Position: common.Point{X: 0, Y: 0}, Width: cu64(200), Height: cu64(200)}
	before := common.Point{X: 10, Y: 10}

	// Legacy/default policy: revert to the pre-phase position (pinned by
	// TestResolve_CancelFallback). best_effort must NOT revert.
	a := circleAgent(1, 100, 100, 5)
	env := blockEnv(bigRect)
	env.CollisionConfig = &models.CollisionConfig{StuckPolicy: models.CollisionStuckBestEffort}
	rs := newRS(env, a)
	resolveCollisions(rs, rs.Agents, map[uint64]common.Point{1: before})
	if *a.A.Position == before {
		t.Fatal("best_effort must keep the partial resolution, not revert to the pre-phase position")
	}
}

// U5: an explicit config carrying the engine defaults behaves exactly like no
// config at all (legacy behavior stays byte-identical).
func TestResolve_ExplicitDefaultsMatchLegacy(t *testing.T) {
	run := func(cfg *models.CollisionConfig) (common.Point, common.Point) {
		a := circleAgent(1, 50, 50, 10)
		b := circleAgent(2, 56, 50, 10)
		env := blockEnv()
		env.CollisionConfig = cfg
		rs := newRS(env, a, b)
		resolveCollisions(rs, rs.Agents, beforeMap(a, b))
		return *a.A.Position, *b.A.Position
	}
	a1, b1 := run(nil)
	a2, b2 := run(&models.CollisionConfig{MaxIterations: 4, StuckPolicy: models.CollisionStuckRevert})
	if a1 != a2 || b1 != b2 {
		t.Fatalf("explicit defaults diverged from legacy: (%v,%v) vs (%v,%v)", a1, b1, a2, b2)
	}
}

// U5: settings resolution — defaults, override, zero-floor.
func TestCollisionSettingsFor(t *testing.T) {
	cases := []struct {
		name string
		env  *datasims.SimulationEnvironment
		want collisionSettings
	}{
		{"nil env", nil, collisionSettings{maxIters: 4}},
		{"no config", &datasims.SimulationEnvironment{}, collisionSettings{maxIters: 4}},
		{"raised cap", &datasims.SimulationEnvironment{CollisionConfig: &models.CollisionConfig{MaxIterations: 12}}, collisionSettings{maxIters: 12}},
		{"zero cap floors to default", &datasims.SimulationEnvironment{CollisionConfig: &models.CollisionConfig{}}, collisionSettings{maxIters: 4}},
		{"best effort", &datasims.SimulationEnvironment{CollisionConfig: &models.CollisionConfig{StuckPolicy: models.CollisionStuckBestEffort}}, collisionSettings{maxIters: 4, bestEffort: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := collisionSettingsFor(tc.env); got != tc.want {
				t.Fatalf("collisionSettingsFor = %+v, want %+v", got, tc.want)
			}
		})
	}
}
