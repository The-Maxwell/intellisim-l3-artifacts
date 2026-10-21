# Engine Config, Events, Memory, Shapes, and Statistics

Architecture reference for the IntelliSim simulation run engine (implemented in
`modules/simulations/`). Covers the configuration knobs the engine honors at
runtime, the event subsystem semantics, the agent memory taxonomy, the per-shape
LLM phase, statistics accumulation, per-step state persistence, and the
determinism invariants the engine preserves.

---

## 1. Engine-Honored vs Reserved Config

### 1a. Model-level simulation config (`LLMSimulationExecutionConfig`)

These fields are read by the run engine from
`Model.SimulationConfig.LLMSimulationExecutionConfig` (model-wide default; the
per-agent knobs live in its embedded `LLMSimulationAgentConfig`) or from
`Agent.LLMConfig.SimulationConfig` (per-archetype override). The agent-level
value wins when set; the model-level value is the fallback.

| Field | Where read | Effect |
|---|---|---|
| `MemorySize` | agent > model | Per-agent FIFO memory cap. `AppendMemory` applies a floor of 20 when the resolved value is 0. |
| `PerceptionRadius` | agent > model | Radius (world units) for the "Nearby:" prompt block. Neighbours within this radius are sorted by ID and capped at 10 for byte-stable prompts. 0 disables the block for that agent. |
| `ParallelWorkers` | model only (no per-agent or per-sim override) | Number of concurrent `errgroup` goroutines in `runLLMPhase`. Falls back to `RunConfig.DefaultParallelWorkers` (env `SIMULATIONS_DEFAULT_PARALLEL_WORKERS`, default 4) when 0. Floored to 1. |
| `RetryTimeoutMs` | model | Run-level per-call timeout fallback (ms). Used by `resolveCallTimeout` when `target.TimeoutMs` is unset. |
| `MaxRetries` | model | Run-level retry budget fallback. Used by `resolveRunMaxRetries` when the fallback policy has no explicit `FallbackRetry` step. |
| `FallbackPolicy` | agent > model | Ordered fallback chain (retry → rules_only / last_decision / skip). |
| `ArchetypeMode` | per-agent strategy | `"identical"` (apply one directive to all members) or `"distribution"` (LLM returns weighted outcomes; each member samples one via `agentJitterRNG`). Empty resolved to `"identical"` at runtime for backward compatibility. Effective only for archetype agents with `type="full"` or `type="decision"`. |
| `VariationStrength` | per-agent strategy | Scale factor (0..1; default 0.15 when unset/0; clamped to 1) for per-agent jitter: numeric property deltas AND movement speed/heading jitter (via the `MovementIntent.Resolved` seam). Replaces the former hardcoded `jitterFrac = 0.15`. Effective only for archetype agents. |

### 1b. Run-time environment knobs (`RunConfig`)

Parsed from environment variables at startup.

| Env variable | Default | Effect |
|---|---|---|
| `SIMULATIONS_MAX_CONCURRENT_RUNS` | 4 | Global engine concurrency cap (RunManager). |
| `SIMULATIONS_STEP_FLUSH_SIZE` | 50 | Timeseries batch size before a forced write. |
| `SIMULATIONS_STEP_FLUSH_INTERVAL_MS` | 500 | Max age (ms) of buffered steps before a flush. |
| `SIMULATIONS_DEFAULT_STEP_DELAY_MS` | 100 | Default inter-step delay when the client does not set a speed. |
| `SIMULATIONS_STREAM_TICKET_TTL_S` | 30 | Single-use WebSocket connect ticket lifetime. |
| `SIMULATIONS_DEFAULT_PARALLEL_WORKERS` | 4 | LLM phase concurrency when the model config is 0. |
| `SIMULATIONS_GENERATION_TIMEOUT` | 300s | Per-LLM-call ceiling for both the generation pipeline and run-time LLM calls (maps to `RunConfig.GenerationTimeout`). |
| `WS_ALLOWED_ORIGIN` | `http://localhost:3000` | Allowed `Origin` header for WebSocket upgrades. |

### 1c. Per-call timeout resolution (`resolveCallTimeout`)

The effective timeout for each LLM call is resolved with the following precedence
(most specific wins):

1. `target.TimeoutMs` — explicit per-target override
2. `simCfg.RetryTimeoutMs > 0` — model-level run config
3. `RunConfig.GenerationTimeout > 0` — env ceiling
4. `defaultRunCallTimeout = 300s` — built-in last resort

### 1d. Stream batch size (`StreamBatchSize`)

