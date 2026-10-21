package simulations

import (
	"math"
	"sort"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// collisionMaxIters bounds the agent-vs-agent relaxation passes per step.
const collisionMaxIters = 4

// collisionSettings is the resolved block-mode tuning for one run (U5).
type collisionSettings struct {
	maxIters   int
	bestEffort bool
}

// collisionSettingsFor resolves the environment's optional CollisionConfig to
// engine settings; nil/zero values fall back to the legacy defaults (4
// iterations, revert policy) so unconfigured worlds stay byte-identical.
func collisionSettingsFor(env *datasims.SimulationEnvironment) collisionSettings {
	s := collisionSettings{maxIters: collisionMaxIters}
	if env == nil || env.CollisionConfig == nil {
		return s
	}
	if env.CollisionConfig.MaxIterations > 0 {
		s.maxIters = int(env.CollisionConfig.MaxIterations)
	}
	s.bestEffort = env.CollisionConfig.StuckPolicy == models.CollisionStuckBestEffort
	return s
}

// collisionSeparation is the extra push (world units) added to every MTV so bodies end
// strictly clear of each other after integer-grid rounding (touching counts as overlap).
const collisionSeparation = 1.0

// resolveCollisions is the deterministic collision-response phase for CollisionBlock worlds.
// After movement integration it removes overlaps so no agent ends a step overlapping a solid
// shape or another agent. Determinism: agents are processed in ascending instance-id order,
// shapes are static obstacles in slice order, agent-vs-agent uses Gauss-Seidel by id with a
// fixed iteration cap and a +X tie-break for coincident centers; any pushable agent still
// inside a solid shape after the cap is reverted to its pre-phase position (before). It runs
// only on the serial spatial sub-phase (never inside the parallel LLM gather) and mutates
// only agent positions.
func resolveCollisions(rs *RuntimeState, agents []*RuntimeAgent, before map[uint64]common.Point) {
	if rs == nil || rs.Env == nil {
		return
	}
	w, h, torus := envDims(rs)

	// Collidable, positioned agents in ascending id order (deterministic).
	parts := make([]*RuntimeAgent, 0, len(agents))
	for _, ra := range agents {
		if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil || !collidable(ra) {
			continue
		}
		parts = append(parts, ra)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].A.ID < parts[j].A.ID })

	// Static solid shape bodies (lines are open ⇒ never overlap; a shape with
	// an explicit Fill=false is a PASSABLE region — doors/exits/sensor zones —
	// and does not collide, U8).
	shapes := make([]CollisionBody, 0, len(rs.Env.Shapes))
	for i := range rs.Env.Shapes {
		sh := &rs.Env.Shapes[i]
		if sh.Fill != nil && !*sh.Fill {
			continue
		}
		b := bodyForShape(sh)
		if b.Open {
			continue
		}
		shapes = append(shapes, b)
	}

	cfg := collisionSettingsFor(rs.Env)
	for iter := 0; iter < cfg.maxIters; iter++ {
		moved := false
		// Agent vs agent (each unordered pair once, in id order).
		for i := 0; i < len(parts); i++ {
			a := parts[i]
			for j := i + 1; j < len(parts); j++ {
				b := parts[j]
				pushA, pushB := pushable(a), pushable(b)
				if !pushA && !pushB {
					continue
				}
				push, ok := mtv(bodyForAgent(a), bodyForAgent(b), w, h, torus)
				if !ok {
					continue
				}
				switch {
				case pushA && pushB:
					half := Vec2{push.X / 2, push.Y / 2}
					moved = pushAgent(a, half, w, h, torus) || moved
					moved = pushAgent(b, Vec2{-half.X, -half.Y}, w, h, torus) || moved
				case pushA:
					moved = pushAgent(a, push, w, h, torus) || moved
				case pushB:
					moved = pushAgent(b, Vec2{-push.X, -push.Y}, w, h, torus) || moved
				}
			}
		}
		// Agent vs static shapes (push the agent out; shapes never move).
		for _, a := range parts {
			if !pushable(a) {
				continue
			}
			for _, sb := range shapes {
				push, ok := mtv(bodyForAgent(a), sb, w, h, torus)
				if !ok {
					continue
				}
				moved = pushAgent(a, push, w, h, torus) || moved
			}
		}
		if !moved {
			break
		}
	}

	// Hard guarantee: a pushable agent still inside a solid shape is reverted to its
	// pre-phase position (which was shape-clear when the prior tick ended).
	// best_effort (U5) skips the revert: the agent keeps the partial resolution the
	// push iterations achieved — a bounded transient overlap that keeps being pushed
	// out on subsequent steps instead of freezing the agent in place.
	if cfg.bestEffort {
		return
	}
	for _, a := range parts {
		if !pushable(a) {
			continue
		}
		ab := bodyForAgent(a)
		for _, sb := range shapes {
			if Overlap(ab, sb, w, h, torus) {
				if bpos, ok := before[a.A.ID]; ok {
					p := bpos
					a.A.Position = &p
				}
				break
			}
		}
	}
}

