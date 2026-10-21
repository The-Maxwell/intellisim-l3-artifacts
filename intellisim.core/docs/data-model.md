# Data Model Reference

This document provides a complete reference for IntelliSim's MongoDB data model, including all collections, document structures, field definitions, and relationships.

## Package Architecture

The data layer is split into two tiers:

| Tier | Packages | Contents |
|------|----------|----------|
| **Storage-agnostic** | `data/{chats,models,simulations,common}` | Repository interfaces, domain entity types (string hex IDs, no bson tags), sentinel errors (`common.ErrNotFound`, `common.ErrInvalidID`), `ListOptions` |
| **MongoDB** | `data/mongodb/{chats,models,simulations,common}` | bson document structs, entity↔document mappers, repository implementations |

**Dependency rule**: only `data/mongodb/*` packages import the MongoDB driver. Modules (`modules/*`) depend exclusively on the interfaces and entities in the storage-agnostic tier.

**ID convention**: Domain entities use `string` hex IDs (e.g. `models.Model.ID`). MongoDB document structs keep `bson.ObjectID` fields (unchanged from before the refactor).

**Error sentinels**: `common.ErrNotFound` and `common.ErrInvalidID` are returned by repository implementations and should be checked by callers instead of driver-specific errors.

**FX wiring**: `data.Module` (package `data`, file `data/module.go`) is an `fx.Option` that binds each repository interface to its MongoDB implementation. It is the only place that names both tiers together.

> **Note on DB migration**: bson field names are unchanged by the 2026-06-06 abstraction refactor — no DB migration is needed. The 2026-06-09 models-module refactor adds a new `locked` field on five entity types; it is `bson:"locked,omitempty"` so absent/false on legacy documents — still no migration.

### ModelsRepository — listing

`models.ModelsRepository` (package `data/models`, file `repository.go`) exposes the usual `Create` / `GetByID` / `Update` / `Delete` plus an owner-scoped, paginated `List` added by the 2026-06-09 models-module refactor:

```go
// ListFilter narrows and pages model listing. Empty CreatedBy = no owner filter
// (admin listing).
type ListFilter struct {
    CreatedBy string
    Page      int // 1-based; values < 1 are treated as 1
    PageSize  int // values < 1 default to 20
}

type ModelsRepository interface {
    Create(ctx context.Context, model Model) (string, error)
    GetByID(ctx context.Context, id string) (*Model, error)
    Update(ctx context.Context, id string, model Model) error
    Delete(ctx context.Context, id string) error
    // List returns one page of models (updated_at desc) and whether more pages
    // exist after it.
    List(ctx context.Context, filter ListFilter) ([]Model, bool, error)
}
```

**`List` semantics** (MongoDB impl in `data/mongodb/models/repository.go`):

- Filters by `created_by` when `ListFilter.CreatedBy != ""`; an **empty** `CreatedBy` returns all owners' models (the admin listing path).
- Sorts `updated_at: -1, _id: -1` (newest-updated first, `_id` tiebreak).
- Page is offset-based: `skip = (Page-1) * PageSize`. Defaults: `Page < 1 → 1`, `PageSize < 1 → 20`.
- Fetches `PageSize + 1` documents to compute the returned `hasMore` bool, then trims back to `PageSize`.

**Supporting index** (`pkg/mongodb/indexes.go`, created by `EnsureIndexes`): compound `models_created_by_updated_at` on the `models` collection — `{created_by: 1, updated_at: -1}` — covering the owner-scoped, recency-sorted `List` query. (The pre-existing `models_created_at` index remains.)

---

## Collections Overview

IntelliSim uses 5 primary MongoDB collections (including 1 timeseries collection). Shared token-accounting types live in `data/common` and are imported by `data/chats` and `data/simulations`.

| Collection | Purpose | Type | Primary Use |
|------------|---------|------|-------------|
| `chats` | Chat sessions for model/simulation creation | Regular | Conversational context for LLM-based generation |
| `chat_messages` | Individual messages within chats | Regular | Chat history for context window management |
| `models` | Agent-based model definitions | Regular | Model templates for simulation runs |
| `simulations` | Simulation execution instances | Regular | Runtime state and configuration |
| `simulation_steps` | Per-step simulation state snapshots | Timeseries | Per-entity state updates for analysis |

Tag policy after the 2026-06-06 abstraction refactor:
- Documents in `data/mongodb/*` carry **`bson` tags only** (they never leave the repository layer).
- Entities in `data/simulations` and `data/common` carry **`json` tags only** (they are serialized directly to the API/WebSocket); entities in `data/chats` and `data/models` carry **no tags** (module mappers translate them to API DTOs).

---

## data/common — Shared Types

Package `data/common` holds four groups of types shared by the chats, models, and simulations packages:

- **`LLMTarget` / `LLMEffort`** — identify WHICH LLM to call and how hard it should think. Shared across all three levels so provider/model/effort selection uses one struct everywhere.
- **`TokenUsage` / `TokenUsageSummary`** — usage attribution. Every entry embeds `LLMTarget` (inline, flat bson keys) so token spend is always attributed to a specific `(provider, model, effort)` triple even when those change mid-chat or mid-run.
- **`Point`** — shared 2D integer grid coordinate used by model shape geometry, simulation agent positions, and step documents.
- **`File` / `FileType`** — metadata for generated simulation artifacts (e.g. PDF analysis reports); bytes live in MongoDB GridFS.

### LLMTarget and LLMEffort

```go
// LLMEffort is the reasoning-effort level requested from the provider.
// Open string so provider-specific values pass through; empty = provider default.
type LLMEffort string

const (
    LLMEffortMinimal LLMEffort = "minimal"
    LLMEffortLow     LLMEffort = "low"
    LLMEffortMedium  LLMEffort = "medium"
    LLMEffortHigh    LLMEffort = "high"
    LLMEffortMax     LLMEffort = "max"
)

// LLMTarget identifies WHICH LLM to call and how hard it should think.
// Core idea of the meta-system: different models for different cases
// (cheap model for simple archetypes, strong one for complex agents).
//
// Resolution order for Level-2 (simulation execution) calls, most specific wins:
//
//   Agent.LLMConfig.Target / ShapeDocument.LLMConfig.Target / EventDefinition.LLMTarget
//           >
//   LLMConfig.Target  (ModelDocument.llm_config.target)
//
// ChatDocument.LLMTarget serves Level-1 and Level-3 conversations only; it does not
// participate in the Level-2 resolution chain. There is no longer a per-run simulation
// target — all Level-2 routing flows through the model snapshot.
type LLMTarget struct {
    ProviderUID string    `bson:"provider_uid,omitempty" json:"provider_uid,omitempty"`
    ModelUID    string    `bson:"model_uid,omitempty"    json:"model_uid,omitempty"` // e.g. "gpt-4o"
    Effort      LLMEffort `bson:"effort,omitempty"       json:"effort,omitempty"`    // empty = provider default
}
```

**Fields — LLMTarget**:

| Field | Type | Description |
|-------|------|-------------|
| `provider_uid` | string | UID of the registered LLM provider to call |
| `model_uid` | string | Provider's model identifier (e.g. `"gpt-4o"`); empty = provider default |
| `effort` | LLMEffort | Reasoning effort level; empty = provider default |

**Enum: LLMEffort**:

| Value | Description |
|-------|-------------|
| `"minimal"` | Lowest reasoning effort (fastest, cheapest) |
| `"low"` | Low effort |
| `"medium"` | Medium effort |
| `"high"` | High reasoning effort |
| `"max"` | Maximum reasoning effort (most capable, most expensive) |

### Point

Shared 2D integer grid coordinate. Used by `ShapeGeometry.Position`, `ShapeGeometry.Points`, `SimulationAgent.Position`, `SimulationShape.Position`, and `SimulationStepDocument.Position`.

```go
type Point struct {
    X uint64 `bson:"x"`
    Y uint64 `bson:"y"`
}
```

Both fields are `uint64` (non-negative grid coordinates). There are no `json` tags — the bson encoding is used for storage; the simulations package defines its own json mapping via the enclosing document tags.

### File and FileType

Metadata for a file generated for a simulation (e.g. an analysis PDF). The file bytes themselves are stored in a MongoDB GridFS bucket and referenced by `GridFSID`. This type was previously named `SimulationFile` in `data/simulations`; it is now canonical in `data/common`.

```go
type FileType string

const (
    SimulationFileAnalysisReport FileType = "analysis_report" // PDF analysis report (Level-3)
)

type File struct {
    UID         string        `json:"uid" bson:"uid"`
    Name        string        `json:"name" bson:"name"`
    Type        FileType      `json:"type" bson:"type"`
    MimeType    string        `json:"mime_type" bson:"mime_type"`
    Size        int64         `json:"size" bson:"size"`
    GridFSID    bson.ObjectID `json:"gridfs_id" bson:"gridfs_id"`
    Step        *int64        `json:"step,omitempty" bson:"step,omitempty"`
    Description string        `json:"description,omitempty" bson:"description,omitempty"`
    CreatedAt   time.Time     `json:"created_at" bson:"created_at"`
    CreatedBy   string        `json:"created_by" bson:"created_by"`
}
```

**Fields — File**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Unique identifier for this file entry within the simulation |
| `name` | string | Human-readable filename (e.g. `"Predator-Prey Analysis.pdf"`) |
| `type` | FileType | Artifact classification — currently `"analysis_report"` |
| `mime_type` | string | MIME type (e.g. `"application/pdf"`) |
| `size` | int64 | File size in bytes |
| `gridfs_id` | ObjectID | File `_id` in the MongoDB GridFS bucket; use this to retrieve the actual bytes |
| `step` | *int64 | Simulation step the analysis covers, if step-scoped; nil = covers the full run |
| `description` | string | Optional human-readable description of file contents |
| `created_at` | time.Time | When the file was generated |
| `created_by` | string | User or system identifier of the generator |

**Enum: FileType**:

| Value | Description |
|-------|-------------|
| `"analysis_report"` | PDF analysis report generated by Level-3 LLM analysis |

### TokenUsage and TokenUsageSummary

```go
// TokenUsage accounts for LLM token consumption for a SINGLE (provider, model, effort)
// target. LLMTarget is embedded inline so bson keys stay flat: provider_uid, model_uid,
// effort. Promoted-field access (u.ModelUID) still compiles; composite literals must nest:
// common.TokenUsage{LLMTarget: common.LLMTarget{...}, ...}
type TokenUsage struct {
    LLMTarget `bson:",inline"`

    PromptTokens     uint64  `bson:"prompt_tokens"          json:"prompt_tokens"`
    CompletionTokens uint64  `bson:"completion_tokens"      json:"completion_tokens"`
    TotalTokens      uint64  `bson:"total_tokens"           json:"total_tokens"`
    Calls            uint64  `bson:"calls"                  json:"calls"`
    LatencyMs        float64 `bson:"latency_ms"             json:"latency_ms"` // cumulative wall-clock ms
}

// TokenUsageSummary aggregates usage that may span MULTIPLE targets. Total is the grand
// total (LLMTarget always empty there); ByModel preserves per-target breakdown so no
// attribution is lost.
type TokenUsageSummary struct {
    Total   TokenUsage   `bson:"total"    json:"total"`
    ByModel []TokenUsage `bson:"by_model" json:"by_model"` // one entry per (provider, model, effort)
}
```

**Fields — TokenUsage** (the `LLMTarget` inline fields appear flat in bson/json):

| Field | Type | bson key | Description |
|-------|------|----------|-------------|
| `provider_uid` | string | `provider_uid` | UID of the LLM provider (from embedded `LLMTarget`) |
| `model_uid` | string | `model_uid` | Provider's model identifier (from embedded `LLMTarget`) |
| `effort` | LLMEffort | `effort` | Reasoning effort level (from embedded `LLMTarget`) |
| `prompt_tokens` | uint64 | `prompt_tokens` | Input tokens consumed |
| `completion_tokens` | uint64 | `completion_tokens` | Output tokens produced |
| `total_tokens` | uint64 | `total_tokens` | Sum of prompt + completion |
| `calls` | uint64 | `calls` | Number of LLM calls rolled into this entry |
| `latency_ms` | float64 | `latency_ms` | Cumulative wall-clock time across all `Calls` |

**Fields — TokenUsageSummary**:

| Field | Type | Description |
|-------|------|-------------|
| `total` | TokenUsage | Grand aggregate; `LLMTarget` fields (`provider_uid`, `model_uid`, `effort`) are always empty here |
| `by_model` | []TokenUsage | One `TokenUsage` entry per `(provider_uid, model_uid, effort)` triple |

---

## ChatDocument

**Collection**: `chats`

> **Storage-agnostic counterpart**: `chats.Chat` (package `data/chats`) — uses a string hex `ID` and no bson tags. `ChatDocument` (package `data/mongodb/chats`) is the MongoDB representation and is the type described in this section.

Represents a chat session for conversational model or simulation creation.

