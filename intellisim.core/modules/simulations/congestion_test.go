package simulations

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// --- fixtures --------------------------------------------------------------------

// congEnv builds a non-collision env with the given globals and shapes.
func congEnv(globals map[string]interface{}, shapes ...datasims.SimulationShape) *datasims.SimulationEnvironment {
	return &datasims.SimulationEnvironment{Width: 200, Height: 200, GlobalProperties: globals, Shapes: shapes}
}

// gateShape is an unfilled (passable) rectangle column used as a flow-limited gate.
func gateShape(uid string, x, y, w, h uint64) datasims.SimulationShape {
	return datasims.SimulationShape{ShapeUID: uid, Type: models.ShapeTypeRectangle,
		Position: common.Point{X: x, Y: y}, Width: cu64(w), Height: cu64(h), Fill: boolPt(false)}
}

func sdGlobals(radius, onset, jam, floor float64) map[string]interface{} {
	return map[string]interface{}{
		congKeySDRadius: radius,
		congKeySDOnset:  onset,
		congKeySDJam:    jam,
		congKeySDFloor:  floor,
	}
}

// movingPointAgent is a movable point-footprint agent with a preset velocity.
func movingPointAgent(id uint64, x, y uint64, vx, vy float64) *RuntimeAgent {
	ra := pointAgent(id, x, y)
	ra.Velocity = Vector{DX: vx, DY: vy}
	return ra
}

// --- settings parser --------------------------------------------------------------

func TestCongestionSettingsFor(t *testing.T) {
	cases := []struct {
		name        string
		env         *datasims.SimulationEnvironment
		wantDensity bool
		wantLimits  int
		wantWarn    string // substring of an expected warning ("" = expect none)
	}{
		{name: "nil env", env: nil},
		{name: "no keys", env: congEnv(map[string]interface{}{"temperature": 20.0})},
		{name: "valid speed density", env: congEnv(sdGlobals(2.0, 0.086, 0.55, 0.5)), wantDensity: true},
		{name: "partial speed density",
			env:      congEnv(map[string]interface{}{congKeySDRadius: 2.0}),
			wantWarn: "incomplete or non-numeric"},
		{name: "non numeric speed density",
			env: congEnv(map[string]interface{}{
				congKeySDRadius: "fast", congKeySDOnset: 0.1, congKeySDJam: 0.5, congKeySDFloor: 0.5}),
			wantWarn: "incomplete or non-numeric"},
		{name: "zero floor rejected",
			env:      congEnv(sdGlobals(2.0, 0.086, 0.55, 0.0)),
			wantWarn: "out of range"},
		{name: "jam below onset rejected",
			env:      congEnv(sdGlobals(2.0, 0.6, 0.5, 0.5)),
			wantWarn: "out of range"},
		{name: "valid flow limit",
			env: congEnv(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 0.351},
				gateShape("gate", 10, 0, 1, 10)),
			wantLimits: 1},
		{name: "unknown shape uid skipped",
			env:      congEnv(map[string]interface{}{congKeyFlowLimitPrefix + "ghost": 0.5}),
			wantWarn: "unknown shape uid"},
		{name: "non positive rate skipped",
			env: congEnv(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 0.0},
				gateShape("gate", 10, 0, 1, 10)),
			wantWarn: "not a positive number"},
		{name: "invalid burst falls back to one",
			env: congEnv(map[string]interface{}{
				congKeyFlowLimitPrefix + "gate": 1.0, congKeyFlowBurstPrefix + "gate": 0.2},
				gateShape("gate", 10, 0, 1, 10)),
			wantLimits: 1, wantWarn: "flow_limit_burst"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := &RuntimeState{Env: tc.env}
			got := congestionSettingsFor(rs)
			if (got.density != nil) != tc.wantDensity {
				t.Fatalf("density enabled = %v, want %v", got.density != nil, tc.wantDensity)
			}
			if len(got.limits) != tc.wantLimits {
				t.Fatalf("limits = %d, want %d", len(got.limits), tc.wantLimits)
			}
			if tc.wantWarn == "" {
				if len(rs.warnings) != 0 {
					t.Fatalf("unexpected warnings: %v", rs.warnings)
				}
			} else {
				found := false
				for _, w := range rs.warnings {
					if strings.Contains(w, tc.wantWarn) {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected warning containing %q, got %v", tc.wantWarn, rs.warnings)
				}
			}
		})
	}
}

