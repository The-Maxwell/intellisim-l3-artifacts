# LLM Integration Reference

This document covers IntelliSim's LLM integration architecture: the three integration levels, the four-value strategy enum and its mapping to the six article strategies, the gRPC bridge protocol, DTO contracts, prompt building, events, fallback handling, and token tracking.

---

## Three LLM Integration Levels

IntelliSim uses LLM at three distinct levels:

### Level 1: Model Generation

**When**: During model creation in the chat interface.

**Input**: User natural language descriptions of agents, rules, environment, and interactions.

**Output**: Complete `ModelDocument` with structured definitions ready for simulation.

**LLM Role**: Translate conversational specifications into executable model definitions. May also analyze images (e.g. spatial diagrams) supplied by the user; these are stored as base64 on `ModelDocument.Images` (type `ModelImage`) and passed to the LLM as multimodal input.

**Example**: "Create a predator-prey simulation with herbivores and carnivores. Herbivores eat plants and flee from predators. Carnivores hunt herbivores."

### Level 2: Simulation Execution

**When**: During each simulation step (configurable frequency via `LLMStrategy.DecisionFrequency`).

**Input**: Current agent/environment state, context from nearby agents (perception radius), past decisions (memory).

**Output**: Agent actions, property updates, state transitions, high-level decisions.

**LLM Role**: Generate behavioral decisions based on rules, memory, and environmental context.

**Cost**: Varies by strategy (see Cost Formulas below).

**Example**: "Agent sees 3 nearby herbivores. Should it chase the nearest or move toward food?"

### Level 3: Results Analysis

**When**: After simulation completion (optional).

**Input**: Simulation summary, aggregated metrics, step recordings.

**Output**: Natural language interpretation of results.

**LLM Role**: Explain emergent behaviors and outcomes.

**Cost**: One call per analysis request (not per step).

**Example**: "Analyze why predator population collapsed in steps 500–600."

---

## Six Execution Strategies

IntelliSim's six execution strategies (from the article) are represented by two orthogonal axes:

| Axis | Values |
|------|--------|
| **Strategy kind** (`LLMStrategy.Type`) | `"none"`, `"full"`, `"decision"`, `"hybrid"` |
| **Scope** (`Agent.Type`) | `"agent"` (individual) or `"archetype"` (population) |

The scope is **not** encoded in the strategy type — it derives from `Agent.Type`. The mapping to the six article strategies is:

| Article strategy | `LLMStrategy.Type` | `Agent.Type` | Scope |
|------------------|--------------------|--------------|-------|
| 1 Rule-based (no LLM) | `"none"` | any | N/A |
| 2 Individual full control | `"full"` | `"agent"` | individual |
| 3 Archetype full control | `"full"` | `"archetype"` | archetype |
| 4 Individual decision-only | `"decision"` | `"agent"` | individual |
| 5 Archetype decision-only | `"decision"` | `"archetype"` | archetype |
| 6 Hybrid | `"hybrid"` (model default) | per-agent overrides differ | mixed |

`LLMConfig.DefaultStrategy` is the baseline for agents that do not set their own `LLMAgentConfig.Strategy`. A per-agent `Agent.LLMConfig.Strategy` overrides it.

The `Type` determines which per-step purposes the LLM owns. The four `use_for_*` flags (`movement`, `interaction`, `properties`, `decisions`) are **canonical** for the Type and re-normalized on save (`LLMStrategy.Canonical`): `none` → all off; `full` → all on; `decision` → decisions only; `hybrid` → user-configurable. Spawn and population are **generation** concerns (gated by `AgentInitialization.CountMode == CountLLM`), not runtime strategy flags.

Note: `LLMAgentConfig.Strategy` governs **HOW** the LLM is used (strategy kind and scope). `LLMAgentConfig.Target` governs **WHICH** LLM is called. These two axes are orthogonal — an agent can use strategy `"full"` against a cheap model, or strategy `"decision"` against a powerful one.

---

### Strategy 1: Rules Only (`"none"`)

**When to Use**:
- High performance requirements (no LLM latency).
- Deterministic, reproducible behavior.
- Simple behavioral patterns.
- Development/testing before adding LLM.

**Cost Formula**: 0 LLM calls.

**Code Entry Point**: `modules/simulations/strategy_rules_only.go`

**How It Works**:
1. Execute all rules in priority order.
2. Evaluate conditions using the rule engine.
3. Execute actions immediately.
4. Process interactions (rule-based only).
5. Move to next step.

**Example Configuration** (`ModelDocument.llm_config.default_strategy`):
```json
{
  "type": "none"
}
```

---

### Strategy 2: Individual Full Control (`"full"` + `"agent"`)

**When to Use**:
- Small-scale simulations (10–100 agents).
- Complex individual behaviors where each agent needs its own LLM decision.
- Can afford higher LLM cost.

**Cost Formula**: `N × F`
- N = number of agent instances
- F = decision frequency (steps between LLM calls; 1 = every step)
- Example: 50 agents, every step → 50 LLM calls/step

**Code Entry Point**: `modules/simulations/strategy_individual_full.go`

**How It Works**:
1. For each agent instance, call LLM requesting full-control decision.
2. LLM returns new position, property changes, state, interaction targets.
3. Apply all changes directly via `DecisionApplier.ApplyIndividualDecision()`.
4. Store decision in agent memory for future context.

**Prompt Used**: `buildIndividualPrompt` (`strategy_individual_full.go`) with the
full-kind decision spec — see [LLM Prompts](llm-prompts.md). Agents are batched
(≤ `BatchSize` per call) and demuxed by their 0-based `instance` index.

**Response Schema** (`fullDecisionResp`):
```json
{
  "agents": [
    {
      "instance": 0,
      "actions": [
        {"type": "move_toward", "target_uid": "lake", "speed": 0.7},
        {"type": "modify_property", "property_uid": "energy", "modifier": "subtract", "value": 2}
      ]
    }
  ]
}
```
Allowed action types: `set_property`, `modify_property`, `move`, `move_toward`,
`move_away`, `interact`, `transition`, `emit_event` (validated by
`MapFullActions`; invalid entries are dropped).

**Example Agent Configuration** (on `Agent`):
```json
{
  "uid": "predator",
  "type": "agent",
  "llm_config": {
    "strategy": {
      "type": "full",
      "decision_frequency": 1,
      "use_for_movement": true,
      "use_for_interaction": true,
      "use_for_properties": true,
      "use_for_decisions": true
    }
  }
}
```

---

### Strategy 3: Archetype Full Control (`"full"` + `"archetype"`)

**When to Use**:
- Large populations (1000+ agents of the same type).
- Consistent archetype-level behaviors.
- Reduce LLM calls while maintaining behavioral control.

**Cost Formula**: `A × F`
- A = number of distinct archetypes with `type="full"`
- F = decision frequency
- Example: 3 archetypes, every step → 3 LLM calls/step regardless of agent count

**Code Entry Point**: `modules/simulations/strategy_archetype_full.go`

