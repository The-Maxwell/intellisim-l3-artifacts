package models

import (
	"time"

	"intelli.sim.core/data/common"
)

// ModelUpdateMode defines how agent state changes are applied within a tick.
//
//	sequential   - each rule immediately updates agent state; later rules see updated
//	               values within the same tick (order-dependent, faster).
//	synchronous  - all agents compute actions from a snapshot; updates commit at tick end
//	               (deterministic, reproducible).
//
// NOTE: enforcing these semantics is engine work (modules/simulations), out of scope here.
type ModelUpdateMode string

const (
	UpdateModeSequential  ModelUpdateMode = "sequential"
	UpdateModeSynchronous ModelUpdateMode = "synchronous"
)

type Model struct {
	ID          string
	Name        string
	Description string

	ChatID string

	// LLMConfig is the Level-1 LLM config used for model generation and the model's chat.
	LLMConfig LLMConfig
	// SimulationConfig groups everything that governs simulations created from this model:
	// the update mode plus the simulation-generation and simulation-execution LLM configs.
	SimulationConfig SimulationConfig

	IsSpatial       bool
	SpatialCellSize uint64

	Agents       []Agent
	Environment  *Environment
	Rules        []Rule
	Interactions []Interaction
	Events       []EventDefinition

	// Images holds reference/schematic images (base64) supplied for Level-1 model generation,
	// e.g. an environment layout the LLM analyses for spatial structure.
	Images []ModelImage

	// Widgets are the dashboard widget templates defined on the model. Each new simulation
	// created from this model receives a copy of these widget descriptors as its initial
	// dashboard layout.
	Widgets []common.WidgetConfig

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type SimulationConfig struct {
	UpdateMode ModelUpdateMode

	LLMSimulationGenerationConfig LLMConfig
	LLMSimulationExecutionConfig  LLMSimulationExecutionConfig

	// Termination is the default early-stop condition stamped onto simulations
	// created from this model (U2). Restricted to the population / step /
	// global_property / composite condition subset; validated when it lands on a
	// simulation (ValidateInstance / ValidateRunnable). Nil = run to Steps.
	Termination *Condition
}

// ModelImage is a reference/schematic image stored with the model (base64-encoded), used as
// multimodal input for Level-1 model generation.
type ModelImage struct {
	UID         string
	Name        string
	MimeType    string // e.g. "image/png"
	Data        string // base64-encoded image bytes
	Description string
}

type AgentType string

const (
	AgentTypePlain     AgentType = "agent"
	AgentTypeArchetype AgentType = "archetype"
)

type AgentMoveType string

const (
	AgentMoveTypeStatic  AgentMoveType = "static"
	AgentMoveTypeMovable AgentMoveType = "movable"
)

// Agent defines an agent or archetype. UID is unique within the model and is the
// identifier used by rules/conditions/actions/interactions (Name is a display label only).
// Type drives LLM strategy scope: "agent" => individual scope, "archetype" => archetype scope.
type Agent struct {
	UID         string
	Name        string
	Description string
	Type        AgentType
	MoveType    AgentMoveType
	Properties  []AgentProperty

	// States is the state-machine definition; the starting state is Initialization.InitialState.
	States []string

	Initialization AgentInitialization

	LLMConfig *LLMAgentConfig

	// Footprint is the agent's SIMULATION body (collision/spatial geometry), anchored &
	// centered on the agent position. nil ⇒ a dimensionless point (legacy behavior).
	// User-authored; independent of Display.
	Footprint *AgentFootprint

	// Display is the agent's VISUAL representation (image / custom figure). nil/auto ⇒
	// the default colored body. The engine ignores Display entirely. User-authored.
	Display *Display

	// Collidable controls whether this agent participates in collision response when the
	// environment CollisionMode is "block". nil ⇒ true. User-authored.
	Collidable *bool

	// Locked, when true, freezes this agent against model-generation LLM edits: the
	// llm merge preserves it verbatim and the prompt lists it as immutable. User-only.
	Locked bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type AgentPropertyType string

const (
	PropertyTypeString  AgentPropertyType = "string"
	PropertyTypeNumber  AgentPropertyType = "number"
	PropertyTypeEnum    AgentPropertyType = "enum"
	PropertyTypeBoolean AgentPropertyType = "boolean"
	PropertyTypeColor   AgentPropertyType = "color" // hex color string, e.g. "#FF8800"; by convention the property with UID "color" is what renderers read
)

type AgentProperty struct {
	UID           string
	Name          string
	Type          AgentPropertyType
	Description   string
	MinValue      *float64
	MaxValue      *float64
	DefaultValue  interface{}
	AllowedValues []interface{}

	// Immutable locks the property VALUE after initialization: rules and LLM actions must
	// not modify it during the simulation (engine-enforced).
	Immutable bool

	LLMPromptContext string

	// Generation describes how the simulation generator should produce the initial
	// value for this property. nil means default behaviour (see PropertyGeneration).
	Generation *PropertyGeneration
}

// PlacementType defines how initial entity positions are determined.
type PlacementType string

const (
	PlacementRandom    PlacementType = "random"
	PlacementUniform   PlacementType = "uniform"
	PlacementClustered PlacementType = "clustered"
	PlacementAtShape   PlacementType = "at_shape"
	PlacementManual    PlacementType = "manual"
	PlacementLLM       PlacementType = "llm"
)

type Placement struct {
	Type          PlacementType
	ShapeUID      string // for at_shape
	PromptContext string // for llm placement

	// Clusters and ClusterRadius configure "clustered" placement (ignored for other types).
	// Clusters is the number of gaussian cluster centres; ClusterRadius is the spread in
	// world units. Both are optional: nil ⇒ derived defaults (Clusters =
	// max(round(sqrt(count)/2), 1); ClusterRadius = 0.15*min(Width,Height)).
	Clusters      *uint64
	ClusterRadius *uint64
}

// AgentInitialization is t=0 setup, needed even for rule-only (non-LLM) models.
type AgentInitialization struct {
	PopulationCount uint64 // initial instance count; must be >= 1
	MinPopulation   *uint64
	MaxPopulation   *uint64
	InitialState    string // must be one of Agent.States
	Placement       *Placement

	// CountMode controls how the simulation generator resolves PopulationCount at
	// simulation creation time. Empty string is treated as CountFixed.
	CountMode PopulationCountMode
}

// EnvironmentUID is the reserved, fixed UID of the model's Environment. No agent, shape,
// or property may reuse it.
const EnvironmentUID = "environment"

// Environment UID is the reserved value "environment"; no agent/shape/property may reuse it.
type Environment struct {
	UID              string
	Width            uint64
	Height           uint64
	Torus            bool
	Shapes           []Shape
	GlobalProperties []AgentProperty

	// CollisionMode selects the world's collision-response behavior for agent footprints
	// during a run. Empty ⇒ CollisionNone (no physical blocking).
	CollisionMode CollisionMode
	// CollisionConfig tunes block-mode collision response (U5); nil = engine
	// defaults (4 iterations, revert). Only meaningful when CollisionMode is block.
	CollisionConfig *CollisionConfig

	LLMPromptContext string
}

type ShapeType string

const (
	ShapeTypeRectangle ShapeType = "rectangle"
	ShapeTypeCircle    ShapeType = "circle"
	ShapeTypeLine      ShapeType = "line"
	ShapeTypePolygon   ShapeType = "polygon"
)

// Shape is an environment shape template. Color is represented as a "color"-typed
// property in Properties (conventional UID "color"), not a dedicated field.
type Shape struct {
	UID         string
	Name        string
	Description string
	Type        ShapeType
	Layer       uint
	Visible     bool
	Properties  []AgentProperty

	Initialization AgentInitialization

	// Geometry is the template geometry for non-LLM creation; required when
	// Placement.Type != "llm" (for "llm" it is an optional hint). Geometry.Position,
	// when set, fully fixes the instance position at model level (Placement skipped).
	Geometry *ShapeGeometry

	// Display is the shape's VISUAL representation (image / custom figure). nil/auto ⇒
	// render the geometry. The engine still uses Geometry as the simulation body; Display
	// only overrides rendering. User-authored.
	Display *Display

	LLMConfig *LLMShapeConfig

	// Locked freezes this shape against model-generation LLM edits. User-only.
	Locked bool
}

// ScalarSpec is a fixed value or a [Min..Max] random range resolved per instance at
// simulation creation. Exactly one of (Value) or (Min+Max, Min <= Max) must be set.
type ScalarSpec struct {
	Value *uint64
	Min   *uint64
	Max   *uint64
}

// ShapeGeometry is the template geometry used when Placement.Type != "llm".
// Points are relative to the anchor — the explicit Position (when set) or the anchor
// computed by Placement; the simulation generator translates them to absolute
// coordinates at creation time.
type ShapeGeometry struct {
	// Position is the absolute anchor position (top-left for rectangle, center for
	// circle, origin for the anchor-relative Points). When set, the simulation
	// generator places the instance exactly here and Placement is not consulted;
	// a fixed Position implies Initialization.PopulationCount == 1 (module-level
	// validation). Enables fully specified environments, e.g. Level-1 generation
	// from a reference image.
	Position *common.Point

	Width     *ScalarSpec    // rectangle
	Height    *ScalarSpec    // rectangle
	Radius    *ScalarSpec    // circle
	Points    []common.Point // polygon / line (anchor-relative)
	Thickness *uint          // line

	Fill        *bool
	Stroke      *bool
	StrokeWidth *uint
}
