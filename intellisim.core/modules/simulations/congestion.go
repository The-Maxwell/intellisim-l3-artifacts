package simulations

import (
	"math"
	"sort"
	"strings"

	"intelli.sim.core/data/common"
)

// U10 — congestion capability (speed–density coupling + shape flow limits),
// configured entirely through the environment's global properties so no core
// data structure changes are needed. Absent keys ⇒ both mechanisms off and the
// spatial phase is byte-identical to pre-U10 behavior.
//
// Key convention (numeric values):
//
//	congestion.speed_density.radius            kernel radius, world cells (> 0)
//	congestion.speed_density.onset_density     D0, agents/cell² (>= 0); f(D<=D0) = 1
//	congestion.speed_density.jam_density       Djam, agents/cell² (> onset)
//	congestion.speed_density.min_speed_factor  factor floor at Djam ((0,1])
//	congestion.flow_limit.<shape_uid>          admissions/step into the shape (> 0)
//	congestion.flow_limit_burst.<shape_uid>    idle-accumulation cap (>= 1; absent ⇒ 1)
const (
	congKeySDRadius = "congestion.speed_density.radius"
	congKeySDOnset  = "congestion.speed_density.onset_density"
	congKeySDJam    = "congestion.speed_density.jam_density"
	congKeySDFloor  = "congestion.speed_density.min_speed_factor"

	congKeyFlowLimitPrefix = "congestion.flow_limit."
	congKeyFlowBurstPrefix = "congestion.flow_limit_burst."
)

// speedDensitySettings is the parsed, validated speed–density configuration.
type speedDensitySettings struct {
	radius float64 // sampling kernel radius (world cells)
	onset  float64 // density at/below which the factor is exactly 1.0
	jam    float64 // density at/above which the factor is the floor
	floor  float64 // minimum speed factor (validated > 0: anti-deadlock, D30)
}

// flowLimitSetting is one parsed, validated per-shape service rate.
type flowLimitSetting struct {
	shapeUID string
	rate     float64 // admissions per step (fractional allowed)
	burst    float64 // idle-accumulation cap (>= 1)
}

// congestionSettings is the per-tick resolved congestion configuration.
type congestionSettings struct {
	density *speedDensitySettings // nil = coupling off
	limits  []flowLimitSetting    // empty = gating off; sorted by shape UID
}

// congestionSettingsFor parses the congestion key convention out of the run
// environment's global properties. Malformed or incomplete configuration
// disables the affected mechanism (or the affected limit) with a warnOnce
// diagnostic — never an error: the globals are ordinary properties as far as
// persistence/validation are concerned. Limit order is sorted by shape UID so
// map iteration order never leaks into admission processing.
func congestionSettingsFor(rs *RuntimeState) congestionSettings {
	var out congestionSettings
	if rs == nil || rs.Env == nil || len(rs.Env.GlobalProperties) == 0 {
		return out
	}
	globals := rs.Env.GlobalProperties

	// Speed–density coupling: enabled only when all four keys parse and bounds hold.
	sdKeys := [4]string{congKeySDRadius, congKeySDOnset, congKeySDJam, congKeySDFloor}
	var sdVals [4]float64
	sdPresent, sdValid := 0, true
	for i, k := range sdKeys {
		raw, ok := globals[k]
		if !ok {
			sdValid = false
			continue
		}
		sdPresent++
		f, okNum := coerceFloat(raw)
		if !okNum {
			sdValid = false
			continue
		}
		sdVals[i] = f
	}
	if sdPresent > 0 {
		switch {
		case !sdValid:
			warnOnce(rs, "congestion: speed_density keys incomplete or non-numeric; coupling disabled")
		case !(sdVals[0] > 0) || sdVals[1] < 0 || !(sdVals[2] > sdVals[1]) || !(sdVals[3] > 0 && sdVals[3] <= 1):
			warnOnce(rs, "congestion: speed_density values out of range (need radius>0, 0<=onset<jam, 0<min_speed_factor<=1); coupling disabled")
		default:
			out.density = &speedDensitySettings{radius: sdVals[0], onset: sdVals[1], jam: sdVals[2], floor: sdVals[3]}
		}
	}

	// Flow limits: collect UIDs first, then sort for deterministic processing order.
	var uids []string
	for k := range globals {
		if strings.HasPrefix(k, congKeyFlowLimitPrefix) {
			uids = append(uids, strings.TrimPrefix(k, congKeyFlowLimitPrefix))
		}
	}
	sort.Strings(uids)
	for _, uid := range uids {
		rate, ok := coerceFloat(globals[congKeyFlowLimitPrefix+uid])
		if !ok || !(rate > 0) {
			warnOnce(rs, "congestion: flow_limit for shape "+uid+" is not a positive number; limit skipped")
			continue
		}
		if !shapeUIDExists(rs, uid) {
			warnOnce(rs, "congestion: flow_limit references unknown shape uid "+uid+"; limit skipped")
			continue
		}
		burst := 1.0
		if rawB, okB := globals[congKeyFlowBurstPrefix+uid]; okB {
			if b, okNum := coerceFloat(rawB); okNum && b >= 1 {
				burst = b
			} else {
				warnOnce(rs, "congestion: flow_limit_burst for shape "+uid+" is invalid (need >= 1); using 1.0")
			}
		}
		out.limits = append(out.limits, flowLimitSetting{shapeUID: uid, rate: rate, burst: burst})
	}
	return out
}