// pushAgent translates an agent by d (rounded to the grid), keeping its whole footprint in
// bounds (torus wrap / extent-aware clamp). Returns whether the integer position changed.
func pushAgent(ra *RuntimeAgent, d Vec2, w, h uint64, torus bool) bool {
	if d.X == 0 && d.Y == 0 {
		return false
	}
	pos := *ra.A.Position
	body := bodyForAgent(ra)
	x := float64(pos.X) + d.X
	y := float64(pos.Y) + d.Y
	if torus {
		x = wrapAxis(x, float64(w))
		y = wrapAxis(y, float64(h))
	} else {
		x = clampAxisExtent(x, float64(w), body.AABBHalf.X)
		y = clampAxisExtent(y, float64(h), body.AABBHalf.Y)
	}
	np := common.Point{X: uint64(math.Round(x)), Y: uint64(math.Round(y))}
	if np == pos {
		return false
	}
	ra.A.Position = &np
	return true
}

// collidable reports whether the agent participates in collision response at all
// (Archetype.Collidable nil ⇒ true).
func collidable(ra *RuntimeAgent) bool {
	if ra.Archetype == nil || ra.Archetype.Collidable == nil {
		return true
	}
	return *ra.Archetype.Collidable
}

// pushable reports whether the agent can be moved by collision response (movable + collidable).
// Static collidable agents are solid obstacles but are never pushed.
func pushable(ra *RuntimeAgent) bool { return movable(ra) && collidable(ra) }

// ---- minimum translation vector ----------------------------------------------

// mtv returns the translation to apply to a so it no longer overlaps b (torus-aware), and
// whether they overlapped. Open bodies (lines) never produce a push.
func mtv(a, b CollisionBody, w, h uint64, torus bool) (Vec2, bool) {
	b = translateBody(b, minImageShift(a.Center, b.Center, float64(w), float64(h), torus))
	if a.Open || b.Open {
		return Vec2{}, false
	}
	ac, ar, aCirc := circleOf(a)
	bc, br, bCirc := circleOf(b)
	switch {
	case aCirc && bCirc:
		return circleCircleMTV(ac, ar, bc, br)
	case aCirc:
		return circlePolyMTV(ac, ar, b)
	case bCirc:
		m, ok := circlePolyMTV(bc, br, a)
		return Vec2{-m.X, -m.Y}, ok
	default:
		return polyMTV(a, b)
	}
}

func circleCircleMTV(ac Vec2, ar float64, bc Vec2, br float64) (Vec2, bool) {
	dir := sub(ac, bc)
	d := math.Hypot(dir.X, dir.Y)
	sep := ar + br
	if d >= sep-collisionEps {
		return Vec2{}, false
	}
	if d < collisionEps {
		return Vec2{X: sep + collisionSeparation}, true // coincident centers: deterministic +X push
	}
	depth := sep - d + collisionSeparation
	return Vec2{X: dir.X / d * depth, Y: dir.Y / d * depth}, true
}

// circlePolyMTV pushes a circle (center c, radius r) out of the closed body p.
func circlePolyMTV(c Vec2, r float64, p CollisionBody) (Vec2, bool) {
	cp, dmin := closestBoundaryPoint(c, p)
	if pointInPoly(c, p.Verts) {
		dir := sub(cp, c) // toward the nearest boundary (outward)
		dl := math.Hypot(dir.X, dir.Y)
		if dl < collisionEps {
			dir = sub(c, vertsCenter(p.Verts))
			dl = math.Hypot(dir.X, dir.Y)
			if dl < collisionEps {
				dir, dl = Vec2{X: 1}, 1
			}
		}
		depth := dmin + r + collisionSeparation
		return Vec2{X: dir.X / dl * depth, Y: dir.Y / dl * depth}, true
	}
	if dmin >= r-collisionEps {
		return Vec2{}, false
	}
	dir := sub(c, cp)
	dl := math.Hypot(dir.X, dir.Y)
	if dl < collisionEps {
		return Vec2{X: r - dmin + collisionSeparation}, true
	}
	depth := r - dmin + collisionSeparation
	return Vec2{X: dir.X / dl * depth, Y: dir.Y / dl * depth}, true
}

