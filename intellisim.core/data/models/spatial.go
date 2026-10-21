package models

import "intelli.sim.core/data/common"

// AgentFootprintKind is the agent's SIMULATION collision body kind. It mirrors the
// shape geometry kinds (minus "line") so agents and shapes share the same spatial
// vocabulary. The footprint is what the generator and run engine use for spatial
// interactions (proximity, containment, boundaries, collisions) — it is independent
// of Display, which is purely visual.
type AgentFootprintKind string

const (
	// FootprintPoint is the default: a dimensionless point (exact legacy behavior).
	FootprintPoint     AgentFootprintKind = "point"
	FootprintCircle    AgentFootprintKind = "circle"
	FootprintRectangle AgentFootprintKind = "rectangle"
	FootprintPolygon   AgentFootprintKind = "polygon"
)

// AgentFootprint is the agent's simulation body, anchored & CENTERED on the agent
// position (unlike ShapeGeometry, whose rectangle anchors at the top-left). Dimensions
// are fixed per archetype (no ScalarSpec ranges): every instance of the archetype shares
// the footprint, so instances stay a single Position. A nil footprint or FootprintPoint
// resolves to a zero-extent point.
type AgentFootprint struct {
	Kind AgentFootprintKind

	Radius *float64 // circle
	Width  *uint64  // rectangle (full width; half-extent = Width/2)
	Height *uint64  // rectangle

	// Points are the polygon vertices, anchor-relative (centered on the agent position).
	Points []common.Point
}

// DisplayKind selects how an entity is rendered on the UI. Display is VISUAL only — the
// simulation engine never reads it. The simulation body comes from AgentFootprint (agents)
// or ShapeGeometry (shapes), set independently.
type DisplayKind string

const (
	// DisplayAuto renders the default representation: the agent body / shape geometry,
	// colored by the conventional "color" property.
	DisplayAuto DisplayKind = "auto"
	// DisplayImage renders a base64 image sprite.
	DisplayImage DisplayKind = "image"
	// DisplayFigure renders a custom user-drawn polygon (display only).
	DisplayFigure DisplayKind = "figure"
)

// DisplayImageData is a compressed base64 image used as an entity sprite. The backend
// downscales/re-encodes oversized uploads before persisting (see pkg/imageutil); Width and
// Height are the intrinsic pixel dimensions after compression (used for aspect/scale).
type DisplayImageData struct {
	Data     string // base64-encoded image bytes (no data: prefix)
	MimeType string // "image/png" | "image/jpeg"
	Width    uint64 // intrinsic width in px after compression
	Height   uint64 // intrinsic height in px after compression
}

// Display is the visual representation shared by agents and shapes. A nil Display (or
// DisplayAuto) renders the default body/geometry. The engine ignores Display entirely.
type Display struct {
	Kind DisplayKind

	// Image holds the sprite when Kind == DisplayImage.
	Image *DisplayImageData

	// Figure holds an anchor-relative polygon (centered on the entity position) when
	// Kind == DisplayFigure. It is independent of the simulation body.
	Figure []common.Point

	// Scale optionally scales the image/figure into world units (default 1).
	Scale *float64
}

// CollisionMode selects how an entity footprint constrains movement during a run. It is a
// property of the world, set on the Environment (parallels Torus).
type CollisionMode string

const (
	// CollisionNone (default / empty) — footprints inform spatial queries and keep the whole
	// body in-bounds, but entities may freely overlap (no physical blocking).
	CollisionNone CollisionMode = ""
	// CollisionBlock — after movement, agents may not end a step overlapping a solid shape or
	// another agent; overlaps are resolved deterministically (push-out / cancel-move).
	CollisionBlock CollisionMode = "block"
)

// CollisionStuckPolicy selects what block-mode collision response does with a
// pushable agent still overlapping a solid shape after the iteration cap (U5).
type CollisionStuckPolicy string

const (
	// CollisionStuckRevert (default / empty) — the agent is reverted to its
	// pre-step position (the legacy hard wall-integrity guarantee; wedged agents
	// freeze for the step).
	CollisionStuckRevert CollisionStuckPolicy = ""
	// CollisionStuckBestEffort — the agent keeps the partial resolution the push
	// iterations achieved: it may retain a small transient shape overlap for a
	// step but continues moving on later steps (no freeze; overlap keeps being
	// pushed out on every subsequent step).
	CollisionStuckBestEffort CollisionStuckPolicy = "best_effort"
)

// CollisionConfig tunes block-mode collision response (U5). Set on the
// Environment beside CollisionMode; copied into the simulation environment at
// generation like Torus/CollisionMode. Nil (and the zero value) reproduces the
// pre-U5 engine byte-for-byte: 4 relaxation iterations, revert policy.
type CollisionConfig struct {
	// MaxIterations raises the Gauss–Seidel relaxation cap for dense scenes.
	// 0 = engine default (4).
	MaxIterations uint
	// StuckPolicy selects the after-cap behavior (revert | best_effort).
	StuckPolicy CollisionStuckPolicy
}

// NormalizeSpatial canonicalizes the spatial/display fields on the model so storage and the
// engine observe consistent values: empty Display.Kind becomes DisplayAuto, empty
// AgentFootprint.Kind becomes FootprintPoint, and a point footprint clears any stray
// dimensions. Call it on save alongside NormalizeLLMStrategies.
func (m *Model) NormalizeSpatial() {
	if m == nil {
		return
	}
	for i := range m.Agents {
		m.Agents[i].Footprint.normalize()
		m.Agents[i].Display.normalize()
	}
	if m.Environment != nil {
		for i := range m.Environment.Shapes {
			m.Environment.Shapes[i].Display.normalize()
			m.Environment.Shapes[i].normalizeFixedPosition()
		}
	}
}

// normalizeFixedPosition enforces the fixed-position => single-instance invariant that
// module-level validation checks (validateShapeGeometry: a shape whose geometry pins an
// absolute Position must have PopulationCount == 1). Only the zero-value count is coerced,
// so a user-authored count is never altered. This makes LLM-generated fixed-position shapes
// — which omit population_count, leaving the zero value — satisfy the manual-edit validator
// and round-trip cleanly through PUT /models/:id.
func (sh *Shape) normalizeFixedPosition() {
	if sh.Geometry != nil && sh.Geometry.Position != nil && sh.Initialization.PopulationCount == 0 {
		sh.Initialization.PopulationCount = 1
	}
}

func (f *AgentFootprint) normalize() {
	if f == nil {
		return
	}
	if f.Kind == "" {
		f.Kind = FootprintPoint
	}
	if f.Kind == FootprintPoint {
		f.Radius = nil
		f.Width = nil
		f.Height = nil
		f.Points = nil
	}
}

func (d *Display) normalize() {
	if d == nil {
		return
	}
	if d.Kind == "" {
		d.Kind = DisplayAuto
	}
}