**How It Works**:
1. Group agents by archetype.
2. For each archetype, build aggregated stats (population count, avg properties, state distribution, spatial centroid).
3. Call LLM requesting archetype-level movement and property update rules.
4. Apply results to each member according to `LLMStrategy.ArchetypeMode`:
   - `identical` — same actions list applied to every member (legacy behavior).
   - `distribution` — LLM returns a weighted set of action variants; each member independently samples one whole variant via its per-agent seeded RNG (`agentJitterRNG = Seed ^ Step ^ agentID`) using `pickChoice` (`modules/simulations/sampler.go`).
5. Store strategy in memory for consistency.

**Prompt Used**: `buildArchetypePrompt` (`strategy_archetype_full.go`) — includes
the per-archetype aggregate block (live count, state distribution, numeric
property mean/min/max, spatial centroid + spread) plus a representative member's
example state/position; see [LLM Prompts](llm-prompts.md).

**Response Schema — identical mode**:
```json
{
  "agents": [
    {
      "archetype_uid": "herbivore",
      "actions": [{"type": "move_toward", "target_uids": ["grass"]}, {"type": "set_property", "property_uid": "energy", "value": 80}]
    }
  ]
}
```

**Response Schema — distribution mode**:
```json
{
  "agents": [
    {
      "archetype_uid": "herbivore",
      "variants": [
        {"weight": 3, "actions": [{"type": "move_toward", "target_uids": ["grass"]}]},
        {"weight": 1, "actions": [{"type": "move_away", "target_uids": ["predator"]}]},
        {"weight": 0.5, "actions": [{"type": "set_state", "value": "resting"}]}
      ]
    }
  ]
}
```

Each member samples one variant (the whole `actions` list within it). Weights are relative and need not sum to 1.

**`ArchetypeMode` / `VariationStrength` Config** (on `LLMStrategy`):

| Field | Default | Effect |
|---|---|---|
| `archetype_mode` | `"identical"` for old models; `"distribution"` for new models | Controls which response schema the LLM is asked for and how results are applied. |
| `variation_strength` | `0.15` when unset/0; clamped to 1 | Scales per-agent numeric-property jitter and movement speed/heading jitter for all archetype modes. |

New models default to `distribution` via `Model.StampDefaultArchetypeMode()` (called on model create and LLM-generation merge only — not on generic saves). Models persisted before this field was added resolve an empty `archetype_mode` to `identical` at runtime, preserving their prior behavior without migration.

**Example Agent (Archetype) Configuration**:
```json
{
  "uid": "herbivore",
  "type": "archetype",
  "llm_config": {
    "strategy": {
      "type": "full",
      "decision_frequency": 1,
      "use_for_movement": true,
      "use_for_properties": true,
      "use_for_decisions": true,
      "archetype_mode": "distribution",
      "variation_strength": 0.2
    }
  }
}
```

---

### Strategy 4: Individual Decision-Only (`"decision"` + `"agent"`)

**When to Use**:
- Large simulations with behavior rules already defined.
- Want LLM input without full control.
- Lower token cost per call than Strategy 2 while retaining intelligent decisions.

**Cost Formula**: `N × F`
- Similar call count to Strategy 2 but shorter responses (decision only, not full state).

**Code Entry Point**: `modules/simulations/strategy_individual_decision.go`

**How It Works**:
1. Call LLM for each agent requesting a high-level decision only.
2. LLM returns an action name, priority, and reasoning.
3. Store decision in the runtime `DecisionCache` via `DecisionApplier.StoreDecisionInCache()`.
4. Rule engine evaluates `ConditionTypeDecision` conditions, which match against the cached decision.
5. Matching rules fire and execute concrete actions.

**Prompt Used**: `buildIndividualPrompt` (`strategy_individual_full.go`, shared
builder) with the decision-label spec — the allowed labels come from the
archetype's `DecisionMenuWithHints` and the prompt may advertise LLM-emittable
event UIDs; see [LLM Prompts](llm-prompts.md).

**Response Schema** (`decisionResp`):
```json
{
  "agents": [
    {"instance": 0, "label": "hunt_nearby_prey", "params": {"target_preference": "closest"}}
  ]
}
```
Each entry may optionally carry `"emit_uid"` (+ `"emit_data"`) to emit an
authorized model event.

**Example Agent + Rule Configuration**:
```json
{
  "uid": "wolf",
  "type": "agent",
  "llm_config": {
    "strategy": {
      "type": "decision",
      "decision_frequency": 2,
      "use_for_decisions": true
    }
  }
}
```
```json
{
  "id": "hunt_rule",
  "applies_to": ["wolf"],
  "conditions": [
    {
      "type": "decision",
      "operator": "eq",
      "value": "hunt_nearby_prey"
    }
  ],
  "actions": [
    {"type": "move_toward", "target_uids": ["sheep"], "speed": 3}
  ]
}
```

---

### Strategy 5: Archetype Decision-Only (`"decision"` + `"archetype"`)

**When to Use**:
- Very large populations needing coordinated behavior.
- Significantly lower LLM cost than Strategy 4.
- Rules define diverse individual responses to the same archetype decision.

**Cost Formula**: `A × F`
- A = number of archetypes with `type="decision"`
- Example: 3 archetypes, every step → 3 LLM calls/step

**Code Entry Point**: `modules/simulations/strategy_archetype_decision.go`

**How It Works**:
1. Group agents by archetype.
2. Build aggregated stats.
3. Call LLM requesting archetype decision according to `LLMStrategy.ArchetypeMode`:
   - `identical` — single label applied to every member (legacy behavior; stored in shared `DecisionCache`).
   - `distribution` — weighted set of labels; each member independently samples one via its per-agent seeded RNG (`agentJitterRNG = Seed ^ Step ^ agentID`) using `pickChoice` (`modules/simulations/sampler.go`), and that label is stored in the per-agent `DecisionCache`.
4. Each agent executes its own rules, matching against its (individual or shared) cached decision.

**Prompt Used**: `buildArchetypePrompt` (`strategy_archetype_full.go`, shared
builder) with the archetype decision spec (union decision menu with hints across
the batch); see [LLM Prompts](llm-prompts.md).

**Response Schema — identical mode**:
```json
{
  "agents": [
    {
      "archetype_uid": "bird",
      "label": "migrate_south",
      "params": {}
    }
  ]
}
```

**Response Schema — distribution mode**:
```json
{
  "agents": [
    {
      "archetype_uid": "bird",
      "choices": [
        {"label": "migrate_south", "weight": 2, "params": {}},
        {"label": "forage",        "weight": 1, "params": {}},
        {"label": "roost",         "weight": 0.5, "params": {}}
      ]
    }
  ]
}
```

Each member samples one label. Weights are relative. The sampled `params` map is stored alongside the label in the `DecisionCache` for rule evaluation.

**`ArchetypeMode` / `VariationStrength` Config**: identical semantics and defaults as described under Strategy 3. New models default to `distribution`; pre-existing models with empty `archetype_mode` resolve to `identical` at runtime.

**Example Archetype Configuration**:
```json
{
  "uid": "bird",
  "type": "archetype",
  "llm_config": {
    "strategy": {
      "type": "decision",
      "decision_frequency": 10,
      "use_for_decisions": true,
      "archetype_mode": "distribution",
      "variation_strength": 0.15
    }
  }
}
```

---

### Strategy 6: Hybrid (per-agent overrides + model `"hybrid"` default)

