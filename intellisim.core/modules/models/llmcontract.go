package models

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// This file defines the STRIPPED LLM-contract DTOs: the exact JSON shape the
// model-generation LLM is allowed to author. It is a SEPARATE DTO set from the
// API DTOs in responses.go — it deliberately mirrors only the STRUCTURAL and
// BEHAVIORAL fields of data/models and EXCLUDES every user-only field:
//
//	- no llm_config / target / strategy / simulation-config / fallback
//	- no locked (entity freeze flag is user-only)
//	- no immutable (property value-freeze flag is user-only)
//	- no ids / timestamps / ownership / chat_id / update_mode / spatial_cell_size
//
// It INCLUDES the descriptive llm_prompt_context free-text hints wherever the
// domain has them (the approved §6a exception — these are guidance authored for
// the LLM, so the LLM may rewrite them).

// LLMGenerationEnvelope is the exact JSON the model-generation LLM must return.
type LLMGenerationEnvelope struct {
	DisplayMessage string    `json:"display_message"`
	Model          *LLMModel `json:"model,omitempty"` // full model OR scoped slice
}

// ===================== tolerant scalars (U6/U6b) =====================
//
// Every integer- and bool-typed field of this envelope uses one of the types
// below so that whatever the LLM emits decodes tolerantly: fractional numbers
// round to the nearest integer (half away from zero), quoted numbers parse,
// negatives into unsigned clamp to 0, overflow clamps to the type max, bools
// accept common string forms, and unparseable garbage degrades to the zero
// value. These unmarshalers NEVER return an error: an error from a custom
// Unmarshaler aborts the rest of the envelope decode (verified against
// encoding/json), so one bad token would kill the whole generation draft.
// Mismatches on the remaining plain-typed fields degrade per-field via the
// parseEnvelopeSoft fail-soft net. The tolerance is envelope-only — API DTOs,
// engine types and Mongo documents stay strict.

// tolerantNumber extracts a float from a JSON number OR a quoted numeric
// string ("1.2"). Returns ok=false for null and unparseable input.
func tolerantNumber(data []byte) (float64, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return 0, false
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return 0, false
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	var f float64
	if err := json.Unmarshal(trimmed, &f); err != nil {
		return 0, false
	}
	return f, true
}

// llmUint64 tolerantly decodes any JSON number (or numeric string) into a
// uint64; garbage and negatives degrade to 0.
type llmUint64 uint64

func (v *llmUint64) UnmarshalJSON(data []byte) error {
	*v = 0
	f, ok := tolerantNumber(data)
	if !ok || math.IsNaN(f) || f < 0 {
		return nil // garbage/negative -> 0; readiness validation flags nonsense
	}
	// >= because float64 rounds the math.MaxUint64 constant up to 2^64: a bare
	// 2^64 (or 2^64-1, which also rounds to 2^64 as float64) must clamp, never
	// overflow the uint64 conversion.
	if f >= math.MaxUint64 {
		*v = llmUint64(math.MaxUint64)
		return nil
	}
	*v = llmUint64(math.Round(f))
	return nil
}

// std returns a nil-safe *uint64 copy for the data/models boundary.
func (v *llmUint64) std() *uint64 {
	if v == nil {
		return nil
	}
	u := uint64(*v)
	return &u
}

// llmUint tolerantly decodes any non-negative JSON number into a uint.
type llmUint uint

func (v *llmUint) UnmarshalJSON(data []byte) error {
	var u llmUint64
	_ = u.UnmarshalJSON(data) // never errors
	*v = llmUint(u)
	return nil
}

// std returns a nil-safe *uint copy for the data/models boundary.
func (v *llmUint) std() *uint {
	if v == nil {
		return nil
	}
	u := uint(*v)
	return &u
}

// llmInt tolerantly decodes any JSON number (or numeric string) into an int,
// sign preserved; garbage degrades to 0 and out-of-range values clamp.
type llmInt int