func TestCongestionSettingsFor_LimitOrderSorted(t *testing.T) {
	env := congEnv(map[string]interface{}{
		congKeyFlowLimitPrefix + "zeta": 1.0,
		congKeyFlowLimitPrefix + "alfa": 1.0,
		congKeyFlowLimitPrefix + "mid":  1.0,
	}, gateShape("zeta", 10, 0, 1, 5), gateShape("alfa", 20, 0, 1, 5), gateShape("mid", 30, 0, 1, 5))
	got := congestionSettingsFor(&RuntimeState{Env: env})
	if len(got.limits) != 3 || got.limits[0].shapeUID != "alfa" || got.limits[1].shapeUID != "mid" || got.limits[2].shapeUID != "zeta" {
		t.Fatalf("limits must be sorted by shape UID, got %+v", got.limits)
	}
}

// --- speed–density factor ----------------------------------------------------------

func TestSpeedDensityFactor(t *testing.T) {
	s := &speedDensitySettings{radius: 2, onset: 0.1, jam: 0.5, floor: 0.25}
	if f := speedDensityFactor(s, 0); f != 1.0 {
		t.Fatalf("f(0) = %v, want exactly 1.0", f)
	}
	if f := speedDensityFactor(s, 0.1); f != 1.0 {
		t.Fatalf("f(onset) = %v, want exactly 1.0", f)
	}
	if f := speedDensityFactor(s, 0.3); math.Abs(f-0.625) > 1e-12 {
		t.Fatalf("f(midpoint) = %v, want 0.625 (linear)", f)
	}
	if f := speedDensityFactor(s, 0.5); f != 0.25 {
		t.Fatalf("f(jam) = %v, want floor 0.25", f)
	}
	if f := speedDensityFactor(s, 5); f != 0.25 {
		t.Fatalf("f(>jam) = %v, want floor 0.25", f)
	}
	prev := 2.0
	for d := 0.0; d <= 1.0; d += 0.01 {
		f := speedDensityFactor(s, d)
		if f > prev+1e-12 {
			t.Fatalf("factor not monotone non-increasing at d=%v", d)
		}
		prev = f
	}
}

func TestDensityFactors_SelfExcludedAndLonerUnaffected(t *testing.T) {
	// Any neighbour within radius 2 pushes density past jam=0.01 → factor = floor.
	s := &speedDensitySettings{radius: 2, onset: 0.005, jam: 0.01, floor: 0.5}
	loner := movingPointAgent(1, 100, 100, 1, 0)
	a := movingPointAgent(2, 10, 10, 1, 0)
	b := movingPointAgent(3, 11, 10, 1, 0)
	rs := newRS(congEnv(nil), loner, a, b)
	factors := densityFactors(rs, rs.Agents, s)
	if _, ok := factors[1]; ok {
		t.Fatal("isolated agent must have no factor (self excluded ⇒ D = 0 ⇒ f = 1)")
	}
	if f := factors[2]; f != 0.5 {
		t.Fatalf("clustered agent factor = %v, want floor 0.5", f)
	}
	if f := factors[3]; f != 0.5 {
		t.Fatalf("clustered agent factor = %v, want floor 0.5", f)
	}
}

func TestDensityFactors_StaticNeighborCountsButGetsNoFactor(t *testing.T) {
	s := &speedDensitySettings{radius: 2, onset: 0.005, jam: 0.01, floor: 0.5}
	walker := movingPointAgent(1, 10, 10, 1, 0)
	standing := pointAgent(2, 11, 10)
	standing.Archetype.MoveType = models.AgentMoveTypeStatic
	rs := newRS(congEnv(nil), walker, standing)
	factors := densityFactors(rs, rs.Agents, s)
	if f := factors[1]; f != 0.5 {
		t.Fatalf("walker beside a standing agent: factor = %v, want 0.5 (static bodies count toward density)", f)
	}
	if _, ok := factors[2]; ok {
		t.Fatal("static agent must not receive a factor (it never moves)")
	}
}