**When to Use**:
- Complex multi-species simulations with varying intelligence requirements.
- Some agents need full LLM control, others just high-level decisions, others purely rules.
- Fine-grained cost optimization.
- Realistic behavioral diversity.

**Cost Formula**: Sum of per-agent/archetype strategies.
- Example: Predator (`"full"`, 5 agents) + Prey (`"decision"`, 50 agents) + Plants (`"none"`) → 5 + 50 = 55 LLM calls/step (vs. 555 for all individual full).

**Code Entry Point**: `modules/simulations/strategy_hybrid.go`

**How It Works**:
1. For each agent, check `Agent.LLMStrategy`. If set, use that agent's specific strategy kind and scope.
2. If `Agent.LLMStrategy` is nil, fall back to `LLMConfig.DefaultStrategy`.
3. Execute the resolved strategy per agent/archetype.
4. Combine results and process interactions.

**How to Configure** (model-level default set to `"hybrid"`; per-agent `llm_config` blocks define the actual strategies and optionally their own model routing):

```json
{
  "llm_config": {
    "target": {
      "provider_uid": "openai-provider",
      "model_uid": "gpt-4o-mini",
      "effort": ""
    },
    "default_strategy": {
      "type": "hybrid"
    }
  },
  "agents": [
    {
      "uid": "predator",
      "type": "agent",
      "llm_config": {
        "target": {
          "provider_uid": "openai-provider",
          "model_uid": "gpt-4o",
          "effort": "high"
        },
        "strategy": {
          "type": "full",
          "decision_frequency": 1,
          "use_for_movement": true,
          "use_for_interaction": true
        }
      }
    },
    {
      "uid": "prey",
      "type": "agent",
      "llm_config": {
        "target": {
          "provider_uid": "openai-provider",
          "model_uid": "gpt-4o-mini",
          "effort": "low"
        },
        "strategy": {
          "type": "decision",
          "decision_frequency": 2,
          "use_for_decisions": true
        }
      }
    },
    {
      "uid": "plant",
      "type": "archetype",
      "llm_config": {
        "strategy": {
          "type": "none"
        }
      }
    }
  ]
}
```

**Result**:
- Predators (5 agents) use full control with `gpt-4o` at high effort: 5 LLM calls/step.
- Prey (50 agents) use decision-only with `gpt-4o-mini` at low effort: 50 LLM calls/step.
- Plants (hundreds) use rules only: 0 LLM calls/step.
- **Total**: 55 calls/step (vs. 555 for all individual full).
- Per-entity `llm_config.target` routing (`Agent.LLMConfig.Target` > `LLMConfig.Target`) allows different models for different behavioral complexity levels without changing the execution strategy.

---

## Events

### EventDefinition

Events are first-class model-level entities. `ModelDocument.Events` holds the definitions; producers emit them at runtime via rules/actions; consumers react via conditions.

```go
type EventDefinition struct {
    UID         string          `bson:"uid"`
    Name        string          `bson:"name"`
    Description string          `bson:"description,omitempty"`
    Scope       EventScope      `bson:"scope"`        // "global" | "local" | "targeted"
    Radius      *float64        `bson:"radius,omitempty"`
    Payload     []AgentProperty `bson:"payload,omitempty"`

    Producers    []EventProducer `bson:"producers"`
    ProducerUIDs []string        `bson:"producer_uids,omitempty"`
    ConsumerUIDs []string        `bson:"consumer_uids,omitempty"`

    TTLSteps         uint   `bson:"ttl_steps,omitempty"`
    LLMPromptContext string `bson:"llm_prompt_context,omitempty"`

    // LLMTarget routes LLM-produced emissions to a specific provider/model/effort.
    // nil = inherit from LLMConfig.Target (model-level default).
    LLMTarget *common.LLMTarget `bson:"llm_target,omitempty"`
}
```

**Producer side**: An agent, environment rule, or LLM response emits the event using `ActionTypeEmitEvent` with `Action.EventUID` set to the `EventDefinition.UID`. Optional `Action.EventData` provides the payload.

**Consumer side**: `ConditionTypeEvent` — `Condition.TargetUID` holds the `EventDefinition.UID`. An optional `Condition.PropertyUID` + `Operator` + `Value` triple matches a specific payload field. Scope, radius, and producer filtering are applied by the engine at runtime.

**Producers**:

| Value | Description |
|-------|-------------|
| `"agent"` | Emitted by an agent via `emit_event` action |
| `"environment"` | Emitted by the environment through its own rules/strategies |
| `"llm"` | Emitted as part of the LLM's decision output |

**Scopes**:

| Value | Description |
|-------|-------------|
| `"global"` | Perceivable by all agents |
| `"local"` | Perceivable by agents within `radius` of the emit position |
| `"targeted"` | Perceivable only by agents in `consumer_uids` |

**TTL**: `ttl_steps=0` means the event is active only on the step it is emitted. `ttl_steps=N` means it remains active for N additional steps.

**Runtime event bus** (queue, TTL expiry, scope-based delivery) is engine work and out of scope for the data layer.

**Example event + rule wiring**:
```json
// EventDefinition on ModelDocument.events:
{
  "uid": "predator_spotted",
  "name": "Predator Spotted",
  "scope": "local",
  "radius": 20.0,
  "producers": ["agent"],
  "ttl_steps": 2
}

// Emitting action in a rule:
{
  "type": "emit_event",
  "event_uid": "predator_spotted"
}

// Reacting condition in another rule:
{
  "type": "event",
  "target_uid": "predator_spotted"
}
```

---

## gRPC Bridge Protocol