func (v *llmInt) UnmarshalJSON(data []byte) error {
	*v = 0
	f, ok := tolerantNumber(data)
	if !ok || math.IsNaN(f) {
		return nil
	}
	if f >= math.MaxInt64 {
		*v = llmInt(math.MaxInt64)
		return nil
	}
	if f <= math.MinInt64 {
		*v = llmInt(math.MinInt64)
		return nil
	}
	*v = llmInt(math.Round(f))
	return nil
}

// or returns the value or def when the pointer is nil (derefInt equivalent).
func (v *llmInt) or(def int) int {
	if v == nil {
		return def
	}
	return int(*v)
}

// llmBool tolerantly decodes a JSON bool, a number (0 = false, else true), or
// a common string form (true/false/yes/no/on/off/1/0, case-insensitive); any
// other value degrades to false. Never errors (see the U6b block comment).
type llmBool bool

func (v *llmBool) UnmarshalJSON(data []byte) error {
	*v = false
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	var b bool
	if err := json.Unmarshal(trimmed, &b); err == nil {
		*v = llmBool(b)
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return nil
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "yes", "on", "1":
			*v = true
		}
		return nil
	}
	if f, ok := tolerantNumber(trimmed); ok && f != 0 && !math.IsNaN(f) {
		*v = true
	}
	return nil
}

// std returns a nil-safe *bool copy for the data/models boundary.
func (v *llmBool) std() *bool {
	if v == nil {
		return nil
	}
	b := bool(*v)
	return &b
}

// or returns the value or def when the pointer is nil (derefBool equivalent).
func (v *llmBool) or(def bool) bool {
	if v == nil {
		return def
	}
	return bool(*v)
}

// Pointer constructors for the model → envelope serialization direction.
func llmU64Ptr(u uint64) *llmUint64 { v := llmUint64(u); return &v }
func llmUPtr(u uint) *llmUint       { v := llmUint(u); return &v }
func llmIPtr(i int) *llmInt         { v := llmInt(i); return &v }
func llmBPtr(b bool) *llmBool       { v := llmBool(b); return &v }

func llmU64From(p *uint64) *llmUint64 {
	if p == nil {
		return nil
	}
	return llmU64Ptr(*p)
}

func llmUFrom(p *uint) *llmUint {
	if p == nil {
		return nil
	}
	return llmUPtr(*p)
}

func llmBFrom(p *bool) *llmBool {
	if p == nil {
		return nil
	}
	return llmBPtr(*p)
}

// LLMModel is the structural/behavioral model the LLM may author. For scoped
// edits only the relevant section is non-nil.
//
// FIELD-LEVEL RETENTION: scalar fields where the zero value is a meaningful,
// settable value (bools, numerics) are pointer-typed with omitempty so an
// absent field is distinguishable from an explicit zero. On UPDATE a nil
// pointer keeps the previous value; an explicit value (including the zero) is
// applied. Slices follow the same contract at the slice level: nil keeps the
// previous list, a non-nil (possibly empty) slice replaces it.
type LLMModel struct {
	Description string   `json:"description,omitempty"`
	IsSpatial   *llmBool `json:"is_spatial,omitempty"`
	// Termination is the run-stop condition (maps to
	// Model.SimulationConfig.Termination, U2). Restricted to the engine's
	// termination subset — population | step | global_property | composite —
	// so the LLM can author "stop when everyone has left" directly (U7).
	Termination  *LLMCondition    `json:"termination,omitempty"`
	Agents       []LLMAgent       `json:"agents,omitempty"`
	Environment  *LLMEnvironment  `json:"environment,omitempty"`
	Rules        []LLMRule        `json:"rules,omitempty"`
	Interactions []LLMInteraction `json:"interactions,omitempty"`
	Events       []LLMEvent       `json:"events,omitempty"`
	Widgets      []LLMWidget      `json:"widgets,omitempty"`
}

// ===================== agents =====================