```go
type ChatDocument struct {
    ID          bson.ObjectID             `bson:"_id,omitempty"`
    Type        ChatType                  `bson:"type"`
    Name        string                    `bson:"name"`
    Description string                    `bson:"description"`
    LLMTarget   common.LLMTarget          `bson:"llm_target"`             // which LLM serves this chat (provider, model, effort)
    ReferenceID *bson.ObjectID            `bson:"reference_id,omitempty"` // model or simulation, per Type
    TokenUsage  *common.TokenUsageSummary `bson:"token_usage,omitempty"`  // running aggregate, per-target breakdown
    CreatedAt   time.Time                 `bson:"created_at"`
    UpdatedAt   time.Time                 `bson:"updated_at"`
    CreatedBy   string                    `bson:"created_by"`
    UpdatedBy   string                    `bson:"updated_by"`
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Unique chat identifier |
| `type` | ChatType | Chat purpose — `"plain"`, `"model"`, or `"simulation"` |
| `name` | string | Display name for the chat |
| `description` | string | Human-readable description |
| `llm_target` | LLMTarget | LLM selection for this chat (provider, model, effort); serves Level-1 and Level-3 conversations |
| `reference_id` | *ObjectID | Links to the associated `ModelDocument` or `SimulationDocument` depending on `type`; absent for `"plain"` chats |
| `token_usage` | *TokenUsageSummary | Running token-usage aggregate with per-`(provider, model, effort)` breakdown; updated after each LLM response |
| `created_at` | time.Time | Creation timestamp (UTC) |
| `updated_at` | time.Time | Last modification timestamp |
| `created_by` | string | User identifier |
| `updated_by` | string | User identifier of last editor |

**Enum: ChatType**:

| Value | Description |
|-------|-------------|
| `"plain"` | General conversational chat; no model/simulation reference |
| `"model"` | Chat focused on model building / refinement; `reference_id` → ModelDocument |
| `"simulation"` | Chat focused on simulation configuration / analysis; `reference_id` → SimulationDocument |

---

## ChatMessageDocument

**Collection**: `chat_messages`

> **Storage-agnostic counterpart**: `chats.ChatMessage` (package `data/chats`) — uses string hex IDs and no bson tags. `ChatMessageDocument` (package `data/mongodb/chats`) is the MongoDB representation and is the type described in this section.

Individual messages within a chat session.

```go
type ChatMessageDocument struct {
    ID          bson.ObjectID      `bson:"_id,omitempty"`
    ChatID      bson.ObjectID      `bson:"chat_id"`
    Role        ChatMessageRole    `bson:"role"`
    TextContent string             `bson:"text_content,omitempty"` // plain text / preview
    RawContent  string             `bson:"raw_content,omitempty"`  // full payload (e.g. structured model spec)
    TokenUsage  *common.TokenUsage `bson:"token_usage,omitempty"`  // assistant messages only; model is TokenUsage.ModelUID
    CreatedAt   time.Time          `bson:"created_at"`
    CreatedBy   string             `bson:"created_by"`
}
```

> **Note on images**: Chat messages no longer carry image attachments. Images supplied for Level-1 model generation are persisted as base64 on `ModelDocument.Images` (see [`ModelImage`](#nested-modelimage) below).

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Unique message identifier |
| `chat_id` | ObjectID | Reference to parent `ChatDocument` |
| `role` | ChatMessageRole | Message role — `"user"`, `"assistant"`, or `"system"` |
| `text_content` | string | Plain text preview of the message content |
| `raw_content` | string | Full payload (e.g. structured model spec returned by the LLM) |
| `token_usage` | *TokenUsage | Token usage for this call; set on `"assistant"` messages only; model and provider are on the embedded `TokenUsage.LLMTarget` (promoted: `TokenUsage.ModelUID`, `TokenUsage.ProviderUID`, `TokenUsage.Effort`) |
| `created_at` | time.Time | Timestamp when message was created |
| `created_by` | string | User identifier (or system identifier for non-user messages) |

**Enum: ChatMessageRole**:

| Value | Description |
|-------|-------------|
| `"user"` | Message authored by the researcher |
| `"assistant"` | Response produced by the LLM (model generation output, replies, etc.) |
| `"system"` | System-authored message persisted in the thread itself (e.g. a pinned instruction or a status event surfaced in the chat timeline). This is **distinct** from the per-request system prompt sent to the LLM, which is assembled dynamically at call time and is never stored |

---

## ModelDocument

**Collection**: `models`

> **Storage-agnostic counterpart**: `models.Model` (package `data/models`) — uses a string hex `ID` and no bson tags. `ModelDocument` (package `data/mongodb/models`) is the MongoDB representation and is the type described in this section.

Represents a complete agent-based model definition. All LLM provider selection, strategy defaults, and runtime config defaults are nested under `llm_config`. Agent/shape/property/environment references use **UID** as the canonical identity; `name` is a display label only.

```go
// ModelUpdateMode defines how agent state changes are applied within a tick.
//
//   sequential   - each rule immediately updates agent state; later rules see updated
//                  values within the same tick (order-dependent, faster).
//   synchronous  - all agents compute actions from a snapshot; updates commit at tick end
//                  (deterministic, reproducible).
type ModelUpdateMode string

const (
    UpdateModeSequential  ModelUpdateMode = "sequential"
    UpdateModeSynchronous ModelUpdateMode = "synchronous"
)

