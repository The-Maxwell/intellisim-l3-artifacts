package simulations

import (
	"math"
	"math/rand"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// PlacedPoint is an absolute, in-bounds coordinate produced by a placement
// algorithm. It is an alias of common.Point so callers can assign directly to
// SimulationAgent.Position / SimulationShape.Position.
type PlacedPoint = common.Point

// rejectionTries bounds the rejection sampling loop used to draw a point inside a
// shape before falling back to a representative point.
const rejectionTries = 30

// defaultClusterRadiusFrac is the fraction of the world's minimum dimension used
// as the cluster spread radius when none is specified.
const defaultClusterRadiusFrac = 0.15

// PlacementDirective is one LLM-supplied placement instruction for a fraction of
// the population. The generator translates the LLM response into a slice of these
// and feeds them to SampleDirectives.
type PlacementDirective struct {
	// Kind selects the directive semantics: "cluster" | "region" | "at_shape" |
	// "explicit".
	Kind string
	// Fraction is the share of the (non-explicit) remaining population this
	// directive should receive. When every directive's Fraction is zero the
	// remaining count is split equally.
	Fraction float64
	// Center is the gaussian centre for a cluster directive (world centre when nil).
	Center *common.Point
	// Radius is the cluster spread (defaults to 0.15*min(W,H) when <= 0).
	Radius float64
	// Polygon is the region for a region directive.
	Polygon []common.Point
	// ShapeUID names the environment shape for an at_shape directive.
	ShapeUID string
	// Points are exact coordinates for an explicit directive; emitted verbatim
	// (clamped) before any fractional allocation.
	Points []common.Point
}

// PlaceAgents resolves count absolute positions for one agent population using
// the model's algorithmic Placement. The returned slice always has exactly count
// entries (padding/truncating as needed), all clamped in-bounds. Warnings flag
// non-fatal degradations (missing environment, missing shape, llm fallback).
// Note: "manual" placement is NOT resolved here — agent instances with manual
// placement are left unplaced by the generator for the user to position
// per-simulation; a manual type reaching PlaceAgents (e.g. a shape) scatters
// randomly via the default branch.
func PlaceAgents(rng *rand.Rand, count uint64, pl *datamodels.Placement, env *datasims.SimulationEnvironment) ([]common.Point, []datasims.GenerationWarning) {
	if count == 0 {
		return []common.Point{}, nil
	}
	if env == nil {
		return []common.Point{}, []datasims.GenerationWarning{{Path: "placement", Reason: "no environment for placement"}}
	}

	typ := datamodels.PlacementRandom
	if pl != nil {
		typ = pl.Type
	}

	var warns []datasims.GenerationWarning

	switch typ {
	case datamodels.PlacementUniform:
		return placeUniform(count, env), warns

	case datamodels.PlacementClustered:
		return placeClustered(rng, count, pl, env), warns

	case datamodels.PlacementAtShape:
		pts, w := placeAtShape(rng, count, pl, env)
		return pts, w

	case datamodels.PlacementLLM:
		warns = append(warns, datasims.GenerationWarning{Path: "placement", Reason: "llm placement resolved as random fallback"})
		return placeRandom(rng, count, env), warns

	default: // PlacementRandom and any unknown type.
		return placeRandom(rng, count, env), warns
	}
}

// placeRandom scatters count points uniformly across the world.
func placeRandom(rng *rand.Rand, count uint64, env *datasims.SimulationEnvironment) []common.Point {
	pts := make([]common.Point, 0, count)
	for range count {
		pts = append(pts, randomPoint(rng, env))
	}
	return pts
}

func randomPoint(rng *rand.Rand, env *datasims.SimulationEnvironment) common.Point {
	x := rng.Float64() * float64(env.Width)
	y := rng.Float64() * float64(env.Height)
	return clampPoint(x, y, env)
}

// placeUniform lays out a near-square lattice of cell centres, row-major, taking
// the first count of them.
func placeUniform(count uint64, env *datasims.SimulationEnvironment) []common.Point {
	side := uint64(math.Ceil(math.Sqrt(float64(count))))
	if side == 0 {
		side = 1
	}
	cw := float64(env.Width) / float64(side)
	ch := float64(env.Height) / float64(side)

	pts := make([]common.Point, 0, count)
	for row := uint64(0); row < side && uint64(len(pts)) < count; row++ {
		for col := uint64(0); col < side && uint64(len(pts)) < count; col++ {
			cx := (float64(col) + 0.5) * cw
			cy := (float64(row) + 0.5) * ch
			pts = append(pts, clampPoint(cx, cy, env))
		}
	}
	return pts
}

// placeClustered scatters points around k gaussian cluster centres. Centres are
// assigned round-robin so output is deterministic and independent of map order.
func placeClustered(rng *rand.Rand, count uint64, pl *datamodels.Placement, env *datasims.SimulationEnvironment) []common.Point {
	k := 0
	radius := 0.0
	if pl != nil {
		if pl.Clusters != nil {
			k = int(*pl.Clusters)
		}
		if pl.ClusterRadius != nil {
			radius = float64(*pl.ClusterRadius)
		}
	}
	if k < 1 {
		k = max(int(math.Round(math.Sqrt(float64(count))/2)), 1)
	}
	if radius <= 0 {
		radius = defaultClusterRadiusFrac * float64(minDim(env))
	}

	centers := make([]common.Point, k)
	for i := range centers {
		centers[i] = randomPoint(rng, env)
	}

	pts := make([]common.Point, 0, count)
	for i := uint64(0); i < count; i++ {
		c := centers[int(i)%k]
		x := float64(c.X) + rng.NormFloat64()*radius
		y := float64(c.Y) + rng.NormFloat64()*radius
		pts = append(pts, clampPoint(x, y, env))
	}
	return pts
}

// placeAtShape places points inside the environment shapes matching pl.ShapeUID,
// assigning agents to matched shapes round-robin. Falls back to random scatter
// (with a warning) when no shape matches.
func placeAtShape(rng *rand.Rand, count uint64, pl *datamodels.Placement, env *datasims.SimulationEnvironment) ([]common.Point, []datasims.GenerationWarning) {
	var shapes []datasims.SimulationShape
	if pl != nil {
		for _, s := range env.Shapes {
			if s.ShapeUID == pl.ShapeUID {
				shapes = append(shapes, s)
			}
		}
	}
	if len(shapes) == 0 {
		warns := []datasims.GenerationWarning{{Path: "placement", Reason: "at_shape target shape not found; randomized"}}
		return placeRandom(rng, count, env), warns
	}

	pts := make([]common.Point, 0, count)
	for i := uint64(0); i < count; i++ {
		s := shapes[int(i)%len(shapes)]
		pts = append(pts, sampleInShape(rng, s, env))
	}
	return pts, nil
}

// SampleDirectives resolves count positions from a set of LLM placement
// directives. Explicit directives are emitted first (clamped, in order), then the
// remaining count is allocated across the other directives by Fraction using the
// largest-remainder method (equal split when all fractions are zero). The result
// is padded with random points / truncated so its length is exactly count.
//
// An empty directive slice (or zero count) returns an empty slice; the caller is
// expected to fall back to PlaceAgents/random in that case.
func SampleDirectives(rng *rand.Rand, count uint64, dirs []PlacementDirective, env *datasims.SimulationEnvironment) []common.Point {
	if len(dirs) == 0 || count == 0 || env == nil {
		return []common.Point{}
	}

	out := make([]common.Point, 0, count)
	remaining := count

	// 1. Emit explicit points first.
	var fractional []PlacementDirective
	for _, d := range dirs {
		if d.Kind == "explicit" {
			n := min(uint64(len(d.Points)), remaining)
			for i := uint64(0); i < n; i++ {
				p := d.Points[i]
				out = append(out, clampPoint(float64(p.X), float64(p.Y), env))
			}
			remaining -= n
			if remaining == 0 {
				return out
			}
			continue
		}
		fractional = append(fractional, d)
	}

	if len(fractional) == 0 {
		// Only explicit directives existed; pad to count with random points.
		return padRandom(rng, out, count, env)
	}

	// 2. Allocate the remaining count across fractional directives.
	alloc := allocate(remaining, fractional)

	// 3. Sample each directive's allocation.
	for i, d := range fractional {
		n := alloc[i]
		for range n {
			out = append(out, sampleDirective(rng, d, env))
		}
	}

	// 4. Pad/truncate to exactly count (rounding slack).
	out = padRandom(rng, out, count, env)
	if uint64(len(out)) > count {
		out = out[:count]
	}
	return out
}

// allocate distributes total across directives proportional to Fraction using the
// largest-remainder method. When every fraction is <= 0 it splits equally.
func allocate(total uint64, dirs []PlacementDirective) []uint64 {
	n := len(dirs)
	res := make([]uint64, n)
	if n == 0 || total == 0 {
		return res
	}

	sum := 0.0
	for _, d := range dirs {
		if d.Fraction > 0 {
			sum += d.Fraction
		}
	}

	// Equal split when no positive fractions.
	if sum <= 0 {
		base := total / uint64(n)
		rem := total % uint64(n)
		for i := range res {
			res[i] = base
			if uint64(i) < rem {
				res[i]++
			}
		}
		return res
	}

	type rema struct {
		idx  int
		frac float64
	}
	remainders := make([]rema, 0, n)
	var assigned uint64
	for i, d := range dirs {
		share := 0.0
		if d.Fraction > 0 {
			share = d.Fraction / sum * float64(total)
		}
		floor := math.Floor(share)
		res[i] = uint64(floor)
		assigned += uint64(floor)
		remainders = append(remainders, rema{idx: i, frac: share - floor})
	}

	// Distribute leftover by largest remainder; iterate the slice (stable order)
	// to keep determinism.
	leftover := total - assigned
	for leftover > 0 {
		best := -1
		bestFrac := -1.0
		for _, r := range remainders {
			if r.frac > bestFrac {
				bestFrac = r.frac
				best = r.idx
			}
		}
		if best < 0 {
			break
		}
		res[best]++
		leftover--
		// Zero-out the chosen remainder so the next pass picks a different one.
		for i := range remainders {
			if remainders[i].idx == best {
				remainders[i].frac = -1
				break
			}
		}
		// If all remainders exhausted but leftover remains, restart the pass.
		allNeg := true
		for _, r := range remainders {
			if r.frac >= 0 {
				allNeg = false
				break
			}
		}
		if allNeg && leftover > 0 {
			// Dump the rest into the first directive (deterministic).
			res[0] += leftover
			leftover = 0
		}
	}
	return res
}

// sampleDirective draws one point for the given directive Kind.
func sampleDirective(rng *rand.Rand, d PlacementDirective, env *datasims.SimulationEnvironment) common.Point {
	switch d.Kind {
	case "cluster":
		cx := float64(env.Width) / 2
		cy := float64(env.Height) / 2
		if d.Center != nil {
			cx = float64(d.Center.X)
			cy = float64(d.Center.Y)
		}
		r := d.Radius
		if r <= 0 {
			r = defaultClusterRadiusFrac * float64(minDim(env))
		}
		return clampPoint(cx+rng.NormFloat64()*r, cy+rng.NormFloat64()*r, env)

	case "region":
		return sampleInPolygon(rng, d.Polygon, env)

	case "at_shape":
		for _, s := range env.Shapes {
			if s.ShapeUID == d.ShapeUID {
				return sampleInShape(rng, s, env)
			}
		}
		return randomPoint(rng, env)

	default:
		return randomPoint(rng, env)
	}
}

// padRandom appends random points until pts has at least count entries.
func padRandom(rng *rand.Rand, pts []common.Point, count uint64, env *datasims.SimulationEnvironment) []common.Point {
	for uint64(len(pts)) < count {
		pts = append(pts, randomPoint(rng, env))
	}
	return pts
}

// --- shape sampling ------------------------------------------------------------

// sampleInShape draws one point inside the shape via bbox rejection sampling,
// falling back to the shape's representative position after rejectionTries.
func sampleInShape(rng *rand.Rand, s datasims.SimulationShape, env *datasims.SimulationEnvironment) common.Point {
	switch s.Type {
	case datamodels.ShapeTypeCircle:
		r := 0.0
		if s.Radius != nil {
			r = *s.Radius
		}
		cx, cy := float64(s.Position.X), float64(s.Position.Y)
		for range rejectionTries {
			x := cx - r + rng.Float64()*2*r
			y := cy - r + rng.Float64()*2*r
			// Test the rounded+clamped point so the returned coordinate stays
			// inside the circle after integer rounding.
			c := clampPoint(x, y, env)
			if containsCircle(cx, cy, r, float64(c.X), float64(c.Y)) {
				return c
			}
		}
		return clampPoint(cx, cy, env)

	case datamodels.ShapeTypeRectangle:
		w, h := 0.0, 0.0
		if s.Width != nil {
			w = float64(*s.Width)
		}
		if s.Height != nil {
			h = float64(*s.Height)
		}
		px, py := float64(s.Position.X), float64(s.Position.Y)
		x := px + rng.Float64()*w
		y := py + rng.Float64()*h
		// Clamp the ROUNDED point back into the rectangle's integer cells: a
		// float sample near the upper edge (e.g. 38.6 in a rect covering cells
		// 1..38) rounds one cell past the shape otherwise — "inside the shape"
		// must hold for the returned integer position, not the float sample.
		p := clampPoint(x, y, env)
		p.X = clampCellRange(p.X, s.Position.X, uint64(w))
		p.Y = clampCellRange(p.Y, s.Position.Y, uint64(h))
		return p

	case datamodels.ShapeTypePolygon:
		abs := absolutePoints(s)
		return sampleInPolygon(rng, abs, env)

	case datamodels.ShapeTypeLine:
		return sampleOnLine(rng, s, env)

	default:
		return clampPoint(float64(s.Position.X), float64(s.Position.Y), env)
	}
}

// sampleInPolygon rejection-samples inside a polygon, falling back to the bbox
// centre when the polygon is degenerate or sampling fails.
func sampleInPolygon(rng *rand.Rand, poly []common.Point, env *datasims.SimulationEnvironment) common.Point {
	if len(poly) < 3 {
		// Degenerate: use centroid of available points, else world centre.
		if len(poly) > 0 {
			var sx, sy float64
			for _, p := range poly {
				sx += float64(p.X)
				sy += float64(p.Y)
			}
			return clampPoint(sx/float64(len(poly)), sy/float64(len(poly)), env)
		}
		return clampPoint(float64(env.Width)/2, float64(env.Height)/2, env)
	}

	minX, minY, maxX, maxY := polyBBox(poly)
	for range rejectionTries {
		x := minX + rng.Float64()*(maxX-minX)
		y := minY + rng.Float64()*(maxY-minY)
		// Test the rounded+clamped point so the returned coordinate is
		// guaranteed to satisfy pointInPolygon (rounding can otherwise nudge a
		// valid float onto/past a boundary edge).
		c := clampPoint(x, y, env)
		if pointInPolygon(poly, float64(c.X), float64(c.Y)) {
			return c
		}
	}
	return clampPoint((minX+maxX)/2, (minY+maxY)/2, env)
}

// sampleOnLine lerps between consecutive line points and jitters within the line
// thickness.
func sampleOnLine(rng *rand.Rand, s datasims.SimulationShape, env *datasims.SimulationEnvironment) common.Point {
	abs := absolutePoints(s)
	if len(abs) == 0 {
		return clampPoint(float64(s.Position.X), float64(s.Position.Y), env)
	}
	if len(abs) == 1 {
		return clampPoint(float64(abs[0].X), float64(abs[0].Y), env)
	}
	// Pick a random segment, lerp along it.
	seg := rng.Intn(len(abs) - 1)
	a, b := abs[seg], abs[seg+1]
	tt := rng.Float64()
	x := float64(a.X) + tt*(float64(b.X)-float64(a.X))
	y := float64(a.Y) + tt*(float64(b.Y)-float64(a.Y))

	jitter := 0.0
	if s.Thickness != nil {
		jitter = float64(*s.Thickness) / 2
	}
	if jitter > 0 {
		x += (rng.Float64()*2 - 1) * jitter
		y += (rng.Float64()*2 - 1) * jitter
	}
	return clampPoint(x, y, env)
}

// absolutePoints translates a shape's anchor-relative Points into absolute
// coordinates using its Position anchor.
func absolutePoints(s datasims.SimulationShape) []common.Point {
	out := make([]common.Point, len(s.Points))
	for i, p := range s.Points {
		out[i] = common.Point{X: s.Position.X + p.X, Y: s.Position.Y + p.Y}
	}
	return out
}

// --- geometry primitives -------------------------------------------------------

// containsCircle reports whether (x,y) lies within radius r of centre (cx,cy).
func containsCircle(cx, cy, r, x, y float64) bool {
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= r*r
}

// containsRect reports whether (x,y) lies within the axis-aligned rectangle whose
// top-left corner is (px,py) with the given width and height.
func containsRect(px, py, w, h, x, y float64) bool {
	return x >= px && x <= px+w && y >= py && y <= py+h
}

// pointInPolygon reports whether (x,y) lies inside the polygon using the
// ray-casting (even-odd) rule.
func pointInPolygon(poly []common.Point, x, y float64) bool {
	if len(poly) < 3 {
		return false
	}
	inside := false
	n := len(poly)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := float64(poly[i].X), float64(poly[i].Y)
		xj, yj := float64(poly[j].X), float64(poly[j].Y)
		if (yi > y) != (yj > y) {
			xint := (xj-xi)*(y-yi)/(yj-yi) + xi
			if x < xint {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

func polyBBox(poly []common.Point) (minX, minY, maxX, maxY float64) {
	minX, minY = math.MaxFloat64, math.MaxFloat64
	maxX, maxY = -math.MaxFloat64, -math.MaxFloat64
	for _, p := range poly {
		fx, fy := float64(p.X), float64(p.Y)
		if fx < minX {
			minX = fx
		}
		if fx > maxX {
			maxX = fx
		}
		if fy < minY {
			minY = fy
		}
		if fy > maxY {
			maxY = fy
		}
	}
	return minX, minY, maxX, maxY
}

// --- misc helpers --------------------------------------------------------------

// clampPoint rounds x,y to the nearest integer and clamps to
// [0,Width-1] x [0,Height-1].
func clampPoint(x, y float64, env *datasims.SimulationEnvironment) common.Point {
	return common.Point{
		X: clampCoord(x, env.Width),
		Y: clampCoord(y, env.Height),
	}
}

// clampFootprintPoint shifts a placement point so the agent's whole footprint stays inside
// a bounded world. It is a no-op on a torus or for a nil/point footprint (so point agents
// keep their existing placement exactly).
func clampFootprintPoint(p common.Point, fp *datamodels.AgentFootprint, env *datasims.SimulationEnvironment) common.Point {
	if fp == nil || env == nil || env.Torus {
		return p
	}
	pp := p
	b := bodyForAgent(&RuntimeAgent{
		A:         &datasims.SimulationAgent{Position: &pp},
		Archetype: &datamodels.Agent{Footprint: fp},
	})
	if b.AABBHalf.X == 0 && b.AABBHalf.Y == 0 {
		return p
	}
	dx := clampAxisExtent(b.Center.X, float64(env.Width), b.AABBHalf.X) - b.Center.X
	dy := clampAxisExtent(b.Center.Y, float64(env.Height), b.AABBHalf.Y) - b.Center.Y
	return common.Point{X: shiftCoord(p.X, dx), Y: shiftCoord(p.Y, dy)}
}

func shiftCoord(v uint64, d float64) uint64 {
	n := math.Round(float64(v) + d)
	if n < 0 {
		return 0
	}
	return uint64(n)
}

// clampCellRange clamps a rounded coordinate into a shape's integer cell span
// [start, start+size-1]; size 0 collapses to the anchor.
func clampCellRange(v, start, size uint64) uint64 {
	if size == 0 {
		return start
	}
	if v < start {
		return start
	}
	if last := start + size - 1; v > last {
		return last
	}
	return v
}

func clampCoord(v float64, dim uint64) uint64 {
	if dim == 0 {
		return 0
	}
	if v < 0 {
		return 0
	}
	r := math.Round(v)
	maxc := float64(dim - 1)
	if r > maxc {
		return dim - 1
	}
	return uint64(r)
}

func minDim(env *datasims.SimulationEnvironment) uint64 {
	if env.Width < env.Height {
		return env.Width
	}
	return env.Height
}
