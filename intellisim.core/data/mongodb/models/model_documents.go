package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

// ModelDocument is the MongoDB representation of a simulation model.
type ModelDocument struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Name        string        `bson:"name"`
	Description string        `bson:"description,omitempty"`

	ChatID bson.ObjectID `bson:"chat_id,omitempty"`

	LLMConfig        LLMConfigDocument        `bson:"llm_config"`
	SimulationConfig SimulationConfigDocument `bson:"simulation_config"`

	IsSpatial       bool   `bson:"is_spatial"`
	SpatialCellSize uint64 `bson:"spatial_cell_size,omitempty"`

	Agents       []AgentDocument           `bson:"agents"`
	Environment  *EnvironmentDocument      `bson:"environment,omitempty"`
	Rules        []RuleDocument            `bson:"rules,omitempty"`
	Interactions []InteractionDocument     `bson:"interactions,omitempty"`
	Events       []EventDefinitionDocument `bson:"events,omitempty"`

	// Images holds reference/schematic images (base64) supplied for Level-1 model generation,
	// e.g. an environment layout the LLM analyses for spatial structure.
	Images []ModelImageDocument `bson:"images,omitempty"`

	// Widgets are the dashboard widget templates copied into each new simulation.
	Widgets []mongocommon.WidgetConfigDocument `bson:"widgets,omitempty"`

	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
	CreatedBy string    `bson:"created_by"`
	UpdatedBy string    `bson:"updated_by"`
}

// ModelImageDocument is a reference/schematic image stored with the model (base64-encoded), used as
// multimodal input for Level-1 model generation.
type ModelImageDocument struct {
	UID         string `bson:"uid"`
	Name        string `bson:"name,omitempty"`
	MimeType    string `bson:"mime_type,omitempty"` // e.g. "image/png"
	Data        string `bson:"data"`                // base64-encoded image bytes
	Description string `bson:"description,omitempty"`
}

