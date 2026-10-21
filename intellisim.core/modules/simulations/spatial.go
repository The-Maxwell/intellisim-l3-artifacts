package simulations

import (
	"math"
	"sort"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// Vector is a transient movement vector (engine-only; not persisted unless the
// model declares a velocity/heading property).
type Vector struct{ DX, DY float64 }

// cellKey addresses one bucket in the SpatialIndex grid.
type cellKey struct{ cx, cy int64 }

// SpatialIndex is a uniform-grid spatial index over agent positions, supporting
// radius and k-nearest queries with optional torus (wrap-around) topology.
type SpatialIndex struct {
	w, h, cell uint64
	torus      bool
	cells      map[cellKey][]uint64
	pos        map[uint64]common.Point
	// maxBoundingRadius is the largest agent footprint bounding radius currently indexed.
	// Body queries (WithinBody) over-fetch candidate cells by this much so a far agent
	// whose center sits outside the ideal ring but whose edge reaches into it is not missed.
	// Refreshed each step in Rebuild; an upper bound is always safe.
	maxBoundingRadius float64
}

// NewSpatialIndex builds an empty index sized to a w×h world with the given cell
// size (cell 0 is treated as 1 to avoid div-by-zero) and torus flag.
func NewSpatialIndex(w, h, cell uint64, torus bool) *SpatialIndex {
	if cell == 0 {
		cell = 1
	}
	return &SpatialIndex{
		w:     w,
		h:     h,
		cell:  cell,
		torus: torus,
		cells: make(map[cellKey][]uint64),
		pos:   make(map[uint64]common.Point),
	}
}

// cellKeyFor returns the grid cell holding point p.
func (s *SpatialIndex) cellKeyFor(p common.Point) cellKey {
	return cellKey{
		cx: int64(p.X / s.cell),
		cy: int64(p.Y / s.cell),
	}
}

// Insert records an agent id at position p.
func (s *SpatialIndex) Insert(id uint64, p common.Point) {
	s.pos[id] = p
	key := s.cellKeyFor(p)
	s.cells[key] = append(s.cells[key], id)
}

// Remove drops an agent id from the index.
func (s *SpatialIndex) Remove(id uint64) {
	p, ok := s.pos[id]
	if !ok {
		return
	}
	key := s.cellKeyFor(p)
	bucket := s.cells[key]
	for i, v := range bucket {
		if v == id {
			s.cells[key] = append(bucket[:i], bucket[i+1:]...)
			break
		}
	}
	if len(s.cells[key]) == 0 {
		delete(s.cells, key)
	}
	delete(s.pos, id)
}

// Move relocates an agent id to position p (Remove + Insert).
func (s *SpatialIndex) Move(id uint64, p common.Point) {
	s.Remove(id)
	s.Insert(id, p)
}

// Rebuild clears the index and re-inserts every positioned agent, refreshing
// maxBoundingRadius from each agent's footprint so body queries over-fetch correctly.
func (s *SpatialIndex) Rebuild(agents []*RuntimeAgent) {
	s.cells = make(map[cellKey][]uint64, len(agents)/4+1)
	s.pos = make(map[uint64]common.Point, len(agents))
	s.maxBoundingRadius = 0
	for _, ra := range agents {
		if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil {
			continue
		}
		s.Insert(ra.A.ID, *ra.A.Position)
		if br := bodyForAgent(ra).BoundR; br > s.maxBoundingRadius {
			s.maxBoundingRadius = br
		}
	}
}

// WithinBody returns the ids whose footprint is within edge-to-edge distance r of the query
// body q (excluding nothing — callers filter self), torus-aware. It over-fetches candidate
// cells by q.BoundR + maxBoundingRadius then exact-filters with BodyDistance. The body
// callback resolves a candidate id to its CollisionBody.
func (s *SpatialIndex) WithinBody(q CollisionBody, r float64, body func(uint64) CollisionBody) []uint64 {
	center := common.Point{X: roundNonNeg(q.Center.X), Y: roundNonNeg(q.Center.Y)}
	reach := r + q.BoundR + s.maxBoundingRadius
	var out []uint64
	for _, id := range s.candidateIDs(center, reach) {
		if BodyDistance(q, body(id), s.w, s.h, s.torus) <= r {
			out = append(out, id)
		}
	}
	return out
}

func roundNonNeg(v float64) uint64 {
	if v <= 0 {
		return 0
	}
	return uint64(math.Round(v))
}

// gridDims returns the number of cells along each axis.
func (s *SpatialIndex) gridDims() (int64, int64) {
	cellsX := int64((s.w + s.cell - 1) / s.cell)
	cellsY := int64((s.h + s.cell - 1) / s.cell)
	if cellsX < 1 {
		cellsX = 1
	}
	if cellsY < 1 {
		cellsY = 1
	}
	return cellsX, cellsY
}

// candidateIDs gathers the ids in the cells covering a ring of radius r around
// center (torus-wrapping cell indices and de-duplicating cells).
func (s *SpatialIndex) candidateIDs(center common.Point, r float64) []uint64 {
	rings := int64(math.Ceil(r / float64(s.cell)))
	if rings < 0 {
		rings = 0
	}
	centerKey := s.cellKeyFor(center)
	cellsX, cellsY := s.gridDims()

	seen := make(map[cellKey]struct{})
	var result []uint64
	for dx := -rings; dx <= rings; dx++ {
		for dy := -rings; dy <= rings; dy++ {
			key := cellKey{cx: centerKey.cx + dx, cy: centerKey.cy + dy}
			if s.torus {
				key.cx = ((key.cx % cellsX) + cellsX) % cellsX
				key.cy = ((key.cy % cellsY) + cellsY) % cellsY
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			if ids, ok := s.cells[key]; ok {
				result = append(result, ids...)
			}
		}
	}
	return result
}

// Within returns the ids whose position is within radius r of center
// (torus-aware exact distance test).
func (s *SpatialIndex) Within(center common.Point, r float64) []uint64 {
	candidates := s.candidateIDs(center, r)
	var result []uint64
	for _, id := range candidates {
		p := s.pos[id]
		if Distance(center, p, s.w, s.h, s.torus) <= r {
			result = append(result, id)
		}
	}
	return result
}

// Nearest returns up to k ids closest to center (torus-aware), nearest first.
func (s *SpatialIndex) Nearest(center common.Point, k int) []uint64 {
	if k <= 0 {
		return nil
	}
	type cand struct {
		id   uint64
		dist float64
	}
	cands := make([]cand, 0, len(s.pos))
	for id, p := range s.pos {
		cands = append(cands, cand{id: id, dist: Distance(center, p, s.w, s.h, s.torus)})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].dist == cands[j].dist {
			return cands[i].id < cands[j].id
		}
		return cands[i].dist < cands[j].dist
	})
	if k > len(cands) {
		k = len(cands)
	}
	out := make([]uint64, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, cands[i].id)
	}
	return out
}