`StreamBatchSize` is a field on `LLMSimulationExecutionConfig` (model level only —
there is no per-archetype or per-simulation override). `resolveStreamBatchSize`
(`engine.go`) reads `sim.ModelSnapshot.SimulationConfig.LLMSimulationExecutionConfig.StreamBatchSize`
and, when the value is 0 or the snapshot is nil, falls back to the code constant
`streamFrameDefaultBatch = 200` (`stream.go`). There is **no environment-variable
knob** for the WebSocket frame batch size.

`SIMULATIONS_STEP_FLUSH_SIZE` (→ `RunConfig.StepFlushSize`, default 50) is a
**separate, unrelated** knob: it controls how many `SimulationStep` timeseries
documents are buffered before a forced MongoDB write (see the `flushSize` variable
in `engine.go`'s run loop). It has no effect on WebSocket chunking.

### 1e. Fallback policy resolution (`resolveFallback` / `resolveModelFallback`)

`FallbackPolicy` is an **ordered list of steps** that governs what happens when an
LLM call fails (error, timeout, or unparseable output). Each step has a `Type`
(`retry` / `rules_only` / `last_decision` / `skip`); `retry` additionally carries
`MaxRetries` + `RetryDelayMs`.

**Which policy applies** (`resolveModelFallback`, `llm_runtime.go`): the per-agent
override (`Agent.LLMConfig.SimulationConfig.FallbackPolicy`) wins when non-nil;
otherwise the model default
(`Model.SimulationConfig.LLMSimulationExecutionConfig.FallbackPolicy`). Both are
pointers — nil = "inherit / unset".

**How a policy is applied** (`resolveFallback`, `llm_runtime.go`):

1. The **first** `retry` step is lifted out as the leading retry budget
   (`MaxRetries` + `RetryDelayMs`); the LLM call is retried that many times before
   any post-retry stage runs. (A run-level `MaxRetries` config is used only when
   the policy has **no** explicit `retry` step — see `resolveRunMaxRetries`.)
2. The remaining non-retry steps become the ordered **post-retry stages**, tried
   in sequence once retries are exhausted:
   - `rules_only` — run the agent's rule engine for this step (no LLM).
   - `last_decision` — reuse the agent's previously recorded decision.
   - `skip` — the agent does nothing this step.

**Empty / nil default:** when the resolved policy is nil or has no steps,
`resolveFallback` returns **no retries and a single implicit `rules_only` stage**
— i.e. a missing configuration degrades gracefully to "run the rules". New models
are **not** stamped with any explicit default (unlike `ArchetypeMode`); an unset
`FallbackPolicy` therefore always means this implicit `rules_only` behavior. This
is the source of the model editor's "no steps → inherit default" hint.

### 1f. Reserved / not yet produced

| Symbol | Status |
|---|---|
| `SimulationStatusQueued` | Declared in `data/simulations/simulation.go` but not produced by the engine today. |

---

## 2. Event Subsystem Semantics

### 2a. Event lifecycle

Events are queued in `RuntimeState.events` as `runtimeEvent` structs during the
step pipeline. At the start of each tick, `pruneEvents` drops events whose TTL
has elapsed (`step > ev.EmitStep + def.TTLSteps`). A nil definition means the
event lives only on its emit step (TTL = 0).

### 2b. Scope (authoritative delivery gate)

Scope is the single authoritative field controlling which agents receive an event.
`eventReaches` in `rules.go` implements the three scopes:

| Scope | Delivery rule |
|---|---|
| `targeted` | Reaches only agents whose archetype UID is in `def.ConsumerUIDs`. Empty `ConsumerUIDs` means nobody is targeted. |
| `local` | Reaches agents within `def.Radius` of the emit position AND (if `ConsumerUIDs` is non-empty) whose archetype UID is in the set. A nil position or radius always returns false. |
| `global` (default) | Reaches all agents; `ConsumerUIDs` when non-empty acts as an optional archetype filter. |

### 2c. Producer authorization (`authorizeEmit`)

Before an event is queued, `authorizeEmit` checks the emitter's kind and
archetype UID against the event definition's `Producers` and `ProducerUIDs` lists:

- If `def == nil` or `def.Producers` is empty: permissive (no restriction).
- Otherwise: the emitter's `EventProducer` kind must be in `def.Producers`, AND
  the emitter's archetype UID must be in `def.ProducerUIDs` (if that list is
  non-empty).

Denied emits produce no event, no memory entry, and no stat increment.

### 2d. Producer kinds

