package simulations

import (
	"math"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// collisionEps is the tolerance for "touching counts as overlapping" comparisons.
const collisionEps = 1e-9

// Vec2 is a float planar vector used by the exact-geometry collision layer.
type Vec2 struct{ X, Y float64 }

// BodyKind is the resolved collision-body kind for an agent or shape.
type BodyKind uint8

const (
	// BodyPoint is a dimensionless point (a circle of radius 0). Agents with a nil or
	// "point" footprint resolve to this — exact legacy behavior.
	BodyPoint BodyKind = iota
	BodyCircle
	BodyRect
	BodyPolygon
)

// CollisionBody is the unified, resolved footprint the exact-overlap math operates on.
// It is built on demand from a SimulationAgent (position + archetype footprint) or a
// SimulationShape (position + geometry) via bodyForAgent / bodyForShape. Rectangles are
// stored center+half-extents (the adapters convert the shape top-left and agent-centered
// conventions). Polygons store absolute CCW vertices plus a convex decomposition (Pieces)
// for SAT. Open marks a polyline (a line shape) that has no interior.
type CollisionBody struct {
	Kind   BodyKind
	Center Vec2
	Radius float64 // circle
	HalfW  float64 // rect half-extent X
	HalfH  float64 // rect half-extent Y
	Verts  []Vec2  // polygon / rect absolute vertices (CCW), closed unless Open
	Pieces [][]Vec2

	AABBHalf Vec2    // AABB half-extents about Center
	BoundR   float64 // bounding-circle radius about Center (index over-fetch + broad phase)
	Open     bool    // polyline (line shape): no interior — never overlaps/contains
}

// ---- adapters -----------------------------------------------------------------

// bodyForAgent resolves the agent's collision body, centered on its position. A nil
// position, nil archetype, or nil/"point" footprint yields a BodyPoint.
func bodyForAgent(ra *RuntimeAgent) CollisionBody {
	if ra == nil || ra.A == nil || ra.A.Position == nil {
		return CollisionBody{Kind: BodyPoint}
	}
	return bodyForAgentAt(ra, Vec2{X: float64(ra.A.Position.X), Y: float64(ra.A.Position.Y)})
}

// bodyForAgentAt builds the agent's footprint body at an arbitrary center — used by
// the flow gate (U10) to evaluate the pre-phase position without touching
// ra.A.Position. bodyForAgent delegates here, so both always agree.
func bodyForAgentAt(ra *RuntimeAgent, center Vec2) CollisionBody {
	var fp *models.AgentFootprint
	if ra != nil && ra.Archetype != nil {
		fp = ra.Archetype.Footprint
	}
	if fp == nil || fp.Kind == models.FootprintPoint || fp.Kind == "" {
		return pointBody(center)
	}
	switch fp.Kind {
	case models.FootprintCircle:
		if fp.Radius == nil || *fp.Radius <= 0 {
			return pointBody(center)
		}
		return circleBody(center, *fp.Radius)
	case models.FootprintRectangle:
		if fp.Width == nil || fp.Height == nil || *fp.Width == 0 || *fp.Height == 0 {
			return pointBody(center)
		}
		return rectBodyCentered(center, float64(*fp.Width)/2, float64(*fp.Height)/2)
	case models.FootprintPolygon:
		if len(fp.Points) < 3 {
			return pointBody(center)
		}
		verts := make([]Vec2, len(fp.Points))
		for i, p := range fp.Points {
			// Footprint polygon points are anchor-relative (centered on the position).
			verts[i] = Vec2{X: center.X + float64(p.X), Y: center.Y + float64(p.Y)}
		}
		return polygonBody(verts, false)
	default:
		return pointBody(center)
	}
}

// bodyForShape resolves a shape instance's collision body. Lines become open polylines
// (no interior — preserving ShapeContains semantics); degenerate shapes become points.
func bodyForShape(sh *datasims.SimulationShape) CollisionBody {
	if sh == nil {
		return CollisionBody{Kind: BodyPoint}
	}
	anchor := Vec2{X: float64(sh.Position.X), Y: float64(sh.Position.Y)}
	switch sh.Type {
	case models.ShapeTypeCircle:
		if sh.Radius == nil || *sh.Radius <= 0 {
			return pointBody(anchor)
		}
		return circleBody(anchor, *sh.Radius)
	case models.ShapeTypeRectangle:
		if sh.Width == nil || sh.Height == nil || *sh.Width == 0 || *sh.Height == 0 {
			return pointBody(anchor)
		}
		// Shape rectangles anchor at the top-left; recenter.
		hw, hh := float64(*sh.Width)/2, float64(*sh.Height)/2
		return rectBodyCentered(Vec2{X: anchor.X + hw, Y: anchor.Y + hh}, hw, hh)
	case models.ShapeTypePolygon:
		if len(sh.Points) < 3 {
			return pointBody(anchor)
		}
		verts := absVerts(anchor, sh.Points)
		return polygonBody(verts, false)
	case models.ShapeTypeLine:
		if len(sh.Points) < 2 {
			return pointBody(anchor)
		}
		return polygonBody(absVerts(anchor, sh.Points), true) // open polyline: no interior
	default:
		return pointBody(anchor)
	}
}

func absVerts(anchor Vec2, pts []common.Point) []Vec2 {
	out := make([]Vec2, len(pts))
	for i, p := range pts {
		out[i] = Vec2{X: anchor.X + float64(p.X), Y: anchor.Y + float64(p.Y)}
	}
	return out
}

func pointBody(c Vec2) CollisionBody {
	return CollisionBody{Kind: BodyPoint, Center: c}
}

func circleBody(c Vec2, r float64) CollisionBody {
	return CollisionBody{Kind: BodyCircle, Center: c, Radius: r, AABBHalf: Vec2{r, r}, BoundR: r}
}

func rectBodyCentered(c Vec2, hw, hh float64) CollisionBody {
	verts := []Vec2{
		{c.X - hw, c.Y - hh}, {c.X + hw, c.Y - hh},
		{c.X + hw, c.Y + hh}, {c.X - hw, c.Y + hh},
	}
	return CollisionBody{
		Kind: BodyRect, Center: c, HalfW: hw, HalfH: hh,
		Verts: verts, Pieces: [][]Vec2{verts},
		AABBHalf: Vec2{hw, hh}, BoundR: math.Hypot(hw, hh),
	}
}

func polygonBody(verts []Vec2, open bool) CollisionBody {
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, v := range verts {
		minX, maxX = math.Min(minX, v.X), math.Max(maxX, v.X)
		minY, maxY = math.Min(minY, v.Y), math.Max(maxY, v.Y)
	}
	// Center the body on its AABB center so torus min-image + index keying are stable.
	center := Vec2{X: (minX + maxX) / 2, Y: (minY + maxY) / 2}
	var boundR float64
	for _, v := range verts {
		boundR = math.Max(boundR, dist(center, v))
	}
	b := CollisionBody{
		Kind:     BodyPolygon,
		Center:   center,
		Verts:    verts,
		AABBHalf: Vec2{(maxX - minX) / 2, (maxY - minY) / 2},
		BoundR:   boundR,
		Open:     open,
	}
	if !open {
		b.Pieces = convexPieces(verts)
	}
	return b
}

// ---- torus -------------------------------------------------------------------

// minImageShift returns the translation to add to b so b is the nearest wrapped image of
// a on a torus world (no-op when torus is false). This isolates wrap-around to a single
// step so every flat-plane overlap/distance test runs on the aligned copy.
func minImageShift(a, b Vec2, w, h float64, torus bool) Vec2 {
	if !torus {
		return Vec2{}
	}
	return Vec2{
		X: axisDelta(a.X, b.X, w, true) - (b.X - a.X),
		Y: axisDelta(a.Y, b.Y, h, true) - (b.Y - a.Y),
	}
}

func translateBody(b CollisionBody, d Vec2) CollisionBody {
	if d.X == 0 && d.Y == 0 {
		return b
	}
	out := b
	out.Center = Vec2{b.Center.X + d.X, b.Center.Y + d.Y}
	if b.Verts != nil {
		out.Verts = translatePts(b.Verts, d)
	}
	if b.Pieces != nil {
		out.Pieces = make([][]Vec2, len(b.Pieces))
		for i, pc := range b.Pieces {
			out.Pieces[i] = translatePts(pc, d)
		}
	}
	return out
}

func translatePts(pts []Vec2, d Vec2) []Vec2 {
	out := make([]Vec2, len(pts))
	for i, p := range pts {
		out[i] = Vec2{p.X + d.X, p.Y + d.Y}
	}
	return out
}

// ---- overlap / distance ------------------------------------------------------

// Overlap reports whether two collision bodies overlap (touching counts), torus-aware.
// An open polyline (line shape) has no interior and never overlaps.
func Overlap(a, b CollisionBody, w, h uint64, torus bool) bool {
	b = translateBody(b, minImageShift(a.Center, b.Center, float64(w), float64(h), torus))
	if dist(a.Center, b.Center) > a.BoundR+b.BoundR+collisionEps {
		return false
	}
	if a.Open || b.Open {
		return false
	}
	ac, ar, aCirc := circleOf(a)
	bc, br, bCirc := circleOf(b)
	switch {
	case aCirc && bCirc:
		return distSq(ac, bc) <= (ar+br)*(ar+br)+collisionEps
	case aCirc:
		return circlePolyOverlap(ac, ar, b)
	case bCirc:
		return circlePolyOverlap(bc, br, a)
	default:
		return polyPolyOverlap(a, b)
	}
}

// BodyDistance returns the edge-to-edge distance between two bodies (0 when they overlap),
// torus-aware. For an open polyline it is the distance to the nearest segment.
func BodyDistance(a, b CollisionBody, w, h uint64, torus bool) float64 {
	b = translateBody(b, minImageShift(a.Center, b.Center, float64(w), float64(h), torus))
	if !a.Open && !b.Open && Overlap(localBody(a), localBody(b), 0, 0, false) {
		return 0
	}
	ac, ar, aCirc := circleOf(a)
	bc, br, bCirc := circleOf(b)
	switch {
	case aCirc && bCirc:
		return math.Max(0, dist(ac, bc)-ar-br)
	case aCirc:
		return math.Max(0, polyEdgeDistance(ac, b)-ar)
	case bCirc:
		return math.Max(0, polyEdgeDistance(bc, a)-br)
	default:
		return polyPolyDistance(a, b)
	}
}

// localBody strips the world (used to re-test overlap after b is already min-image aligned).
func localBody(b CollisionBody) CollisionBody { return b }

// BodyOverlapsShape reports whether an agent's body overlaps a shape (touching counts).
// It replaces ShapeContains at collision call sites; for a point agent and a closed shape
// it equals ShapeContains (boundary-inclusive). Line shapes (open) never overlap.
func BodyOverlapsShape(ra *RuntimeAgent, sh *datasims.SimulationShape, w, h uint64, torus bool) bool {
	return Overlap(bodyForAgent(ra), bodyForShape(sh), w, h, torus)
}

// circleOf returns (center, radius, true) for point/circle bodies; ok=false otherwise.
func circleOf(b CollisionBody) (Vec2, float64, bool) {
	switch b.Kind {
	case BodyPoint:
		return b.Center, 0, true
	case BodyCircle:
		return b.Center, b.Radius, true
	}
	return Vec2{}, 0, false
}

// circlePolyOverlap reports whether a circle (center c, radius r) overlaps the closed
// polygon/rect body p: the center is inside, or some edge is within r.
func circlePolyOverlap(c Vec2, r float64, p CollisionBody) bool {
	if pointInPoly(c, p.Verts) {
		return true
	}
	return polyEdgeDistance(c, p) <= r+collisionEps
}

// polyPolyOverlap runs SAT over the convex pieces of two closed bodies (rect = one piece).
func polyPolyOverlap(a, b CollisionBody) bool {
	for _, pa := range pieces(a) {
		for _, pb := range pieces(b) {
			if satOverlap(pa, pb) {
				return true
			}
		}
	}
	return false
}

func pieces(b CollisionBody) [][]Vec2 {
	if len(b.Pieces) > 0 {
		return b.Pieces
	}
	if len(b.Verts) >= 3 {
		return [][]Vec2{b.Verts}
	}
	return nil
}

// satOverlap reports whether two convex polygons overlap using the separating-axis
// theorem. A strict gap on any axis means separated; touching (equal projections) counts
// as overlapping.
func satOverlap(a, b []Vec2) bool {
	for _, poly := range [2][]Vec2{a, b} {
		n := len(poly)
		for i := 0; i < n; i++ {
			p1, p2 := poly[i], poly[(i+1)%n]
			axis := Vec2{X: -(p2.Y - p1.Y), Y: p2.X - p1.X}
			if axis.X == 0 && axis.Y == 0 {
				continue
			}
			minA, maxA := projectRange(a, axis)
			minB, maxB := projectRange(b, axis)
			if maxA < minB-collisionEps || maxB < minA-collisionEps {
				return false
			}
		}
	}
	return true
}

func projectRange(poly []Vec2, axis Vec2) (float64, float64) {
	mn, mx := math.MaxFloat64, -math.MaxFloat64
	for _, p := range poly {
		d := p.X*axis.X + p.Y*axis.Y
		mn, mx = math.Min(mn, d), math.Max(mx, d)
	}
	return mn, mx
}

// polyEdgeDistance returns the distance from point c to the body's nearest edge (0 if the
// body is degenerate). For open polylines the segments are the consecutive vertex pairs;
// for closed bodies the final closing edge is included.
func polyEdgeDistance(c Vec2, b CollisionBody) float64 {
	n := len(b.Verts)
	if n == 0 {
		return dist(c, b.Center)
	}
	if n == 1 {
		return dist(c, b.Verts[0])
	}
	best := math.MaxFloat64
	last := n - 1
	if b.Open {
		last = n - 2
	}
	for i := 0; i <= last; i++ {
		seg := b.Verts[(i+1)%n]
		best = math.Min(best, pointSegDistance(c, b.Verts[i], seg))
	}
	return best
}

// polyPolyDistance returns the min edge-to-edge distance between two closed/open bodies.
func polyPolyDistance(a, b CollisionBody) float64 {
	best := math.MaxFloat64
	for _, va := range a.Verts {
		best = math.Min(best, polyEdgeDistance(va, b))
	}
	for _, vb := range b.Verts {
		best = math.Min(best, polyEdgeDistance(vb, a))
	}
	return best
}

// ---- geometry primitives -----------------------------------------------------

func sub(a, b Vec2) Vec2      { return Vec2{a.X - b.X, a.Y - b.Y} }
func cross(a, b Vec2) float64 { return a.X*b.Y - a.Y*b.X }
func distSq(a, b Vec2) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx + dy*dy
}
func dist(a, b Vec2) float64 { return math.Sqrt(distSq(a, b)) }

