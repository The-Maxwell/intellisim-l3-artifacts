# Models Module Reference

The models module (`modules/models`) makes an **agent-based model (ABM) a first-class resource**. A researcher can build a model three ways — fully manually, manually with LLM assistance, or fully by the LLM — with strict per-user ownership, draft-first validation, and LLM generation/editing that protects user-authored content (locks, scope, user-only fields). It mirrors the [chats module](chats-module.md) async pattern and persists its companion conversation in the **shared** `chats` / `chat_messages` collections.

> Data layer: [`docs/data-model.md`](data-model.md).

---

## Overview

A **model** (`data/models.Model`, persisted as `data/mongodb/models.ModelDocument` in the `models` collection) is owned by exactly one user (`CreatedBy`) and holds the structural ABM definition: agents/archetypes, environment + shapes, rules, interactions, events, and the user-owned `LLMConfig` (provider/strategy/runtime config). CRUD is synchronous; LLM generation is asynchronous.

### Three creation paths

All three are first-class:

1. **Manual** — `POST /models {name, description?, is_spatial}` → a blank model, **no companion chat**. Edited entirely via `PUT /models/:id`.
2. **Fully by LLM** — `POST /models {name, llm_target, initial_message}` → the model is created synchronously, then the companion chat is created and the first async generation is fired (mirrors `SendMessage`). The client subscribes to SSE for the result.
3. **Manual + LLM** — create manually, then attach a target via `GET`/`PUT /models/:id/chat` and ask for assistance via `POST /models/:id/chat/messages` at any time.

### The optional companion chat

The LLM is reached only through a **1:1 companion chat**, created **lazily** on first LLM use (so a manual-only model has no chat). It is a `chats.Chat` with:

- `Type = ChatTypeModel` (`"model"`),
- `ReferenceID = modelID`, and the model's `ChatID` points back at it;

persisted in the existing shared `chats` / `chat_messages` collections via `chats.ChatsRepository` (the models service depends on that interface — the chats *module* is untouched). Each assistant turn stores both fields the data layer was designed for:

- `ChatMessage.TextContent` = the human-readable **display message** shown in the chat;
- `ChatMessage.RawContent` = the **full structured JSON** the LLM returned (`{display_message, model}`).

Deleting a model deletes its companion chat **and** all its messages (`DeleteMessagesByChatID` then `Delete`).

---

## Ownership & Auth

All routes are mounted under `/api/v1/intellisim/models` and require a valid `Authorization: Bearer <token>` (the group is wired with `auth.Middleware.Require()` in `api/api.go`). Ownership is enforced in the service layer, never in handlers.

- **Stamping**: `Model.CreatedBy` / `UpdatedBy` are set to `UserPrincipal.ID` (the `preferred_username` claim, falling back to `sub`).
- **`getOwnedModel(ctx, id)`** loads a model and gates access via `auth.RequirePrincipal` + `UserPrincipal.Authorize(model.CreatedBy)`:
  - missing **or** malformed id → `ErrModelNotFound` (**404**),
  - a model owned by another non-admin user → `auth.ErrForbidden` (**403**).
- **Admin-sees-all**: in `ListModels`, `owner := p.ID; if p.IsAdmin { owner = "" }`. An empty `ListFilter.CreatedBy` returns every owner's models. Admin capability is env-gated and reused unchanged from the chats/auth refactor: `AUTH_ADMIN_ENABLED` (default `false`) + `AUTH_ADMIN_PERMISSION` (default `intellisim:admin`); no new env vars.

---

## API Reference

Routes from `modules/models/api.go` `SetupRoutes`. All require a Bearer token.