| Kind | Who uses it | Where |
|---|---|---|
| `EventProducerAgent` | Rule actions (`ActionTypeEmitEvent`) fired by agent rules | `applyEmitEvent` called from `ApplyAction` |
| `EventProducerLLM` | LLM-requested emit intents (`AgentIntent.Emits`) | `ApplyIntentActions` (sequential mutation sub-phase) |
| `EventProducerEnvironment` | Environment-scoped rules (`AppliesTo` contains `EnvironmentUID`) | `runEnvironmentRules` (once per step, after agent mutation) |

LLM-emitted events are authorized at decode time by `validateRawAction` and then
double-checked in `applyEmitEvent` at apply time. They are drained during the
sequential mutation sub-phase (never during the parallel LLM gather phase) so
`rs.events` appends are single-threaded.

### 2e. Advisory payload validation

`validateEventPayload` validates event payload data against the declared
`EventDefinition.Payload` schema. Warnings are appended to `RuntimeState.warnings`
but the event is **never rejected**. Checks:

- Missing required field keys.
- Type mismatch for `number` and `boolean` typed fields.
- Value not in `AllowedValues` set.
- Non-map payload when the schema declares more than one field.

### 2f. Environment rules (formerly "environment events")

`runEnvironmentRules` runs once per step after the agent mutation phases (rules +
intents + interactions + spatial) and before spawn/remove. It iterates `rs.rules`
(pre-sorted by priority), selects rules whose `AppliesTo` contains
`models.EnvironmentUID`, evaluates conditions via `evalEnvCondition` (handles
`global_property`, `step`, `random`, `composite`; all agent-positional conditions
return false), and executes:

- `emit_event` via `applyEmitEvent` with `EventProducerEnvironment`;
- `set_property` / `modify_property` against `Env.GlobalProperties` via the
  shared descriptor-driven helpers (declared descriptors only; immutable +
  min/max clamping honored). Writes apply immediately (the phase is serial on
  the real rs) and land in the SAME tick's env step row; agents evaluate them
  next tick.

Any other action type warns once (`warnOnce`) and is skipped. The environment is
represented by a synthetic `RuntimeAgent` with `ID=0`, `AgentUID=EnvironmentUID`,
and `nil` position. Env-scoped rules honor `CooldownSteps` / `ExecuteOnce` via
run-level `envCooldowns` / `envOnceFired` maps, and a fired rule counts toward
`RuleExecutions` / `ByRule`.

Environment-emitted events become perceivable by agents on the next tick (they
are queued after the agent rules phase has already run for the same tick).

### 2g. Runtime-mutable global properties

Environment global properties are writable at runtime from three paths:

1. **Env-scoped rules** (above) — immediate serial writes.
2. **Agent rule actions** — `set_property` / `modify_property` with
   `TargetUID == "environment"` route to `applyGlobalPropertyAction`. Purpose
   gating is unchanged (set/modify → properties purpose), mirroring agent
   property ownership.
3. **LLM full-control intents** — the same `target_uid:"environment"` contract,
   validated at decode time by `validateRawAction` against the declared global
   descriptors and advertised in both full-control prompt specs.

Undeclared globals are never materialized (warnOnce + no-op — the environment is
schema'd: conditions, widgets, and the env step row key off declared
descriptors). In synchronous mode all agent-side global writes are DEFERRED
(`rs.deferGlobalWrites` + `rs.globalWrites` queue; the workView's queue is nil'd
and folded back like spawns) and drained serially after the commit phase in
insertion order — `set` = last-write-wins, `modify` = accumulate — matching what
sequential mode produces for the same queue order while preserving frozen reads.

### 2g. `EventDefinition.Locked` (editor flag, not a runtime field)

`EventDefinition.Locked` is an **editor-only** flag that guards the event
definition against model-generation LLM edits — the same role as `Rule.Locked`
and `Interaction.Locked`. When set, `mergeEvents` (`llmmapper.go`) skips
overwriting the definition from LLM output, preserving it as a user-authored
element.

`Locked` is **not read by the runtime event subsystem**. Neither `authorizeEmit`,
`eventReaches`, `validateEventPayload`, nor any rule/intent draining code
consults this field. Runtime authorization is controlled exclusively by
`def.Producers` (which producer kinds may emit) and `def.ProducerUIDs` (optional
archetype filter for producers).

---

## 3. Memory Taxonomy

### 3a. Memory types

| Type constant | String value | When recorded |
|---|---|---|
| `AgentMemoryMovement` | `movement` | Agent changes position (spatial phase `applySpatial`) |
| `AgentMemoryDecision` | `decision` | Rule fires a transition/spawn/remove/emit action; LLM issues a transition |
| `AgentMemoryInteraction` | `interaction` | Agent-agent or agent-shape interaction completes |
| `AgentMemoryProperty` | `property` | Rule or intent sets/modifies a property |
| `AgentMemoryRule` | `rule` | A rule fires (records rule name/ID + condition count); per-article taxonomy |
| `AgentMemoryLLM` | `llm_response` | LLM returns a `LLMNote` (summary kept for context control); per-article taxonomy |