// polyMTV pushes closed body a out of closed body b using the deepest overlapping convex
// piece pair's separating axis.
func polyMTV(a, b CollisionBody) (Vec2, bool) {
	best := -1.0
	var bestVec Vec2
	found := false
	for _, pa := range pieces(a) {
		for _, pb := range pieces(b) {
			axis, depth, ok := satMTV(pa, pb)
			if !ok {
				continue
			}
			if depth > best {
				d := depth + collisionSeparation
				best, bestVec, found = depth, Vec2{X: axis.X * d, Y: axis.Y * d}, true
			}
		}
	}
	return bestVec, found
}

// satMTV returns the unit axis (pushing a away from b) and penetration depth of two convex
// polygons, or ok=false if they are separated.
func satMTV(a, b []Vec2) (Vec2, float64, bool) {
	best := math.MaxFloat64
	var axisBest Vec2
	for _, poly := range [2][]Vec2{a, b} {
		n := len(poly)
		for i := 0; i < n; i++ {
			p1, p2 := poly[i], poly[(i+1)%n]
			axis := normalize(Vec2{X: -(p2.Y - p1.Y), Y: p2.X - p1.X})
			if axis == (Vec2{}) {
				continue
			}
			minA, maxA := projectRange(a, axis)
			minB, maxB := projectRange(b, axis)
			if maxA < minB || maxB < minA {
				return Vec2{}, 0, false
			}
			ov := math.Min(maxA, maxB) - math.Max(minA, minB)
			if ov < best {
				best, axisBest = ov, axis
			}
		}
	}
	// Orient the axis from b's center toward a's center.
	if dot(sub(vertsCenter(a), vertsCenter(b)), axisBest) < 0 {
		axisBest = Vec2{X: -axisBest.X, Y: -axisBest.Y}
	}
	return axisBest, best, true
}

// ---- small vector helpers ----------------------------------------------------

func dot(a, b Vec2) float64 { return a.X*b.X + a.Y*b.Y }

func normalize(v Vec2) Vec2 {
	l := math.Hypot(v.X, v.Y)
	if l < collisionEps {
		return Vec2{}
	}
	return Vec2{X: v.X / l, Y: v.Y / l}
}

func vertsCenter(pts []Vec2) Vec2 {
	if len(pts) == 0 {
		return Vec2{}
	}
	var sx, sy float64
	for _, p := range pts {
		sx, sy = sx+p.X, sy+p.Y
	}
	n := float64(len(pts))
	return Vec2{X: sx / n, Y: sy / n}
}

// closestBoundaryPoint returns the closest point on the closed body's boundary to c and its
// distance.
func closestBoundaryPoint(c Vec2, b CollisionBody) (Vec2, float64) {
	n := len(b.Verts)
	if n == 0 {
		return b.Center, dist(c, b.Center)
	}
	if n == 1 {
		return b.Verts[0], dist(c, b.Verts[0])
	}
	best := math.MaxFloat64
	var bestPt Vec2
	for i := 0; i < n; i++ {
		a1 := b.Verts[i]
		a2 := b.Verts[(i+1)%n]
		pt := closestPointOnSeg(c, a1, a2)
		if d := dist(c, pt); d < best {
			best, bestPt = d, pt
		}
	}
	return bestPt, best
}

func closestPointOnSeg(p, a, b Vec2) Vec2 {
	ab := sub(b, a)
	denom := ab.X*ab.X + ab.Y*ab.Y
	if denom == 0 {
		return a
	}
	t := ((p.X-a.X)*ab.X + (p.Y-a.Y)*ab.Y) / denom
	t = math.Max(0, math.Min(1, t))
	return Vec2{X: a.X + t*ab.X, Y: a.Y + t*ab.Y}
}
