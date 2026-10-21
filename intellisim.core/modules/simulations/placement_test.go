package simulations

import (
	"math"
	"math/rand"
	"testing"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

const (
	envW = 100
	envH = 80
)

func testEnv() *datasims.SimulationEnvironment {
	return &datasims.SimulationEnvironment{Width: envW, Height: envH}
}

func inBounds(t *testing.T, pts []common.Point) {
	t.Helper()
	for i, p := range pts {
		if p.X >= envW || p.Y >= envH {
			t.Fatalf("point %d (%d,%d) out of bounds [%dx%d]", i, p.X, p.Y, envW, envH)
		}
	}
}

func samePoints(a, b []common.Point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- per-algorithm count + bounds ---------------------------------------------

func TestPlaceAgents_CountAndBounds(t *testing.T) {
	const count = 50
	types := []datamodels.PlacementType{
		datamodels.PlacementRandom,
		datamodels.PlacementUniform,
		datamodels.PlacementClustered,
		datamodels.PlacementManual,
		datamodels.PlacementLLM,
	}
	for _, ty := range types {
		rng := RunRNG(1, 1)
		pl := &datamodels.Placement{Type: ty}
		pts, _ := PlaceAgents(rng, count, pl, testEnv())
		if len(pts) != count {
			t.Fatalf("type %s: expected %d points, got %d", ty, count, len(pts))
		}
		inBounds(t, pts)
	}
}

func TestPlaceAgents_ZeroCount(t *testing.T) {
	rng := RunRNG(1, 1)
	pts, _ := PlaceAgents(rng, 0, &datamodels.Placement{Type: datamodels.PlacementRandom}, testEnv())
	if len(pts) != 0 {
		t.Fatalf("expected empty for zero count, got %d", len(pts))
	}
}

func TestPlaceAgents_NilEnvWarns(t *testing.T) {
	rng := RunRNG(1, 1)
	pts, warns := PlaceAgents(rng, 10, &datamodels.Placement{Type: datamodels.PlacementRandom}, nil)
	if len(pts) != 0 {
		t.Fatalf("expected empty when env nil, got %d", len(pts))
	}
	if len(warns) == 0 {
		t.Fatal("expected a warning when env nil")
	}
}

func TestPlaceAgents_NilPlacementTreatedRandom(t *testing.T) {
	rng := RunRNG(1, 1)
	pts, warns := PlaceAgents(rng, 20, nil, testEnv())
	if len(pts) != 20 {
		t.Fatalf("expected 20 points, got %d", len(pts))
	}
	if len(warns) != 0 {
		t.Fatalf("nil placement (random) should not warn, got %+v", warns)
	}
	inBounds(t, pts)
}

// --- uniform lattice -----------------------------------------------------------

func TestPlaceAgents_UniformLattice(t *testing.T) {
	rng := RunRNG(2, 1)
	const count = 16 // perfect 4x4 grid
	pts, _ := PlaceAgents(rng, count, &datamodels.Placement{Type: datamodels.PlacementUniform}, testEnv())
	if len(pts) != count {
		t.Fatalf("expected %d, got %d", count, len(pts))
	}
	inBounds(t, pts)
	// All points must be distinct cells.
	seen := map[common.Point]bool{}
	for _, p := range pts {
		if seen[p] {
			t.Fatalf("uniform produced duplicate cell %+v", p)
		}
		seen[p] = true
	}
	// Distinct X coords and Y coords should number ~sqrt(count)=4 each.
	xs := map[uint64]bool{}
	ys := map[uint64]bool{}
	for _, p := range pts {
		xs[p.X] = true
		ys[p.Y] = true
	}
	if len(xs) != 4 || len(ys) != 4 {
		t.Fatalf("expected 4x4 lattice, got %d distinct x, %d distinct y", len(xs), len(ys))
	}
}

// --- clustered -----------------------------------------------------------------

func TestPlaceAgents_ClusteredNearCenters(t *testing.T) {
	rng := RunRNG(3, 1)
	const count = 200
	clusters := uint64(3)
	radius := uint64(5)
	pl := &datamodels.Placement{
		Type:          datamodels.PlacementClustered,
		Clusters:      &clusters,
		ClusterRadius: &radius,
	}
	pts, _ := PlaceAgents(rng, count, pl, testEnv())
	if len(pts) != count {
		t.Fatalf("expected %d, got %d", count, len(pts))
	}
	inBounds(t, pts)
	// We don't know the centers, but with a small radius the points must form a
	// small number of dense groups: bucket points on a coarse grid and ensure
	// the occupied-bucket count is modest (<= a generous bound).
	buckets := map[[2]uint64]int{}
	for _, p := range pts {
		buckets[[2]uint64{p.X / 10, p.Y / 10}]++
	}
	if len(buckets) > 24 {
		t.Fatalf("clustered points too dispersed: %d occupied buckets", len(buckets))
	}
}

// Note: "manual" placement is intentionally NOT resolved by PlaceAgents — the
// generator leaves manual-placement agent instances unplaced for the user to
// position per-simulation (covered by the generator + validation tests). A
// manual type reaching PlaceAgents (e.g. a shape) scatters randomly via the
// default branch, exercised by TestPlaceAgents_CountAndBounds above.

// --- at_shape ------------------------------------------------------------------

func circleEnv(uid string, cx, cy uint64, r float64) *datasims.SimulationEnvironment {
	env := testEnv()
	env.Shapes = []datasims.SimulationShape{{
		ShapeUID: uid,
		Type:     datamodels.ShapeTypeCircle,
		Position: common.Point{X: cx, Y: cy},
		Radius:   f64ptr(r),
	}}
	return env
}

func TestPlaceAgents_AtShapeCircle(t *testing.T) {
	rng := RunRNG(4, 1)
	cx, cy, r := uint64(50), uint64(40), 15.0
	env := circleEnv("c1", cx, cy, r)
	pl := &datamodels.Placement{Type: datamodels.PlacementAtShape, ShapeUID: "c1"}
	const count = 100
	pts, warns := PlaceAgents(rng, count, pl, env)
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %+v", warns)
	}
	if len(pts) != count {
		t.Fatalf("expected %d, got %d", count, len(pts))
	}
	for _, p := range pts {
		if !containsCircle(float64(cx), float64(cy), r, float64(p.X), float64(p.Y)) {
			t.Fatalf("point %+v outside circle center(%d,%d) r=%v", p, cx, cy, r)
		}
	}
}

func polyEnv(uid string, pts []common.Point) *datasims.SimulationEnvironment {
	env := testEnv()
	env.Shapes = []datasims.SimulationShape{{
		ShapeUID: uid,
		Type:     datamodels.ShapeTypePolygon,
		Position: common.Point{X: 0, Y: 0},
		Points:   pts,
	}}
	return env
}

func TestPlaceAgents_AtShapePolygon(t *testing.T) {
	rng := RunRNG(5, 1)
	// A diamond polygon (anchor-relative points, anchor at 0,0).
	poly := []common.Point{{X: 50, Y: 20}, {X: 70, Y: 40}, {X: 50, Y: 60}, {X: 30, Y: 40}}
	env := polyEnv("p1", poly)
	pl := &datamodels.Placement{Type: datamodels.PlacementAtShape, ShapeUID: "p1"}
	const count = 80
	pts, _ := PlaceAgents(rng, count, pl, env)
	if len(pts) != count {
		t.Fatalf("expected %d, got %d", count, len(pts))
	}
	for _, p := range pts {
		if !pointInPolygon(poly, float64(p.X), float64(p.Y)) {
			t.Fatalf("point %+v outside polygon", p)
		}
	}
}

func TestPlaceAgents_AtShapeMissingFallsBackRandom(t *testing.T) {
	rng := RunRNG(6, 1)
	env := circleEnv("c1", 50, 40, 10)
	pl := &datamodels.Placement{Type: datamodels.PlacementAtShape, ShapeUID: "nope"}
	pts, warns := PlaceAgents(rng, 30, pl, env)
	if len(pts) != 30 {
		t.Fatalf("expected 30, got %d", len(pts))
	}
	if len(warns) == 0 {
		t.Fatal("expected warning for missing shape")
	}
	inBounds(t, pts)
}

// --- determinism ---------------------------------------------------------------

func TestPlaceAgents_Determinism(t *testing.T) {
	types := []datamodels.PlacementType{
		datamodels.PlacementRandom,
		datamodels.PlacementUniform,
		datamodels.PlacementClustered,
		datamodels.PlacementManual,
	}
	clusters4 := uint64(4)
	radius6 := uint64(6)
	for _, ty := range types {
		pl := &datamodels.Placement{Type: ty, Clusters: &clusters4, ClusterRadius: &radius6}
		a, _ := PlaceAgents(RunRNG(99, 1), 60, pl, testEnv())
		b, _ := PlaceAgents(RunRNG(99, 1), 60, pl, testEnv())
		if !samePoints(a, b) {
			t.Fatalf("type %s: placement not deterministic", ty)
		}
	}
	// at_shape determinism
	env := circleEnv("c1", 50, 40, 15)
	pl := &datamodels.Placement{Type: datamodels.PlacementAtShape, ShapeUID: "c1"}
	a, _ := PlaceAgents(RunRNG(99, 1), 60, pl, env)
	b, _ := PlaceAgents(RunRNG(99, 1), 60, pl, env)
	if !samePoints(a, b) {
		t.Fatal("at_shape placement not deterministic")
	}
}

// --- SampleDirectives ----------------------------------------------------------

func TestSampleDirectives_FractionAllocation(t *testing.T) {
	rng := RunRNG(20, 1)
	dirs := []PlacementDirective{
		{Kind: "cluster", Fraction: 0.6, Center: &common.Point{X: 25, Y: 20}, Radius: 4},
		{Kind: "cluster", Fraction: 0.4, Center: &common.Point{X: 75, Y: 60}, Radius: 4},
	}
	const count = 100
	pts := SampleDirectives(rng, count, dirs, testEnv())
	if len(pts) != count {
		t.Fatalf("expected total %d, got %d", count, len(pts))
	}
	inBounds(t, pts)
	// Roughly 60 near the first center, 40 near the second.
	near1 := 0
	for _, p := range pts {
		d := math.Hypot(float64(p.X)-25, float64(p.Y)-20)
		if d < 20 {
			near1++
		}
	}
	if near1 < 50 || near1 > 70 {
		t.Fatalf("expected ~60 near center1, got %d", near1)
	}
}

func TestSampleDirectives_EqualSplitWhenZeroFractions(t *testing.T) {
	rng := RunRNG(21, 1)
	dirs := []PlacementDirective{
		{Kind: "cluster", Center: &common.Point{X: 25, Y: 20}, Radius: 3},
		{Kind: "cluster", Center: &common.Point{X: 75, Y: 60}, Radius: 3},
	}
	pts := SampleDirectives(rng, 100, dirs, testEnv())
	if len(pts) != 100 {
		t.Fatalf("expected 100, got %d", len(pts))
	}
}

func TestSampleDirectives_ExplicitFirst(t *testing.T) {
	rng := RunRNG(22, 1)
	explicit := []common.Point{{X: 10, Y: 10}, {X: 20, Y: 20}, {X: 30, Y: 30}}
	dirs := []PlacementDirective{
		{Kind: "cluster", Fraction: 1.0, Center: &common.Point{X: 90, Y: 70}, Radius: 2},
		{Kind: "explicit", Points: explicit},
	}
	const count = 10
	pts := SampleDirectives(rng, count, dirs, testEnv())
	if len(pts) != count {
		t.Fatalf("expected %d, got %d", count, len(pts))
	}
	// The first three points must be the explicit ones in order.
	for i, want := range explicit {
		if pts[i] != want {
			t.Fatalf("explicit point %d: want %+v got %+v", i, want, pts[i])
		}
	}
}

func TestSampleDirectives_RegionInsidePolygon(t *testing.T) {
	rng := RunRNG(23, 1)
	poly := []common.Point{{X: 40, Y: 20}, {X: 80, Y: 30}, {X: 70, Y: 60}, {X: 35, Y: 55}}
	dirs := []PlacementDirective{
		{Kind: "region", Fraction: 1.0, Polygon: poly},
	}
	const count = 100
	pts := SampleDirectives(rng, count, dirs, testEnv())
	if len(pts) != count {
		t.Fatalf("expected %d, got %d", count, len(pts))
	}
	for _, p := range pts {
		if !pointInPolygon(poly, float64(p.X), float64(p.Y)) {
			t.Fatalf("region point %+v outside polygon", p)
		}
	}
}

func TestSampleDirectives_Empty(t *testing.T) {
	rng := RunRNG(24, 1)
	if pts := SampleDirectives(rng, 0, nil, testEnv()); len(pts) != 0 {
		t.Fatalf("expected empty for zero count, got %d", len(pts))
	}
	if pts := SampleDirectives(rng, 50, nil, testEnv()); len(pts) != 0 {
		t.Fatalf("expected empty for nil dirs, got %d", len(pts))
	}
}

func TestSampleDirectives_Determinism(t *testing.T) {
	dirs := []PlacementDirective{
		{Kind: "cluster", Fraction: 0.5, Center: &common.Point{X: 25, Y: 20}, Radius: 4},
		{Kind: "region", Fraction: 0.5, Polygon: []common.Point{{X: 40, Y: 20}, {X: 80, Y: 30}, {X: 70, Y: 60}, {X: 35, Y: 55}}},
	}
	a := SampleDirectives(RunRNG(50, 1), 80, dirs, testEnv())
	b := SampleDirectives(RunRNG(50, 1), 80, dirs, testEnv())
	if !samePoints(a, b) {
		t.Fatal("SampleDirectives not deterministic")
	}
}

// --- geometry helpers ----------------------------------------------------------

func TestPointInPolygon(t *testing.T) {
	square := []common.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}}
	if !pointInPolygon(square, 5, 5) {
		t.Fatal("center should be inside square")
	}
	if pointInPolygon(square, 20, 20) {
		t.Fatal("far point should be outside square")
	}
}

