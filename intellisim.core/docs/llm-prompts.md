# LLM Prompts in the IntelliSim Meta-System

Authoritative reference for every prompt the core backend sends to an LLM.
All statements below are derived from the implemented prompt layer:
`modules/models/prompt.go` + `llmcontract.go` (Level 1),
`modules/simulations/prompt.go` (Level 2a),
`modules/simulations/llm_runtime.go`, `strategy_*.go`, `decision.go`, `memory.go` (Level 2b),
and `modules/simulations/analysis_prompt.go` + `analysis_llm.go` (Level 3).

---

## Overview

### The four prompt phases

| Phase | Purpose | Prompt files |
|---|---|---|
| **Level 1 — model generation** | Translate natural language (or reference images) into/onto a structured ABM model | `modules/models/prompt.go` |
| **Level 2a — simulation generation** | Seed one simulation instance from a model: counts, global values, per-instance properties, placement | `modules/simulations/prompt.go`, called from `generator.go` |
| **Level 2b — simulation execution** | Control agents/shapes/interactions per step of a running simulation | `modules/simulations/llm_runtime.go`, `strategy_individual_full.go`, `strategy_archetype_full.go`, `strategy_individual_decision.go`, `strategy_archetype_decision.go`, `strategy_shape.go` |
| **Level 3 — results analysis** | Produce a sectioned analysis report + synthesis over aggregated run evidence | `modules/simulations/analysis_prompt.go` |

### Prompt-site map

| # | Site | Builder | Trigger |
|---|---|---|---|
| 1 | Model generation / edit | `buildSystemPrompt` (`modules/models/prompt.go`) | Model chat send / async generation (`Generator.Generate`) |
| 2 | Vision environment extraction | `buildVisionSystemPrompt` | `Generator.GenerateEnvironmentFromImage` |
| 3 | Population count | `buildCountPrompt` | `CountMode == "llm"` per archetype |
| 4 | Environment global values | `buildGlobalValuesPrompt` | ≥1 global property with `Generation.Mode == "llm"` |
| 5 | Agent property batch | `buildAgentBatchPrompt` | ≥1 agent property with `Generation.Mode == "llm"`, batched |
| 6 | Placement directives | `buildPlacementPrompt` | `Placement.Type == "llm"` per archetype |
| 7 | Individual decision — full kind (concrete actions) | `buildIndividualPrompt` + full-kind `purposeSystemSpec` | `individual_full`, `UseForDecisions` |
| 8 | Individual decision — decision kind (label) | `buildIndividualPrompt` + label `purposeSystemSpec` | `individual_decision`, `UseForDecisions` |
| 9 | Individual properties | `buildIndividualPrompt` + properties spec | either individual strategy, `UseForProperties` |
| 10 | Individual movement | `buildIndividualPrompt` + movement spec | either individual strategy, `UseForMovement` |
| 11 | Archetype decision — full kind, identical (action template) | `buildArchetypePrompt` + `archetypeSystemSpec` | `archetype_full`, `ArchetypeMode == identical` |
| 12 | Archetype decision — full kind, distribution (weighted variants) | same | `archetype_full`, `ArchetypeMode == distribution` |
| 13 | Archetype decision — decision kind, identical (one label) | same | `archetype_decision`, identical |
| 14 | Archetype decision — decision kind, distribution (weighted labels) | same | `archetype_decision`, distribution |
| 15 | Archetype properties | same (mode-independent spec) | archetype strategies, `UseForProperties` |
| 16 | Archetype movement | same (mode-independent spec) | archetype strategies, `UseForMovement` |
| 17 | Interaction mediation | `buildInteractionPrompt` (`llm_runtime.go`) | `Interaction.UseLLM` fires |
| 18 | Shape phase | `buildShapePrompt` (`strategy_shape.go`) | shape has a non-`none` `LLMShapeStrategy` |
| 19 | Analysis section | `buildSectionPrompts` (5 section kinds: dynamics, spatial, emergence, events, rules) | Level-3 analysis pipeline |
| 20 | Analysis synthesis | `buildSynthesisPrompts` | after sections complete |

The `hybrid` and `rules_only` strategies add no prompt sites of their own:
`hybridStrategy` (`strategy_hybrid.go`) resolves each archetype's own strategy and
delegates to sites 7–16; `rulesOnlyStrategy` issues no LLM calls.

### Design principles

- **Strict JSON everywhere.** Every system prompt states the output contract, embeds
  the exact schema, and carries at least one parseable example. The response format
  is core-controlled (`llm.ResponseFormatJSON` on every invocation) — never a user
  tunable.
- **Role split by phase.** Generation prompts (2a) open with `systemHeader`
  ("You initialize…"); execution prompts (2b) open with `runSystemHeader`
  ("You control agents in a RUNNING… simulation"). Level 1 and Level 3 have their
  own role framings.
- **Guidance is data-driven.** All free-text steering comes from `PromptContext`
  fields in the data model, rendered as bullets via `guidanceChain`/`writeGuidance`
  (see the resolution table below). Empty contexts render nothing.
- **Deterministic, byte-stable rendering.** Map keys are sorted (globals, current
  shape values, agent property JSON via `json.Marshal`), neighbours are sorted by
  agent ID, archetype groups keep first-seen order — reruns build identical prompts.