`movement` and `property` are implementation extensions not named in the article's
taxonomy. `rule` and `llm_response` are article-faithful additions.

### 3b. Storage and cap

`AppendMemory` (`memory.go`) appends one `AgentMemory` entry and FIFO-evicts
the oldest entries beyond the cap. The cap is resolved at agent creation time by
`resolveMemoryCap`:

1. `arch.LLMConfig.SimulationConfig.MemorySize` if > 0 (agent-level override)
2. `model.SimulationConfig.LLMSimulationExecutionConfig.MemorySize` if > 0 (model-level default)
3. `defaultMemoryCap = 20` (package floor)

Memory is stored in `SimulationAgent.Memory` and is persisted via the timeseries
indirectly (the agent's memory is part of the runtime state, not the step document;
it influences future prompts but is not written to `simulation_steps`).

### 3c. Feedback into prompts

Every runtime prompt's user message starts with `buildSharedRuntimeBlocks`
(`llm_runtime.go`): the step line, the world summary + shape listing (spatial
models, capped at 12 instances), and the current environment global values
(key-sorted). It lives outside `buildContext` so batched archetype calls carry it
once. `buildContext` (`llm_runtime.go`) then assembles per-group prompt context
in this order:

1. **Guidance chain** —
   `model.SimulationConfig.LLMSimulationExecutionConfig.PromptContext` (the
   execution config; NOT `Model.LLMConfig.PromptContext`, which is the
   model-generation config) → archetype `LLMConfig.PromptContext` (the
   generation-time-only hint `AgentInitialization.Placement.PromptContext` —
   reused for the population/count and placement prompts — is intentionally
   excluded from runtime calls).
2. **Recent memory** — `SummarizeMemory(ra, memBudget)` renders all memory types
   newest-first up to the effective cap.