func TestContainsRect(t *testing.T) {
	// top-left (10,10), 20x30
	if !containsRect(10, 10, 20, 30, 15, 25) {
		t.Fatal("interior point should be inside rect")
	}
	if containsRect(10, 10, 20, 30, 5, 5) {
		t.Fatal("exterior point should be outside rect")
	}
}

// at_shape sampling must return positions INSIDE the shape's integer cell
// range after rounding: a float sample near the upper edge used to round one
// cell PAST the rectangle — into whatever lies beyond (walls, doors). Found
// live: 6/100 agents placed at x=39/y=39 by an interior rect (1,1) 38×38.
func TestSampleInShape_RectangleStaysInsideAfterRounding(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	s := datasims.SimulationShape{
		Type:     datamodels.ShapeTypeRectangle,
		Position: common.Point{X: 1, Y: 1},
		Width:    cu64(38),
		Height:   cu64(38),
	}
	env := &datasims.SimulationEnvironment{Width: 40, Height: 40}
	for i := 0; i < 5000; i++ {
		p := sampleInShape(rng, s, env)
		if p.X < 1 || p.X > 38 || p.Y < 1 || p.Y > 38 {
			t.Fatalf("sample %d outside the rect's cells: (%d,%d)", i, p.X, p.Y)
		}
	}
}