| Method | Path | Request | Success | Description |
|--------|------|---------|---------|-------------|
| `POST` | `/models` | `CreateModelRequest` | 201 `ModelResponse` | Create (manual / seed / with-LLM kickoff) |
| `GET` | `/models` | query `page`, `page_size` | 200 `ListModelsResponse` | List caller's models (admin: all) |
| `GET` | `/models/:id` | — | 200 `ModelResponse` | Full model + advisory validation |
| `PUT` | `/models/:id` | `UpdateModelRequest` | 200 `ModelResponse` | Full manual replace (form or JSON) |
| `DELETE` | `/models/:id` | — | 204 | Delete model (+ companion chat & messages) |
| `GET` | `/models/:id/validation` | — | 200 `ValidationResult` | Recompute advisory completeness validation |
| `GET` | `/models/:id/chat` | — | 200 `ModelChatResponse` | Get the companion chat meta + target + status |
| `PUT` | `/models/:id/chat` | `SetModelChatTargetRequest` | 200 `ModelChatResponse` | Set/replace the LLM target (creates the chat if absent) |
| `GET` | `/models/:id/chat/messages` | query `before`, `limit` | 200 `MessagesPageResponse` | Cursor-paginated chat history |
| `POST` | `/models/:id/chat/messages` | `SendModelMessageRequest` | 202 `SendMessageResponse` | Enqueue async generate/edit (optional `scope`) |
| `GET` | `/models/:id/chat/events` | — | 200 SSE stream | Subscribe to real-time model-chat events |
| `POST` | `/models/:id/images` | `AddModelImageRequest` | 201 `ModelResponse` | Add one image to the managed library (compressed on add) |
| `DELETE` | `/models/:id/images/:imageId` | — | 200 `ModelResponse` | Remove one image from the managed library by UID |
| `POST` | `/models/:id/environment/generate` | `GenerateEnvironmentRequest` | 202 `SendMessageResponse` | Enqueue async environment generation from stored images (Vision) |

### Key request DTOs (`modules/models/dtos.go`)

- **`CreateModelRequest`** — `{name?, description?, is_spatial, llm_target?, initial_message?}`. With both `llm_target` **and** a non-blank `initial_message`, the companion chat is created and the first generation fired async.
- **`UpdateModelRequest`** — `{model: ModelInputDTO}`. A **full manual replace** of the editable body (`ModelInputDTO` mirrors the domain `Model` 1:1, including `llm_config`, per-entity `locked`, and `immutable`). Server-owned fields (`ID`, `ChatID`, `CreatedAt`, `CreatedBy`) are carried over from the existing model; `UpdatedAt`/`UpdatedBy` are refreshed. The image library (`Model.Images`) is **not touched** by this endpoint — it is managed exclusively via `POST/DELETE /:id/images[/:imageId]`.
- **`SetModelChatTargetRequest`** — `{llm_target}` (required). Validated for `MODEL_CAPABILITY_MODEL_GENERATION`.
- **`SendModelMessageRequest`** — `{content (required, max 10000), scope?}` where `scope = {section?, uid?}`.
- **`AddModelImageRequest`** — `{name?, description?, mime_type (png/jpeg/webp/gif, required), data (base64, required)}`. Core compresses the image via `pkg/imageutil` (max dim `MODELS_MODEL_IMAGE_MAX_DIM`, max bytes `MODELS_MODEL_IMAGE_MAX_BYTES`) before storing. Rejects with **422 `too_many_images`** if the library is already at the `MODELS_MAX_MODEL_IMAGES` cap (default 5), **422 `image_too_large`** if the raw upload exceeds `MODELS_MAX_IMAGE_BYTES`, or **422 `image_invalid`** on bad base64/format.
- **`GenerateEnvironmentRequest`** — `{llm_target?, prompt?}`. Triggers async vision generation from the model's stored image library. Rejects with **422 `no_images`** if the library is empty, **422 `too_many_images_for_model`** (with `fields.images = "remove N"`) if the stored image count exceeds the selected vision model's advertised `max_images`.
- **`ExecLLMTargetDTO`** — the models-module form of `common.LLMTarget`. Unlike the chats `LLMTargetDTO`, it carries dynamic `generation` params + an explicit `timeout_ms`, because model generation is an "execution" call the user may fine-tune. `provider_uid` / `model_uid` are optional at the DTO layer (a draft model, or an inheriting per-entity override, may carry an empty target); a target is validated against the live registry only when it is actually **used** to talk to the LLM.