// LLMAgent mirrors the structural fields of data/models.Agent. Excludes
// LLMConfig, Locked, CreatedAt, UpdatedAt.
type LLMAgent struct {
	UID            string                 `json:"uid"`
	Name           string                 `json:"name,omitempty"`
	Description    string                 `json:"description,omitempty"`
	Type           string                 `json:"type,omitempty"`      // agent | archetype
	MoveType       string                 `json:"move_type,omitempty"` // static | movable
	Properties     []LLMAgentProperty     `json:"properties,omitempty"`
	States         []string               `json:"states,omitempty"`
	Initialization LLMAgentInitialization `json:"initialization"`
}

// LLMAgentProperty mirrors data/models.AgentProperty. Excludes Immutable;
// includes the descriptive LLMPromptContext.
type LLMAgentProperty struct {
	UID              string                 `json:"uid"`
	Name             string                 `json:"name,omitempty"`
	Type             string                 `json:"type,omitempty"` // string | number | enum | boolean | color
	Description      string                 `json:"description,omitempty"`
	MinValue         *float64               `json:"min_value,omitempty"`
	MaxValue         *float64               `json:"max_value,omitempty"`
	DefaultValue     interface{}            `json:"default_value,omitempty"`
	AllowedValues    []interface{}          `json:"allowed_values,omitempty"`
	LLMPromptContext string                 `json:"llm_prompt_context,omitempty"`
	Generation       *LLMPropertyGeneration `json:"generation,omitempty"`
}

// LLMPropertyGeneration mirrors data/models.PropertyGeneration: how the simulation
// generator produces the property's initial value at t=0. Numeric fields are
// pointer-typed so an omitted field keeps the previous value on update.
type LLMPropertyGeneration struct {
	Mode          string            `json:"mode,omitempty"`         // default | random | llm
	Distribution  string            `json:"distribution,omitempty"` // uniform | normal | choice (mode "random" only)
	Mean          *float64          `json:"mean,omitempty"`
	StdDev        *float64          `json:"std_dev,omitempty"`
	Min           *float64          `json:"min,omitempty"`
	Max           *float64          `json:"max,omitempty"`
	Choices       []LLMChoiceWeight `json:"choices,omitempty"`
	PromptContext string            `json:"prompt_context,omitempty"` // mode "llm" hint
}

// LLMChoiceWeight mirrors data/models.ChoiceWeight.
type LLMChoiceWeight struct {
	Value  interface{} `json:"value"`
	Weight float64     `json:"weight"`
}

// LLMAgentInitialization mirrors data/models.AgentInitialization. PopulationCount
// is pointer-typed so an omitted count keeps the previous value on update.
type LLMAgentInitialization struct {
	PopulationCount *llmUint64    `json:"population_count,omitempty"`
	MinPopulation   *llmUint64    `json:"min_population,omitempty"`
	MaxPopulation   *llmUint64    `json:"max_population,omitempty"`
	InitialState    string        `json:"initial_state,omitempty"`
	Placement       *LLMPlacement `json:"placement,omitempty"`
	CountMode       string        `json:"count_mode,omitempty"` // fixed | random_range | llm
}

// LLMPlacement mirrors data/models.Placement.
type LLMPlacement struct {
	Type          string `json:"type,omitempty"` // random | uniform | clustered | at_shape | manual | llm
	ShapeUID      string `json:"shape_uid,omitempty"`
	PromptContext string `json:"prompt_context,omitempty"`

	Clusters      *llmUint64 `json:"clusters,omitempty"`       // clustered: number of centres
	ClusterRadius *llmUint64 `json:"cluster_radius,omitempty"` // clustered: gaussian spread
}

// ===================== environment + shapes =====================