// --- byte-identity pins -------------------------------------------------------------

// TestApplySpatial_CongestionKeysIdle_ByteIdentical: a state with congestion keys
// that never bind (high onset; a gated shape nobody approaches) must produce the
// exact same positions and remainders as a state with no congestion keys at all.
func TestApplySpatial_CongestionKeysIdle_ByteIdentical(t *testing.T) {
	build := func(withKeys bool) *RuntimeState {
		var globals map[string]interface{}
		var shapes []datasims.SimulationShape
		if withKeys {
			globals = sdGlobals(2.0, 100.0, 200.0, 0.5) // onset far above any reachable density
			globals[congKeyFlowLimitPrefix+"gate"] = 0.5
			shapes = append(shapes, gateShape("gate", 190, 190, 2, 4)) // far corner, untouched
		}
		env := congEnv(globals, shapes...)
		a := movingPointAgent(1, 10, 10, 0.7, 0.3)
		b := movingPointAgent(2, 12, 10, -0.4, 0.9)
		c := movingPointAgent(3, 11, 12, 1.0, 0.0)
		return newRS(env, a, b, c)
	}
	with, without := build(true), build(false)
	for step := 0; step < 25; step++ {
		applySpatial(with, with.Agents, map[uint64]*AgentIntent{})
		applySpatial(without, without.Agents, map[uint64]*AgentIntent{})
		for i := range with.Agents {
			pw, po := *with.Agents[i].A.Position, *without.Agents[i].A.Position
			if pw != po {
				t.Fatalf("step %d agent %d: position diverged with idle congestion keys: %v vs %v", step, i+1, pw, po)
			}
			if with.Agents[i].rem != without.Agents[i].rem {
				t.Fatalf("step %d agent %d: remainder diverged: %v vs %v", step, i+1, with.Agents[i].rem, without.Agents[i].rem)
			}
		}
	}
}

// TestApplySpatial_DensityScalesLocalCopyNotVelocity: the factor must scale each
// step's integration without mutating the sticky velocity — displacement over k
// steps is k·f·v (linear), not v·(f + f² + …) (geometric compounding).
func TestApplySpatial_DensityScalesLocalCopyNotVelocity(t *testing.T) {
	globals := sdGlobals(2.0, 0.005, 0.01, 0.5) // any neighbour ⇒ factor = 0.5
	a := movingPointAgent(1, 10, 10, 1, 0)
	b := movingPointAgent(2, 10, 11, 1, 0) // parallel mover keeps the pair adjacent
	rs := newRS(congEnv(globals), a, b)
	for step := 0; step < 8; step++ {
		applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	}
	if a.Velocity != (Vector{DX: 1, DY: 0}) {
		t.Fatalf("sticky velocity mutated: %+v", a.Velocity)
	}
	// 8 steps × (1.0 × 0.5) = 4 cells. Geometric compounding would give ~2.
	if a.A.Position.X != 14 {
		t.Fatalf("agent X = %d after 8 half-speed steps from 10, want 14 (linear slowdown)", a.A.Position.X)
	}
}

// --- flow limits --------------------------------------------------------------------

// flowState builds n point agents at x=start (rows y=1..n) all moving +1/step toward
// a vertical gate column at x=gx spanning the agents' rows.
func flowState(globals map[string]interface{}, n int, start, gx uint64) *RuntimeState {
	agents := make([]*RuntimeAgent, 0, n)
	for i := 0; i < n; i++ {
		agents = append(agents, movingPointAgent(uint64(i+1), start, uint64(i+1), 1, 0))
	}
	env := congEnv(globals, gateShape("gate", gx, 0, 1, uint64(n+2)))
	return newRS(env, agents...)
}