The gRPC bridge (`llmbridge.v1.LLMBridge`) connects remote LLM providers to core over a
long-lived gRPC connection. The proto lives in the SDK repository at
`proto/llmbridge/v1/llmbridge.proto`; regenerate with
`scripts/gen-llmbridge-proto.ps1` (delegates to the SDK's gen script).

### Two RPCs

```
service LLMBridge {
  rpc Connect(ConnectRequest) returns (stream Invocation);
  rpc CompleteInvocation(CompleteInvocationRequest) returns (CompleteInvocationAck);
}
```

`Connect` is a **server-stream**: the provider opens one long-lived stream that doubles as
both registration and the delivery channel for invocations. `CompleteInvocation` is a
**unary** call: each result is an independent HTTP/2 request on the same connection, so
there is no head-of-line blocking between results.

### Registration (inside Connect)

The provider sends `ConnectRequest` as the single request message:

| Field | Type | Description |
|---|---|---|
| `provider_uid` | string | Unique identifier — matches `LLMTarget.ProviderUID` |
| `name` / `description` / `version` | string | Informational |
| `models` | `[]Model` | At least one required |

Each `Model` carries:

| Field | Type | Description |
|---|---|---|
| `uid` | string | Matches `LLMTarget.ModelUID` (e.g. `"gpt-4o"`) |
| `capabilities` | `[]ModelCapability` | Closed enum — see capability list in DTO Contract |
| `efforts` | `[]Effort` | Allowed effort levels; empty = model has no effort control |
| `context_window` | uint32 | Total context tokens |
| `max_output_tokens` | uint32 | Max tokens the model can emit |
| `max_images` | uint32 | Maximum number of images accepted in a single `VisionRequest`; **0 = unset** — core treats an unset or zero value as **1** (conservative single-image default). Only meaningful for models with the `vision_analysis` capability. `GET /llms` exposes this field as `max_images` on each `ModelInfo` entry |

Core validates the registration (non-empty `provider_uid`, at least one model, unique
non-empty model UIDs, non-empty capabilities from the closed enum, efforts from the closed
enum). A validation failure returns gRPC `InvalidArgument` — the SDK surfaces this from
`Run()` and does **not** retry (a config bug won't fix itself).

After the header is sent by core, registration is accepted and the provider appears in the
registry. If a provider reconnects with the same UID while the old session is still alive,
core **kicks the old session** (newest-wins) and fails its pending invocations.

### Invocation Flow (core → provider, via stream)

Core pushes `Invocation` messages on the open stream:

| Field | Type | Description |
|---|---|---|
| `invocation_id` | string | UUID; correlates the unary completion |
| `model_uid` | string | **REQUIRED** — core resolves and validates the target before sending |
| `effort` | Effort | `UNSPECIFIED` = model default |
| `timeout_ms` | uint64 | Core's deadline; provider aborts the handler after this (0 = SDK default) |
| `request` | oneof | `simple`, `chat`, `vision`, or `reasoning` |

`GenerationConfig` (inside each request type) carries optional tuning: `temperature`,
`max_tokens`, `top_p`, `frequency_penalty`, `presence_penalty`, `seed` (reproducible ABM
runs), `stop` sequences, `response_format` (`text` or `json`), and a `provider_options`
map for provider-specific overrides.

### Completion Flow (provider → core, unary)

Provider calls `CompleteInvocation`:

```
CompleteInvocationRequest {
  provider_uid   — sanity check: must match the invoked provider
  invocation_id  — correlates to the Invocation
  result oneof:
    Completion         { text, finish_reason, TokenUsage }   — for simple | chat | vision
    ReasoningCompletion{ analysis, reasoning, finish_reason, TokenUsage }
    Error              { ErrorCode, message }
}
```

`TokenUsage` on the wire carries the **actual** model/effort used:

| Field | Description |
|---|---|
| `model_uid` | Model actually used (may differ if provider substituted) |
| `effort` | Effort actually applied |
| `prompt_tokens` / `completion_tokens` / `total_tokens` | uint64 |
| `latency_ms` | Provider-measured wall-clock time |

Core maps this onto `common.TokenUsage`, adding `ProviderUID` and `Calls=1`.

`CompleteInvocationAck.accepted`:
- `true` — core delivered the result to the waiting caller.
- `false` — core no longer waits (caller timed out or provider reconnected and the old
  pending entry was cleaned up). The provider **drops the result** silently.

`CompleteInvocationAck` is never a gRPC error; the provider should not retry on
`accepted=false`.

### Liveness

Liveness is owned entirely by **gRPC keepalive** — there are no `Heartbeat` or `Unregister`
RPCs. Server parameters:

```
Time:    30s  — ping interval for idle connections
Timeout: 10s  — how long to wait for the keepalive ping ACK
MaxConnectionIdle/Age/AgeGrace: omitted (∞) — long-lived provider streams must not be evicted
EnforcementPolicy: MinTime 10s, PermitWithoutStream true
MaxRecvMsgSize / MaxSendMsgSize: 32 MB (vision payloads exceed the 4 MB default)
```

When the stream breaks (network blip, provider restart, keepalive timeout), core
unregisters the provider and fails its pending invocations. Provider callers' fallback
policies take over immediately.

### Reconnect (newest wins)

The SDK reconnects automatically with exponential backoff (1 s → 30 s cap, ±20% jitter).
Each reconnect sends a fresh `ConnectRequest`; if the old TCP connection is still open,
core kicks it and its pending invocations the moment the new `Connect` arrives.

### Failure-Behavior Table

| Failure | Behavior |
|---|---|
| Provider process restart | SDK reconnects (backoff 1 s→30 s, jitter); registration rides the new `Connect`; core kicks the lingering old session (newest wins) |
| Core restart | All streams break; providers reconnect-loop until core is back; in-flight invocations fail core-side (fallback policies act) |
| Network blip | Keepalive detects ≤ ~40 s (server) / ~20 s (client); both sides treat it as a stream break → reconnect |
| Provider hangs mid-call | Core's `Invoke` honors ctx deadline → pending entry removed → late completion gets `accepted=false` |
| Handler panic (provider) | SDK recovers → `ERROR_CODE_INTERNAL` completion; provider keeps serving |
| Bad registration | gRPC `InvalidArgument`; SDK surfaces the error from `Run()` without retry |

---

## DTO Contract

All LLM operations go through `modules/llm`. Source files: `service.go`, `dtos.go`, `errors.go`.

### Service Interface

```go
// Service is the consumer-facing LLM API: it routes a request to the provider
// named by its LLMTarget, strictly validating target/effort/purpose against
// the provider's registration before anything reaches the wire.
type Service interface {
    Simple(ctx context.Context, req SimpleRequest) (*Completion, error)
    Chat(ctx context.Context, req ChatRequest) (*Completion, error)
    Vision(ctx context.Context, req VisionRequest) (*Completion, error)
    Reasoning(ctx context.Context, req ReasoningRequest) (*ReasoningCompletion, error)

    Providers() []ProviderInfo
    ProvidersByCapability(capability common.ModelCapability) []ProviderInfo
    ProvidersByModel(modelUID string) []ProviderInfo
}
```

Modules inject `llm.Service` (FX-provided) and never interact with providers directly.

### Request Types

All four request types carry `Target` (routing), `Purpose` (capability intent), and
`Config` (per-request tuning):

```go
// SimpleRequest — single-turn completion; Level-2 workhorse.
type SimpleRequest struct {
    Target       common.LLMTarget       // ProviderUID + ModelUID required; Effort optional
    Purpose      common.ModelCapability // optional: what the call is FOR; validated when set
    Message      string
    SystemPrompt string
    Config       GenerationConfig
}

// ChatRequest — multi-turn with history (Level 1 / Level 3 chats).
type ChatRequest struct {
    Target           common.LLMTarget
    Purpose          common.ModelCapability
    Message          string                  // the new user message
    SystemMessage    string
    Messages         []chats.ChatMessage     // chronological, oldest first; fitted by Service
    OverflowStrategy ContextOverflowStrategy // core-side: "cut" (default) or "summarize"
    ChatType         string                  // "plain" | "model" | "simulation" — summarization context
    Config           GenerationConfig
}

// VisionRequest — multi-image analysis (environment/shape extraction, Level 1).
// Images carries one entry per ModelImage in the stored library (all images are
// sent in a single LLM call). Each VisionImage includes the compressed base64
// bytes, format, and an optional description label prepended as a text part in
// the prompt so the LLM can distinguish multiple reference images.
type VisionRequest struct {
    Target       common.LLMTarget
    Purpose      common.ModelCapability
    Message      string
    Images       []VisionImage // one entry per stored ModelImage (replaces single ImageData/ImageFormat)
    Detail       string        // "auto" | "low" | "high"
    SystemPrompt string
    Config       GenerationConfig
}

// VisionImage is one image within a multi-image VisionRequest.
// Mirrors the proto VisionImage message (repeated in VisionRequest on the wire).
type VisionImage struct {
    ImageData   []byte // raw bytes decoded from ModelImage.Data (base64)
    ImageFormat string // "png" | "jpeg" | "webp" | "gif"
    Description string // optional label forwarded as a text part before the image
}

// ReasoningRequest — deep-analysis call (Level 3 results analysis).
type ReasoningRequest struct {
    Target       common.LLMTarget
    Purpose      common.ModelCapability
    Prompt       string
    Context      string
    SystemPrompt string
    Config       GenerationConfig
}
```

`GenerationConfig` carries optional tuning (all pointer fields — nil = provider/model
default): `Temperature *float64`, `MaxTokens *uint32`, `TopP`, `FrequencyPenalty`,
`PresencePenalty`, `Seed *uint64`, `Stop []string`, `ResponseFormat`, `ProviderOptions map[string]string`.

### Response Types

```go
type Completion struct {
    Text         string
    FinishReason string
    Usage        common.TokenUsage // fully attributed; Calls=1
}

type ReasoningCompletion struct {
    Analysis     string
    Reasoning    string
    FinishReason string
    Usage        common.TokenUsage
}
```

`common.TokenUsage` embeds `common.LLMTarget` (flat bson/json keys `provider_uid`,
`model_uid`, `effort`) and adds `PromptTokens`, `CompletionTokens`, `TotalTokens uint64`,
`Calls uint64`, `LatencyMs float64`. The model/effort fields reflect the **actual** values
reported by the provider on the wire (not necessarily the requested ones).

### Validation Order (Service.resolve)

`Service` performs strict validation before any request reaches the wire:

1. `Target.ProviderUID` set and connected → else `ErrProviderNotFound`
2. `Target.ModelUID` set and registered by that provider → else `ErrModelNotFound`
3. `Target.Effort` (when set) ∈ model's declared efforts → else `ErrEffortNotAllowed`
4. `Purpose` (when set) ∈ model's capabilities → else `ErrCapabilityNotSupported`
5. Implicit gates: `Vision` call always requires `vision_analysis`; `Reasoning` always requires `results_analysis` (regardless of `Purpose` field)
6. Token budget via `TokenEstimator`; `Chat` runs `ContextFitter` first (trim or summarize to fit context window; summarize = recursive `Service.Simple` on the same target)

There is **no silent clamping**: a disallowed effort or missing capability is always an error.

### Capability Closed List (`common.ModelCapability`)

| Constant | Wire value | Purpose |
|---|---|---|
| `ModelCapabilityChat` | `"chat"` | General conversation |
| `ModelCapabilityModelGeneration` | `"model_generation"` | Level 1 — ABM model generation |
| `ModelCapabilitySimulationGeneration` | `"simulation_generation"` | Level 2 — simulation generation (env + agent seeding) |
| `ModelCapabilitySimulationExecution` | `"simulation_execution"` | Level 2 — full-control strategies |
| `ModelCapabilitySimulationDecisions` | `"simulation_decisions"` | Level 2 — decision-only strategies |
| `ModelCapabilityResultsAnalysis` | `"results_analysis"` | Level 3 — results analysis |
| `ModelCapabilityVisionAnalysis` | `"vision_analysis"` | Image → environment/shapes (multi-image; count limited by `Model.max_images`, defaulting to 1 when unset) |

> **`simulation_generation` usage:** the simulation generation phase
> (`modules/simulations/generator.go`) calls the LLM with `Purpose:
> ModelCapabilitySimulationGeneration`, resolving its target from the model's
> `SimulationConfig.LLMSimulationGenerationConfig.Target` (overridable per-simulation
> via `Simulation.LLMGenerationTargetOverride`). The execution phase uses
> `simulation_execution`/`simulation_decisions` from
> `SimulationConfig.LLMSimulationExecutionConfig`.

### Error Taxonomy

**Sentinels** (match with `errors.Is`):

| Sentinel | Meaning |
|---|---|
| `ErrProviderNotFound` | ProviderUID not set or not connected |
| `ErrModelNotFound` | ModelUID empty or not registered by the named provider |
| `ErrEffortNotAllowed` | Requested effort not in the model's declared effort list |
| `ErrCapabilityNotSupported` | Purpose or required capability not declared by the model |
| `ErrProviderDisconnected` | Provider stream ended while waiting for a response |
| `ErrMalformedProviderResponse` | Provider returned an unexpected or malformed result type |

**Typed provider errors** (match with `errors.As(*ProviderError)`):

`*ProviderError{Code ProviderErrorCode, Message string}` surfaces failures reported by the
provider on the wire (e.g. `ProviderErrUpstreamError`, `ProviderErrTimeout`,
`ProviderErrInternal`). These are distinct from core-side validation sentinels so that
fallback logic can distinguish upstream failures from config errors.

### Provider and Model Info Types

```go
type ProviderInfo struct {
    UID         string      `json:"uid"`
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Version     string      `json:"version"`
    ConnectedAt time.Time   `json:"connected_at"`
    Models      []ModelInfo `json:"models"`
}

type ModelInfo struct {
    UID             string                   `json:"uid"`
    Name            string                   `json:"name"`
    Capabilities    []common.ModelCapability `json:"capabilities"`
    Efforts         []common.LLMEffort       `json:"efforts"` // empty = no effort control
    ContextWindow   uint32                   `json:"context_window"`
    MaxOutputTokens uint32                   `json:"max_output_tokens"`
    MaxImages       uint32                   `json:"max_images"` // 0 = unset (core resolves to 1); only meaningful for vision_analysis models
}
```

---

## Provider and Model Selection

IntelliSim uses `common.LLMTarget` as the single shape for LLM selection everywhere: which provider to call, which model to request, and at what reasoning effort level.

### Purpose

All four request types carry an optional `Purpose common.ModelCapability` field that declares what the call is FOR. When set, `Service` validates it against the model's declared capabilities before sending anything to the provider. This ensures a chat-only model is never accidentally called for simulation execution.

Typical `Purpose` values by call site:
- Simulation decision strategies → `ModelCapabilitySimulationDecisions`
- Simulation full-control strategies → `ModelCapabilitySimulationExecution`
- Model generation (chat/models) → `ModelCapabilityModelGeneration`
- Results analysis → `ModelCapabilityResultsAnalysis`
- Image analysis → `ModelCapabilityVisionAnalysis`

`Service` strictly validates both `Purpose` (when set) and effort against the provider's registration — there is no silent clamping or fallback to another model.

```go
type LLMTarget struct {
    ProviderUID string    // bson/json: "provider_uid"
    ModelUID    string    // bson/json: "model_uid"    — e.g. "gpt-4o"; empty = provider default
    Effort      LLMEffort // bson/json: "effort"       — empty = provider default
}
```

`LLMEffort` constants: `"minimal"`, `"low"`, `"medium"`, `"high"`, `"max"`, or empty (provider default).

### Per-entity model routing

A core idea of the meta-system is that **different entities can be routed to different models**. A cheap model can handle simple archetypes while a more capable model handles complex agents that require deeper reasoning. LLM target selection is carried inside a **config wrapper struct** at agent and shape level, or directly as a nullable pointer for events:

| Location | Accessor | bson key | Purpose |
|----------|----------|----------|---------|
| `LLMConfig.Target` | `model.LLMConfig.Target` | `target` | Model-level default — lowest-priority fallback |
| `Agent.LLMConfig.Target` | `agent.LLMConfig.Target` | `llm_config.target` | Per-agent routing inside wrapper — most specific for agent calls |
| `ShapeDocument.LLMConfig.Target` | `shape.LLMConfig.Target` | `llm_config.target` | Per-shape routing inside wrapper |
| `EventDefinition.LLMTarget` | `event.LLMTarget` | `llm_target` | Per-event routing (direct pointer, no wrapper) when `Producers` contains `"llm"` |
| `ChatDocument.LLMTarget` | `chat.LLMTarget` | `llm_target` | Chat-level target for Level-1 and Level-3 conversations |

`Agent.LLMConfig` and `ShapeDocument.LLMConfig` are nullable (`*LLMAgentConfig` / `*LLMShapeConfig`). A nil wrapper means "no per-entity override — use model default." `EventDefinition.LLMTarget` remains a direct `*common.LLMTarget` pointer.

> **Removed**: `SimulationDocument.LLMTarget`. There is no longer a run-level LLM target for Level-2 calls. The model-level `LLMConfig.Target` is the lowest-priority fallback.

### Resolution order (Level-2 simulation execution calls)

The most specific target wins. Two levels only:

```
Agent.LLMConfig.Target                 (per-agent wrapper; highest priority)
  or ShapeDocument.LLMConfig.Target    (per-shape wrapper)
  or EventDefinition.LLMTarget         (per-event direct pointer)
        >
LLMConfig.Target                       (model-level default, lowest priority)
```

A nil per-entity target (or nil wrapper) means "inherit from the model default." The chat-level `LLMTarget` (`ChatDocument.LLMTarget`) is used exclusively for Level-1 (model generation) and Level-3 (results analysis) conversations and does not participate in the Level-2 resolution chain.

### Effort level

`LLMEffort` controls reasoning depth for providers that support it (e.g. OpenAI reasoning models). Five named levels: `"minimal"` / `"low"` / `"medium"` / `"high"` / `"max"`. Different effort levels affect token consumption, latency, and cost. Setting effort to `"high"` or `"max"` on complex agent archetypes and `"minimal"` on simple rule-only agents optimizes cost without sacrificing behavioral quality. An empty effort value delegates the choice to the provider's default.

---

## Token and Cost Tracking

Token usage is **target-attributed**: every LLM call records `ProviderUID`, `ModelUID`, and `Effort` on a `common.TokenUsage` entry via the embedded `common.LLMTarget`, so attribution is preserved even when the provider, model, or effort changes mid-chat or mid-run.

### Per-call: TokenUsage

A single LLM call produces one `common.TokenUsage`. `LLMTarget` is embedded inline so bson/json keys are flat:

```go
type TokenUsage struct {
    LLMTarget        // inline — bson/json keys: "provider_uid", "model_uid", "effort"
    PromptTokens     uint64
    CompletionTokens uint64
    TotalTokens      uint64
    Calls            uint64
    LatencyMs        float64
}
```

Composite literal construction must nest the embedded struct:
```go
common.TokenUsage{
    LLMTarget: common.LLMTarget{ProviderUID: "openai-provider", ModelUID: "gpt-4o", Effort: common.LLMEffortHigh},
    PromptTokens: 1200,
    // ...
}
```

Promoted-field access (`u.ModelUID`, `u.ProviderUID`, `u.Effort`) still compiles without nesting.

For assistant messages: stored as `ChatMessageDocument.TokenUsage`. The model used is `TokenUsage.ModelUID`.

### Aggregate: TokenUsageSummary

Running totals that may span multiple targets (different providers, models, or effort levels):

```go
type TokenUsageSummary struct {
    Total   TokenUsage   // grand total; LLMTarget fields always empty
    ByModel []TokenUsage // one entry per (provider_uid, model_uid, effort) triple
}
```

Stored as:
- `ChatDocument.TokenUsage` — running aggregate for the chat.
- `SimulationDocument.TokenUsage` — running aggregate for the simulation run (as `*SimulationTokenUsage` which extends this with per-agent and per-purpose axes).

The aggregator in `modules/simulations` accumulates per `(ProviderUID, ModelUID, Effort)` so no attribution is lost even when the provider, model, or effort changes mid-run.

### Token Budget (Tokenizer)

**File**: `modules/llm/tokenizer.go`

The `TokenEstimator` manages context window constraints using tiktoken.

#### Methods

- **`CountTokens(text)`**: Estimate tokens for a string.
- **`CountChatTokens(messages)`**: Estimate tokens for a chat message array (includes per-message overhead).
- **`ValidateTokenBudget(promptTokens, contextWindow, maxOutputTokens)`**: Check if prompt fits; returns `TokenBudgetError` if it exceeds the window.
- **`TrimChatHistory(messages, promptTokens, contextWindow, maxOutputTokens)`**: Remove oldest messages to fit budget; keeps newest messages.
- **`SplitForSummarization(messages, promptTokens, contextWindow, maxOutputTokens)`**: Splits into `ToSummarize` (old) and `ToKeep` (recent); returns nil if all messages fit.
- **`BuildSummarizationPrompt(messages, chatType)`**: Create an LLM prompt to summarize old messages.

#### Constants

```go
const (
    defaultContextWindow   = 128000
    defaultMaxOutputTokens = 4096
    safetyMarginTokens     = 200
    tokensPerMessage       = 4
    summaryTargetRatio     = 0.2
)
```

### Cost Estimation by Strategy

| Strategy | LLM calls per step | Approximate tokens per call |
|----------|-------------------|----------------------------|
| Strategy 1 (`"none"`) | 0 | 0 |
| Strategy 2 (`"full"` + `"agent"`) | N (agents) × (1/F) | ~2000–4000 |
| Strategy 3 (`"full"` + `"archetype"`) | A (archetypes) × (1/F) | ~2000–3000 |
| Strategy 4 (`"decision"` + `"agent"`) | N × (1/F) | ~1500–2500 (shorter response) |
| Strategy 5 (`"decision"` + `"archetype"`) | A × (1/F) | ~1500–2500 |
| Strategy 6 (`"hybrid"`) | Sum of per-agent strategies | Varies |

N = number of agent instances; A = number of archetypes; F = `DecisionFrequency` (call every F steps).

---

## LLM Runtime Configuration

### Run-Level: LLMSimulationConfig

Applied to the entire simulation run. Stored as `LLMConfig.DefaultSimulationConfig` on the model. There is no per-run `SimulationDocument.Config` override — all run-level LLM configuration comes from the model definition.

```go
type LLMSimulationConfig struct {
    LLMSimulationAgentConfig `bson:",inline"` // inlined per-agent fields below

    ParallelWorkers uint `bson:"parallel_workers,omitempty"`
    RetryTimeoutMs  uint `bson:"retry_timeout_ms,omitempty"` // per-call timeout
    MaxRetries      uint `bson:"max_retries,omitempty"`
    StreamBatchSize uint `bson:"stream_batch_size,omitempty"` // WebSocket chunk size
}
```

| Field | Type | Description |
|-------|------|-------------|
| `parallel_workers` | uint | Concurrent workers for LLM calls within a step (was `int`) |
| `retry_timeout_ms` | uint | Per-LLM-call timeout in milliseconds (was `int`) |
| `max_retries` | uint | Maximum retries before applying fallback (was `int`) |
| `stream_batch_size` | uint | Number of step documents batched into each WebSocket push (was `int`) |

### Per-Agent: LLMSimulationAgentConfig

Spatial and memory constraints per agent/archetype. Set on `Agent.LLMConfig.SimulationConfig`; if nil, inherits from the model's `LLMConfig.DefaultSimulationConfig`.

```go
type LLMSimulationAgentConfig struct {
    PerceptionRadius  float64         `bson:"perception_radius,omitempty"`
    InteractionRadius float64         `bson:"interaction_radius,omitempty"`
    MemorySize        int             `bson:"memory_size,omitempty"`
    FallbackPolicy    *FallbackPolicy `bson:"fallback_policy,omitempty"`
}
```

| Field | Description |
|-------|-------------|
| `perception_radius` | Distance this agent can sense nearby entities |
| `interaction_radius` | Distance within which interactions trigger |
| `memory_size` | Maximum `AgentMemory` entries to retain |
| `fallback_policy` | LLM failure policy; nil = inherit from next level |

---

## Fallback Handling

> **Note:** This section reflects the pre-refactor LLM API still present in `modules/simulations` (currently paused mid-migration). It will be updated when the module is migrated — see `docs/migration/2026-06-06-llm-service-migration.md` (C4).

**File**: `modules/simulations/fallback.go`

### FallbackPolicy

`FallbackPolicy` is a **multi-step sequence** of `FallbackStep` entries. The engine walks through `Steps` in order; if a step's action is exhausted or itself fails, it proceeds to the next.

```go
type FallbackPolicy struct {
    Steps []FallbackStep
}

type FallbackStep struct {
    Type         FallbackPolicyType // "retry" | "rules_only" | "last_decision" | "skip"
    MaxRetries   uint
    RetryDelayMs uint
}
```

### Step Types

#### 1. retry

Retry the LLM call up to `MaxRetries` times (uint), waiting `RetryDelayMs` milliseconds (uint) between attempts. If all retries fail, the policy proceeds to the next step.

```json
{"type": "retry", "max_retries": 3, "retry_delay_ms": 1000}
```

#### 2. rules_only

Skip the LLM call for this tick and fall back to rule engine evaluation only. The agent executes any matching rules; position/state is unchanged if none match.

```json
{"type": "rules_only"}
```

#### 3. last_decision

Search agent memory for the last LLM decision of the same type and reuse it. If not found, proceeds to the next step.

```json
{"type": "last_decision"}
```

#### 4. skip

Agent does nothing this step (no position change, no action). Simulation continues normally.

```json
{"type": "skip"}
```

### Multi-Step Example

```json
{
  "steps": [
    {"type": "retry", "max_retries": 2, "retry_delay_ms": 500},
    {"type": "last_decision"},
    {"type": "rules_only"}
  ]
}
```

### Fallback Resolution Chain

The effective `FallbackPolicy` for a given agent is resolved in this order (first explicitly set value wins):

1. **Agent-level**: `Agent.LLMConfig.SimulationConfig.FallbackPolicy`
2. **Model default**: `ModelDocument.LLMConfig.DefaultSimulationConfig.FallbackPolicy`
3. **System default**: Retry with `max_retries=2`, `retry_delay_ms=1000`

> **Removed**: `SimulationDocument.FallbackPolicy`. There is no per-run fallback override on the simulation document.

**Code**: `FallbackHandler.ResolveFallbackPolicy(model, agentConfig)`

### CallWithFallback

```go
CallWithFallback(
    ctx context.Context,
    provider LLMProvider,
    prompt string,
    systemMsg string,
    policy FallbackPolicy,
    agent *simulations.SimulationAgent,
) (string, error)
```

**Flow**:
1. Check if context is done (avoid timeout cascades).
2. Attempt LLM call, working through `policy.Steps` in order.
3. If successful, return response.
4. If all steps exhausted, return error.

---

## Prompt Architecture

There is no single `PromptBuilder` type; prompts are assembled by plain functions
in the files that own each call site. **[docs/llm-prompts.md](llm-prompts.md) is
the authoritative reference** for every prompt (anatomy, schemas, PromptContext
resolution, fallbacks). In brief:

- **Level 2a (seeding)** — `buildCountPrompt`, `buildGlobalValuesPrompt`,
  `buildAgentBatchPrompt`, `buildPlacementPrompt` (`modules/simulations/prompt.go`),
  all opening with `systemHeader` ("You initialize an agent-based simulation
  instance…").
- **Level 2b (execution)** — `buildIndividualPrompt`
  (`strategy_individual_full.go`), `buildArchetypePrompt`
  (`strategy_archetype_full.go`), `buildInteractionPrompt` (`llm_runtime.go`),
  `buildShapePrompt` (`strategy_shape.go`), all opening with `runSystemHeader`
  ("You control agents in a RUNNING agent-based simulation…").
- **Shared runtime blocks** — every execution prompt starts with
  `buildSharedRuntimeBlocks` (step line, world + shape listing, environment
  globals) and appends `buildContext` (guidance chain → recent memory → prior
  LLM context → perception "Nearby:" block).
- **Decision vocabulary** — `DecisionMenu` / `DecisionMenuWithHints` /
  `unionDecisionMenuWithHints` and the emit advertisement
  `llmEmittableEventLabels` — event UID + capped `LLMPromptContext` (`decision.go`).
- **Response contracts** — engine-owned structs (`decisionResp`, `propsResp`,
  `movementResp`, `fullDecisionResp`, `fullArchetypeResp`/`fullArchetypeDistResp`,
  `decisionDistResp`, `interactionResp`, `shapePropsResp`); full-control action
  lists are validated by `MapFullActions`/`validateRawAction` (`decision.go`).

Layered construction (as implemented): shared runtime blocks (global) → entity
enumeration with the demux key (instance index / archetype_uid) → settable
property schema (constraints) → guidance + memory + perception context →
strict-JSON schema + example in the system prompt.

Note: `call_llm` rule actions do not build a prompt of their own — they set a
pending flag consumed by the NEXT step's regular decision-purpose call.

---

## Decision Applier

**File**: `modules/simulations/decision_applier.go`

The `DecisionApplier` translates LLM responses into agent state changes.

### Methods

- **`ApplyIndividualDecision()`**: Applies Strategy 2 response (position, properties, state) to a single agent.
- **`ApplyArchetypeStrategy()`**: Applies Strategy 3 response (movement pattern, property rules, state) to all agents of an archetype.
- **`StoreDecisionInCache()`**: Stores high-level decisions (Strategies 4/5) for rule engine evaluation via `ConditionTypeDecision`.

### How LLM JSON is Parsed and Applied

1. **Unmarshal Response**: Parse LLM JSON into `AgentDecision` or `ArchetypeStrategy` struct.
2. **Validate**: Check positions are in bounds, properties respect min/max, states are valid.
3. **Apply Properties**: Update `agent.Properties` with new values.
4. **Apply Position**: Update `agent.Position`, clamping to environment bounds or wrapping (torus mode).
5. **Apply State**: Update `agent.State` to new state-machine state.
6. **Store for Rules**: For decision-only strategies, store decision in `DecisionCache` for `ConditionTypeDecision` evaluation.

### Example Flow — Strategy 4 (Individual Decision)

```
LLM Response JSON ("action": "hunt")
    ↓
DecisionApplier.StoreDecisionInCache(agentID, {"action": "hunt"})
    ↓
Rule Engine checks ConditionTypeDecision
    ├─ Finds rule with ConditionTypeDecision matching "hunt"
    ├─ Conditions met → executes actions
    └─ Actions modify agent state, position, etc.
```

---

## Integration Example: Strategy 4 (Individual Decision)

```
Simulation Step N
├─ For each agent (10 total)
│  ├─ buildIndividualPrompt (decision-label spec)
│  │  ├─ buildSharedRuntimeBlocks (step, world + shapes, globals)
│  │  ├─ agent lines (instance idx, id, state, pos, properties JSON)
│  │  ├─ buildContext (guidance chain, memory ≤ MemorySize,
│  │  │                prior LLM context, "Nearby:" ≤ PerceptionRadius)
│  │  └─ decision menu with hints + emit advertisement (system spec)
│  │
│  ├─ TokenEstimator.CountTokens(prompt) → ~2500 tokens
│  ├─ TokenEstimator.ValidateTokenBudget(2500, 128000, 1000) → OK
│  │
│  ├─ FallbackHandler.ResolveFallbackPolicy(model, agentConfig)
│  │  └─ [retry×2, rules_only]
│  │
│  ├─ FallbackHandler.CallWithFallback(ctx, provider, prompt, systemMsg, policy, agent)
│  │  └─ provider.Simple(ctx, prompt, systemMsg)
│  │     └─ Returns: {"agents":[{"instance":0,"label":"explore","params":{}}]}
│  │
│  ├─ json.Unmarshal() → decisionResp
│  ├─ demuxDecisionsWithState → AgentIntent.Decisions + ra.LastDecision
│  │
│  └─ RuleEngine.EvaluateRules(agent, runtime)
│     ├─ Rule "explore_rule": ConditionTypeDecision "explore"
│     ├─ Condition matches → execute actions
│     │  └─ move_toward(target_uids=["food_source"])
│     └─ Agent position updated

Summary
├─ 10 agents × 1 prompt each = 10 LLM calls
├─ Token usage accumulated into SimulationDocument.TokenUsage.ByModel
└─ All agents moved/acted based on LLM decisions + rule execution
```

---

## Latency Optimization

- Use `decision_frequency` > 1 to call LLM every Nth step (e.g., every 5 steps).
- Batch related agents in a single prompt via `LLMStrategy.BatchSize`.
- Use Strategy 3, 5, or 6 to reduce call count for large populations.
- Set `parallel_workers` (in `LLMSimulationConfig`) to issue concurrent LLM calls within a step.
- Use `stream_batch_size` to tune WebSocket chunk frequency to the frontend.

---

## Model Availability Principle

LLM-bound entities in IntelliSim do not validate their target once at creation time and then trust it forever. Instead, target availability is re-checked at every **use-time** against the live provider registry. This is the model availability principle.

### Rationale

Providers connect and disconnect dynamically over gRPC. A model that was available when a chat was created may become unavailable later if the provider process restarts, the network drops, or the provider's registration changes. A stale record in the database cannot detect this — only a live registry query can.

### How it works today (chats module)

The chats module is the reference implementation. On every operation that touches generation (`SendMessage`, and defensively on `GetChat` for the `model_available` flag), the service calls `checkTarget(chat.LLMTarget)`:

```go
func (s *service) checkTarget(target common.LLMTarget) error {
    for _, provider := range s.llm.Providers() {
        if provider.UID != target.ProviderUID { continue }
        for _, model := range provider.Models {
            if model.UID != target.ModelUID { continue }
            if !slices.Contains(model.Capabilities, common.ModelCapabilityChat) {
                return ErrModelUnavailable
            }
            if target.Effort != "" && !slices.Contains(model.Efforts, target.Effort) {
                return ErrEffortNotAllowed
            }
            return nil
        }
        return ErrModelUnavailable
    }
    return ErrModelUnavailable
}
```

The `ChatResponse` DTO exposes the result as `model_available: bool` — a live field computed at response-build time, not stored in MongoDB.

When the target is unavailable at send time, the API returns 409 `model_unavailable`. The frontend reacts by displaying an availability banner that blocks the chat input until the user selects a different target. See [Chats Module Reference](chats-module.md) for the full error code table and the SSE flow for persisted error-marked assistant messages.

A generation failure (provider disconnects mid-call) is persisted as an **error-marked assistant message** in the database with `code: "model_unavailable"` (or the specific provider error code). This keeps the chat history truthful after reload.

### Future modules (models and simulations)

The models and simulations modules are currently paused mid-migration. When they are re-enabled they will apply the same principle: the model generation endpoint and the simulation execution loop will re-validate their `LLMTarget` at use-time. The `llm.Service.resolve()` validation chain already enforces this for any caller that goes through `llm.Service` (provider lookup → model lookup → effort check → capability check), so the pattern is automatically inherited.

---

## Related Documentation

- [LLM Prompts](llm-prompts.md) — authoritative prompt reference: every prompt site, PromptContext resolution, response contracts, tuning knobs
- [Data Model Reference](data-model.md) — LLMTarget, LLMEffort, LLMConfig, LLMStrategy, LLMSimulationConfig, FallbackPolicy, TokenUsage definitions; per-entity routing resolution order
- [Rule Engine Reference](rule-engine.md) — ConditionTypeDecision, ConditionTypeEvent, call_llm action
- [WebSocket Protocol](websocket-protocol.md) — Real-time simulation step broadcasting
- [Chats Module Reference](chats-module.md) — Reference implementation of the model availability principle

## Footprints & collision in prompts (2026-06-24)

When agents have a simulation footprint (see data-model.md) the generation/execution prompts describe it so the LLM places and reasons about sized bodies:

- The placement prompt's archetype line appends a body hint, e.g. `[body: circle r=3]`, `[body: 4×6 rectangle]`, `[body: 5-point polygon]` (`footprintText`).
- `envOneLiner` and `placementEnvSummary` note the collision mode when `Environment.CollisionMode == "block"` (e.g. "solid bodies (agents cannot overlap shapes/each other)"), so LLM-directed placement avoids overlaps.
- Perception ("Nearby:") neighbour discovery is footprint-aware (edge-to-edge), though the rendered relative `(dx,dy)` stays center-to-center.

`Display` (image/figure) is purely visual and is never sent to any LLM.

## Typed placement config (2026-06-24)

`Placement` is fully typed (the old untyped `Parameters` map is gone). The model-generation LLM contract `LLMPlacement` exposes the same typed fields: `clusters` + `cluster_radius` (clustered placement) and `manual_positions` (exact per-instance coords for `manual` placement). The model-generation merge honors any the LLM sets; all are optional and nil-safe (clustered falls back to derived defaults, manual falls back to random scatter).