- **Fail-soft.** Every call site has a non-LLM fallback (deterministic random,
  rules, last decision, fire-by-default, skip); a provider outage never aborts a
  generation or a run.

---

## Shared building blocks

### `systemHeader` vs `runSystemHeader`

- `systemHeader` (`modules/simulations/prompt.go`) — prepended to every **Level 2a
  generation** system prompt: *"You initialize an agent-based simulation instance
  from its model. Output STRICT JSON only, matching the given schema exactly — no
  prose, no markdown."*
- `runSystemHeader` (`modules/simulations/llm_runtime.go`) — prepended to every
  **Level 2b execution** system prompt (agent purposes, interaction mediation,
  shape phase): *"You control agents in a RUNNING agent-based simulation, deciding
  one step of its execution. Ground every choice in the provided state, memory,
  perception and guidance. Output STRICT JSON only…"*

The literal substring `"Output STRICT JSON only"` in both headers is asserted by
tests and relied on by providers to suppress prose/markdown.

### `userJSONReminder` — who appends it

`userJSONReminder` (`prompt.go`) = *"Respond with a single valid JSON object
only."* The OpenAI Responses API requires the literal word "json" in the **user**
message (not the system/instructions field) before it honours
`text.format=json_object`, so the reminder lives in the user message:

- Level 2a: each of the four builders appends it to the user message itself.
- Level 2b: `llmRuntime.callJSON` appends it to every user message before invoking.
- Level 3: `analysisCaller.callJSON` appends its own variant, `analysisJSONReminder`
  (*"Return ONLY the JSON object, with no markdown fences or commentary."*).

### `guidanceChain` / `writeGuidance`

`guidanceChain(parts...)` turns an ordered chain of prompt-context strings into
one bullet per non-empty entry; `writeGuidance` emits them under a
`"Modeling guidance:"` heading, or nothing when every entry is empty. Each level's
chain order is fixed (model-level config → entity-level config → field-level hint).

### `buildSharedRuntimeBlocks` (Level 2b only)

Every execution prompt's user message **starts** with this block
(`llm_runtime.go`):

1. **Step line** — `Step N of M.` (or `Step N.` when the run has no step budget).
2. **World summary** (spatial models) — `World: WxH`, plus `toroidal (wrap-around)
   edges` and/or `solid bodies (agents cannot overlap shapes/each other)`
   (`CollisionMode == "block"`) when applicable, followed by the shape instances
   (`Shapes (usable as movement target_ref by uid):` — name, uid, type, center,
   size), capped at `worldShapeCap = 12` with a `… and N more` line.
3. **Environment globals** — `Environment globals: k=v …`, key-sorted. The LLM may
   read them and, in full-control prompts, write declared ones via
   `target_uid:"environment"`.

It deliberately lives **outside** `buildContext`: `buildContext` runs once per
archetype group in a batched archetype call and would duplicate these blocks.

### `buildContext` (Level 2b per-group context)

`buildContext(rs, group, members)` (`llm_runtime.go`) assembles, in order:

1. **Guidance chain** —
   `Model.SimulationConfig.LLMSimulationExecutionConfig.PromptContext` →
   archetype `LLMConfig.PromptContext`. (`Model.LLMConfig.PromptContext` — the
   model-generation context — deliberately does NOT appear; nor does the
   generation-time-only `Placement.PromptContext`.)
2. **Recent memory** — per driving agent: `SummarizeMemory(ra, cap)` under
   `"Recent memory of agent <id> (<archetype>):"`. Cap = agent
   `MemorySize` override, else `defaultMemoryCap = 20`.
3. **Prior LLM context** — per driving agent:
   `SummarizeMemoryByType(ra, [llm_response, rule], 5)` under
   `"Prior LLM context for agent <id>:"` — the article's "контроль контексту"
   feedback loop (most recent LLM notes + rule fires, max 5 lines).
4. **Perception ("Nearby:") block** — spatial models only, per driving agent with
   a position and a resolved `PerceptionRadius > 0` (agent override → model
   execution-config default → 0 = off): neighbours within radius, sorted by agent
   ID, capped at 10, rendered as `<archetype_uid> @ (+dx,+dy) state=<state>`.

### Memory taxonomy rendering

`SummarizeMemory` / `SummarizeMemoryByType` (`memory.go`) render newest-first
lines `- step N [type]: summary`, collapsing CONSECUTIVE identical
`(Type, Summary)` entries into one line suffixed ` ×N` (each collapsed run costs
one line of the budget; for the by-type variant, adjacency is judged on the
filtered stream). The six memory types are `decision`, `interaction`, `movement`,
`property`, `rule`, `llm_response`; only `llm_response` + `rule` feed the
"Prior LLM context" block.

---

## PromptContext resolution

Every `PromptContext`-like field in the data model, and exactly which prompt
builders consume it (verified by grep over `modules/`):