### Key response DTOs (`modules/models/responses.go`)

- **`ModelResponse`** — `{id, chat_id?, model: ModelInputDTO, validation: ValidationResult, created_at, updated_at, created_by, updated_by}`.
- **`ModelSummaryResponse`** (list rows) — `{id, name, description, is_spatial, has_chat, agent_count, simulation_ready, updated_at, created_by}`.
- **`ListModelsResponse`** — `{models[], page, page_size, has_more}`.
- **`ModelChatResponse`** — `{model_id, chat_id, llm_target, model_available, status (idle|thinking), token_usage?}`. `model_available` is a live re-check of the chat target against the registry.
- **`ModelChatMessageResponse`** — `{id, chat_id, role, content (TextContent), raw? (RawContent — assistant only), llm_target?, usage?, error?, created_at, created_by}`.
- **`MessagesPageResponse`** — `{messages[], has_more}` (chronological within the page).
- **`SendMessageResponse`** — `{message, chat_status}`.
- **`ValidationResult`** — `{errors[], warnings[], simulation_ready}` where each `ValidationIssue = {path, code, message, severity}`.

### Error envelope & status codes

All errors use the shared `pkg/httpapi.APIError` envelope `{code, message, fields?}`. `mapError` (`api.go`) translates service sentinels; a `*httpapi.APIError` (e.g. the 422 from `PUT`) passes through unchanged.

| HTTP | Code | Cause |
|------|------|-------|
| 401 | `unauthorized` | Missing principal (`auth.ErrNoPrincipal`) |
| 400 | `bad_request` | Unparseable request body |
| 400 | `invalid_cursor` | `before` message does not belong to this model's chat |
| 403 | `forbidden` | Model exists but owned by another user (caller not admin) |
| 404 | `not_found` | Model missing or malformed id |
| 409 | `model_unavailable` | The chat's target provider/model is not connected |
| 409 | `effort_not_allowed` | Requested effort not in the model's declared effort list |
| 409 | `capability_not_supported` | Selected LLM lacks `MODEL_GENERATION` (or `VISION_ANALYSIS` for image gen) |
| 409 | `generation_in_progress` | A generation is already running for this model |
| 409 | `no_chat` | A chat operation was attempted but the model has no companion chat |
| 409 | `not_spatial` | Environment/image operation on a non-spatial model |
| 422 | `too_many_images` | Image add rejected: library already at the `MODELS_MAX_MODEL_IMAGES` cap |
| 422 | `image_too_large` | Raw upload exceeds `MODELS_MAX_IMAGE_BYTES` |
| 422 | `image_invalid` | Non-base64 or unsupported mime type |
| 422 | `no_images` | `POST /environment/generate` called with an empty image library |
| 422 | `too_many_images_for_model` | Stored image count exceeds the selected vision model's `max_images`; `fields.images` = `"remove N"` |
| 404 | `image_not_found` | `DELETE /:id/images/:imageId` — no image with that UID in the library |
| 422 | `validation_failed` | DTO field-constraint violation (`go-playground/validator`), `ErrNoChanges`, or field-shape validation on `PUT` (with `fields` map) |
| 501 | `not_implemented` | `ErrGenerationNotImplemented` (legacy sentinel; not on the live happy path) |
| 500 | `internal` | Unexpected server error |

> `validation_failed` is uniformly **422** app-wide: DTO `validate`-tag failures (`go-playground/validator`) and `ErrNoChanges` share the code with the field-*shape* gate raised by `ValidateShape` on `PUT` (built by `shapeValidationError`, with a `fields` map of `path → message`).

---

## Validation (draft-first)

Two independent tiers (`modules/models/validation.go`):

