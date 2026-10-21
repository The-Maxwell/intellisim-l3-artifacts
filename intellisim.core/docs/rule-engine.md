# Rule Engine Reference

This document provides complete reference for IntelliSim's rule engine, including all 15 condition types, 14 action types, evaluation flow, and interaction processing.

---

## Overview

The rule engine is the core behavioral system in IntelliSim. Rules enable flexible, data-driven agent behavior without LLM involvement.

**Key Concepts**:
- **Rule**: Condition-action pair that defines agent behavior
- **Condition**: Logical check on agent/environment state
- **Action**: State modification or behavior command
- **Evaluation**: Rules are evaluated in priority order
- **Execution**: Actions are executed immediately when conditions are met

**When Rules Execute**:
1. Every simulation step, for each agent
2. Rules apply to agents matching their `AppliesTo` types
3. Conditions are evaluated (AND logic)
4. If all conditions true, all actions execute
5. Rule execution state is tracked (for execute_once, cooldown)

---

## Condition Types

The rule engine supports 10 condition types for flexible behavioral specification:

### 1. Property (property)

**Type Key**: `"property"`

**Check**: Agent property value against expected value/range

**Fields**:
- `property` (string): Property name to check (e.g., "energy", "age")
- `operator` (ConditionOperator): Comparison operator
- `value` (interface{}): Value to compare against
- `min_value` (*float64): Minimum value (for range checks)
- `max_value` (*float64): Maximum value (for range checks)

**Operators**:
- `"eq"` — Equals
- `"neq"` — Not equals
- `"gt"` — Greater than
- `"lt"` — Less than
- `"gte"` — Greater than or equal
- `"lte"` — Less than or equal
- `"in_range"` — Between min_value and max_value
- `"contains"` — String contains substring

**Example JSON**:
```json
{
  "type": "property",
  "property": "energy",
  "operator": "lt",
  "value": 30
}
```

Evaluates to true if agent.Properties["energy"] < 30

---

### 2. Position (position)

**Type Key**: `"position"`

**Check**: Agent's spatial position in environment

**Fields**:
- `operator` (ConditionOperator): Comparison operator
- `min_value` (*float64): Minimum coordinate
- `max_value` (*float64): Maximum coordinate

**Operators**:
- `"in_range"` — Agent X or Y within min/max (primary use)

**Example JSON**:
```json
{
  "type": "position",
  "operator": "in_range",
  "min_value": 0,
  "max_value": 100
}
```

Evaluates to true if agent.Position.X or Y is between 0-100

---

### 3. Proximity (proximity)

**Type Key**: `"proximity"`

**Check**: Nearby agents/shapes within radius

**Fields**:
- `radius` (*float64): Search radius
- `target_type` (string): Agent type or shape name to check for (e.g., "predator", "food")
- `value` (interface{}): Expected count for comparison
- `operator` (ConditionOperator): Comparison for count

**Operators**:
- Count comparison: `"eq"`, `"neq"`, `"gt"`, `"lt"`, `"gte"`, `"lte"`
- Default: `count > 0` if no value specified

**Example JSON**:
```json
{
  "type": "proximity",
  "radius": 50.0,
  "target_type": "predator",
  "operator": "gte",
  "value": 2
}
```

Evaluates to true if 2+ predators within radius 50

---

### 4. Time (time)

**Type Key**: `"time"`

**Check**: Simulation step/time conditions

**Fields**:
- `operator` (ConditionOperator): Comparison operator
- `value` (interface{}): Step number to compare
- `min_value` (*float64): Minimum step
- `max_value` (*float64): Maximum step

**Operators**:
- `"eq"`, `"neq"`, `"gt"`, `"lt"`, `"gte"`, `"lte"`, `"in_range"`

**Example JSON**:
```json
{
  "type": "time",
  "operator": "gte",
  "value": 100
}
```

Evaluates to true if current step >= 100

---

### 5. Composite (composite)

**Type Key**: `"composite"`

**Check**: Logical combination of multiple conditions (AND/OR/NOT)