| Data-model field | Consumed by | Phase |
|---|---|---|
| `Model.LLMConfig.PromptContext` (model-generation config) | `researcherGuidance` → `buildSystemPrompt` and `buildVisionSystemPrompt` (`modules/models/prompt.go`), injected as a `## RESEARCHER GUIDANCE (user-authored, follow it)` section (Level 1 text + vision). Level 2a deliberately uses the sim-generation config instead (B2), and Level 2b uses the execution config (B3, asserted by `prompt_runtime_test.go`) — no cross-purpose leak. | 1 |
| `Model.SimulationConfig.LLMSimulationGenerationConfig.PromptContext` | `modelPromptContext` → all four Level 2a builders: `buildCountPrompt`, `buildGlobalValuesPrompt`, `buildAgentBatchPrompt`, `buildPlacementPrompt` (`modules/simulations/prompt.go`) | 2a |
| `Model.SimulationConfig.LLMSimulationExecutionConfig.PromptContext` | `buildContext` (all Level 2b agent-purpose prompts + interaction mediation, `llm_runtime.go`); `buildShapePrompt` (`strategy_shape.go`) | 2b |
| Agent/archetype `LLMConfig.PromptContext` | `agentPromptContext` → `buildCountPrompt`, `buildAgentBatchPrompt`, `buildPlacementPrompt` (2a); `buildContext` for the archetype driving the call (2b, incl. interaction mediation via the initiator's archetype) | 2a + 2b |
| Shape `LLMConfig.PromptContext` | `buildShapePrompt` only | 2b |
| Property `LLMPromptContext` | `propGuidance` → `propSchemaLine` — rendered wherever a property schema line appears: `buildGlobalValuesPrompt`, `buildAgentBatchPrompt` (2a); `settablePropertyLines` (individual + archetype properties prompts and full-kind decision prompts) and `buildShapePrompt` (2b) | 2a + 2b |
| Property `Generation.PromptContext` (`GenModeLLM` hint) | Same `propGuidance` path as `LLMPromptContext` (the two are joined with `"; "`). Note: although intended as a seeding hint, `propSchemaLine` renders it in every property schema line, so it also reaches the 2b settable-property blocks. | 2a + 2b |
| `AgentInitialization.Placement.PromptContext` | `buildCountPrompt` (population/count guidance — the single initialization-level hint) and `buildPlacementPrompt`. Explicitly excluded from runtime `buildContext`. | 2a |
| `Environment.LLMPromptContext` | `buildGlobalValuesPrompt` only | 2a |
| `Interaction.LLMPromptContext` | `buildInteractionPrompt` — appended as an extra guidance bullet after `buildContext` | 2b |
| `Action.LLMPromptContext` | `decisionLabelHint` (`decision.go`) — the FIRST non-empty context among a decision label's triggering-rule actions is appended (capped at 100 runes) after the trigger types in `DecisionMenuWithHints`. Also projected into the Level-1 CURRENT MODEL slice (`actionsToLLM`, `modules/models/prompt.go`) as editable data. Note: `call_llm` actions do NOT inject it into the resulting decision call. | 2b |
| `EventDefinition.LLMPromptContext` | `llmEmittableEventLabels` (`decision.go`) — rendered as `"<uid> (<context>)"` (context capped at 100 runes via `capRunes`) in the emit advertisement on the decision-spec and full-control specs. Also projected into the Level-1 CURRENT MODEL slice (`eventToLLM`). | 2b |

> Note on Level 1: all `llm_prompt_context` fields are shown to the
> model-generation LLM inside the CURRENT MODEL JSON slice — as *data it may
> rewrite* (the approved contract exception in `llmcontract.go`), not as prompt
> instructions. "Consumed" above means "injected as guidance into a prompt".

---

## Level 1 — model generation

### System prompt anatomy (`buildSystemPrompt`, `modules/models/prompt.go`)

Assembled in order:

1. **`systemPromptHeader`** — role ("model-generation engine (Level 1) of an
   LLM-driven meta-system for ABM"), the STRICT output contract (single JSON
   envelope, no prose/fences; never emit ids/timestamps/ownership/llm_config/
   targets/strategies/locked/immutable), referential integrity ("environment" is
   a reserved uid; all references must resolve), and **incremental editing at
   both levels**: return only added/changed entities; within a changed entity emit
   only changed fields (+ its uid/id); an empty array clears a list, omission
   keeps it.
2. **Scope-trimmed schema** (`schemaDocFor`) — only the in-scope sections'
   schema docs are emitted (`agents`, `environment`, `rules`, `interactions`,
   `events`, `widgets`; model scalars only for whole-model scope). Notable
   embedded nudges:
   - **Geometry nudge** (`schemaDocEnvironment`): *"geometry width/height/radius
     are OBJECTS — `{"value":n}` fixed or `{"min":a,"max":b}` ranged — never a
     bare number."* (Backstopped by the tolerant `LLMScalarSpec.UnmarshalJSON`,
     see Response contracts.)
   - **Interaction nudge** (`schemaDocInteractions`): *"initiator_actions apply to
     the INITIATOR, target_actions to the TARGET (e.g. when a predator consumes
     prey, remove_agent belongs in target_actions)."*
   - **Widget/source matrix** (`schemaDocWidgets`): which source types each widget
     type accepts.
3. **`schemaDocSemantics` legend** — included when the scope is whole-model,
   `rules`, or `interactions`. One line per non-obvious family, verified against
   the engine (`modules/simulations/rules.go`): proximity/count/barrier/memory/
   decision/composite/random conditions; interact (force-fire by interaction id),
   call_llm (on-demand decision next step), transform (turn into the target
   archetype: replacement spawned at the agent's position, original removed the
   same step), teleport, emit_event (requires "agent" producer),
   set/modify_property targeting `"environment"` globals.