3. **Prior LLM context** — `SummarizeMemoryByType` with types
   `[AgentMemoryLLM, AgentMemoryRule]`, capped at 5 entries ("Prior LLM context
   for agent N:" block). This implements the article's "контроль контексту"
   (context control) requirement.
4. **Perception / "Nearby:" block** — for spatial models with a non-zero
   `PerceptionRadius`: neighbours within radius, sorted by ID (byte-stable),
   capped at 10, rendered as `archetype_uid @ (dx,dy) state=<state>`.

`AgentMemoryLLM` entries are written by `ApplyIntentActions` when
`AgentIntent.LLMNote != ""`, absorbing the per-agent LLM-note flush into the
unified memory path.

---

## 4. Per-Shape LLM Phase

### 4a. Overview

`runShapePhase` (`strategy_shape.go`) is invoked once per step on the real
`RuntimeState`, before the agent LLM phase. It groups runtime shapes by
`ShapeUID` in first-seen deterministic order, and for each group with a
non-empty, non-`none` `LLMShapeStrategy`, issues one LLM call via
`planShapeUpdates` using purpose `LLMUsagePurposeSimulationShape`.

Running before `runLLMPhase` means agents perceive the updated shape properties
this tick (the spatial index has already been rebuilt by the perceive phase).

### 4b. Target resolution

`resolveShapeTarget` performs a three-level field-wise merge mirroring the agent
resolver: `model.SimulationConfig.LLMSimulationExecutionConfig.Target` is the
base, `Sim.LLMExecutionTargetOverride` overlays it, and the shape's own
`LLMConfig.Target` overlays that. The shape phase is part of Level-2 execution,
so it reads the execution config — not the model-generation config.

Agent target resolution (`resolveTarget` in `llm_runtime.go`) is the same
three-level field-wise merge: model execution default →
`Sim.LLMExecutionTargetOverride` → agent archetype `LLMConfig.Target`, with the
most specific non-empty field winning at each level.

### 4c. Prompt and response

`buildShapePrompt` assembles:
- System: the shared run header (`runSystemHeader`, "You control agents in a
  RUNNING… simulation") + task description + JSON schema
  (`{"properties": {"<uid>": <value>}}`) + the settable-UID rule.
- User: the shared runtime blocks (step / world / globals via
  `buildSharedRuntimeBlocks`), shape name/UID/type + description, the settable
  (mutable-only) property schema lines, current property values of the first
  group member (all members share the same UID and semantically equivalent
  properties), and guidance from
  `model.SimulationConfig.LLMSimulationExecutionConfig.PromptContext` +
  `shape.LLMConfig.PromptContext`.

Property keys in the prompt are sorted for byte-stable output. See
[llm-prompts.md](llm-prompts.md) for the full prompt reference.

On success, `applyShapeUpdates` writes the returned map to every group member's
`Properties`. Immutable properties (per the model shape descriptor) are silently
skipped.

### 4d. Fail-soft

An LLM error for one group leaves that group's shapes unchanged for the tick;
other groups and the rest of the step pipeline are unaffected.

### 4e. Determinism

Because the shape phase runs on the real `RuntimeState` before any agent clones
are made, and groups are iterated in fixed first-seen UID order, writes are
deterministic across reruns with the same inputs. The shape phase does not use
`rs.RNG` (no random calls), so it does not disturb the deterministic RNG sequence.

---

## 5. Statistics Accumulation

### 5a. Accumulator

`statsAccumulator` (`runtime.go`) is allocated once per run in `NewRuntimeState`
and stored as a pointer (`rs.stats`) so shallow copies of `RuntimeState` (e.g.,
`workView := *rs` in the synchronous mutation path) share the same accumulator.
The accumulator is never replaced after allocation; only its fields are mutated.

### 5b. Counters and where they are incremented

| Counter | Incremented in | What it counts |
|---|---|---|
| `AgentsSpawned` | `applySpawns` | Each agent materialised from a spawn queue entry |
| `AgentsRemoved` | `applyRemovals` | Each agent dropped from the live set |
| `Interactions` | `ProcessInteractions` | Each interaction that fires (including UseLLM-mediated) |
| `Events` | `applyEmitEvent` (after auth gate passes) | Each event successfully queued |
| `RuleExecutions` | `ApplyRules` | Each rule that fires at least one action |
| `MovementDistance` | `applySpatial` | Euclidean distance (with torus correction) per actual position change |
| `ByRule[ruleID]` | `ApplyRules` | Per-rule fire count |
| `ByEvent[eventUID]` | `applyEmitEvent` | Per-event-UID emit count |
| `ByInteraction[interactionID]` | `ProcessInteractions` | Per-interaction fire count |
| `PopSpawned[uid]`, `PopRemoved[uid]` | `applySpawns`, `applyRemovals` | Per-archetype spawn/remove counts |
| `PopPeak[uid]`, `PopMin[uid]` | `accumulatePopulation` (after spawn/remove) | Per-archetype population peak and minimum (lazy-init on first observation) |

### 5c. `ComputeStatistics` merge

`ComputeStatistics` (`stats.go`) is called by the run loop after each step to
produce a `SimulationStatistics` snapshot. It:

1. Computes `StepsCompleted`, `TotalAgents`, `Population` (per archetype: current
   count + `ByState` distribution), and `Properties` (per-archetype numeric
   avg/min/max) from the current live agent set — these are derivable from a
   single snapshot.
2. Merges the accumulator's run-history counters (flat counters + `ByRule` /
   `ByEvent` / `ByInteraction` / `PopPeak` / `PopMin` / `PopSpawned` /
   `PopRemoved`) into the snapshot.
3. Appends zero-current rows for extinct archetypes (archetypes that had agents
   earlier in the run but have none alive at this tick).
4. Sorts `ByRule`, `ByEvent`, `ByInteraction`, and extinct archetype rows by key
   for deterministic output.

The merged snapshot is the source of truth for the Level-3 PDF analysis and the
`text_log` events widget.

---

## 6. Per-Step State Persistence

`buildStepRows` (`step.go`) assembles one `SimulationStep` document per live
agent per tick. Each agent row carries:

| Field | Value |
|---|---|
| `Metadata.Type` | `MetadataTypeAgent` |
| `Metadata.UID` | Archetype UID (`Agent.UID`) |
| `Metadata.ReferenceID` | Runtime instance ID (unique within the simulation) |
| `Position` | Current position (nil for non-spatial agents) |
| `Properties` | Deep copy of agent's current property map |
| `State` | Current state-machine state string (`omitempty` — omitted when empty) |

**Shape rows.** After the agent rows and before the environment row, one row per
*recordable* shape instance is appended (`Metadata.Type = MetadataTypeShape`).
`shapeRecordable` selects a shape template when it declares at least one property
**and** either that property set contains any mutable property or the shape has a
non-`none` LLM shape strategy — static decorative shapes are never recorded. Each
shape row carries `Metadata.UID` = shape **template** UID, `Metadata.ReferenceID` =
shape **instance** id, `Position`, and a deep copy of the instance's `Properties`
(no `State`). Shape rows are sorted by instance ID.

An additional environment row with `MetadataTypeEnvironment` is appended when the
simulation has an environment with at least one global property. It carries
`GlobalProperties` but no `State`.

Emission order within each tick is deterministic: agents (id-sorted) → recordable
shapes (id-sorted) → environment.

The `State` field was added to `SimulationStep` as part of the remediation
(previously omitted).

### 6a. WS frame parity

The live WebSocket step stream (`StreamMessage`) mirrors this persistence:

- `agents` — one `AgentFrame` per live agent (id, position, properties, `state`).
- `shapes` — one `ShapeFrame` per recordable shape instance (`{id, properties}`),
  attached to the **first chunk only** when a step is split across chunks; keyframe
  frames include them too.
- `global_properties` — the environment globals, also first-chunk-only.

---

## 7. Determinism Invariants

The engine preserves the following invariants to ensure that reruns with the same
seed produce the same sequence of states.

| Invariant | Mechanism |
|---|---|
| **RNG seeding** | `RuntimeState.RNG` is seeded from `sim.Seed` at run start; the same seed + model produces the same sequence. |
| **Synchronous mode clone/commit** | In `UpdateModeSynchronous`, each agent receives a working copy before mutation; neighbour/condition reads resolve against the frozen live set; working copies are committed atomically. No mid-tick cross-agent observation. |
| **Rule sort** | `rs.rules` is sorted by descending `Priority` (stable for equal priority) once at run start. Per-agent rule evaluation uses this pre-sorted slice. |
| **Agent group order** | `groupByArchetype` and `groupShapesByUID` produce groups in first-seen UID order (slice iteration order is fixed given a fixed `Sim.Agents` ordering). |
| **Parallel LLM gather** | `runLLMPhase` uses `errgroup` with each goroutine writing only to a pre-sized disjoint `results[i]` slot. The serial merge after `eg.Wait()` is the only write to the shared `intents` map. No `rs.RNG` use occurs in goroutines. |
| **Token fold mutex** | `llmRuntime.mu` guards `usage.AddRun` calls from parallel Plan goroutines. |
| **Perception prompt sort** | Neighbours are sorted by agent ID before rendering the "Nearby:" block. Shape property keys are sorted before rendering shape prompts. |
| **Stats accumulator sharing** | `rs.stats` is a pointer; `workView := *rs` copies the pointer, not the accumulator. Both the real `rs` and its synchronous `workView` accumulate into the same struct. This means spawn/remove stats from the synchronous path (which goes through `workView`) correctly land in the shared accumulator. |
| **Extinct archetype sort** | `ComputeStatistics` collects extinct UIDs from the accumulator maps and sorts them before appending, so the `Population` slice order is deterministic even when archetypes die mid-run. |
| **Step document sort** | Live agents are sorted by instance ID before `buildStepRows` assembles the slice, so the timeseries documents arrive in a fixed order for a given tick. |
| **Archetype distribution sampling** | In `distribution` mode, each agent samples its outcome using `agentJitterRNG` seeded as `Seed ^ (Step << 32) ^ agentID` — a distinct stream per (run, step, agent) triple. The draw is local to the apply call and never advances `RuntimeState.RNG`. In `identical` mode, `len(DecisionDist) == 0` and `len(ActionVariants) == 0` guards skip all `pickChoice` calls, so existing seeded runs reproduce byte-for-byte with zero extra RNG draws. |
| **Flow-limit budgets (U10)** | `rs.flow` is a pointer; the synchronous `workView := *rs` shares it like `stats`/`warned`, so fractional budgets carry across ticks in both update modes. Accrual happens exactly once per tick (`applySpatial` runs once per tick in both modes); admissions process in ascending instance id; zero RNG use. With no `congestion.flow_limit.*` globals the gate is never entered. |
| **Density factors (U10)** | Speed–density factors are computed at spatial-phase start from the frozen pre-integration (post-rules) positions via a locally built index (or a linear scan without one) — order-independent in both update modes; zero RNG draws; self is excluded so an isolated agent's factor is exactly 1.0, and factors scale a local velocity copy (never the sticky `ra.Velocity`). With no `congestion.speed_density.*` globals the integration path is byte-identical to pre-U10. |

## Rule-action completions: interact + call_llm (2026-07-02)

Both previously-no-op rule actions are now implemented:

**`interact` (force-fire).** A fired `interact` action (rule or LLM full-control
intent; `TargetUID` carries the `models.Interaction.ID`) queues a
`forcedInteraction` on `rs.forced`; `ProcessInteractions` drains the queue after
the regular pass the same tick (rules/intents run before interactions in both
modes). Forced fires SKIP the interaction's `Conditions` and `UseLLM` mediation
(the firing rule/LLM already made that decision) but KEEP geometric eligibility:
`agent_agent` picks the nearest eligible target (explicit `(BodyDistance, agent
id)` tie-break), `agent_shape` needs an overlapping target shape (Env.Shapes
order), `agent_global` always fires. A per-drain dedup set keyed
`initiator|interaction` fires each pair at most once per step (self-chaining
interactions terminate); the drain loop is index-based so entries appended
during the drain are processed. Invalid requests (missing/unknown/disabled
interaction, non-initiator archetype, no eligible target) warn once and no-op.
Synchronous mode: the workView's COPIED slice header is how the drain sees the
rules-phase queue — `workView.forced` is intentionally NOT nil'd; nothing folds
back.

**`call_llm` (on-demand LLM decision).** `applyCallLLM` requires a
decision-capable resolved strategy (non-none with `UseForDecisions`; warnOnce
otherwise), sets the runtime-only `RuntimeAgent.llmCallPending` flag, and records
a `decision` memory event. The NEXT step's LLM phase consumes the flag:
`decisionDueMembers` / `decisionDueGroups` include only pending members when
`DecisionFrequency` is not due (a scheduled call satisfies pending requests);
archetype directives are fanned only to pending members via shallow group
copies. Flags are consume-once even when the call fails (fail-soft —
LastDecision replay / rules cover). `call_llm` deliberately has NO entry in
`actionPurpose`: the old mapping to the decision purpose suppressed the action
for exactly the agents whose LLM strategy could honor it. Same-step consumption
is impossible by design (rules run after the LLM phase; a mid-step LLM pass
would break the batched parallel-gather and frozen-read invariants).

**Archetype LastDecision replay.** When an archetype DECISION directive is
missing/failed, each live member with a prior `LastDecision` replays its own
label (per-member — in distribution mode these are the per-member sampled
labels). The full kind is intentionally not replayed (parity with
individual_full).

**Memory summary collapse.** `SummarizeMemory` / `SummarizeMemoryByType`
collapse CONSECUTIVE identical `(Type, Summary)` entries into one line suffixed
`` ×N`` rendered at the run's newest step (for ByType, adjacency is judged on
the filtered stream). A collapsed run consumes one line of the budget.