- **`ValidateShape(Model) []ValidationIssue` — field-shape, always enforced on `PUT`.** A non-empty result **blocks the save** with **422 `validation_failed`**. Checks: enum membership (agent/shape/condition/action/event/placement types, operators, modifiers, scopes, producers), numeric ranges (`min_value ≤ max_value`), `ScalarSpec` exactly-one-of (`Value` XOR `Min`+`Max`, `Min ≤ Max`), UID uniqueness across agents + shapes + events, the reserved `"environment"` UID, geometry-vs-placement consistency (`Placement.Type != "llm"` ⇒ type-correct `Geometry`: rectangle needs width+height, circle needs radius, polygon ≥3 points, line exactly 2), and `Geometry.Position` set ⇒ `PopulationCount == 1`.
- **`ValidateCompleteness(Model) ValidationResult` — advisory, never blocks save.** Returns `{errors, warnings, simulation_ready}` with `simulation_ready = (len(errors) == 0)`. Examples: no agents (error), spatial model missing environment width/height (error), `initial_state ∉ Agent.States` (error), `Rule.AppliesTo` referencing an unknown agent UID (error), missing name (warning). It is attached to every `ModelResponse` (via `buildResponse`) and recomputed on `GET /models/:id/validation`.

**Draft-first philosophy**: a model can always be saved as a draft even if incomplete. The strict simulation-ready *gate* (rejecting a non-`simulation_ready` model) is **deferred to simulation creation** and is out of scope for this module.

---

## LLM Partial-Edit — the Three Layers

The LLM can never corrupt user-authored content. Three independent protection layers (design spec §6):

### (a) User-only fields stripped from the LLM contract

`modules/models/llmcontract.go` defines a **separate** DTO set (`LLMGenerationEnvelope`, `LLMModel`, `LLMAgent`, `LLMShape`, `LLMRule`, …) that is the exact JSON the LLM is allowed to author. It mirrors only the **structural / behavioral** fields and deliberately **excludes** every user-only field:

- all LLM routing/strategy/config (`LLMConfig`, every nested `common.LLMTarget`/`LLMStrategy`/`LLMAgentConfig`/`LLMShapeConfig`/`LLMSimulationConfig`/`FallbackPolicy`, `EventDefinition.LLMTarget`, generation params, `TimeoutMs`);
- protection/semantics flags: `locked` (all entities) and `AgentProperty.Immutable`;
- identity/audit: `ID`, `ChatID`, `CreatedBy`/`UpdatedBy`, `CreatedAt`/`UpdatedAt`, `update_mode`, `spatial_cell_size`.

It **includes** the descriptive free-text `llm_prompt_context` hints wherever the domain has them (the approved exception — these are guidance authored *for* the LLM, so it may rewrite them).

### (b) Per-call `EditScope`

`POST /models/:id/chat/messages` accepts an optional `scope` (domain `EditScope{Section, UID}`; `EditScopeDTO` validates `section ∈ {agents, environment, rules, interactions, events}`):

- empty / absent → the **whole model**;
- `section` only → that whole section;
- `section + uid` → that **single entity** (agent/shape by UID, rule/interaction by ID, event by UID).

A smaller scope means a smaller prompt (only the relevant schema slice + the current values of that slice) and a smaller merge. The image→environment path always uses `EditScope{Section: "environment"}`.

### (c) Per-entity `locked`

A new user-only `Locked bool` exists on `Agent`, `Shape`, `Rule`, `Interaction`, and `EventDefinition` (domain + bson `locked,omitempty` + DTO). On any LLM merge, a locked entity is preserved verbatim and a warning is emitted; the prompt lists locked entities explicitly under **"FROZEN ENTITIES — do NOT modify, rename, or remove these"**.

### The fail-soft merge (`llmmapper.go`)

`llmMerge(existing, envelope, scope) (Model, []MergeWarning)` applies the LLM output. It **never panics** and degrades gracefully, governed by four hard rules:

- **SCOPE** — only in-scope sections are touched; out-of-scope sections are copied through untouched. With `scope.UID` set, incoming entities not matching that UID are ignored with a warning.
- **UPSERT** — entities are matched by key (UID for agents/shapes/events, ID for rules/interactions). A known key updates **in place** (preserving `CreatedAt`, locked status, and user-only sub-fields); an unknown key appends a new entity. **Existing entities absent from the incoming slice are retained** — a subset edit never deletes others.
- **LOCKS** — an existing locked entity is skipped (warning), never modified.
- **USER-ONLY PRESERVATION** — `LLMConfig`, `Locked`, `Immutable`, `EventDefinition.LLMTarget`, timestamps and identity are carried over from the previous entity verbatim; the LLM can never overwrite them.