**Fields**:
- `logic_operator` (LogicOperator): `"AND"`, `"OR"`, `"NOT"`
- `sub_conditions` ([]Condition): Nested conditions to combine

**Operators**:
- `"AND"` — All sub-conditions must be true (default)
- `"OR"` — At least one sub-condition must be true
- `"NOT"` — Negate first sub-condition

**Example JSON**:
```json
{
  "type": "composite",
  "logic_operator": "AND",
  "sub_conditions": [
    {
      "type": "property",
      "property": "energy",
      "operator": "lt",
      "value": 30
    },
    {
      "type": "proximity",
      "radius": 100,
      "target_type": "food",
      "value": 1
    }
  ]
}
```

Evaluates to true if energy < 30 AND food nearby

---

### 6. Count (count)

**Type Key**: `"count"`

**Check**: Number of agents in radius matching criteria

**Fields**:
- `radius` (*float64): Search radius
- `target_type` (string): Agent type to count
- `operator` (ConditionOperator): Count comparison
- `value` (interface{}): Expected count

**Operators**:
- `"eq"`, `"neq"`, `"gt"`, `"lt"`, `"gte"`, `"lte"`

**Example JSON**:
```json
{
  "type": "count",
  "radius": 200.0,
  "target_type": "prey",
  "operator": "gt",
  "value": 5
}
```

Evaluates to true if >5 prey agents within radius 200

---

### 7. Barrier (barrier)

**Type Key**: `"barrier"`

**Check**: Agent at environment boundary or near obstacle

**Fields**:
- `target_type` (string, optional): Specific shape name to check for

**Operators**:
- None (implicit check)

**Example JSON**:
```json
{
  "type": "barrier",
  "target_type": "wall"
}
```

Evaluates to true if agent is at world boundary (unless torus) or near wall shape

---

### 8. State (state)

**Type Key**: `"state"`

**Check**: Agent's current state machine state

**Fields**:
- `operator` (ConditionOperator): Comparison
- `value` (interface{}): Expected state name

**Operators**:
- `"eq"`, `"neq"`

**Example JSON**:
```json
{
  "type": "state",
  "operator": "eq",
  "value": "resting"
}
```

Evaluates to true if agent.State == "resting"

---

### 9. Distance (distance)

**Type Key**: `"distance"`

**Check**: Distance to nearest target matches criteria

**Fields**:
- `radius` (*float64): Search radius for candidates
- `target_type` (string): Agent type or shape to measure to
- `operator` (ConditionOperator): Distance comparison
- `value` (interface{}): Expected distance
- `min_value` (*float64): Minimum distance
- `max_value` (*float64): Maximum distance

**Operators**:
- `"eq"`, `"neq"`, `"gt"`, `"lt"`, `"gte"`, `"lte"`, `"in_range"`

**Example JSON**:
```json
{
  "type": "distance",
  "radius": 500.0,
  "target_type": "food",
  "operator": "lt",
  "value": 50
}
```

Evaluates to true if nearest food is <50 units away (within search radius 500)

---

### 10. Custom (custom)

**Type Key**: `"custom"`

**Check**: User-defined condition using Parameters map

**Fields**:
- `parameters` (map[string]interface{}): Custom parameters
  - Standard pattern: `"property"`, `"operator"`, `"value"`
  - Can be extended for custom logic

**Operators**:
- Depends on custom implementation

**Example JSON**:
```json
{
  "type": "custom",
  "parameters": {
    "property": "hunger",
    "operator": "gt",
    "value": 75
  }
}
```

Evaluates using custom logic in rule engine

---

### 11. Decision (decision) [Special]

**Type Key**: `"decision"`

**Check**: LLM decision from Strategies 4-5 in DecisionCache

**Fields**:
- `property` (string): Decision key to look up in cache
- `operator` (ConditionOperator): Value comparison
- `value` (interface{}): Expected decision value

**Operators**:
- `"eq"`, `"neq"`, `"gt"`, `"lt"`, etc.

**Example JSON**:
```json
{
  "type": "decision",
  "property": "action",
  "operator": "eq",
  "value": "hunt"
}
```