// shapeUIDExists reports whether any environment shape instance was created
// from the given template UID. Uses the run lookup map when present, with a
// linear fallback for hand-built test states.
func shapeUIDExists(rs *RuntimeState, uid string) bool {
	if rs.shapesByUID != nil {
		_, ok := rs.shapesByUID[uid]
		return ok
	}
	if rs.Env == nil {
		return false
	}
	for i := range rs.Env.Shapes {
		if rs.Env.Shapes[i].ShapeUID == uid {
			return true
		}
	}
	return false
}

// speedDensityFactor maps a local density to a speed factor: exactly 1.0 at or
// below the onset (the no-op-at-low-density guarantee), the floor at or above
// jam, linear in between.
func speedDensityFactor(s *speedDensitySettings, d float64) float64 {
	if d <= s.onset {
		return 1.0
	}
	if d >= s.jam {
		return s.floor
	}
	t := (d - s.onset) / (s.jam - s.onset)
	return 1.0 + t*(s.floor-1.0)
}

// densityFactors computes the per-agent speed factor from the agents'
// pre-integration positions (post-rules — identical in sequential and
// synchronous modes, which is what keeps the two update modes in parity; the
// perceive-phase rs.Index is deliberately NOT reused because rules may have
// teleported agents after it was built). Density counts OTHER agents whose
// position lies within the kernel radius — self is excluded so an isolated
// agent's factor is exactly 1.0. Only factors != 1 are returned. Zero RNG use.
func densityFactors(rs *RuntimeState, agents []*RuntimeAgent, s *speedDensitySettings) map[uint64]float64 {
	area := math.Pi * s.radius * s.radius
	if area <= 0 {
		return nil
	}
	factors := make(map[uint64]float64)
	var idx *SpatialIndex
	if rs.Index != nil {
		// Grid-accelerated frozen view over the current (pre-integration) positions.
		idx = NewSpatialIndex(indexDims(rs))
		idx.Rebuild(agents)
	}
	w, h, torus := envDims(rs)
	for _, ra := range agents {
		if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil || !movable(ra) {
			continue
		}
		n := 0
		if idx != nil {
			for _, id := range idx.Within(*ra.A.Position, s.radius) {
				if id != ra.A.ID {
					n++
				}
			}
		} else {
			for _, other := range agents {
				if other == nil || other.A == nil || other.removed || other.A.Position == nil || other.A.ID == ra.A.ID {
					continue
				}
				if Distance(*ra.A.Position, *other.A.Position, w, h, torus) <= s.radius {
					n++
				}
			}
		}
		if n == 0 {
			continue
		}
		if f := speedDensityFactor(s, float64(n)/area); f != 1.0 {
			factors[ra.A.ID] = f
		}
	}
	return factors
}

// flowRuntime is the per-run mutable flow-limiter state. It is a POINTER field
// on RuntimeState so the synchronous workView (a shallow copy) shares it — the
// same invariant class as rs.stats / rs.warned. Budgets are keyed by shape
// template UID; multiple instances of one UID share a budget. Runtime-only,
// never persisted (same acceptance as the U4 sub-cell remainder).
type flowRuntime struct {
	budgets map[string]float64
}