// axisDelta returns the signed shorter-path delta b-a along one axis of length
// dim, accounting for torus wrap-around when torus is true.
func axisDelta(a, b, dim float64, torus bool) float64 {
	d := b - a
	if torus && dim > 0 {
		if d > dim/2 {
			d -= dim
		} else if d < -dim/2 {
			d += dim
		}
	}
	return d
}

// Distance returns the (torus-aware) Euclidean distance between two points.
func Distance(a, b common.Point, w, h uint64, torus bool) float64 {
	dx := axisDelta(float64(a.X), float64(b.X), float64(w), torus)
	dy := axisDelta(float64(a.Y), float64(b.Y), float64(h), torus)
	return math.Sqrt(dx*dx + dy*dy)
}

// IntegrateMovement applies a velocity vector to a point, clamping to bounds
// (non-torus) or wrapping (torus). Coordinates are rounded to the nearest
// integer grid position.
func IntegrateMovement(p common.Point, v Vector, w, h uint64, torus bool) common.Point {
	x := float64(p.X) + v.DX
	y := float64(p.Y) + v.DY
	if torus {
		x = wrapAxis(x, float64(w))
		y = wrapAxis(y, float64(h))
	} else {
		x = clampAxis(x, float64(w))
		y = clampAxis(y, float64(h))
	}
	return common.Point{X: uint64(math.Round(x)), Y: uint64(math.Round(y))}
}

// IntegrateMovementBody is the footprint-aware counterpart of IntegrateMovement: it
// applies a velocity, then keeps the agent's whole footprint inside the world on a bounded
// axis (clamping the center inward by the body's AABB half-extent) or wraps the center on a
// torus axis. A zero-extent (point) body yields exactly the same result as IntegrateMovement.
func IntegrateMovementBody(p common.Point, v Vector, body CollisionBody, w, h uint64, torus bool) common.Point {
	x := float64(p.X) + v.DX
	y := float64(p.Y) + v.DY
	if torus {
		x = wrapAxis(x, float64(w))
		y = wrapAxis(y, float64(h))
	} else {
		x = clampAxisExtent(x, float64(w), body.AABBHalf.X)
		y = clampAxisExtent(y, float64(h), body.AABBHalf.Y)
	}
	return common.Point{X: uint64(math.Round(x)), Y: uint64(math.Round(y))}
}