// LLMEnvironment mirrors the structural fields of data/models.Environment.
// Width/Height/Torus are pointer-typed so an omitted field keeps the previous
// value on update. Includes the descriptive LLMPromptContext.
type LLMEnvironment struct {
	UID    string     `json:"uid,omitempty"`
	Width  *llmUint64 `json:"width,omitempty"`
	Height *llmUint64 `json:"height,omitempty"`
	Torus  *llmBool   `json:"torus,omitempty"`
	// CollisionMode ("none" | "block") selects the world's collision response
	// (U7): "block" makes agents and non-line shapes solid.
	CollisionMode    string             `json:"collision_mode,omitempty"`
	Shapes           []LLMShape         `json:"shapes,omitempty"`
	GlobalProperties []LLMAgentProperty `json:"global_properties,omitempty"`
	LLMPromptContext string             `json:"llm_prompt_context,omitempty"`
}

// LLMShape mirrors the structural fields of data/models.Shape. Excludes
// LLMConfig and Locked. Layer/Visible are pointer-typed so an omitted field
// keeps the previous value on update.
type LLMShape struct {
	UID            string                 `json:"uid"`
	Name           string                 `json:"name,omitempty"`
	Description    string                 `json:"description,omitempty"`
	Type           string                 `json:"type,omitempty"` // rectangle | circle | line | polygon
	Layer          *llmUint               `json:"layer,omitempty"`
	Visible        *llmBool               `json:"visible,omitempty"`
	Properties     []LLMAgentProperty     `json:"properties,omitempty"`
	Initialization LLMAgentInitialization `json:"initialization"`
	Geometry       *LLMShapeGeometry      `json:"geometry,omitempty"`
}

// LLMShapeGeometry mirrors data/models.ShapeGeometry.
type LLMShapeGeometry struct {
	Position    *LLMPoint      `json:"position,omitempty"`
	Width       *LLMScalarSpec `json:"width,omitempty"`
	Height      *LLMScalarSpec `json:"height,omitempty"`
	Radius      *LLMScalarSpec `json:"radius,omitempty"`
	Points      []LLMPoint     `json:"points,omitempty"`
	Thickness   *llmUint       `json:"thickness,omitempty"`
	Fill        *llmBool       `json:"fill,omitempty"`
	Stroke      *llmBool       `json:"stroke,omitempty"`
	StrokeWidth *llmUint       `json:"stroke_width,omitempty"`
}

// LLMScalarSpec mirrors data/models.ScalarSpec (exactly one of value or min+max).
type LLMScalarSpec struct {
	Value *llmUint64 `json:"value,omitempty"`
	Min   *llmUint64 `json:"min,omitempty"`
	Max   *llmUint64 `json:"max,omitempty"`
}

// UnmarshalJSON accepts BOTH the documented object form ({"value":n} /
// {"min":a,"max":b}) AND a bare number or numeric string, which LLMs
// occasionally emit despite the schema doc ("radius": 5, "width": "1.2"). A
// bare number decodes as a fixed value; numeric tolerance (fractional
// rounding, negative clamping) comes from llmUint64 in both forms.
func (s *LLMScalarSpec) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '-' || (trimmed[0] >= '0' && trimmed[0] <= '9')) {
		var v llmUint64
		_ = v.UnmarshalJSON(trimmed) // never errors
		*s = LLMScalarSpec{Value: &v}
		return nil
	}
	// A quoted numeric string is a bare fixed value too ("width": "1.2").
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var v llmUint64
		_ = v.UnmarshalJSON(trimmed)
		*s = LLMScalarSpec{Value: &v}
		return nil
	}
	// Object form (or null / anything else — same errors as before). The local
	// alias type drops the Unmarshaler method, avoiding infinite recursion.
	type llmScalarSpecAlias LLMScalarSpec
	var a llmScalarSpecAlias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = LLMScalarSpec(a)
	return nil
}

// LLMPoint mirrors common.Point.
type LLMPoint struct {
	X llmUint64 `json:"x"`
	Y llmUint64 `json:"y"`
}

// ===================== rules =====================

