package simulations

import (
	"math"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

func cf64(v float64) *float64 { return &v }
func cu64(v uint64) *uint64   { return &v }

func TestOverlap_CircleCircle(t *testing.T) {
	a := circleBody(Vec2{0, 0}, 5)
	cases := []struct {
		name string
		b    CollisionBody
		want bool
	}{
		{"overlap", circleBody(Vec2{6, 0}, 5), true},
		{"touch", circleBody(Vec2{10, 0}, 5), true},
		{"separate", circleBody(Vec2{11, 0}, 5), false},
	}
	for _, c := range cases {
		if got := Overlap(a, c.b, 1000, 1000, false); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestOverlap_CircleRect(t *testing.T) {
	rect := rectBodyCentered(Vec2{50, 50}, 10, 10) // [40,60]x[40,60]
	cases := []struct {
		name string
		c    CollisionBody
		want bool
	}{
		{"inside", circleBody(Vec2{50, 50}, 2), true},
		{"edge-touch", circleBody(Vec2{65, 50}, 5), true},
		{"corner-miss", circleBody(Vec2{67, 67}, 5), false},
		{"corner-hit", circleBody(Vec2{63, 63}, 5), true},
		{"far", circleBody(Vec2{100, 100}, 5), false},
	}
	for _, c := range cases {
		if got := Overlap(c.c, rect, 1000, 1000, false); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestOverlap_RectRect(t *testing.T) {
	a := rectBodyCentered(Vec2{0, 0}, 10, 10)
	cases := []struct {
		name string
		b    CollisionBody
		want bool
	}{
		{"overlap", rectBodyCentered(Vec2{15, 0}, 10, 10), true},
		{"touch", rectBodyCentered(Vec2{20, 0}, 10, 10), true},
		{"separate", rectBodyCentered(Vec2{21, 0}, 10, 10), false},
	}
	for _, c := range cases {
		if got := Overlap(a, c.b, 1000, 1000, false); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestOverlap_PolyPoly_ConcaveNotch(t *testing.T) {
	// A C-shaped (concave) polygon with a notch on the right side around y=50.
	cShape := polygonBody([]Vec2{
		{0, 0}, {100, 0}, {100, 40}, {40, 40}, {40, 60}, {100, 60}, {100, 100}, {0, 100},
	}, false)
	// A small circle sitting in the notch (right of x=40, between y=40 and 60) must NOT overlap.
	inNotch := circleBody(Vec2{70, 50}, 5)
	if Overlap(inNotch, cShape, 1000, 1000, false) {
		t.Errorf("circle in concave notch must not overlap the C-shape")
	}
	// A circle on the left solid arm must overlap.
	inArm := circleBody(Vec2{20, 50}, 5)
	if !Overlap(inArm, cShape, 1000, 1000, false) {
		t.Errorf("circle on the solid arm must overlap the C-shape")
	}
}

// TestOverlap_PointPreservesShapeContains is the critical regression guard: for a point
// agent, Overlap(pointBody, shapeBody) must equal the legacy ShapeContains(shape, point)
// for clearly-interior / clearly-exterior points.
func TestOverlap_PointPreservesShapeContains(t *testing.T) {
	shapes := map[string]*datasims.SimulationShape{
		"circle":    {Type: models.ShapeTypeCircle, Position: common.Point{X: 50, Y: 50}, Radius: cf64(10)},
		"rectangle": {Type: models.ShapeTypeRectangle, Position: common.Point{X: 40, Y: 40}, Width: cu64(20), Height: cu64(20)},
		"polygon":   {Type: models.ShapeTypePolygon, Position: common.Point{X: 0, Y: 0}, Points: []common.Point{{X: 40, Y: 40}, {X: 60, Y: 40}, {X: 60, Y: 60}, {X: 40, Y: 60}}},
		"line":      {Type: models.ShapeTypeLine, Position: common.Point{X: 0, Y: 0}, Points: []common.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}},
	}
	pts := []common.Point{{X: 50, Y: 50}, {X: 0, Y: 0}, {X: 45, Y: 45}, {X: 55, Y: 55}, {X: 100, Y: 100}, {X: 5, Y: 80}}
	for name, sh := range shapes {
		sb := bodyForShape(sh)
		for _, p := range pts {
			want := ShapeContains(sh, p)
			got := Overlap(pointBody(Vec2{float64(p.X), float64(p.Y)}), sb, 1000, 1000, false)
			if got != want {
				t.Errorf("%s @ (%d,%d): Overlap=%v ShapeContains=%v", name, p.X, p.Y, got, want)
			}
		}
	}
}

func TestOverlap_TorusMinImage(t *testing.T) {
	// World 100x100 torus. Circles near opposite edges should overlap via wrap.
	a := circleBody(Vec2{2, 50}, 5)
	b := circleBody(Vec2{98, 50}, 5)
	if !Overlap(a, b, 100, 100, true) {
		t.Errorf("circles across the torus seam should overlap")
	}
	if Overlap(a, b, 100, 100, false) {
		t.Errorf("same circles on a bounded world should not overlap")
	}
}

func TestBodyDistance_EdgeToEdge(t *testing.T) {
	a := circleBody(Vec2{0, 0}, 5)
	b := circleBody(Vec2{20, 0}, 5)
	if d := BodyDistance(a, b, 1000, 1000, false); math.Abs(d-10) > 1e-6 {
		t.Errorf("circle gap: got %v want 10", d)
	}
	if d := BodyDistance(a, circleBody(Vec2{6, 0}, 5), 1000, 1000, false); d != 0 {
		t.Errorf("overlapping circles must have distance 0, got %v", d)
	}
	// symmetry
	r := rectBodyCentered(Vec2{30, 0}, 5, 5) // left edge at x=25
	d1 := BodyDistance(a, r, 1000, 1000, false)
	d2 := BodyDistance(r, a, 1000, 1000, false)
	if math.Abs(d1-d2) > 1e-6 {
		t.Errorf("BodyDistance not symmetric: %v vs %v", d1, d2)
	}
	if math.Abs(d1-20) > 1e-6 { // circle right edge at x=5, rect left edge at x=25 → gap 20
		t.Errorf("circle-rect gap: got %v want 20", d1)
	}
}

func TestConvexPieces_Decomposition(t *testing.T) {
	convex := []Vec2{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	if got := convexPieces(convex); len(got) != 1 {
		t.Errorf("convex polygon should be a single piece, got %d", len(got))
	}
	// Concave arrow / chevron.
	concave := []Vec2{{0, 0}, {10, 5}, {0, 10}, {3, 5}}
	pieces := convexPieces(concave)
	if len(pieces) < 2 {
		t.Errorf("concave polygon should decompose into >=2 pieces, got %d", len(pieces))
	}
	for i, pc := range pieces {
		if !isConvex(pc) {
			t.Errorf("piece %d is not convex", i)
		}
	}
}

func TestBodyForShape_Conventions(t *testing.T) {
	// Rectangle anchors at top-left; body must be centered.
	rsh := &datasims.SimulationShape{Type: models.ShapeTypeRectangle, Position: common.Point{X: 10, Y: 20}, Width: cu64(40), Height: cu64(60)}
	b := bodyForShape(rsh)
	if b.Center.X != 30 || b.Center.Y != 50 {
		t.Errorf("rect center: got (%v,%v) want (30,50)", b.Center.X, b.Center.Y)
	}
	if b.HalfW != 20 || b.HalfH != 30 {
		t.Errorf("rect half-extents: got (%v,%v) want (20,30)", b.HalfW, b.HalfH)
	}
	// Line is an open polyline (no interior).
	lsh := &datasims.SimulationShape{Type: models.ShapeTypeLine, Position: common.Point{X: 0, Y: 0}, Points: []common.Point{{X: 0, Y: 0}, {X: 50, Y: 0}}}
	if lb := bodyForShape(lsh); !lb.Open {
		t.Errorf("line shape must produce an open body")
	}
}

func TestWithinBody_MultiCellFootprint(t *testing.T) {
	idx := NewSpatialIndex(100, 100, 10, false)
	idx.Insert(1, common.Point{X: 10, Y: 10})
	idx.Insert(2, common.Point{X: 50, Y: 10}) // large circle centered here
	bodies := map[uint64]CollisionBody{
		1: pointBody(Vec2{10, 10}),
		2: circleBody(Vec2{50, 10}, 20), // edge reaches x=30 → 20 from the point at x=10
	}
	idx.maxBoundingRadius = 20
	resolve := func(id uint64) CollisionBody { return bodies[id] }

	has := func(ids []uint64, want uint64) bool {
		for _, id := range ids {
			if id == want {
				return true
			}
		}
		return false
	}
	// Center distance is 40; edge-to-edge is 20. r=25 must find the big circle.
	if got := idx.WithinBody(bodies[1], 25, resolve); !has(got, 2) {
		t.Errorf("WithinBody(r=25) should find the multi-cell footprint, got %v", got)
	}
	// r=15 is inside the edge gap (20) → not found.
	if got := idx.WithinBody(bodies[1], 15, resolve); has(got, 2) {
		t.Errorf("WithinBody(r=15) must not find the footprint (edge gap 20), got %v", got)
	}
}

func TestRebuild_TracksMaxBoundingRadius(t *testing.T) {
	idx := NewSpatialIndex(100, 100, 10, false)
	agents := []*RuntimeAgent{
		{A: &datasims.SimulationAgent{ID: 1, Position: &common.Point{X: 5, Y: 5}}, Archetype: &models.Agent{}},
		{A: &datasims.SimulationAgent{ID: 2, Position: &common.Point{X: 50, Y: 50}}, Archetype: &models.Agent{Footprint: &models.AgentFootprint{Kind: models.FootprintCircle, Radius: cf64(12)}}},
	}
	idx.Rebuild(agents)
	if idx.maxBoundingRadius != 12 {
		t.Errorf("maxBoundingRadius: got %v want 12", idx.maxBoundingRadius)
	}
}

func TestIntegrateMovementBody(t *testing.T) {
	point := pointBody(Vec2{0, 0})
	// Point body must equal legacy IntegrateMovement exactly (bounded + torus).
	for _, torus := range []bool{false, true} {
		start := common.Point{X: 98, Y: 40}
		v := Vector{DX: 5, DY: 0}
		legacy := IntegrateMovement(start, v, 100, 80, torus)
		got := IntegrateMovementBody(start, v, point, 100, 80, torus)
		if got != legacy {
			t.Errorf("torus=%v: point body %v != legacy %v", torus, got, legacy)
		}
	}
	// A circle(r=5) moving toward the right wall on a 100-wide bounded world stops with its
	// whole footprint inside: center clamped to x=95.
	circle := circleBody(Vec2{0, 0}, 5)
	got := IntegrateMovementBody(common.Point{X: 90, Y: 50}, Vector{DX: 20, DY: 0}, circle, 100, 100, false)
	if got.X != 95 {
		t.Errorf("extent clamp: got x=%d want 95", got.X)
	}
	// Same on a torus wraps the center (no extent clamp).
	gotT := IntegrateMovementBody(common.Point{X: 98, Y: 50}, Vector{DX: 5, DY: 0}, circle, 100, 100, true)
	if gotT.X != 3 {
		t.Errorf("torus wrap: got x=%d want 3", gotT.X)
	}
}

func TestBodyForAgent_FootprintCentered(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{Position: &common.Point{X: 100, Y: 100}},
		Archetype: &models.Agent{Footprint: &models.AgentFootprint{Kind: models.FootprintRectangle, Width: cu64(20), Height: cu64(20)}},
	}
	b := bodyForAgent(ra)
	if b.Center.X != 100 || b.Center.Y != 100 {
		t.Errorf("agent rect must center on position, got (%v,%v)", b.Center.X, b.Center.Y)
	}
	if b.HalfW != 10 || b.HalfH != 10 {
		t.Errorf("agent rect half-extents: got (%v,%v) want (10,10)", b.HalfW, b.HalfH)
	}
	// nil footprint -> point
	rp := &RuntimeAgent{A: &datasims.SimulationAgent{Position: &common.Point{X: 5, Y: 5}}, Archetype: &models.Agent{}}
	if pb := bodyForAgent(rp); pb.Kind != BodyPoint || pb.BoundR != 0 {
		t.Errorf("nil footprint must yield a zero-extent point body")
	}
}