// pastOrIn counts agents whose x reached the gate column or beyond (admitted at
// some earlier step; nothing removes them in these unit tests).
func pastOrIn(rs *RuntimeState, gx uint64) int {
	n := 0
	for _, ra := range rs.Agents {
		if ra.A.Position.X >= gx {
			n++
		}
	}
	return n
}

func TestFlowLimit_FractionalThroughputStaircase(t *testing.T) {
	rs := flowState(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 0.351}, 6, 9, 10)
	admittedAt := map[int]int{} // step → cumulative admitted
	for step := 1; step <= 20; step++ {
		applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
		admittedAt[step] = pastOrIn(rs, 10)
	}
	// budget: 0.351k with one unit consumed per admission; admissions land at
	// steps 3, 6, 9, 12, 15, 18 (clamp-before-accrue keeps the fractional carry).
	wantSteps := []int{3, 6, 9, 12, 15, 18}
	cum := 0
	for step := 1; step <= 20; step++ {
		if cum < len(wantSteps) && step == wantSteps[cum] {
			cum++
		}
		if admittedAt[step] != cum {
			t.Fatalf("step %d: admitted %d, want %d (staircase %v)", step, admittedAt[step], cum, wantSteps)
		}
	}
	if got := int(math.Ceil(6 / 0.351)); got != 18 {
		t.Fatalf("sanity: N/rate = %d, expected 18", got)
	}
}

func TestFlowLimit_IntegerAndSupraUnitRates(t *testing.T) {
	// rate 2.0 ⇒ exactly two admissions per step.
	rs := flowState(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 2.0}, 6, 9, 10)
	for step := 1; step <= 3; step++ {
		applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
		if got := pastOrIn(rs, 10); got != 2*step {
			t.Fatalf("rate 2.0 step %d: admitted %d, want %d", step, got, 2*step)
		}
	}
	// rate 2.5 ⇒ 2, then 3 (carry), then the last of six.
	rs2 := flowState(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 2.5}, 6, 9, 10)
	wantCum := []int{2, 5, 6}
	for step := 1; step <= 3; step++ {
		applySpatial(rs2, rs2.Agents, map[uint64]*AgentIntent{})
		if got := pastOrIn(rs2, 10); got != wantCum[step-1] {
			t.Fatalf("rate 2.5 step %d: admitted %d, want %d", step, got, wantCum[step-1])
		}
	}
}

func TestFlowLimit_BurstCapAfterIdle(t *testing.T) {
	run := func(burst interface{}) int {
		globals := map[string]interface{}{congKeyFlowLimitPrefix + "gate": 0.5}
		if burst != nil {
			globals[congKeyFlowBurstPrefix+"gate"] = burst
		}
		// Agents parked far from the gate for 20 idle ticks.
		agents := make([]*RuntimeAgent, 0, 5)
		for i := 0; i < 5; i++ {
			agents = append(agents, movingPointAgent(uint64(i+1), 100, uint64(i+1), 0, 0))
		}
		env := congEnv(globals, gateShape("gate", 10, 0, 1, 8))
		rs := newRS(env, agents...)
		for step := 0; step < 20; step++ {
			applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
		}
		// Teleport the whole group to x=9 and give them +1 velocity: all arrive together.
		for _, ra := range rs.Agents {
			p := common.Point{X: 9, Y: ra.A.Position.Y}
			ra.A.Position = &p
			ra.Velocity = Vector{DX: 1, DY: 0}
		}
		applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
		return pastOrIn(rs, 10)
	}
	if got := run(nil); got != 1 {
		t.Fatalf("default burst 1.0: first contact step admitted %d, want 1 (idle budget capped)", got)
	}
	if got := run(3.0); got != 3 {
		t.Fatalf("burst 3.0: first contact step admitted %d, want 3", got)
	}
}