// integrateMovementBodyRem is the sub-cell-accurate counterpart of IntegrateMovementBody
// (U4): it carries a per-agent float remainder between steps so fractional velocities
// converge to the commanded speed instead of being re-rounded away each step. The true
// (float) position is pos+rem+v; it is wrapped/clamped exactly like IntegrateMovementBody,
// then split back into an integer grid position and a new remainder in [-0.5, 0.5]. Zero
// remainder + integer velocity reproduces IntegrateMovementBody exactly (regression-pinned).
func integrateMovementBodyRem(p common.Point, rem Vec2, v Vector, body CollisionBody, w, h uint64, torus bool) (common.Point, Vec2) {
	x := float64(p.X) + rem.X + v.DX
	y := float64(p.Y) + rem.Y + v.DY
	if torus {
		x = wrapAxis(x, float64(w))
		y = wrapAxis(y, float64(h))
	} else {
		x = clampAxisExtent(x, float64(w), body.AABBHalf.X)
		y = clampAxisExtent(y, float64(h), body.AABBHalf.Y)
	}
	px, py := math.Round(x), math.Round(y)
	return common.Point{X: uint64(px), Y: uint64(py)}, Vec2{X: x - px, Y: y - py}
}

// clampAxisExtent clamps a center coordinate so the body (half-extent half) stays within
// [0, dim]. A body wider than the world is centered. half==0 reduces to clampAxis.
func clampAxisExtent(v, dim, half float64) float64 {
	if dim <= 0 {
		return 0
	}
	lo, hi := half, dim-half
	if hi < lo {
		return dim / 2
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// wrapAxis wraps v into [0, dim) for a torus axis (dim<=0 => 0).
func wrapAxis(v, dim float64) float64 {
	if dim <= 0 {
		return 0
	}
	v = math.Mod(v, dim)
	if v < 0 {
		v += dim
	}
	return v
}

// clampAxis clamps v into [0, dim] for a bounded axis (dim<=0 => 0).
func clampAxis(v, dim float64) float64 {
	if dim <= 0 {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > dim {
		return dim
	}
	return v
}

// VectorToward returns a velocity vector of magnitude speed pointing from -> to
// (torus-aware: chooses the wrapped shorter path). A zero distance yields a zero
// vector.
func VectorToward(from, to common.Point, w, h uint64, torus bool, speed float64) Vector {
	dx := axisDelta(float64(from.X), float64(to.X), float64(w), torus)
	dy := axisDelta(float64(from.Y), float64(to.Y), float64(h), torus)
	mag := math.Sqrt(dx*dx + dy*dy)
	if mag == 0 {
		return Vector{}
	}
	return Vector{DX: dx / mag * speed, DY: dy / mag * speed}
}

// PointInPolygon reports whether p lies inside the polygon defined by poly
// (ray-casting even-odd rule). It is a common.Point-shaped adapter over the
// single ray-casting implementation in pointInPolygon (placement.go).
func PointInPolygon(p common.Point, poly []common.Point) bool {
	return pointInPolygon(poly, float64(p.X), float64(p.Y))
}

// PointInCircle reports whether p lies within radius r of center. Adapter over
// containsCircle (placement.go).
func PointInCircle(p, center common.Point, r float64) bool {
	return containsCircle(float64(center.X), float64(center.Y), r, float64(p.X), float64(p.Y))
}

// PointInRect reports whether p lies inside the axis-aligned rectangle anchored
// at topLeft with size w×h. Adapter over containsRect (placement.go).
func PointInRect(p, topLeft common.Point, w, h float64) bool {
	return containsRect(float64(topLeft.X), float64(topLeft.Y), w, h, float64(p.X), float64(p.Y))
}

// ShapeContains dispatches by shape geometry to the matching point-in-shape test
// (used by at_shape / agent_shape interactions). Polygons use anchor-relative
// Points offset by the shape Position; rectangles anchor at Position (top-left);
// circles center on Position.
func ShapeContains(sh *datasims.SimulationShape, p common.Point) bool {
	if sh == nil {
		return false
	}
	switch sh.Type {
	case models.ShapeTypeRectangle:
		if sh.Width == nil || sh.Height == nil {
			return false
		}
		return PointInRect(p, sh.Position, float64(*sh.Width), float64(*sh.Height))
	case models.ShapeTypeCircle:
		if sh.Radius == nil {
			return false
		}
		return PointInCircle(p, sh.Position, *sh.Radius)
	case models.ShapeTypePolygon:
		if len(sh.Points) < 3 {
			return false
		}
		poly := make([]common.Point, len(sh.Points))
		for i, v := range sh.Points {
			poly[i] = common.Point{X: sh.Position.X + v.X, Y: sh.Position.Y + v.Y}
		}
		return PointInPolygon(p, poly)
	default:
		// Lines have no interior; nothing else is containable.
		return false
	}
}