// pointInPoly reports whether point p is inside the (closed) polygon via ray casting.
func pointInPoly(p Vec2, poly []Vec2) bool {
	n := len(poly)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[i].X, poly[i].Y
		xj, yj := poly[j].X, poly[j].Y
		if (yi > p.Y) != (yj > p.Y) {
			xint := (xj-xi)*(p.Y-yi)/(yj-yi) + xi
			if p.X < xint {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

// pointSegDistance returns the distance from p to segment ab.
func pointSegDistance(p, a, b Vec2) float64 {
	ab := sub(b, a)
	denom := ab.X*ab.X + ab.Y*ab.Y
	if denom == 0 {
		return dist(p, a)
	}
	t := ((p.X-a.X)*ab.X + (p.Y-a.Y)*ab.Y) / denom
	t = math.Max(0, math.Min(1, t))
	proj := Vec2{a.X + t*ab.X, a.Y + t*ab.Y}
	return dist(p, proj)
}

// signedArea returns twice the signed area of the polygon (positive ⇒ CCW).
func signedArea(poly []Vec2) float64 {
	var s float64
	n := len(poly)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		s += poly[i].X*poly[j].Y - poly[j].X*poly[i].Y
	}
	return s
}

func isConvex(poly []Vec2) bool {
	n := len(poly)
	if n < 4 {
		return true
	}
	var sign float64
	for i := 0; i < n; i++ {
		a := poly[i]
		b := poly[(i+1)%n]
		c := poly[(i+2)%n]
		z := cross(sub(b, a), sub(c, b))
		if math.Abs(z) < collisionEps {
			continue
		}
		if sign == 0 {
			sign = z
		} else if (z > 0) != (sign > 0) {
			return false
		}
	}
	return true
}

// convexPieces decomposes a (possibly concave) polygon into convex pieces. A convex
// polygon returns itself; otherwise it is ear-clipped into triangles. The decomposition
// is deterministic (fixed index-order traversal, CCW-normalized).
func convexPieces(verts []Vec2) [][]Vec2 {
	if len(verts) < 3 {
		return nil
	}
	if isConvex(verts) {
		return [][]Vec2{verts}
	}
	poly := append([]Vec2(nil), verts...)
	if signedArea(poly) < 0 {
		// normalize to CCW
		for i, j := 0, len(poly)-1; i < j; i, j = i+1, j-1 {
			poly[i], poly[j] = poly[j], poly[i]
		}
	}
	idx := make([]int, len(poly))
	for i := range idx {
		idx[i] = i
	}
	var tris [][]Vec2
	guard := 0
	for len(idx) > 3 && guard < 10000 {
		guard++
		m := len(idx)
		ear := false
		for i := 0; i < m; i++ {
			ip := idx[(i-1+m)%m]
			ic := idx[i]
			in := idx[(i+1)%m]
			a, b, c := poly[ip], poly[ic], poly[in]
			if cross(sub(b, a), sub(c, b)) <= collisionEps {
				continue // reflex or degenerate
			}
			clear := true
			for _, vi := range idx {
				if vi == ip || vi == ic || vi == in {
					continue
				}
				if pointInTriangle(poly[vi], a, b, c) {
					clear = false
					break
				}
			}
			if !clear {
				continue
			}
			tris = append(tris, []Vec2{a, b, c})
			idx = append(idx[:i], idx[i+1:]...)
			ear = true
			break
		}
		if !ear {
			break // degenerate polygon; bail with what we have
		}
	}
	if len(idx) == 3 {
		tris = append(tris, []Vec2{poly[idx[0]], poly[idx[1]], poly[idx[2]]})
	}
	if len(tris) == 0 {
		return [][]Vec2{verts} // fallback: treat as single piece
	}
	return tris
}

func pointInTriangle(p, a, b, c Vec2) bool {
	d1 := cross(sub(b, a), sub(p, a))
	d2 := cross(sub(c, b), sub(p, b))
	d3 := cross(sub(a, c), sub(p, c))
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}