func TestFlowLimit_AdmissionOrderAndHeldRevert(t *testing.T) {
	rs := flowState(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 1.0}, 2, 9, 10)
	// Seed distinct sub-cell remainders to verify the held agent's rem is restored.
	rs.Agents[0].rem = Vec2{X: 0.25}
	rs.Agents[1].rem = Vec2{X: 0.3}
	applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	a, b := rs.Agents[0], rs.Agents[1]
	if a.A.Position.X < 10 {
		t.Fatalf("lowest id must be admitted first, agent 1 at %v", a.A.Position)
	}
	if b.A.Position.X != 9 {
		t.Fatalf("agent 2 must be held at its pre-phase position, got %v", b.A.Position)
	}
	if b.rem != (Vec2{X: 0.3}) {
		t.Fatalf("held agent's remainder must be restored, got %v", b.rem)
	}
}

func TestFlowLimit_SharedBudgetPerUID_IndependentAcrossUIDs(t *testing.T) {
	// Two instances of ONE template UID share a single budget.
	shared := congEnv(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 1.0},
		gateShape("gate", 10, 0, 1, 4), gateShape("gate", 10, 20, 1, 4))
	a := movingPointAgent(1, 9, 1, 1, 0)
	b := movingPointAgent(2, 9, 21, 1, 0)
	rs := newRS(shared, a, b)
	applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	if got := pastOrIn(rs, 10); got != 1 {
		t.Fatalf("two instances of one UID must share one budget: admitted %d, want 1", got)
	}

	// Two different UIDs are independent.
	indep := congEnv(map[string]interface{}{
		congKeyFlowLimitPrefix + "north": 1.0,
		congKeyFlowLimitPrefix + "south": 1.0,
	}, gateShape("north", 10, 0, 1, 4), gateShape("south", 10, 20, 1, 4))
	c := movingPointAgent(1, 9, 1, 1, 0)
	d := movingPointAgent(2, 9, 21, 1, 0)
	rs2 := newRS(indep, c, d)
	applySpatial(rs2, rs2.Agents, map[uint64]*AgentIntent{})
	if got := pastOrIn(rs2, 10); got != 2 {
		t.Fatalf("independent UIDs must not share budgets: admitted %d, want 2", got)
	}
}

func TestFlowLimit_TorusEntry(t *testing.T) {
	env := congEnv(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 0.25},
		gateShape("gate", 0, 0, 2, 8))
	env.Torus = true
	a := movingPointAgent(1, 199, 1, 2, 0) // wraps 199 → 1, into the gate span [0,2]
	rs := newRS(env, a)
	applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	// budget 0.25 < 1 on the first tick ⇒ held at the pre-wrap position.
	if *a.A.Position != (common.Point{X: 199, Y: 1}) {
		t.Fatalf("torus entry must be gated: agent at %v, want held at (199,1)", a.A.Position)
	}
	for step := 0; step < 3; step++ {
		applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	}
	// After 4 accruals the budget reaches 1.0 ⇒ admitted through the wrap.
	if a.A.Position.X > 100 {
		t.Fatalf("agent should have been admitted through the torus seam, still at %v", a.A.Position)
	}
}

func TestFlowLimit_StartInsideGateNeverCharged(t *testing.T) {
	env := congEnv(map[string]interface{}{congKeyFlowLimitPrefix + "gate": 0.001},
		gateShape("gate", 10, 0, 4, 8))
	inside := movingPointAgent(1, 11, 2, 1, 0) // starts inside, moves within then out
	rs := newRS(env, inside)
	for step := 0; step < 5; step++ {
		applySpatial(rs, rs.Agents, map[uint64]*AgentIntent{})
	}
	// Never held: interior movement and exit are not entry transitions.
	if inside.A.Position.X != 16 {
		t.Fatalf("agent starting inside the gate must never be held, at %v after 5 steps", inside.A.Position)
	}
}

// --- determinism / update-mode parity ------------------------------------------------