type ModelDocument struct {
    ID          bson.ObjectID `bson:"_id,omitempty"`
    Name        string        `bson:"name"`
    Description string        `bson:"description,omitempty"`

    ChatID bson.ObjectID `bson:"chat_id,omitempty"`

    LLMConfig  LLMConfig       `bson:"llm_config"`
    UpdateMode ModelUpdateMode `bson:"update_mode"` // step update semantics; was on SimulationDocument

    IsSpatial       bool   `bson:"is_spatial"`
    SpatialCellSize uint64 `bson:"spatial_cell_size,omitempty"` // uint64 (was float64)

    Agents       []Agent           `bson:"agents"`
    Environment  *Environment      `bson:"environment,omitempty"`
    Rules        []Rule            `bson:"rules,omitempty"`
    Interactions []Interaction     `bson:"interactions,omitempty"`
    Events       []EventDefinition `bson:"events,omitempty"`

    // Images is the managed library of reference/schematic images (inline base64,
    // compressed on add) used for vision-based environment generation. Managed via
    // POST/DELETE /models/:id/images[/:imageId]; not touched by UpdateModel (PUT).
    // Capped at MODELS_MAX_MODEL_IMAGES (default 5).
    Images []ModelImage `bson:"images,omitempty"`

    CreatedAt time.Time `bson:"created_at"`
    UpdatedAt time.Time `bson:"updated_at"`
    CreatedBy string    `bson:"created_by"`
    UpdatedBy string    `bson:"updated_by"`
}
```

**Top-Level Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Unique model identifier |
| `name` | string | Model display name |
| `description` | string | Purpose and context of the model |
| `chat_id` | ObjectID | Reference to the chat that produced this model |
| `llm_config` | LLMConfig | Provider selection, default strategy, and default runtime config |
| `update_mode` | ModelUpdateMode | Step update semantics (`"sequential"` or `"synchronous"`); previously lived on `SimulationDocument` |
| `is_spatial` | bool | Whether the model includes a spatial environment |
| `spatial_cell_size` | uint64 | Grid cell size for spatial indexing (only when `is_spatial=true`); changed from `float64` to `uint64` |
| `agents` | []Agent | Agent / archetype definitions |
| `environment` | *Environment | Spatial environment definition (required when `is_spatial=true`) |
| `rules` | []Rule | Condition-action rules governing agent behavior |
| `interactions` | []Interaction | Inter-agent / agent-shape / agent-global interaction definitions |
| `events` | []EventDefinition | First-class event definitions (producers, consumers, scope, payload) |
| `images` | []ModelImage | Managed library of reference/schematic images (inline base64, compressed); used for vision-based environment generation via `POST /models/:id/environment/generate`. Capped at `MODELS_MAX_MODEL_IMAGES` (default 5). Not modified by `PUT /models/:id` |
| `created_at` | time.Time | Creation timestamp (UTC) |
| `updated_at` | time.Time | Last modification timestamp |
| `created_by` | string | User identifier |
| `updated_by` | string | User identifier of last editor |

**Enum: ModelUpdateMode**:

| Value | Description |
|-------|-------------|
| `"sequential"` | Each rule immediately updates agent state; later rules in the same tick see updated values (order-dependent, faster) |
| `"synchronous"` | All agents compute actions from a snapshot; updates commit at tick end (deterministic, reproducible) |

### Nested: ModelImage

One entry in the model's managed image library. Images are added via `POST /models/:id/images` (compressed inline by `pkg/imageutil` before storage), removed via `DELETE /models/:id/images/:imageId`, and analysed all at once in a single vision LLM call by `POST /models/:id/environment/generate`. The library is capped at `MODELS_MAX_MODEL_IMAGES` (default 5) entries. Chat messages do not carry image attachments; images are persisted here instead.

Each image's `description` (together with a positional label) is forwarded as a text part in the vision prompt so the LLM can distinguish multiple reference images and synthesise one coherent environment from all of them.

```go
type ModelImage struct {
    UID         string `bson:"uid"`
    Name        string `bson:"name,omitempty"`
    MimeType    string `bson:"mime_type,omitempty"` // e.g. "image/png"
    Data        string `bson:"data"`                // base64-encoded, compressed image bytes
    Description string `bson:"description,omitempty"`
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Unique image identifier within the model; generated on add |
| `name` | string | Optional display name supplied by the caller (e.g. `"floor_plan.png"`) |
| `mime_type` | string | MIME type, e.g. `"image/png"` (one of `png`, `jpeg`, `webp`, `gif`) |
| `data` | string | Base64-encoded compressed image bytes (resized to `MODELS_MODEL_IMAGE_MAX_DIM`, capped at `MODELS_MODEL_IMAGE_MAX_BYTES`) |
| `description` | string | Optional label describing what the image shows; sent to the LLM as a positional hint in the vision prompt |

---

### Nested: Agent

Defines an agent or archetype. `UID` is unique within the model and is the identifier used by rules, conditions, actions, and interactions. `Name` is a display label only.

`Type` drives LLM strategy scope: `"agent"` → individual scope; `"archetype"` → archetype scope (see the strategy mapping table under [LLMStrategy](#nested-llmstrategy)).

```go
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

type Agent struct {
    UID         string          `bson:"uid"`   // unique within model; used for targeting
    Name        string          `bson:"name"`
    Description string          `bson:"description,omitempty"`
    Type        AgentType       `bson:"type"`      // "agent" => individual scope, "archetype" => archetype scope
    MoveType    AgentMoveType   `bson:"move_type"`
    Properties  []AgentProperty `bson:"properties"`

    // States is the state-machine definition; the starting state is Initialization.InitialState.
    States []string `bson:"states,omitempty"`

    Initialization AgentInitialization `bson:"initialization"`

    // LLMConfig consolidates per-agent LLM target routing, prompt context, strategy, and
    // simulation config into a single wrapper. nil = inherit model defaults from LLMConfig.
    // Supersedes the previous separate LLMTarget / LLMStrategy / LLMSimulationConfig fields.
    LLMConfig *LLMAgentConfig `bson:"llm_config,omitempty"`

    // Locked, when true, freezes this agent against model-generation LLM edits: the
    // llm merge preserves it verbatim and the prompt lists it as immutable. User-only.
    Locked bool `bson:"locked,omitempty"`

    CreatedAt time.Time `bson:"created_at"`
    UpdatedAt time.Time `bson:"updated_at"`
}
```

**Agent Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Canonical identity within the model; used in `Rule.AppliesTo`, `Condition.TargetUID`, `Action.TargetUID`, `Interaction.InitiatorUIDs`, etc. |
| `name` | string | Display label (e.g. `"Predator"`) |
| `description` | string | What this agent/archetype represents |
| `type` | AgentType | `"agent"` (individual scope) or `"archetype"` (population template) |
| `move_type` | AgentMoveType | `"static"` (cannot move) or `"movable"` (can move) |
| `properties` | []AgentProperty | Dynamic properties (health, age, energy, …) |
| `states` | []string | Valid state-machine states (e.g. `["healthy","infected","dead"]`) |
| `initialization` | AgentInitialization | t=0 setup: population count, placement, initial state |
| `llm_config` | *LLMAgentConfig | Per-agent LLM wrapper (WHICH provider/model/effort + HOW + simulation config); nil = inherit model defaults. Supersedes the previous three separate fields (`llm_target`, `llm_strategy`, `llm_simulation_config`) |
| `locked` | bool | User-only flag; when `true`, freezes this entity against model-generation LLM edits (the merge preserves it verbatim; the prompt lists it as frozen). Distinct from `AgentProperty.Immutable` (which freezes a property *value* at runtime). bson key `locked,omitempty` — absent/false on legacy documents, so no migration is needed |

> **Rename note**: This type was previously named `AgentDocument`; it is now `Agent`. All `modules/` code and mappers that referenced `models.AgentDocument` must use `models.Agent`.

**Enum: AgentType**:

| Value | Description |
|-------|-------------|
| `"agent"` | Individual scope — LLM calls are per-instance |
| `"archetype"` | Archetype scope — LLM calls are per-population-type |

**Enum: AgentMoveType**:

| Value | Description |
|-------|-------------|
| `"static"` | Cannot move; position is fixed |
| `"movable"` | Can move; position changes each step |

### Nested: AgentProperty

```go
type AgentPropertyType string

const (
    PropertyTypeString  AgentPropertyType = "string"
    PropertyTypeNumber  AgentPropertyType = "number"
    PropertyTypeEnum    AgentPropertyType = "enum"
    PropertyTypeBoolean AgentPropertyType = "boolean"
    PropertyTypeColor   AgentPropertyType = "color" // hex color string, e.g. "#FF8800"; by convention the property with UID "color" is what renderers read
)

type AgentProperty struct {
    UID           string            `bson:"uid"`
    Name          string            `bson:"name"`
    Type          AgentPropertyType `bson:"type"`
    Description   string            `bson:"description,omitempty"`
    MinValue      *float64          `bson:"min_value,omitempty"`
    MaxValue      *float64          `bson:"max_value,omitempty"`
    DefaultValue  interface{}       `bson:"default_value,omitempty"`
    AllowedValues []interface{}     `bson:"allowed_values,omitempty"`

    // Immutable locks the property VALUE after initialization: rules and LLM actions must
    // not modify it during the simulation (engine-enforced).
    Immutable bool `bson:"immutable,omitempty"`

    LLMPromptContext string `bson:"llm_prompt_context,omitempty"`
}
```

**Property Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Unique property identifier within its owning entity; used in `Condition.PropertyUID`, `Action.PropertyUID`, `Action.SpawnInheritFrom` |
| `name` | string | Display label (e.g. `"health"`) |
| `type` | AgentPropertyType | Data type — `"string"`, `"number"`, `"enum"`, `"boolean"`, `"color"` |
| `description` | string | Human-readable description |
| `min_value` | *float64 | Minimum value (numeric properties) |
| `max_value` | *float64 | Maximum value (numeric properties) |
| `default_value` | interface{} | Value assigned when agent is created |
| `allowed_values` | []interface{} | Valid values (enum properties) |
| `immutable` | bool | When `true`, the property value is locked after initialization; the rule engine and LLM decision applier must reject writes to this property during the simulation run |
| `llm_prompt_context` | string | Guidance for the LLM about this property's meaning / constraints |

**Enum: AgentPropertyType**:

| Value | Description |
|-------|-------------|
| `"string"` | Text value |
| `"number"` | Numeric (float64) value |
| `"enum"` | Selection from `allowed_values` |
| `"boolean"` | True/false |
| `"color"` | Hex color string (e.g. `"#FF8800"`); by convention the property with UID `"color"` is what renderers read |

**Reserved pseudo-property UIDs**: Two built-in attributes are not user-defined `AgentProperty` entries but can be referenced in conditions and actions by their reserved UIDs:

| UID | Meaning |
|-----|---------|
| `"state"` | Agent state-machine state — used with `ConditionTypeState` / `ActionTypeTransition` |
| `"position"` | Agent spatial position — used with `ConditionTypePosition` |

### Nested: AgentInitialization

t=0 setup required for every agent, whether LLM-driven or rule-only.

```go
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
    Type          PlacementType  `bson:"type"`
    ShapeUID      string         `bson:"shape_uid,omitempty"`      // for at_shape
    PromptContext string         `bson:"prompt_context,omitempty"` // for llm placement
    // Typed, deterministic placement config (replaced the old untyped Parameters map).
    Clusters        *uint64        `bson:"clusters,omitempty"`         // clustered: number of gaussian centres (nil ⇒ max(round(sqrt(count)/2),1))
    ClusterRadius   *uint64        `bson:"cluster_radius,omitempty"`   // clustered: spread in world units (nil ⇒ 0.15*min(W,H))
    ManualPositions []common.Point `bson:"manual_positions,omitempty"` // manual: exact per-instance coords (clamped; padded w/ random; extras ignored)
}

type AgentInitialization struct {
    PopulationCount uint64     `bson:"population_count"` // initial instance count; must be >= 1
    MinPopulation   *uint64    `bson:"min_population,omitempty"`
    MaxPopulation   *uint64    `bson:"max_population,omitempty"`
    InitialState    string     `bson:"initial_state,omitempty"` // must be one of Agent.States
    Placement       *Placement `bson:"placement,omitempty"`     // pointer; nil = default placement
}
```

**AgentInitialization Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `population_count` | uint64 | Initial number of agent instances; must be ≥ 1 |
| `min_population` | *uint64 | Optional lower bound for dynamic population sizing |
| `max_population` | *uint64 | Optional upper bound for dynamic population sizing |
| `initial_state` | string | Starting state for all new instances; must be one of `Agent.States` |
| `placement` | *Placement | How initial positions are determined; pointer (`omitempty`) — nil means engine default |

**Enum: PlacementType**:

| Value | Description |
|-------|-------------|
| `"random"` | Random positions within the environment |
| `"uniform"` | Uniformly distributed (grid / evenly spaced) |
| `"clustered"` | Clustered around one or more points; `parameters` carries cluster radius |
| `"at_shape"` | Placed at / within a specific shape; `shape_uid` references it |
| `"manual"` | Explicit coordinates provided in `parameters` |
| `"llm"` | LLM determines initial placement; `prompt_context` guides it |

### Nested: Environment

`UID` is the reserved constant value `"environment"` (see `EnvironmentUID` const). No agent, shape, or property may reuse it.

```go
const EnvironmentUID = "environment"

type Environment struct {
    UID              string          `bson:"uid"`   // always "environment"
    Width            uint64          `bson:"width"`
    Height           uint64          `bson:"height"`
    Torus            bool            `bson:"torus"`
    Shapes           []ShapeDocument `bson:"shapes"`
    GlobalProperties []AgentProperty `bson:"global_properties,omitempty"`

    LLMPromptContext string `bson:"llm_prompt_context,omitempty"`
}
```

**Environment Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Always `"environment"` (reserved) |
| `width` | uint64 | Grid width in units (changed from `int`) |
| `height` | uint64 | Grid height in units (changed from `int`) |
| `torus` | bool | Whether edges wrap (toroidal topology) or act as hard boundaries |
| `shapes` | []ShapeDocument | Static environmental shapes (obstacles, zones, etc.) |
| `global_properties` | []AgentProperty | Global state accessible to all agents |
| `llm_prompt_context` | string | Environment description for the LLM. Injected as guidance into the simulation-generation global-values prompt (`buildGlobalValuesPrompt`); shown to the Level-1 LLM as editable model data. Not injected into per-step execution prompts; see [llm-prompts.md](llm-prompts.md) |

### Nested: ShapeDocument

> **Color convention:** `ShapeDocument` does not have a dedicated `Color` field. Shape color is represented as a `"color"`-typed `AgentProperty` in `Properties` with the conventional UID `"color"` (e.g. `{UID: "color", Type: "color", DefaultValue: "#FF8800"}`). Renderers must read `Properties["color"]` via the property map; the old `shape.Color` field no longer exists.

```go
type ShapeType string

const (
    ShapeTypeRectangle ShapeType = "rectangle"
    ShapeTypeCircle    ShapeType = "circle"
    ShapeTypeLine      ShapeType = "line"
    ShapeTypePolygon   ShapeType = "polygon"
)

// ShapeDocument is an environment shape template and a first-class placeable entity.
// Color is represented as a "color"-typed property in Properties (conventional UID "color"),
// not a dedicated field. Shapes use the same AgentInitialization mechanism as agents for
// population count and placement.
type ShapeDocument struct {
    UID         string          `bson:"uid"`
    Name        string          `bson:"name"`
    Description string          `bson:"description,omitempty"`
    Type        ShapeType       `bson:"type"`
    Layer       uint            `bson:"layer,omitempty"` // uint (was int)
    Visible     bool            `bson:"visible"`
    Properties  []AgentProperty `bson:"properties"`

    // Initialization carries population count and placement (same mechanism as agents).
    Initialization AgentInitialization `bson:"initialization"`

    // Geometry is the template geometry for non-LLM creation; required when
    // Placement.Type != "llm" (for "llm" it is an optional hint). Geometry.Position,
    // when set, fully fixes the instance position at model level (Placement skipped).
    Geometry *ShapeGeometry `bson:"geometry,omitempty"`

    LLMConfig *LLMShapeConfig `bson:"llm_config,omitempty"`

    // Locked freezes this shape against model-generation LLM edits. User-only.
    Locked bool `bson:"locked,omitempty"`
}
```

> **Note on manual placement:** There is no `ManualInstances` / `ShapeInstanceSpec` field. Manual placement (`Placement.Type == "manual"`) uses `Placement.Parameters` to carry explicit coordinates — the population count and placement parameters together drive instance creation.

**Shape Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Unique shape identifier within the model; used in `Placement.ShapeUID`, `Action.TargetUID`, `SpawnPosition.ShapeUID` |
| `name` | string | Display label |
| `description` | string | What the shape represents |
| `type` | ShapeType | Geometry — `"rectangle"`, `"circle"`, `"line"`, or `"polygon"` |
| `layer` | uint | Z-order for rendering (changed from `int`) |
| `visible` | bool | Whether visible in the UI |
| `properties` | []AgentProperty | Dynamic properties on this shape, including the conventional `"color"` property (type `"color"`, UID `"color"`) that renderers read for visualization color |
| `initialization` | AgentInitialization | Instance count + placement (same struct as agents); placement computes the anchor position of each instance |
| `geometry` | *ShapeGeometry | Template geometry for non-LLM creation (see below); required when `Placement.Type != "llm"`. A set `geometry.position` pins the instance to exact coordinates (placement skipped) |
| `llm_config` | *LLMShapeConfig | Per-shape LLM routing, prompt context, and generation strategy wrapper |
| `locked` | bool | User-only flag; when `true`, freezes this shape against model-generation LLM edits (preserved verbatim by the merge). bson key `locked,omitempty` |

**Enum: ShapeType**:

| Value | Description |
|-------|-------------|
| `"rectangle"` | Rectangular area |
| `"circle"` | Circular area |
| `"line"` | Line segment |
| `"polygon"` | Arbitrary polygon |

### Nested: ShapeGeometry and ScalarSpec

For non-LLM placement the model template must fully describe shape geometry; the simulation
generator combines `Placement` (anchor position per instance) with `Geometry` (dimensions +
style) to materialize runtime `SimulationShape` instances. Alternatively, `Geometry.Position`
fixes the anchor explicitly at model level — the template then carries the complete visual
spec (used e.g. by Level-1 generation from a reference image). Points use `common.Point`
(shared uint64 grid coordinates).

```go
// ScalarSpec is a fixed value or a [Min..Max] random range resolved per instance at
// simulation creation. Exactly one of (Value) or (Min+Max, Min <= Max) must be set.
// All values are uint64 (non-negative grid units).
type ScalarSpec struct {
    Value *uint64 `bson:"value,omitempty"`
    Min   *uint64 `bson:"min,omitempty"`
    Max   *uint64 `bson:"max,omitempty"`
}

// ShapeGeometry is the template geometry used when Placement.Type != "llm".
// Points ([]common.Point) are relative to the anchor — the explicit Position (when set)
// or the anchor computed by Placement; the simulation generator translates them to
// absolute coordinates at creation time.
type ShapeGeometry struct {
    // Position is the absolute anchor position (top-left for rectangle, center for
    // circle, origin for the anchor-relative Points). When set, the simulation
    // generator places the instance exactly here and Placement is not consulted;
    // a fixed Position implies Initialization.PopulationCount == 1.
    Position *common.Point `bson:"position,omitempty"`

    Width     *ScalarSpec    `bson:"width,omitempty"`     // rectangle
    Height    *ScalarSpec    `bson:"height,omitempty"`    // rectangle
    Radius    *ScalarSpec    `bson:"radius,omitempty"`    // circle
    Points    []common.Point `bson:"points,omitempty"`    // polygon / line (anchor-relative)
    Thickness *uint          `bson:"thickness,omitempty"` // line

    Fill        *bool `bson:"fill,omitempty"`
    Stroke      *bool `bson:"stroke,omitempty"`
    StrokeWidth *uint `bson:"stroke_width,omitempty"`
}
```

**Simulation-creation semantics**:

- `Geometry.Position` set: the instance anchor is taken as-is and `Placement` is not
  consulted; `Geometry` → dimensions; style copied as-is. Anchor semantics match the
  runtime engine: rectangle → top-left, circle → center, polygon/line → origin for the
  anchor-relative `Points`.
- `random` / `uniform` / `clustered` / `at_shape`: `Placement` → anchor (x,y) per instance ×
  `PopulationCount`; `Geometry` → dimensions (each `ScalarSpec` range resolved randomly per
  instance); style copied as-is.
- `manual`: `Placement.Parameters` carries explicit coordinates; `PopulationCount` controls how
  many instances are created. There is no `ManualInstances` / `ShapeInstanceSpec` concept.
- `llm`: the LLM plans geometry + positions; `Geometry`, if present, serves as a hint.
- Polygon/line points translate by anchor: runtime point = anchor + template point.

**Validation rules (model save time)**:

- `Placement.Type != "llm"` ⇒ `Geometry` required and type-consistent: rectangle ⇒ `Width` +
  `Height`; circle ⇒ `Radius`; polygon ⇒ ≥ 3 `Points`; line ⇒ exactly 2 `Points`.
- `ScalarSpec`: exactly one of `Value` / (`Min` and `Max`, `Min ≤ Max`) set; values > 0.
- `Geometry.Position` set ⇒ `Initialization.PopulationCount == 1`.

---

## Rules, Conditions, and Actions

### Condition

All entity references in conditions use UIDs (not names).

```go
type ConditionType string

const (
    ConditionTypeGlobalProperty ConditionType = "global_property" // check global property
    ConditionTypeProperty       ConditionType = "property"        // check agent property
    ConditionTypePosition       ConditionType = "position"        // check spatial position
    ConditionTypeProximity      ConditionType = "proximity"       // nearby agents/shapes within radius
    ConditionTypeStep           ConditionType = "step"            // check simulation step number
    ConditionTypeComposite      ConditionType = "composite"       // AND/OR/NOT combination of sub-conditions
    ConditionTypeCount          ConditionType = "count"           // count agents in area
    ConditionTypeBarrier        ConditionType = "barrier"         // check if movement is blocked
    ConditionTypeState          ConditionType = "state"           // check agent state (pseudo-prop "state")
    ConditionTypeDistance       ConditionType = "distance"        // distance to a target
    ConditionTypeDecision       ConditionType = "decision"        // match LLM decision output (strategies 4/5)
    ConditionTypeRandom         ConditionType = "random"          // stochastic / probability gate
    ConditionTypeMemory         ConditionType = "memory"          // inspect agent memory / history
    ConditionTypeEvent          ConditionType = "event"           // react to an emitted EventDefinition
    ConditionTypeCustom         ConditionType = "custom"          // user-defined
)

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
    ConditionOpIn          ConditionOperator = "in"      // membership in a set
    ConditionOpNotIn       ConditionOperator = "not_in"  // non-membership
)

type LogicOperator string

const (
    LogicOpAND LogicOperator = "AND"
    LogicOpOR  LogicOperator = "OR"
    LogicOpNOT LogicOperator = "NOT"
)

type Condition struct {
    Type          ConditionType          `bson:"type"`
    PropertyUID   string                 `bson:"property_uid,omitempty"` // property UID or reserved pseudo-prop ("state","position")
    Operator      ConditionOperator      `bson:"operator,omitempty"`
    Value         interface{}            `bson:"value,omitempty"`
    MinValue      *float64               `bson:"min_value,omitempty"`
    MaxValue      *float64               `bson:"max_value,omitempty"`
    Radius        *float64               `bson:"radius,omitempty"`
    TargetUID     string                 `bson:"target_uid,omitempty"`
    TargetUIDs    []string               `bson:"target_uids,omitempty"`
    LogicOperator LogicOperator          `bson:"logic_operator,omitempty"`
    SubConditions []Condition            `bson:"sub_conditions,omitempty"`
    Parameters    map[string]interface{} `bson:"parameters,omitempty"`
    Description   string                 `bson:"description,omitempty"`
}
```

**Condition Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `type` | ConditionType | What kind of check to perform |
| `property_uid` | string | Property UID to check, or a reserved pseudo-property (`"state"`, `"position"`) |
| `operator` | ConditionOperator | Comparison operator |
| `value` | interface{} | Value to compare against |
| `min_value` / `max_value` | *float64 | Range bounds (used with `"in_range"`) |
| `radius` | *float64 | Spatial radius (used with `"proximity"`, `"count"`, `"barrier"`) |
| `target_uid` | string | Single agent/shape/archetype UID or EventDefinition UID (for `"event"`) |
| `target_uids` | []string | Multiple agent/shape/archetype UIDs |
| `logic_operator` | LogicOperator | `"AND"`, `"OR"`, or `"NOT"` (used with `"composite"`) |
| `sub_conditions` | []Condition | Sub-conditions for composite logic |
| `parameters` | map | Flexible / user-defined condition parameters |
| `description` | string | Human-readable note |

**ConditionType usage notes**:

| Type | Key fields |
|------|-----------|
| `"global_property"` | `property_uid`, `operator`, `value` |
| `"property"` | `property_uid`, `operator`, `value` / `min_value`…`max_value` |
| `"position"` | `property_uid` = `"position"`, `operator`, coordinates in `value` |
| `"proximity"` | `target_uids`, `radius` |
| `"step"` | `operator`, `value` (step number) |
| `"composite"` | `logic_operator`, `sub_conditions` |
| `"count"` | `target_uids`, `radius`, `operator`, `value` |
| `"barrier"` | `target_uid` (shape UID), `radius` |
| `"state"` | `property_uid` = `"state"`, `operator`, `value` |
| `"distance"` | `target_uid`, `operator`, `value` |
| `"decision"` | `operator`, `value` (matches LLM decision output; strategies 4/5) |
| `"random"` | `value` (probability 0–1) |
| `"memory"` | `property_uid` (memory field), `operator`, `value` |
| `"event"` | `target_uid` = EventDefinition UID; optional `property_uid`+`operator`+`value` match payload field |
| `"custom"` | `parameters` |

**ConditionOperator values**:

| Value | Meaning |
|-------|---------|
| `"eq"` | Equal |
| `"neq"` | Not equal |
| `"gt"` | Greater than |
| `"lt"` | Less than |
| `"gte"` | Greater than or equal |
| `"lte"` | Less than or equal |
| `"contains"` | String / collection contains |
| `"in_range"` | Between `min_value` and `max_value` |
| `"in"` | Value is a member of a set |
| `"not_in"` | Value is not a member of a set |

**LogicOperator values** (for `ConditionTypeComposite`):

| Value | Meaning |
|-------|---------|
| `"AND"` | All sub-conditions must be true |
| `"OR"` | At least one sub-condition must be true |
| `"NOT"` | Sub-condition must be false |

### Action

All entity references in actions use UIDs.

```go
type ActionType string

const (
    ActionTypeSetProperty    ActionType = "set_property"
    ActionTypeModifyProperty ActionType = "modify_property"
    ActionTypeMove           ActionType = "move"
    ActionTypeMoveToward     ActionType = "move_toward"
    ActionTypeMoveAway       ActionType = "move_away"
    ActionTypeInteract       ActionType = "interact"
    ActionTypeEmitEvent      ActionType = "emit_event"
    ActionTypeTransition     ActionType = "transition" // state transition
    ActionTypeCallLLM        ActionType = "call_llm"   // trigger LLM decision
    ActionTypeSpawnAgent     ActionType = "spawn_agent"
    ActionTypeRemoveAgent    ActionType = "remove_agent"
    ActionTypeTransform      ActionType = "transform"   // change agent type
    ActionTypeTeleport       ActionType = "teleport"
    ActionTypeCustom         ActionType = "custom"
)

type PropertyModifier string

const (
    PropertyModifierSet       PropertyModifier = "set"
    PropertyModifierAdd       PropertyModifier = "add"
    PropertyModifierSubtract  PropertyModifier = "subtract"
    PropertyModifierMultiply  PropertyModifier = "multiply"
    PropertyModifierDivide    PropertyModifier = "divide"
    PropertyModifierIncrement PropertyModifier = "increment"
    PropertyModifierDecrement PropertyModifier = "decrement"
    PropertyModifierToggle    PropertyModifier = "toggle" // boolean toggle
)

type Action struct {
    Type        ActionType       `bson:"type"`
    PropertyUID string           `bson:"property_uid,omitempty"`
    Value       interface{}      `bson:"value,omitempty"`
    Modifier    PropertyModifier `bson:"modifier,omitempty"`
    TargetUID   string           `bson:"target_uid,omitempty"`
    TargetUIDs  []string         `bson:"target_uids,omitempty"`
    Speed       *float64         `bson:"speed,omitempty"`
    Direction   *float64         `bson:"direction,omitempty"`
    EventUID    string           `bson:"event_uid,omitempty"` // references an EventDefinition
    EventData   interface{}      `bson:"event_data,omitempty"`
    NewState    string           `bson:"new_state,omitempty"`

    // Agent creation during simulation.
    SpawnAgentUID    string                 `bson:"spawn_agent_uid,omitempty"` // archetype UID
    SpawnCount       int                    `bson:"spawn_count,omitempty"`
    SpawnProperties  map[string]interface{} `bson:"spawn_properties,omitempty"`
    SpawnPosition    *SpawnPosition         `bson:"spawn_position,omitempty"`
    SpawnInheritFrom []string               `bson:"spawn_inherit_from,omitempty"` // property UIDs

    Parameters       map[string]interface{} `bson:"parameters,omitempty"`
    Description      string                 `bson:"description,omitempty"`
    LLMPromptContext string                 `bson:"llm_prompt_context,omitempty"`
}
```

**Action Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `type` | ActionType | What action to perform (14 types) |
| `property_uid` | string | Property UID to read/write; may be a reserved pseudo-property |
| `value` | interface{} | Value to set or compare |
| `modifier` | PropertyModifier | How to modify the property (for `modify_property`) |
| `target_uid` | string | Single target agent/shape UID |
| `target_uids` | []string | Multiple target UIDs |
| `speed` | *float64 | Movement speed (for move actions) |
| `direction` | *float64 | Movement direction in degrees (for `move`) |
| `event_uid` | string | References an `EventDefinition.UID` (for `emit_event`) |
| `event_data` | interface{} | Payload data for the emitted event |
| `new_state` | string | Target state for `transition`; must be in `Agent.States` |
| `spawn_agent_uid` | string | Archetype UID to instantiate (for `spawn_agent`) |
| `spawn_count` | int | Number of instances to spawn |
| `spawn_properties` | map | Initial property overrides for spawned agents |
| `spawn_position` | *SpawnPosition | Where to place spawned agents |
| `spawn_inherit_from` | []string | Property UIDs to copy from the spawning agent |
| `parameters` | map | Flexible / user-defined parameters |
| `description` | string | Human-readable note |
| `llm_prompt_context` | string | Reserved per-action LLM hint. **Not currently injected into any prompt** — no prompt builder consumes it (it only appears in the Level-1 model slice as editable data); see [llm-prompts.md](llm-prompts.md) |

**ActionType values** (14 total):

| Value | Description |
|-------|-------------|
| `"set_property"` | Set `property_uid` to `value` |
| `"modify_property"` | Modify `property_uid` using `modifier` and `value` |
| `"move"` | Move agent in `direction` at `speed` |
| `"move_toward"` | Move toward `target_uid` / `target_uids` |
| `"move_away"` | Move away from `target_uid` / `target_uids` |
| `"interact"` | Trigger interaction with `target_uid` |
| `"emit_event"` | Emit the event identified by `event_uid` with optional `event_data` |
| `"transition"` | State-machine transition to `new_state` |
| `"call_llm"` | Request an on-demand LLM decision consumed by the NEXT step's decision-purpose call (its `llm_prompt_context` is currently not injected into that prompt) |
| `"spawn_agent"` | Create new agent instances of archetype `spawn_agent_uid` |
| `"remove_agent"` | Remove the agent from the simulation |
| `"transform"` | Change the agent's type |
| `"teleport"` | Jump to a new position |
| `"custom"` | User-defined action; `parameters` carries the payload |

**PropertyModifier values** (8 total):

| Value | Description |
|-------|-------------|
| `"set"` | Replace value |
| `"add"` | Numeric addition |
| `"subtract"` | Numeric subtraction |
| `"multiply"` | Numeric multiplication |
| `"divide"` | Numeric division |
| `"increment"` | Add 1 |
| `"decrement"` | Subtract 1 |
| `"toggle"` | Flip boolean value |

### SpawnPosition

```go
type SpawnPositionType string

const (
    SpawnPositionSame    SpawnPositionType = "same"
    SpawnPositionRandom  SpawnPositionType = "random"
    SpawnPositionOffset  SpawnPositionType = "offset"
    SpawnPositionNearby  SpawnPositionType = "nearby"
    SpawnPositionAtShape SpawnPositionType = "at_shape"
)

type SpawnPosition struct {
    Type     SpawnPositionType `bson:"type"`
    OffsetX  *float64          `bson:"offset_x,omitempty"`
    OffsetY  *float64          `bson:"offset_y,omitempty"`
    Radius   *float64          `bson:"radius,omitempty"`
    ShapeUID string            `bson:"shape_uid,omitempty"` // for at_shape
}
```

**Enum: SpawnPositionType**:

| Value | Description |
|-------|-------------|
| `"same"` | Same position as the spawning agent |
| `"random"` | Random position within the environment |
| `"offset"` | Fixed offset from spawning agent (`offset_x`, `offset_y`) |
| `"nearby"` | Random position within `radius` of spawning agent |
| `"at_shape"` | Within/at the shape identified by `shape_uid` |

### Rule

```go
type Rule struct {
    ID          string      `bson:"id"`
    Name        string      `bson:"name"`
    Description string      `bson:"description,omitempty"`
    Priority    int         `bson:"priority"`   // higher priority rules execute first
    AppliesTo   []string    `bson:"applies_to"` // agent/archetype UIDs
    // Conditions are AND-combined; use ConditionTypeComposite for OR/NOT logic.
    Conditions    []Condition `bson:"conditions"`
    Actions       []Action    `bson:"actions"`
    ExecuteOnce   bool        `bson:"execute_once"`
    CooldownSteps int         `bson:"cooldown_steps,omitempty"`
    Enabled       bool        `bson:"enabled"`
    Locked        bool        `bson:"locked,omitempty"` // freezes this rule against LLM edits (user-only)
}
```

**Rule Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Model-local rule identifier; referenced at runtime via `RuleState.rule_id` |
| `name` | string | Human-readable name |
| `description` | string | What the rule does |
| `priority` | int | Execution order (higher value = runs first) |
| `applies_to` | []string | Agent/archetype **UIDs** this rule applies to |
| `conditions` | []Condition | All conditions are AND-combined; use `ConditionTypeComposite` for OR/NOT logic |
| `actions` | []Action | All actions executed when conditions are met |
| `execute_once` | bool | Rule fires at most once per agent per simulation |
| `cooldown_steps` | int | Minimum simulation steps between executions |
| `enabled` | bool | Whether rule is active |
| `locked` | bool | User-only flag; when `true`, freezes this rule against model-generation LLM edits (preserved verbatim by the merge). bson key `locked,omitempty` |

### Interaction

```go
type InteractionType string

const (
    InteractionTypeAgentAgent  InteractionType = "agent_agent"
    InteractionTypeAgentShape  InteractionType = "agent_shape"
    InteractionTypeAgentGlobal InteractionType = "agent_global"
)

type Interaction struct {
    ID               string          `bson:"id"`
    Name             string          `bson:"name"`
    Description      string          `bson:"description,omitempty"`
    Type             InteractionType `bson:"type"`
    InitiatorUIDs    []string        `bson:"initiator_uids"`
    TargetUIDs       []string        `bson:"target_uids,omitempty"`
    Radius           *float64        `bson:"radius,omitempty"`
    Conditions       []Condition     `bson:"conditions,omitempty"`
    InitiatorActions []Action        `bson:"initiator_actions,omitempty"`
    TargetActions    []Action        `bson:"target_actions,omitempty"`
    Bidirectional    bool            `bson:"bidirectional"`
    UseLLM           bool            `bson:"use_llm"`
    Enabled          bool            `bson:"enabled"`
    Locked           bool            `bson:"locked,omitempty"` // freezes this interaction against LLM edits (user-only)

    LLMPromptContext string `bson:"llm_prompt_context,omitempty"`
}
```

**Interaction Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Model-local interaction identifier |
| `name` | string | Display name |
| `description` | string | What happens in this interaction |
| `type` | InteractionType | `"agent_agent"`, `"agent_shape"`, or `"agent_global"` |
| `initiator_uids` | []string | Agent/archetype UIDs that can initiate |
| `target_uids` | []string | Agent/archetype/shape UIDs that can be targets |
| `radius` | *float64 | Interaction range |
| `conditions` | []Condition | Conditions for the interaction to trigger |
| `initiator_actions` | []Action | Actions applied to the initiating agent |
| `target_actions` | []Action | Actions applied to the target |
| `bidirectional` | bool | Both participants affected equally |
| `use_llm` | bool | Use LLM to determine interaction outcome |
| `enabled` | bool | Whether interaction is active |
| `locked` | bool | User-only flag; when `true`, freezes this interaction against model-generation LLM edits (preserved verbatim by the merge). bson key `locked,omitempty` |
| `llm_prompt_context` | string | Additional LLM guidance for this interaction |

**Enum: InteractionType**:

| Value | Description |
|-------|-------------|
| `"agent_agent"` | Interaction between two agents |
| `"agent_shape"` | Agent interacting with an environmental shape |
| `"agent_global"` | Agent interacting with global environment state |

---

## EventDefinition

First-class, model-level event definitions. Producers emit events via `ActionTypeEmitEvent` (`Action.EventUID`). Consumers react via `ConditionTypeEvent` (`Condition.TargetUID` holds the `EventUID`). The runtime event bus (queue, TTL expiry, scope-based delivery) is engine work, out of scope for the data layer.

```go
type EventProducer string

const (
    EventProducerAgent       EventProducer = "agent"
    EventProducerEnvironment EventProducer = "environment"
    EventProducerLLM         EventProducer = "llm"
)

type EventScope string

const (
    EventScopeGlobal   EventScope = "global"   // perceivable by all agents
    EventScopeLocal    EventScope = "local"     // within Radius of the emit position
    EventScopeTargeted EventScope = "targeted"  // only ConsumerUIDs
)

type EventDefinition struct {
    UID         string          `bson:"uid"` // unique within model; referenced by emit/condition
    Name        string          `bson:"name"`
    Description string          `bson:"description,omitempty"`
    Scope       EventScope      `bson:"scope"`
    Radius      *float64        `bson:"radius,omitempty"`  // for local scope
    Payload     []AgentProperty `bson:"payload,omitempty"` // typed payload fields

    Producers    []EventProducer `bson:"producers"`               // who may emit
    ProducerUIDs []string        `bson:"producer_uids,omitempty"` // optional: specific agent/archetype UIDs
    ConsumerUIDs []string        `bson:"consumer_uids,omitempty"` // optional: reacting UIDs (empty = any)

    TTLSteps         uint   `bson:"ttl_steps,omitempty"` // 0 = active only on emit step; N = active for N additional steps
    LLMPromptContext string `bson:"llm_prompt_context,omitempty"`

    // LLMTarget routes LLM-produced emissions of this event (Producers contains "llm")
    // to a specific provider/model/effort. nil = inherit from LLMConfig.Target.
    // EventDefinition keeps a direct *LLMTarget (not a wrapper) because events do not
    // have their own strategy or simulation config.
    LLMTarget *common.LLMTarget `bson:"llm_target,omitempty"`

    // Locked freezes this event against model-generation LLM edits. User-only.
    Locked bool `bson:"locked,omitempty"`
}
```

**EventDefinition Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Unique within the model; referenced in `Action.EventUID` and `Condition.TargetUID` |
| `name` | string | Display name |
| `description` | string | What the event represents |
| `scope` | EventScope | Who can perceive the event |
| `radius` | *float64 | Perception radius (for `"local"` scope) |
| `payload` | []AgentProperty | Typed payload fields (reuses `AgentProperty` type for consistent type/constraint semantics) |
| `producers` | []EventProducer | Which producer types may emit this event |
| `producer_uids` | []string | Optional: specific agent/archetype UIDs allowed to produce |
| `consumer_uids` | []string | Optional: reacting agent/archetype UIDs; empty = any agent can react |
| `ttl_steps` | uint | How long the event remains active: 0 = emit step only; N = N additional steps |
| `llm_prompt_context` | string | Reserved per-event LLM hint. **Not currently injected into any prompt** (the decision prompts advertise emittable event UIDs only); see [llm-prompts.md](llm-prompts.md) |
| `llm_target` | *LLMTarget | Routes LLM-produced emissions (when `Producers` contains `"llm"`) to a specific provider/model/effort; nil = inherit from `LLMConfig.Target` |
| `locked` | bool | User-only flag; when `true`, freezes this event against model-generation LLM edits (preserved verbatim by the merge). bson key `locked,omitempty` |

**Enum: EventProducer**:

| Value | Description |
|-------|-------------|
| `"agent"` | An agent emits the event (via `emit_event` action) |
| `"environment"` | The environment emits the event (via environment rules/strategies) |
| `"llm"` | The LLM emits the event as part of its decision output |

**Enum: EventScope**:

| Value | Description |
|-------|-------------|
| `"global"` | Perceivable by all agents in the simulation |
| `"local"` | Perceivable by agents within `radius` of the emit position |
| `"targeted"` | Perceivable only by agents listed in `consumer_uids` |

---

## LLM Configuration

### LLMConfig

Nested on `ModelDocument` as `llm_config`. Holds the provider selection, a permanent model-level prompt context, and defaults that individual agents and shapes can override.

```go
type LLMConfig struct {
    Target        common.LLMTarget `bson:"target"`                   // model-level default (provider, model, effort)
    PromptContext string           `bson:"prompt_context,omitempty"` // permanent per-model context for generation

    // Default for model agents/archetypes.
    DefaultStrategy         LLMStrategy          `bson:"default_strategy"`
    DefaultSimulationConfig *LLMSimulationConfig `bson:"default_simulation_config,omitempty"`
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `target` | LLMTarget | Model-level default LLM selection (provider, model, effort) for Level-1 generation and as the lowest-priority fallback for Level-2 calls |
| `prompt_context` | string | Model-generation context. Injected (when non-empty) as a `## RESEARCHER GUIDANCE (user-authored, follow it)` section into the Level-1 system prompt (`buildSystemPrompt`) and the vision prompt (`buildVisionSystemPrompt`) via `researcherGuidance`. The sim-generation / execution prompts use their own configs' contexts; see [llm-prompts.md](llm-prompts.md) |
| `default_strategy` | LLMStrategy | Baseline strategy applied to agents that do not set their own `LLMStrategy` |
| `default_simulation_config` | *LLMSimulationConfig | Default run-level config (perception, memory, fallback, parallel workers, etc.) |

### LLMStrategy and LLMStrategyType

The four-value `LLMStrategyType` encodes **how** the LLM is used at runtime. The **scope** (individual vs. archetype) is derived from `Agent.Type`, not from the strategy type itself.

The six article strategies map as follows:

| Article strategy | `LLMStrategy.Type` | `Agent.Type` |
|------------------|--------------------|----------------------|
| 1 Rule-based (no LLM) | `"none"` | any |
| 2 Individual full control | `"full"` | `"agent"` |
| 3 Archetype full control | `"full"` | `"archetype"` |
| 4 Individual decision-only | `"decision"` | `"agent"` |
| 5 Archetype decision-only | `"decision"` | `"archetype"` |
| 6 Hybrid | `"hybrid"` (model default) | per-agent overrides differ |

`LLMConfig.DefaultStrategy` is the baseline; per-agent `Agent.LLMStrategy` overrides it. Model-level `"hybrid"` signals that agents define their own strategies individually.

The `Type` is the **single source of truth** for which per-step purposes the LLM owns. The four `UseFor*` flags are **canonical** for the Type — the backend re-normalizes them on save (`LLMStrategy.Canonical` / `Model.NormalizeLLMStrategies` in `data/models`), applied uniformly to the model default strategy and every per-agent/per-shape override:

| Type | UseForMovement | UseForInteraction | UseForProperties | UseForDecisions | User-editable flags? |
|------|:--:|:--:|:--:|:--:|:--:|
| `none` | ✗ | ✗ | ✗ | ✗ | no (all off) |
| `full` | ✓ | ✓ | ✓ | ✓ | no (all on) |
| `decision` | ✗ | ✗ | ✗ | ✓ | no (decisions only) |
| `hybrid` | free | free | free | free | yes |

> Spawn and population are **simulation-generation** concerns, not execution: whether the LLM seeds the initial population is gated by `AgentInitialization.CountMode == CountLLM`, and per-placement guidance lives on `AgentInitialization.Placement.PromptContext` (reused as the population/count prompt). The old `UseForSpawn`/`UseForPopulation` flags and the `SpawnPromptContext`/`PopulationPromptContext` strings were removed.

```go
type LLMStrategyType string

const (
    LLMStrategyNone     LLMStrategyType = "none"
    LLMStrategyFull     LLMStrategyType = "full"
    LLMStrategyDecision LLMStrategyType = "decision"
    LLMStrategyHybrid   LLMStrategyType = "hybrid"
)

type LLMStrategy struct {
    Type      LLMStrategyType `bson:"type"`
    BatchSize int             `bson:"batch_size,omitempty"` // agents/archetypes per LLM call

    UseForMovement    bool `bson:"use_for_movement"`
    UseForInteraction bool `bson:"use_for_interaction"`
    UseForProperties  bool `bson:"use_for_properties"`
    UseForDecisions   bool `bson:"use_for_decisions"`

    DecisionFrequency int `bson:"decision_frequency,omitempty"` // call LLM every N steps
}
```

**LLMStrategy Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `type` | LLMStrategyType | How the LLM is used (`"none"`, `"full"`, `"decision"`, `"hybrid"`) — canonicalizes the flags below |
| `batch_size` | int | Agents/archetypes processed per LLM call (batching) |
| `use_for_movement` | bool | Use LLM to determine movement (canonical per Type) |
| `use_for_interaction` | bool | Use LLM to determine interaction outcomes (canonical per Type) |
| `use_for_properties` | bool | Use LLM to update properties (canonical per Type) |
| `use_for_decisions` | bool | Use LLM for high-level decisions (canonical per Type) |
| `decision_frequency` | int | Call LLM every N steps (0 or 1 = every step) |

**Enum: LLMStrategyType**:

| Value | Meaning |
|-------|---------|
| `"none"` | No LLM calls during simulation execution (pure rule engine); all four flags off |
| `"full"` | LLM controls all four purposes each step; all four flags forced on (not user-editable) |
| `"decision"` | LLM makes high-level decisions only; only `use_for_decisions` on (not user-editable) |
| `"hybrid"` | The four flags are user-configurable; at model level also signals agents define their own strategies |

### LLMAgentConfig and LLMShapeConfig

These wrapper types consolidate all per-entity LLM settings into a single struct. They supersede the previous design (FU-C6) where `LLMTarget`, `LLMStrategy`, and `LLMSimulationConfig` were separate fields on the entity. Using a wrapper means nil-checking one field (the wrapper itself) determines whether the entity has any LLM override at all.

```go
// LLMAgentConfig is the per-agent/archetype LLM configuration wrapper.
// PromptContext replaces the old GeneralPromptContext field that was on LLMAgentStrategy.
type LLMAgentConfig struct {
    Target        common.LLMTarget          `bson:"target"`                   // WHICH provider/model/effort
    PromptContext string                    `bson:"prompt_context,omitempty"` // general LLM context for this agent
    Strategy         LLMAgentStrategy          `bson:"strategy"`
    SimulationConfig *LLMSimulationAgentConfig `bson:"simulation_config,omitempty"`
}

// LLMShapeConfig is the per-shape LLM configuration wrapper.
type LLMShapeConfig struct {
    Target        common.LLMTarget `bson:"target"`
    PromptContext string           `bson:"prompt_context,omitempty"`
    Strategy      LLMShapeStrategy `bson:"strategy"`
}
```

**LLMAgentConfig Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `target` | LLMTarget | WHICH provider/model/effort to use for this agent's LLM calls |
| `prompt_context` | string | General LLM context for this agent/archetype (replaces old `GeneralPromptContext` on `LLMAgentStrategy`) |
| `strategy` | LLMAgentStrategy | HOW the LLM is used (strategy type + canonical use-flags) |
| `simulation_config` | *LLMSimulationAgentConfig | Per-agent runtime config (perception/memory/fallback); nil = inherit model default |

**LLMShapeConfig Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `target` | LLMTarget | WHICH provider/model/effort to use for this shape's LLM calls |
| `prompt_context` | string | General LLM context for this shape |
| `strategy` | LLMShapeStrategy | HOW the LLM is used for this shape (strategy type + canonical use-flags) |

### LLMAgentStrategy

Per-agent/archetype LLM behavior (embedded inside `LLMAgentConfig.Strategy`). It is now a thin wrapper over `LLMStrategy` (a distinct named type so agent-specific fields can be added later). Population counts/placements live in `AgentInitialization`; per-placement and population/count LLM guidance is `AgentInitialization.Placement.PromptContext`; general LLM context is `LLMAgentConfig.PromptContext`.

```go
type LLMAgentStrategy struct {
    LLMStrategy `bson:",inline"`
}
```

> **Removed:** `spawn_prompt_context` (dead) and `population_prompt_context` — the population/count prompt now reuses `AgentInitialization.Placement.PromptContext`. `general_prompt_context` had earlier moved to `LLMAgentConfig.PromptContext`.

### LLMShapeStrategy

Per-shape LLM strategy, embedded inside `LLMShapeConfig.Strategy`. Analogous to `LLMAgentStrategy` for shapes; intentionally a distinct type so shape-specific fields can evolve independently. A thin wrapper over `LLMStrategy`.

```go
type LLMShapeStrategy struct {
    LLMStrategy `bson:",inline"`
}
```

> **Removed:** `spawn_prompt_context` / `population_prompt_context` (see `LLMAgentStrategy` above); `general_prompt_context` lives at `LLMShapeConfig.PromptContext`.

### LLMSimulationConfig and LLMSimulationAgentConfig

`LLMSimulationConfig` is the **model-level default** config (stored as `LLMConfig.DefaultSimulationConfig`). It inlines `LLMSimulationAgentConfig` to add run-wide settings. There is no longer a per-simulation-run override — the config comes entirely from the model snapshot.

`LLMSimulationAgentConfig` is the **per-agent** portion (also reachable via `LLMAgentConfig.SimulationConfig`). `FallbackPolicy` is a pointer so "unset" means "inherit from the enclosing level."

```go
// Per-agent runtime/spatial config (overridable per agent).
type LLMSimulationAgentConfig struct {
    PerceptionRadius  float64         `bson:"perception_radius,omitempty"`
    InteractionRadius float64         `bson:"interaction_radius,omitempty"`
    MemorySize        int             `bson:"memory_size,omitempty"`
    FallbackPolicy    *FallbackPolicy `bson:"fallback_policy,omitempty"`
}

// Model-level default run config.
type LLMSimulationConfig struct {
    LLMSimulationAgentConfig `bson:",inline"`

    ParallelWorkers uint `bson:"parallel_workers,omitempty"` // uint (was int)
    RetryTimeoutMs  uint `bson:"retry_timeout_ms,omitempty"` // uint (was int)
    MaxRetries      uint `bson:"max_retries,omitempty"`      // uint (was int)
    StreamBatchSize uint `bson:"stream_batch_size,omitempty"` // uint (was int)
}
```

**LLMSimulationAgentConfig Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `perception_radius` | float64 | Distance the agent can sense nearby entities |
| `interaction_radius` | float64 | Distance within which interactions trigger |
| `memory_size` | int | Maximum number of `AgentMemory` entries to retain |
| `fallback_policy` | *FallbackPolicy | LLM failure policy; nil = inherit from next level |

**LLMSimulationConfig Additional Fields** (inlines all `LLMSimulationAgentConfig` fields):

| Field | Type | Description |
|-------|------|-------------|
| `parallel_workers` | uint | Concurrent workers for LLM calls in a simulation step |
| `retry_timeout_ms` | uint | Per-LLM-call timeout in milliseconds |
| `max_retries` | uint | Maximum LLM retry attempts before applying fallback |
| `stream_batch_size` | uint | Batch size for WebSocket message chunking to the frontend |

---

## FallbackPolicy

### Types

```go
type FallbackPolicyType string

const (
    FallbackRetry        FallbackPolicyType = "retry"
    FallbackRulesOnly    FallbackPolicyType = "rules_only"
    FallbackLastDecision FallbackPolicyType = "last_decision"
    FallbackSkip         FallbackPolicyType = "skip"
)

type FallbackStep struct {
    Type         FallbackPolicyType `bson:"type"`
    MaxRetries   uint               `bson:"max_retries,omitempty"`   // uint (was int)
    RetryDelayMs uint               `bson:"retry_delay_ms,omitempty"` // uint (was int)
}

type FallbackPolicy struct {
    Steps []FallbackStep `bson:"steps"`
}
```

`FallbackPolicy` is a **multi-step sequence**. The engine works through `Steps` in order: if a step's action is exhausted or fails, it proceeds to the next step.

**FallbackStep Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `type` | FallbackPolicyType | What to do when an LLM call fails |
| `max_retries` | uint | Maximum retry attempts (for `"retry"` type) |
| `retry_delay_ms` | uint | Delay between retry attempts in milliseconds |

**FallbackPolicyType values**:

| Value | Description |
|-------|-------------|
| `"retry"` | Retry the LLM call up to `max_retries` times with `retry_delay_ms` delay |
| `"rules_only"` | Skip the LLM call and fall back to rule engine evaluation only |
| `"last_decision"` | Reuse the agent's last LLM decision from memory |
| `"skip"` | Agent does nothing this step; simulation continues |

### Fallback Resolution Chain

The effective `FallbackPolicy` for a given agent is resolved in this order (first non-nil wins):

1. **Agent-level**: `Agent.LLMConfig.SimulationConfig.FallbackPolicy`
2. **Model default**: `ModelDocument.LLMConfig.DefaultSimulationConfig.FallbackPolicy`
3. **System default**: Retry with `max_retries=2`, `retry_delay_ms=1000`

> **Removed**: the former `SimulationDocument.FallbackPolicy` override level no longer exists — per-run LLM overrides (`LLMStrategy`, `FallbackPolicy`, `Config`) were removed from `SimulationDocument`. All configuration flows from the model snapshot.

---

## SimulationDocument

**Collection**: `simulations`

> **Storage-agnostic counterpart**: `simulations.Simulation` (package `data/simulations`) — uses a string hex `ID` and no bson tags. `SimulationDocument` (package `data/mongodb/simulations`) is the MongoDB representation and is the type described in this section.

Represents a single simulation execution instance created from a `ModelDocument`. Uses both `bson` and `json` tags throughout. All static LLM configuration (strategy, fallback, config) comes from the embedded model snapshot; there are no per-run overrides.

```go
type SimulationDocument struct {
    ID          bson.ObjectID `json:"id" bson:"_id,omitempty"`
    Name        string        `json:"name,omitempty" bson:"name,omitempty"`
    Description string        `json:"description,omitempty" bson:"description,omitempty"`

    ModelID bson.ObjectID `json:"model_id" bson:"model_id"`
    ChatID  bson.ObjectID `json:"chat_id" bson:"chat_id"`

    // ModelSnapshot is the frozen model snapshot taken at creation — the source of all static
    // definitions (UpdateMode, LLM config, agents, shapes, rules, interactions, events).
    // Not excluded from JSON (owner decision); immutable after creation.
    ModelSnapshot *models.ModelDocument `bson:"model_snapshot"`

    // Runtime state.
    Agents      []SimulationAgent      `json:"agents" bson:"agents"`
    Environment *SimulationEnvironment `json:"environment,omitempty" bson:"environment,omitempty"`

    Status        SimulationStatus         `json:"status" bson:"status"`
    StatusText    string                   `json:"status_text" bson:"status_text"` // human-readable status detail
    StatusHistory []SimulationStatusChange `json:"status_history,omitempty" bson:"status_history,omitempty"`

    CurrentStep uint64 `bson:"current_step"`       // last completed step number
    Steps       uint64 `json:"steps" bson:"steps"` // total steps to run

    Statistics *SimulationStatistics `json:"statistics,omitempty" bson:"statistics,omitempty"`
    TokenUsage *SimulationTokenUsage `json:"token_usage,omitempty" bson:"token_usage,omitempty"`

    Widgets []WidgetConfig `json:"widgets,omitempty" bson:"widgets,omitempty"`
    Files   []common.File  `json:"files,omitempty" bson:"files,omitempty"` // common.File (was []SimulationFile)

    CreatedAt  time.Time  `json:"created_at" bson:"created_at"`
    CreatedBy  string     `json:"created_by" bson:"created_by"`
    UpdatedAt  time.Time  `json:"updated_at" bson:"updated_at"`
    UpdatedBy  string     `json:"updated_by" bson:"updated_by"`
    StartedAt  *time.Time `json:"started_at,omitempty" bson:"started_at,omitempty"`
    FinishedAt *time.Time `json:"finished_at,omitempty" bson:"finished_at,omitempty"`
}
```

**Top-Level Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Unique simulation identifier |
| `name` | string | Display name for this run |
| `description` | string | Additional context |
| `model_id` | ObjectID | Reference to the parent `ModelDocument` |
| `chat_id` | ObjectID | Reference to the creation chat |
| `model_snapshot` | *ModelDocument | Embedded frozen model snapshot (bson key `model_snapshot`; was `model` with `json:"-"`). Contains all static definitions including `UpdateMode` and LLM config. **Immutable after creation** — never rewritten on per-step `$set` updates |
| `agents` | []SimulationAgent | Runtime agent instances with live state |
| `environment` | *SimulationEnvironment | Runtime environment state |
| `status` | SimulationStatus | Current simulation state |
| `status_text` | string | Human-readable status detail (e.g. error message) |
| `status_history` | []SimulationStatusChange | Ordered log of status transitions |
| `current_step` | uint64 | Last completed step number (bson `current_step`) |
| `steps` | uint64 | Total steps to run (changed from `int64` to `uint64`) |
| `statistics` | *SimulationStatistics | Materialised run summary (population, counts, movement); detail is in `simulation_steps` |
| `token_usage` | *SimulationTokenUsage | Multi-dimensional token accounting: total + per-model + per-agent + per-purpose breakdowns |
| `widgets` | []WidgetConfig | Dashboard widget configurations |
| `files` | []common.File | Generated file artifacts (e.g. analysis PDF reports); type changed from `[]SimulationFile` to `[]common.File`; metadata only — bytes are in GridFS |
| `created_at` | time.Time | Creation timestamp |
| `created_by` | string | User identifier |
| `updated_at` | time.Time | Last update timestamp |
| `updated_by` | string | User identifier of last editor |
| `started_at` | *time.Time | When execution began |
| `finished_at` | *time.Time | When execution completed (or failed/stopped) |

**Removed fields** (per-run LLM overrides no longer exist; all config comes from the model snapshot):

| Removed field | Reason |
|---|---|
| `update_mode` | Moved to `ModelDocument.UpdateMode` |
| `llm_target` | Removed — no per-run LLM target override; resolution goes directly from per-entity configs to `LLMConfig.Target` |
| `is_spatial` | Removed — derive from model snapshot (`ModelSnapshot.IsSpatial`) |
| `llm_prompt_context` | Removed — comes from model snapshot LLM config |
| `llm_strategy` | Removed — per-run strategy override eliminated |
| `fallback_policy` | Removed — per-run fallback override eliminated |
| `config` | Removed — per-run config override eliminated |

**Field rename**:

| Old field | Old bson | New field | New bson |
|---|---|---|---|
| `Model *models.ModelDocument` | `model` (with `json:"-"`) | `ModelSnapshot *models.ModelDocument` | `model_snapshot` |

**Enum: SimulationStatus**:

| Value | Description |
|-------|-------------|
| `"initiated"` | Created, not yet queued |
| `"queued"` | Waiting to start |
| `"running"` | Currently executing steps |
| `"paused"` | Temporarily paused |
| `"finished"` | Completed successfully |
| `"failed"` | Stopped due to error |
| `"stopped"` | Manually stopped |

### Nested: SimulationStatusChange

```go
type SimulationStatusChange struct {
    Status SimulationStatus `json:"status" bson:"status"`
    At     time.Time        `json:"at" bson:"at"`
    By     string           `json:"by" bson:"by"`
}
```

Records a single status transition: the new status, when it occurred, and who triggered it.

### Nested: SimulationAgent

Runtime state of a single agent instance. Stores only runtime-varying data plus a `Name` field for display convenience.

```go
type SimulationAgent struct {
    ID         uint64                 `json:"id" bson:"id"`   // uint64 (was int64); runtime instance id
    UID        string                 `json:"uid" bson:"uid"` // model Agent definition UID (was AgentUID, bson "agent_uid")
    Name       string                 `bson:"name"`           // per-instance runtime display name
    Position   *common.Point          `json:"position,omitempty" bson:"position,omitempty"` // integer grid coords (was *SpacePosition float64)
    Properties map[string]interface{} `json:"properties" bson:"properties"`
    State      string                 `json:"state,omitempty" bson:"state,omitempty"`
    Memory     []AgentMemory          `json:"memory,omitempty" bson:"memory,omitempty"`
}
```

**Agent Instance Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | uint64 | Runtime instance ID (changed from `int64` to `uint64`), unique within this simulation |
| `uid` | string | UID of the `models.Agent` definition this instance was spawned from (bson `uid`; previously field was named `AgentUID`, bson `agent_uid`) |
| `name` | string | Per-instance runtime display name |
| `position` | *common.Point | Current X, Y integer grid coordinates (nil if not spatial); changed from `*SpacePosition` (`float64` fields) to `*common.Point` (`uint64` fields) |
| `properties` | map | Current runtime property values keyed by property UID |
| `state` | string | Current state-machine state |
| `memory` | []AgentMemory | Past decisions, interactions, and events |

**Removed fields**:

| Removed field | Note |
|---|---|
| `rule_execution_state` | `RuleExecutionState map[string]RuleState` removed; `RuleState` type deleted |
| `SpacePosition` type | Replaced by `common.Point` (uint64 grid coords) |

### Nested: AgentMemory

```go
// AgentMemoryType classifies an agent memory entry.
type AgentMemoryType string

const (
    EventTypeMovement    AgentMemoryType = "movement"
    EventTypeDecision    AgentMemoryType = "decision"
    EventTypeInteraction AgentMemoryType = "interaction"
    EventTypeProperty    AgentMemoryType = "property"
)

type AgentMemory struct {
    Step      uint64          `json:"step" bson:"step"`           // uint64 (was int64)
    Type      AgentMemoryType `json:"type" bson:"event_type"`     // Go field "Type"; bson key "event_type"
    Summary   string          `json:"summary" bson:"summary"`
    Details   interface{}     `json:"details,omitempty" bson:"details,omitempty"`
    Timestamp time.Time       `json:"timestamp" bson:"timestamp"`
}
```

**Memory Fields**:

| Field | Go name | bson key | Description |
|-------|---------|----------|-------------|
| `step` | `Step` | `step` | Simulation step when this memory was recorded (uint64, was int64) |
| `type` | `Type` | `event_type` | Classification of the memory entry |
| `summary` | `Summary` | `summary` | Brief description of what happened |
| `details` | `Details` | `details` | Structured event payload (polymorphic; decodes as `bson.D` on read) |
| `timestamp` | `Timestamp` | `timestamp` | Wall-clock time when this memory was recorded |

**Enum: AgentMemoryType** (renamed from `AgentMemoryEventType`):

| Value | Description |
|-------|-------------|
| `"movement"` | Agent movement event |
| `"decision"` | Decision (rule-based or LLM) |
| `"interaction"` | Agent interaction event |
| `"property"` | Property value change |

**Removed values** (previous `AgentMemoryEventType` constants): `"llm_decision"`, `"llm_response"`, `"llm_fallback"`, `"rule_execution"` — collapsed into the four values above.

> **Removed:** `RuleState` type and `SimulationAgent.RuleExecutionState` field. Rule execution tracking has been removed from the persistence layer.

> **Removed:** `SpacePosition` type. Agent and shape positions are now `*common.Point` (uint64 grid coordinates).

### Nested: SimulationEnvironment

Stores the **runtime state** of the environment. Denormalized copies of `UID`, `Width`, `Height`, and `Torus` are now stored here so the engine can operate without touching the model snapshot on every step. Shape templates are still defined in the model snapshot; placed instances live in `Shapes`.

```go
type SimulationEnvironment struct {
    UID    string `bson:"uid"`
    Width  uint64 `bson:"width"`
    Height uint64 `bson:"height"`
    Torus  bool   `bson:"torus"`
    // GlobalProperties holds the runtime values of the model Environment.GlobalProperties
    // (without this, rule-driven changes to global state would be lost on restart).
    GlobalProperties map[string]interface{} `json:"global_properties,omitempty" bson:"global_properties,omitempty"`
    Shapes           []SimulationShape      `json:"shapes,omitempty" bson:"shapes,omitempty"`
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `uid` | string | Environment UID (denormalized from model snapshot for engine convenience) |
| `width` | uint64 | Grid width (denormalized from model snapshot; was absent in previous version) |
| `height` | uint64 | Grid height (denormalized from model snapshot; was absent in previous version) |
| `torus` | bool | Whether the space wraps (denormalized; was absent in previous version) |
| `global_properties` | map | Runtime values of the model `Environment.GlobalProperties`. Keyed by property UID |
| `shapes` | []SimulationShape | Placed shape instances with their runtime property values and geometry |

> **U10 congestion convention (2026-08-03):** global properties whose UIDs start with
> `congestion.` configure the engine's opt-in congestion capability (speed–density coupling
> via `congestion.speed_density.{radius,onset_density,jam_density,min_speed_factor}`, per-shape
> flow limits via `congestion.flow_limit.<shape_uid>` [+ optional
> `congestion.flow_limit_burst.<shape_uid>`]). They are ordinary numeric globals — no schema
> change — read each step by `modules/simulations/congestion.go`. See
> `docs/engine-config-and-events.md` §"Congestion via environment globals".

### Nested: SimulationShape

Runtime placed shape instance. Descriptive fields (`Type`, `Name`, `Layer`, `Visible`) are now **denormalized** directly onto the instance so the engine does not need to look up the model snapshot for every shape on every step. Geometry uses `*common.Point` for position and `[]common.Point` for polygon/line vertices (uint64 grid coordinates). `StrokeWidth` and `Thickness` are `*uint` (were `*int`). `Width` and `Height` are `*uint64` (were `*int`). The old `X *int` / `Y *int` flat position fields are replaced by `Position *common.Point`.

> **Removed:** The old local `Point` type (with `int` coordinates) is deleted; all geometry uses `common.Point`.

```go
type SimulationShape struct {
    ID         uint64                 `json:"id" bson:"id"` // runtime instance id, unique within the shapes in simulation
    UID        string                 `json:"uid" bson:"uid"`
    Name       string                 `bson:"name"`
    Type       models.ShapeType       `bson:"type"`
    Properties map[string]interface{} `json:"properties,omitempty" bson:"properties,omitempty"` // runtime property values

    Layer   uint `bson:"layer,omitempty"`
    Visible bool `bson:"visible"`

    Position *common.Point  `json:"position,omitempty" bson:"position,omitempty"`
    Width    *uint64        `json:"width,omitempty" bson:"width,omitempty"`
    Height   *uint64        `json:"height,omitempty" bson:"height,omitempty"`
    Radius   *float64       `json:"radius,omitempty" bson:"radius,omitempty"`
    Points   []common.Point `json:"points,omitempty" bson:"points,omitempty"`

    Fill        *bool `json:"fill,omitempty" bson:"fill,omitempty"`
    Stroke      *bool `json:"stroke,omitempty" bson:"stroke,omitempty"`
    StrokeWidth *uint `json:"stroke_width,omitempty" bson:"stroke_width,omitempty"`
    Thickness   *uint `json:"thickness,omitempty" bson:"thickness,omitempty"`
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | uint64 | Runtime instance ID, unique among shapes within a simulation |
| `uid` | string | Links to `models.ShapeDocument.UID` template |
| `name` | string | Denormalized from `ShapeDocument.Name` |
| `type` | models.ShapeType | Denormalized from `ShapeDocument.Type` |
| `properties` | map | Runtime property values keyed by property UID |
| `layer` | uint | Denormalized from `ShapeDocument.Layer` (was `int`, now `uint`) |
| `visible` | bool | Denormalized from `ShapeDocument.Visible` |
| `position` | *common.Point | Grid position: `{X, Y uint64}`; replaces old `x *int` / `y *int` flat fields |
| `width`, `height` | *uint64 | Dimensions for rectangles (were `*int`) |
| `radius` | *float64 | Radius for circles |
| `points` | []common.Point | Vertices for lines and polygons (each `{X, Y uint64}`) |
| `fill` | *bool | Whether shape interior is filled |
| `stroke` | *bool | Whether shape outline is stroked |
| `stroke_width` | *uint | Outline width (was `*int`) |
| `thickness` | *uint | Line thickness (was `*int`) |

**Shape type field usage**:

| Shape type | Relevant geometry fields |
|------------|--------------------------|
| `"rectangle"` | `position`, `width`, `height` |
| `"circle"` | `position`, `radius` |
| `"line"` | `points` ([]common.Point) |
| `"polygon"` | `points` ([]common.Point) |
| All | `fill`, `stroke`, `stroke_width`, `thickness` for styling |

**Changed from previous version**:

| Old field / type | New field / type | Note |
|---|---|---|
| `x *int`, `y *int` | `position *common.Point` | Position is now a nested `{X, Y uint64}` grid coordinate |
| `width *int`, `height *int` | `width *uint64`, `height *uint64` | Unsigned |
| `stroke_width *int`, `thickness *int` | `stroke_width *uint`, `thickness *uint` | Unsigned |
| `points []Point` (local) | `points []common.Point` | Uses shared uint64 point type |
| (absent) | `id uint64` | Runtime instance ID added |
| (absent) | `name`, `type`, `layer`, `visible` | Denormalized from model template |

---

## SimulationTokenUsage

Token accounting for a simulation run. Replaces the former use of `*common.TokenUsageSummary` on `SimulationDocument`, which only provided a total and per-model breakdown. `SimulationTokenUsage` adds per-agent (archetype) and per-purpose dimensions so it is possible to see how much was spent, by whom, and for what.

```go
type LLMUsagePurpose string

const (
    LLMUsageGeneration  LLMUsagePurpose = "generation"   // Level 1: model generation
    LLMUsageDecision    LLMUsagePurpose = "decision"     // Level 2: decision strategies (4/5)
    LLMUsageFullControl LLMUsagePurpose = "full_control" // Level 2: full-control strategies (2/3)
    LLMUsageInteraction LLMUsagePurpose = "interaction"  // Level 2: LLM-driven interactions
    LLMUsageFallback    LLMUsagePurpose = "fallback"     // fallback path
    LLMUsageAnalysis    LLMUsagePurpose = "analysis"     // Level 3: results analysis
)

// AgentTokenUsage is token spend attributed to one agent/archetype definition (by AgentUID).
type AgentTokenUsage struct {
    AgentUID          string `json:"agent_uid" bson:"agent_uid"`
    common.TokenUsage `bson:",inline"`
}

// PurposeTokenUsage is token spend attributed to one LLM usage purpose.
type PurposeTokenUsage struct {
    Purpose           LLMUsagePurpose `json:"purpose" bson:"purpose"`
    common.TokenUsage `bson:",inline"`
}

type SimulationTokenUsage struct {
    Total     common.TokenUsage   `json:"total" bson:"total"`
    ByModel   []common.TokenUsage `json:"by_model,omitempty" bson:"by_model,omitempty"`     // per (provider, model)
    ByAgent   []AgentTokenUsage   `json:"by_agent,omitempty" bson:"by_agent,omitempty"`     // per AgentUID (agent/archetype type)
    ByPurpose []PurposeTokenUsage `json:"by_purpose,omitempty" bson:"by_purpose,omitempty"` // per LLMUsagePurpose
}
```

**Fields — SimulationTokenUsage**:

| Field | Type | Description |
|-------|------|-------------|
| `total` | TokenUsage | Grand aggregate across all LLM calls in the run; `LLMTarget` fields (`provider_uid`, `model_uid`, `effort`) are empty here |
| `by_model` | []TokenUsage | One entry per `(provider_uid, model_uid, effort)` triple |
| `by_agent` | []AgentTokenUsage | One entry per `AgentUID` (model Agent definition); reflects token spend per archetype |
| `by_purpose` | []PurposeTokenUsage | One entry per `LLMUsagePurpose`; shows spend by integration level |

**Fields — AgentTokenUsage** (inlines `common.TokenUsage`):

| Field | Type | Description |
|-------|------|-------------|
| `agent_uid` | string | Model `Agent.UID` this token spend is attributed to |
| *(inlined)* | TokenUsage fields | `prompt_tokens`, `completion_tokens`, `total_tokens`, `calls`, `latency_ms`, etc. |

**Fields — PurposeTokenUsage** (inlines `common.TokenUsage`):

| Field | Type | Description |
|-------|------|-------------|
| `purpose` | LLMUsagePurpose | Which integration level/operation the spend is attributed to |
| *(inlined)* | TokenUsage fields | Same as above |

**Enum: LLMUsagePurpose**:

| Value | Description |
|-------|-------------|
| `"generation"` | Level 1 — LLM-driven model generation calls |
| `"decision"` | Level 2 — decision strategies (strategies 4/5: individual/archetype decision) |
| `"full_control"` | Level 2 — full-control strategies (strategies 2/3: individual/archetype full) |
| `"interaction"` | Level 2 — LLM-driven interaction resolution |
| `"fallback"` | Any fallback LLM call (triggered after primary call failure) |
| `"analysis"` | Level 3 — results analysis calls |

---

## SimulationStatistics

Materialised summary of a simulation run kept on `SimulationDocument` for fast reads. Detailed per-step series live in `simulation_steps` (timeseries) and are queried on demand. The flat counters are fast totals; the `By*` slices break them down per model definition (`Rule.ID`, `EventDefinition.UID`, `Interaction.ID`). The LLM call count is not duplicated here — use `SimulationTokenUsage.Total.Calls`.

```go
type AgentPopulation struct {
    AgentUID string `json:"agent_uid" bson:"agent_uid"`
    Current  uint64 `json:"current" bson:"current"`
    Peak     uint64 `json:"peak" bson:"peak"`
    Min      uint64 `json:"min" bson:"min"`
    Spawned  uint64 `json:"spawned" bson:"spawned"` // born during the run (excl. initial population)
    Removed  uint64 `json:"removed" bson:"removed"`
    // ByState is the CURRENT state distribution (keys = Agent.States values).
    ByState map[string]uint64 `json:"by_state,omitempty" bson:"by_state,omitempty"`
}

type RuleStatistic struct {
    RuleID     string `json:"rule_id" bson:"rule_id"`
    Executions uint64 `json:"executions" bson:"executions"`
}

type EventStatistic struct {
    EventUID string `json:"event_uid" bson:"event_uid"`
    Emitted  uint64 `json:"emitted" bson:"emitted"`
}

type InteractionStatistic struct {
    InteractionID string `json:"interaction_id" bson:"interaction_id"`
    Count         uint64 `json:"count" bson:"count"`
}

type PropertyStatistic struct {
    AgentUID    string  `json:"agent_uid" bson:"agent_uid"`
    PropertyUID string  `json:"property_uid" bson:"property_uid"`
    Avg         float64 `json:"avg" bson:"avg"`
    Min         float64 `json:"min" bson:"min"`
    Max         float64 `json:"max" bson:"max"`
}

type SimulationStatistics struct {
    StepsCompleted uint64 `json:"steps_completed" bson:"steps_completed"`
    TotalAgents    uint64 `json:"total_agents" bson:"total_agents"` // current live agent count

    Population []AgentPopulation `json:"population,omitempty" bson:"population,omitempty"` // per agent/archetype definition

    AgentsSpawned    uint64  `json:"agents_spawned" bson:"agents_spawned"`
    AgentsRemoved    uint64  `json:"agents_removed" bson:"agents_removed"`
    Interactions     uint64  `json:"interactions" bson:"interactions"`
    Events           uint64  `json:"events" bson:"events"`
    RuleExecutions   uint64  `json:"rule_executions" bson:"rule_executions"`
    MovementDistance float64 `json:"movement_distance" bson:"movement_distance"` // total distance moved by all agents

    ByRule        []RuleStatistic        `json:"by_rule,omitempty" bson:"by_rule,omitempty"`
    ByEvent       []EventStatistic       `json:"by_event,omitempty" bson:"by_event,omitempty"`
    ByInteraction []InteractionStatistic `json:"by_interaction,omitempty" bson:"by_interaction,omitempty"`
    Properties    []PropertyStatistic    `json:"properties,omitempty" bson:"properties,omitempty"` // number-typed property aggregates per agent/archetype

    Custom map[string]interface{} `json:"custom,omitempty" bson:"custom,omitempty"` // flexible extra metrics
}
```

**Fields — SimulationStatistics**:

| Field | Type | Description |
|-------|------|-------------|
| `steps_completed` | uint64 | Number of simulation steps completed |
| `total_agents` | uint64 | Current live agent count at the time of last update |
| `population` | []AgentPopulation | Per agent/archetype definition population tracking |
| `agents_spawned` | uint64 | Total agent instances spawned during the run |
| `agents_removed` | uint64 | Total agent instances removed during the run |
| `interactions` | uint64 | Total agent interactions that occurred |
| `events` | uint64 | Total events emitted |
| `rule_executions` | uint64 | Total rule firings across all agents |
| `movement_distance` | float64 | Cumulative movement distance across all agents |
| `by_rule` | []RuleStatistic | Executions per model `Rule.ID` (which rules fire) |
| `by_event` | []EventStatistic | Emissions per model `EventDefinition.UID` |
| `by_interaction` | []InteractionStatistic | Occurrences per model `Interaction.ID` |
| `properties` | []PropertyStatistic | Current avg/min/max of number-typed properties per agent/archetype (e.g. average prey energy) — feeds Level-3 analysis and widgets without scanning the timeseries |
| `custom` | map | Flexible extra metrics defined by the simulation model |

**Fields — AgentPopulation** (tracks any agent definition — plain agent or archetype):

| Field | Type | Description |
|-------|------|-------------|
| `agent_uid` | string | Model `Agent.UID` this population entry tracks |
| `current` | uint64 | Current live count for this agent/archetype definition |
| `peak` | uint64 | Peak count reached at any point during the run |
| `min` | uint64 | Minimum count reached at any point during the run |
| `spawned` | uint64 | Instances born during the run (excludes the initial population) |
| `removed` | uint64 | Instances removed during the run |
| `by_state` | map[string]uint64 | Current state distribution; keys are `Agent.States` values |

---

## SimulationStepDocument

**Collection**: `simulation_steps` (Timeseries)

> **Storage-agnostic counterpart**: `simulations.SimulationStep` (package `data/simulations`) — uses string hex IDs and no bson tags. `SimulationStepDocument` (package `data/mongodb/simulations`) is the MongoDB representation and is the type described in this section.

Per-step snapshots of agent, shape, and environment state. One document per entity per step. Stored as a timeseries collection with `timestamp` as the time field.

```go
type MetadataType string

const (
    MetadataTypeAgent       MetadataType = "agent"
    MetadataTypeShape       MetadataType = "shape"
    MetadataTypeEnvironment MetadataType = "environment"
)

type SimulationStepDocument struct {
    ID         bson.ObjectID                  `json:"id" bson:"_id,omitempty"`
    Metadata   SimulationStepDocumentMetadata `json:"metadata" bson:"metadata"`
    Timestamp  time.Time                      `json:"timestamp" bson:"timestamp"`
    Position   *common.Point                  `json:"position,omitempty" bson:"position,omitempty"`
    Properties map[string]interface{}         `json:"properties" bson:"properties"`
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Unique document identifier |
| `metadata` | SimulationStepDocumentMetadata | Step context (simulation, step number, entity type, UID) |
| `timestamp` | time.Time | When this state was recorded (timeseries index field) |
| `position` | *common.Point | Current grid position `{X, Y uint64}` (for movable agents; was `*SpacePosition` float64) |
| `properties` | map | All current properties/state for this entity at this step |

### Nested: SimulationStepDocumentMetadata

Identifies a per-entity step row. `UID` is the entity's **model definition UID**: `Agent.UID` for agent rows, the shape **template** UID for shape rows, and the environment UID for environment rows — per-archetype/per-template aggregations filter on `Type` + `UID`. `ReferenceID` is the numeric runtime **instance** ID for agents and shapes (a shape instance id for shape rows), enabling efficient per-instance filtering on the timeseries.

```go
// SimulationStepDocumentMetadata identifies a per-entity step row. UID is the model definition
// UID: Agent.UID for agent rows, the shape template UID for shape rows, and "environment" for
// environment rows. ReferenceID is the runtime instance id, unique within agents/shapes in the
// simulation.
type SimulationStepDocumentMetadata struct {
    SimulationID bson.ObjectID `json:"simulation_id" bson:"simulation_id"`
    Step         int64         `json:"step" bson:"step"`
    ReferenceID  uint64        `json:"reference_id" bson:"reference_id"`
    Type         MetadataType  `json:"type" bson:"type"`
    UID          string        `json:"uid" bson:"uid"`
}
```

**Metadata Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `simulation_id` | ObjectID | Which simulation this step belongs to |
| `step` | int64 | Step number within the simulation |
| `reference_id` | uint64 | Runtime instance ID of the agent or shape (unique within their collection in the simulation) |
| `type` | MetadataType | Entity type: `"agent"`, `"shape"`, or `"environment"` |
| `uid` | string | Entity definition UID — agent's `UID` (linking to `models.Agent`), `ShapeDocument.UID`, or environment UID |

**Changed from previous version**:

| Old field | New field | Note |
|---|---|---|
| `agent_uid string` (bson `agent_uid`) | `reference_id uint64` (bson `reference_id`) | Replaced: per-archetype grouping now uses `uid`; instance filtering uses the numeric `reference_id` |
| `position *SpacePosition` (float64) | `position *common.Point` (uint64) | On `SimulationStepDocument` (parent), not metadata |

**Enum: MetadataType**:

| Value | Description |
|-------|-------------|
| `"agent"` | Document represents agent state at this step |
| `"shape"` | Document represents shape state at this step |
| `"environment"` | Document represents environment-level state at this step |

---

## WidgetConfig

Dashboard widget configuration embedded in `SimulationDocument`.

```go
type WidgetConfig struct {
    ID       string                 `bson:"id" json:"id"`
    Type     string                 `bson:"type" json:"type"`
    Title    string                 `bson:"title" json:"title"`
    Query    map[string]interface{} `bson:"query" json:"query"`
    Position WidgetPosition         `bson:"position" json:"position"`
    Options  map[string]interface{} `bson:"options,omitempty" json:"options,omitempty"`
}

type WidgetPosition struct {
    X uint `bson:"x" json:"x"`
    Y uint `bson:"y" json:"y"`
    W uint `bson:"w" json:"w"`
    H uint `bson:"h" json:"h"`
}
```

**WidgetConfig Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique widget identifier within the simulation |
| `type` | string | Widget type (free-form; e.g. `"line_chart"`, `"scatter"`, `"heatmap"`) |
| `title` | string | Display title |
| `query` | map | Data query parameters (collection, filters, aggregation) |
| `position` | WidgetPosition | Grid position and size |
| `options` | map | Type-specific rendering options |

**WidgetPosition Fields**: `x`, `y` (grid column/row), `w`, `h` (width/height in grid units). All four fields are `uint` (previously `int`).

---

## Relationships Diagram

```
ChatDocument (chats)
    |  .llm_target.{provider_uid,model_uid,effort} ──→ [LLM provider / model / effort]
    |  .reference_id ──────────────────────────────────────────────────────────────────┐
    |                                                                                   │
    +──→ ModelDocument (models)                                                         │
    │         │  .llm_config.target.{provider_uid,model_uid,effort} ──→ [LLM target]  │
    │         │  .update_mode (ModelUpdateMode: sequential|synchronous)                │
    │         │  .chat_id ──────────────────────────────────────────→ ChatDocument     │
    │         │                                                                         │
    │         ├── Agents[] (Agent)                                                      │
    │         │       │  .uid ←── Rule.applies_to                                      │
    │         │       │        ←── Condition.target_uid(s)                             │
    │         │       │        ←── Interaction.initiator/target                        │
    │         │       │        ←── SimulationAgent.uid                                 │
    │         │       ├── .llm_config (*LLMAgentConfig)                                │
    │         │       │       ├── .target (*common.LLMTarget, per-agent override)      │
    │         │       │       ├── .prompt_context                                      │
    │         │       │       ├── .strategy (*LLMAgentStrategy)                        │
    │         │       │       └── .simulation_config (*LLMSimulationAgentConfig)       │
    │         │       └── .initialization (AgentInitialization)                        │
    │         │                                                                         │
    │         ├── Images[] (ModelImage, base64)                                         │
    │         │       └── used as Level-1 multimodal LLM input                         │
    │         │                                                                         │
    │         ├── Environment (*Environment)                                            │
    │         │       │  .width/.height uint64, .torus bool                            │
    │         │       └── Shapes[] (ShapeDocument)                                     │
    │         │               ├── .uid, .layer uint, .initialization, .geometry        │
    │         │               └── .llm_config (*LLMShapeConfig)                        │
    │         │                       ├── .target (*common.LLMTarget)                  │
    │         │                       ├── .prompt_context                              │
    │         │                       └── .strategy (*LLMShapeStrategy)               │
    │         │                                                                         │
    │         ├── Rules[] (Rule)                                                        │
    │         │       ├── Conditions[] (Condition, UID refs)                           │
    │         │       └── Actions[] (Action, UID refs)                                 │
    │         │                                                                         │
    │         ├── Interactions[] (Interaction, UID refs)                                │
    │         └── Events[] (EventDefinition, .uid)                                     │
    │                 │  .llm_target (*common.LLMTarget, per-event override)           │
    │                 ←── Action.event_uid                                              │
    │                 ←── Condition.target_uid (type=event)                            │
    │                                                                                   │
    +──→ SimulationDocument (simulations) ←────────────────────────────────────────────┘
              │  .model_id ──→ ModelDocument
              │  .model_snapshot (*ModelDocument, immutable after creation)
              │  .token_usage (*SimulationTokenUsage)
              │  .statistics (*SimulationStatistics)
              │  .files []common.File ──→ [GridFS bucket] (actual bytes via .gridfs_id)
              │
              ├── Agents[] (SimulationAgent, runtime instances)
              │       ├── .uid ──→ Agent.uid in model snapshot
              │       │   (Name/Type/MoveType/Description resolved from snapshot via uid)
              │       ├── .id uint64 (runtime instance ID)
              │       ├── .position *common.Point (uint64 grid coords)
              │       └── Memory[] (AgentMemory, type AgentMemoryType)
              │
              ├── Environment (*SimulationEnvironment, runtime state + denorm copies)
              │       │   .uid/.width/.height uint64/.torus (denormalized from snapshot)
              │       ├── .global_properties (runtime values)
              │       └── Shapes[] (SimulationShape)
              │               ├── .id uint64 (runtime instance ID)
              │               ├── .uid ──→ ShapeDocument.uid
              │               ├── .name/.type/.layer uint/.visible (denormalized)
              │               ├── .position *common.Point (uint64 grid coords)
              │               ├── .width/.height *uint64, .points []common.Point
              │               └── styling (fill/stroke/stroke_width *uint/thickness *uint)
              │
              └──→ SimulationStepDocument[] (simulation_steps, timeseries)
                        ├── .position *common.Point (uint64)
                        └── Metadata
                                ├── .uid ──→ Agent.uid or ShapeDocument.uid or env UID
                                └── .reference_id uint64 (runtime instance ID)
```

---

## Indexes and Performance Considerations

Recommended indexes for efficient querying:

| Collection | Index |
|------------|-------|
| `chats` | `{type: 1}`, `{created_at: -1}`, `{reference_id: 1}` |
| `chat_messages` | `{chat_id: 1, created_at: 1}` (compound) |
| `models` | `{chat_id: 1}`, `{created_by: 1, created_at: -1}` |
| `simulations` | `{model_id: 1}`, `{status: 1}`, `{created_by: 1}`, `{created_at: -1}` |
| `simulation_steps` | `{metadata.simulation_id: 1, metadata.step: 1, timestamp: 1}` (timeseries compound); `{metadata.simulation_id: 1, metadata.agent_uid: 1, timestamp: 1}` for per-archetype aggregation |

---

## Notes

- All timestamps use UTC `time.Time` values.
- `bson.ObjectID` is used for primary keys and cross-collection references.
- `data/models` documents use **bson tags only** (no json tags); they are always accessed via DTO mappers.
- `data/simulations` documents use **both bson and json tags** consistently, as they may be serialized directly to the API.
- The `Parameters` field on `Condition` and `Action` provides metadata-driven extensibility without code changes.
- `SimulationStepDocument` is a timeseries collection; queries should always include `metadata.simulation_id` and a time range for efficiency. Per-archetype aggregation groups on `metadata.uid` (the model `Agent.UID`). Per-instance filtering uses `metadata.reference_id` (the numeric runtime ID). The former `metadata.agent_uid` field is removed.
- `SimulationDocument.ModelSnapshot` embeds the full source `ModelDocument` snapshot for engine use (bson key `model_snapshot`; field was formerly named `Model` with bson `model`). This snapshot is **immutable after creation** and must never be rewritten on per-step `$set` updates. The field is **not** `json:"-"` — it is included in JSON serialization.
- Live state (`agents`, `environment`, `statistics`, `token_usage`, `status`, `steps`) must be persisted with targeted `$set` updates. Full `ReplaceOne` on the simulation document risks overwriting the immutable `ModelSnapshot` and approaches the 16 MB BSON document limit for large simulations.
- Entity identity throughout the system is by **UID** (not name). The reserved UID `"environment"` identifies the model's environment; `"state"` and `"position"` are reserved pseudo-property UIDs usable in conditions and actions.
- The `Agent` type was previously named `AgentDocument`; all module code must use `models.Agent`.
- `SimulationAgent.UID` (bson `uid`) links any agent instance to its `models.Agent` definition in the snapshot. The field was formerly `AgentUID` (bson `agent_uid`). Static attributes (`Name`, `Type`, `MoveType`, `Description`) are derived from that definition at engine startup; `Name` is also denormalized onto the instance (bson `name`, no json tag).
- `SimulationEnvironment` now stores denormalized copies of `UID`, `Width`, `Height`, and `Torus` alongside `GlobalProperties` and `Shapes`. Structure (shape templates) still comes from the model snapshot; only runtime values (global properties, placed shape state) change during a simulation.
- `SimulationShape` gained a numeric `ID uint64` runtime instance identifier and denormalized `Name`, `Type`, `Layer`, `Visible` from the model template. Position is `*common.Point` (uint64 grid coords); the old `X *int` / `Y *int` flat fields are removed. `Width`/`Height` are `*uint64`; `StrokeWidth`/`Thickness` are `*uint`. Vertices use `[]common.Point`. Color remains a `"color"`-typed `AgentProperty` in `Properties["color"]`.
- `SimulationDocument.TokenUsage` is `*SimulationTokenUsage` (was `*common.TokenUsageSummary`). The new type adds per-agent and per-purpose breakdowns in addition to the total and per-model axes.
- `SimulationDocument.Files` is `[]common.File` (moved from `data/simulations.SimulationFile`). The type is now `common.File` / `common.FileType`. Bytes live in a MongoDB GridFS bucket referenced by `common.File.GridFSID`.
- `common.LLMTarget` and `common.LLMEffort` are the canonical types for LLM selection. Five effort constants exist: `LLMEffortMinimal`, `LLMEffortLow`, `LLMEffortMedium`, `LLMEffortHigh`, `LLMEffortMax`. `TokenUsage` embeds `LLMTarget` inline (flat bson keys `provider_uid`, `model_uid`, `effort`).
- `LLMConfig.Target common.LLMTarget` (bson key `target`) is the model-level LLM selector. Accessed as `model.LLMConfig.Target.ProviderUID` / `.ModelUID` / `.Effort`.
- `ChatDocument.LLMTarget common.LLMTarget` (bson `llm_target`) is the chat-level selector for Level-1 model generation and Level-3 analysis.
- Per-entity LLM routing uses **config wrapper structs**: `Agent.LLMConfig *LLMAgentConfig` and `ShapeDocument.LLMConfig *LLMShapeConfig`. Each wrapper contains `Target *common.LLMTarget`, `PromptContext`, `Strategy`, and (for agents) `SimulationConfig`. `EventDefinition` keeps a direct `LLMTarget *common.LLMTarget` field (no wrapper).
- Level-2 resolution order (most specific wins): `Agent.LLMConfig.Target` / `ShapeDocument.LLMConfig.Target` / `EventDefinition.LLMTarget` → `ModelDocument.LLMConfig.Target`. There is no `SimulationDocument`-level LLM target for Level-2 (that field has been removed).
- `ModelDocument.UpdateMode ModelUpdateMode` (sequential/synchronous) specifies agent activation order. This field was moved from `SimulationDocument` and is now part of the model definition.
- `AgentMemoryType` (renamed from `AgentMemoryEventType`) has four values: `movement`, `decision`, `interaction`, `property`. The previous six values (`llm_decision`, `llm_response`, `llm_fallback`, `rule_execution`, `decision`, `interaction`) are replaced.
- **Deleted types**: `RuleState`, `SpacePosition` (local to `data/simulations`), `SimulationFile`, `SimulationFileType`. Also removed: `SimulationAgent.RuleExecutionState`. Use `common.File`/`common.Point` for artifacts and coordinates.
- `TokenUsageSummary.ByModel` is one entry per `(provider_uid, model_uid, effort)` triple. The `effort` dimension matters because different effort levels drive different token consumption patterns.

## Spatial bodies & per-entity display (2026-06-24)

Agents gained a **simulation footprint** and both agents and shapes gained a visual **display** override, kept independent (the engine never reads `Display`).

- **`AgentFootprint`** (`Agent.Footprint`, nil ⇒ point) — the agent's collision/simulation body, anchored & **centered** on the agent position. `Kind` ∈ `point|circle|rectangle|polygon`; `Radius *float64` (circle), `Width/Height *uint64` (rectangle), `Points []common.Point` (polygon, anchor-relative, centered). Fixed dims per archetype (no `ScalarSpec`) — instances stay a single `Position`, so timeseries rows / WS frames are unchanged.
- **`Display`** (`Agent.Display`, `Shape.Display`, nil/`auto` ⇒ default render) — `Kind` ∈ `auto|image|figure`; `Image *DisplayImageData{Data(base64),MimeType,Width,Height}`; `Figure []common.Point` (anchor-relative polygon, display-only); `Scale *float64`. Display images are downscaled/re-encoded by the backend on save (`pkg/imageutil`, caps `MODELS_DISPLAY_IMAGE_MAX_DIM`/`MODELS_DISPLAY_IMAGE_MAX_BYTES`).
- **`Environment.CollisionMode`** (`""`=none / `"block"`) + **`Agent.Collidable *bool`** (nil⇒true) — world collision-response mode; copied into `SimulationEnvironment` at generation like `Torus`.
- `(*Model).NormalizeSpatial()` normalizes empty kinds → defaults and clears stray point-footprint dims; called on save next to `NormalizeLLMStrategies()`. The simulations **snapshot** DTOs carry footprint/display/collision_mode so the editor + runner can render them.