Evaluates to true if runtime.DecisionCache[agentID]["action"] == "hunt"

**Note**: Used in Strategies 4 & 5 (Decision-only) to bridge LLM decisions to rule actions.

---

## Universal Parameters

All conditions support a `parameters` map for extensibility without code changes:

```go
Parameters map[string]interface{} `bson:"parameters,omitempty"`
```

**Common Patterns**:
- Property-based: `{"property": "X", "operator": "op", "value": V}`
- Custom metadata: `{"description": "...", "source": "..."}`

---

## Action Types

The rule engine supports 14 action types for flexible behavior execution:

### 1. Set Property (set_property)

**Type Key**: `"set_property"`

**Effect**: Set property to exact value

**Fields**:
- `property` (string): Property name
- `value` (interface{}): New value

**Example JSON**:
```json
{
  "type": "set_property",
  "property": "energy",
  "value": 100
}
```

Result: agent.Properties["energy"] = 100

---

### 2. Modify Property (modify_property)

**Type Key**: `"modify_property"`

**Effect**: Modify property using operation (add, subtract, multiply, divide)

**Fields**:
- `property` (string): Property name
- `modifier` (PropertyModifier): Operation type
- `value` (interface{}): Operand (not needed for increment/decrement)

**Modifiers**:
- `"add"` — Add value
- `"subtract"` — Subtract value
- `"multiply"` — Multiply by value
- `"divide"` — Divide by value
- `"increment"` — Add 1 (value ignored)
- `"decrement"` — Subtract 1 (value ignored)

**Example JSON**:
```json
{
  "type": "modify_property",
  "property": "energy",
  "modifier": "subtract",
  "value": 5
}
```

Result: agent.Properties["energy"] -= 5

---

### 3. Move (move)

**Type Key**: `"move"`

**Effect**: Move agent in random or specified direction

**Fields**:
- `speed` (*float64): Distance to move (default: 1.0)
- `direction` (*float64): Direction in radians (default: random)

**Example JSON**:
```json
{
  "type": "move",
  "speed": 2.5
}
```

Result: Agent moves 2.5 units in random direction

---

### 4. Move Toward (move_toward)

**Type Key**: `"move_toward"`

**Effect**: Move agent toward nearest target (agent or shape)

**Fields**:
- `target_type` (string): Agent type or shape name
- `speed` (*float64): Distance to move (default: 1.0)

**Example JSON**:
```json
{
  "type": "move_toward",
  "target_type": "food",
  "speed": 3.0
}
```

Result: Agent moves 3.0 units toward nearest food

---

### 5. Move Away (move_away)

**Type Key**: `"move_away"`

**Effect**: Move agent away from nearest target

**Fields**:
- `target_type` (string): Agent type or shape name
- `speed` (*float64): Distance to move (default: 1.0)

**Example JSON**:
```json
{
  "type": "move_away",
  "target_type": "predator",
  "speed": 2.0
}
```

Result: Agent moves 2.0 units away from nearest predator

---

### 6. Interact (interact)

**Type Key**: `"interact"`

**Effect**: Force-fire a named interaction for this agent in the interactions
phase this tick. The interaction's own `Conditions` and `UseLLM` mediation are
SKIPPED (the firing rule already made that decision); geometric eligibility is
kept — `agent_agent` fires against the nearest eligible target within the
resolved radius, `agent_shape` against the first overlapping target shape,
`agent_global` always. Invalid requests (unknown/disabled interaction,
non-initiator archetype, no eligible target in range) warn once and no-op. Each
(agent, interaction) pair fires at most once per step.

**Fields**:
- `target_uid` (string): the `Interaction.ID` to force-fire

**Example JSON**:
```json
{
  "type": "interact",
  "target_uid": "hunt"
}
```

Result: the "hunt" interaction fires for this agent against the nearest eligible target

---

### 7. Emit Event (emit_event)

**Type Key**: `"emit_event"`

**Effect**: Emit named event for other agents to react to

**Fields**:
- `event_name` (string): Event identifier
- `event_data` (interface{}): Event payload