// AgentDocument defines an agent or archetype. UID is unique within the model and is the
// identifier used by rules/conditions/actions/interactions (Name is a display label only).
// Type drives LLM strategy scope: "agent" => individual scope, "archetype" => archetype scope.
type AgentDocument struct {
	UID         string                  `bson:"uid"`
	Name        string                  `bson:"name"`
	Description string                  `bson:"description,omitempty"`
	Type        models.AgentType        `bson:"type"`
	MoveType    models.AgentMoveType    `bson:"move_type"`
	Properties  []AgentPropertyDocument `bson:"properties"`

	// States is the state-machine definition; the starting state is Initialization.InitialState.
	States []string `bson:"states,omitempty"`

	Initialization AgentInitializationDocument `bson:"initialization"`

	LLMConfig *LLMAgentConfigDocument `bson:"llm_config,omitempty"`

	// Footprint is the agent's simulation collision body (nil ⇒ point).
	Footprint *AgentFootprintDocument `bson:"footprint,omitempty"`
	// Display is the agent's visual representation (nil/auto ⇒ default body).
	Display *DisplayDocument `bson:"display,omitempty"`
	// Collidable gates collision-response participation in block mode (nil ⇒ true).
	Collidable *bool `bson:"collidable,omitempty"`

	Locked bool `bson:"locked,omitempty"`

	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// AgentFootprintDocument is the agent's simulation body (collision geometry), centered on
// the agent position. Dimensions are fixed per archetype (no ScalarSpec).
type AgentFootprintDocument struct {
	Kind   models.AgentFootprintKind   `bson:"kind"`
	Radius *float64                    `bson:"radius,omitempty"` // circle
	Width  *uint64                     `bson:"width,omitempty"`  // rectangle
	Height *uint64                     `bson:"height,omitempty"` // rectangle
	Points []mongocommon.PointDocument `bson:"points,omitempty"` // polygon (anchor-relative, centered)
}

// DisplayDocument is the VISUAL representation shared by agents and shapes (image / figure).
type DisplayDocument struct {
	Kind   models.DisplayKind          `bson:"kind"`
	Image  *DisplayImageDocument       `bson:"image,omitempty"`
	Figure []mongocommon.PointDocument `bson:"figure,omitempty"` // anchor-relative polygon, display only
	Scale  *float64                    `bson:"scale,omitempty"`
}

// DisplayImageDocument is a compressed base64 sprite stored inline on the entity.
type DisplayImageDocument struct {
	Data     string `bson:"data"`
	MimeType string `bson:"mime_type,omitempty"`
	Width    uint64 `bson:"width,omitempty"`
	Height   uint64 `bson:"height,omitempty"`
}

// AgentPropertyDocument is a property of an agent or shape.
type AgentPropertyDocument struct {
	UID           string                   `bson:"uid"`
	Name          string                   `bson:"name"`
	Type          models.AgentPropertyType `bson:"type"`
	Description   string                   `bson:"description,omitempty"`
	MinValue      *float64                 `bson:"min_value,omitempty"`
	MaxValue      *float64                 `bson:"max_value,omitempty"`
	DefaultValue  interface{}              `bson:"default_value,omitempty"`
	AllowedValues []interface{}            `bson:"allowed_values,omitempty"`

	// Immutable locks the property VALUE after initialization: rules and LLM actions must
	// not modify it during the simulation (engine-enforced).
	Immutable bool `bson:"immutable,omitempty"`

	LLMPromptContext string `bson:"llm_prompt_context,omitempty"`

	// Generation describes how the simulation generator should produce the initial
	// value for this property. nil means default behaviour.
	Generation *PropertyGenerationDocument `bson:"generation,omitempty"`
}

// PlacementDocument defines how initial entity positions are determined.
type PlacementDocument struct {
	Type          models.PlacementType `bson:"type"`
	ShapeUID      string               `bson:"shape_uid,omitempty"`      // for at_shape
	PromptContext string               `bson:"prompt_context,omitempty"` // for llm placement

	Clusters      *uint64 `bson:"clusters,omitempty"`       // clustered: number of centres
	ClusterRadius *uint64 `bson:"cluster_radius,omitempty"` // clustered: gaussian spread
}

// AgentInitializationDocument is t=0 setup, needed even for rule-only (non-LLM) models.
type AgentInitializationDocument struct {
	PopulationCount uint64             `bson:"population_count"` // initial instance count; must be >= 1
	MinPopulation   *uint64            `bson:"min_population,omitempty"`
	MaxPopulation   *uint64            `bson:"max_population,omitempty"`
	InitialState    string             `bson:"initial_state,omitempty"` // must be one of Agent.States
	Placement       *PlacementDocument `bson:"placement,omitempty"`

	// CountMode controls how the simulation generator resolves PopulationCount.
	// Empty string is treated as CountFixed.
	CountMode models.PopulationCountMode `bson:"count_mode,omitempty"`
}

// EnvironmentDocument UID is the reserved value "environment"; no agent/shape/property may reuse it.
type EnvironmentDocument struct {
	UID              string                  `bson:"uid"`
	Width            uint64                  `bson:"width"`
	Height           uint64                  `bson:"height"`
	Torus            bool                    `bson:"torus"`
	Shapes           []ShapeDocument         `bson:"shapes"`
	GlobalProperties []AgentPropertyDocument `bson:"global_properties,omitempty"`

	CollisionMode models.CollisionMode `bson:"collision_mode,omitempty"`
	// CollisionConfig tunes block-mode collision response (U5); absent = engine defaults.
	CollisionConfig *CollisionConfigDocument `bson:"collision_config,omitempty"`

	LLMPromptContext string `bson:"llm_prompt_context,omitempty"`
}

// CollisionConfigDocument mirrors models.CollisionConfig (U5). Reused by the
// simulations documents (like ConditionDocument).
type CollisionConfigDocument struct {
	MaxIterations uint                        `bson:"max_iterations,omitempty"`
	StuckPolicy   models.CollisionStuckPolicy `bson:"stuck_policy,omitempty"`
}

// ShapeDocument is an environment shape template. Color is represented as a "color"-typed
// property in Properties (conventional UID "color"), not a dedicated field.
type ShapeDocument struct {
	UID         string                  `bson:"uid"`
	Name        string                  `bson:"name"`
	Description string                  `bson:"description,omitempty"`
	Type        models.ShapeType        `bson:"type"`
	Layer       uint                    `bson:"layer,omitempty"`
	Visible     bool                    `bson:"visible"`
	Properties  []AgentPropertyDocument `bson:"properties"`

	Initialization AgentInitializationDocument `bson:"initialization"`

	// Geometry is the template geometry for non-LLM creation; required when
	// Placement.Type != "llm" (for "llm" it is an optional hint). Geometry.Position,
	// when set, fully fixes the instance position at model level (Placement skipped).
	Geometry *ShapeGeometryDocument `bson:"geometry,omitempty"`

	// Display overrides rendering with an image / custom figure (nil/auto ⇒ render geometry).
	Display *DisplayDocument `bson:"display,omitempty"`

	LLMConfig *LLMShapeConfigDocument `bson:"llm_config,omitempty"`

	Locked bool `bson:"locked,omitempty"`
}

// ScalarSpecDocument is a fixed value or a [Min..Max] random range resolved per instance at
// simulation creation. Exactly one of (Value) or (Min+Max, Min <= Max) must be set.
type ScalarSpecDocument struct {
	Value *uint64 `bson:"value,omitempty"`
	Min   *uint64 `bson:"min,omitempty"`
	Max   *uint64 `bson:"max,omitempty"`
}

// ShapeGeometryDocument is the template geometry used when Placement.Type != "llm".
// Points are relative to the anchor — the explicit Position (when set) or the anchor
// computed by Placement; the simulation generator translates them to absolute
// coordinates at creation time.
type ShapeGeometryDocument struct {
	// Position is the absolute anchor position (top-left for rectangle, center for
	// circle, origin for the anchor-relative Points). When set, Placement is not
	// consulted; a fixed Position implies Initialization.PopulationCount == 1.
	Position *mongocommon.PointDocument `bson:"position,omitempty"`

	Width     *ScalarSpecDocument         `bson:"width,omitempty"`     // rectangle
	Height    *ScalarSpecDocument         `bson:"height,omitempty"`    // rectangle
	Radius    *ScalarSpecDocument         `bson:"radius,omitempty"`    // circle
	Points    []mongocommon.PointDocument `bson:"points,omitempty"`    // polygon / line (anchor-relative)
	Thickness *uint                       `bson:"thickness,omitempty"` // line

	Fill        *bool `bson:"fill,omitempty"`
	Stroke      *bool `bson:"stroke,omitempty"`
	StrokeWidth *uint `bson:"stroke_width,omitempty"`
}