**Per-run lookup maps.** `NewRuntimeState` builds read-only maps (`eventDefs`,
`shapesByUID`, `modelShapeDefs`, `interactionsByID`, `envPropDefs`) used by
`eventDefByUID` / `findShapeByUID` / `interactionByID` / `envPropDef` /
`groupShapesByUID` with linear-scan fallbacks for test-built states. `warnOnce`
dedups run warnings via a shared `warned` set (reference-shared with the
synchronous workView).

**Synchronous warnings fold-back.** `workView.warnings` is nil'd with the other
queues and folded back after `applySpatial` — interactions-phase advisory
warnings (e.g. event payload validation) were previously dropped with the copied
slice header.

**Live-agent query.** `RunManager.LiveAgent` answers
`GET /:id/agents/:agentId` with a deep-copied snapshot via a `ctrlAgentQuery`
control message handled at a step boundary (or immediately while paused), so the
read never races the pipeline.

Additional determinism invariants introduced by this batch:

| Invariant | Mechanism |
|---|---|
| **Forced-interaction order** | Queue order = rules-phase agent iteration order (ascending id) then interactions-phase appends; serial drain; explicit (BodyDistance, id) target tie-break. |
| **Global-write fold** | Synchronous drain in insertion order (rs queue first, then folded workView queue); frozen reads until after commit. |
| **call_llm flag** | Cloned + committed with the synchronous working copy (same path as LastDecision); consumed inside Plan, which is safe under the parallel gather because group member sets are disjoint. |