// LLMRule mirrors the structural fields of data/models.Rule. Excludes Locked.
// Priority/ExecuteOnce/CooldownSteps/Enabled are pointer-typed so an omitted
// field keeps the previous value on update (and Enabled defaults to true on a
// newly created rule).
type LLMRule struct {
	ID            string         `json:"id"`
	Name          string         `json:"name,omitempty"`
	Description   string         `json:"description,omitempty"`
	Priority      *llmInt        `json:"priority,omitempty"`
	AppliesTo     []string       `json:"applies_to,omitempty"`
	Conditions    []LLMCondition `json:"conditions,omitempty"`
	Actions       []LLMAction    `json:"actions,omitempty"`
	ExecuteOnce   *llmBool       `json:"execute_once,omitempty"`
	CooldownSteps *llmInt        `json:"cooldown_steps,omitempty"`
	Enabled       *llmBool       `json:"enabled,omitempty"`
}

// LLMCondition mirrors data/models.Condition.
type LLMCondition struct {
	Type          string                 `json:"type,omitempty"`
	PropertyUID   string                 `json:"property_uid,omitempty"`
	Operator      string                 `json:"operator,omitempty"`
	Value         interface{}            `json:"value,omitempty"`
	MinValue      *float64               `json:"min_value,omitempty"`
	MaxValue      *float64               `json:"max_value,omitempty"`
	Radius        *float64               `json:"radius,omitempty"`
	TargetUID     string                 `json:"target_uid,omitempty"`
	TargetUIDs    []string               `json:"target_uids,omitempty"`
	LogicOperator string                 `json:"logic_operator,omitempty"`
	SubConditions []LLMCondition         `json:"sub_conditions,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	Description   string                 `json:"description,omitempty"`
}

// LLMAction mirrors data/models.Action. Includes the descriptive LLMPromptContext.
type LLMAction struct {
	Type             string                 `json:"type,omitempty"`
	PropertyUID      string                 `json:"property_uid,omitempty"`
	Value            interface{}            `json:"value,omitempty"`
	Modifier         string                 `json:"modifier,omitempty"`
	TargetUID        string                 `json:"target_uid,omitempty"`
	TargetUIDs       []string               `json:"target_uids,omitempty"`
	Speed            *float64               `json:"speed,omitempty"`
	Direction        *float64               `json:"direction,omitempty"`
	EventUID         string                 `json:"event_uid,omitempty"`
	EventData        interface{}            `json:"event_data,omitempty"`
	NewState         string                 `json:"new_state,omitempty"`
	SpawnAgentUID    string                 `json:"spawn_agent_uid,omitempty"`
	SpawnCount       *llmInt                `json:"spawn_count,omitempty"`
	SpawnProperties  map[string]interface{} `json:"spawn_properties,omitempty"`
	SpawnPosition    *LLMSpawnPosition      `json:"spawn_position,omitempty"`
	SpawnInheritFrom []string               `json:"spawn_inherit_from,omitempty"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Description      string                 `json:"description,omitempty"`
	LLMPromptContext string                 `json:"llm_prompt_context,omitempty"`
}

// LLMSpawnPosition mirrors data/models.SpawnPosition.
type LLMSpawnPosition struct {
	Type     string   `json:"type,omitempty"`
	OffsetX  *float64 `json:"offset_x,omitempty"`
	OffsetY  *float64 `json:"offset_y,omitempty"`
	Radius   *float64 `json:"radius,omitempty"`
	ShapeUID string   `json:"shape_uid,omitempty"`
}

// ===================== interactions =====================