**Example JSON**:
```json
{
  "type": "emit_event",
  "event_name": "alarm",
  "event_data": {"severity": "high"}
}
```

Result: Event stored in agent memory or broadcast to nearby agents

---

### 8. Transition (transition)

**Type Key**: `"transition"`

**Effect**: Change agent state machine state

**Fields**:
- `new_state` (string): Target state

**Example JSON**:
```json
{
  "type": "transition",
  "new_state": "resting"
}
```

Result: agent.State = "resting"

---

### 9. Call LLM (call_llm)

**Type Key**: `"call_llm"`

**Effect**: Request an ON-DEMAND LLM decision for this agent, consumed by the
NEXT step's LLM phase (bypassing the strategy's `DecisionFrequency` gate for the
decision purpose only). Requires the agent's resolved LLM strategy to be
decision-capable (non-none with `UseForDecisions`); otherwise the action warns
once and no-ops. The request is consume-once even if the resulting LLM call
fails (LastDecision replay / rules cover). A "call_llm requested" decision
memory event is recorded.

**Fields**: none (the agent's own strategy/target/prompt-context drive the call).

**Example JSON**:
```json
{
  "type": "call_llm"
}
```

Result: the agent is included in the next step's decision-purpose LLM call even
when the frequency gate is closed; the returned label flows through the normal
decision-condition machinery

---

### 10. Spawn Agent (spawn_agent)

**Type Key**: `"spawn_agent"`

**Effect**: Create new agent(s) during simulation

**Fields**:
- `spawn_agent_type` (string): Archetype to spawn
- `spawn_count` (int): Number to create (default: 1)
- `spawn_properties` (map[string]interface{}): Initial properties
- `spawn_position` (*SpawnPosition): Where to spawn
- `spawn_inherit_from` ([]string): Properties to inherit from parent

**SpawnPosition**:
- `type` (SpawnPositionType): "same", "random", "offset", "nearby", "at_shape"
- `offset_x`, `offset_y`: For offset type
- `radius`: For nearby type
- `shape_name`: For at_shape type

**Example JSON**:
```json
{
  "type": "spawn_agent",
  "spawn_agent_type": "offspring",
  "spawn_count": 2,
  "spawn_properties": {"energy": 50},
  "spawn_position": {
    "type": "nearby",
    "radius": 20
  },
  "spawn_inherit_from": ["age"]
}
```

Result: 2 new agents of type "offspring" created nearby, inheriting age from parent

---

### 11. Remove Agent (remove_agent)

**Type Key**: `"remove_agent"`

**Effect**: Remove (kill) the agent

**Fields**:
- None

**Example JSON**:
```json
{
  "type": "remove_agent"
}
```

Result: Agent is marked dead, removed from active simulation

---

### 12. Transform (transform)

**Type Key**: `"transform"`

**Effect**: Turn the agent into another archetype. At tick end (spawn/remove phase) a replacement agent of the target archetype is created — by default at the source agent's position — and the source agent is removed through the normal removal path (stats count one spawn of the target archetype and one removal of the source archetype). The replacement gets a fresh instance id and participates from the next tick.

**Fields**:
- `spawn_agent_uid` (string) or `target_uid` (string): Target archetype UID (spawn_agent_uid wins when both set)
- `spawn_properties` (map, optional): Explicit property overrides for the replacement
- `spawn_inherit_from` ([]string, optional): Source property UIDs copied to the replacement
- `spawn_position` (object, optional): Placement directive (same/random/offset/nearby/at_shape); defaults to the source's position

**Example JSON**:
```json
{
  "type": "transform",
  "target_uid": "butterfly",
  "spawn_inherit_from": ["energy"]
}
```

Result: The agent is replaced by a new "butterfly" agent at its position (inheriting `energy`); the original agent is removed the same tick.

**Edge cases**: If the source was already removed this tick (another rule/interaction), the transform is skipped (no orphan replacement). If the spawn fails (unknown target archetype), the original agent stays alive.

---

### 13. Teleport (teleport)

**Type Key**: `"teleport"`

**Effect**: Instantly move agent to specific location

**Fields**:
- `value` (interface{}): Target position (X, Y or {x, y})

**Example JSON**:
```json
{
  "type": "teleport",
  "value": {"x": 250, "y": 300}
}
```

Result: agent.Position = {250, 300} (bypassing barriers)

---

### 14. Custom (custom)

**Type Key**: `"custom"`

**Effect**: User-defined action using Parameters map

**Fields**:
- `parameters` (map[string]interface{}): Custom parameters

**Example JSON**:
```json
{
  "type": "custom",
  "parameters": {
    "action_name": "special_behavior",
    "intensity": 0.8
  }
}
```

Result: Custom logic processes parameters

---

## Rule Evaluation Flow

The rule evaluation process is deterministic and reproducible:

```
For each agent in simulation:
  ├─ Get agent's archetype definition
  ├─ Get applicable rules (rules.AppliesTo includes agent type)
  ├─ Sort rules by priority (descending)
  │
  └─ For each rule (in priority order):
      ├─ Check rule.Enabled
      ├─ Check rule execution state:
      │  ├─ If ExecuteOnce=true and already completed → skip
      │  └─ If CooldownSteps > 0 and not enough steps elapsed → skip
      │
      ├─ Evaluate rule.Conditions (AND logic)
      │  ├─ For each condition:
      │  │  ├─ Call RuleEngine.EvaluateCondition(cond, agent, runtime)
      │  │  ├─ If false, short-circuit: SKIP THIS RULE
      │  │  └─ Continue to next condition
      │  └─ All conditions passed
      │
      ├─ Execute rule.Actions (in order)
      │  ├─ For each action:
      │  │  └─ Call RuleEngine.ExecuteAction(action, agent, runtime)
      │  └─ All actions complete
      │
      ├─ Update rule execution state:
      │  ├─ Set LastExecution = current step
      │  ├─ Increment ExecutionCount
      │  ├─ If ExecuteOnce, set Completed = true
      │  └─ Store in agent.RuleExecutionState
      │
      └─ Continue to next rule (or end if execute_once limit hit per agent)

All agents now have updated state for this step
```

---

## Condition Evaluation Details

Each condition type has specialized evaluation logic:

### Property Evaluation
- Retrieve agent.Properties[property]
- Convert to comparable type
- Apply operator

### Proximity Evaluation
- Use SpatialIndex if available (O(1) radius query)
- Otherwise: O(n) scan of all agents
- Count agents within radius matching target_type
- Compare count to threshold

### Time Evaluation
- Evaluate as: EvaluateCondition(runtime.CurrentStep, operator, value)

### Composite Evaluation
- For AND: return false on first failed sub-condition (short-circuit)
- For OR: return true on first passed sub-condition (short-circuit)
- For NOT: return !EvaluateCondition(subConditions[0])

### Distance Evaluation
- Find nearest target (agent or shape)
- Calculate Euclidean distance
- Compare to threshold

---

## Interaction Processing

**File**: `modules/simulations/strategy.go`

Interactions define agent-to-agent or agent-to-shape interactions with automatic action execution.

### Interaction Types

**Type Enum: InteractionType**:
- `"agent_agent"` — Between two agents
- `"agent_shape"` — Agent with environmental shape
- `"agent_global"` — Agent with global state

### Interaction Structure

```go
type Interaction struct {
    ID               string          `bson:"id"`
    Name             string          `bson:"name"`
    Description      string          `bson:"description,omitempty"`
    Type             InteractionType `bson:"type"`
    InitiatorTypes   []string        `bson:"initiator_types"`
    TargetTypes      []string        `bson:"target_types,omitempty"`
    Radius           *float64        `bson:"radius,omitempty"`
    Conditions       []Condition     `bson:"conditions,omitempty"`
    InitiatorActions []Action        `bson:"initiator_actions,omitempty"`
    TargetActions    []Action        `bson:"target_actions,omitempty"`
    Bidirectional    bool            `bson:"bidirectional"`
    UseLLM           bool            `bson:"use_llm"`
    LLMPromptHint    string          `bson:"llm_prompt_hint,omitempty"`
    Enabled          bool            `bson:"enabled"`
}
```

### Rule-Based Interaction Flow

```
For each interaction:
  ├─ If !interaction.Enabled or interaction.UseLLM → skip (handle separately)
  │
  ├─ For each agent (initiator candidate):
  │  ├─ Check InitiatorTypes match
  │  ├─ Evaluate interaction.Conditions
  │  │  └─ All must be true
  │  │
  │  └─ If conditions met:
  │     ├─ If bidirectional: apply InitiatorActions to initiator
  │     └─ Else: apply both Initiator and Target actions
  │        (no specific target matching, affects all agents in range)
```

### LLM-Based Interaction Flow

```
For each interaction:
  ├─ If !interaction.UseLLM → skip (handle as rule-based)
  │
  ├─ For each agent (initiator):
  │  ├─ Check InitiatorTypes match
  │  ├─ Evaluate interaction.Conditions
  │  │
  │  ├─ If conditions met:
  │  │  ├─ For each potential target (nearby, matching TargetTypes):
  │  │  │  ├─ Check radius distance
  │  │  │  ├─ Build mediation prompt (buildInteractionPrompt, llm_runtime.go)
  │  │  │  ├─ Call LLM (MediateInteraction; fail-open fire=true)
  │  │  │  ├─ Parse LLM response {"fire": bool, "params": {...}}
  │  │  │  ├─ If fire: apply InitiatorActions
  │  │  │  ├─ If fire: apply TargetActions
  │  │  │  └─ Break (one target per initiator per step)
```

### Processing Order

From `modules/simulations/strategy.go`:

- **Strategy 1 (None)**: `ProcessInteractions()` — rule-based only
- **Strategies 2, 3, 6**: `ProcessInteractionsWithLLM()` — all interactions (rule-based + LLM)
- **Strategies 4, 5**: `ProcessInteractions()` — rule-based only

---

## Rule Configuration Example

**Complete Rule Definition** (JSON):

```json
{
  "id": "hunt_when_hungry",
  "name": "Hunt When Hungry",
  "description": "Predator hunts prey when energy is low",
  "priority": 10,
  "applies_to": ["predator"],
  "enabled": true,
  "execute_once": false,
  "cooldown_steps": 0,
  "conditions": [
    {
      "type": "property",
      "property": "energy",
      "operator": "lt",
      "value": 40
    },
    {
      "type": "proximity",
      "radius": 100,
      "target_type": "prey",
      "operator": "gte",
      "value": 1
    }
  ],
  "actions": [
    {
      "type": "move_toward",
      "target_type": "prey",
      "speed": 2.0
    },
    {
      "type": "modify_property",
      "property": "energy",
      "modifier": "subtract",
      "value": 1
    }
  ]
}
```

**Execution**:
1. Agent matches type "predator"
2. Condition 1: energy < 40? (check agent.Properties["energy"])
3. Condition 2: >=1 prey within 100 units?
4. If both true:
   - Action 1: Move toward prey at speed 2.0
   - Action 2: Decrease energy by 1

---

## Performance Considerations

### Spatial Index

For proximity, distance, count conditions:
- Backend uses SpatialIndex for O(1) neighbor lookup
- Falls back to O(n) scan if index unavailable
- Reduces rule evaluation overhead for large populations

### Rule Priority

- Higher priority rules run first
- Use priority to control execution order
- Example: "flee from predator" (priority 100) before "hunt prey" (priority 10)

### Execute Once & Cooldown

- `execute_once=true`: Rule fires at most once per agent per simulation
- `cooldown_steps=N`: Rule can't fire again for N steps
- Useful for expensive operations (spawn agents, remove agents)

### Condition Short-Circuiting

- Composite AND: stops at first false condition
- Composite OR: stops at first true condition
- Reduces evaluation cost

---

## Related Documentation

- [Data Model Reference](data-model.md) — Rule, Condition, Action, Interaction definitions
- [LLM Integration Reference](llm-integration.md) — call_llm action, decision conditions