// congestedSim builds a NewRuntimeState-backed sim: n agents behind a flow-limited
// gate, driven by a move_toward rule, removed by a position-in rule, with
// speed–density coupling active — the full EXP-C mechanism in miniature.
func congestedSim(t *testing.T, mode models.ModelUpdateMode, seed uint64, n int, rate float64) *RuntimeState {
	t.Helper()
	speed := 1.0
	person := models.Agent{
		UID:      "person",
		Type:     models.AgentTypePlain,
		MoveType: models.AgentMoveTypeMovable,
		States:   []string{"moving"},
		Initialization: models.AgentInitialization{InitialState: "moving"},
	}
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{UpdateMode: mode},
		IsSpatial:        true,
		SpatialCellSize:  5,
		Agents:           []models.Agent{person},
		Environment: &models.Environment{
			Width: 40, Height: 40,
			Shapes: []models.Shape{{UID: "gate", Type: models.ShapeTypeRectangle}},
		},
		Rules: []models.Rule{
			{
				ID: "walk", Enabled: true, Priority: 10, AppliesTo: []string{"person"},
				Actions: []models.Action{{Type: models.ActionTypeMoveToward, TargetUID: "gate", Speed: &speed}},
			},
			{
				ID: "leave", Enabled: true, Priority: 20, AppliesTo: []string{"person"},
				Conditions: []models.Condition{{Type: models.ConditionTypePosition, Operator: models.ConditionOpIn, TargetUID: "gate"}},
				Actions:    []models.Action{{Type: models.ActionTypeRemoveAgent}},
			},
		},
	}
	globals := sdGlobals(2.0, 0.02, 0.2, 0.5)
	globals[congKeyFlowLimitPrefix+"gate"] = rate
	sim := &datasims.Simulation{
		ID: "sim-congestion", Seed: seed, Steps: 500,
		Environment: &datasims.SimulationEnvironment{
			Width: 40, Height: 40, GlobalProperties: globals,
			Shapes: []datasims.SimulationShape{gateShape("gate", 30, 18, 1, 3)},
		},
		ModelSnapshot: model,
	}
	for i := 0; i < n; i++ {
		sim.Agents = append(sim.Agents, datasims.SimulationAgent{
			ID: uint64(i + 1), AgentUID: "person", State: "moving",
			Position: &common.Point{X: uint64(20 + i%3), Y: uint64(16 + i%5)},
		})
	}
	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	return rs
}

func alive(rs *RuntimeState) int {
	n := 0
	for _, ra := range rs.Agents {
		if ra != nil && ra.A != nil && !ra.removed {
			n++
		}
	}
	return n
}

func TestCongestion_SameSeedDeterminism(t *testing.T) {
	withFixedClock(t)
	run := func() []string {
		rs := congestedSim(t, models.UpdateModeSequential, 42, 8, 0.5)
		var trace []string
		for step := 0; step < 60; step++ {
			if _, err := RunStep(context.Background(), rs, nil); err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
			for _, ra := range rs.Agents {
				trace = append(trace, fmt.Sprintf("%d:%d,%d", ra.A.ID, ra.A.Position.X, ra.A.Position.Y))
			}
			trace = append(trace, "|")
		}
		return trace
	}
	t1, t2 := run(), run()
	if len(t1) != len(t2) {
		t.Fatalf("trace lengths differ: %d vs %d", len(t1), len(t2))
	}
	for i := range t1 {
		if t1[i] != t2[i] {
			t.Fatalf("trace diverged at %d: %q vs %q", i, t1[i], t2[i])
		}
	}
}

func TestCongestion_SequentialVsSynchronousParity(t *testing.T) {
	withFixedClock(t)
	seq := congestedSim(t, models.UpdateModeSequential, 7, 8, 0.5)
	syn := congestedSim(t, models.UpdateModeSynchronous, 7, 8, 0.5)
	for step := 0; step < 60; step++ {
		if _, err := RunStep(context.Background(), seq, nil); err != nil {
			t.Fatalf("seq step %d: %v", step, err)
		}
		if _, err := RunStep(context.Background(), syn, nil); err != nil {
			t.Fatalf("syn step %d: %v", step, err)
		}
		if alive(seq) != alive(syn) {
			t.Fatalf("step %d: population diverged seq=%d syn=%d", step, alive(seq), alive(syn))
		}
		seqPos := map[uint64]common.Point{}
		for _, ra := range seq.Agents {
			seqPos[ra.A.ID] = *ra.A.Position
		}
		for _, ra := range syn.Agents {
			if p, ok := seqPos[ra.A.ID]; ok && p != *ra.A.Position {
				t.Fatalf("step %d agent %d: position diverged seq=%v syn=%v", step, ra.A.ID, p, *ra.A.Position)
			}
		}
	}
}