4. **CURRENT MODEL (baseline to edit)** — `currentSliceJSON` marshals the
   contract projection (`projectToContract`) of the in-scope slice (narrowed to
   `scope.UID` when set) so the LLM edits against real current values.
5. **FROZEN entities** — `lockedSummary` lists locked in-scope entities as
   `- <kind> <id> ("name")` under *"do NOT modify, rename, or remove these"*.
6. **Scope instruction** (`scopeInstruction`) — whole model / one section / one
   entity ("Return only the … array containing just that one entity").
7. **`envelopeContract`** — restates the exact wrapper
   `{"display_message": "...", "model": { … }}` plus a compact one-agent few-shot
   so the model anchors on the envelope rather than emitting a bare model.

**Language:** `display_message` must be written *"in the researcher's language
(the language of their latest message)"* (stated in the header).

The user message is the researcher's chat message; prior chat history is passed
as `Messages` with `OverflowStrategyCut` (`Generator.Generate`,
`modules/models/generator.go`).

### Envelope contract (`modules/models/llmcontract.go`)

`LLMGenerationEnvelope{display_message, model}` wraps `LLMModel` — a stripped DTO
set mirroring only structural/behavioral fields (no llm_config/targets/locked/
immutable/ids/timestamps). Field-level retention: scalars whose zero value is
meaningful are pointer-typed (`omitempty`) so "absent" ≠ "explicit zero"; nil
slices keep the previous list, non-nil (possibly empty) slices replace it.
`LLMScalarSpec.UnmarshalJSON` tolerantly accepts a bare non-negative integer
(decoded as `{"value":n}`) in addition to the documented object form.

### Vision variant (`buildVisionSystemPrompt`)

Frames an image→environment extraction: analyse the reference image(s) and
produce ONLY `model.environment`. Multiple images are labelled
(`"Reference image 2 — obstacles: …"`) and must be SYNTHESIZED into one coherent
layout. Contract: shapes as rectangle/circle/line/polygon templates; integer GRID
coordinates with origin (0,0) top-left; colour encoded as a property
`{uid:"color", type:"color", "#RRGGBB"}`; descriptive snake_case uids. Appends
`schemaDocEnvironment`, a `TARGET GRID` block with the existing width/height when
set, and the same `envelopeContract`.

---

## Level 2a — simulation generation

All four builders live in `modules/simulations/prompt.go` and are invoked from
`modules/simulations/generator.go`. Every system prompt = `systemHeader` + task
line + schema + example; every user message ends with `userJSONReminder`.
Guidance chain base = sim-generation config context (`modelPromptContext`) —
NOT `Model.LLMConfig.PromptContext` (B2).

| Call | System schema | User content | Guidance chain | Fallback |
|---|---|---|---|---|
| **Count** (`buildCountPrompt`) | `{"count": <uint>, "rationale": "<short>"}` + example | Model name/description; environment one-liner; archetype name/description; already-resolved counts of other archetypes (sorted); count bounds (min/max "stay within"); suggested baseline (`PopulationCount`); balance instruction | sim-gen config → agent config → `Placement.PromptContext` | `ResolveCount` (algorithmic) |
| **Globals** (`buildGlobalValuesPrompt`) | `{"values": {"<uid>": <value>}}` + data-derived example (`globalsExample`) | Model line; environment one-liner; one `propSchemaLine` per LLM-mode global | sim-gen config → `Environment.LLMPromptContext` | randomized `ResolveValue` per property |
| **Agent batch** (`buildAgentBatchPrompt`) | `{"instances":[{"instance":<0-based>,"values":{…}}]}` + "one entry per index 0..N-1, do not skip or repeat" + data-derived example (`batchExample`) | Archetype line; batch range ("instances i..j of the population"); context summary (final count + env one-liner); `propSchemaLine` per LLM property; diversity instruction ("DIVERSE, plausible… keep each instance internally consistent") | sim-gen config → agent config | randomized values (also fills gaps per instance via `MapAgentBatch`) |
| **Placement** (`buildPlacementPrompt`) | Directives: `{"directives":[{"kind":"cluster\|region\|at_shape\|explicit","fraction":…,"center":…,"radius":…,"polygon":…,"shape_uid":…,"points":…}]}`; rules: FEW directives (1–6), fraction = population share, explicit points only when count ≤ 50; example | `placementEnvSummary` (dimensions, integer coordinate range, torus, solid-body warning, shape list with uids/centers/sizes); archetype line **with footprint hint** (`footprintText`, e.g. `[body: circle r=3]`); agent count; context summary | sim-gen config → agent config → `Placement.PromptContext` | random placement (`placeRandomFallback`) |

Details:

- **`propSchemaLine`** renders `uid (type) [min..max] allowed{…} — description;
  guidance: <LLMPromptContext; Generation.PromptContext>` — bounds/allowed/
  description/guidance segments omitted when absent.
- **`envOneLiner`** appends `", toroidal (wrap-around) edges"` and
  `", solid bodies (agents cannot overlap shapes/each other)"` when applicable.
