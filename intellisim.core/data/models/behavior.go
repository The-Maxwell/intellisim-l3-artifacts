package models

// ConditionType defines the type of condition in rules.
type ConditionType string

const (
	ConditionTypeGlobalProperty ConditionType = "global_property" // Check global property
	ConditionTypeProperty       ConditionType = "property"        // Check agent property
	ConditionTypePosition       ConditionType = "position"        // Check spatial position
	ConditionTypeProximity      ConditionType = "proximity"       // Check nearby agents/shapes
	ConditionTypeStep           ConditionType = "step"            // Check simulation step number
	ConditionTypeComposite      ConditionType = "composite"       // AND/OR/NOT combination
	ConditionTypeCount          ConditionType = "count"           // Count agents in area
	ConditionTypeBarrier        ConditionType = "barrier"         // Check if movement is blocked
	ConditionTypeState          ConditionType = "state"           // Check agent state
	ConditionTypeDistance       ConditionType = "distance"        // Check distance to target
	ConditionTypeDecision       ConditionType = "decision"        // Check LLM decision output (strategies 4/5)
	ConditionTypeRandom         ConditionType = "random"          // Stochastic / probability gate
	ConditionTypeMemory         ConditionType = "memory"          // Check agent memory / history
	ConditionTypeEvent          ConditionType = "event"           // React to an emitted EventDefinition
	ConditionTypeCustom         ConditionType = "custom"          // User-defined custom condition
	ConditionTypePopulation     ConditionType = "population"      // Live agent count, total or per archetype (U2 termination)
)

// ConditionOperator defines comparison operators.
type ConditionOperator string

const (
	ConditionOpEquals      ConditionOperator = "eq"
	ConditionOpNotEquals   ConditionOperator = "neq"
	ConditionOpGreaterThan ConditionOperator = "gt"
	ConditionOpLessThan    ConditionOperator = "lt"
	ConditionOpGreaterOrEq ConditionOperator = "gte"
	ConditionOpLessOrEq    ConditionOperator = "lte"
	ConditionOpContains    ConditionOperator = "contains"
	ConditionOpInRange     ConditionOperator = "in_range"
	ConditionOpIn          ConditionOperator = "in"     // membership in a set
	ConditionOpNotIn       ConditionOperator = "not_in" // non-membership
)

// LogicOperator for composite conditions.
type LogicOperator string

const (
	LogicOpAND LogicOperator = "AND"
	LogicOpOR  LogicOperator = "OR"
	LogicOpNOT LogicOperator = "NOT"
)

// Reserved pseudo-property UIDs usable as Condition.PropertyUID / Action.PropertyUID for
// built-in attributes that are not user-defined AgentProperty entries.
const (
	PseudoPropertyState    = "state"    // agent state-machine state (ConditionTypeState / ActionTypeTransition)
	PseudoPropertyPosition = "position" // agent spatial position (ConditionTypePosition)
)

// Condition represents a condition in a rule. Entity references use UIDs.
// For ConditionTypeEvent, TargetUID holds the EventDefinition UID and an optional
// PropertyUID+Operator+Value match a payload field.
type Condition struct {
	Type          ConditionType
	PropertyUID   string            // property UID or reserved pseudo-prop ("state","position")
	Operator      ConditionOperator
	Value         interface{}
	MinValue      *float64
	MaxValue      *float64
	Radius        *float64
	TargetUID     string
	TargetUIDs    []string
	LogicOperator LogicOperator
	SubConditions []Condition

	// Universal parameters for flexible, user-defined conditions.
	Parameters map[string]interface{}

	Description string
}

// ActionType defines the type of action in rules.
type ActionType string