## Archetype movement heading jitter fix (2026-06-28)

Previously `resolveMovementForAgent` computed a jittered velocity but `applySpatial`
discarded it by re-calling `resolveMovement` on the raw directive. The fix introduces a
`MovementIntent.Resolved *Vector` seam: when an archetype strategy pins a jittered velocity
it sets `Resolved`; `applySpatial` integrates that vector verbatim instead of re-deriving it.
Individual strategies leave `Resolved` nil and follow the unchanged resolution path
(byte-identical). `VariationStrength` now scales both numeric-property jitter and movement
speed/heading jitter uniformly through this seam.

## Exact-geometry spatial model & collision response (2026-06-24)

Agents now carry a geometric **footprint** (see data-model.md). All engine spatial tests use exact geometry (`modules/simulations/collision.go`):

- **`CollisionBody`** is the unified resolved footprint built by `bodyForAgent`/`bodyForShape` (rect = center+half-extents; polygon = absolute CCW verts + convex-piece decomposition via ear-clipping; line shapes are `Open` polylines with no interior; nil/point footprint ⇒ zero-extent point).
- **`Overlap`** (SAT for polygons, exact per kind-pair, touching counts), **`BodyDistance`** (edge-to-edge, 0 on overlap), and **`BodyOverlapsShape`** (replaces `ShapeContains` at collision sites). Regression invariant: `Overlap(pointBody, shapeBody) == ShapeContains(shape, point)` for non-boundary points, so **point agents behave exactly as before**.
- **Torus** is handled by `minImageShift` (nearest wrapped image) before every flat-plane test.
- **Spatial index**: center-keyed as before; `Rebuild` tracks `maxBoundingRadius`; `WithinBody` over-fetches candidate cells by `r + queryBoundR + maxBoundingRadius` then exact-filters with `BodyDistance`. `Nearest` stays center-based (movement target anchor).
- **Boundary**: `IntegrateMovementBody` keeps the whole footprint in-bounds (AABB clamp / torus wrap); a point body is byte-identical to the legacy `IntegrateMovement`.