- **Batching**: `SIMULATIONS_LLM_BATCH_SIZE` (default 40) instances per property
  batch call.
- **Reasoning routing**: count and placement calls prefer the provider's
  `Reasoning()` API when the target model advertises reasoning; globals and
  batch calls use `Simple()` (`genRun.callJSON` `reasoningPreferred` flag).
- **Target resolution** (`resolveAgentTarget`): sim `LLMGenerationTargetOverride`
  → agent `LLMConfig.Target` → model sim-generation config target.
- Every response is parsed with plain `json.Unmarshal`; malformed JSON degrades
  to the same deterministic-random fallback with a `GenerationWarning`.

---

## Level 2b — execution

### Strategy × purpose matrix

`selectStrategy` (`strategy.go`) maps `LLMStrategyType × Agent.Type` to a
strategy; `enabledPurposes` gates purposes by the `UseFor*` flags in fixed order
(decision, properties, movement, interaction). The interaction purpose issues no
batch call — participation is mediated per interaction (`MediateInteraction`).

| Prompt | Strategies | System spec | Demux key |
|---|---|---|---|
| Individual (per-agent) | `individual_full`, `individual_decision` (properties/movement also reused by decision strategy) | `purposeSystemSpec` (`strategy_individual_full.go`) | 0-based `instance` index **within the batch** |
| Archetype (per-archetype directive) | `archetype_full`, `archetype_decision` | `archetypeSystemSpec` (`strategy_archetype_full.go`) | `archetype_uid` |
| Interaction mediation | any (per `Interaction.UseLLM`) | inline in `buildInteractionPrompt` | single object |
| Shape | per-shape `LLMShapeStrategy` | inline in `buildShapePrompt` | flat properties map |

Capability validation: full strategies call under
`ModelCapabilitySimulationExecution`; decision strategies (incl. their
properties/movement purposes), interaction mediation under
`ModelCapabilitySimulationDecisions`; the shape phase under
`ModelCapabilitySimulationExecution`. Token usage is attributed separately per
sub-purpose (`movement`/`properties`/`decision`/`interaction`/shape).

### Batching + scheduling

- Individual batches: `chunkMembers(members, Strategy.BatchSize)` — ≤ BatchSize
  agents per call (min 1); removed members skipped.
- Archetype batches: `chunkGroups` — ≤ BatchSize ARCHETYPES per call, demuxed by
  `archetype_uid`; groups in one batch share the first group's resolved strategy.
- `DecisionFrequency` gates every purpose (`purposeDue`: run when
  `step % max(1,freq) == 0`). The decision purpose additionally honors on-demand
  `call_llm` requests: off-schedule steps include only pending members
  (`decisionDueMembers` / `decisionDueGroups`; flags consume-once, fail-soft).

### Individual prompt anatomy (`buildIndividualPrompt`)

System = `runSystemHeader` + `purposeSystemSpec(purpose, strategy, menu, emitUIDs)`.
User message:

1. `buildSharedRuntimeBlocks` (step / world+shapes / globals).
2. `Archetype "<uid>" has N agent(s) acting this step:` followed by one
   `writeAgentLine` per member:
   `- instance <i> (id <id>) state="…" pos=(x,y) properties={…}` — the property
   map is `json.Marshal`ed (key-sorted, canonical JSON).
3. **Settable-properties block** when `wantsPropertySchema` (the properties
   purpose, or the FULL-kind decision purpose which may emit
   set/modify_property): `Properties (settable; values are clamped to bounds):`
   + one `propSchemaLine` per declared **mutable** property
   (`settablePropertyLines`; immutable properties excluded).
4. `buildContext` (guidance → memory → prior-LLM → perception).

### Per-purpose system specs (`purposeSystemSpec`)

- **Properties**: "update agent property values"; only DECLARED settable UIDs
  accepted, values clamped, unknown/immutable dropped; schema
  `{"agents":[{"instance":<idx>,"values":{…}}]}`.
- **Movement**: `target_ref` semantics (archetype/shape uid → NEAREST matching
  instance; literal `"x,y"`; empty = nearest agent of any archetype); `speed`
  0..1 of max per-step distance; `hold`/`wander` explained; schema
  `{"agents":[{"instance":<idx>,"intent":"toward|away|wander|hold","target_ref":…,"speed":…}]}`.