#### Field-level retention on UPDATE

The edit contract is incremental at **both** the entity and the field level. On an **update of an existing entity** the LLM emits only the fields it is changing; every omitted field keeps its current value. The contract DTOs (`llmcontract.go`) make absence distinguishable from an explicit zero:

- **Scalar fields where the zero value is meaningful** — bools (`enabled`, `execute_once`, `bidirectional`, `use_llm`, `visible`, `torus`, `is_spatial`) and numerics (`priority`, `cooldown_steps`, `ttl_steps`, `layer`, `width`, `height`, `population_count`, `spawn_count`, geometry scalars) — are **pointer-typed with `omitempty`**. On update a nil pointer keeps the previous value; an explicit value (including the zero, e.g. `"enabled": false`) is applied.
- **Strings** (`name`, `description`, `type`, `move_type`, `initial_state`, `llm_prompt_context`, …) — an empty/absent string keeps the previous value on update; on create it is simply empty.
- **Slices** (`applies_to`, `conditions`, `actions`, `states`, `properties`, initiator/target UID lists, `producers`, geometry `points`, …) — `nil` (absent) keeps the previous list; a non-nil slice — **including an explicitly emitted empty array `[]`** — replaces it, so an empty array CLEARS the list.

**New-entity defaults** (id/uid not matched): scalar fields take their schema default. `enabled` defaults to **true** for a newly created rule/interaction (an LLM creating one intends it active); all other absent bools default to **false**; absent numerics default to `0`.

Per-field fail-soft: an invalid/missing field keeps the previous value (on update) or a schema default (on new), accumulating `MergeWarning{Path, Reason}` rather than failing. Invalid enums (agent/move/property/shape/event-scope/producer types) fall back to the previous value or a sensible default with a warning. Warnings are logged (`info`) by the generate goroutine; they do not block persistence.

> The merge input model and its slices are never mutated — `out` is a value copy and each section is rebuilt as a fresh slice.

### Prompt engineering (`prompt.go`)

`buildSystemPrompt(model, scope)` assembles: a strict role/contract header (Level-1 framing; "return ONLY a single JSON envelope `{display_message, model}`, no prose/markdown/fences"; referential-integrity & reserved-`environment`-UID rules; the incremental-edit instruction — incremental at **both** the entity and field level: re-emit only changed entities, and within a changed entity emit only the changed fields, with an empty array `[]` to clear a list and omission to keep it), a **scope-trimmed schema description** (`schemaDocFor`, kept in lockstep with `llmcontract.go`), the **current in-scope model slice** as a baseline to edit (`currentSliceJSON` → `projectToContract`), the list of **frozen (locked) entities** (`lockedSummary`), a scope instruction, and the envelope contract + a compact few-shot example. When rules or interactions are in scope (or the scope is the whole model) it also appends the `schemaDocSemantics` legend — one line per non-obvious condition/action family (proximity, count, barrier, memory, decision, composite, random; interact, call_llm, transform, teleport, emit_event, environment-global property writes), verified against the engine. The header instructs the LLM to write `display_message` in the researcher's language. See [llm-prompts.md](llm-prompts.md) for the full prompt reference. `buildVisionSystemPrompt` is a separate prompt asking for **only** the `environment` slice (shapes with grid coordinates, top-left origin, the `color`-property convention, fixed `geometry.position` + `population_count == 1`).

---

## Async Generation Flow

Generation is two-phase, mirroring the chats module.

**HTTP phase (synchronous, fast) — `SendMessage` → `startGeneration`:**