// LLMInteraction mirrors the structural fields of data/models.Interaction.
// Excludes Locked. Bidirectional/UseLLM/Enabled are pointer-typed so an omitted
// field keeps the previous value on update (and Enabled defaults to true on a
// newly created interaction). Includes the descriptive LLMPromptContext.
type LLMInteraction struct {
	ID               string         `json:"id"`
	Name             string         `json:"name,omitempty"`
	Description      string         `json:"description,omitempty"`
	Type             string         `json:"type,omitempty"`
	InitiatorUIDs    []string       `json:"initiator_uids,omitempty"`
	TargetUIDs       []string       `json:"target_uids,omitempty"`
	Radius           *float64       `json:"radius,omitempty"`
	Conditions       []LLMCondition `json:"conditions,omitempty"`
	InitiatorActions []LLMAction    `json:"initiator_actions,omitempty"`
	TargetActions    []LLMAction    `json:"target_actions,omitempty"`
	Bidirectional    *llmBool       `json:"bidirectional,omitempty"`
	UseLLM           *llmBool       `json:"use_llm,omitempty"`
	Enabled          *llmBool       `json:"enabled,omitempty"`
	LLMPromptContext string         `json:"llm_prompt_context,omitempty"`
}

// ===================== widgets =====================

// LLMWidget mirrors data/common.WidgetConfig. Widgets are model-level dashboard
// templates and carry NO locked flag, so no lock handling applies. Position is
// pointer-typed so an omitted layout keeps the previous grid cell on update.
type LLMWidget struct {
	ID       string             `json:"id"`
	Type     string             `json:"type,omitempty"` // monitor | line_chart | bar_chart | pie_chart | histogram | agent_table | text_log
	Title    string             `json:"title,omitempty"`
	Source   *LLMWidgetSource   `json:"source,omitempty"`
	Series   []LLMWidgetSeries  `json:"series,omitempty"`
	Columns  []string           `json:"columns,omitempty"`
	Buckets  *llmUint           `json:"buckets,omitempty"`
	LogKinds []string           `json:"log_kinds,omitempty"` // events | state_transitions | llm_decisions
	Position *LLMWidgetPosition `json:"position,omitempty"`
}

// LLMWidgetSource mirrors data/common.WidgetSource.
type LLMWidgetSource struct {
	Type        string   `json:"type,omitempty"` // agent_property | global_property | population | state_distribution | sim_metric
	PropertyUID string   `json:"property_uid,omitempty"`
	AgentUIDs   []string `json:"agent_uids,omitempty"`
	Aggregation string   `json:"aggregation,omitempty"` // avg | sum | min | max | count | median
	// Metric is only for sim_metric sources: total_agents | agents_spawned |
	// agents_removed | interactions | events | rule_executions | movement_distance
	Metric string `json:"metric,omitempty"`
}

// LLMWidgetSeries mirrors data/common.WidgetSeries.
type LLMWidgetSeries struct {
	Label  string          `json:"label,omitempty"`
	Color  string          `json:"color,omitempty"`
	Source LLMWidgetSource `json:"source"`
}

// LLMWidgetPosition mirrors data/common.WidgetPosition.
type LLMWidgetPosition struct {
	X llmUint `json:"x"`
	Y llmUint `json:"y"`
	W llmUint `json:"w"`
	H llmUint `json:"h"`
}

// ===================== events =====================

// LLMEvent mirrors the structural fields of data/models.EventDefinition.
// Excludes LLMTarget and Locked. TTLSteps is pointer-typed so an omitted value
// keeps the previous one on update. Includes the descriptive LLMPromptContext.
type LLMEvent struct {
	UID              string             `json:"uid"`
	Name             string             `json:"name,omitempty"`
	Description      string             `json:"description,omitempty"`
	Scope            string             `json:"scope,omitempty"`
	Radius           *float64           `json:"radius,omitempty"`
	Payload          []LLMAgentProperty `json:"payload,omitempty"`
	Producers        []string           `json:"producers,omitempty"`
	ProducerUIDs     []string           `json:"producer_uids,omitempty"`
	ConsumerUIDs     []string           `json:"consumer_uids,omitempty"`
	TTLSteps         *llmUint           `json:"ttl_steps,omitempty"`
	LLMPromptContext string             `json:"llm_prompt_context,omitempty"`
}