**Collision-response mode** (`Environment.CollisionMode == "block"`, default off): `resolveCollisions` (`collision_resolve.go`) runs in the serial spatial sub-phase inside `applySpatial` (after integration; in synchronous mode on the working copies before commit, so the clone/commit + parallel-gather invariants are untouched). Determinism: agents in ascending instance-id order; static shapes/agents are solid obstacles; agent-vs-agent Gauss-Seidel by id, capped at 4 iterations, MTV push-out (split for two movables) with a +X coincident tie-break and a 1px separation margin; a pushable agent still inside a solid shape after the cap is cancel-moved to its pre-phase position (guaranteeing no agent ends inside a solid shape). `Collidable==false` agents are skipped. Movement-distance/`moved`-memory accounting happens after resolution (so pushes are counted). With `none` mode the resolver is a no-op and behavior is identical to before.

## Congestion via environment globals — U10 (2026-08-03)

An opt-in, domain-neutral congestion capability (crowds, traffic, queues) configured
entirely through `Environment.GlobalProperties` — **no data-structure changes**; the keys are
ordinary numeric globals (precedent: the conventional `color` property). All logic lives in
`modules/simulations/congestion.go` + two guarded hooks in `applySpatial`.

**Key convention** (numeric values; absent keys ⇒ mechanism off, byte-identical to pre-U10):

| Key | Meaning |
|---|---|
| `congestion.speed_density.radius` | density kernel radius, world cells (> 0) |
| `congestion.speed_density.onset_density` | D0, agents/cell²; f(D ≤ D0) = 1.0 exactly |
| `congestion.speed_density.jam_density` | Djam, agents/cell² (> onset) |
| `congestion.speed_density.min_speed_factor` | factor floor at Djam ((0,1]; must stay > 0 — anti-deadlock) |
| `congestion.flow_limit.<shape_uid>` | max admissions into that shape per step (fractional OK) |
| `congestion.flow_limit_burst.<shape_uid>` | idle-accumulation cap (≥ 1; absent ⇒ 1.0) |

**Speed–density coupling**: per movable agent, local density D = (neighbours within radius,
self excluded) / (π·radius²) over frozen pre-integration positions; the factor (1.0 at ≤ onset,
linear down to the floor at ≥ jam) scales a local velocity copy for this step's integration
only — sticky `ra.Velocity` is never mutated. The spatial phase becomes: resolve velocity →
density-scale (local copy) → integrate (U4 rem) → block collisions → flow gate → accounting.

**Flow limits (turnstile)**: per shape-template-UID fractional budget with clamp-before-accrue
(`budget = min(budget, burst) + rate` once per tick), charged on the outside→overlapping
transition between pre-phase and end-of-phase positions using the same `Overlap` predicate
family as the U8 `position in` condition (gate and removal rule agree exactly). Runs AFTER
`resolveCollisions` (passable gate shapes aren't solid — a crowd push into the gate is charged
or held, closing the pushed-through-unpaid leak) and, by phase order, always binds BEFORE a
`position in` remove rule can see the agent. Denied agents revert to pre-phase position + rem
(U5 revert contract). Instances of one UID share a budget; agents starting/spawning inside a
gated shape are never charged (no entry transition). Malformed keys ⇒ warnOnce + that
mechanism/limit off. Parameterization rule: `min_speed_factor × commanded_speed` (cells/step)
must exceed the largest flow rate a queue feeds, or the queue starves the gate.