const (
	ActionTypeSetProperty    ActionType = "set_property"
	ActionTypeModifyProperty ActionType = "modify_property"
	ActionTypeMove           ActionType = "move"
	ActionTypeMoveToward     ActionType = "move_toward"
	ActionTypeMoveAway       ActionType = "move_away"
	ActionTypeInteract       ActionType = "interact"
	ActionTypeEmitEvent      ActionType = "emit_event"
	ActionTypeTransition     ActionType = "transition" // State transition
	ActionTypeCallLLM        ActionType = "call_llm"   // Trigger LLM decision
	ActionTypeSpawnAgent     ActionType = "spawn_agent"
	ActionTypeRemoveAgent    ActionType = "remove_agent"
	ActionTypeTransform      ActionType = "transform" // Change agent type
	ActionTypeTeleport       ActionType = "teleport"
	ActionTypeCustom         ActionType = "custom"
)

// PropertyModifier defines how to modify a property.
type PropertyModifier string

const (
	PropertyModifierSet       PropertyModifier = "set"
	PropertyModifierAdd       PropertyModifier = "add"
	PropertyModifierSubtract  PropertyModifier = "subtract"
	PropertyModifierMultiply  PropertyModifier = "multiply"
	PropertyModifierDivide    PropertyModifier = "divide"
	PropertyModifierIncrement PropertyModifier = "increment"
	PropertyModifierDecrement PropertyModifier = "decrement"
	PropertyModifierToggle    PropertyModifier = "toggle" // boolean
)

// Action represents an action in a rule. Entity references use UIDs.
type Action struct {
	Type        ActionType
	PropertyUID string
	Value       interface{}
	Modifier    PropertyModifier
	TargetUID   string
	TargetUIDs  []string
	Speed       *float64
	Direction   *float64
	EventUID    string      // references an EventDefinition
	EventData   interface{}
	NewState    string

	// Agent creation during simulation.
	SpawnAgentUID    string                 // archetype UID
	SpawnCount       int
	SpawnProperties  map[string]interface{}
	SpawnPosition    *SpawnPosition
	SpawnInheritFrom []string               // property UIDs

	Parameters       map[string]interface{}
	Description      string
	LLMPromptContext string
}

// SpawnPositionType defines how to calculate spawn position.
type SpawnPositionType string

const (
	SpawnPositionSame    SpawnPositionType = "same"
	SpawnPositionRandom  SpawnPositionType = "random"
	SpawnPositionOffset  SpawnPositionType = "offset"
	SpawnPositionNearby  SpawnPositionType = "nearby"
	SpawnPositionAtShape SpawnPositionType = "at_shape"
)

type SpawnPosition struct {
	Type     SpawnPositionType
	OffsetX  *float64
	OffsetY  *float64
	Radius   *float64
	ShapeUID string // for at_shape
}

// Rule represents a condition-action rule for agent behavior. ID is a model-local rule
// identifier (referenced at runtime via RuleState.rule_id); it is distinct from the entity
// "uid" namespace used to target agents/shapes/properties.
type Rule struct {
	ID          string
	Name        string
	Description string
	Priority    int      // higher priority rules execute first
	AppliesTo   []string // agent/archetype UIDs
	// Conditions are AND-combined; use ConditionTypeComposite for OR/NOT logic.
	Conditions    []Condition
	Actions       []Action
	ExecuteOnce   bool
	CooldownSteps int
	Enabled       bool
	Locked        bool // freezes this rule against LLM edits (user-only)
}

// InteractionType defines the type of interaction.
type InteractionType string

const (
	InteractionTypeAgentAgent  InteractionType = "agent_agent"
	InteractionTypeAgentShape  InteractionType = "agent_shape"
	InteractionTypeAgentGlobal InteractionType = "agent_global"
)

// Interaction defines how agents interact with each other or the environment. ID is a
// model-local interaction identifier (distinct from the entity "uid" namespace).
// InitiatorUIDs/TargetUIDs reference agent/archetype/shape UIDs.
type Interaction struct {
	ID               string
	Name             string
	Description      string
	Type             InteractionType
	InitiatorUIDs    []string
	TargetUIDs       []string
	Radius           *float64
	Conditions       []Condition
	InitiatorActions []Action
	TargetActions    []Action
	Bidirectional    bool
	UseLLM           bool
	Enabled          bool
	Locked           bool // freezes this interaction against LLM edits (user-only)

	LLMPromptContext string
}
