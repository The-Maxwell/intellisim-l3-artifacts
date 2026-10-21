# Simulation Engine

## Engine Variants

The system has 3 engine implementations in `modules/simulations/`:

| Engine | File | Description |
|--------|------|-------------|
| SimulationEngine | `engine.go` | **Legacy** — original simple rule-based engine |
| (engine2) | `engine2.go` | **Legacy** — optimized archetype-aware engine |
| EnhancedSimulationEngine | `engine_enhanced.go` | **Primary** — supports all 6 LLM strategies |

All engines share the same rule engine (`rule_engine.go`) for condition evaluation and action execution. Use `engine_enhanced.go` for new implementations.

## Simulation Lifecycle

1. **Create** — simulation created from a model; model snapshot embedded in simulation document
2. **Generate** — initial agent instances spawned from archetypes using generator (`generator.go` / `generator2.go`)
3. **Execute** — engine runs N steps, each step: evaluate rules → (optional) call LLM → apply actions → record state
4. **Broadcast** — each step pushed via WebSocket to connected clients
5. **Store** — steps saved to MongoDB timeseries collection

## Rule Engine (`rule_engine.go`)

Rules are condition-action pairs with priority ordering, cooldowns, and execute-once semantics.

### Condition Types

| Type | Key | Evaluates |
|------|-----|-----------|
| Property | `property` | Agent property value (eq, gt, lt, etc.) |
| Position | `position` | Spatial coordinates |
| Proximity | `proximity` | Nearby agents/shapes within radius |
| Time | `time` | Simulation step number |
| Composite | `composite` | AND/OR/NOT combination of sub-conditions |
| Count | `count` | Number of agents in area |
| Barrier | `barrier` | Movement blocked by shapes |
| State | `state` | Agent state machine state |
| Distance | `distance` | Distance to target type |
| Custom | `custom` | Universal parameters (metadata-driven) |

### Action Types

| Type | Key | Effect |
|------|-----|--------|
| Set Property | `set_property` | Set property to exact value |
| Modify Property | `modify_property` | Add/subtract/multiply/divide property |
| Move | `move` | Move in direction at speed |
| Move Toward | `move_toward` | Move toward target type |
| Move Away | `move_away` | Move away from target type |
| Interact | `interact` | Trigger interaction with target |
| Emit Event | `emit_event` | Broadcast event |
| Transition | `transition` | State machine transition |
| Call LLM | `call_llm` | Trigger LLM decision for agent |
| Spawn Agent | `spawn_agent` | Create new agent (with position/property inheritance) |
| Remove Agent | `remove_agent` | Remove agent from simulation |
| Transform | `transform` | Change agent archetype |
| Teleport | `teleport` | Move to specific coordinates |
| Custom | `custom` | Universal parameters |

### Universal Parameters

Both conditions and actions support a `Parameters map[string]interface{}` field for flexible, metadata-driven configuration. This enables LLM-generated rules without requiring code changes for new condition/action types.

## Model Structure

A model (`data/models/documents.go`) contains:

- **Agents** — typed as `agent` (individual) or `archetype` (template for many); each has properties, behavior hints, memory size
- **Environment** — width/height, torus wrapping, shapes (rectangle, circle, line, polygon)
- **Rules** — condition-action pairs with priority, cooldown, applies_to list
- **Interactions** — agent-agent, agent-shape, or agent-global; with radius, conditions, bidirectionality, optional LLM
- **LLMStrategy** — which of the 6 strategies to use, with frequency/batch/scope controls

## Interactions

Three interaction types:
- `agent_agent` — between agents (e.g., predator-prey)
- `agent_shape` — agent with environment shape (e.g., agent enters zone)
- `agent_global` — agent with global state

Each interaction defines initiator/target types, radius, conditions, and separate action lists for initiator and target.

## Agent Memory

Agents with `MemorySize > 0` accumulate past decisions and interaction events. This memory is passed to the LLM in subsequent calls, enabling context-aware behavior evolution over simulation steps.

## Supporting Components

| Component | File | Role |
|-----------|------|------|
| SimulationBroadcaster | `broadcaster.go` | WebSocket connection management and step broadcasting |
| SimulationAnalyzer | `analysis.go` | Post-simulation analysis |
| ResultFormatter | `result_formatter.go` | Output formatting for results |
| PDFGenerator | `pdf_generator.go` | PDF export of simulation results |
| SpatialIndex | `spatial_index.go` | Grid-based acceleration for neighbor queries |
| FallbackHandler | `fallback.go` | LLM call failure recovery and fallback policies |
| DecisionApplier | `decision_applier.go` | LLM response parsing, validation, and agent state updates |
| SimulationGenerator | `generator.go`, `generator2.go` | Agent spawning and initialization from archetypes |

### Spatial Index

The `SpatialIndex` provides O(1) neighbor lookups using grid-based hashing:
- **Cell Size** — Configurable spatial granularity (default: 50.0 units)
- **Query Methods** — `FindInRadius()` returns agent indices within radius
- **Torus Wrapping** — Supports wraparound coordinates for bounded environments
- **Rebuild** — Re-indexes all agents after position changes

### Fallback Handler

LLM call failures are handled by an **ordered fallback policy** resolved in
`llm_runtime.go` (`resolveFallback` / `resolveModelFallback`). A policy is a list
of steps applied in order:
- **Retry** — re-invoke the LLM up to `MaxRetries` times with `RetryDelayMs`
  between attempts. The first `retry` step is lifted out as the leading retry
  budget; a run-level `MaxRetries` config applies only when the policy has no
  explicit `retry` step.
- **Rules Only** — run the agent's rule engine for this step (no LLM).
- **Last Decision** — reuse the agent's previously recorded decision.
- **Skip** — the agent does nothing this step.
- **Resolution** — the per-agent override (`Agent.LLMConfig.SimulationConfig.FallbackPolicy`)
  wins over the model default (`LLMSimulationExecutionConfig.FallbackPolicy`);
  both are pointers, nil = unset.
- **Empty / unset default** — a nil or step-less policy degrades gracefully to
  **no retries + a single implicit `rules_only` stage**. New models are not
  stamped with any explicit default.

See `docs/engine-config-and-events.md` §1e for the authoritative description.

### Decision Applier

The `DecisionApplier` applies LLM decisions and archetype strategies to agent state:
- **Individual Decision Application** — Parses LLM JSON response and applies to single agent
- **Position Validation** — Ensures moves respect environment bounds and move types
- **Property Updates** — Applies arbitrary property changes from LLM decisions
- **State Transitions** — Handles agent state machine transitions
- **Archetype Strategies** — Applies strategies uniformly to all agents of same archetype

### Generator

The `SimulationGenerator` spawns initial agent instances from model archetypes:
- **Agent Instantiation** — Creates agents from archetype templates
- **Position Generation** — Randomly places agents within environment bounds
- **Property Initialization** — Sets initial property values from archetype defaults
- **ID Assignment** — Assigns unique numeric IDs to agents
- **Memory Allocation** — Initializes agent memory based on `MemorySize`

**Note:** `generator.go` and `generator2.go` both provide agent generation; refer to current implementation for active variant.

## Cross-References

- See [Rule Engine Documentation](rule-engine.md) for detailed condition/action schemas and universal parameters
- See [Architecture](architecture.md) for LLM integration strategies overview