- **Decision, decision kind**: "choose one decision label per agent" +
  `Allowed labels: …` from `DecisionMenuWithHints` (or "You may return a free
  label" when the menu is empty); schema
  `{"agents":[{"instance":<idx>,"label":…,"params":{}}]}`; plus the **emit
  advertisement** when `llmEmittableEventLabels` is non-empty: *"Optionally add
  `"emit_uid":"<uid>"` (+ `"emit_data":<payload>`) to an agent entry to emit one
  of: …"* — each entry is the event UID enriched with its (capped)
  `LLMPromptContext`.
- **Decision, full kind**: "choose concrete actions per agent"; **allowed action
  types enumerated**: `set_property, modify_property, move, move_toward,
  move_away, interact, transition, emit_event`; **modifier enumeration**
  `set|add|subtract|multiply|divide|increment|decrement|toggle`; emit_event
  contract (`event_uid` required, only events whose Producers include `"llm"`,
  unauthorized emits silently dropped); env-global targeting
  (`"target_uid":"environment"` on set/modify_property against DECLARED globals);
  move_toward/move_away target_uid follows the movement target_ref semantics;
  full action-object schema + two-action example.

### Decision menu + hints (`decision.go`)

`DecisionMenu(model, archetypeUID)` = de-duplicated `decision`-condition labels
from enabled rules applying to the archetype (composite sub-conditions walked
recursively; first-seen order). `DecisionMenuWithHints` annotates each label with
`" (triggers: t1, t2, …)"` — the ≤4 first-seen action types of the enabled rules
consuming that label — so the LLM knows what each label does. Batched archetype
calls use `unionDecisionMenuWithHints` (first-seen union across the batch, hint
from the first archetype carrying the label). Prompt-side only; validation and
fallbacks keep using the bare `DecisionMenu`.

`llmEmittableEventLabels(model, archetypeUID)` advertises the events the LLM may
emit (Producers include `"llm"`; `ProducerUIDs` empty or listing the archetype),
each rendered as the event UID enriched with the `EventDefinition.LLMPromptContext`
when non-empty (capped at 100 runes via `capRunes`) — e.g. `howl (wolf howl to
rally the pack)`. Authorization is still enforced at demux time (`authorizeEmit`).

### Archetype prompt anatomy (`buildArchetypePrompt`)

System = `runSystemHeader` + `archetypeSystemSpec(purpose, kind, mode, rs, batch)`.
User message: `buildSharedRuntimeBlocks`, then
`N archetype(s) acting this step (one directive each):` and per archetype:

1. `- archetype "<uid>": N agent(s) example state="…" example pos=(x,y)`
   (representative member).
2. **`archetypeAggregateBlock`** — the article's aggregated-statistics view
   (Strategies 3/5): `Population stats: N live; states: a=… b=…` (key-sorted),
   `Property stats: <uid> mean=… min=… max=…;` per declared numeric property
   (declared order), and `Spatial: centroid=(x,y) spread=<RMS>` for spatial
   models. Per-member memory/perception blocks are intentionally kept alongside
   (softer variant per the design spec).
3. Settable-properties block (same rule as individual prompts).
4. Per-group `buildContext` (guidance + that group's members' memory blocks).

`archetypeSystemSpec` mirrors the individual specs with `archetype_uid` as the
key. Under `ArchetypeMode == distribution` the decision/full specs request a
**weighted distribution** instead of a single directive ("each agent
independently samples one… Weights are relative (need not sum to 1)"):
decision kind → `"choices":[{"label","weight","params"}]`; full kind →
`"variants":[{"weight","actions":[…]}]` (each member samples ONE whole variant).
Identical mode emits the legacy single-directive schemas byte-for-byte.
Properties/movement specs are mode-independent. Decision specs carry the same
emit advertisement (union across the batch).

### Interaction mediation (`MediateInteraction` / `buildInteractionPrompt`)

For `UseLLM` interactions whose declared Conditions already passed. System:
`runSystemHeader` + "decide whether an agent interaction fires this step" +
schema `{"fire": <bool>, "params": {…}}`. User: shared runtime blocks;
`Interaction: <name> — <description>`; `Initiator:`/`Target:` lines
(archetype uid, instance id, state, position, key-sorted properties JSON);
`Distance:` (footprint- and torus-aware `BodyDistance`); **stakes** via
`interactionEffectSummary` — `On fire: initiator gets […]; target gets […]`
(first-seen-deduped action types per side); `buildContext` for the initiator plus
the interaction's own `LLMPromptContext` as an extra bullet.
Target resolution uses the initiator's archetype; capability =
`SimulationDecisions`, usage purpose = `interaction`. **Fail-open**: any error /
missing target / malformed response returns `fire=true` (the conditions already
gated it) so a provider outage never disables the model's interactions.

### Shape phase (`buildShapePrompt`, `strategy_shape.go`)

One call per LLM-active `ShapeUID` group per step (before the agent phase, on the
real state, so agents perceive updates the same tick). System: `runSystemHeader`
+ "update properties for an environment shape this step" + settable-UID rule +
schema `{"properties": {"<uid>": <value>}}`. User: shared runtime blocks;
`Shape: <name> (uid=…, <type>) — <description>`; settable-property schema lines
(mutable only); `Current values:` from the first group member (key-sorted);
guidance = execution-config context (B3 — formerly the model-generation config)
→ shape `LLMConfig.PromptContext`. Target: model execution default →
`Sim.LLMExecutionTargetOverride` → shape `LLMConfig.Target` (field-wise merge,
`resolveShapeTarget`). Usage purpose `simulation_shape`, attributed to the
ShapeUID. Fail-soft: the group is left unchanged for the tick; immutable
properties are skipped at apply time too.

### Response application & validation (context for the contracts below)

Full-control action lists decode into `rawAction` and pass through
`MapFullActions`/`validateRawAction` (`decision.go`): unknown verbs, unknown or
immutable properties, unknown states, and unauthorized emits are dropped;
`target_uid:"environment"` writes are validated against declared global
descriptors. emit_event intents are partitioned into `AgentIntent.Emits`
(producer kind `EventProducerLLM`); movement is always routed via
`intent.Movement` (never applied twice). Archetype directives fan out via
`ApplyDirectiveWithVariation` with per-agent seeded jitter/sampling.

---

## Level 3 — analysis

### Sections and evidence pack

`plannedSections` (`analysis_pipeline.go`): `dynamics` when population/properties
focus is enabled; `spatial` when the model is spatial and spatial/interactions
focus is enabled; `events`/`rules` only when EXPLICITLY focused AND the
corresponding statistics map (`ByEvent`/`ByRule`) is non-empty; `emergence`
always. Each section is one LLM call; `marshalEvidence` slices the evidence pack
per section:

| Section | Evidence (JSON keys beyond `model` + `run`) |
|---|---|
| `dynamics` | `population_over_time`, `property_series`, `statistics`, `shape_property_series` (when non-empty) |
| `spatial` | `statistics`, `spatial_snapshots` |
| `emergence` | `population_over_time`, `property_series`, `statistics`, `shape_property_series` (when non-empty) |
| `events` | `by_event` (per-event emission counts), `population_over_time` |
| `rules` | `by_rule` (per-rule fire counts), `population_over_time` |

`shape_property_series` (`EvidencePack.ShapePropertySeries`, built in
`analysis_evidence.go`) carries, per shape template × numeric declared property,
the value averaged across that template's instances over time. It uses the same
thinning/downsampling and the same `PropertyUIDs` selection filter as
`property_series`, and only properties actually recorded in the timeseries are
included. It is marshaled into the `dynamics` and `emergence` sections only, and
is omitted entirely when empty (runs with no recordable shape properties). The section
system prompts explicitly invite the analyst to treat shape properties as
first-class environment dynamics and to consider agent–environment feedback
loops when the key is present.

### Section prompts (`buildSectionPrompts`)

System: expert-ABM-analyst role; a **domain ontology** paragraph (archetypes,
typed properties, state machine, rule engine condition/action families,
interactions, events, spatial environment; per-step timeseries recording); the
section's title + analytical objective (`sectionGuide`); a **grounding + citation
requirement**: *"Ground EVERY claim strictly in the provided evidence; never
invent numbers or entities. Cite concrete numbers and step indices/ranges from
the evidence when making claims. Prefer causal and emergent insight over
restating values."*; the output shape
`{"title", "narrative", "key_findings", "causal_links", "emergent_phenomena"}`
with per-field length guidance; and the language line — *"Write all
natural-language content in English/Ukrainian"* (`languageName` from
`Analysis.Language`, en/uk).

User: the section's evidence JSON between `=== SIMULATION EVIDENCE (JSON) ===`
markers, the researcher's optional `RESEARCHER CONTEXT / QUESTIONS`, and the
production instruction.

### Synthesis (`buildSynthesisPrompts`)

System: conclusion-writer role; synthesize the section analyses + headline
statistics into `{"executive_summary", "conclusions", "recommendations"}`
("Ground everything in the provided material; do not introduce unsupported
claims"); same language line. User: simulation name + steps completed; every
produced section's title, narrative, key findings, causal links, and emergent
phenomena; `=== HEADLINE STATISTICS (JSON) ===` (`marshalStatistics`: statistics
+ population series); researcher context.

### Caller behaviour (`analysis_llm.go`)

`analysisCaller.callJSON` appends `analysisJSONReminder` to the user message,
prefers `Reasoning()` when the target advertises reasoning, retries up to
`AnalysisConfig.MaxRetries` (`SIMULATIONS_ANALYSIS_MAX_RETRIES`, default 1) on
transport error **or** malformed JSON, bounds each call with
`SIMULATIONS_ANALYSIS_TIMEOUT` (default 300s) unless `target.TimeoutMs` is set,
and parses via `extractJSON` (fence/prose-tolerant outermost `{…}`). A failed
section yields a placeholder ("This section could not be generated.") + warning;
the run errors only when no section AND no synthesis was produced
(`ErrAllLLMCallsFailed`).

---

## Response contracts

| Prompt site | Schema (Go type, file) | Parser | Failure fallback |
|---|---|---|---|
| L1 generation / vision | `LLMGenerationEnvelope` (`modules/models/llmcontract.go`); tolerant `LLMScalarSpec.UnmarshalJSON` accepts bare numbers for geometry scalars | `parseEnvelope` (`modules/models/generator.go`) — strips a ```` ``` ````/```` ```json ```` fence, extracts the outermost balanced `{…}` (string-aware), unmarshals; merge is fail-soft (`llmMerge` warnings) | Error surfaced: error-marked assistant message (chat) / `generation.failed` |
| 2a count | `LLMCountResponse` (`modules/simulations/llmcontract.go`) | `json.Unmarshal` in `resolveCountFor` + `MapCount` (bounds clamp, warning) | `ResolveCount` (algorithmic) |
| 2a globals | `LLMGlobalValuesResponse` | `MapGlobalValues` (`llmmapper.go`) — per-property validation, random fill for bad values | randomized `ResolveValue` |
| 2a agent batch | `LLMAgentBatchResponse` | `MapAgentBatch` — validates per instance, random-fills missing/invalid entries | fully randomized batch |
| 2a placement | `LLMPlacementResponse` | `MapDirectives` + `SampleDirectives` (`llmmapper.go` / `placement.go`) | random placement |
| 2b individual full decision | `fullDecisionResp` (`strategy_individual_full.go`) | `callJSON` unmarshal → `demuxFullActions` → `MapFullActions` validation | rules cover the purpose for the batch |
| 2b individual decision label | `decisionResp` (`llm_runtime.go`) | `demuxDecisionsWithState` (+ `authorizeEmit` for optional emits) | per-agent `LastDecision` replay, else rules |
| 2b properties (both scopes) | `propsResp` (`llm_runtime.go`) | `demuxProperties` / archetype branch of `callArchetypePurpose` | rules |
| 2b movement (both scopes) | `movementResp` (`llm_runtime.go`) | `demuxMovement` / archetype branch | rules |
| 2b archetype full (identical) | `fullArchetypeResp` (`strategy_archetype_full.go`) | `mapArchetypeIntent` (validated against a representative member) | rules |
| 2b archetype full (distribution) | `fullArchetypeDistResp` | `mapArchetypeVariants` (empty variants dropped; archetype omitted when none survive) | rules |
| 2b archetype decision (identical) | `decisionResp` | archetype branch + `authorizeEmit` | per-member `LastDecision` replay (decision kind only), else rules |
| 2b archetype decision (distribution) | `decisionDistResp` | `mapArchetypeDecisionDist`; each member samples via `pickChoice` + per-agent RNG | per-member `LastDecision` replay, else rules |
| 2b interaction mediation | `interactionResp` (`llm_runtime.go`) | `callJSON` in `MediateInteraction` | **fire=true** (fail-open) |
| 2b shape | `shapePropsResp` (`strategy_shape.go`) | `planShapeUpdates` | no update this tick |
| L3 section | `llmSectionResponse` (`analysis_llmcontract.go`) | `analysisCaller.callJSON` + `extractJSON` | placeholder section + `section_failed` warning |
| L3 synthesis | `llmSynthesisResponse` | same | `synthesis_failed` warning; whole run fails only if nothing was produced |

All 2b calls flow through `llmRuntime.callJSON` (`llm_runtime.go`): reasoning
routing per advertised flag, core-controlled JSON response format, per-call
timeout, retry budget, then the post-retry fallback stages
(`rules_only`/`last_decision`/`skip` all surface as `ok=false` — "no fresh LLM
intent"; the caller decides). Malformed JSON is not retried at this layer — it
falls through to the fallback stages.

---

## Tuning knobs

| Knob | Where | Effect on prompts/calls |
|---|---|---|
| `BatchSize` (`LLMStrategy`) | per-agent strategy | ≤ N agents per individual call / ≤ N archetypes per archetype call (min 1). Bigger batches = fewer, larger prompts. |
| `DecisionFrequency` (`LLMStrategy`) | per-agent strategy | Purposes run when `step % max(1,freq) == 0`; between runs the engine reuses prior intents. `call_llm` requests bypass it for the decision purpose (pending-only calls). |
| `MemorySize` | agent `LLMConfig.SimulationConfig` > model execution config > floor 20 | Bounds both the stored memory and the "Recent memory" block budget. |
| `PerceptionRadius` | agent > model execution config > 0 | Radius of the "Nearby:" block (cap 10 neighbours, ID-sorted). 0 disables it. Distinct from `InteractionRadius`. |
| `VariationStrength` (`LLMStrategy`) | per-agent strategy | (0..1], default 0.15 when unset — scales per-agent numeric-property jitter and movement speed/heading jitter when archetype directives fan out. Prompt-invisible (applied after the response). |
| `ArchetypeMode` (`LLMStrategy`) | per-agent strategy | `identical` (legacy single-directive schemas) vs `distribution` (weighted choices/variants schemas + per-member sampling). Changes the requested response schema. |
| `PromptContext` fields | see the resolution table | The only free-text steering channels. |
| `target.TimeoutMs` / `RetryTimeoutMs` / `SIMULATIONS_GENERATION_TIMEOUT` | target > model execution config > env (default 300s) | Per-call deadline for 2b (`resolveCallTimeout`); 2a uses target timeout else `SIMULATIONS_GENERATION_TIMEOUT`. Level 3 uses `SIMULATIONS_ANALYSIS_TIMEOUT` (300s). |
| Retries | `FallbackPolicy` `retry` step (`MaxRetries` + `RetryDelayMs`) > model `MaxRetries` > 0 | Retry budget per 2b call (`resolveRunMaxRetries`). Level 3: `SIMULATIONS_ANALYSIS_MAX_RETRIES` (default 1). Level 2a: no retries (immediate deterministic fallback). |
| `FallbackPolicy` stages | agent `LLMConfig.SimulationConfig` > model execution config > implicit `rules_only` | Post-retry degradation: `rules_only` / `last_decision` / `skip`. Individual/archetype decision strategies additionally replay `LastDecision` per member. |
| `SIMULATIONS_LLM_BATCH_SIZE` | env (default 40) | Instances per Level-2a property-batch prompt. |
| `ParallelWorkers` | model execution config > env `SIMULATIONS_DEFAULT_PARALLEL_WORKERS` (4) | Concurrency of the 2b LLM phase (prompt content unaffected; group order and merge stay deterministic). |