1. `getOwnedModel` (ownership gate); require a companion chat (else `ErrNoChat`).
2. Validate the chat's target for `MODEL_GENERATION` (`validateGenTarget` → `llm.Service.ValidateTarget`).
3. Reserve the per-model slot via `pendingTracker.tryStart` — `ErrGenerationInProgress` (409) if already running.
4. Persist the user message (`chatsRepo.AddMessage`).
5. Publish `message.created` + `chat.status {status: "thinking"}` on the `EventsHub`.
6. Detach the generation goroutine: `go s.generate(context.WithoutCancel(ctx), …)` — so it survives the 202 return.
7. Return **202** with the user message DTO and `chat_status: "thinking"`.

**Background phase (`generate`):**

1. `defer`: `pending.finish` **before** publishing `chat.status idle` (so a client reacting to idle never reads a stale "thinking").
2. Call under a `MODELS_GENERATION_TIMEOUT`-bounded context. Load history (up to `MODELS_HISTORY_LIMIT` prior messages, chronological, error-marked messages filtered out).
3. `Generator.Generate` → `llm.Service.Chat` (`ResponseFormat = JSON`, purpose `MODEL_GENERATION`) → `parseEnvelope` (tolerant: strips a code fence, extracts the outermost balanced `{...}`) → `llmMerge`.
4. Build the assistant message: on success `TextContent = display message`, `RawContent = raw JSON`, plus token usage; on failure an **error-marked** assistant message (`{code, message}` from `classifyLLMError`) with empty `TextContent` (so history stays truthful after reload).
5. **Persist on a fresh 10s context** (`context.WithoutCancel` + new deadline) since the call context may already be expired.
6. On success: persist the **merged model** (full-document replace — preserving `ID`/`ChatID`/`CreatedAt`/`CreatedBy`; refreshing `UpdatedAt`/`UpdatedBy` to `llm:<provider>`), update the chat's `TokenUsage` aggregate + `UpdatedAt`, then publish `message.created` + `model.updated` (carrying the full `ModelResponse`, validation included) + `chat.status idle`. On failure: publish `message.created` (the error message) + `chat.error`.

### SSE (`EventsHub` + `pendingTracker`, `GET /models/:id/chat/events`)

`EventsHub` is an in-process per-model pub/sub (subscriber buffer 16; a slow subscriber that overflows is disconnected and must reconnect + refetch — **no replay**). The stream sets `text/event-stream`, sends a `: ping` heartbeat every 25 s, and is gated by the same ownership check as `GET /models/:id` (404 for missing/foreign). `pendingTracker` is an in-memory `map[modelID]` enforcing one in-flight generation per model and answering the transient `idle`/`thinking` status; a crash empties it, so post-restart status is correctly `idle`.

**Event types** (`events.go`): `message.created`, `model.updated` (payload `ModelUpdatedPayload{model_id, model}` — the `ModelResponse` already embeds `validation`), `chat.status` (`ChatStatusPayload{model_id, status}`), `chat.error` (`ChatErrorPayload{model_id, code, message}`), and `model.deleted` (`ModelDeletedPayload{model_id}`, published by `DeleteModel`, after which the hub closes all subscribers via `CloseChat`).

---

## Image Library & Environment Generation (Vision)

Models carry a managed library of up to `MODELS_MAX_MODEL_IMAGES` (default **5**) reference/schematic images. Images are added and removed via dedicated endpoints; the library persists on the model document regardless of whether a generation is in progress.

### Image library management

- **`POST /models/:id/images`** — adds one image to the library. The raw upload is guarded by `MODELS_MAX_IMAGE_BYTES` (default 6 MiB); core then compresses the image via `pkg/imageutil` (resize to `MODELS_MODEL_IMAGE_MAX_DIM`, clamp to `MODELS_MODEL_IMAGE_MAX_BYTES`) and stores the result as base64 on `Model.Images` with a generated UID. Returns **201** with the full `ModelResponse`. Error codes: **422 `too_many_images`** (library cap reached), **422 `image_too_large`** (raw upload exceeds guard), **422 `image_invalid`** (bad base64 or unsupported MIME type).
- **`DELETE /models/:id/images/:imageId`** — removes one image by UID. Returns **200** with the full `ModelResponse`. Error code: **404 `image_not_found`**.
- A full-model `PUT /models/:id` (UpdateModel) **does not touch** the image library — it preserves the persisted `Images` slice unchanged.