// applyFlowLimits enforces the per-shape service rates: it accrues each
// limit's fractional budget once per tick (clamp-before-accrue: idle
// accumulation is capped FIRST, then the rate is added, so carry below 1 is
// never clipped and long-run throughput equals the rate even for rates > 1),
// then charges one budget unit per agent whose body transitions from outside
// to overlapping a gated shape between its pre-phase and end-of-phase
// positions. Admissions run in ascending instance id (matching
// resolveCollisions). A denied agent is held: reverted to its pre-phase
// position and pre-phase sub-cell remainder, exactly like the U5 revert
// contract. Entry detection uses the same Overlap predicate family as the U8
// `position in` condition, so the gate and a removal rule agree on what "in
// the shape" means; because rules read previous-tick-end positions, a
// `position in` remove rule can only ever fire for agents this gate admitted.
func applyFlowLimits(rs *RuntimeState, agents []*RuntimeAgent, before map[uint64]common.Point, beforeRem map[uint64]Vec2, limits []flowLimitSetting) {
	if len(limits) == 0 {
		return
	}
	fl := rs.flow
	if fl == nil {
		// Defensive lazy alloc for hand-built sequential test states. Synchronous
		// hand-built states must pre-allocate (NewRuntimeState-built runs always
		// do): a lazy alloc on the workView copy would be lost at commit.
		fl = &flowRuntime{budgets: make(map[string]float64)}
		rs.flow = fl
	}
	if fl.budgets == nil {
		fl.budgets = make(map[string]float64)
	}
	w, h, torus := envDims(rs)

	// Accrue once per tick (applySpatial runs exactly once per tick in both modes).
	for _, lim := range limits {
		b := fl.budgets[lim.shapeUID]
		if b > lim.burst {
			b = lim.burst
		}
		fl.budgets[lim.shapeUID] = b + lim.rate
	}

	// Admissions in ascending instance id.
	ordered := make([]*RuntimeAgent, 0, len(agents))
	for _, ra := range agents {
		if ra == nil || ra.A == nil || ra.removed || ra.A.Position == nil {
			continue
		}
		if _, ok := before[ra.A.ID]; !ok {
			continue
		}
		ordered = append(ordered, ra)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].A.ID < ordered[j].A.ID })

	for _, ra := range ordered {
		bpos := before[ra.A.ID]
		if bpos == *ra.A.Position {
			continue // no movement this phase ⇒ no entry transition possible
		}
		var entered []string
		for i := range limits {
			uid := limits[i].shapeUID
			if overlapsShapeUIDAt(rs, ra, bpos, uid, w, h, torus) {
				continue // already inside at phase start (admitted earlier / spawned inside)
			}
			if overlapsShapeUIDAt(rs, ra, *ra.A.Position, uid, w, h, torus) {
				entered = append(entered, uid)
			}
		}
		if len(entered) == 0 {
			continue
		}
		// All-or-nothing across every gated shape entered this step: either every
		// budget covers the admission or the agent is held (no partial deduction).
		admit := true
		for _, uid := range entered {
			if fl.budgets[uid] < 1.0 {
				admit = false
				break
			}
		}
		if admit {
			for _, uid := range entered {
				fl.budgets[uid] -= 1.0
			}
			continue
		}
		p := bpos
		ra.A.Position = &p
		if r, ok := beforeRem[ra.A.ID]; ok {
			ra.rem = r
		} else {
			ra.rem = Vec2{}
		}
	}
}

// overlapsShapeUIDAt reports whether the agent's footprint, evaluated at pos,
// overlaps ANY environment shape instance created from the given template UID
// (touching counts; torus-aware via Overlap's min-image handling).
func overlapsShapeUIDAt(rs *RuntimeState, ra *RuntimeAgent, pos common.Point, uid string, w, h uint64, torus bool) bool {
	if rs.Env == nil {
		return false
	}
	body := bodyForAgentAt(ra, Vec2{X: float64(pos.X), Y: float64(pos.Y)})
	for i := range rs.Env.Shapes {
		sh := &rs.Env.Shapes[i]
		if sh.ShapeUID != uid {
			continue
		}
		if Overlap(body, bodyForShape(sh), w, h, torus) {
			return true
		}
	}
	return false
}
