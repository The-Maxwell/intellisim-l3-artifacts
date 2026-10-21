package models

import (
	"intelli.sim.core/data/models"
)

// ConditionDocument represents a condition in a rule. Entity references use UIDs.
// For ConditionTypeEvent, TargetUID holds the EventDefinition UID and an optional
// PropertyUID+Operator+Value match a payload field.
type ConditionDocument struct {
	Type          models.ConditionType     `bson:"type"`
	PropertyUID   string                   `bson:"property_uid,omitempty"` // property UID or reserved pseudo-prop ("state","position")
	Operator      models.ConditionOperator `bson:"operator,omitempty"`
	Value         interface{}              `bson:"value,omitempty"`
	MinValue      *float64                 `bson:"min_value,omitempty"`
	MaxValue      *float64                 `bson:"max_value,omitempty"`
	Radius        *float64                 `bson:"radius,omitempty"`
	TargetUID     string                   `bson:"target_uid,omitempty"`
	TargetUIDs    []string                 `bson:"target_uids,omitempty"`
	LogicOperator models.LogicOperator     `bson:"logic_operator,omitempty"`
	SubConditions []ConditionDocument      `bson:"sub_conditions,omitempty"`

	// Universal parameters for flexible, user-defined conditions.
	Parameters map[string]interface{} `bson:"parameters,omitempty"`

	Description string `bson:"description,omitempty"`
}

// ActionDocument represents an action in a rule. Entity references use UIDs.
type ActionDocument struct {
	Type        models.ActionType       `bson:"type"`
	PropertyUID string                  `bson:"property_uid,omitempty"`
	Value       interface{}             `bson:"value,omitempty"`
	Modifier    models.PropertyModifier `bson:"modifier,omitempty"`
	TargetUID   string                  `bson:"target_uid,omitempty"`
	TargetUIDs  []string                `bson:"target_uids,omitempty"`
	Speed       *float64                `bson:"speed,omitempty"`
	Direction   *float64                `bson:"direction,omitempty"`
	EventUID    string                  `bson:"event_uid,omitempty"` // references an EventDefinition
	EventData   interface{}             `bson:"event_data,omitempty"`
	NewState    string                  `bson:"new_state,omitempty"`

	// Agent creation during simulation.
	SpawnAgentUID    string                 `bson:"spawn_agent_uid,omitempty"` // archetype UID
	SpawnCount       int                    `bson:"spawn_count,omitempty"`
	SpawnProperties  map[string]interface{} `bson:"spawn_properties,omitempty"`
	SpawnPosition    *SpawnPositionDocument `bson:"spawn_position,omitempty"`
	SpawnInheritFrom []string               `bson:"spawn_inherit_from,omitempty"` // property UIDs

	Parameters       map[string]interface{} `bson:"parameters,omitempty"`
	Description      string                 `bson:"description,omitempty"`
	LLMPromptContext string                 `bson:"llm_prompt_context,omitempty"`
}

// SpawnPositionDocument defines how to calculate spawn position.
type SpawnPositionDocument struct {
	Type     models.SpawnPositionType `bson:"type"`
	OffsetX  *float64                 `bson:"offset_x,omitempty"`
	OffsetY  *float64                 `bson:"offset_y,omitempty"`
	Radius   *float64                 `bson:"radius,omitempty"`
	ShapeUID string                   `bson:"shape_uid,omitempty"` // for at_shape
}

// RuleDocument represents a condition-action rule for agent behavior. ID is a model-local rule
// identifier (referenced at runtime via RuleState.rule_id); it is distinct from the entity
// "uid" namespace used to target agents/shapes/properties.
type RuleDocument struct {
	ID          string   `bson:"id"`
	Name        string   `bson:"name"`
	Description string   `bson:"description,omitempty"`
	Priority    int      `bson:"priority"`   // higher priority rules execute first
	AppliesTo   []string `bson:"applies_to"` // agent/archetype UIDs
	// Conditions are AND-combined; use ConditionTypeComposite for OR/NOT logic.
	Conditions    []ConditionDocument `bson:"conditions"`
	Actions       []ActionDocument    `bson:"actions"`
	ExecuteOnce   bool                `bson:"execute_once"`
	CooldownSteps int                 `bson:"cooldown_steps,omitempty"`
	Enabled       bool                `bson:"enabled"`
	Locked        bool                `bson:"locked,omitempty"`
}

// InteractionDocument defines how agents interact with each other or the environment. ID is a
// model-local interaction identifier (distinct from the entity "uid" namespace).
// InitiatorUIDs/TargetUIDs reference agent/archetype/shape UIDs.
type InteractionDocument struct {
	ID               string                 `bson:"id"`
	Name             string                 `bson:"name"`
	Description      string                 `bson:"description,omitempty"`
	Type             models.InteractionType `bson:"type"`
	InitiatorUIDs    []string               `bson:"initiator_uids"`
	TargetUIDs       []string               `bson:"target_uids,omitempty"`
	Radius           *float64               `bson:"radius,omitempty"`
	Conditions       []ConditionDocument    `bson:"conditions,omitempty"`
	InitiatorActions []ActionDocument       `bson:"initiator_actions,omitempty"`
	TargetActions    []ActionDocument       `bson:"target_actions,omitempty"`
	Bidirectional    bool                   `bson:"bidirectional"`
	UseLLM           bool                   `bson:"use_llm"`
	Enabled          bool                   `bson:"enabled"`
	Locked           bool                   `bson:"locked,omitempty"`

	LLMPromptContext string `bson:"llm_prompt_context,omitempty"`
}