### Async environment generation

`POST /models/:id/environment/generate` triggers vision-based environment generation from the stored image library.

- **Spatial-only**: a non-spatial model returns **409 `not_spatial`** (`ErrNotSpatial`).
- **Vision target**: `resolveVisionTarget` picks the target — an explicit request `llm_target` (validated for `MODEL_CAPABILITY_VISION_ANALYSIS`) wins; otherwise the model chat's target is reused **if** it supports vision. A capability mismatch maps to **409 `capability_not_supported`**.
- **Image count validation**: the selected vision model's registered `max_images` (from the provider's gRPC registration; `0` → treated as `1`) is compared against the library size. If the library exceeds that limit, the request is rejected with **422 `too_many_images_for_model`** (with `fields.images = "remove N"`) before any goroutine is started. An empty library returns **422 `no_images`**.
- **Multi-image prompt**: `Generator.GenerateEnvironment` calls `llm.Service.Vision` with a `VisionRequest` carrying all stored images as `repeated VisionImage`. Each image's position label and description (when set) are sent as a text part immediately before that image in the prompt, so the LLM can tell the reference images apart and synthesise **one** environment from all of them. `ResponseFormat = JSON`.
- **Async, no chat turn**: reserves the `pendingTracker` slot (409 `generation_in_progress` if occupied), then `go s.generateEnvironment(...)`. On completion the merged model is persisted and `model.updated` is published on the `EventsHub`; on failure `chat.error` is published. No conversation message is written, because the vision target may differ from the chat target. Returns **202** with `chat_status: "thinking"`.

---

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `MODELS_GENERATION_TIMEOUT` | `300s` | Deadline for one background generation/vision LLM call. |
| `MODELS_HISTORY_LIMIT` | `30` | Max prior model-chat messages sent as LLM context. |
| `MODELS_MAX_IMAGE_BYTES` | `6291456` (6 MiB) | Raw upload guard: max decoded bytes of a single incoming image before compression. |
| `MODELS_MAX_MODEL_IMAGES` | `5` | Maximum number of images the managed library may hold per model. |
| `MODELS_MODEL_IMAGE_MAX_DIM` | `1536` | Compression target: images are resized so neither dimension exceeds this (pixels). |
| `MODELS_MODEL_IMAGE_MAX_BYTES` | `1572864` (1.5 MiB) | Compression target: compressed image byte cap stored in the library. |

(Admin-listing behavior reuses `AUTH_ADMIN_ENABLED` / `AUTH_ADMIN_PERMISSION` from the auth module — no models-specific env var.)

---

## DTO Layering

Four distinct, non-shared sets — no cross-layer reuse:

1. **Domain entities** — `data/models` (no tags; mapper-translated). The source of truth.
2. **bson documents** — `data/mongodb/models` (`*Document` structs; `docMapper`).
3. **Module API DTOs** — `modules/models/dtos.go` (request DTOs) + `responses.go` (`ModelInputDTO`, `ModelResponse`, `ModelSummaryResponse`, `ModelChatResponse`, `ModelChatMessageResponse`, `ValidationResult`, SSE payloads). Translated by `ModelMapper` (`mapper.go`).
4. **LLM-contract DTOs** — `modules/models/llmcontract.go` (the *stripped* structural schema the prompt emits/parses). The single source of truth for the prompt's schema description and response parsing; translated by the fail-soft `llmMapper` (`llmmapper.go`) and the projection helpers in `prompt.go`.

---

## FX Wiring

`modules/models.Module` (`fx.go`) provides `NewModuleConfig`, `NewEventsHub`, `NewModelMapper`, `NewGenerator`, `NewModelService`, `NewModelAPI`. The service depends on `data/models.ModelsRepository`, `data/chats.ChatsRepository`, `modules/llm.Service`, the `Generator`, the `EventsHub`, and `ModuleConfig`. The `/models` group is mounted (with the auth-required middleware) in `api/api.go`.