// TestCongestion_SyncWorkViewSharesBudget: with rate 0.5 an admission requires two
// ticks of accrual; if the budget did not survive the synchronous workView
// commit, it would restart from zero every tick and nobody would ever be admitted.
func TestCongestion_SyncWorkViewSharesBudget(t *testing.T) {
	withFixedClock(t)
	rs := congestedSim(t, models.UpdateModeSynchronous, 3, 2, 0.5)
	start := alive(rs)
	for step := 0; step < 30; step++ {
		if _, err := RunStep(context.Background(), rs, nil); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
	if alive(rs) >= start {
		t.Fatalf("no agent was ever admitted/removed in synchronous mode: budget lost across ticks (alive %d of %d)", alive(rs), start)
	}
}

// TestCongestion_GateBindsBeforeRemoveRule: with rate 1.0 and the default burst
// cap 1.0 at most ⌊burst+rate⌋ = 2 agents can be admitted in any tick (two only
// right after an idle tick), and the position-in remove rule fires one tick after
// admission — so the population may never drop by more than two per tick and every
// removal corresponds to a prior admission.
func TestCongestion_GateBindsBeforeRemoveRule(t *testing.T) {
	withFixedClock(t)
	rs := congestedSim(t, models.UpdateModeSequential, 11, 6, 1.0)
	prev := alive(rs)
	for step := 0; step < 80 && alive(rs) > 0; step++ {
		if _, err := RunStep(context.Background(), rs, nil); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		cur := alive(rs)
		if prev-cur > 2 {
			t.Fatalf("step %d: %d agents removed in one tick despite rate 1.0 / burst 1.0 (gate must bind before removal)", step, prev-cur)
		}
		prev = cur
	}
	if alive(rs) != 0 {
		t.Fatalf("world did not drain: %d agents left", alive(rs))
	}
}

// TestCongestion_DrainTimeMatchesServiceRate: a mini EXP-C — the total drain time
// of a flow-limited world must track N/rate, not the unconstrained crowd speed.
func TestCongestion_DrainTimeMatchesServiceRate(t *testing.T) {
	withFixedClock(t)
	rs := congestedSim(t, models.UpdateModeSequential, 5, 8, 0.351)
	drained := -1
	for step := 1; step <= 120; step++ {
		if _, err := RunStep(context.Background(), rs, nil); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if alive(rs) == 0 {
			drained = step
			break
		}
	}
	if drained < 0 {
		t.Fatal("world did not drain within 120 steps")
	}
	// 8 agents / 0.351 per step ≈ 22.8 steps of service + startup/removal offsets.
	if drained < 22 || drained > 45 {
		t.Fatalf("drain took %d steps, want ≈ N/rate (23) within [22,45]", drained)
	}
}

// TestCongestion_NoWallTunnelingUnderPressure (D30 pin): a crowd pressing against a
// solid wall with a single flow-limited passable gap must never end a tick
// overlapping the wall, and the world must still drain.
func TestCongestion_NoWallTunnelingUnderPressure(t *testing.T) {
	withFixedClock(t)
	speed := 1.0
	person := models.Agent{
		UID: "person", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable,
		States:         []string{"moving"},
		Footprint:      &models.AgentFootprint{Kind: models.FootprintCircle, Radius: cf64(0.5)},
		Initialization: models.AgentInitialization{InitialState: "moving"},
	}
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{UpdateMode: models.UpdateModeSequential},
		IsSpatial:        true, SpatialCellSize: 5,
		Agents: []models.Agent{person},
		Environment: &models.Environment{Width: 40, Height: 40,
			Shapes: []models.Shape{{UID: "gate", Type: models.ShapeTypeRectangle}}},
		Rules: []models.Rule{
			{ID: "walk", Enabled: true, Priority: 10, AppliesTo: []string{"person"},
				Actions: []models.Action{{Type: models.ActionTypeMoveToward, TargetUID: "gate", Speed: &speed}}},
			{ID: "leave", Enabled: true, Priority: 20, AppliesTo: []string{"person"},
				Conditions: []models.Condition{{Type: models.ConditionTypePosition, Operator: models.ConditionOpIn, TargetUID: "gate"}},
				Actions:    []models.Action{{Type: models.ActionTypeRemoveAgent}}},
		},
	}
	globals := sdGlobals(2.0, 0.02, 0.2, 0.5)
	globals[congKeyFlowLimitPrefix+"gate"] = 0.351
	wallTop := datasims.SimulationShape{ShapeUID: "wall_top", Type: models.ShapeTypeRectangle,
		Position: common.Point{X: 20, Y: 0}, Width: cu64(1), Height: cu64(18)}
	wallBottom := datasims.SimulationShape{ShapeUID: "wall_bottom", Type: models.ShapeTypeRectangle,
		Position: common.Point{X: 20, Y: 21}, Width: cu64(1), Height: cu64(19)}
	sim := &datasims.Simulation{
		ID: "sim-wall", Seed: 3, Steps: 500,
		Environment: &datasims.SimulationEnvironment{
			Width: 40, Height: 40, CollisionMode: models.CollisionBlock,
			GlobalProperties: globals,
			Shapes:           []datasims.SimulationShape{wallTop, wallBottom, gateShape("gate", 20, 18, 1, 3)},
		},
		ModelSnapshot: model,
	}
	id := uint64(1)
	for dx := uint64(0); dx < 4; dx++ {
		for dy := uint64(0); dy < 3; dy++ {
			sim.Agents = append(sim.Agents, datasims.SimulationAgent{
				ID: id, AgentUID: "person", State: "moving",
				Position: &common.Point{X: 15 + dx, Y: 17 + dy*2},
			})
			id++
		}
	}
	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	walls := []*datasims.SimulationShape{&rs.Env.Shapes[0], &rs.Env.Shapes[1]}
	for step := 1; step <= 150 && alive(rs) > 0; step++ {
		if _, err := RunStep(context.Background(), rs, nil); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		for _, ra := range rs.Agents {
			if ra.removed || ra.A.Position == nil {
				continue
			}
			for _, wall := range walls {
				if Overlap(bodyForAgent(ra), bodyForShape(wall), rs.Env.Width, rs.Env.Height, false) {
					t.Fatalf("step %d: agent %d ended the tick overlapping wall %s at %v (D30 tunneling)",
						step, ra.A.ID, wall.ShapeUID, *ra.A.Position)
				}
			}
		}
	}
	if alive(rs) != 0 {
		t.Fatalf("gated world did not drain: %d agents left after 150 steps", alive(rs))
	}
}

// TestCongestion_PushedIntoGateIsChargedOrHeld: under block-mode crowd pressure an
// agent shoved into the gate must consume budget (or be held) — never slip through
// unpaid. Rate 0.5 with the default burst 1.0 bounds admissions at ⌊1+0.5⌋ = 1 per
// tick, so any tick losing more than one agent proves an unpaid slip-through.
func TestCongestion_PushedIntoGateIsChargedOrHeld(t *testing.T) {
	withFixedClock(t)
	rs := congestedSim(t, models.UpdateModeSequential, 13, 10, 0.5)
	rs.Env.CollisionMode = models.CollisionBlock
	prev := alive(rs)
	for step := 0; step < 150 && alive(rs) > 0; step++ {
		if _, err := RunStep(context.Background(), rs, nil); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if prev-alive(rs) > 1 {
			t.Fatalf("step %d: more than one admission per tick under crowd pressure", step)
		}
		prev = alive(rs)
	}
	if alive(rs) != 0 {
		t.Fatal("pressured world did not drain")
	}
}
